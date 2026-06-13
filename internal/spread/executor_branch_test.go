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
