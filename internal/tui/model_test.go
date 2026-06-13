package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/liyown/git-spread/internal/state"
)

func TestTaskListShowsTasksAndRunsSelectedTask(t *testing.T) {
	called := false
	m := NewTaskModelWithHandler([]TaskItem{
		{Name: "release", Kind: "branch", Source: "develop", Targets: []string{"release/*", "main"}, Mode: "direct"},
		{Name: "backport", Kind: "commit", Targets: []string{"release/*"}, Mode: "pr"},
	}, func(action Action, targetIndex int) (state.Run, string, error) {
		called = true
		if action != ActionRunTask {
			t.Fatalf("action = %q, want run task", action)
		}
		if targetIndex != 1 {
			t.Fatalf("target index = %d, want 1", targetIndex)
		}
		return state.Run{ID: "run-1", Source: "develop", Mode: "direct", Targets: []state.Target{{Branch: "release/1.0", Status: state.StatusRunning}}}, "started backport", nil
	})
	view := m.View().Content
	for _, want := range []string{"Tasks", "release", "backport", "Enter run", "p plan"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	updated, cmd := updated.(Model).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected task run command")
	}
	updated, _ = updated.(Model).Update(cmd())
	model := updated.(Model)
	if !called {
		t.Fatal("expected task handler to be called")
	}
	if !strings.Contains(model.View().Content, "Targets") || !strings.Contains(model.View().Content, "started backport") {
		t.Fatalf("expected run panel after task run:\n%s", model.View().Content)
	}
}

func TestTaskListPlanShowsPlanMessage(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Source: "develop", Targets: []string{"main"}, Mode: "direct"}}, func(action Action, targetIndex int) (state.Run, string, error) {
		if action != ActionPlanTask {
			t.Fatalf("action = %q, want plan task", action)
		}
		return state.Run{}, "Plan\n  release -> main", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	if cmd == nil {
		t.Fatal("expected plan command")
	}
	updated, _ = updated.(Model).Update(cmd())
	if !strings.Contains(updated.(Model).View().Content, "Plan") {
		t.Fatalf("view missing plan message:\n%s", updated.(Model).View().Content)
	}
}

func TestTaskRunShowsProcessingMessageBeforeCommandCompletes(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Targets: []string{"main"}}}, func(action Action, targetIndex int) (state.Run, string, error) {
		return state.Run{Targets: []state.Target{{Branch: "main", Status: state.StatusRunning}}}, "started", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected run command")
	}
	view := updated.(Model).View().Content
	if !strings.Contains(view, "Working: Starting task") {
		t.Fatalf("view missing processing message:\n%s", view)
	}
}

func TestViewShowsConflictWorkspace(t *testing.T) {
	m := NewModel(state.Run{
		ID:     "run-1",
		Mode:   "direct",
		Source: "develop",
		Targets: []state.Target{
			{Branch: "release/1.0", Status: state.StatusDone},
			{Branch: "release/1.1", Status: state.StatusConflict, WorkspacePath: ".spread/release-1.1", ConflictedFiles: []string{"user.go", "config.yaml"}},
		},
	})
	view := m.View().Content
	for _, want := range []string{"release/1.1", ".spread/release-1.1", "user.go", "open workspace"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewShowsSelectedTargetError(t *testing.T) {
	m := NewModel(state.Run{
		ID:   "run-1",
		Mode: "direct",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusFailed, Error: "merge failed"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"Current issue", "merge failed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewUsesReadableStatusLabels(t *testing.T) {
	m := NewModel(state.Run{
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusRejected, Error: "push rejected"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"push rejected", "remote rejected the push"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "XX") || strings.Contains(view, "OK") {
		t.Fatalf("view should not use symbolic status codes:\n%s", view)
	}
}

func TestKeyBindingsSetActions(t *testing.T) {
	cases := []struct {
		key  string
		want Action
	}{
		{key: "o", want: ActionOpenWorkspace},
		{key: "r", want: ActionRefresh},
		{key: "c", want: ActionContinue},
		{key: "p", want: ActionSwitchToPR},
		{key: "a", want: ActionAbort},
	}
	for _, tc := range cases {
		m := NewModel(state.Run{Targets: []state.Target{{Branch: "release/1.0", Status: state.StatusConflict, WorkspacePath: ".spread/release-1.0"}}})
		updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: tc.key, Code: []rune(tc.key)[0]}))
		got := updated.(Model).LastAction
		if got != tc.want {
			t.Fatalf("key %q action = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestKeyBindingRunsHandlerAndUpdatesMessage(t *testing.T) {
	called := false
	m := NewModelWithHandler(state.Run{Targets: []state.Target{{Branch: "main", Status: state.StatusFailed}}}, func(action Action, targetIndex int) (state.Run, string, error) {
		called = true
		if action != ActionRefresh {
			t.Fatalf("action = %q, want refresh", action)
		}
		if targetIndex != 0 {
			t.Fatalf("target index = %d, want 0", targetIndex)
		}
		return state.Run{Targets: []state.Target{{Branch: "main", Status: state.StatusDone}}}, "refreshed", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	if cmd == nil {
		t.Fatal("expected action command")
	}
	updated, _ = updated.(Model).Update(cmd())
	model := updated.(Model)
	if !called {
		t.Fatal("expected handler to be called")
	}
	if model.run.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", model.run.Targets[0].Status)
	}
	if !strings.Contains(model.View().Content, "refreshed") {
		t.Fatalf("view missing action message:\n%s", model.View().Content)
	}
}
