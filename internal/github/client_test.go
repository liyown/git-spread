package github

import (
	"encoding/json"
	"io"
	"testing"
)

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

func TestGoGHClientPullRequestUsesREST(t *testing.T) {
	rest := &fakeREST{
		get: func(path string, resp interface{}) error {
			if path != "repos/OWNER/REPO/pulls/123/commits" {
				t.Fatalf("path = %q", path)
			}
			out := resp.(*[]struct {
				SHA string `json:"sha"`
			})
			*out = []struct {
				SHA string `json:"sha"`
			}{{SHA: "abc"}, {SHA: "def"}}
			return nil
		},
	}
	client := NewGoGHClientWithREST("OWNER", "REPO", rest)
	pr, err := client.PullRequest("123")
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Commits) != 2 || pr.Commits[1] != "def" {
		t.Fatalf("pr = %#v", pr)
	}
}

func TestGoGHClientCreatePullRequestUsesREST(t *testing.T) {
	rest := &fakeREST{
		post: func(path string, body io.Reader, resp interface{}) error {
			if path != "repos/OWNER/REPO/pulls" {
				t.Fatalf("path = %q", path)
			}
			var payload CreatePullRequestInput
			if err := json.NewDecoder(body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Head != "spread/release-1.0/abc" || payload.Base != "release/1.0" {
				t.Fatalf("payload = %#v", payload)
			}
			out := resp.(*struct {
				URL string `json:"html_url"`
			})
			out.URL = "https://github.com/OWNER/REPO/pull/1"
			return nil
		},
	}
	client := NewGoGHClientWithREST("OWNER", "REPO", rest)
	created, err := client.CreatePullRequest(CreatePullRequestInput{Title: "Backport", Head: "spread/release-1.0/abc", Base: "release/1.0", Body: "Created by Git Spread."})
	if err != nil {
		t.Fatal(err)
	}
	if created.URL == "" {
		t.Fatal("expected URL")
	}
}

type fakeREST struct {
	get  func(string, interface{}) error
	post func(string, io.Reader, interface{}) error
}

func (f *fakeREST) Get(path string, resp interface{}) error {
	return f.get(path, resp)
}

func (f *fakeREST) Post(path string, body io.Reader, resp interface{}) error {
	return f.post(path, body, resp)
}
