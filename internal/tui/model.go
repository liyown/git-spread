package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/liyown/git-spread/internal/state"
)

const (
	surfaceWidth = 96
	innerWidth   = 90
	leftWidth    = 42
	rightWidth   = 46
)

var (
	frameStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("153")).
			Bold(true)
	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
	focusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("120")).
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true)
	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("114")).
		Bold(true)
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
	return Model{run: run, screen: ScreenRun, cursor: initialTargetCursor(run)}
}

func NewModelWithHandler(run state.Run, handler ActionHandler) Model {
	return Model{run: run, screen: ScreenRun, cursor: initialTargetCursor(run), handler: handler}
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
		if len(msg.run.Targets) > 0 || m.screen == ScreenRun {
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

func initialTargetCursor(run state.Run) int {
	if run.CurrentTarget >= 0 && run.CurrentTarget < len(run.Targets) && run.Targets[run.CurrentTarget].Status != state.StatusDone {
		return run.CurrentTarget
	}
	for i, target := range run.Targets {
		if target.Status != state.StatusDone {
			return i
		}
	}
	return 0
}

func (m Model) View() tea.View {
	if m.screen == ScreenTasks {
		return m.taskView()
	}
	return m.runView()
}

func (m Model) taskView() tea.View {
	header := titleStyle.Render("Git Spread - Tasks") + "  " + subtleStyle.Render("choose a configured propagation")
	tasks := m.renderTaskList()
	preview := m.renderTaskPreview()
	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("Tasks", tasks, leftWidth),
			"  ",
			panel("Preview", preview, rightWidth),
		),
	)
	if m.message != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", messageBlock(m.processing, m.message, innerWidth))
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", actionBar("Actions", "Enter run   p plan   j/k move   q quit", innerWidth))
	return altView(frameStyle.Width(surfaceWidth).Render(body))
}

func (m Model) renderTaskList() string {
	var b strings.Builder
	if len(m.tasks) == 0 {
		return subtleStyle.Render("no tasks configured")
	}
	for i, task := range m.tasks {
		prefix := " "
		lineStyle := lipgloss.NewStyle()
		if i == m.cursor {
			prefix = ">"
			lineStyle = focusStyle
		}
		fmt.Fprintf(&b, "%s %-14s %-7s %s\n", prefix, task.Name, task.Kind, task.Mode)
		if i == m.cursor {
			lines := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
			lines[len(lines)-1] = lineStyle.Render(lines[len(lines)-1])
			b.Reset()
			b.WriteString(strings.Join(lines, "\n"))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderTaskPreview() string {
	if len(m.tasks) == 0 {
		return subtleStyle.Render("Create .git-spread.yml tasks to use this view.")
	}
	task := m.tasks[clampCursor(m.cursor, len(m.tasks))]
	lines := []string{
		"Name:   " + task.Name,
		"Type:   " + valueOrDash(task.Kind),
		"Mode:   " + valueOrDash(task.Mode),
	}
	lines = append(lines, "Flow:   "+taskFlow(task))
	if task.Source != "" {
		lines = append(lines, "From:   "+task.Source)
	}
	lines = append(lines, "To:     "+strings.Join(task.Targets, ", "))
	return strings.Join(lines, "\n")
}

func (m Model) runView() tea.View {
	headerParts := []string{titleStyle.Render("Git Spread")}
	if m.run.ID != "" {
		headerParts = append(headerParts, subtleStyle.Render("run "+m.run.ID))
	}
	headerParts = append(headerParts, subtleStyle.Render("source "+valueOrDash(m.run.Source)), subtleStyle.Render("mode "+valueOrDash(m.run.Mode)))
	body := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(headerParts, "  "),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("Targets", m.renderTargetList(), leftWidth),
			"  ",
			panel("Details", m.renderTargetDetails(), rightWidth),
		),
	)
	if m.message != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", messageBlock(m.processing, m.message, innerWidth))
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", actionBar("Actions", "enter/o open workspace   c continue   r refresh   p PR help   a abort   q quit", innerWidth))
	return altView(frameStyle.Width(surfaceWidth).Render(body))
}

func (m Model) renderTargetList() string {
	var b strings.Builder
	if len(m.run.Targets) == 0 {
		return subtleStyle.Render("no targets")
	}
	for i, target := range m.run.Targets {
		prefix := " "
		if i == m.cursor {
			prefix = ">"
		}
		status := renderStatus(target.Status)
		line := fmt.Sprintf("%s %-18s %s", prefix, status, target.Branch)
		if i == m.cursor {
			line = focusStyle.Render(line)
		}
		fmt.Fprintln(&b, line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderTargetDetails() string {
	if len(m.run.Targets) == 0 {
		return subtleStyle.Render("No active target.")
	}
	target := m.run.Targets[clampCursor(m.cursor, len(m.run.Targets))]
	lines := []string{
		"Target: " + target.Branch,
		"Status: " + statusLabel(target.Status),
	}
	if target.WorkspacePath != "" {
		lines = append(lines, "", "Workspace:", "  "+target.WorkspacePath)
	}
	if len(target.ConflictedFiles) > 0 {
		lines = append(lines, "", "Conflicts:", "  "+strings.Join(target.ConflictedFiles, ", "))
	}
	if target.Error != "" && targetIssueVisible(target.Status) {
		lines = append(lines, "", "Current issue:", "  "+target.Error)
	}
	if explanation := statusExplanation(target.Status); explanation != "" {
		lines = append(lines, "", "Meaning:", "  "+explanation)
	}
	return strings.Join(lines, "\n")
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

func taskFlow(task TaskItem) string {
	targets := strings.Join(task.Targets, ", ")
	if targets == "" {
		targets = "-"
	}
	if task.Source != "" {
		return task.Source + " -> " + targets
	}
	return "-> " + targets
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

func targetIssueVisible(status state.Status) bool {
	switch status {
	case state.StatusFailed, state.StatusRejected, state.StatusConflict:
		return true
	default:
		return false
	}
}

func renderStatus(status state.Status) string {
	label := statusLabel(status)
	switch status {
	case state.StatusDone:
		return okStyle.Render(label)
	case state.StatusRunning:
		return focusStyle.Render(label)
	case state.StatusRejected, state.StatusFailed:
		return errorStyle.Render(label)
	case state.StatusConflict:
		return warnStyle.Render(label)
	default:
		return subtleStyle.Render(label)
	}
}

func panel(title string, body string, width int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Width(width).
		Render(subtleStyle.Render(title) + "\n" + body)
}

func actionBar(title string, text string, width int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Foreground(lipgloss.Color("111")).
		Width(width).
		Padding(0, 1).
		Render(subtleStyle.Render(title) + "\n" + text)
}

func messageBlock(processing bool, message string, width int) string {
	label := "Status"
	style := subtleStyle
	if processing {
		label = "Working"
		style = focusStyle
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(width).
		Padding(0, 1).
		Render(style.Render(label + ": " + message))
}

func altView(content string) tea.View {
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
