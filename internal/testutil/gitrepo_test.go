package testutil

import "testing"

func TestRepoCreatesInitialBranch(t *testing.T) {
	repo := NewGitRepo(t)
	repo.Write("README.md", "hello\n")
	repo.Commit("initial")
	repo.Branch("develop")

	if got := repo.CurrentBranch(); got != "main" {
		t.Fatalf("current branch = %q, want main", got)
	}
}
