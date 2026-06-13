package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/liyown/git-spread/internal/config"
	"github.com/liyown/git-spread/internal/editor"
	"github.com/liyown/git-spread/internal/git"
	ghclient "github.com/liyown/git-spread/internal/github"
	"github.com/liyown/git-spread/internal/spread"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/tui"
)

var Version = "dev"

type app struct {
	Init     initCmd     `cmd:"" help:"Create a .git-spread.yml config."`
	Run      runCmd      `cmd:"" help:"Run a configured task."`
	Plan     planCmd     `cmd:"" help:"Show what Git Spread would do."`
	Branch   branchCmd   `cmd:"" help:"Propagate a branch."`
	Commit   commitCmd   `cmd:"" help:"Propagate explicit commits or ranges."`
	PR       prCmd       `cmd:"pr" help:"Propagate a pull request."`
	Status   statusCmd   `cmd:"" help:"Show active run state."`
	Open     openCmd     `cmd:"" help:"Open the current conflicted workspace."`
	Continue continueCmd `cmd:"" help:"Continue a paused run."`
	Abort    abortCmd    `cmd:"" help:"Abort the active run."`
	Reset    resetCmd    `cmd:"" help:"Reset Git Spread state without deleting workspaces."`
	NoTUI    bool        `help:"Disable interactive TUI."`
}

type initCmd struct {
	Print bool `help:"Print config template instead of writing a file."`
}

type runCmd struct {
	Task  string `arg:"" help:"Configured task name."`
	Mode  string `help:"Override execution mode."`
	NoTUI bool   `help:"Disable interactive TUI."`
}

type planCmd struct {
	Branch branchCmd `cmd:"" help:"Plan branch propagation."`
	Commit commitCmd `cmd:"" help:"Plan commit propagation."`
	PR     prCmd     `cmd:"pr" help:"Plan pull request propagation."`
	Run    runCmd    `cmd:"" help:"Plan a configured task."`
	NoTUI  bool      `help:"Disable interactive TUI."`
}

type branchCmd struct {
	Source string   `arg:"" optional:"" help:"Source branch. Defaults to current branch."`
	To     []string `required:"" sep:"," help:"Target branches or patterns."`
	Mode   string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
	Task   string   `help:"Configured task defaults to apply."`
	NoTUI  bool     `help:"Disable interactive TUI."`
}

type commitCmd struct {
	Items []string `arg:"" optional:"" help:"Commit SHAs or ranges."`
	To    []string `sep:"," help:"Target branches or patterns."`
	Mode  string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
	Task  string   `help:"Configured task defaults to apply."`
	NoTUI bool     `help:"Disable interactive TUI."`
}

type prCmd struct {
	Item  string   `arg:"" optional:"" help:"Pull request number or URL."`
	To    []string `sep:"," help:"Target branches or patterns."`
	Mode  string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
	Task  string   `help:"Configured task defaults to apply."`
	NoTUI bool     `help:"Disable interactive TUI."`
}

type statusCmd struct{}

type openCmd struct {
	Editor string `enum:"auto,code,idea,cursor" default:"auto" help:"Editor to open."`
	Print  bool   `help:"Print the editor command instead of executing it."`
}

type continueCmd struct{}

type abortCmd struct{}

type resetCmd struct{}

const configTemplate = `version: 1

defaults:
  mode: direct
  remote: origin
  workspace: isolated
  workspaceDir: .spread
  editor: auto
  github:
    collaboration: auto
    forkRemote: fork

tasks:
  release:
    type: branch
    from: develop
    to:
      - release/*
      - main
`

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "git-spread %s\n", Version)
		return 0
	}
	if len(args) == 0 {
		return renderActiveRun(stdout, stderr)
	}

	var cli app
	parser, err := kong.New(&cli, kong.Name("git spread"), kong.Writers(stdout, stderr))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	code, handled := handleNonPropagation(ctx.Command(), cli, stdout, stderr)
	if handled {
		return code
	}

	input, planOnly, err := inputFromContext(ctx.Command(), cli)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return runPropagation(input, planOnly, stdout, stderr)
}

func handleNonPropagation(command string, cli app, stdout io.Writer, stderr io.Writer) (int, bool) {
	switch strings.TrimSpace(command) {
	case "init":
		if cli.Init.Print {
			fmt.Fprint(stdout, configTemplate)
			return 0, true
		}
		if err := os.WriteFile(".git-spread.yml", []byte(configTemplate), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1, true
		}
		fmt.Fprintln(stdout, "created .git-spread.yml")
		return 0, true
	case "status":
		return printStatus(stdout, stderr), true
	case "open":
		return openCurrent(cli.Open, stdout, stderr), true
	case "continue":
		return continueRun(stdout, stderr), true
	case "abort":
		return abortRun(stdout, stderr), true
	case "reset":
		return resetRun(stdout, stderr), true
	default:
		return 0, false
	}
}

func inputFromContext(command string, cli app) (spread.CLIInput, bool, error) {
	command = strings.TrimSpace(command)
	switch {
	case strings.HasPrefix(command, "plan run"):
		return spread.CLIInput{Task: cli.Plan.Run.Task, Mode: cli.Plan.Run.Mode}, true, nil
	case strings.HasPrefix(command, "plan branch"):
		return branchInput(cli.Plan.Branch), true, nil
	case strings.HasPrefix(command, "plan commit"):
		return commitInput(cli.Plan.Commit), true, nil
	case strings.HasPrefix(command, "plan pr"):
		return prInput(cli.Plan.PR), true, nil
	case strings.HasPrefix(command, "run"):
		return spread.CLIInput{Task: cli.Run.Task, Mode: cli.Run.Mode}, false, nil
	case strings.HasPrefix(command, "branch"):
		return branchInput(cli.Branch), false, nil
	case strings.HasPrefix(command, "commit"):
		return commitInput(cli.Commit), false, nil
	case strings.HasPrefix(command, "pr"):
		return prInput(cli.PR), false, nil
	default:
		return spread.CLIInput{}, false, fmt.Errorf("unsupported command %q", command)
	}
}

func branchInput(cmd branchCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindBranch, Source: cmd.Source, Targets: cmd.To, Mode: cmd.Mode, Task: cmd.Task}
}

func commitInput(cmd commitCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindCommit, Items: cmd.Items, Targets: cmd.To, Mode: cmd.Mode, Task: cmd.Task}
}

func prInput(cmd prCmd) spread.CLIInput {
	items := []string(nil)
	if cmd.Item != "" {
		items = []string{cmd.Item}
	}
	return spread.CLIInput{Kind: spread.KindPR, Items: items, Targets: cmd.To, Mode: cmd.Mode, Task: cmd.Task}
}

func runPropagation(input spread.CLIInput, planOnly bool, stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	plan, run, err := prepareAndMaybeExecute(ctx, input, planOnly)
	if err != nil {
		if errors.Is(err, errInvalidInput) {
			fmt.Fprintln(stderr, strings.TrimPrefix(err.Error(), errInvalidInput.Error()+": "))
			return 2
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	if planOnly {
		printPlan(stdout, plan)
		return 0
	}
	printRun(stdout, run)
	return 0
}

var errInvalidInput = errors.New("invalid input")

func prepareAndMaybeExecute(ctx repoContext, input spread.CLIInput, planOnly bool) (spread.Plan, state.Run, error) {
	input.Config = ctx.config
	input.CurrentBranch = ctx.currentBranch

	req, err := spread.Normalize(input)
	if err != nil {
		return spread.Plan{}, state.Run{}, fmt.Errorf("%w: %v", errInvalidInput, err)
	}
	var client ghclient.Client
	if needsGitHubClient(req, planOnly) {
		client, err = prepareGitHub(&req, ctx)
		if err != nil {
			return spread.Plan{}, state.Run{}, err
		}
	}
	plan, err := buildPlan(req, ctx.git, client)
	if err != nil {
		return spread.Plan{}, state.Run{}, err
	}
	if planOnly {
		return plan, state.Run{}, nil
	}
	run, err := executePlan(plan, ctx.git, ctx.store, client)
	if err != nil {
		return plan, run, err
	}
	return plan, run, nil
}

func needsGitHubClient(req spread.Request, planOnly bool) bool {
	if req.Kind == spread.KindPR {
		return true
	}
	return req.Mode == spread.ModePR && !planOnly
}

func buildPlan(req spread.Request, runner git.Runner, client ghclient.Client) (spread.Plan, error) {
	if req.Kind == spread.KindPR {
		return spread.BuildPlanWithGitHub(req, runner, client)
	}
	return spread.BuildPlan(req, runner)
}

func executePlan(plan spread.Plan, runner git.Runner, store state.Store, client ghclient.Client) (state.Run, error) {
	if plan.Request.Mode == spread.ModePR {
		return spread.ExecuteWithGitHub(plan, runner, store, client)
	}
	return spread.Execute(plan, runner, store)
}

func prepareGitHub(req *spread.Request, ctx repoContext) (ghclient.Client, error) {
	base, err := repositoryForRemote(ctx.git, req.Remote)
	if err != nil {
		return nil, fmt.Errorf("GitHub repository for remote %q: %w", req.Remote, err)
	}
	if req.Mode == spread.ModePR {
		if err := configurePRHead(req, ctx.git); err != nil {
			return nil, err
		}
	}
	return ghclient.NewGoGHClient(base.Owner, base.Name)
}

func configurePRHead(req *spread.Request, runner git.Runner) error {
	switch req.Collaboration {
	case "shared", "auto":
		req.HeadRemote = req.Remote
		return nil
	case "fork":
		fork, err := repositoryForRemote(runner, req.ForkRemote)
		if err != nil {
			return fmt.Errorf("github collaboration fork requires a Git remote named %q pointing to your fork: %w", req.ForkRemote, err)
		}
		req.HeadRemote = req.ForkRemote
		req.HeadOwner = fork.Owner
		return nil
	default:
		return fmt.Errorf("github collaboration %q is invalid", req.Collaboration)
	}
}

func repositoryForRemote(runner git.Runner, remote string) (repository.Repository, error) {
	if remote == "" {
		remote = "origin"
	}
	url, err := runner.Output("remote", "get-url", remote)
	if err == nil {
		return repository.Parse(strings.TrimSpace(url))
	}
	if override := os.Getenv("GH_REPO"); override != "" {
		return repository.Parse(override)
	}
	return repository.Repository{}, err
}

type repoContext struct {
	root          string
	currentBranch string
	config        config.Config
	git           git.Runner
	store         state.Store
}

func loadRepoContext() (repoContext, error) {
	rootOut, err := git.NewRunner("").Output("rev-parse", "--show-toplevel")
	if err != nil {
		return repoContext{}, err
	}
	worktreeRoot := strings.TrimSpace(rootOut)
	worktreeRunner := git.NewRunner(worktreeRoot)
	commonDirOut, err := worktreeRunner.Output("rev-parse", "--git-common-dir")
	if err != nil {
		return repoContext{}, err
	}
	commonDir := strings.TrimSpace(commonDirOut)
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktreeRoot, commonDir)
	}
	root := filepath.Dir(commonDir)
	runner := git.NewRunner(root)
	branchOut, _ := runner.Output("branch", "--show-current")
	cfg := config.Config{}
	if loaded, err := config.LoadFile(filepath.Join(root, ".git-spread.yml")); err == nil {
		cfg = loaded
	} else {
		config.ApplyDefaults(&cfg)
	}
	return repoContext{
		root:          root,
		currentBranch: strings.TrimSpace(branchOut),
		config:        cfg,
		git:           runner,
		store:         state.NewStore(filepath.Join(root, ".git", "spread")),
	}, nil
}

func printPlan(stdout io.Writer, plan spread.Plan) {
	fmt.Fprintf(stdout, "Plan\n  kind: %s\n  mode: %s\n", plan.Request.Kind, plan.Request.Mode)
	if plan.Request.Source != "" {
		fmt.Fprintf(stdout, "  source: %s\n", plan.Request.Source)
	}
	if len(plan.Commits) > 0 {
		fmt.Fprintf(stdout, "  commits: %s\n", strings.Join(plan.Commits, ", "))
	}
	fmt.Fprintln(stdout, "  targets:")
	for _, target := range plan.Targets {
		fmt.Fprintf(stdout, "    - %s\n", target.Branch)
	}
}

func printRun(stdout io.Writer, run state.Run) {
	fmt.Fprintf(stdout, "Run %s\n", run.ID)
	for _, target := range run.Targets {
		fmt.Fprintf(stdout, "  %-10s %s\n", target.Status, target.Branch)
	}
}

func renderActiveRun(stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	run, err := ctx.store.Load()
	if errors.Is(err, os.ErrNotExist) {
		tasks := taskItemsFromConfig(ctx.config)
		if interactiveOutput(stdout) {
			if err := tui.RunTasksWithHandler(tasks, taskActionHandler(ctx, tasks)); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
		fmt.Fprint(stdout, tui.NewTaskModel(tasks).View().Content)
		return 0
	}
	if err != nil {
		printStateResetHint(stdout, []string{"state is corrupted"})
		return 0
	}
	if reasons := validateActiveRun(ctx, run); len(reasons) > 0 {
		printStateResetHint(stdout, reasons)
		return 0
	}
	if interactiveOutput(stdout) {
		if err := tui.RunWithHandler(run, tuiActionHandler(ctx)); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, tui.NewModel(run).View().Content)
	return 0
}

func taskItemsFromConfig(cfg config.Config) []tui.TaskItem {
	names := make([]string, 0, len(cfg.Tasks))
	for name := range cfg.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]tui.TaskItem, 0, len(names))
	for _, name := range names {
		task := cfg.Tasks[name]
		mode := task.Mode
		if mode == "" {
			mode = cfg.Defaults.Mode
		}
		items = append(items, tui.TaskItem{
			Name:    name,
			Kind:    task.Type,
			Source:  task.From,
			Targets: append([]string(nil), task.To...),
			Mode:    mode,
		})
	}
	return items
}

func taskActionHandler(ctx repoContext, tasks []tui.TaskItem) tui.ActionHandler {
	return func(action tui.Action, targetIndex int) (state.Run, string, error) {
		if targetIndex < 0 || targetIndex >= len(tasks) {
			return state.Run{}, "", fmt.Errorf("selected task is outside task list")
		}
		task := tasks[targetIndex]
		switch action {
		case tui.ActionRunTask:
			_, run, err := prepareAndMaybeExecute(ctx, spread.CLIInput{Task: task.Name}, false)
			if err != nil {
				return run, "", err
			}
			return run, "started task " + task.Name, nil
		case tui.ActionPlanTask:
			plan, _, err := prepareAndMaybeExecute(ctx, spread.CLIInput{Task: task.Name}, true)
			if err != nil {
				return state.Run{}, "", err
			}
			return state.Run{}, planText(plan), nil
		default:
			return state.Run{}, "", nil
		}
	}
}

func planText(plan spread.Plan) string {
	var b strings.Builder
	printPlan(&b, plan)
	return strings.TrimSpace(b.String())
}

func tuiActionHandler(ctx repoContext) tui.ActionHandler {
	return func(action tui.Action, targetIndex int) (state.Run, string, error) {
		switch action {
		case tui.ActionRefresh:
			run, err := ctx.store.Load()
			if errors.Is(err, os.ErrNotExist) {
				return state.Run{}, "No active run. Press q to quit or restart git-spread.", nil
			}
			return run, "refreshed", err
		case tui.ActionOpenWorkspace:
			run, err := ctx.store.Load()
			if err != nil {
				return run, "", err
			}
			if err := openTargetWorkspace(ctx, run, targetIndex, ctx.config.Defaults.Editor); err != nil {
				return run, "", err
			}
			return run, "opened workspace in editor", nil
		case tui.ActionContinue:
			run, err := continueActiveRun(ctx)
			return run, "continued run", err
		case tui.ActionAbort:
			run, err := ctx.store.Load()
			if errors.Is(err, os.ErrNotExist) {
				return state.Run{}, "No active run. Press q to quit or restart git-spread.", nil
			}
			if err != nil {
				return run, "", err
			}
			if err := spread.Abort(ctx.store); err != nil {
				return run, "", err
			}
			return state.Run{}, "Aborted active run. Press q to quit or restart git-spread.", nil
		case tui.ActionReset:
			if err := spread.Abort(ctx.store); err != nil {
				return state.Run{}, "", err
			}
			return state.Run{}, "Reset Git Spread state. Press q to quit or restart git-spread.", nil
		case tui.ActionSwitchToPR:
			run, err := ctx.store.Load()
			return run, "switch to PR mode from TUI is not wired yet; run the command again with --mode pr", err
		default:
			run, err := ctx.store.Load()
			return run, "", err
		}
	}
}

func interactiveOutput(stdout io.Writer) bool {
	file, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func printStatus(stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	run, err := ctx.store.Load()
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stdout, "No active Git Spread run.")
		return 0
	}
	if err != nil {
		printStateResetHint(stdout, []string{"state is corrupted"})
		return 0
	}
	if reasons := validateActiveRun(ctx, run); len(reasons) > 0 {
		printStateResetHint(stdout, reasons)
		return 0
	}
	printRun(stdout, run)
	return 0
}

func continueRun(stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	run, err := continueActiveRun(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRun(stdout, run)
	return 0
}

func continueActiveRun(ctx repoContext) (state.Run, error) {
	active, err := ctx.store.Load()
	if err != nil {
		return state.Run{}, err
	}
	var run state.Run
	if active.Mode == string(spread.ModePR) {
		req := requestForContinuingPR(active)
		client, err := prepareGitHub(&req, ctx)
		if err != nil {
			return state.Run{}, err
		}
		run, err = spread.ContinueWithGitHub(ctx.git, ctx.store, client)
	} else {
		run, err = spread.Continue(ctx.git, ctx.store)
	}
	return run, err
}

func requestForContinuingPR(run state.Run) spread.Request {
	forkRemote := run.ForkRemote
	if forkRemote == "" {
		forkRemote = "fork"
	}
	collaboration := run.Collaboration
	if collaboration == "" {
		collaboration = "auto"
	}
	return spread.Request{
		Mode:          spread.ModePR,
		Remote:        run.Remote,
		Collaboration: collaboration,
		ForkRemote:    forkRemote,
		HeadRemote:    run.HeadRemote,
		HeadOwner:     run.HeadOwner,
	}
}

func abortRun(stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := spread.Abort(ctx.store); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "aborted active Git Spread run")
	return 0
}

func resetRun(stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := spread.Abort(ctx.store); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "reset Git Spread state")
	return 0
}

func validateActiveRun(ctx repoContext, run state.Run) []string {
	var reasons []string
	if len(run.Targets) == 0 {
		reasons = append(reasons, "run has no targets")
	}
	if run.CurrentTarget < 0 || run.CurrentTarget >= len(run.Targets) {
		reasons = append(reasons, "current target is outside target list")
	}
	for _, target := range run.Targets {
		if !knownStatus(target.Status) {
			reasons = append(reasons, fmt.Sprintf("unknown target status %q on %s", target.Status, branchOrDash(target.Branch)))
		}
		if target.WorkspacePath != "" && target.Status != state.StatusPending && target.Status != state.StatusDone {
			if _, err := os.Stat(filepath.Join(ctx.root, target.WorkspacePath)); errors.Is(err, os.ErrNotExist) {
				reasons = append(reasons, "workspace is missing: "+target.WorkspacePath)
			}
		}
	}
	return reasons
}

func knownStatus(status state.Status) bool {
	switch status {
	case state.StatusPending, state.StatusRunning, state.StatusDone, state.StatusConflict, state.StatusBlocked, state.StatusRejected, state.StatusFailed:
		return true
	default:
		return false
	}
}

func printStateResetHint(stdout io.Writer, reasons []string) {
	fmt.Fprintln(stdout, "State needs reset")
	for _, reason := range reasons {
		fmt.Fprintf(stdout, "  - %s\n", reason)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Run:")
	fmt.Fprintln(stdout, "  git-spread reset")
}

func branchOrDash(branch string) string {
	if branch == "" {
		return "-"
	}
	return branch
}

func openCurrent(cmd openCmd, stdout io.Writer, stderr io.Writer) int {
	if cmd.Print {
		return printCurrentOpenCommand(cmd, stdout, stderr)
	}
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	run, err := ctx.store.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := openTargetWorkspace(ctx, run, run.CurrentTarget, cmd.Editor); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func openTargetWorkspace(ctx repoContext, run state.Run, targetIndex int, editorName string) error {
	if targetIndex < 0 || targetIndex >= len(run.Targets) {
		return fmt.Errorf("current target is outside target list")
	}
	workspace := filepath.Join(ctx.root, run.Targets[targetIndex].WorkspacePath)
	editorCmd, args, err := editor.Command(editorName, workspace)
	if err != nil {
		return err
	}
	return exec.Command(editorCmd, args...).Start()
}

func printCurrentOpenCommand(cmd openCmd, stdout io.Writer, stderr io.Writer) int {
	ctx, err := loadRepoContext()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	run, err := ctx.store.Load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if run.CurrentTarget < 0 || run.CurrentTarget >= len(run.Targets) {
		fmt.Fprintln(stderr, "current target is outside target list")
		return 1
	}
	workspace := filepath.Join(ctx.root, run.Targets[run.CurrentTarget].WorkspacePath)
	editorCmd, args, err := editor.Command(cmd.Editor, workspace)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s %s\n", editorCmd, strings.Join(args, " "))
	return 0
}
