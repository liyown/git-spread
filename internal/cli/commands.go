package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

const Version = "dev"

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
	input.Config = ctx.config
	input.CurrentBranch = ctx.currentBranch

	req, err := spread.Normalize(input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var client ghclient.Client
	if needsGitHubClient(req, planOnly) {
		client, err = prepareGitHub(&req, ctx)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	plan, err := buildPlan(req, ctx.git, client)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if planOnly {
		printPlan(stdout, plan)
		return 0
	}
	run, err := executePlan(plan, ctx.git, ctx.store, client)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRun(stdout, run)
	return 0
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
	root := strings.TrimSpace(rootOut)
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
		fmt.Fprintln(stdout, "No active Git Spread run.")
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if interactiveOutput(stdout) {
		if err := tui.Run(run); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stdout, tui.NewModel(run).View().Content)
	return 0
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
		fmt.Fprintln(stderr, err)
		return 1
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
	run, err := spread.Continue(ctx.git, ctx.store)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printRun(stdout, run)
	return 0
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

func openCurrent(cmd openCmd, stdout io.Writer, stderr io.Writer) int {
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
	if cmd.Print {
		fmt.Fprintf(stdout, "%s %s\n", editorCmd, strings.Join(args, " "))
		return 0
	}
	if err := exec.Command(editorCmd, args...).Start(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
