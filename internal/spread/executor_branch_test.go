package spread

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
)

type recordingProgressReporter struct {
	messages []string
}

func (r *recordingProgressReporter) Report(run state.Run, message string) {
	r.messages = append(r.messages, message)
}

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
	if _, err := store.Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active run should be cleared after completion, err = %v", err)
	}
}

func TestExecuteReportsProgressSteps(t *testing.T) {
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

	reporter := &recordingProgressReporter{}
	if _, err := ExecuteWithProgress(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")), reporter); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(reporter.messages, "\n")
	for _, want := range []string{"release/1.0: checkout release/1.0", "release/1.0: merge develop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("progress missing %q:\n%s", want, got)
		}
	}
}

func TestExecuteBranchDirectRefreshesNewTargetWorkspaceFromRemote(t *testing.T) {
	origin, workDir := repoWithStaleTarget(t)

	req := Request{
		Kind:         KindBranch,
		Source:       "develop",
		Targets:      []string{"release/1.0"},
		Mode:         ModeDirect,
		Remote:       "origin",
		Workspace:    WorkspaceIsolated,
		WorkspaceDir: ".spread",
	}
	plan, err := BuildPlan(req, git.NewRunner(workDir))
	if err != nil {
		t.Fatal(err)
	}

	run, err := Execute(plan, git.NewRunner(workDir), state.NewStore(filepath.Join(workDir, ".git", "spread")))
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", run.Targets[0].Status)
	}
	workspace := filepath.Join(workDir, ".spread", "release-1.0")
	if _, err := git.NewRunner(workspace).Output("show", "HEAD:remote.txt"); err != nil {
		t.Fatal(err)
	}
	assertRemoteHasFile(t, origin, "release/1.0", "feature.txt")
}

func TestExecuteBranchDirectRefreshesExistingTargetWorkspaceFromRemote(t *testing.T) {
	origin, workDir := repoWithStaleTarget(t)
	workspace := filepath.Join(workDir, ".spread", "release-1.0")
	if err := git.NewRunner(workDir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}

	req := Request{
		Kind:         KindBranch,
		Source:       "develop",
		Targets:      []string{"release/1.0"},
		Mode:         ModeDirect,
		Remote:       "origin",
		Workspace:    WorkspaceIsolated,
		WorkspaceDir: ".spread",
	}
	plan, err := BuildPlan(req, git.NewRunner(workDir))
	if err != nil {
		t.Fatal(err)
	}

	run, err := Execute(plan, git.NewRunner(workDir), state.NewStore(filepath.Join(workDir, ".git", "spread")))
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", run.Targets[0].Status)
	}
	if _, err := git.NewRunner(workspace).Output("show", "HEAD:remote.txt"); err != nil {
		t.Fatal(err)
	}
	assertRemoteHasFile(t, origin, "release/1.0", "feature.txt")
}

func TestExecuteBranchDirectReusesExistingTargetWorkspace(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "develop")
	repo.Write("feature-one.txt", "one\n")
	repo.Commit("feature one")

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
	if _, err := Execute(plan, git.NewRunner(repo.Dir), store); err != nil {
		t.Fatal(err)
	}

	repo.Write("feature-two.txt", "two\n")
	repo.Commit("feature two")
	plan, err = BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	run, err := Execute(plan, git.NewRunner(repo.Dir), store)
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", run.Targets[0].Status)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".spread", "release-1.0", "feature-two.txt")); err != nil {
		t.Fatal(err)
	}
}

func repoWithStaleTarget(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	if err := git.NewRunner("").Run("init", "--bare", origin); err != nil {
		t.Fatal(err)
	}

	seed := testutil.NewGitRepo(t)
	seed.Write("README.md", "base\n")
	seed.Commit("initial")
	seed.Branch("release/1.0")
	seed.Checkout("-b", "develop")
	seed.Write("feature.txt", "feature\n")
	seed.Commit("add feature")
	seedRunner := git.NewRunner(seed.Dir)
	if err := seedRunner.Run("remote", "add", "origin", origin); err != nil {
		t.Fatal(err)
	}
	if err := seedRunner.Run("push", "origin", "main", "develop", "release/1.0"); err != nil {
		t.Fatal(err)
	}

	workDir := filepath.Join(root, "work")
	if err := git.NewRunner("").Run("clone", origin, workDir); err != nil {
		t.Fatal(err)
	}
	configureTestUser(t, git.NewRunner(workDir))
	if err := git.NewRunner(workDir).Run("checkout", "-b", "release/1.0", "origin/release/1.0"); err != nil {
		t.Fatal(err)
	}
	if err := git.NewRunner(workDir).Run("checkout", "develop"); err != nil {
		t.Fatal(err)
	}

	updaterDir := filepath.Join(root, "updater")
	if err := git.NewRunner("").Run("clone", origin, updaterDir); err != nil {
		t.Fatal(err)
	}
	updater := git.NewRunner(updaterDir)
	configureTestUser(t, updater)
	if err := updater.Run("checkout", "release/1.0"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, updaterDir, "remote.txt", "remote\n")
	if err := updater.Run("add", "."); err != nil {
		t.Fatal(err)
	}
	if err := updater.Run("commit", "-m", "remote target update"); err != nil {
		t.Fatal(err)
	}
	if err := updater.Run("push", "origin", "release/1.0"); err != nil {
		t.Fatal(err)
	}

	return origin, workDir
}

func configureTestUser(t *testing.T, runner git.Runner) {
	t.Helper()
	if err := runner.Run("config", "user.email", "test@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run("config", "user.name", "Git Spread Test"); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, dir string, path string, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRemoteHasFile(t *testing.T, origin string, branch string, path string) {
	t.Helper()
	verifyDir := filepath.Join(t.TempDir(), "verify")
	if err := git.NewRunner("").Run("clone", origin, verifyDir); err != nil {
		t.Fatal(err)
	}
	if err := git.NewRunner(verifyDir).Run("checkout", branch); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(verifyDir, path)); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteBranchDirectBlocksWhenExistingWorkspaceIsDirty(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")

	workspace := filepath.Join(repo.Dir, ".spread", "main")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "main", workspace, "main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "scratch.txt"), []byte("draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := Request{
		Kind:         KindBranch,
		Source:       "develop",
		Targets:      []string{"main"},
		Mode:         ModeDirect,
		Remote:       ".",
		Workspace:    WorkspaceIsolated,
		WorkspaceDir: ".spread",
	}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	run, err := Execute(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")))
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusBlocked {
		t.Fatalf("status = %q, want blocked", run.Targets[0].Status)
	}
	if !strings.Contains(run.Targets[0].Error, "Workspace has uncommitted changes") {
		t.Fatalf("error = %q, want workspace action message", run.Targets[0].Error)
	}
}

func TestExecuteBranchFailureRecordsError(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")

	req := Request{
		Kind:         KindBranch,
		Source:       "missing-source",
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

	run, err := Execute(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")))
	if err == nil {
		t.Fatal("expected merge failure")
	}
	if run.Targets[0].Status != state.StatusFailed {
		t.Fatalf("status = %q, want failed", run.Targets[0].Status)
	}
	if run.Targets[0].Error == "" {
		t.Fatalf("expected target error, run=%#v", run)
	}
	if run.Targets[0].Step != "merge missing-source" {
		t.Fatalf("step = %q, want merge step", run.Targets[0].Step)
	}
}
