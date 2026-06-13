package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/liyown/git-spread/internal/state"
)

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
