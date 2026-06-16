package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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
	Task          string   `json:"task,omitempty"`
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
	PRTitle       string   `json:"prTitle,omitempty"`
	PRBody        string   `json:"prBody,omitempty"`
	PRDraft       bool     `json:"prDraft,omitempty"`
	PRLabels      []string `json:"prLabels,omitempty"`
	PRReviewers   []string `json:"prReviewers,omitempty"`
	Targets       []Target `json:"targets"`
	CurrentTarget int      `json:"currentTarget"`
}

type HistoryEntry struct {
	RecordedAt time.Time      `json:"recordedAt"`
	Run        Run            `json:"run"`
	Summary    map[Status]int `json:"summary"`
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

func (s Store) HistoryPath() string {
	return filepath.Join(s.dir, "history.jsonl")
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

func (s Store) AppendHistory(run Run) error {
	if run.ID == "" || len(run.Targets) == 0 {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	entry := HistoryEntry{
		RecordedAt: time.Now().UTC(),
		Run:        run,
		Summary:    summarizeRun(run),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.HistoryPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s Store) History(limit int) ([]HistoryEntry, error) {
	file, err := os.Open(s.HistoryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []HistoryEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry HistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		if entry.Summary == nil {
			entry.Summary = summarizeRun(entry.Run)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func summarizeRun(run Run) map[Status]int {
	summary := map[Status]int{}
	for _, target := range run.Targets {
		summary[target.Status]++
	}
	return summary
}
