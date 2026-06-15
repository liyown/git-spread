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
