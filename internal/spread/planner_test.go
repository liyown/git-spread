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
		Kind:         KindBranch,
		Source:       "feature/login-fix",
		Targets:      []string{"release/*"},
		Mode:         ModeDirect,
		Remote:       ".",
		WorkspaceDir: ".spread",
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
		Kind:         KindCommit,
		Items:        []string{"main..feature/login-fix"},
		Targets:      []string{"main"},
		Mode:         ModeDirect,
		Remote:       ".",
		WorkspaceDir: ".spread",
	}

	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(plan.Commits); got != 2 {
		t.Fatalf("commits = %d, want 2", got)
	}
}
