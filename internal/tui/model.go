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
	ActionRunTask       Action = "run-task"
	ActionPlanTask      Action = "plan-task"
)

type ActionHandler func(Action, int) (state.Run, string, error)

type Screen string

const (
	ScreenRun   Screen = "run"
	ScreenTasks Screen = "tasks"
)

type TaskItem struct {
	Name    string
	Kind    string
	Source  string
	Targets []string
	Mode    string
}

type Model struct {
	run        state.Run
	tasks      []TaskItem
	screen     Screen
	cursor     int
	message    string
	processing bool
	handler    ActionHandler
	LastAction Action
}

func NewModel(run state.Run) Model {
	return Model{run: run, screen: ScreenRun}
}

func NewModelWithHandler(run state.Run, handler ActionHandler) Model {
	return Model{run: run, screen: ScreenRun, handler: handler}
}

func NewTaskModel(tasks []TaskItem) Model {
	return Model{tasks: tasks, screen: ScreenTasks}
}

func NewTaskModelWithHandler(tasks []TaskItem, handler ActionHandler) Model {
	return Model{tasks: tasks, screen: ScreenTasks, handler: handler}
}

func Run(run state.Run) error {
	_, err := tea.NewProgram(NewModel(run)).Run()
	return err
}

func RunWithHandler(run state.Run, handler ActionHandler) error {
	_, err := tea.NewProgram(NewModelWithHandler(run, handler)).Run()
	return err
}

func RunTasksWithHandler(tasks []TaskItem, handler ActionHandler) error {
	_, err := tea.NewProgram(NewTaskModelWithHandler(tasks, handler)).Run()
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
			if m.cursor < m.itemCount()-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "o", "enter":
			if m.screen == ScreenTasks {
				m.LastAction = ActionRunTask
				return m.startAction(ActionRunTask)
			}
			m.LastAction = ActionOpenWorkspace
			return m.startAction(ActionOpenWorkspace)
		case "r":
			if m.screen == ScreenTasks {
				return m, nil
			}
			m.LastAction = ActionRefresh
			return m.startAction(ActionRefresh)
		case "c":
			if m.screen == ScreenTasks {
				return m, nil
			}
			m.LastAction = ActionContinue
			return m.startAction(ActionContinue)
		case "p":
			if m.screen == ScreenTasks {
				m.LastAction = ActionPlanTask
				return m.startAction(ActionPlanTask)
			}
			m.LastAction = ActionSwitchToPR
			return m.startAction(ActionSwitchToPR)
		case "a":
			if m.screen == ScreenTasks {
				return m, nil
			}
			m.LastAction = ActionAbort
			return m.startAction(ActionAbort)
		}
	case actionResultMsg:
		m.processing = false
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		if len(msg.run.Targets) > 0 {
			m.run = msg.run
			m.screen = ScreenRun
			m.cursor = clampCursor(m.cursor, len(m.run.Targets))
		}
		m.message = msg.message
	}
	return m, nil
}

func (m Model) startAction(action Action) (tea.Model, tea.Cmd) {
	m.processing = true
	if m.screen == ScreenTasks && action == ActionRunTask {
		m.message = "Starting task..."
	} else {
		m.message = "Working..."
	}
	return m, m.runAction(action)
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

func (m Model) itemCount() int {
	if m.screen == ScreenTasks {
		return len(m.tasks)
	}
	return len(m.run.Targets)
}

func clampCursor(cursor int, count int) int {
	if count <= 0 {
		return 0
	}
	if cursor >= count {
		return count - 1
	}
	return cursor
}

func (m Model) View() tea.View {
	if m.screen == ScreenTasks {
		return m.taskView()
	}
	return m.runView()
}

func (m Model) taskView() tea.View {
	var b strings.Builder
	fmt.Fprintf(&b, "Git Spread\n")
	fmt.Fprintf(&b, "Task runner\n\n")
	fmt.Fprintf(&b, "Tasks\n")
	if len(m.tasks) == 0 {
		fmt.Fprintf(&b, "  no tasks configured\n")
	} else {
		for i, task := range m.tasks {
			prefix := " "
			if i == m.cursor {
				prefix = ">"
			}
			fmt.Fprintf(&b, "%s %-16s %-7s %s\n", prefix, task.Name, task.Kind, taskSummary(task))
		}
	}
	m.writeMessage(&b)
	fmt.Fprintf(&b, "\nEnter run   p plan   q quit\n")
	return altView(b.String())
}

func (m Model) runView() tea.View {
	var b strings.Builder
	fmt.Fprintf(&b, "Git Spread\n")
	if m.run.ID != "" {
		fmt.Fprintf(&b, "Run %s\n", m.run.ID)
	}
	fmt.Fprintf(&b, "\nSource: %-20s Mode: %s\n\n", valueOrDash(m.run.Source), valueOrDash(m.run.Mode))
	fmt.Fprintf(&b, "Targets\n")
	for i, target := range m.run.Targets {
		prefix := " "
		if i == m.cursor {
			prefix = ">"
		}
		fmt.Fprintf(&b, "%s %-14s %s\n", prefix, statusLabel(target.Status), target.Branch)
		if target.Status == state.StatusConflict {
			fmt.Fprintf(&b, "\nConflict summary for %s\n  Workspace: %s\n  Files:     %s\n", target.Branch, target.WorkspacePath, strings.Join(target.ConflictedFiles, ", "))
		}
	}
	if m.cursor >= 0 && m.cursor < len(m.run.Targets) && m.run.Targets[m.cursor].Error != "" {
		fmt.Fprintf(&b, "\nCurrent issue for %s\n  %s\n", m.run.Targets[m.cursor].Branch, m.run.Targets[m.cursor].Error)
	}
	if m.cursor >= 0 && m.cursor < len(m.run.Targets) {
		if explanation := statusExplanation(m.run.Targets[m.cursor].Status); explanation != "" {
			fmt.Fprintf(&b, "\nMeaning\n  %s\n", explanation)
		}
	}
	m.writeMessage(&b)
	fmt.Fprintf(&b, "\no open workspace   c continue   r refresh   p PR help   a abort   q quit\n")
	return altView(b.String())
}

func (m Model) writeMessage(b *strings.Builder) {
	if m.message == "" {
		return
	}
	prefix := "Status"
	if m.processing {
		prefix = "Working"
	}
	fmt.Fprintf(b, "\n%s: %s\n", prefix, m.message)
}

func taskSummary(task TaskItem) string {
	targets := strings.Join(task.Targets, ", ")
	if targets == "" {
		targets = "-"
	}
	if task.Source != "" {
		return task.Source + " -> " + targets + " (" + valueOrDash(task.Mode) + ")"
	}
	return "-> " + targets + " (" + valueOrDash(task.Mode) + ")"
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func statusLabel(status state.Status) string {
	switch status {
	case state.StatusDone:
		return "done"
	case state.StatusRunning:
		return "running"
	case state.StatusConflict:
		return "conflict"
	case state.StatusRejected:
		return "push rejected"
	case state.StatusFailed:
		return "failed"
	case state.StatusPending:
		return "pending"
	default:
		return string(status)
	}
}

func statusExplanation(status state.Status) string {
	switch status {
	case state.StatusRejected:
		return "The remote rejected the push. You can retry after fixing permissions/protection, or run with --mode pr."
	case state.StatusFailed:
		return "Git Spread could not complete this target. See Current issue for the Git error."
	case state.StatusConflict:
		return "Resolve conflicts in the workspace, then press c or run git-spread continue."
	default:
		return ""
	}
}

func altView(content string) tea.View {
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
