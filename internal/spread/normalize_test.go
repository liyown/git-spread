package spread

import (
	"testing"

	"github.com/liyown/git-spread/internal/config"
)

func TestNormalizeBranchUsesCurrentBranch(t *testing.T) {
	input := CLIInput{
		Kind:          KindBranch,
		Targets:       []string{"release/1.0"},
		CurrentBranch: "feature/login-fix",
		Config:        config.Config{},
	}

	req, err := Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	if req.Source != "feature/login-fix" {
		t.Fatalf("source = %q, want current branch", req.Source)
	}
	if req.Mode != ModeDirect {
		t.Fatalf("mode = %q, want direct", req.Mode)
	}
}

func TestNormalizeTaskFromAutoUsesCurrentBranch(t *testing.T) {
	cfg := config.Config{
		Tasks: map[string]config.Task{
			"release": {
				Type: "branch",
				From: "auto",
				To:   []string{"release/*", "main"},
			},
		},
	}
	req, err := Normalize(CLIInput{Task: "release", CurrentBranch: "feature/login-fix", Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if req.Source != "feature/login-fix" {
		t.Fatalf("source = %q, want current branch", req.Source)
	}
}

func TestNormalizeCommitRequiresInput(t *testing.T) {
	_, err := Normalize(CLIInput{Kind: KindCommit, Targets: []string{"release/1.0"}})
	if err == nil {
		t.Fatal("expected error for missing commit input")
	}
}

func TestNormalizePRRequiresInput(t *testing.T) {
	_, err := Normalize(CLIInput{Kind: KindPR, Targets: []string{"release/1.0"}})
	if err == nil {
		t.Fatal("expected error for missing pull request input")
	}
}

func TestNormalizePRURLUsesNumber(t *testing.T) {
	req, err := Normalize(CLIInput{Kind: KindPR, Items: []string{"https://github.com/acme/app/pull/123/"}, Targets: []string{"release/1.0"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Items) != 1 || req.Items[0] != "123" {
		t.Fatalf("items = %#v, want PR number", req.Items)
	}
}

func TestNormalizeRejectsUnknownGitHubCollaboration(t *testing.T) {
	cfg := config.Config{}
	config.ApplyDefaults(&cfg)
	cfg.Defaults.GitHub.Collaboration = "magic"
	_, err := Normalize(CLIInput{Kind: KindBranch, Source: "develop", Targets: []string{"main"}, Config: cfg})
	if err == nil {
		t.Fatal("expected error for unknown collaboration mode")
	}
}

func TestNormalizeTaskAppliesTaskDefaults(t *testing.T) {
	cfg := config.Config{
		Tasks: map[string]config.Task{
			"release": {
				Type: "branch",
				From: "develop",
				To:   []string{"release/*", "main"},
				Mode: "pr",
			},
		},
	}
	req, err := Normalize(CLIInput{Task: "release", Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if req.Kind != KindBranch || req.Source != "develop" || req.Mode != ModePR || len(req.Targets) != 2 {
		t.Fatalf("request = %#v", req)
	}
}
