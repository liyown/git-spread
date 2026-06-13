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
