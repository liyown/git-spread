package editor

import (
	"errors"
	"fmt"
	"os/exec"
)

var ErrNotFound = errors.New("editor not found")

var known = map[string]string{
	"code":   "code",
	"idea":   "idea",
	"cursor": "cursor",
}

func Command(name string, workspace string) (string, []string, error) {
	return CommandWithLookup(name, workspace, func(command string) (string, error) {
		path, err := exec.LookPath(command)
		if err != nil {
			return "", ErrNotFound
		}
		return path, nil
	})
}

func CommandWithLookup(name string, workspace string, lookup func(string) (string, error)) (string, []string, error) {
	if name == "" || name == "auto" {
		for _, candidate := range []string{"code", "idea", "cursor"} {
			if path, err := lookup(candidate); err == nil {
				return path, []string{workspace}, nil
			}
		}
		return "", nil, fmt.Errorf("no supported editor found; open %s manually", workspace)
	}
	cmd, ok := known[name]
	if !ok {
		return "", nil, fmt.Errorf("editor %q is not supported", name)
	}
	path, err := lookup(cmd)
	if err != nil {
		return "", nil, fmt.Errorf("editor %q not found in PATH", name)
	}
	return path, []string{workspace}, nil
}
