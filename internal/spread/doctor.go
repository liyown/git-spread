package spread

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
)

type DoctorFinding struct {
	Target  string
	Message string
}

func Doctor(root git.Runner, run state.Run) []DoctorFinding {
	var findings []DoctorFinding
	if len(run.Targets) == 0 {
		findings = append(findings, DoctorFinding{Message: "run has no targets"})
	}
	if run.CurrentTarget < 0 || run.CurrentTarget >= len(run.Targets) {
		findings = append(findings, DoctorFinding{Message: "current target is outside target list"})
	}
	for _, target := range run.Targets {
		if !knownRunStatus(target.Status) {
			findings = append(findings, DoctorFinding{Target: target.Branch, Message: fmt.Sprintf("unknown target status %q on %s", target.Status, branchOrDash(target.Branch))})
		}
		if target.WorkspacePath == "" || target.Status == state.StatusPending || target.Status == state.StatusDone {
			continue
		}
		findings = append(findings, inspectWorkspace(root, target)...)
	}
	return findings
}

func inspectWorkspace(root git.Runner, target state.Target) []DoctorFinding {
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	if _, err := os.Stat(workspace); err != nil {
		if os.IsNotExist(err) {
			return []DoctorFinding{{Target: target.Branch, Message: "workspace is missing: " + target.WorkspacePath}}
		}
		return []DoctorFinding{{Target: target.Branch, Message: err.Error()}}
	}
	runner := git.NewRunner(workspace)
	inside, err := runner.Output("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return []DoctorFinding{{Target: target.Branch, Message: "workspace is not a git worktree: " + target.WorkspacePath}}
	}
	var findings []DoctorFinding
	current, err := runner.Output("branch", "--show-current")
	if err != nil {
		findings = append(findings, DoctorFinding{Target: target.Branch, Message: err.Error()})
	} else if branch := strings.TrimSpace(current); branch != "" && branch != target.Branch && target.CreatedBranch == "" {
		findings = append(findings, DoctorFinding{Target: target.Branch, Message: fmt.Sprintf("workspace is on branch %q, expected %q", branch, target.Branch)})
	}
	status, err := runner.Output("status", "--porcelain")
	if err != nil {
		findings = append(findings, DoctorFinding{Target: target.Branch, Message: err.Error()})
	} else if strings.TrimSpace(status) != "" {
		findings = append(findings, DoctorFinding{Target: target.Branch, Message: "workspace has uncommitted changes: " + target.WorkspacePath})
	}
	return findings
}

func knownRunStatus(status state.Status) bool {
	switch status {
	case state.StatusPending, state.StatusRunning, state.StatusDone, state.StatusConflict, state.StatusBlocked, state.StatusRejected, state.StatusFailed:
		return true
	default:
		return false
	}
}

func branchOrDash(branch string) string {
	if branch == "" {
		return "-"
	}
	return branch
}
