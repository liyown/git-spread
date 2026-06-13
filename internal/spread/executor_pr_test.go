package spread

import (
	"path/filepath"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	gh "github.com/liyown/git-spread/internal/github"
	"github.com/liyown/git-spread/internal/state"
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

func TestPRModeCreatesPullRequestPerTarget(t *testing.T) {
	client := &RecordingClient{}
	created, err := CreateTargetPR(client, "spread/release-1.0/abc123", "release/1.0", "Propagate changes to release/1.0")
	if err != nil {
		t.Fatal(err)
	}
	if created.URL == "" {
		t.Fatal("expected pull request URL")
	}
	if client.Input.Head != "spread/release-1.0/abc123" || client.Input.Base != "release/1.0" {
		t.Fatalf("input = %#v", client.Input)
	}
}

func TestExecuteBranchPRModeCreatesPRFromSourceBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "develop")
	repo.Write("feature.txt", "feature\n")
	repo.Commit("add feature")

	req := Request{Kind: KindBranch, Source: "develop", Targets: []string{"release/1.0"}, Mode: ModePR, Remote: ".", WorkspaceDir: ".spread"}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	client := &RecordingClient{}
	run, err := ExecuteWithGitHub(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")), client)
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].PullRequestURL == "" || client.Input.Head != "develop" || client.Input.Base != "release/1.0" {
		t.Fatalf("run=%#v input=%#v", run, client.Input)
	}
}

func TestExecuteCommitPRModeCreatesPropagationBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	repo.Checkout("-b", "feature/login-fix")
	repo.Write("fix.txt", "fix\n")
	repo.Commit("fix login")
	commit := repo.Head()

	req := Request{Kind: KindCommit, Items: []string{commit}, Targets: []string{"release/1.0"}, Mode: ModePR, Remote: ".", WorkspaceDir: ".spread"}
	plan, err := BuildPlan(req, git.NewRunner(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	client := &RecordingClient{}
	run, err := ExecuteWithGitHub(plan, git.NewRunner(repo.Dir), state.NewStore(filepath.Join(repo.Dir, ".git", "spread")), client)
	if err != nil {
		t.Fatal(err)
	}
	if run.Targets[0].CreatedBranch == "" || client.Input.Head != run.Targets[0].CreatedBranch {
		t.Fatalf("run=%#v input=%#v", run, client.Input)
	}
}

type RecordingClient struct {
	Input gh.CreatePullRequestInput
}

func (r *RecordingClient) PullRequest(id string) (gh.PullRequest, error) {
	return gh.PullRequest{}, nil
}

func (r *RecordingClient) CreatePullRequest(input gh.CreatePullRequestInput) (gh.CreatedPullRequest, error) {
	r.Input = input
	return gh.CreatedPullRequest{URL: "https://example.test/pull/1"}, nil
}
