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

func TestContinueRunsRemainingTargets(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Branch("release/1.1")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")

	workspace := filepath.Join(repo.Dir, ".spread", "release-1.0")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}
	if err := git.NewRunner(workspace).Run("commit", "--allow-empty", "-m", "manual resolution"); err != nil {
		t.Fatal(err)
	}

	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	run := state.Run{
		ID:            "run-1",
		Kind:          "branch",
		Mode:          "direct",
		Source:        "develop",
		Remote:        ".",
		WorkspaceDir:  ".spread",
		CurrentTarget: 0,
		Targets: []state.Target{
			{Branch: "release/1.0", Status: state.StatusConflict, WorkspacePath: ".spread/release-1.0"},
			{Branch: "release/1.1", Status: state.StatusPending, WorkspacePath: ".spread/release-1.1"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatal(err)
	}

	updated, err := Continue(git.NewRunner(repo.Dir), store)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range updated.Targets {
		if target.Status != state.StatusDone {
			t.Fatalf("%s status = %q, want done", target.Branch, target.Status)
		}
	}
	if _, err := git.NewRunner(filepath.Join(repo.Dir, ".spread", "release-1.1")).Output("show", "HEAD:feature.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestAbortClearsRunState(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if err := store.Save(state.Run{ID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	if err := Abort(store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("expected state to be removed")
	}
}
