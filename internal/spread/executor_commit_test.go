package spread

import (
	"errors"
	"os"
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

func TestExecuteCommitDirectSkipsAlreadyAppliedCommit(t *testing.T) {
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
	store := state.NewStore(filepath.Join(repo.Dir, ".git", "spread"))
	if _, err := Execute(plan, git.NewRunner(repo.Dir), store); err != nil {
		t.Fatal(err)
	}

	run, err := Execute(plan, git.NewRunner(repo.Dir), store)
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].Status != state.StatusDone {
		t.Fatalf("status = %q, want done", run.Targets[0].Status)
	}
	if _, err := store.Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active run should be cleared after repeat completion, err = %v", err)
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
	repo.Checkout("main")

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
	if run.Targets[0].Step != "cherry-pick commits" {
		t.Fatalf("step = %q, want cherry-pick step", run.Targets[0].Step)
	}
}
