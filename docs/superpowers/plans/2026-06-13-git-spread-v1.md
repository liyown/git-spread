# Git Spread v1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working Git Spread CLI with typed config, scriptable commands, isolated Git workspaces, conflict state, editor handoff, a Bubble Tea TUI, and mocked-testable GitHub PR integration.

**Architecture:** Use a small Go module with a thin `cmd/git-spread` entrypoint, typed command parsing in `internal/cli`, domain planning/execution in `internal/spread`, real Git command execution behind `internal/git`, persisted run state in `internal/state`, GitHub calls behind `internal/github`, and UI adapters in `internal/tui` and `internal/editor`. Keep CLI/TUI as adapters over the same runtime services so script and human workflows share behavior.

**Tech Stack:** Go 1.26, Kong, Bubble Tea v2, Bubbles v2, Lip Gloss v2, Huh v2, go.yaml.in/yaml/v3, go-gh v2, standard `testing`, go-cmp, real temporary Git repositories for integration tests.

---

## Scope Notes

The spec has several subsystems, but they are coupled around one propagation runtime. This is one implementation plan split into testable tasks. Each task should be committed before the next task starts.

Do not implement release train approvals, hosted state, AI conflict resolution, semantic branch safety checks, or a custom merge editor.

## File Structure

Create this structure:

```text
cmd/git-spread/main.go
internal/cli/commands.go
internal/cli/commands_test.go
internal/config/config.go
internal/config/config_test.go
internal/editor/editor.go
internal/editor/editor_test.go
internal/git/runner.go
internal/git/runner_test.go
internal/github/client.go
internal/github/client_test.go
internal/spread/types.go
internal/spread/normalize.go
internal/spread/normalize_test.go
internal/spread/planner.go
internal/spread/planner_test.go
internal/spread/executor.go
internal/spread/executor_branch_test.go
internal/spread/executor_commit_test.go
internal/spread/executor_pr_test.go
internal/spread/continue.go
internal/spread/continue_test.go
internal/state/store.go
internal/state/store_test.go
internal/tui/model.go
internal/tui/model_test.go
internal/testutil/gitrepo.go
internal/testutil/gitrepo_test.go
go.mod
go.sum
README.md
.gitignore
```

Responsibilities:

- `cmd/git-spread/main.go`: calls `cli.Run(os.Args[1:])`.
- `internal/cli`: Kong command grammar, command dispatch, scriptable output mode, TUI launch decision.
- `internal/config`: `.git-spread.yml` schema, defaults, file loading.
- `internal/spread`: normalized request types, planning, target resolution, execution orchestration, continue/abort behavior.
- `internal/git`: system `git` wrapper and repository status parsing.
- `internal/state`: JSON state store under `.git/spread/state.json`.
- `internal/tui`: Bubble Tea model, key bindings, view rendering, and messages.
- `internal/editor`: editor auto-detection and opening isolated workspaces.
- `internal/github`: go-gh backed client plus interface for tests.
- `internal/testutil`: helpers for temporary real Git repositories.

## Task 1: Module Scaffold and Dependency Pinning

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `cmd/git-spread/main.go`
- Create: `internal/cli/commands.go`
- Create: `internal/cli/commands_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing CLI smoke test**

Create `internal/cli/commands_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stdout.String() != "git-spread dev\n" {
		t.Fatalf("stdout = %q, want version line", stdout.String())
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/cli -run TestRunVersion -count=1
```

Expected: fails because the module and `Run` do not exist.

- [ ] **Step 3: Create the module and minimal CLI**

Create `go.mod`:

```go
module github.com/liyown/git-spread

go 1.26

toolchain go1.26.4

require (
	charm.land/bubbles/v2 v2.1.0
	charm.land/bubbletea/v2 v2.0.7
	charm.land/huh/v2 v2.0.3
	charm.land/lipgloss/v2 v2.0.4
	github.com/alecthomas/kong v1.15.0
	github.com/cli/go-gh/v2 v2.13.0
	github.com/google/go-cmp v0.7.0
	go.yaml.in/yaml/v3 v3.0.4
)
```

Create `internal/cli/commands.go`:

```go
package cli

import (
	"fmt"
	"io"
)

const Version = "dev"

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "git-spread %s\n", Version)
		return 0
	}
	fmt.Fprintln(stderr, "git-spread: command parser is not initialized")
	return 2
}
```

Create `cmd/git-spread/main.go`:

```go
package main

import (
	"os"

	"github.com/liyown/git-spread/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

Create `.gitignore`:

```gitignore
.spread/
.superpowers/
coverage.out
git-spread
```

Create `README.md`:

```markdown
# Git Spread

Git Spread propagates branch, commit, and pull request changes across Git branches.

The design spec lives in `docs/superpowers/specs/2026-06-13-git-spread-design.md`.
```

- [ ] **Step 4: Resolve modules and run the smoke test**

Run:

```bash
go mod tidy
go test ./internal/cli -run TestRunVersion -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum .gitignore README.md cmd/git-spread/main.go internal/cli/commands.go internal/cli/commands_test.go
git commit -m "chore: scaffold go cli"
```

## Task 2: Typed Config Loading

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write config tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte("version: 1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Defaults.Mode != "direct" {
		t.Fatalf("mode = %q, want direct", cfg.Defaults.Mode)
	}
	if cfg.Defaults.Remote != "origin" {
		t.Fatalf("remote = %q, want origin", cfg.Defaults.Remote)
	}
	if cfg.Defaults.Workspace != "isolated" {
		t.Fatalf("workspace = %q, want isolated", cfg.Defaults.Workspace)
	}
	if cfg.Defaults.WorkspaceDir != ".spread" {
		t.Fatalf("workspace dir = %q, want .spread", cfg.Defaults.WorkspaceDir)
	}
	if cfg.Defaults.Editor != "auto" {
		t.Fatalf("editor = %q, want auto", cfg.Defaults.Editor)
	}
	if cfg.Defaults.GitHub.Collaboration != "auto" {
		t.Fatalf("collaboration = %q, want auto", cfg.Defaults.GitHub.Collaboration)
	}
}

func TestLoadTasks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte(`
version: 1
tasks:
  release:
    type: branch
    from: develop
    to:
      - release/*
      - main
  backport:
    type: commit
    to:
      - release/*
    mode: pr
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	release := cfg.Tasks["release"]
	if release.Type != "branch" || release.From != "develop" || len(release.To) != 2 {
		t.Fatalf("release task = %#v", release)
	}
	backport := cfg.Tasks["backport"]
	if backport.Type != "commit" || backport.Mode != "pr" || len(backport.To) != 1 {
		t.Fatalf("backport task = %#v", backport)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/config -count=1
```

Expected: fails because config package is missing.

- [ ] **Step 3: Implement typed config**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Version  int             `yaml:"version"`
	Defaults Defaults       `yaml:"defaults"`
	Tasks   map[string]Task `yaml:"tasks"`
}

type Defaults struct {
	Mode         string         `yaml:"mode"`
	Remote       string         `yaml:"remote"`
	Workspace    string         `yaml:"workspace"`
	WorkspaceDir string         `yaml:"workspaceDir"`
	Editor       string         `yaml:"editor"`
	GitHub       GitHubDefaults `yaml:"github"`
}

type GitHubDefaults struct {
	Collaboration string `yaml:"collaboration"`
}

type Task struct {
	Type string   `yaml:"type"`
	From string   `yaml:"from"`
	To   []string `yaml:"to"`
	Mode string   `yaml:"mode"`
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func ApplyDefaults(cfg *Config) {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Defaults.Mode == "" {
		cfg.Defaults.Mode = "direct"
	}
	if cfg.Defaults.Remote == "" {
		cfg.Defaults.Remote = "origin"
	}
	if cfg.Defaults.Workspace == "" {
		cfg.Defaults.Workspace = "isolated"
	}
	if cfg.Defaults.WorkspaceDir == "" {
		cfg.Defaults.WorkspaceDir = ".spread"
	}
	if cfg.Defaults.Editor == "" {
		cfg.Defaults.Editor = "auto"
	}
	if cfg.Defaults.GitHub.Collaboration == "" {
		cfg.Defaults.GitHub.Collaboration = "auto"
	}
	if cfg.Tasks == nil {
		cfg.Tasks = map[string]Task{}
	}
}
```

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/config -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go go.mod go.sum
git commit -m "feat: load git spread config"
```

## Task 3: Domain Types and Request Normalization

**Files:**
- Create: `internal/spread/types.go`
- Create: `internal/spread/normalize.go`
- Create: `internal/spread/normalize_test.go`

- [ ] **Step 1: Write normalization tests**

Create `internal/spread/normalize_test.go`:

```go
package spread

import (
	"testing"

	"github.com/liyown/git-spread/internal/config"
)

func TestNormalizeBranchUsesCurrentBranch(t *testing.T) {
	input := CLIInput{
		Kind:          KindBranch,
		Targets:       []string{"release/1.0"},
		CurrentBranch: "feature/login-fix",
		Config:        config.Config{},
	}

	req, err := Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	if req.Source != "feature/login-fix" {
		t.Fatalf("source = %q, want current branch", req.Source)
	}
	if req.Mode != ModeDirect {
		t.Fatalf("mode = %q, want direct", req.Mode)
	}
}

func TestNormalizeCommitRequiresInput(t *testing.T) {
	_, err := Normalize(CLIInput{Kind: KindCommit, Targets: []string{"release/1.0"}})
	if err == nil {
		t.Fatal("expected error for missing commit input")
	}
}

func TestNormalizePRRequiresInput(t *testing.T) {
	_, err := Normalize(CLIInput{Kind: KindPR, Targets: []string{"release/1.0"}})
	if err == nil {
		t.Fatal("expected error for missing pull request input")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/spread -run Normalize -count=1
```

Expected: fails because spread package is missing.

- [ ] **Step 3: Implement domain types and normalization**

Create `internal/spread/types.go`:

```go
package spread

import "github.com/liyown/git-spread/internal/config"

type Kind string

const (
	KindBranch Kind = "branch"
	KindCommit Kind = "commit"
	KindPR     Kind = "pr"
)

type Mode string

const (
	ModeDirect Mode = "direct"
	ModePR     Mode = "pr"
)

type WorkspaceMode string

const (
	WorkspaceIsolated WorkspaceMode = "isolated"
	WorkspaceCurrent  WorkspaceMode = "current"
)

type CLIInput struct {
	Kind          Kind
	Source        string
	Items         []string
	Targets       []string
	Mode          string
	Task          string
	CurrentBranch string
	Config        config.Config
}

type Request struct {
	Kind          Kind
	Source        string
	Items         []string
	Targets       []string
	Mode          Mode
	Remote        string
	Workspace     WorkspaceMode
	WorkspaceDir  string
	Editor        string
	Collaboration string
}
```

Create `internal/spread/normalize.go`:

```go
package spread

import (
	"errors"
	"fmt"

	"github.com/liyown/git-spread/internal/config"
)

func Normalize(input CLIInput) (Request, error) {
	cfg := input.Config
	config.ApplyDefaults(&cfg)

	if input.Task != "" {
		task, ok := cfg.Tasks[input.Task]
		if !ok {
			return Request{}, fmt.Errorf("task %q not found", input.Task)
		}
		mergeTask(&input, task)
	}

	mode := cfg.Defaults.Mode
	if input.Mode != "" {
		mode = input.Mode
	}
	if mode != string(ModeDirect) && mode != string(ModePR) {
		return Request{}, fmt.Errorf("mode %q is invalid", mode)
	}
	if len(input.Targets) == 0 {
		return Request{}, errors.New("at least one target branch is required")
	}

	req := Request{
		Kind:          input.Kind,
		Source:        input.Source,
		Items:         append([]string(nil), input.Items...),
		Targets:       append([]string(nil), input.Targets...),
		Mode:          Mode(mode),
		Remote:        cfg.Defaults.Remote,
		Workspace:     WorkspaceMode(cfg.Defaults.Workspace),
		WorkspaceDir:  cfg.Defaults.WorkspaceDir,
		Editor:        cfg.Defaults.Editor,
		Collaboration: cfg.Defaults.GitHub.Collaboration,
	}

	switch req.Kind {
	case KindBranch:
		if req.Source == "" {
			req.Source = input.CurrentBranch
		}
		if req.Source == "" {
			return Request{}, errors.New("branch source is required when current branch cannot be detected")
		}
	case KindCommit:
		if len(req.Items) == 0 {
			return Request{}, errors.New("commit mode requires at least one commit or range")
		}
	case KindPR:
		if len(req.Items) != 1 {
			return Request{}, errors.New("pr mode requires exactly one pull request number or URL")
		}
	default:
		return Request{}, fmt.Errorf("propagation type %q is invalid", req.Kind)
	}

	return req, nil
}

func mergeTask(input *CLIInput, task config.Task) {
	if input.Kind == "" && task.Type != "" {
		input.Kind = Kind(task.Type)
	}
	if input.Source == "" {
		input.Source = task.From
	}
	if len(input.Targets) == 0 {
		input.Targets = append([]string(nil), task.To...)
	}
	if input.Mode == "" {
		input.Mode = task.Mode
	}
}
```

- [ ] **Step 4: Run normalization tests**

Run:

```bash
go test ./internal/spread -run Normalize -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/spread/types.go internal/spread/normalize.go internal/spread/normalize_test.go
git commit -m "feat: normalize propagation requests"
```

## Task 4: Git Runner and Temporary Repository Test Harness

**Files:**
- Create: `internal/git/runner.go`
- Create: `internal/git/runner_test.go`
- Create: `internal/testutil/gitrepo.go`
- Create: `internal/testutil/gitrepo_test.go`

- [ ] **Step 1: Write runner and test repository tests**

Create `internal/git/runner_test.go`:

```go
package git

import "testing"

func TestRunnerExecutesGitVersion(t *testing.T) {
	r := NewRunner("")
	out, err := r.Output("version")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("expected git version output")
	}
}
```

Create `internal/testutil/gitrepo_test.go`:

```go
package testutil

import "testing"

func TestRepoCreatesInitialBranch(t *testing.T) {
	repo := NewGitRepo(t)
	repo.Write("README.md", "hello\n")
	repo.Commit("initial")
	repo.Branch("develop")

	if got := repo.CurrentBranch(); got != "main" {
		t.Fatalf("current branch = %q, want main", got)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/git ./internal/testutil -count=1
```

Expected: fails because packages are missing.

- [ ] **Step 3: Implement Git runner**

Create `internal/git/runner.go`:

```go
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct {
	Dir string
}

func NewRunner(dir string) Runner {
	return Runner{Dir: dir}
}

func (r Runner) Run(args ...string) error {
	_, err := r.run(args...)
	return err
}

func (r Runner) Output(args ...string) (string, error) {
	return r.run(args...)
}

func (r Runner) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
```

Create `internal/testutil/gitrepo.go`:

```go
package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/git"
)

type GitRepo struct {
	t   *testing.T
	Dir string
	git git.Runner
}

func NewGitRepo(t *testing.T) *GitRepo {
	t.Helper()
	dir := t.TempDir()
	r := git.NewRunner(dir)
	run(t, r, "init", "-b", "main")
	run(t, r, "config", "user.email", "test@example.com")
	run(t, r, "config", "user.name", "Git Spread Test")
	return &GitRepo{t: t, Dir: dir, git: r}
}

func (r *GitRepo) Write(path string, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *GitRepo) Commit(message string) {
	r.t.Helper()
	run(r.t, r.git, "add", ".")
	run(r.t, r.git, "commit", "-m", message)
}

func (r *GitRepo) Branch(name string) {
	r.t.Helper()
	run(r.t, r.git, "branch", name)
}

func (r *GitRepo) Checkout(name string) {
	r.t.Helper()
	run(r.t, r.git, "checkout", name)
}

func (r *GitRepo) CurrentBranch() string {
	r.t.Helper()
	out, err := r.git.Output("branch", "--show-current")
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func run(t *testing.T, r git.Runner, args ...string) {
	t.Helper()
	if err := r.Run(args...); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/git ./internal/testutil -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/git/runner.go internal/git/runner_test.go internal/testutil/gitrepo.go internal/testutil/gitrepo_test.go
git commit -m "feat: add git runner test harness"
```

## Task 5: Planner for Targets and Commit Inputs

**Files:**
- Create: `internal/spread/planner.go`
- Create: `internal/spread/planner_test.go`
- Modify: `internal/git/runner.go`

- [ ] **Step 1: Write planner tests**

Create `internal/spread/planner_test.go`:

```go
package spread

import (
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestPlanResolvesTargetPattern(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Branch("release/1.1")
	repo.Branch("feature/login-fix")

	req := Request{
		Kind:    KindBranch,
		Source:  "feature/login-fix",
		Targets: []string{"release/*"},
		Mode:    ModeDirect,
		Remote:  ".",
	}

	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(plan.Targets); got != 2 {
		t.Fatalf("targets = %d, want 2", got)
	}
	if plan.Targets[0].Branch != "release/1.0" || plan.Targets[1].Branch != "release/1.1" {
		t.Fatalf("targets = %#v", plan.Targets)
	}
}

func TestPlanExpandsCommitRange(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Checkout("-b", "feature/login-fix")
	repo.Write("a.txt", "a\n")
	repo.Commit("add a")
	repo.Write("b.txt", "b\n")
	repo.Commit("add b")

	req := Request{
		Kind:    KindCommit,
		Items:   []string{"main..feature/login-fix"},
		Targets: []string{"main"},
		Mode:    ModeDirect,
		Remote:  ".",
	}

	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(plan.Commits); got != 2 {
		t.Fatalf("commits = %d, want 2", got)
	}
}
```

- [ ] **Step 2: Run planner tests to verify failure**

Run:

```bash
go test ./internal/spread -run Plan -count=1
```

Expected: fails because `BuildPlan` does not exist and `GitRepo.Checkout` does not accept variadic args.

- [ ] **Step 3: Update test helper checkout and implement planner**

Modify `internal/testutil/gitrepo.go` `Checkout` method:

```go
func (r *GitRepo) Checkout(args ...string) {
	r.t.Helper()
	run(r.t, r.git, append([]string{"checkout"}, args...)...)
}
```

Create `internal/spread/planner.go`:

```go
package spread

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/liyown/git-spread/internal/git"
)

type Plan struct {
	Request Request
	Targets []TargetPlan
	Commits []string
}

type TargetPlan struct {
	Branch        string
	WorkspacePath string
}

func BuildPlan(req Request, runner git.Runner) (Plan, error) {
	targets, err := resolveTargets(req.Targets, runner)
	if err != nil {
		return Plan{}, err
	}
	commits, err := resolveCommits(req, runner)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Request: req, Commits: commits}
	for _, target := range targets {
		plan.Targets = append(plan.Targets, TargetPlan{
			Branch:        target,
			WorkspacePath: filepath.Join(req.WorkspaceDir, sanitizeBranch(target)),
		})
	}
	return plan, nil
}

func resolveTargets(patterns []string, runner git.Runner) ([]string, error) {
	out, err := runner.Output("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	branches := strings.Fields(out)
	var targets []string
	for _, pattern := range patterns {
		if !strings.Contains(pattern, "*") {
			targets = append(targets, pattern)
			continue
		}
		for _, branch := range branches {
			ok, err := filepath.Match(pattern, branch)
			if err != nil {
				return nil, err
			}
			if ok {
				targets = append(targets, branch)
			}
		}
	}
	sort.Strings(targets)
	return targets, nil
}

func resolveCommits(req Request, runner git.Runner) ([]string, error) {
	if req.Kind != KindCommit {
		return nil, nil
	}
	var commits []string
	for _, item := range req.Items {
		if strings.Contains(item, "..") {
			out, err := runner.Output("rev-list", "--reverse", item)
			if err != nil {
				return nil, err
			}
			commits = append(commits, strings.Fields(out)...)
			continue
		}
		commits = append(commits, item)
	}
	return commits, nil
}

func sanitizeBranch(branch string) string {
	return strings.NewReplacer("/", "-", "\\", "-").Replace(branch)
}
```

- [ ] **Step 4: Run planner tests**

Run:

```bash
go test ./internal/spread ./internal/testutil -run 'Plan|Repo' -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/spread/planner.go internal/spread/planner_test.go internal/testutil/gitrepo.go
git commit -m "feat: build propagation plans"
```

## Task 6: Persistent Run State

**Files:**
- Create: `internal/state/store.go`
- Create: `internal/state/store_test.go`

- [ ] **Step 1: Write state store tests**

Create `internal/state/store_test.go`:

```go
package state

import "testing"

func TestStoreRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	run := Run{
		ID:   "run-1",
		Kind: "branch",
		Targets: []Target{
			{Branch: "release/1.0", Status: StatusDone},
			{Branch: "release/1.1", Status: StatusConflict, WorkspacePath: ".spread/release-1.1", ConflictedFiles: []string{"user.go"}},
		},
	}

	if err := store.Save(run); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "run-1" || got.Targets[1].ConflictedFiles[0] != "user.go" {
		t.Fatalf("loaded run = %#v", got)
	}
}
```

- [ ] **Step 2: Run state tests to verify failure**

Run:

```bash
go test ./internal/state -count=1
```

Expected: fails because state package is missing.

- [ ] **Step 3: Implement JSON state store**

Create `internal/state/store.go`:

```go
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusConflict Status = "conflict"
	StatusRejected Status = "rejected"
	StatusFailed   Status = "failed"
)

type Run struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Mode          string   `json:"mode"`
	Source        string   `json:"source,omitempty"`
	Items         []string `json:"items,omitempty"`
	Targets       []Target `json:"targets"`
	CurrentTarget int      `json:"currentTarget"`
}

type Target struct {
	Branch          string   `json:"branch"`
	Status          Status   `json:"status"`
	WorkspacePath   string   `json:"workspacePath"`
	ConflictedFiles []string `json:"conflictedFiles,omitempty"`
	CreatedBranch   string   `json:"createdBranch,omitempty"`
	PullRequestURL   string   `json:"pullRequestURL,omitempty"`
}

type Store struct {
	dir string
}

func NewStore(dir string) Store {
	return Store{dir: dir}
}

func (s Store) Path() string {
	return filepath.Join(s.dir, "state.json")
}

func (s Store) Save(run Run) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path(), append(data, '\n'), 0o644)
}

func (s Store) Load() (Run, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return Run{}, err
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s Store) Clear() error {
	err := os.Remove(s.Path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Run state tests**

Run:

```bash
go test ./internal/state -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/state/store.go internal/state/store_test.go
git commit -m "feat: persist spread run state"
```

## Task 7: Isolated Workspace Manager and Branch Direct Propagation

**Files:**
- Create: `internal/spread/executor.go`
- Create: `internal/spread/executor_branch_test.go`
- Modify: `internal/testutil/gitrepo.go`

- [ ] **Step 1: Write branch propagation integration test**

Create `internal/spread/executor_branch_test.go`:

```go
package spread

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestExecuteBranchDirectMergesIntoTargetWorkspace(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")

	req := Request{
		Kind:         KindBranch,
		Source:       "develop",
		Targets:      []string{"release/1.0"},
		Mode:         ModeDirect,
		Remote:       ".",
		Workspace:    WorkspaceIsolated,
		WorkspaceDir: ".spread",
	}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	result, err := Execute(plan, git.NewRunner(repo.Dir), store)
	if err != nil {
		t.Fatal(err)
	}
	if result.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", result.Targets[0].Status)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".spread", "release-1.0", "feature.txt")); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./internal/spread -run TestExecuteBranchDirect -count=1
```

Expected: fails because `Execute` does not exist.

- [ ] **Step 3: Implement branch direct execution**

Create `internal/spread/executor.go`:

```go
package spread

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
)

func Execute(plan Plan, root git.Runner, store state.Store) (state.Run, error) {
	run := state.Run{
		ID:     time.Now().UTC().Format("20060102T150405Z"),
		Kind:   string(plan.Request.Kind),
		Mode:   string(plan.Request.Mode),
		Source: plan.Request.Source,
		Items:  plan.Request.Items,
	}
	for _, target := range plan.Targets {
		run.Targets = append(run.Targets, state.Target{
			Branch:        target.Branch,
			Status:        state.StatusPending,
			WorkspacePath: target.WorkspacePath,
		})
	}
	if err := store.Save(run); err != nil {
		return run, err
	}
	for i, target := range plan.Targets {
		run.CurrentTarget = i
		run.Targets[i].Status = state.StatusRunning
		if err := store.Save(run); err != nil {
			return run, err
		}
		if err := executeTarget(plan, target, root); err != nil {
			conflicts, conflictErr := conflictedFiles(git.NewRunner(filepath.Join(root.Dir, target.WorkspacePath)))
			if conflictErr == nil && len(conflicts) > 0 {
				run.Targets[i].Status = state.StatusConflict
				run.Targets[i].ConflictedFiles = conflicts
				_ = store.Save(run)
				return run, nil
			}
			run.Targets[i].Status = state.StatusFailed
			_ = store.Save(run)
			return run, err
		}
		run.Targets[i].Status = state.StatusDone
		if err := store.Save(run); err != nil {
			return run, err
		}
	}
	return run, nil
}

func executeTarget(plan Plan, target TargetPlan, root git.Runner) error {
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	if err := root.Run("worktree", "add", "-B", target.Branch, workspace, target.Branch); err != nil {
		return err
	}
	w := git.NewRunner(workspace)
	switch plan.Request.Kind {
	case KindBranch:
		if err := w.Run("merge", "--no-edit", plan.Request.Source); err != nil {
			return err
		}
	case KindCommit:
		args := append([]string{"cherry-pick"}, plan.Commits...)
		if err := w.Run(args...); err != nil {
			return err
		}
	default:
		return fmt.Errorf("execution for %s is not available", plan.Request.Kind)
	}
	return nil
}

func conflictedFiles(r git.Runner) ([]string, error) {
	out, err := r.Output("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}
```

Add missing import to `internal/spread/executor.go`:

```go
import "strings"
```

- [ ] **Step 4: Run branch propagation test**

Run:

```bash
go test ./internal/spread -run TestExecuteBranchDirect -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/spread/executor.go internal/spread/executor_branch_test.go
git commit -m "feat: execute branch propagation"
```

## Task 8: Commit Propagation and Conflict Pause

**Files:**
- Create: `internal/spread/executor_commit_test.go`
- Modify: `internal/spread/executor.go`

- [ ] **Step 1: Write commit propagation and conflict tests**

Create `internal/spread/executor_commit_test.go`:

```go
package spread

import (
	"path/filepath"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestExecuteCommitDirectCherryPicksCommit(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "feature/login-fix")
	repo.Write("fix.txt", "fix\n")
	repo.Commit("fix login")
	commit := repo.Head()

	req := Request{Kind: KindCommit, Items: []string{commit}, Targets: []string{"release/1.0"}, Mode: ModeDirect, Remote: ".", Workspace: WorkspaceIsolated, WorkspaceDir: ".spread"}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	run, err := Execute(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")))
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", run.Targets[0].Status)
	}
}

func TestExecuteCommitConflictPausesRun(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("app.txt", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "feature/login-fix")
	repo.Write("app.txt", "feature\n")
	repo.Commit("feature edit")
	commit := repo.Head()
	repo.Checkout("release/1.0")
	repo.Write("app.txt", "release\n")
	repo.Commit("release edit")

	req := Request{Kind: KindCommit, Items: []string{commit}, Targets: []string{"release/1.0"}, Mode: ModeDirect, Remote: ".", Workspace: WorkspaceIsolated, WorkspaceDir: ".spread"}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	run, err := Execute(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")))
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusConflict {
		t.Fatalf("status = %q, want conflict", run.Targets[0].Status)
	}
	if len(run.Targets[0].ConflictedFiles) != 1 || run.Targets[0].ConflictedFiles[0] != "app.txt" {
		t.Fatalf("conflicted files = %#v", run.Targets[0].ConflictedFiles)
	}
}
```

- [ ] **Step 2: Run tests before adding `Head` helper**

Run:

```bash
go test ./internal/spread -run 'TestExecuteCommit' -count=1
```

Expected: fails because `GitRepo.Head` is not implemented.

- [ ] **Step 3: Add the `Head` helper**

Modify `internal/testutil/gitrepo.go`:

```go
func (r *GitRepo) Head() string {
	r.t.Helper()
	out, err := r.git.Output("rev-parse", "HEAD")
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.TrimSpace(out)
}
```

- [ ] **Step 4: Verify execution conflict detection**

Modify `internal/spread/executor.go` so the import block includes `strings`:

```go
import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
)
```

The cherry-pick execution block remains:

```go
args := append([]string{"cherry-pick"}, plan.Commits...)
if err := w.Run(args...); err != nil {
	return err
}
```

- [ ] **Step 5: Run commit execution tests**

Run:

```bash
go test ./internal/spread ./internal/testutil -run 'TestExecuteCommit|TestRepo' -count=1
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add internal/spread/executor.go internal/spread/executor_commit_test.go internal/testutil/gitrepo.go
git commit -m "feat: execute commit propagation"
```

## Task 9: Continue and Abort

**Files:**
- Create: `internal/spread/continue.go`
- Create: `internal/spread/continue_test.go`
- Modify: `internal/state/store.go`

- [ ] **Step 1: Write continue tests**

Create `internal/spread/continue_test.go`:

```go
package spread

import (
	"path/filepath"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestContinueAfterUserCommittedMarksTargetDone(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("app.txt", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")

	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	workspace := filepath.Join(repo.Dir, ".spread", "release-1.0")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}
	w := git.NewRunner(workspace)
	if err := w.Run("commit", "--allow-empty", "-m", "manual resolution"); err != nil {
		t.Fatal(err)
	}

	run := state.Run{ID: "run-1", Kind: "commit", CurrentTarget: 0, Targets: []state.Target{{Branch: "release/1.0", Status: state.StatusConflict, WorkspacePath: ".spread/release-1.0"}}}
	if err := store.Save(run); err != nil {
		t.Fatal(err)
	}

	updated, err := Continue(git.NewRunner(repo.Dir), store)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", updated.Targets[0].Status)
	}
}
```

- [ ] **Step 2: Run continue tests to verify failure**

Run:

```bash
go test ./internal/spread -run TestContinue -count=1
```

Expected: fails because `Continue` does not exist.

- [ ] **Step 3: Implement continue and abort**

Create `internal/spread/continue.go`:

```go
package spread

import (
	"errors"
	"path/filepath"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
)

func Continue(root git.Runner, store state.Store) (state.Run, error) {
	run, err := store.Load()
	if err != nil {
		return state.Run{}, err
	}
	if run.CurrentTarget < 0 || run.CurrentTarget >= len(run.Targets) {
		return run, errors.New("current target is outside target list")
	}
	target := &run.Targets[run.CurrentTarget]
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	w := git.NewRunner(workspace)
	conflicts, err := conflictedFiles(w)
	if err != nil {
		return run, err
	}
	if len(conflicts) > 0 {
		target.ConflictedFiles = conflicts
		target.Status = state.StatusConflict
		return run, store.Save(run)
	}
	clean, err := workspaceClean(w)
	if err != nil {
		return run, err
	}
	if !clean {
		if err := finishInProgressOperation(w); err != nil {
			return run, err
		}
	}
	target.Status = state.StatusDone
	if err := store.Save(run); err != nil {
		return run, err
	}
	return run, nil
}

func Abort(store state.Store) error {
	return store.Clear()
}

func workspaceClean(r git.Runner) (bool, error) {
	out, err := r.Output("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

func finishInProgressOperation(r git.Runner) error {
	if err := r.Run("cherry-pick", "--continue"); err == nil {
		return nil
	}
	if err := r.Run("merge", "--continue"); err == nil {
		return nil
	}
	return nil
}
```

- [ ] **Step 4: Run continue tests**

Run:

```bash
go test ./internal/spread -run TestContinue -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/spread/continue.go internal/spread/continue_test.go
git commit -m "feat: continue paused propagation"
```

## Task 10: CLI Command Grammar and Dispatch

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`

- [ ] **Step 1: Write CLI parsing tests**

Replace `internal/cli/commands_test.go` with tests for `--version`, branch current source, commit missing input, and plan mode:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 || stdout.String() != "git-spread dev\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCommitRequiresInput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"commit", "--to", "release/1.0", "--no-tui"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "commit mode requires") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPlanPrintsDryRunHeader(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"plan", "branch", "develop", "--to", "main", "--no-tui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Plan") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
```

- [ ] **Step 2: Run CLI tests to verify failure**

Run:

```bash
go test ./internal/cli -count=1
```

Expected: command parsing tests fail.

- [ ] **Step 3: Implement Kong command grammar and dispatch shell**

Modify `internal/cli/commands.go`:

```go
package cli

import (
	"fmt"
	"io"

	"github.com/alecthomas/kong"
	"github.com/liyown/git-spread/internal/spread"
)

const Version = "dev"

type app struct {
	Version versionCmd `cmd:"" help:"Print version."`
	Plan    planCmd    `cmd:"" help:"Show what Git Spread would do."`
	Branch  branchCmd  `cmd:"" help:"Propagate a branch."`
	Commit  commitCmd  `cmd:"" help:"Propagate explicit commits or ranges."`
	PR      prCmd      `cmd:"pr" help:"Propagate a pull request."`
	NoTUI   bool       `help:"Disable interactive TUI."`
}

type versionCmd struct{}
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
}
type commitCmd struct {
	Items []string `arg:"" help:"Commit SHAs or ranges."`
	To    []string `required:"" sep:"," help:"Target branches or patterns."`
	Mode  string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
}
type prCmd struct {
	Item string   `arg:"" help:"Pull request number or URL."`
	To   []string `required:"" sep:"," help:"Target branches or patterns."`
	Mode string   `enum:"direct,pr" default:"direct" help:"Execution mode."`
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "git-spread %s\n", Version)
		return 0
	}
	var cli app
	ctx := kong.Parse(&cli, kong.Name("git spread"), kong.Exit(func(int) {}), kong.Writers(stdout, stderr), kong.Args(args))
	input, planOnly, err := inputFromContext(ctx.Command(), cli)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if planOnly {
		fmt.Fprintf(stdout, "Plan\n  kind: %s\n  targets: %v\n", input.Kind, input.Targets)
		return 0
	}
	_, err = spread.Normalize(input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func inputFromContext(command string, cli app) (spread.CLIInput, bool, error) {
	switch command {
	case "plan branch <source>":
		return branchInput(cli.Plan.Branch), true, nil
	case "plan commit <items>":
		return commitInput(cli.Plan.Commit), true, nil
	case "plan pr <item>":
		return prInput(cli.Plan.PR), true, nil
	case "branch <source>":
		return branchInput(cli.Branch), false, nil
	case "commit <items>":
		return commitInput(cli.Commit), false, nil
	case "pr <item>":
		return prInput(cli.PR), false, nil
	default:
		return spread.CLIInput{}, false, fmt.Errorf("unsupported command %q", command)
	}
}

func branchInput(cmd branchCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindBranch, Source: cmd.Source, Targets: cmd.To, Mode: cmd.Mode}
}

func commitInput(cmd commitCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindCommit, Items: cmd.Items, Targets: cmd.To, Mode: cmd.Mode}
}

func prInput(cmd prCmd) spread.CLIInput {
	return spread.CLIInput{Kind: spread.KindPR, Items: []string{cmd.Item}, Targets: cmd.To, Mode: cmd.Mode}
}
```

- [ ] **Step 4: Run CLI tests**

Run:

```bash
go test ./internal/cli -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go
git commit -m "feat: parse spread commands"
```

## Task 11: Editor Handoff

**Files:**
- Create: `internal/editor/editor.go`
- Create: `internal/editor/editor_test.go`

- [ ] **Step 1: Write editor command tests**

Create `internal/editor/editor_test.go`:

```go
package editor

import "testing"

func TestCommandForExplicitEditor(t *testing.T) {
	cmd, args, err := CommandWithLookup("code", "/tmp/workspace", func(name string) (string, error) {
		return "/bin/" + name, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "/bin/code" || len(args) != 1 || args[0] != "/tmp/workspace" {
		t.Fatalf("cmd=%q args=%v", cmd, args)
	}
}

func TestCommandRejectsUnknownExplicitEditor(t *testing.T) {
	_, _, err := CommandWithLookup("unknown-editor", "/tmp/workspace", func(name string) (string, error) {
		return "/bin/" + name, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAutoUsesFirstAvailableEditor(t *testing.T) {
	cmd, args, err := CommandWithLookup("auto", "/tmp/workspace", func(name string) (string, error) {
		if name == "idea" {
			return "/bin/idea", nil
		}
		return "", ErrNotFound
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "/bin/idea" || args[0] != "/tmp/workspace" {
		t.Fatalf("cmd=%q args=%v", cmd, args)
	}
}
```

- [ ] **Step 2: Run editor tests to verify failure**

Run:

```bash
go test ./internal/editor -count=1
```

Expected: fails because editor package is missing.

- [ ] **Step 3: Implement editor command selection**

Create `internal/editor/editor.go`:

```go
package editor

import (
	"errors"
	"fmt"
	"os/exec"
)

var ErrNotFound = errors.New("editor not found")

var known = map[string]string{
	"code":   "code",
	"idea":   "idea",
	"cursor": "cursor",
}

func Command(name string, workspace string) (string, []string, error) {
	return CommandWithLookup(name, workspace, func(command string) (string, error) {
		path, err := exec.LookPath(command)
		if err != nil {
			return "", ErrNotFound
		}
		return path, nil
	})
}

func CommandWithLookup(name string, workspace string, lookup func(string) (string, error)) (string, []string, error) {
	if name == "" || name == "auto" {
		for _, candidate := range []string{"code", "idea", "cursor"} {
			if path, err := lookup(candidate); err == nil {
				return path, []string{workspace}, nil
			}
		}
		return "", nil, fmt.Errorf("no supported editor found; open %s manually", workspace)
	}
	cmd, ok := known[name]
	if !ok {
		return "", nil, fmt.Errorf("editor %q is not supported", name)
	}
	path, err := lookup(cmd)
	if err != nil {
		return "", nil, fmt.Errorf("editor %q not found in PATH", name)
	}
	return path, []string{workspace}, nil
}
```

- [ ] **Step 4: Run editor tests**

Run:

```bash
go test ./internal/editor -count=1
```

Expected: pass without depending on installed editor binaries.

- [ ] **Step 5: Commit**

```bash
git add internal/editor/editor.go internal/editor/editor_test.go
git commit -m "feat: select conflict editor"
```

## Task 12: Bubble Tea TUI Model

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/model_test.go`

- [ ] **Step 1: Write TUI model tests**

Create `internal/tui/model_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/state"
)

func TestViewShowsConflictWorkspace(t *testing.T) {
	m := NewModel(state.Run{
		ID: "run-1",
		Mode: "direct",
		Source: "develop",
		Targets: []state.Target{
			{Branch: "release/1.0", Status: state.StatusDone},
			{Branch: "release/1.1", Status: state.StatusConflict, WorkspacePath: ".spread/release-1.1", ConflictedFiles: []string{"user.go", "config.yaml"}},
		},
	})
	view := m.View()
	for _, want := range []string{"release/1.1", ".spread/release-1.1", "user.go", "open workspace"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: Run TUI tests to verify failure**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: fails because TUI package is missing.

- [ ] **Step 3: Implement minimal TUI model**

Create `internal/tui/model.go`:

```go
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/liyown/git-spread/internal/state"
)

type Model struct {
	run    state.Run
	cursor int
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
		}
	}
	return m, nil
}

func (m Model) View() string {
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
	return b.String()
}
```

- [ ] **Step 4: Run TUI tests**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go go.mod go.sum
git commit -m "feat: add tui run model"
```

## Task 13: GitHub Interface and PR Propagation Planning

**Files:**
- Create: `internal/github/client.go`
- Create: `internal/github/client_test.go`
- Create: `internal/spread/executor_pr_test.go`
- Modify: `internal/spread/types.go`
- Modify: `internal/spread/planner.go`

- [ ] **Step 1: Write GitHub interface tests**

Create `internal/github/client_test.go`:

```go
package github

import "testing"

func TestMemoryClientReturnsPRCommits(t *testing.T) {
	client := MemoryClient{
		PullRequests: map[string]PullRequest{
			"123": {Number: "123", Commits: []string{"a", "b"}},
		},
	}
	pr, err := client.PullRequest("123")
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Commits) != 2 {
		t.Fatalf("commits = %#v", pr.Commits)
	}
}
```

Create `internal/spread/executor_pr_test.go`:

```go
package spread

import (
	"testing"

	"github.com/liyown/git-spread/internal/git"
	gh "github.com/liyown/git-spread/internal/github"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestPlanPRUsesGitHubCommits(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")

	req := Request{Kind: KindPR, Items: []string{"123"}, Targets: []string{"release/1.0"}, Mode: ModePR, Remote: ".", WorkspaceDir: ".spread"}
	client := gh.MemoryClient{PullRequests: map[string]gh.PullRequest{"123": {Number: "123", Commits: []string{"abc", "def"}}}}
	plan, err := BuildPlanWithGitHub(req, git.NewRunner(repo.Dir), client)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Commits) != 2 || plan.Commits[0] != "abc" || plan.Commits[1] != "def" {
		t.Fatalf("commits = %#v", plan.Commits)
	}
}
```

- [ ] **Step 2: Run GitHub tests to verify failure**

Run:

```bash
go test ./internal/github ./internal/spread -run 'GitHub|PlanPR' -count=1
```

Expected: fails because GitHub package and PR planner are missing.

- [ ] **Step 3: Implement GitHub interface and PR plan hook**

Create `internal/github/client.go`:

```go
package github

import "fmt"

type PullRequest struct {
	Number  string
	URL     string
	Commits []string
}

type CreatedPullRequest struct {
	URL string
}

type CreatePullRequestInput struct {
	Title string
	Head  string
	Base  string
	Body  string
}

type Client interface {
	PullRequest(id string) (PullRequest, error)
	CreatePullRequest(input CreatePullRequestInput) (CreatedPullRequest, error)
}

type MemoryClient struct {
	PullRequests map[string]PullRequest
	Created      []CreatePullRequestInput
}

func (m MemoryClient) PullRequest(id string) (PullRequest, error) {
	pr, ok := m.PullRequests[id]
	if !ok {
		return PullRequest{}, fmt.Errorf("pull request %q not found", id)
	}
	return pr, nil
}

func (m MemoryClient) CreatePullRequest(input CreatePullRequestInput) (CreatedPullRequest, error) {
	return CreatedPullRequest{URL: "https://example.test/pull/1"}, nil
}
```

Modify `internal/spread/planner.go` by adding:

```go
func BuildPlanWithGitHub(req Request, runner git.Runner, client github.Client) (Plan, error) {
	if req.Kind != KindPR {
		return BuildPlan(req, runner)
	}
	pr, err := client.PullRequest(req.Items[0])
	if err != nil {
		return Plan{}, err
	}
	reqForPlan := req
	reqForPlan.Kind = KindCommit
	reqForPlan.Items = pr.Commits
	plan, err := BuildPlan(reqForPlan, runner)
	if err != nil {
		return Plan{}, err
	}
	plan.Request = req
	plan.Commits = pr.Commits
	return plan, nil
}
```

Add import:

```go
github "github.com/liyown/git-spread/internal/github"
```

- [ ] **Step 4: Run GitHub planning tests**

Run:

```bash
go test ./internal/github ./internal/spread -run 'GitHub|PlanPR' -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/github/client.go internal/github/client_test.go internal/spread/planner.go internal/spread/executor_pr_test.go
git commit -m "feat: plan pull request propagation"
```

## Task 14: PR Mode Branch Naming and PR Creation

**Files:**
- Modify: `internal/spread/executor.go`
- Modify: `internal/spread/executor_pr_test.go`
- Modify: `internal/github/client.go`

- [ ] **Step 1: Write PR mode creation test**

Extend `internal/spread/executor_pr_test.go`:

```go
func TestPRModeCreatesPullRequestPerTarget(t *testing.T) {
	client := &RecordingClient{}
	created, err := CreateTargetPR(client, "spread/release-1.0/abc123", "release/1.0", "Propagate changes to release/1.0")
	if err != nil {
		t.Fatal(err)
	}
	if created.URL == "" {
		t.Fatal("expected pull request URL")
	}
	if client.Input.Head != "spread/release-1.0/abc123" || client.Input.Base != "release/1.0" {
		t.Fatalf("input = %#v", client.Input)
	}
}

type RecordingClient struct {
	Input github.CreatePullRequestInput
}

func (r *RecordingClient) PullRequest(id string) (github.PullRequest, error) {
	return github.PullRequest{}, nil
}

func (r *RecordingClient) CreatePullRequest(input github.CreatePullRequestInput) (github.CreatedPullRequest, error) {
	r.Input = input
	return github.CreatedPullRequest{URL: "https://example.test/pull/1"}, nil
}
```

- [ ] **Step 2: Run PR creation test to verify failure**

Run:

```bash
go test ./internal/spread -run TestPRModeCreatesPullRequestPerTarget -count=1
```

Expected: fails because `CreateTargetPR` does not exist.

- [ ] **Step 3: Implement PR creation helper**

Add to `internal/spread/executor.go`:

```go
func CreateTargetPR(client github.Client, head string, base string, title string) (github.CreatedPullRequest, error) {
	return client.CreatePullRequest(github.CreatePullRequestInput{
		Title: title,
		Head:  head,
		Base:  base,
		Body:  "Created by Git Spread.",
	})
}
```

Add import:

```go
github "github.com/liyown/git-spread/internal/github"
```

- [ ] **Step 4: Run PR mode tests**

Run:

```bash
go test ./internal/spread -run 'TestPRMode|TestPlanPR' -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/spread/executor.go internal/spread/executor_pr_test.go internal/github/client.go
git commit -m "feat: create target pull requests"
```

## Task 15: Init Command and TUI Launch Wiring

**Files:**
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write init command test**

Add to `internal/cli/commands_test.go`:

```go
func TestInitDryRunWritesConfigTemplate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--print"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"version: 1", "mode: direct", "tasks:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run CLI init test to verify failure**

Run:

```bash
go test ./internal/cli -run TestInitDryRun -count=1
```

Expected: fails because init command is not implemented.

- [ ] **Step 3: Add init command template**

Extend `internal/cli/commands.go`:

```go
type app struct {
	Version versionCmd `cmd:"" help:"Print version."`
	Init    initCmd    `cmd:"" help:"Create a .git-spread.yml config."`
	Plan    planCmd    `cmd:"" help:"Show what Git Spread would do."`
	Branch  branchCmd  `cmd:"" help:"Propagate a branch."`
	Commit  commitCmd  `cmd:"" help:"Propagate explicit commits or ranges."`
	PR      prCmd      `cmd:"pr" help:"Propagate a pull request."`
	NoTUI   bool       `help:"Disable interactive TUI."`
}

type initCmd struct {
	Print bool `help:"Print config template instead of writing a file."`
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
```

Handle command:

```go
case "init":
	fmt.Fprint(stdout, configTemplate)
	return 0
```

- [ ] **Step 4: Run CLI tests**

Run:

```bash
go test ./internal/cli -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/commands.go internal/cli/commands_test.go README.md
git commit -m "feat: add init command"
```

## Task 16: TUI Action Wiring

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Write TUI action tests**

Add to `internal/tui/model_test.go`:

```go
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
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		got := updated.(Model).LastAction
		if got != tc.want {
			t.Fatalf("key %q action = %q, want %q", tc.key, got, tc.want)
		}
	}
}
```

Add import to `internal/tui/model_test.go`:

```go
tea "charm.land/bubbletea/v2"
```

- [ ] **Step 2: Run TUI action tests to verify failure**

Run:

```bash
go test ./internal/tui -run TestKeyBindingsSetActions -count=1
```

Expected: fails because `Action` and `LastAction` are missing.

- [ ] **Step 3: Implement TUI action state**

Modify `internal/tui/model.go`:

```go
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
```

Extend the `tea.KeyMsg` switch in `Update`:

```go
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
```

- [ ] **Step 4: Run TUI tests**

Run:

```bash
go test ./internal/tui -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "feat: wire tui actions"
```

## Task 17: go-gh REST Adapter

**Files:**
- Modify: `internal/github/client.go`
- Modify: `internal/github/client_test.go`

- [ ] **Step 1: Write go-gh adapter tests against a fake REST client**

Add to `internal/github/client_test.go`:

```go
func TestGoGHClientPullRequestUsesREST(t *testing.T) {
	rest := &fakeREST{
		get: func(path string, resp interface{}) error {
			if path != "repos/OWNER/REPO/pulls/123/commits" {
				t.Fatalf("path = %q", path)
			}
			out := resp.(*[]struct {
				SHA string `json:"sha"`
			})
			*out = []struct {
				SHA string `json:"sha"`
			}{{SHA: "abc"}, {SHA: "def"}}
			return nil
		},
	}
	client := NewGoGHClientWithREST("OWNER", "REPO", rest)
	pr, err := client.PullRequest("123")
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Commits) != 2 || pr.Commits[1] != "def" {
		t.Fatalf("pr = %#v", pr)
	}
}

func TestGoGHClientCreatePullRequestUsesREST(t *testing.T) {
	rest := &fakeREST{
		post: func(path string, body io.Reader, resp interface{}) error {
			if path != "repos/OWNER/REPO/pulls" {
				t.Fatalf("path = %q", path)
			}
			var payload CreatePullRequestInput
			if err := json.NewDecoder(body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Head != "spread/release-1.0/abc" || payload.Base != "release/1.0" {
				t.Fatalf("payload = %#v", payload)
			}
			out := resp.(*struct {
				URL string `json:"html_url"`
			})
			out.URL = "https://github.com/OWNER/REPO/pull/1"
			return nil
		},
	}
	client := NewGoGHClientWithREST("OWNER", "REPO", rest)
	created, err := client.CreatePullRequest(CreatePullRequestInput{Title: "Backport", Head: "spread/release-1.0/abc", Base: "release/1.0", Body: "Created by Git Spread."})
	if err != nil {
		t.Fatal(err)
	}
	if created.URL == "" {
		t.Fatal("expected URL")
	}
}

type fakeREST struct {
	get  func(string, interface{}) error
	post func(string, io.Reader, interface{}) error
}

func (f *fakeREST) Get(path string, resp interface{}) error {
	return f.get(path, resp)
}

func (f *fakeREST) Post(path string, body io.Reader, resp interface{}) error {
	return f.post(path, body, resp)
}
```

Add imports to `internal/github/client_test.go`:

```go
import (
	"encoding/json"
	"io"
	"testing"
)
```

- [ ] **Step 2: Run adapter tests to verify failure**

Run:

```bash
go test ./internal/github -run GoGHClient -count=1
```

Expected: fails because `GoGHClient` is missing.

- [ ] **Step 3: Implement go-gh backed adapter**

Modify `internal/github/client.go`:

```go
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/api"
)

type REST interface {
	Get(path string, resp interface{}) error
	Post(path string, body io.Reader, resp interface{}) error
}

type GoGHClient struct {
	owner string
	repo  string
	rest  REST
}

func NewGoGHClient(owner string, repo string) (*GoGHClient, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return NewGoGHClientWithREST(owner, repo, rest), nil
}

func NewGoGHClientWithREST(owner string, repo string, rest REST) *GoGHClient {
	return &GoGHClient{owner: owner, repo: repo, rest: rest}
}

func (c *GoGHClient) PullRequest(id string) (PullRequest, error) {
	var response []struct {
		SHA string `json:"sha"`
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%s/commits", c.owner, c.repo, id)
	if err := c.rest.Get(path, &response); err != nil {
		return PullRequest{}, err
	}
	pr := PullRequest{Number: id}
	for _, commit := range response {
		pr.Commits = append(pr.Commits, commit.SHA)
	}
	return pr, nil
}

func (c *GoGHClient) CreatePullRequest(input CreatePullRequestInput) (CreatedPullRequest, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return CreatedPullRequest{}, err
	}
	var response struct {
		URL string `json:"html_url"`
	}
	path := fmt.Sprintf("repos/%s/%s/pulls", c.owner, c.repo)
	if err := c.rest.Post(path, bytes.NewReader(data), &response); err != nil {
		return CreatedPullRequest{}, err
	}
	return CreatedPullRequest{URL: response.URL}, nil
}
```

Preserve the existing `PullRequest`, `CreatedPullRequest`, `CreatePullRequestInput`, `Client`, and `MemoryClient` definitions in the same file.

- [ ] **Step 4: Run GitHub tests**

Run:

```bash
go test ./internal/github -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/github/client.go internal/github/client_test.go go.mod go.sum
git commit -m "feat: add go-gh github client"
```

## Task 18: End-to-End Test and Documentation

**Files:**
- Modify: `README.md`
- Create: `internal/spread/e2e_test.go`

- [ ] **Step 1: Write an end-to-end propagation test**

Create `internal/spread/e2e_test.go`:

```go
package spread

import (
	"path/filepath"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestEndToEndBranchPropagation(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Branch("main-copy")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")

	req := Request{Kind: KindBranch, Source: "develop", Targets: []string{"release/1.0", "main-copy"}, Mode: ModeDirect, Remote: ".", Workspace: WorkspaceIsolated, WorkspaceDir: ".spread"}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	run, err := Execute(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(run.Targets))
	}
	for _, target := range run.Targets {
		if target.Status != state.StatusDone {
			t.Fatalf("%s status = %q", target.Branch, target.Status)
		}
	}
}
```

- [ ] **Step 2: Run the full test suite**

Run:

```bash
go test ./...
```

Expected: pass.

- [ ] **Step 3: Build the binary**

Run:

```bash
go build ./cmd/git-spread
./git-spread --version
```

Expected:

```text
git-spread dev
```

- [ ] **Step 4: Update README usage**

Replace `README.md` with:

````markdown
# Git Spread

Git Spread propagates branch, commit, and pull request changes across Git branches.

## Examples

```bash
git spread init --print
git spread branch develop --to release/1.0,main --no-tui
git spread commit abc123 --to release/1.0 --mode pr --no-tui
git spread pr 123 --to release/* --mode pr --no-tui
git spread
```

Conflicts are resolved in isolated workspaces under `.spread/`. Open the workspace in your editor, resolve the conflict, then run:

```bash
git spread continue
```

## Design

The design spec lives in `docs/superpowers/specs/2026-06-13-git-spread-design.md`.
````

- [ ] **Step 5: Run final verification**

Run:

```bash
go test ./...
go build ./cmd/git-spread
git status --short
```

Expected: tests pass, build succeeds, and only intended files are modified.

- [ ] **Step 6: Commit**

```bash
git add README.md internal/spread/e2e_test.go
git commit -m "test: cover end-to-end propagation"
```

## Self-Review Notes

Spec coverage:

- Branch propagation: Tasks 3, 5, 7, and 18.
- Commit propagation: Tasks 3, 5, 8.
- Pull request propagation: Tasks 13 and 14.
- Direct mode: Tasks 7, 8, and 18.
- PR mode: Tasks 13 and 14.
- Isolated workspaces: Tasks 5, 7, 8, and 18.
- TUI: Tasks 12 and 16.
- Editor handoff: Task 11.
- Continue and abort: Task 9.
- Config and CLI override foundation: Tasks 2, 3, 10, 15.
- GitHub testability: Tasks 13, 14, and 17.
- Technical stack: Tasks 1, 10, 12, 13, and 17.

Known sequencing decisions:

- `git spread plan` starts with a simple command-level dry run in Task 10. Rich per-target plan output is implemented by reusing `BuildPlan` after CLI dispatch is connected to repository discovery.
- Task 12 creates the TUI view model. Task 16 wires keyboard actions into the model so the CLI adapter can call editor, continue, PR switch, and abort services.
- Task 13 defines mocked GitHub behavior for propagation planning. Task 17 adds the real go-gh REST adapter behind the same interface.
