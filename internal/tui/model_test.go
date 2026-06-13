package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/liyown/git-spread/internal/state"
)

func updateWithActionResult(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, batched := range batch {
			msg = batched()
			if _, ok := msg.(actionResultMsg); ok {
				updated, _ := m.Update(msg)
				return updated.(Model)
			}
		}
		t.Fatalf("batch did not contain action result: %#v", batch)
	}
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestTaskListShowsTasksAndRunsSelectedTask(t *testing.T) {
	called := false
	m := NewTaskModelWithHandler([]TaskItem{
		{Name: "release", Kind: "branch", Source: "develop", Targets: []string{"release/*", "main"}, Mode: "direct"},
		{Name: "backport", Kind: "commit", Targets: []string{"release/*"}, Mode: "pr"},
	}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
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
	for _, want := range []string{"Git Spread Control Console", "Tasks", "release", "backport", "develop -> release/*, main", "Enter run", "p plan"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	updated, cmd := updated.(Model).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected task run command")
	}
	model := updateWithActionResult(t, updated.(Model), cmd)
	if !called {
		t.Fatal("expected task handler to be called")
	}
	if !strings.Contains(model.View().Content, "Targets") || !strings.Contains(model.View().Content, "started backport") {
		t.Fatalf("expected run panel after task run:\n%s", model.View().Content)
	}
}

func TestTaskViewUsesFramedLayout(t *testing.T) {
	m := NewTaskModel([]TaskItem{
		{Name: "release", Kind: "branch", Source: "develop", Targets: []string{"release/*", "main"}, Mode: "direct"},
	})
	view := m.View().Content
	for _, want := range []string{"┌", "┐", "└", "┘", "Git Spread Control Console", "Preview", "Actions"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestTaskViewShowsEmptyStateWithInitHint(t *testing.T) {
	m := NewTaskModel(nil)
	view := m.View().Content
	for _, want := range []string{"No tasks configured", "git spread init", "Create .git-spread.yml"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestTaskListPlanShowsPlanMessage(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Source: "develop", Targets: []string{"main"}, Mode: "direct"}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		if action != ActionPlanTask {
			t.Fatalf("action = %q, want plan task", action)
		}
		return state.Run{}, "Plan\n  release -> main", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	if cmd == nil {
		t.Fatal("expected plan command")
	}
	model := updateWithActionResult(t, updated.(Model), cmd)
	if !strings.Contains(model.View().Content, "Plan") {
		t.Fatalf("view missing plan message:\n%s", model.View().Content)
	}
}

func TestTaskRunShowsProcessingMessageBeforeCommandCompletes(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Targets: []string{"main"}}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		return state.Run{Targets: []state.Target{{Branch: "main", Status: state.StatusRunning}}}, "started", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected run command")
	}
	view := updated.(Model).View().Content
	if !strings.Contains(view, "Working: Starting selected task") {
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
	for _, want := range []string{"release/1.1", ".spread/release-1.1", "user.go", "Next action", "Open workspace, resolve conflicts"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewShowsFailedTargetActionAndFailure(t *testing.T) {
	m := NewModel(state.Run{
		ID:   "run-1",
		Mode: "direct",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusFailed, Error: "merge failed"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"Next action", "Read the error", "Failure", "merge failed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Current issue") {
		t.Fatalf("view should not use vague issue title:\n%s", view)
	}
}

func TestViewHidesStaleErrorForDoneTarget(t *testing.T) {
	m := NewModel(state.Run{
		ID:   "run-1",
		Mode: "direct",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusDone, Error: "old worktree failure"},
		},
	})
	view := m.View().Content
	if strings.Contains(view, "Current issue") || strings.Contains(view, "Failure") || strings.Contains(view, "old worktree failure") {
		t.Fatalf("done target should not show stale error:\n%s", view)
	}
}

func TestRunViewUsesReadableStatusLabels(t *testing.T) {
	m := NewModel(state.Run{
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusRejected, Error: "push rejected"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"push rejected", "Push rejected", "rerun this propagation with --mode pr"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "XX") || strings.Contains(view, "OK") {
		t.Fatalf("view should not use symbolic status codes:\n%s", view)
	}
}

func TestRunViewShowsBlockedTargetAsActionNeeded(t *testing.T) {
	m := NewModel(state.Run{
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusBlocked, WorkspacePath: ".spread/main", Error: "Workspace has uncommitted changes"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"needs action", "Action needed", "Open workspace, commit/stash/discard"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "failed") || strings.Contains(view, "Git error") {
		t.Fatalf("blocked target should not look like a git failure:\n%s", view)
	}
}

func TestRunViewUsesFramedTargetsDetailsActionsLayout(t *testing.T) {
	m := NewModel(state.Run{
		ID:     "run-1",
		Source: "develop",
		Mode:   "direct",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusRejected, Error: "push rejected"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"┌", "┐", "└", "┘", "Run overview", "Targets", "Target details", "Actions", "Target: main", "Status: push rejected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewShowsProgressAndStatusSummary(t *testing.T) {
	m := NewModel(state.Run{
		ID:     "run-1",
		Source: "develop",
		Mode:   "direct",
		Targets: []state.Target{
			{Branch: "release/1.0", Status: state.StatusDone},
			{Branch: "release/1.1", Status: state.StatusRunning},
			{Branch: "release/1.2", Status: state.StatusConflict},
			{Branch: "main", Status: state.StatusPending},
		},
	})
	view := m.View().Content
	for _, want := range []string{"Progress", "1/4 complete", "25%", "done 1", "running 1", "conflict 1", "pending 1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewShowsCurrentStepForRunningTarget(t *testing.T) {
	m := NewModel(state.Run{
		ID:     "run-1",
		Source: "develop",
		Mode:   "direct",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusRunning, Step: "merge develop", WorkspacePath: ".spread/main"},
		},
	})
	view := m.View().Content
	for _, want := range []string{"Current step", "merge develop"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestActionStartsProgressEventSubscription(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Targets: []string{"main"}}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		return state.Run{Targets: []state.Target{{Branch: "main", Status: state.StatusRunning}}}, "started", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected command")
	}
	model := updated.(Model)
	if model.progress == nil {
		t.Fatal("expected action to create a progress event channel")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) < 2 {
		t.Fatalf("expected batched action and progress event command, got %#v", msg)
	}
}

func TestProgressEventKeepsProcessingAndUpdatesCurrentStep(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Targets: []string{"main"}}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		return state.Run{}, "", nil
	})
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)

	updated, cmd := model.Update(progressEventMsg{
		run: state.Run{
			ID:     "run-1",
			Source: "develop",
			Mode:   "direct",
			Targets: []state.Target{
				{Branch: "main", Status: state.StatusRunning, Step: "merge develop"},
			},
		},
		message: "main: merge develop",
	})
	model = updated.(Model)
	if !model.processing {
		t.Fatal("progress update should keep action processing")
	}
	if cmd == nil {
		t.Fatal("expected next progress event wait command")
	}
	view := model.View().Content
	for _, want := range []string{"Working: main: merge develop", "Current step", "merge develop"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestActionHandlerCanReportProgressEvent(t *testing.T) {
	m := NewTaskModelWithHandler([]TaskItem{{Name: "release", Kind: "branch", Targets: []string{"main"}}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		run := state.Run{
			ID:     "run-1",
			Source: "develop",
			Mode:   "direct",
			Targets: []state.Target{
				{Branch: "main", Status: state.StatusRunning, Step: "merge develop"},
			},
		}
		progress.Report(run, "main: merge develop")
		return run, "started", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	msg := cmd()
	batch := msg.(tea.BatchMsg)
	actionMsg := batch[0]()
	eventMsg := batch[1]()
	if _, ok := eventMsg.(progressEventMsg); !ok {
		t.Fatalf("event message = %#v, want progress event", eventMsg)
	}
	updated, _ = model.Update(eventMsg)
	model = updated.(Model)
	if !strings.Contains(model.View().Content, "main: merge develop") {
		t.Fatalf("view missing progress event:\n%s", model.View().Content)
	}
	updated, _ = model.Update(actionMsg)
	if updated.(Model).processing {
		t.Fatal("action result should finish processing")
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
		{key: "x", want: ActionReset},
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

func TestResetKeyClearsRunView(t *testing.T) {
	m := NewModelWithHandler(state.Run{ID: "run-1", Targets: []state.Target{{Branch: "main", Status: state.StatusBlocked}}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		if action != ActionReset {
			t.Fatalf("action = %q, want reset", action)
		}
		return state.Run{}, "Reset Git Spread state. Press q to quit or restart git-spread.", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	if cmd == nil {
		t.Fatal("expected reset command")
	}
	model := updateWithActionResult(t, updated.(Model), cmd)
	view := model.View().Content
	for _, want := range []string{"no targets", "Reset Git Spread state"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "main") {
		t.Fatalf("view should not show stale target after reset:\n%s", view)
	}
}

func TestKeyBindingRunsHandlerAndUpdatesMessage(t *testing.T) {
	called := false
	m := NewModelWithHandler(state.Run{Targets: []state.Target{{Branch: "main", Status: state.StatusFailed}}}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
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
	model := updateWithActionResult(t, updated.(Model), cmd)
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

func TestRunScreenPRHelpShowsExecutableCommandHint(t *testing.T) {
	m := NewModelWithHandler(state.Run{
		Kind:   "branch",
		Source: "develop",
		Mode:   "direct",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusRejected},
		},
	}, func(action Action, targetIndex int, progress ProgressReporter) (state.Run, string, error) {
		if action != ActionSwitchToPR {
			t.Fatalf("action = %q, want PR help", action)
		}
		return state.Run{
			Kind:   "branch",
			Source: "develop",
			Mode:   "direct",
			Targets: []state.Target{
				{Branch: "main", Status: state.StatusRejected},
			},
		}, "PR mode: run git spread branch develop --to main --mode pr", nil
	})

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	if cmd == nil {
		t.Fatal("expected PR help command")
	}
	model := updateWithActionResult(t, updated.(Model), cmd)
	view := model.View().Content
	for _, want := range []string{"PR mode", "git spread branch develop --to main --mode pr"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "not wired yet") {
		t.Fatalf("view should not expose implementation status:\n%s", view)
	}
}

func TestRunScreenCanClearMissingActiveRun(t *testing.T) {
	m := NewModel(state.Run{ID: "run-1", Targets: []state.Target{{Branch: "main", Status: state.StatusDone}}})
	updated, _ := m.Update(actionResultMsg{run: state.Run{}, message: "No active run. Press q to quit or restart git-spread."})
	view := updated.(Model).View().Content
	for _, want := range []string{"no targets", "No active run"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "state.json") || strings.Contains(view, "main") {
		t.Fatalf("view should not show stale run or state path:\n%s", view)
	}
}
