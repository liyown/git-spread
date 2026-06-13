package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/liyown/git-spread/internal/state"
)

type Action string

const (
	ActionNone          Action = ""
	ActionOpenWorkspace Action = "open-workspace"
	ActionRefresh       Action = "refresh"
	ActionContinue      Action = "continue"
	ActionSwitchToPR    Action = "switch-to-pr"
	ActionAbort         Action = "abort"
)

type Model struct {
	run        state.Run
	cursor     int
	LastAction Action
}

func NewModel(run state.Run) Model {
	return Model{run: run}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.run.Targets)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "o", "enter":
			m.LastAction = ActionOpenWorkspace
		case "r":
			m.LastAction = ActionRefresh
		case "c":
			m.LastAction = ActionContinue
		case "p":
			m.LastAction = ActionSwitchToPR
		case "a":
			m.LastAction = ActionAbort
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder
	fmt.Fprintf(&b, "Git Spread\n\nSource: %s                  Mode: %s\n\nTargets\n", m.run.Source, m.run.Mode)
	for i, target := range m.run.Targets {
		prefix := " "
		if i == m.cursor {
			prefix = ">"
		}
		fmt.Fprintf(&b, "%s %-10s %s\n", prefix, target.Status, target.Branch)
		if target.Status == state.StatusConflict {
			fmt.Fprintf(&b, "\nConflict summary for %s\n  Workspace: %s\n  Files:     %s\n", target.Branch, target.WorkspacePath, strings.Join(target.ConflictedFiles, ", "))
		}
	}
	fmt.Fprintf(&b, "\nActions\n  o   open workspace in editor\n  r   refresh status\n  c   continue\n  p   create PR instead\n  a   abort run\n")
	return tea.NewView(b.String())
}
