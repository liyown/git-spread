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

type ActionHandler func(Action, int) (state.Run, string, error)

type Model struct {
	run        state.Run
	cursor     int
	message    string
	handler    ActionHandler
	LastAction Action
}

func NewModel(run state.Run) Model {
	return Model{run: run}
}

func NewModelWithHandler(run state.Run, handler ActionHandler) Model {
	return Model{run: run, handler: handler}
}

func Run(run state.Run) error {
	_, err := tea.NewProgram(NewModel(run)).Run()
	return err
}

func RunWithHandler(run state.Run, handler ActionHandler) error {
	_, err := tea.NewProgram(NewModelWithHandler(run, handler)).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return nil
}

type actionResultMsg struct {
	run     state.Run
	message string
	err     error
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
			return m, m.runAction(ActionOpenWorkspace)
		case "r":
			m.LastAction = ActionRefresh
			return m, m.runAction(ActionRefresh)
		case "c":
			m.LastAction = ActionContinue
			return m, m.runAction(ActionContinue)
		case "p":
			m.LastAction = ActionSwitchToPR
			return m, m.runAction(ActionSwitchToPR)
		case "a":
			m.LastAction = ActionAbort
			return m, m.runAction(ActionAbort)
		}
	case actionResultMsg:
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.run = msg.run
		m.message = msg.message
	}
	return m, nil
}

func (m Model) runAction(action Action) tea.Cmd {
	if m.handler == nil {
		return nil
	}
	cursor := m.cursor
	return func() tea.Msg {
		run, message, err := m.handler(action, cursor)
		return actionResultMsg{run: run, message: message, err: err}
	}
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
	if m.message != "" {
		fmt.Fprintf(&b, "\n%s\n", m.message)
	}
	fmt.Fprintf(&b, "\nActions\n  o   open workspace in editor\n  r   refresh status\n  c   continue\n  p   create PR instead\n  a   abort run\n")
	view := tea.NewView(b.String())
	view.AltScreen = true
	return view
}
