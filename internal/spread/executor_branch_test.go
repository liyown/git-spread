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
}
