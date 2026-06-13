package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/api"
)

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

type REST interface {
	Get(path string, resp interface{}) error
	Post(path string, body io.Reader, resp interface{}) error
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

type GoGHClient struct {
	owner string
	repo  string
	rest  REST
}

func NewGoGHClient(owner string, repo string) (*GoGHClient, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return NewGoGHClientWithREST(owner, repo, rest), nil
}

func NewGoGHClientWithREST(owner string, repo string, rest REST) *GoGHClient {
	return &GoGHClient{owner: owner, repo: repo, rest: rest}
}

func (c *GoGHClient) PullRequest(id string) (PullRequest, error) {
	var response []struct {
		SHA string `json:"sha"`
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%s/commits", c.owner, c.repo, id)
	if err := c.rest.Get(path, &response); err != nil {
		return PullRequest{}, err
	}
	pr := PullRequest{Number: id}
	for _, commit := range response {
		pr.Commits = append(pr.Commits, commit.SHA)
	}
	return pr, nil
}

func (c *GoGHClient) CreatePullRequest(input CreatePullRequestInput) (CreatedPullRequest, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return CreatedPullRequest{}, err
	}
	var response struct {
		URL string `json:"html_url"`
	}
	path := fmt.Sprintf("repos/%s/%s/pulls", c.owner, c.repo)
	if err := c.rest.Post(path, bytes.NewReader(data), &response); err != nil {
		return CreatedPullRequest{}, err
	}
	return CreatedPullRequest{URL: response.URL}, nil
}
