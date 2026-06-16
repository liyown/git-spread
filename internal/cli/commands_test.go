package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/spread"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
	"github.com/liyown/git-spread/internal/tui"
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

func TestInitDryRunWritesConfigTemplate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--print"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"version: 1", "mode: direct", "tasks:", "from: auto"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestInitTemplateDocumentsOptionalValues(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--print"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"# mode: direct, pr",
		"# remote: origin, ., or another git remote",
		"# workspace: isolated, current",
		"# editor: auto, code, idea, cursor",
		"# collaboration: auto, shared, fork",
		"# type: branch, commit, pr",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("template missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestInitAddsSpreadToGitignore(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".gitignore"), []byte("dist/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(repo.Dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); !strings.Contains(got, "\n.spread\n") {
		t.Fatalf(".gitignore missing .spread entry:\n%s", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"init"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second init code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if data, err = os.ReadFile(filepath.Join(repo.Dir, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(data), ".spread"); count != 1 {
		t.Fatalf(".spread should be added once, count=%d:\n%s", count, string(data))
	}
}

func TestExamplesCommandPrintsCommonWorkflows(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"examples"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"git spread branch", "git spread commit", "git spread pr", "git spread retry"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestCompletionCommandPrintsShellCompletion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"completion", "zsh"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "#compdef git-spread") || !strings.Contains(stdout.String(), "history") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestUpdateCommandPrintsInstallCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"update"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "scripts/install.sh") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunWithoutArgsShowsConfiguredTasksWhenNoActiveRun(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git-spread.yml"), []byte(`
version: 1
tasks:
  release:
    type: branch
    from: develop
    to:
      - main
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Tasks", "release", "develop -> main", "direct"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunWithoutArgsReportsInvalidConfig(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git-spread.yml"), []byte(`
version: 1
tasks:
  release:
    type: branch
    from: develop
    to:
      - main
  release:
    type: branch
    from: develop
    to:
      - release/1.0
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `duplicate task name "release"`) {
		t.Fatalf("stderr missing duplicate task error:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "No tasks configured") {
		t.Fatalf("invalid config should not render empty task state:\n%s", stdout.String())
	}
}

func TestHistoryCommandPrintsRecentRuns(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.AppendHistory(state.Run{ID: "run-1", Task: "release", Kind: "branch", Mode: "direct", Targets: []state.Target{{Branch: "main", Status: state.StatusDone}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendHistory(state.Run{ID: "run-2", Task: "backport", Kind: "commit", Mode: "pr", Targets: []state.Target{{Branch: "release/1.0", Status: state.StatusConflict}}}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"history"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"run-2", "backport", "conflict 1", "run-1", "release", "done 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunLastUsesMostRecentTaskHistory(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git-spread.yml"), []byte(`
version: 1
defaults:
  remote: "."
tasks:
  release:
    type: branch
    from: develop
    to:
      - main
`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.AppendHistory(state.Run{ID: "old", Task: "release", Kind: "branch", Mode: "direct", Targets: []state.Target{{Branch: "main", Status: state.StatusDone}}}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"run", "--last", "--no-tui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Run ") || !strings.Contains(stdout.String(), "done") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".spread", "main", "feature.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestRetryUsesLastFailedTargets(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")
	if err := os.WriteFile(filepath.Join(repo.Dir, ".git-spread.yml"), []byte(`
version: 1
defaults:
  remote: "."
`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.AppendHistory(state.Run{
		ID:           "failed",
		Kind:         "branch",
		Mode:         "direct",
		Source:       "develop",
		Remote:       ".",
		WorkspaceDir: ".spread",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusFailed},
			{Branch: "release/1.0", Status: state.StatusDone},
		},
	}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"retry", "--no-tui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".spread", "main", "feature.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".spread", "release-1.0", "feature.txt")); !os.IsNotExist(err) {
		t.Fatalf("release target should not be retried, err=%v", err)
	}
}

func TestDoctorCommandReportsFindings(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.Save(state.Run{
		ID:            "run-1",
		CurrentTarget: 0,
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusConflict, WorkspacePath: ".spread/main"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "workspace is missing: .spread/main") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestResetTargetRemovesTargetAndWorktree(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	workspace := filepath.Join(repo.Dir, ".spread", "release-1.0")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.Save(state.Run{
		ID:            "run-1",
		CurrentTarget: 0,
		Targets: []state.Target{
			{Branch: "release/1.0", Status: state.StatusFailed, WorkspacePath: ".spread/release-1.0"},
			{Branch: "main", Status: state.StatusPending, WorkspacePath: ".spread/main"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo.Dir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"reset", "--target", "release/1.0", "--clean-worktree"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	run, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Targets) != 1 || run.Targets[0].Branch != "main" {
		t.Fatalf("run targets = %#v", run.Targets)
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace should be removed, err=%v", err)
	}
}

func TestConfigurePRHeadUsesForkRemoteOwner(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := git.NewRunner(repo.Dir).Run("remote", "add", "fork", "https://github.com/me/example.git"); err != nil {
		t.Fatal(err)
	}

	req := spread.Request{Collaboration: "fork", ForkRemote: "fork"}
	if err := configurePRHead(&req, git.NewRunner(repo.Dir)); err != nil {
		t.Fatal(err)
	}
	if req.HeadRemote != "fork" || req.HeadOwner != "me" {
		t.Fatalf("request = %#v", req)
	}
}

func TestLoadRepoContextFromWorktreeUsesMainRepositoryState(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	workspace := filepath.Join(repo.Dir, ".spread", "release-1.0")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	ctx, err := loadRepoContext()
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, err := filepath.EvalSymlinks(ctx.root)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("root = %q, want %q", gotRoot, wantRoot)
	}
	gotStore, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(ctx.store.Path())))
	if err != nil {
		t.Fatal(err)
	}
	wantStore, err := filepath.EvalSymlinks(filepath.Join(repo.Dir, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if gotStore != wantStore || filepath.Base(filepath.Dir(ctx.store.Path())) != "spread" {
		t.Fatalf("store path = %q", ctx.store.Path())
	}
}

func TestTUIRefreshAfterAbortDoesNotExposeMissingStatePath(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	t.Chdir(repo.Dir)
	ctx, err := loadRepoContext()
	if err != nil {
		t.Fatal(err)
	}

	run, message, err := tuiActionHandler(ctx)(tui.ActionRefresh, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Targets) != 0 {
		t.Fatalf("run = %#v, want empty run", run)
	}
	if !strings.Contains(message, "No active run") || strings.Contains(message, "state.json") {
		t.Fatalf("message = %q", message)
	}
}

func TestPRModeHintUsesExecutableCommand(t *testing.T) {
	run := state.Run{
		Kind:   string(spread.KindBranch),
		Source: "develop",
		Targets: []state.Target{
			{Branch: "main", Status: state.StatusRejected},
		},
	}

	got := prModeHint(run, 0)
	want := "PR mode: run git spread branch develop --to main --mode pr"
	if got != want {
		t.Fatalf("hint = %q, want %q", got, want)
	}
	if strings.Contains(got, "not wired yet") {
		t.Fatalf("hint exposes implementation status: %q", got)
	}
}

func TestResetClearsActiveRunState(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	t.Chdir(repo.Dir)
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.Save(state.Run{
		ID:            "run-1",
		CurrentTarget: 0,
		Targets:       []state.Target{{Branch: "main", Status: state.StatusBlocked, WorkspacePath: ".spread/main"}},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"reset"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := store.Load(); !os.IsNotExist(err) {
		t.Fatalf("active run should be cleared, err=%v", err)
	}
	if !strings.Contains(stdout.String(), "reset Git Spread state") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunWithoutArgsShowsResetHintForCorruptedState(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	t.Chdir(repo.Dir)
	stateDir := filepath.Join(repo.Dir, ".git", "spread")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"State needs reset", "git-spread reset"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "invalid character") || strings.Contains(stderr.String(), "invalid character") {
		t.Fatalf("should not expose JSON parser details: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestStatusShowsResetHintForCorruptedState(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	t.Chdir(repo.Dir)
	stateDir := filepath.Join(repo.Dir, ".git", "spread")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"State needs reset", "git-spread reset"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunWithoutArgsShowsResetHintForInvalidActiveRun(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	t.Chdir(repo.Dir)
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if err := store.Save(state.Run{
		ID:            "run-1",
		CurrentTarget: 4,
		Targets:       []state.Target{{Branch: "main", Status: state.Status("strange"), WorkspacePath: ".spread/main"}},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"State needs reset", "current target is outside target list", "unknown target status", "git-spread reset"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}
