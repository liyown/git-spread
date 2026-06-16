package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

type PullRequest struct {
	Number  string
	URL     string
	Commits []string
}

type CreatedPullRequest struct {
	URL    string
	Number int
}

type CreatePullRequestInput struct {
	Title     string   `json:"title"`
	Head      string   `json:"head"`
	Base      string   `json:"base"`
	Body      string   `json:"body"`
	Draft     bool     `json:"draft,omitempty"`
	Labels    []string `json:"-"`
	Reviewers []string `json:"-"`
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
	return CreatedPullRequest{URL: "https://example.test/pull/1", Number: 1}, nil
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
	if existing, ok, err := c.existingOpenPullRequest(input); err != nil {
		return CreatedPullRequest{}, err
	} else if ok {
		return existing, nil
	}

	data, err := json.Marshal(input)
	if err != nil {
		return CreatedPullRequest{}, err
	}
	var response struct {
		URL    string `json:"html_url"`
		Number int    `json:"number"`
	}
	path := fmt.Sprintf("repos/%s/%s/pulls", c.owner, c.repo)
	if err := c.rest.Post(path, bytes.NewReader(data), &response); err != nil {
		return CreatedPullRequest{}, err
	}
	if response.Number != 0 {
		if err := c.addLabels(response.Number, input.Labels); err != nil {
			return CreatedPullRequest{}, err
		}
		if err := c.requestReviewers(response.Number, input.Reviewers); err != nil {
			return CreatedPullRequest{}, err
		}
	}
	return CreatedPullRequest{URL: response.URL, Number: response.Number}, nil
}

func (c *GoGHClient) existingOpenPullRequest(input CreatePullRequestInput) (CreatedPullRequest, bool, error) {
	head := input.Head
	if !strings.Contains(head, ":") {
		head = c.owner + ":" + head
	}
	var response []struct {
		URL    string `json:"html_url"`
		Number int    `json:"number"`
	}
	path := fmt.Sprintf(
		"repos/%s/%s/pulls?state=open&head=%s&base=%s",
		c.owner,
		c.repo,
		url.QueryEscape(head),
		url.QueryEscape(input.Base),
	)
	if err := c.rest.Get(path, &response); err != nil {
		return CreatedPullRequest{}, false, err
	}
	if len(response) == 0 {
		return CreatedPullRequest{}, false, nil
	}
	return CreatedPullRequest{URL: response[0].URL, Number: response[0].Number}, true, nil
}

func (c *GoGHClient) addLabels(number int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	payload := struct {
		Labels []string `json:"labels"`
	}{Labels: labels}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/%s/issues/%d/labels", c.owner, c.repo, number)
	return c.rest.Post(path, bytes.NewReader(data), &struct{}{})
}

func (c *GoGHClient) requestReviewers(number int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}
	payload := struct {
		Reviewers []string `json:"reviewers"`
	}{Reviewers: reviewers}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/requested_reviewers", c.owner, c.repo, number)
	return c.rest.Post(path, bytes.NewReader(data), &struct{}{})
}
