package spread

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyown/git-spread/internal/git"
	gh "github.com/liyown/git-spread/internal/github"
	"github.com/liyown/git-spread/internal/state"
)

func Execute(plan Plan, root git.Runner, store state.Store) (state.Run, error) {
	run := state.Run{
		ID:     time.Now().UTC().Format("20060102T150405Z"),
		Kind:   string(plan.Request.Kind),
		Mode:   string(plan.Request.Mode),
		Source: plan.Request.Source,
		Items:  plan.Request.Items,
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

		if err := executeTarget(plan, target, root); err != nil {
			conflicts, conflictErr := conflictedFiles(git.NewRunner(filepath.Join(root.Dir, target.WorkspacePath)))
			if conflictErr == nil && len(conflicts) > 0 {
				run.Targets[i].Status = state.StatusConflict
				run.Targets[i].ConflictedFiles = conflicts
				_ = store.Save(run)
				return run, nil
			}
			run.Targets[i].Status = state.StatusFailed
			_ = store.Save(run)
			return run, err
		}
		if err := pushTarget(plan, target, root); err != nil {
			run.Targets[i].Status = state.StatusRejected
			_ = store.Save(run)
			return run, err
		}
		run.Targets[i].Status = state.StatusDone
		if err := store.Save(run); err != nil {
			return run, err
		}
	}

	return run, nil
}

func ExecuteWithGitHub(plan Plan, root git.Runner, store state.Store, client gh.Client) (state.Run, error) {
	if plan.Request.Mode != ModePR {
		return Execute(plan, root, store)
	}
	run := state.Run{
		ID:     time.Now().UTC().Format("20060102T150405Z"),
		Kind:   string(plan.Request.Kind),
		Mode:   string(plan.Request.Mode),
		Source: plan.Request.Source,
		Items:  plan.Request.Items,
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
		if plan.Request.Kind != KindBranch {
			head = propagationBranch(plan, target)
			run.Targets[i].CreatedBranch = head
			if err := executePropagationBranch(plan, target, head, root); err != nil {
				conflicts, conflictErr := conflictedFiles(git.NewRunner(filepath.Join(root.Dir, target.WorkspacePath)))
				if conflictErr == nil && len(conflicts) > 0 {
					run.Targets[i].Status = state.StatusConflict
					run.Targets[i].ConflictedFiles = conflicts
					_ = store.Save(run)
					return run, nil
				}
				run.Targets[i].Status = state.StatusFailed
				_ = store.Save(run)
				return run, err
			}
			if err := pushHead(plan, target, head, root); err != nil {
				run.Targets[i].Status = state.StatusRejected
				_ = store.Save(run)
				return run, err
			}
		} else if err := pushBranchHead(plan, head, root); err != nil {
			run.Targets[i].Status = state.StatusRejected
			_ = store.Save(run)
			return run, err
		}

		created, err := CreateTargetPR(client, head, target.Branch, fmt.Sprintf("Propagate changes to %s", target.Branch))
		if err != nil {
			run.Targets[i].Status = state.StatusFailed
			_ = store.Save(run)
			return run, err
		}
		run.Targets[i].PullRequestURL = created.URL
		run.Targets[i].Status = state.StatusDone
		if err := store.Save(run); err != nil {
			return run, err
		}
	}
	return run, nil
}

func executeTarget(plan Plan, target TargetPlan, root git.Runner) error {
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	if err := root.Run("worktree", "add", "-B", target.Branch, workspace, target.Branch); err != nil {
		return err
	}
	w := git.NewRunner(workspace)
	switch plan.Request.Kind {
	case KindBranch:
		if err := w.Run("merge", "--no-edit", plan.Request.Source); err != nil {
			return err
		}
	case KindCommit, KindPR:
		args := append([]string{"cherry-pick"}, plan.Commits...)
		if err := w.Run(args...); err != nil {
			return err
		}
	default:
		return fmt.Errorf("execution for %s is not available", plan.Request.Kind)
	}
	return nil
}

func executePropagationBranch(plan Plan, target TargetPlan, head string, root git.Runner) error {
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	if err := root.Run("worktree", "add", "-B", head, workspace, target.Branch); err != nil {
		return err
	}
	w := git.NewRunner(workspace)
	args := append([]string{"cherry-pick"}, plan.Commits...)
	return w.Run(args...)
}

func pushTarget(plan Plan, target TargetPlan, root git.Runner) error {
	if plan.Request.Mode != ModeDirect || plan.Request.Remote == "" || plan.Request.Remote == "." {
		return nil
	}
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	return git.NewRunner(workspace).Run("push", plan.Request.Remote, "HEAD:"+target.Branch)
}

func pushHead(plan Plan, target TargetPlan, head string, root git.Runner) error {
	if plan.Request.Remote == "" || plan.Request.Remote == "." {
		return nil
	}
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	return git.NewRunner(workspace).Run("push", plan.Request.Remote, "HEAD:"+head)
}

func pushBranchHead(plan Plan, head string, root git.Runner) error {
	if plan.Request.Remote == "" || plan.Request.Remote == "." {
		return nil
	}
	return root.Run("push", plan.Request.Remote, head)
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
