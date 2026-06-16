package spread

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
	"github.com/liyown/git-spread/internal/testutil"
)

func TestDoctorReportsMissingWorkspaceAndUnknownStatus(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	run := state.Run{
		ID:            "run-1",
		CurrentTarget: 0,
		Targets: []state.Target{
			{Branch: "main", Status: state.Status("weird"), WorkspacePath: ".spread/main"},
		},
	}

	findings := Doctor(git.NewRunner(repo.Dir), run)
	got := doctorMessages(findings)
	for _, want := range []string{`unknown target status "weird" on main`, "workspace is missing: .spread/main"} {
		if !strings.Contains(got, want) {
			t.Fatalf("findings missing %q:\n%s", want, got)
		}
	}
}

func TestDoctorReportsDirtyWorkspace(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.Write("README.md", "base\n")
	repo.Commit("initial")
	repo.Branch("release/1.0")
	workspace := filepath.Join(repo.Dir, ".spread", "release-1.0")
	if err := git.NewRunner(repo.Dir).Run("worktree", "add", "-B", "release/1.0", workspace, "release/1.0"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, workspace, "dirty.txt", "dirty\n")

	findings := Doctor(git.NewRunner(repo.Dir), state.Run{
		ID:            "run-1",
		CurrentTarget: 0,
		Targets: []state.Target{
			{Branch: "release/1.0", Status: state.StatusConflict, WorkspacePath: ".spread/release-1.0"},
		},
	})
	got := doctorMessages(findings)
	if !strings.Contains(got, "workspace has uncommitted changes: .spread/release-1.0") {
		t.Fatalf("findings = %s", got)
	}
}

func doctorMessages(findings []DoctorFinding) string {
	var messages []string
	for _, finding := range findings {
		messages = append(messages, finding.Message)
	}
	return strings.Join(messages, "\n")
}
