package spread

import "github.com/liyown/git-spread/internal/config"

type Kind string

const (
	KindBranch Kind = "branch"
	KindCommit Kind = "commit"
	KindPR     Kind = "pr"
)

type Mode string

const (
	ModeDirect Mode = "direct"
	ModePR     Mode = "pr"
)

type WorkspaceMode string

const (
	WorkspaceIsolated WorkspaceMode = "isolated"
	WorkspaceCurrent  WorkspaceMode = "current"
)

type CLIInput struct {
	Kind          Kind
	Source        string
	Items         []string
	Targets       []string
	Mode          string
	Task          string
	Last          bool
	Retry         bool
	CurrentBranch string
	Config        config.Config
}

type Request struct {
	Task          string
	Kind          Kind
	Source        string
	Items         []string
	Targets       []string
	Mode          Mode
	Remote        string
	Workspace     WorkspaceMode
	WorkspaceDir  string
	Editor        string
	Collaboration string
	ForkRemote    string
	HeadRemote    string
	HeadOwner     string
	PRTitle       string
	PRBody        string
	PRDraft       bool
	PRLabels      []string
	PRReviewers   []string
}
