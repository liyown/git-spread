package spread

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyown/git-spread/internal/git"
	gh "github.com/liyown/git-spread/internal/github"
	"github.com/liyown/git-spread/internal/state"
)

type ProgressReporter interface {
	Report(state.Run, string)
}

func Execute(plan Plan, root git.Runner, store state.Store) (state.Run, error) {
	return ExecuteWithProgress(plan, root, store, nil)
}

func ExecuteWithProgress(plan Plan, root git.Runner, store state.Store, progress ProgressReporter) (state.Run, error) {
	run := state.Run{
		ID:            time.Now().UTC().Format("20060102T150405Z"),
		Kind:          string(plan.Request.Kind),
		Mode:          string(plan.Request.Mode),
		Source:        plan.Request.Source,
		Items:         plan.Request.Items,
		Commits:       plan.Commits,
		Remote:        plan.Request.Remote,
		WorkspaceDir:  plan.Request.WorkspaceDir,
		Collaboration: plan.Request.Collaboration,
		ForkRemote:    plan.Request.ForkRemote,
		HeadRemote:    plan.Request.HeadRemote,
		HeadOwner:     plan.Request.HeadOwner,
	}
	for _, target := range plan.Targets {
		run.Targets = append(run.Targets, state.Target{
			Branch:        target.Branch,
			Status:        state.StatusPending,
			WorkspacePath: target.WorkspacePath,
		})
	}
	if err := store.Save(run); err != nil {
		return run, err
	}

	for i, target := range plan.Targets {
		run.CurrentTarget = i
		run.Targets[i].Status = state.StatusRunning
		if err := store.Save(run); err != nil {
			return run, err
		}

		report := func(step string) error {
			return setTargetStep(store, &run, i, step, progress)
		}
		if err := executeTarget(plan, target, root, report); err != nil {
			if workspaceActionNeeded(err) {
				setTargetError(&run.Targets[i], state.StatusBlocked, err)
				_ = store.Save(run)
				return run, nil
			}
			conflicts, conflictErr := conflictedFiles(git.NewRunner(filepath.Join(root.Dir, target.WorkspacePath)))
			if conflictErr == nil && len(conflicts) > 0 {
				run.Targets[i].Status = state.StatusConflict
				run.Targets[i].ConflictedFiles = conflicts
				_ = store.Save(run)
				return run, nil
			}
			setTargetError(&run.Targets[i], state.StatusFailed, err)
			_ = store.Save(run)
			return run, err
		}
		if shouldPush(plan.Request) {
			if err := report("push " + target.Branch); err != nil {
				return run, err
			}
		}
		if err := pushTarget(plan, target, root); err != nil {
			setTargetError(&run.Targets[i], state.StatusRejected, err)
			_ = store.Save(run)
			return run, err
		}
		markTargetDone(&run.Targets[i])
		if err := store.Save(run); err != nil {
			return run, err
		}
	}

	if err := clearRunIfComplete(store, run); err != nil {
		return run, err
	}
	return run, nil
}

func ExecuteWithGitHub(plan Plan, root git.Runner, store state.Store, client gh.Client) (state.Run, error) {
	return ExecuteWithGitHubProgress(plan, root, store, client, nil)
}

func ExecuteWithGitHubProgress(plan Plan, root git.Runner, store state.Store, client gh.Client, progress ProgressReporter) (state.Run, error) {
	if plan.Request.Mode != ModePR {
		return ExecuteWithProgress(plan, root, store, progress)
	}
	run := state.Run{
		ID:            time.Now().UTC().Format("20060102T150405Z"),
		Kind:          string(plan.Request.Kind),
		Mode:          string(plan.Request.Mode),
		Source:        plan.Request.Source,
		Items:         plan.Request.Items,
		Commits:       plan.Commits,
		Remote:        plan.Request.Remote,
		WorkspaceDir:  plan.Request.WorkspaceDir,
		Collaboration: plan.Request.Collaboration,
		ForkRemote:    plan.Request.ForkRemote,
		HeadRemote:    plan.Request.HeadRemote,
		HeadOwner:     plan.Request.HeadOwner,
	}
	for _, target := range plan.Targets {
		run.Targets = append(run.Targets, state.Target{
			Branch:        target.Branch,
			Status:        state.StatusPending,
			WorkspacePath: target.WorkspacePath,
		})
	}
	if err := store.Save(run); err != nil {
		return run, err
	}
	for i, target := range plan.Targets {
		run.CurrentTarget = i
		run.Targets[i].Status = state.StatusRunning
		if err := store.Save(run); err != nil {
			return run, err
		}

		head := plan.Request.Source
		report := func(step string) error {
			return setTargetStep(store, &run, i, step, progress)
		}
		if plan.Request.Kind != KindBranch {
			head = propagationBranch(plan, target)
			run.Targets[i].CreatedBranch = head
			if err := executePropagationBranch(plan, target, head, root, report); err != nil {
				if workspaceActionNeeded(err) {
					setTargetError(&run.Targets[i], state.StatusBlocked, err)
					_ = store.Save(run)
					return run, nil
				}
				conflicts, conflictErr := conflictedFiles(git.NewRunner(filepath.Join(root.Dir, target.WorkspacePath)))
				if conflictErr == nil && len(conflicts) > 0 {
					run.Targets[i].Status = state.StatusConflict
					run.Targets[i].ConflictedFiles = conflicts
					_ = store.Save(run)
					return run, nil
				}
				setTargetError(&run.Targets[i], state.StatusFailed, err)
				_ = store.Save(run)
				return run, err
			}
			if shouldPushHead(plan.Request) {
				if err := report("push " + head); err != nil {
					return run, err
				}
			}
			if err := pushHead(plan, target, head, root); err != nil {
				setTargetError(&run.Targets[i], state.StatusRejected, err)
				_ = store.Save(run)
				return run, err
			}
		} else {
			if shouldPushHead(plan.Request) {
				if err := report("push " + head); err != nil {
					return run, err
				}
			}
			if err := pushBranchHead(plan, head, root); err != nil {
				setTargetError(&run.Targets[i], state.StatusRejected, err)
				_ = store.Save(run)
				return run, err
			}
		}

		if err := report("create pull request"); err != nil {
			return run, err
		}
		created, err := CreateTargetPR(client, prHead(plan.Request, head), target.Branch, fmt.Sprintf("Propagate changes to %s", target.Branch))
		if err != nil {
			setTargetError(&run.Targets[i], state.StatusFailed, err)
			_ = store.Save(run)
			return run, err
		}
		run.Targets[i].PullRequestURL = created.URL
		markTargetDone(&run.Targets[i])
		if err := store.Save(run); err != nil {
			return run, err
		}
	}
	if err := clearRunIfComplete(store, run); err != nil {
		return run, err
	}
	return run, nil
}

type stepReporter func(string) error

func executeTarget(plan Plan, target TargetPlan, root git.Runner, report stepReporter) error {
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	if err := ensureWorktree(root, target.Branch, workspace, target.Branch, report); err != nil {
		return err
	}
	w := git.NewRunner(workspace)
	switch plan.Request.Kind {
	case KindBranch:
		if err := reportStep(report, "merge "+plan.Request.Source); err != nil {
			return err
		}
		if err := w.Run("merge", "--no-edit", plan.Request.Source); err != nil {
			return err
		}
	case KindCommit, KindPR:
		if err := reportStep(report, "cherry-pick commits"); err != nil {
			return err
		}
		args := append([]string{"cherry-pick"}, plan.Commits...)
		if err := w.Run(args...); err != nil {
			return err
		}
	default:
		return fmt.Errorf("execution for %s is not available", plan.Request.Kind)
	}
	return nil
}

func executePropagationBranch(plan Plan, target TargetPlan, head string, root git.Runner, report stepReporter) error {
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	if err := ensureWorktree(root, head, workspace, target.Branch, report); err != nil {
		return err
	}
	w := git.NewRunner(workspace)
	if err := reportStep(report, "cherry-pick commits"); err != nil {
		return err
	}
	args := append([]string{"cherry-pick"}, plan.Commits...)
	return w.Run(args...)
}

func ensureWorktree(root git.Runner, branch string, workspace string, startPoint string, report stepReporter) error {
	if _, err := os.Stat(workspace); err != nil {
		if os.IsNotExist(err) {
			if err := reportStep(report, "checkout "+branch); err != nil {
				return err
			}
			return root.Run("worktree", "add", "-B", branch, workspace, startPoint)
		}
		return err
	}

	if err := reportStep(report, "check workspace "+branch); err != nil {
		return err
	}
	w := git.NewRunner(workspace)
	inside, err := w.Output("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return fmt.Errorf("isolated workspace %s exists but is not a git worktree", workspace)
	}

	clean, err := workspaceClean(w)
	if err != nil {
		return err
	}
	if !clean {
		return workspaceActionError{message: fmt.Sprintf("Workspace has uncommitted changes. Open %s, commit, stash, or discard them, then press c to continue.", workspace)}
	}

	current, err := w.Output("branch", "--show-current")
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) != branch {
		return fmt.Errorf("isolated workspace %s is on branch %q, expected %q", workspace, strings.TrimSpace(current), branch)
	}
	return nil
}

type workspaceActionError struct {
	message string
}

func (e workspaceActionError) Error() string {
	return e.message
}

func workspaceActionNeeded(err error) bool {
	var actionErr workspaceActionError
	return errors.As(err, &actionErr)
}

func pushTarget(plan Plan, target TargetPlan, root git.Runner) error {
	if plan.Request.Mode != ModeDirect || plan.Request.Remote == "" || plan.Request.Remote == "." {
		return nil
	}
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	return git.NewRunner(workspace).Run("push", plan.Request.Remote, "HEAD:"+target.Branch)
}

func pushHead(plan Plan, target TargetPlan, head string, root git.Runner) error {
	remote := headRemote(plan.Request)
	if remote == "" || remote == "." {
		return nil
	}
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	return git.NewRunner(workspace).Run("push", remote, "HEAD:"+head)
}

func pushBranchHead(plan Plan, head string, root git.Runner) error {
	remote := headRemote(plan.Request)
	if remote == "" || remote == "." {
		return nil
	}
	return root.Run("push", remote, head)
}

func shouldPush(req Request) bool {
	return req.Mode == ModeDirect && req.Remote != "" && req.Remote != "."
}

func shouldPushHead(req Request) bool {
	remote := headRemote(req)
	return remote != "" && remote != "."
}

func reportStep(report stepReporter, step string) error {
	if report == nil {
		return nil
	}
	return report(step)
}

func setTargetStep(store state.Store, run *state.Run, index int, step string, progress ProgressReporter) error {
	if index < 0 || index >= len(run.Targets) {
		return nil
	}
	run.Targets[index].Step = step
	if err := store.Save(*run); err != nil {
		return err
	}
	if progress != nil {
		progress.Report(*run, run.Targets[index].Branch+": "+step)
	}
	return nil
}

func propagationBranch(plan Plan, target TargetPlan) string {
	seed := "changes"
	if len(plan.Commits) > 0 {
		seed = plan.Commits[0]
		if len(seed) > 12 {
			seed = seed[:12]
		}
	}
	return "spread/" + sanitizeBranch(target.Branch) + "/" + seed
}

func headRemote(req Request) string {
	if req.HeadRemote != "" {
		return req.HeadRemote
	}
	return req.Remote
}

func prHead(req Request, branch string) string {
	if req.HeadOwner == "" {
		return branch
	}
	return req.HeadOwner + ":" + branch
}

func setTargetError(target *state.Target, status state.Status, err error) {
	target.Status = status
	if err != nil {
		target.Error = err.Error()
	}
}

func markTargetDone(target *state.Target) {
	target.Status = state.StatusDone
	target.Step = ""
	target.Error = ""
	target.ConflictedFiles = nil
}

func clearRunIfComplete(store state.Store, run state.Run) error {
	if !runComplete(run) {
		return nil
	}
	return store.Clear()
}

func runComplete(run state.Run) bool {
	if len(run.Targets) == 0 {
		return false
	}
	for _, target := range run.Targets {
		if target.Status != state.StatusDone {
			return false
		}
	}
	return true
}

func conflictedFiles(r git.Runner) ([]string, error) {
	out, err := r.Output("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

func CreateTargetPR(client gh.Client, head string, base string, title string) (gh.CreatedPullRequest, error) {
	return client.CreatePullRequest(gh.CreatePullRequestInput{
		Title: title,
		Head:  head,
		Base:  base,
		Body:  "Created by Git Spread.",
	})
}
