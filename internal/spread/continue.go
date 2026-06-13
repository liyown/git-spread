package spread

import (
	"errors"
	"path/filepath"

	"github.com/liyown/git-spread/internal/git"
	gh "github.com/liyown/git-spread/internal/github"
	"github.com/liyown/git-spread/internal/state"
)

func Continue(root git.Runner, store state.Store) (state.Run, error) {
	return continueRun(root, store, nil)
}

func ContinueWithGitHub(root git.Runner, store state.Store, client gh.Client) (state.Run, error) {
	return continueRun(root, store, client)
}

func continueRun(root git.Runner, store state.Store, client gh.Client) (state.Run, error) {
	run, err := store.Load()
	if err != nil {
		return state.Run{}, err
	}
	if run.CurrentTarget < 0 || run.CurrentTarget >= len(run.Targets) {
		return run, errors.New("current target is outside target list")
	}

	plan := planFromRun(run)
	if err := completeTarget(root, store, plan, &run, run.CurrentTarget, client); err != nil {
		return run, err
	}
	if run.Targets[run.CurrentTarget].Status != state.StatusDone {
		return run, nil
	}
	for i := run.CurrentTarget + 1; i < len(run.Targets); i++ {
		run.CurrentTarget = i
		run.Targets[i].Status = state.StatusRunning
		if err := store.Save(run); err != nil {
			return run, err
		}
		if err := executeContinuingTarget(root, plan, TargetPlan{Branch: run.Targets[i].Branch, WorkspacePath: run.Targets[i].WorkspacePath}, client); err != nil {
			conflicts, conflictErr := conflictedFiles(git.NewRunner(filepath.Join(root.Dir, run.Targets[i].WorkspacePath)))
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
		if err := finishPropagatedTarget(root, plan, &run, i, client); err != nil {
			_ = store.Save(run)
			return run, err
		}
		if err := store.Save(run); err != nil {
			return run, err
		}
	}
	return run, nil
}

func completeTarget(root git.Runner, store state.Store, plan Plan, run *state.Run, index int, client gh.Client) error {
	target := &run.Targets[index]
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	w := git.NewRunner(workspace)
	conflicts, err := conflictedFiles(w)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		target.ConflictedFiles = conflicts
		target.Status = state.StatusConflict
		return store.Save(*run)
	}

	clean, err := workspaceClean(w)
	if err != nil {
		return err
	}
	if !clean {
		if err := finishInProgressOperation(w); err != nil {
			return err
		}
	}

	if err := finishPropagatedTarget(root, plan, run, index, client); err != nil {
		_ = store.Save(*run)
		return err
	}
	return store.Save(*run)
}

func executeContinuingTarget(root git.Runner, plan Plan, target TargetPlan, client gh.Client) error {
	if plan.Request.Mode == ModePR {
		if plan.Request.Kind == KindBranch {
			return nil
		}
		head := propagationBranch(plan, target)
		return executePropagationBranch(plan, target, head, root)
	}
	return executeTarget(plan, target, root)
}

func finishPropagatedTarget(root git.Runner, plan Plan, run *state.Run, index int, client gh.Client) error {
	target := TargetPlan{Branch: run.Targets[index].Branch, WorkspacePath: run.Targets[index].WorkspacePath}
	switch plan.Request.Mode {
	case ModeDirect:
		if err := pushTarget(plan, target, root); err != nil {
			setTargetError(&run.Targets[index], state.StatusRejected, err)
			return err
		}
	case ModePR:
		if client == nil {
			return errors.New("GitHub client is required to continue PR mode")
		}
		head := run.Targets[index].CreatedBranch
		if head == "" {
			head = plan.Request.Source
		}
		if plan.Request.Kind == KindBranch {
			if err := pushBranchHead(plan, head, root); err != nil {
				setTargetError(&run.Targets[index], state.StatusRejected, err)
				return err
			}
		} else {
			if head == "" {
				head = propagationBranch(plan, target)
			}
			run.Targets[index].CreatedBranch = head
			if err := pushHead(plan, target, head, root); err != nil {
				setTargetError(&run.Targets[index], state.StatusRejected, err)
				return err
			}
		}
		created, err := CreateTargetPR(client, prHead(plan.Request, head), target.Branch, "Propagate changes to "+target.Branch)
		if err != nil {
			setTargetError(&run.Targets[index], state.StatusFailed, err)
			return err
		}
		run.Targets[index].PullRequestURL = created.URL
	}
	markTargetDone(&run.Targets[index])
	return nil
}

func planFromRun(run state.Run) Plan {
	req := Request{
		Kind:          Kind(run.Kind),
		Mode:          Mode(run.Mode),
		Source:        run.Source,
		Items:         run.Items,
		Remote:        run.Remote,
		WorkspaceDir:  run.WorkspaceDir,
		Collaboration: run.Collaboration,
		ForkRemote:    run.ForkRemote,
		HeadRemote:    run.HeadRemote,
		HeadOwner:     run.HeadOwner,
	}
	return Plan{Request: req, Commits: run.Commits}
}

func Abort(store state.Store) error {
	return store.Clear()
}

func workspaceClean(r git.Runner) (bool, error) {
	out, err := r.Output("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

func finishInProgressOperation(r git.Runner) error {
	if err := r.Run("cherry-pick", "--continue"); err == nil {
		return nil
	}
	if err := r.Run("merge", "--continue"); err == nil {
		return nil
	}
	return nil
}
