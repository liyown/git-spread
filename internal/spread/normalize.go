package spread

import (
	"errors"
	"fmt"

	"github.com/liyown/git-spread/internal/config"
)

func Normalize(input CLIInput) (Request, error) {
	cfg := input.Config
	config.ApplyDefaults(&cfg)

	if input.Task != "" {
		task, ok := cfg.Tasks[input.Task]
		if !ok {
			return Request{}, fmt.Errorf("task %q not found", input.Task)
		}
		mergeTask(&input, task)
	}

	mode := cfg.Defaults.Mode
	if input.Mode != "" {
		mode = input.Mode
	}
	if mode != string(ModeDirect) && mode != string(ModePR) {
		return Request{}, fmt.Errorf("mode %q is invalid", mode)
	}
	if len(input.Targets) == 0 {
		return Request{}, errors.New("at least one target branch is required")
	}

	req := Request{
		Kind:          input.Kind,
		Source:        input.Source,
		Items:         append([]string(nil), input.Items...),
		Targets:       append([]string(nil), input.Targets...),
		Mode:          Mode(mode),
		Remote:        cfg.Defaults.Remote,
		Workspace:     WorkspaceMode(cfg.Defaults.Workspace),
		WorkspaceDir:  cfg.Defaults.WorkspaceDir,
		Editor:        cfg.Defaults.Editor,
		Collaboration: cfg.Defaults.GitHub.Collaboration,
	}

	switch req.Kind {
	case KindBranch:
		if req.Source == "" {
			req.Source = input.CurrentBranch
		}
		if req.Source == "" {
			return Request{}, errors.New("branch source is required when current branch cannot be detected")
		}
	case KindCommit:
		if len(req.Items) == 0 {
			return Request{}, errors.New("commit mode requires at least one commit or range")
		}
	case KindPR:
		if len(req.Items) != 1 {
			return Request{}, errors.New("pr mode requires exactly one pull request number or URL")
		}
	default:
		return Request{}, fmt.Errorf("propagation type %q is invalid", req.Kind)
	}

	return req, nil
}

func mergeTask(input *CLIInput, task config.Task) {
	if input.Kind == "" && task.Type != "" {
		input.Kind = Kind(task.Type)
	}
	if input.Source == "" {
		input.Source = task.From
	}
	if len(input.Targets) == 0 {
		input.Targets = append([]string(nil), task.To...)
	}
	if input.Mode == "" {
		input.Mode = task.Mode
	}
}
