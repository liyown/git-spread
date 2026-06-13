package config

import (
	"os"
	"path/filepath"
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
}

func TestLoadTasks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".git-spread.yml")
	err := os.WriteFile(path, []byte(`
version: 1
tasks:
  release:
    type: branch
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
	if release.Type != "branch" || release.From != "develop" || len(release.To) != 2 {
		t.Fatalf("release task = %#v", release)
	}
	backport := cfg.Tasks["backport"]
	if backport.Type != "commit" || backport.Mode != "pr" || len(backport.To) != 1 {
		t.Fatalf("backport task = %#v", backport)
	}
}
