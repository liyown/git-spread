package spread

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/liyown/git-spread/internal/git"
	gh "github.com/liyown/git-spread/internal/github"
)

type Plan struct {
	Request Request
	Targets []TargetPlan
	Commits []string
}

type TargetPlan struct {
	Branch        string
	WorkspacePath string
}

func BuildPlan(req Request, runner git.Runner) (Plan, error) {
	targets, err := resolveTargets(req.Targets, runner)
	if err != nil {
		return Plan{}, err
	}
	commits, err := resolveCommits(req, runner)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Request: req, Commits: commits}
	for _, target := range targets {
		plan.Targets = append(plan.Targets, TargetPlan{
			Branch:        target,
			WorkspacePath: filepath.Join(req.WorkspaceDir, sanitizeBranch(target)),
		})
	}
	return plan, nil
}

func BuildPlanWithGitHub(req Request, runner git.Runner, client gh.Client) (Plan, error) {
	if req.Kind != KindPR {
		return BuildPlan(req, runner)
	}
	pr, err := client.PullRequest(req.Items[0])
	if err != nil {
		return Plan{}, err
	}
	reqForPlan := req
	reqForPlan.Kind = KindCommit
	reqForPlan.Items = pr.Commits
	plan, err := BuildPlan(reqForPlan, runner)
	if err != nil {
		return Plan{}, err
	}
	plan.Request = req
	plan.Commits = pr.Commits
	return plan, nil
}

func resolveTargets(patterns []string, runner git.Runner) ([]string, error) {
	out, err := runner.Output("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	branches := strings.Fields(out)
	var targets []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		if !strings.Contains(pattern, "*") {
			if !seen[pattern] {
				targets = append(targets, pattern)
				seen[pattern] = true
			}
			continue
		}
		for _, branch := range branches {
			ok, err := filepath.Match(pattern, branch)
			if err != nil {
				return nil, err
			}
			if ok && !seen[branch] {
				targets = append(targets, branch)
				seen[branch] = true
			}
		}
	}
	sort.Strings(targets)
	return targets, nil
}

func resolveCommits(req Request, runner git.Runner) ([]string, error) {
	if req.Kind != KindCommit {
		return nil, nil
	}
	var commits []string
	for _, item := range req.Items {
		if strings.Contains(item, "..") {
			out, err := runner.Output("rev-list", "--reverse", item)
			if err != nil {
				return nil, err
			}
			commits = append(commits, strings.Fields(out)...)
			continue
		}
		commits = append(commits, item)
	}
	return commits, nil
}

func sanitizeBranch(branch string) string {
	return strings.NewReplacer("/", "-", "\\", "-").Replace(branch)
}
