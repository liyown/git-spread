package spread

import (
	"errors"
	"path/filepath"

	"github.com/liyown/git-spread/internal/git"
	"github.com/liyown/git-spread/internal/state"
)

func Continue(root git.Runner, store state.Store) (state.Run, error) {
	run, err := store.Load()
	if err != nil {
		return state.Run{}, err
	}
	if run.CurrentTarget < 0 || run.CurrentTarget >= len(run.Targets) {
		return run, errors.New("current target is outside target list")
	}

	target := &run.Targets[run.CurrentTarget]
	workspace := filepath.Join(root.Dir, target.WorkspacePath)
	w := git.NewRunner(workspace)

	conflicts, err := conflictedFiles(w)
	if err != nil {
		return run, err
	}
	if len(conflicts) > 0 {
		target.ConflictedFiles = conflicts
		target.Status = state.StatusConflict
		return run, store.Save(run)
	}

	clean, err := workspaceClean(w)
	if err != nil {
		return run, err
	}
	if !clean {
		if err := finishInProgressOperation(w); err != nil {
			return run, err
		}
	}

	target.Status = state.StatusDone
	if err := store.Save(run); err != nil {
		return run, err
	}
	return run, nil
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
