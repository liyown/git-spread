package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusConflict Status = "conflict"
	StatusBlocked  Status = "blocked"
	StatusRejected Status = "rejected"
	StatusFailed   Status = "failed"
)

type Run struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Mode          string   `json:"mode"`
	Source        string   `json:"source,omitempty"`
	Items         []string `json:"items,omitempty"`
	Commits       []string `json:"commits,omitempty"`
	Remote        string   `json:"remote,omitempty"`
	WorkspaceDir  string   `json:"workspaceDir,omitempty"`
	Collaboration string   `json:"collaboration,omitempty"`
	ForkRemote    string   `json:"forkRemote,omitempty"`
	HeadRemote    string   `json:"headRemote,omitempty"`
	HeadOwner     string   `json:"headOwner,omitempty"`
	Targets       []Target `json:"targets"`
	CurrentTarget int      `json:"currentTarget"`
}

type Target struct {
	Branch          string   `json:"branch"`
	Status          Status   `json:"status"`
	Step            string   `json:"step,omitempty"`
	WorkspacePath   string   `json:"workspacePath"`
	ConflictedFiles []string `json:"conflictedFiles,omitempty"`
	CreatedBranch   string   `json:"createdBranch,omitempty"`
	PullRequestURL  string   `json:"pullRequestURL,omitempty"`
	Error           string   `json:"error,omitempty"`
}

type Store struct {
	dir string
}

func NewStore(dir string) Store {
	return Store{dir: dir}
}

func (s Store) Path() string {
	return filepath.Join(s.dir, "state.json")
}

func (s Store) Save(run Run) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path(), append(data, '\n'), 0o644)
}

func (s Store) Load() (Run, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return Run{}, err
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s Store) Clear() error {
	err := os.Remove(s.Path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
