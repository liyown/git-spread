package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/spread"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 || stdout.String() != "git-spread dev\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCommitRequiresInput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"commit", "--to", "release/1.0", "--no-tui"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "commit mode requires") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestPlanPrintsDryRunHeader(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"plan", "branch", "develop", "--to", "main", "--no-tui"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Plan") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitDryRunWritesConfigTemplate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"init", "--print"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"version: 1", "mode: direct", "tasks:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestConfigurePRHeadUsesForkRemoteOwner(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := git.NewRunner(repo.Dir).Run("remote", "add", "fork", "https://github.com/me/example.git"); err != nil {
		t.Fatal(err)
	}

	req := spread.Request{Collaboration: "fork", ForkRemote: "fork"}
	if err := configurePRHead(&req, git.NewRunner(repo.Dir)); err != nil {
		t.Fatal(err)
	}
	if req.HeadRemote != "fork" || req.HeadOwner != "me" {
		t.Fatalf("request = %#v", req)
	}
}

func TestLoadRepoContextFromWorktreeUsesMainRepositoryState(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	workspace := filepath.Join(repo.Dir, ".spread", "release-1.0")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	ctx, err := loadRepoContext()
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, err := filepath.EvalSymlinks(ctx.root)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("root = %q, want %q", gotRoot, wantRoot)
	}
	gotStore, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(ctx.store.Path())))
	if err != nil {
		t.Fatal(err)
	}
	wantStore, err := filepath.EvalSymlinks(filepath.Join(repo.Dir, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if gotStore != wantStore || filepath.Base(filepath.Dir(ctx.store.Path())) != "spread" {
		t.Fatalf("store path = %q", ctx.store.Path())
	}
}
