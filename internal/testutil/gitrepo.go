package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/git"
)

type GitRepo struct {
	t   *testing.T
	Dir string
	git git.Runner
}

func NewGitRepo(t *testing.T) *GitRepo {
	t.Helper()
	dir := t.TempDir()
	r := git.NewRunner(dir)
	run(t, r, "init", "-b", "main")
	run(t, r, "config", "user.email", "test@example.com")
	run(t, r, "config", "user.name", "Git Spread Test")
	return &GitRepo{t: t, Dir: dir, git: r}
}

func (r *GitRepo) Write(path string, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *GitRepo) Commit(message string) {
	r.t.Helper()
	run(r.t, r.git, "add", ".")
	run(r.t, r.git, "commit", "-m", message)
}

func (r *GitRepo) Branch(name string) {
	r.t.Helper()
	run(r.t, r.git, "branch", name)
}

func (r *GitRepo) Checkout(args ...string) {
	r.t.Helper()
	run(r.t, r.git, append([]string{"checkout"}, args...)...)
}

func (r *GitRepo) CurrentBranch() string {
	r.t.Helper()
	out, err := r.git.Output("branch", "--show-current")
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func (r *GitRepo) Head() string {
	r.t.Helper()
	out, err := r.git.Output("rev-parse", "HEAD")
	if err != nil {
		r.t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func run(t *testing.T, r git.Runner, args ...string) {
	t.Helper()
	if err := r.Run(args...); err != nil {
		t.Fatal(err)
	}
}
