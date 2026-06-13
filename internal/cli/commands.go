package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/liyown/git-spread/internal/spread"
)

const Version = "dev"

type app struct {
	Init   initCmd   `cmd:"" help:"Create a .git-spread.yml config."`
	Plan   planCmd   `cmd:"" help:"Show what Git Spread would do."`
	Branch branchCmd `cmd:"" help:"Propagate a branch."`
	Commit commitCmd `cmd:"" help:"Propagate explicit commits or ranges."`
	PR     prCmd     `cmd:"pr" help:"Propagate a pull request."`
	NoTUI  bool      `help:"Disable interactive TUI."`
}

type initCmd struct {
	Print bool `help:"Print config template instead of writing a file."`
}

type planCmd struct {
	Branch branchCmd `cmd:"" help:"Plan branch propagation."`
	Commit commitCmd `cmd:"" help:"Plan commit propagation."`
	PR     prCmd     `cmd:"pr" help:"Plan pull request propagation."`
	NoTUI  bool      `help:"Disable interactive TUI."`
}

type branchCmd struct {
	Source string   `arg:"" optional:"" help:"Source branch. Defaults to current branch."`
	To     []string `required:"" sep:"," help:"Target branches or patterns."`
	Mode   string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
	NoTUI  bool     `help:"Disable interactive TUI."`
}

type commitCmd struct {
	Items []string `arg:"" optional:"" help:"Commit SHAs or ranges."`
	To    []string `required:"" sep:"," help:"Target branches or patterns."`
	Mode  string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
	NoTUI bool     `help:"Disable interactive TUI."`
}

type prCmd struct {
	Item  string   `arg:"" optional:"" help:"Pull request number or URL."`
	To    []string `required:"" sep:"," help:"Target branches or patterns."`
	Mode  string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
	NoTUI bool     `help:"Disable interactive TUI."`
}

const configTemplate = `version: 1

defaults:
  mode: direct
  remote: origin
  workspace: isolated
  workspaceDir: .spread
  editor: auto
  github:
    collaboration: auto

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
	if strings.TrimSpace(ctx.Command()) == "init" {
		fmt.Fprint(stdout, configTemplate)
		return 0
	}

	input, planOnly, handled, err := inputFromContext(ctx.Command(), cli)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if handled {
		return 0
	}
	if planOnly {
		fmt.Fprintf(stdout, "Plan\n  kind: %s\n  targets: %v\n", input.Kind, input.Targets)
		return 0
	}
	if _, err := spread.Normalize(input); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func inputFromContext(command string, cli app) (spread.CLIInput, bool, bool, error) {
	command = strings.TrimSpace(command)
	switch {
	case command == "init":
		return spread.CLIInput{}, false, true, nil
	case strings.HasPrefix(command, "plan branch"):
		return branchInput(cli.Plan.Branch), true, false, nil
	case strings.HasPrefix(command, "plan commit"):
		return commitInput(cli.Plan.Commit), true, false, nil
	case strings.HasPrefix(command, "plan pr"):
		return prInput(cli.Plan.PR), true, false, nil
	case strings.HasPrefix(command, "branch"):
		return branchInput(cli.Branch), false, false, nil
	case strings.HasPrefix(command, "commit"):
		return commitInput(cli.Commit), false, false, nil
	case strings.HasPrefix(command, "pr"):
		return prInput(cli.PR), false, false, nil
	default:
		return spread.CLIInput{}, false, false, fmt.Errorf("unsupported command %q", command)
	}
}

func branchInput(cmd branchCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindBranch, Source: cmd.Source, Targets: cmd.To, Mode: cmd.Mode}
}

func commitInput(cmd commitCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindCommit, Items: cmd.Items, Targets: cmd.To, Mode: cmd.Mode}
}

func prInput(cmd prCmd) spread.CLIInput {
	items := []string(nil)
	if cmd.Item != "" {
		items = []string{cmd.Item}
	}
	return spread.CLIInput{Kind: spread.KindPR, Items: items, Targets: cmd.To, Mode: cmd.Mode}
}
