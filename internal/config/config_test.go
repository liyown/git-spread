package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte("version: 1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Defaults.Mode != "direct" {
		t.Fatalf("mode = %q, want direct", cfg.Defaults.Mode)
	}
	if cfg.Defaults.Remote != "origin" {
		t.Fatalf("remote = %q, want origin", cfg.Defaults.Remote)
	}
	if cfg.Defaults.Workspace != "isolated" {
		t.Fatalf("workspace = %q, want isolated", cfg.Defaults.Workspace)
	}
	if cfg.Defaults.WorkspaceDir != ".spread" {
		t.Fatalf("workspace dir = %q, want .spread", cfg.Defaults.WorkspaceDir)
	}
	if cfg.Defaults.Editor != "auto" {
		t.Fatalf("editor = %q, want auto", cfg.Defaults.Editor)
	}
	if cfg.Defaults.GitHub.Collaboration != "auto" {
		t.Fatalf("collaboration = %q, want auto", cfg.Defaults.GitHub.Collaboration)
	}
	if cfg.Defaults.GitHub.ForkRemote != "fork" {
		t.Fatalf("fork remote = %q, want fork", cfg.Defaults.GitHub.ForkRemote)
	}
	if cfg.Defaults.GitHub.PRTitle == "" || cfg.Defaults.GitHub.PRBody == "" {
		t.Fatalf("github PR templates should have defaults: %#v", cfg.Defaults.GitHub)
	}
}

func TestLoadTasks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte(`
version: 1
tasks:
  release:
    type: branch
    description: Move develop into release train branches
    group: release
    from: develop
    to:
      - release/*
      - main
  backport:
    type: commit
    to:
      - release/*
    mode: pr
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	release := cfg.Tasks["release"]
	if release.Type != "branch" || release.From != "develop" || release.Description != "Move develop into release train branches" || release.Group != "release" || len(release.To) != 2 {
		t.Fatalf("release task = %#v", release)
	}
	backport := cfg.Tasks["backport"]
	if backport.Type != "commit" || backport.Mode != "pr" || len(backport.To) != 1 {
		t.Fatalf("backport task = %#v", backport)
	}
}

func TestLoadRejectsDuplicateTaskNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte(`
version: 1
tasks:
  release:
    type: branch
    from: develop
    to:
      - main
  release:
    type: branch
    from: develop
    to:
      - release/1.0
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadFile(path)
	if err == nil {
		t.Fatal("expected duplicate task error")
	}
	if !strings.Contains(err.Error(), `duplicate task name "release"`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadGitHubPRDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte(`
version: 1
defaults:
  github:
    prTitle: "Backport {source} to {target}"
    prBody: "Created for {target}"
    draft: true
    labels:
      - backport
    reviewers:
      - octocat
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gh := cfg.Defaults.GitHub
	if gh.PRTitle != "Backport {source} to {target}" || gh.PRBody != "Created for {target}" || !gh.Draft {
		t.Fatalf("github defaults = %#v", gh)
	}
	if len(gh.Labels) != 1 || gh.Labels[0] != "backport" || len(gh.Reviewers) != 1 || gh.Reviewers[0] != "octocat" {
		t.Fatalf("github defaults = %#v", gh)
	}
}
