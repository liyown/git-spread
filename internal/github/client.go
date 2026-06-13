package github

import "fmt"

type PullRequest struct {
	Number  string
	URL     string
	Commits []string
}

type CreatedPullRequest struct {
	URL string
}

type CreatePullRequestInput struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

type Client interface {
	PullRequest(id string) (PullRequest, error)
	CreatePullRequest(input CreatePullRequestInput) (CreatedPullRequest, error)
}

type MemoryClient struct {
	PullRequests map[string]PullRequest
	Created      []CreatePullRequestInput
}

func (m MemoryClient) PullRequest(id string) (PullRequest, error) {
	pr, ok := m.PullRequests[id]
	if !ok {
		return PullRequest{}, fmt.Errorf("pull request %q not found", id)
	}
	return pr, nil
}

func (m MemoryClient) CreatePullRequest(input CreatePullRequestInput) (CreatedPullRequest, error) {
	return CreatedPullRequest{URL: "https://example.test/pull/1"}, nil
}
