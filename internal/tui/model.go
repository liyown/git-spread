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

	taskListPageSize = 2
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
	ActionReset         Action = "reset"
	ActionRunTask       Action = "run-task"
	ActionPlanTask      Action = "plan-task"
	ActionPrepareTask   Action = "prepare-task"
)

type ProgressReporter interface {
	Report(state.Run, string)
}

type ActionHandler func(Action, int, ProgressReporter) (state.Run, string, error)

type Screen string

const (
	ScreenRun     Screen = "run"
	ScreenTasks   Screen = "tasks"
	ScreenConfirm Screen = "confirm"
)

type TaskItem struct {
	Name        string
	Kind        string
	Description string
	Group       string
	Source      string
	Targets     []string
	Mode        string
}

type Model struct {
	run         state.Run
	tasks       []TaskItem
	screen      Screen
	cursor      int
	message     string
	processing  bool
	handler     ActionHandler
	progress    <-chan tea.Msg
	LastAction  Action
	searching   bool
	search      string
	plan        string
	confirmTask int
}

func NewModel(run state.Run) Model {
	return Model{run: run, screen: ScreenRun, cursor: initialTargetCursor(run), confirmTask: -1}
}

func NewModelWithHandler(run state.Run, handler ActionHandler) Model {
	return Model{run: run, screen: ScreenRun, cursor: initialTargetCursor(run), handler: handler, confirmTask: -1}
}

func NewTaskModel(tasks []TaskItem) Model {
	return Model{tasks: tasks, screen: ScreenTasks, confirmTask: -1}
}

func NewTaskModelWithHandler(tasks []TaskItem, handler ActionHandler) Model {
	return Model{tasks: tasks, screen: ScreenTasks, handler: handler, confirmTask: -1}
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

type progressEventMsg struct {
	run     state.Run
	message string
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.screen == ScreenTasks && m.searching {
			return m.updateTaskSearch(msg), nil
		}
		if m.screen == ScreenConfirm {
			return m.updateConfirm(msg)
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j", "down":
			if m.screen == ScreenTasks {
				m.moveTaskCursor(1)
			} else if m.cursor < m.itemCount()-1 {
				m.cursor++
			}
		case "k", "up":
			if m.screen == ScreenTasks {
				m.moveTaskCursor(-1)
			} else if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			if m.screen == ScreenTasks {
				m.jumpTaskCursor(false)
			}
		case "G":
			if m.screen == ScreenTasks {
				m.jumpTaskCursor(true)
			}
		case "/":
			if m.screen == ScreenTasks {
				m.searching = true
				m.search = ""
				m.alignTaskCursor()
			}
		case "esc":
			return m, nil
		case "o", "enter":
			if m.screen == ScreenTasks {
				m.LastAction = ActionPrepareTask
				return m.startAction(ActionPrepareTask)
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
		case "x":
			if m.screen == ScreenTasks {
				return m, nil
			}
			m.LastAction = ActionReset
			return m.startAction(ActionReset)
		}
	case actionResultMsg:
		m.processing = false
		m.progress = nil
		if msg.err != nil {
			if len(msg.run.Targets) > 0 {
				m.run = msg.run
				m.screen = ScreenRun
				m.cursor = clampCursor(m.cursor, len(m.run.Targets))
				m.plan = ""
				m.confirmTask = -1
			}
			m.message = msg.err.Error()
			return m, nil
		}
		if m.LastAction == ActionPrepareTask {
			m.plan = msg.message
			m.message = ""
			m.confirmTask = m.selectedTaskIndex()
			m.screen = ScreenConfirm
			return m, nil
		}
		if runComplete(msg.run) && len(m.tasks) > 0 {
			m.run = msg.run
			m.screen = ScreenTasks
			m.cursor = clampCursor(m.cursor, len(m.tasks))
			m.plan = ""
			m.confirmTask = -1
		} else if len(msg.run.Targets) > 0 || m.screen == ScreenRun {
			m.run = msg.run
			m.screen = ScreenRun
			m.cursor = clampCursor(m.cursor, len(m.run.Targets))
			m.plan = ""
			m.confirmTask = -1
		}
		m.message = msg.message
	case progressEventMsg:
		if !m.processing {
			return m, nil
		}
		if len(msg.run.Targets) > 0 {
			m.run = msg.run
			m.screen = ScreenRun
			m.cursor = clampCursor(m.cursor, len(m.run.Targets))
			m.plan = ""
			m.confirmTask = -1
			if msg.message != "" {
				m.message = msg.message
			} else if step := currentStepMessage(m.run); step != "" {
				m.message = step
			}
		}
		return m, m.waitForProgress()
	case progressDoneMsg:
		return m, nil
	}
	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.screen = ScreenTasks
		m.plan = ""
		m.message = ""
		m.confirmTask = -1
		return m, nil
	case "enter":
		m.LastAction = ActionRunTask
		return m.startAction(ActionRunTask)
	default:
		return m, nil
	}
}

func (m Model) startAction(action Action) (tea.Model, tea.Cmd) {
	m.processing = true
	m.message = workingMessage(action)
	progress := make(chan tea.Msg, 32)
	m.progress = progress
	actionCmd := m.runAction(action, progress)
	if actionCmd == nil {
		return m, nil
	}
	return m, tea.Batch(actionCmd, m.waitForProgress())
}

func workingMessage(action Action) string {
	switch action {
	case ActionRunTask:
		return "Starting selected task..."
	case ActionPlanTask:
		return "Rendering execution plan..."
	case ActionPrepareTask:
		return "Preparing run confirmation..."
	case ActionOpenWorkspace:
		return "Opening workspace in editor..."
	case ActionRefresh:
		return "Refreshing run state..."
	case ActionContinue:
		return "Continuing active run..."
	case ActionSwitchToPR:
		return "Preparing PR mode guidance..."
	case ActionAbort:
		return "Aborting active run..."
	case ActionReset:
		return "Resetting local state..."
	default:
		return "Working..."
	}
}

func (m Model) runAction(action Action, progress chan tea.Msg) tea.Cmd {
	if m.handler == nil {
		return nil
	}
	cursor := m.cursor
	switch action {
	case ActionRunTask:
		if m.screen == ScreenConfirm && m.confirmTask >= 0 && m.confirmTask < len(m.tasks) {
			cursor = m.confirmTask
		} else {
			cursor = m.selectedTaskIndex()
		}
	case ActionPlanTask, ActionPrepareTask:
		cursor = m.selectedTaskIndex()
	}
	return func() tea.Msg {
		defer close(progress)
		run, message, err := m.handler(action, cursor, progressReporter{ch: progress})
		return actionResultMsg{run: run, message: message, err: err}
	}
}

type progressDoneMsg struct{}

type progressReporter struct {
	ch chan<- tea.Msg
}

func (r progressReporter) Report(run state.Run, message string) {
	if r.ch == nil {
		return
	}
	r.ch <- progressEventMsg{run: run, message: message}
}

func (m Model) waitForProgress() tea.Cmd {
	if m.progress == nil {
		return nil
	}
	progress := m.progress
	return func() tea.Msg {
		msg, ok := <-progress
		if !ok {
			return progressDoneMsg{}
		}
		return msg
	}
}

func (m Model) updateTaskSearch(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "enter":
		m.searching = false
	case "esc":
		m.searching = false
		m.search = ""
	case "backspace":
		runes := []rune(m.search)
		if len(runes) > 0 {
			m.search = string(runes[:len(runes)-1])
		}
	default:
		text := msg.String()
		if len([]rune(text)) == 1 {
			m.search += text
		}
	}
	m.alignTaskCursor()
	return m
}

func (m *Model) moveTaskCursor(delta int) {
	indices := m.taskIndices()
	if len(indices) == 0 {
		return
	}
	pos := m.taskCursorPosition(indices)
	pos += delta
	if pos < 0 {
		pos = 0
	}
	if pos >= len(indices) {
		pos = len(indices) - 1
	}
	m.cursor = indices[pos]
}

func (m *Model) jumpTaskCursor(last bool) {
	indices := m.taskIndices()
	if len(indices) == 0 {
		return
	}
	if last {
		m.cursor = indices[len(indices)-1]
		return
	}
	m.cursor = indices[0]
}

func (m *Model) alignTaskCursor() {
	indices := m.taskIndices()
	if len(indices) == 0 {
		m.cursor = 0
		return
	}
	for _, index := range indices {
		if index == m.cursor {
			return
		}
	}
	m.cursor = indices[0]
}

func (m Model) taskCursorPosition(indices []int) int {
	for i, index := range indices {
		if index == m.cursor {
			return i
		}
	}
	return 0
}

func (m Model) selectedTaskIndex() int {
	indices := m.taskIndices()
	if len(indices) == 0 {
		return -1
	}
	for _, index := range indices {
		if index == m.cursor {
			return index
		}
	}
	return indices[0]
}

func (m Model) previewTaskIndex() int {
	if m.screen == ScreenConfirm && m.confirmTask >= 0 && m.confirmTask < len(m.tasks) {
		return m.confirmTask
	}
	return m.selectedTaskIndex()
}

func (m Model) taskIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.search))
	indices := make([]int, 0, len(m.tasks))
	for i, task := range m.tasks {
		if query == "" || taskMatches(task, query) {
			indices = append(indices, i)
		}
	}
	return indices
}

func taskMatches(task TaskItem, query string) bool {
	fields := []string{
		task.Name,
		task.Kind,
		task.Description,
		task.Group,
		task.Source,
		task.Mode,
		strings.Join(task.Targets, " "),
	}
	return strings.Contains(strings.ToLower(strings.Join(fields, " ")), query)
}

func currentStepMessage(run state.Run) string {
	index := initialTargetCursor(run)
	if index < 0 || index >= len(run.Targets) {
		return ""
	}
	target := run.Targets[index]
	if target.Step == "" {
		return ""
	}
	return target.Branch + ": " + target.Step
}

func runComplete(run state.Run) bool {
	if len(run.Targets) == 0 {
		return false
	}
	for _, target := range run.Targets {
		if target.Status != state.StatusDone {
			return false
		}
	}
	return true
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
	switch m.screen {
	case ScreenTasks:
		return m.taskView()
	case ScreenConfirm:
		return m.confirmView()
	default:
		return m.runView()
	}
}

func (m Model) taskView() tea.View {
	header := titleStyle.Render("Git Spread Control Console") + "  " + subtleStyle.Render("choose a configured propagation")
	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		panel("Task overview", m.renderTaskOverview(), innerWidth),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("Tasks", m.renderTaskList(), leftWidth),
			"  ",
			panel("Preview", m.renderTaskPreview(), rightWidth),
		),
	)
	if m.message != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", messageBlock(m.processing, m.message, innerWidth))
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", actionBar("Actions", "Enter confirm   p plan   / search   g/G top/bottom   j/k move   q quit", innerWidth))
	return altView(frameStyle.Width(surfaceWidth).Render(body))
}

func (m Model) renderTaskOverview() string {
	if len(m.tasks) == 0 {
		return "Tasks: 0\nRun git spread init to create .git-spread.yml."
	}
	indices := m.taskIndices()
	lines := []string{fmt.Sprintf("Tasks: %d", len(m.tasks))}
	if m.search != "" || m.searching {
		lines = append(lines, "Search: "+m.search)
	}
	if len(indices) == 0 {
		lines = append(lines, "Matches: 0")
		return strings.Join(lines, "\n")
	}
	if len(indices) > taskListPageSize {
		start, end := taskWindow(m.taskCursorPosition(indices), len(indices))
		lines = append(lines, fmt.Sprintf("Showing %d-%d of %d. Use j/k to move, Enter to confirm.", start+1, end, len(indices)))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Select a configured propagation, preview it with p, or press Enter to confirm.")
	return strings.Join(lines, "\n")
}

func (m Model) renderTaskList() string {
	var b strings.Builder
	if len(m.tasks) == 0 {
		return subtleStyle.Render("No tasks configured")
	}
	indices := m.taskIndices()
	if len(indices) == 0 {
		return subtleStyle.Render("No matching tasks")
	}
	start, end := taskWindow(m.taskCursorPosition(indices), len(indices))
	for pos := start; pos < end; pos++ {
		i := indices[pos]
		task := m.tasks[i]
		prefix := "  "
		lineStyle := subtleStyle
		if i == m.cursor {
			prefix = "> "
			lineStyle = focusStyle
		}
		lines := []string{
			prefix + taskDisplayName(task),
			"  type " + valueOrDash(task.Kind) + "  mode " + valueOrDash(task.Mode),
		}
		if task.Description != "" {
			lines = append(lines, "  "+task.Description)
		}
		if task.Source != "" {
			lines = append(lines, "  from "+task.Source)
		}
		lines = append(lines, "  targets "+valueOrDash(strings.Join(task.Targets, ", ")))
		for _, line := range lines {
			b.WriteString(lineStyle.Render(fitLine(line, leftWidth-4)))
			b.WriteString("\n")
		}
		if pos < end-1 {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func taskWindow(cursor int, count int) (int, int) {
	if count <= 0 {
		return 0, 0
	}
	if count <= taskListPageSize {
		return 0, count
	}
	cursor = clampCursor(cursor, count)
	start := cursor - taskListPageSize + 1
	if start < 0 {
		start = 0
	}
	if start+taskListPageSize > count {
		start = count - taskListPageSize
	}
	return start, start + taskListPageSize
}

func taskDisplayName(task TaskItem) string {
	if task.Group == "" {
		return valueOrDash(task.Name)
	}
	return "[" + task.Group + "] " + valueOrDash(task.Name)
}

func (m Model) renderTaskPreview() string {
	if len(m.tasks) == 0 {
		return subtleStyle.Render("No configured propagations.\nRun git spread init to create .git-spread.yml.\nCreate .git-spread.yml with tasks for repeated flows.")
	}
	index := m.previewTaskIndex()
	if index < 0 {
		return subtleStyle.Render("No tasks match the current search.")
	}
	task := m.tasks[index]
	lines := []string{
		"Next run:",
		"  git spread run " + task.Name,
		"",
		"Name:    " + task.Name,
	}
	if task.Group != "" {
		lines = append(lines, "Group:   "+task.Group)
	}
	if task.Description != "" {
		lines = append(lines, "About:   "+task.Description)
	}
	lines = append(lines,
		"Type:    "+valueOrDash(task.Kind),
		"Mode:    "+valueOrDash(task.Mode),
	)
	lines = append(lines, "Flow:    "+taskFlow(task))
	if task.Source != "" {
		lines = append(lines, "Source:  "+task.Source)
	}
	lines = append(lines, "Targets: "+strings.Join(task.Targets, ", "))
	return strings.Join(lines, "\n")
}

func (m Model) confirmView() tea.View {
	header := titleStyle.Render("Git Spread Control Console") + "  " + subtleStyle.Render("confirm selected propagation")
	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		panel("Confirm run", m.renderTaskPreview(), innerWidth),
		"",
		panel("Execution plan", valueOrDash(m.plan), innerWidth),
	)
	if m.message != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", messageBlock(m.processing, m.message, innerWidth))
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", actionBar("Actions", "Enter run   Esc back   q quit", innerWidth))
	return altView(frameStyle.Width(surfaceWidth).Render(body))
}

func (m Model) runView() tea.View {
	headerParts := []string{titleStyle.Render("Git Spread Control Console")}
	if m.run.ID != "" {
		headerParts = append(headerParts, subtleStyle.Render("run "+m.run.ID))
	}
	headerParts = append(headerParts, subtleStyle.Render("source "+valueOrDash(m.run.Source)), subtleStyle.Render("mode "+valueOrDash(m.run.Mode)))
	body := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(headerParts, "  "),
		"",
		panel("Run overview", m.renderRunOverview(), innerWidth),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top,
			panel("Targets", m.renderTargetList(), leftWidth),
			"  ",
			panel("Target details", m.renderTargetDetails(), rightWidth),
		),
	)
	if m.message != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", messageBlock(m.processing, m.message, innerWidth))
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, "", actionBar("Actions", "enter/o open workspace   c continue   r refresh   p PR help   a abort   x reset   q quit", innerWidth))
	return altView(frameStyle.Width(surfaceWidth).Render(body))
}

func (m Model) renderRunOverview() string {
	total := len(m.run.Targets)
	if total == 0 {
		return "Progress: no active targets\nStatus: no active run"
	}
	stats := countStatuses(m.run.Targets)
	done := stats[state.StatusDone]
	percent := done * 100 / total
	lines := []string{
		fmt.Sprintf("Progress: %s %d/%d complete (%d%%)", progressBar(done, total, 18), done, total, percent),
		"Status: " + statusSummary(stats),
	}
	return strings.Join(lines, "\n")
}

func countStatuses(targets []state.Target) map[state.Status]int {
	stats := map[state.Status]int{}
	for _, target := range targets {
		stats[target.Status]++
	}
	return stats
}

func statusSummary(stats map[state.Status]int) string {
	ordered := []state.Status{
		state.StatusDone,
		state.StatusRunning,
		state.StatusConflict,
		state.StatusBlocked,
		state.StatusRejected,
		state.StatusFailed,
		state.StatusPending,
	}
	parts := make([]string, 0, len(ordered))
	for _, status := range ordered {
		count := stats[status]
		if count == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", statusLabel(status), count))
	}
	if len(parts) == 0 {
		return "no active targets"
	}
	return strings.Join(parts, "  ")
}

func progressBar(done int, total int, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat("-", width-filled) + "]"
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
		branch := target.Branch
		if target.Step != "" && target.Status == state.StatusRunning {
			branch += "  " + target.Step
		}
		line := fmt.Sprintf("%s %-18s %s", prefix, status, branch)
		if i == m.cursor {
			line = focusStyle.Render(line)
		}
		fmt.Fprintln(&b, line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderTargetDetails() string {
	if len(m.run.Targets) == 0 {
		return subtleStyle.Render("No active target.\n\nNext action:\n  Start a run from the task screen or CLI.")
	}
	target := m.run.Targets[clampCursor(m.cursor, len(m.run.Targets))]
	lines := []string{
		"Target: " + target.Branch,
		"Status: " + statusLabel(target.Status),
		"",
		"Next action:",
		"  " + nextAction(target),
	}
	if target.WorkspacePath != "" {
		lines = append(lines, "", "Workspace:", "  "+target.WorkspacePath)
	}
	if target.Step != "" && target.Status != state.StatusDone {
		lines = append(lines, "", "Current step:", "  "+target.Step)
	}
	if len(target.ConflictedFiles) > 0 {
		lines = append(lines, "", fmt.Sprintf("Conflicts: %d files", len(target.ConflictedFiles)), "  "+strings.Join(target.ConflictedFiles, ", "))
	}
	if target.PullRequestURL != "" {
		lines = append(lines, "", "Pull request:", "  "+target.PullRequestURL)
	}
	if targetIssueVisible(target.Status) {
		lines = append(lines, "", targetIssueTitle(target.Status)+":", "  "+targetIssue(target))
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

func fitLine(line string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= maxWidth {
		return line
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	limit := maxWidth - 3
	var b strings.Builder
	width := 0
	for _, r := range line {
		runeWidth := lipgloss.Width(string(r))
		if width+runeWidth > limit {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	return b.String() + "..."
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
	case state.StatusBlocked:
		return "needs action"
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

func nextAction(target state.Target) string {
	switch target.Status {
	case state.StatusDone:
		return "No action needed."
	case state.StatusRunning:
		if target.Step != "" {
			return "Wait for " + target.Step + " to finish."
		}
		return "Wait for the current Git operation to finish."
	case state.StatusPending:
		return "Waiting for earlier targets."
	case state.StatusBlocked:
		return "Open workspace, commit/stash/discard local changes, then press c to continue."
	case state.StatusRejected:
		return "rerun this propagation with --mode pr after fixing push access."
	case state.StatusFailed:
		return "Read the error, fix the workspace or abort this run."
	case state.StatusConflict:
		return "Open workspace, resolve conflicts in your editor, then press c to continue."
	default:
		return "Inspect this target before continuing."
	}
}

func targetIssueVisible(status state.Status) bool {
	switch status {
	case state.StatusBlocked, state.StatusFailed, state.StatusRejected, state.StatusConflict:
		return true
	default:
		return false
	}
}

func targetIssueTitle(status state.Status) string {
	switch status {
	case state.StatusBlocked:
		return "Action needed"
	case state.StatusRejected:
		return "Push rejected"
	case state.StatusFailed:
		return "Failure"
	case state.StatusConflict:
		return "Why paused"
	default:
		return "Details"
	}
}

func targetIssue(target state.Target) string {
	if target.Error != "" {
		return target.Error
	}
	switch target.Status {
	case state.StatusConflict:
		return "Conflicts remain in the workspace."
	case state.StatusRejected:
		return "The remote rejected the push."
	case state.StatusBlocked:
		return "The workspace needs local cleanup."
	case state.StatusFailed:
		return "Git Spread could not complete this target."
	default:
		return "-"
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
	case state.StatusBlocked, state.StatusConflict:
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
