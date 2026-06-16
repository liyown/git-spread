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
	var labelPath string
	var reviewerPath string
	rest := &fakeREST{
		post: func(path string, body io.Reader, resp interface{}) error {
			switch path {
			case "repos/OWNER/REPO/pulls":
				var payload CreatePullRequestInput
				if err := json.NewDecoder(body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Head != "spread/release-1.0/abc" || payload.Base != "release/1.0" || !payload.Draft {
					t.Fatalf("payload = %#v", payload)
				}
				out := resp.(*struct {
					URL    string `json:"html_url"`
					Number int    `json:"number"`
				})
				out.URL = "https://github.com/OWNER/REPO/pull/1"
				out.Number = 1
			case "repos/OWNER/REPO/issues/1/labels":
				labelPath = path
				var payload struct {
					Labels []string `json:"labels"`
				}
				if err := json.NewDecoder(body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.Labels) != 1 || payload.Labels[0] != "backport" {
					t.Fatalf("labels payload = %#v", payload)
				}
			case "repos/OWNER/REPO/pulls/1/requested_reviewers":
				reviewerPath = path
				var payload struct {
					Reviewers []string `json:"reviewers"`
				}
				if err := json.NewDecoder(body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.Reviewers) != 1 || payload.Reviewers[0] != "octocat" {
					t.Fatalf("reviewers payload = %#v", payload)
				}
			default:
				t.Fatalf("path = %q", path)
			}
			return nil
		},
	}
	client := NewGoGHClientWithREST("OWNER", "REPO", rest)
	created, err := client.CreatePullRequest(CreatePullRequestInput{Title: "Backport", Head: "spread/release-1.0/abc", Base: "release/1.0", Body: "Created by Git Spread.", Draft: true, Labels: []string{"backport"}, Reviewers: []string{"octocat"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.URL == "" || created.Number != 1 {
		t.Fatalf("created = %#v", created)
	}
	if labelPath == "" || reviewerPath == "" {
		t.Fatalf("labelPath=%q reviewerPath=%q", labelPath, reviewerPath)
	}
}

func TestGoGHClientCreatePullRequestReusesExistingOpenPR(t *testing.T) {
	var posted bool
	rest := &fakeREST{
		get: func(path string, resp interface{}) error {
			if path != "repos/OWNER/REPO/pulls?state=open&head=OWNER%3Adevelop&base=release%2F1.0" {
				t.Fatalf("path = %q", path)
			}
			out := resp.(*[]struct {
				URL    string `json:"html_url"`
				Number int    `json:"number"`
			})
			*out = []struct {
				URL    string `json:"html_url"`
				Number int    `json:"number"`
			}{{URL: "https://github.com/OWNER/REPO/pull/12", Number: 12}}
			return nil
		},
		post: func(path string, body io.Reader, resp interface{}) error {
			posted = true
			return nil
		},
	}
	client := NewGoGHClientWithREST("OWNER", "REPO", rest)

	created, err := client.CreatePullRequest(CreatePullRequestInput{Title: "Release", Head: "develop", Base: "release/1.0", Body: "Created by Git Spread."})
	if err != nil {
		t.Fatal(err)
	}
	if posted {
		t.Fatal("existing pull request should be reused without creating another one")
	}
	if created.URL != "https://github.com/OWNER/REPO/pull/12" || created.Number != 12 {
		t.Fatalf("created = %#v", created)
	}
}

type fakeREST struct {
	get  func(string, interface{}) error
	post func(string, io.Reader, interface{}) error
}

func (f *fakeREST) Get(path string, resp interface{}) error {
	if f.get == nil {
		return nil
	}
	return f.get(path, resp)
}

func (f *fakeREST) Post(path string, body io.Reader, resp interface{}) error {
	if f.post == nil {
		return nil
	}
	return f.post(path, body, resp)
}
