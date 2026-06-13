package github

import "testing"

func TestMemoryClientReturnsPRCommits(t *testing.T) {
	client := MemoryClient{
		PullRequests: map[string]PullRequest{
			"123": {Number: "123", Commits: []string{"a", "b"}},
		},
	}
	pr, err := client.PullRequest("123")
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Commits) != 2 {
		t.Fatalf("commits = %#v", pr.Commits)
	}
}
