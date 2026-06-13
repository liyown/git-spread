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
