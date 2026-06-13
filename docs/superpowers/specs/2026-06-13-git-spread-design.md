# Git Spread v1 Design

Date: 2026-06-13

## Product Boundary

Git Spread v1 is a change propagation CLI for Git repositories.

It helps users apply the same branch, commit set, or pull request changes to multiple target branches. It is not a release approval platform, deployment system, or CI orchestrator.

The first version includes:

- branch propagation
- commit propagation
- pull request propagation
- direct push mode
- GitHub pull request mode
- isolated target workspaces
- TUI control panel
- conflict pause, editor handoff, continue, and abort

The first version does not include:

- release train approvals
- environment promotion policy
- AI conflict resolution
- automatic semantic safety checks for branch contents
- hosted service state

## User Mental Model

Users only need four concepts:

- what: `branch`, `commit`, or `pr`
- where: target branches or branch patterns
- how: `direct` or `pr`
- state: `continue` or `abort`

The tool should avoid exposing Git worktree terminology in normal output. It should use "isolated workspace" in user-facing text.

## Primary API

Human users can start with the TUI:

```bash
git spread
```

The TUI lets users choose a configured task, inspect progress, open a conflicted workspace in an editor, continue, abort, or switch a blocked target to PR mode.

Scriptable commands remain available:

```bash
git spread init
git spread run <task>

git spread branch [source] --to <targets> [--mode direct|pr]
git spread commit <commit-or-range>... --to <targets> [--mode direct|pr]
git spread pr <number-or-url> --to <targets> [--mode direct|pr]

git spread plan <same propagation args>
git spread status
git spread open [--editor code|idea|cursor]
git spread continue
git spread abort
```

Commands that run on an interactive terminal may show the TUI by default. Automation can use `--no-tui`.

## Propagation Inputs

### Branch

Branch propagation applies a source branch to one or more target branches.

If the source branch is omitted, Git Spread uses the current branch:

```bash
git spread branch --to release/1.0 --mode pr
```

This is equivalent to:

```bash
git spread branch <current-branch> --to release/1.0 --mode pr
```

Direct mode merges the source branch into each target branch, then pushes the target branch.

PR mode opens pull requests from the source branch to each target branch. If the source branch exists only locally, Git Spread pushes that branch to the selected head repository first. This intentionally does not try to inspect whether the source branch contains unrelated commits. Users choose branch mode when they want branch-level propagation.

### Commit

Commit propagation applies only the commits explicitly provided by the user.

```bash
git spread commit abc123 --to release/1.0
git spread commit abc123 def456 --to release/1.0,release/1.1
git spread commit main..feature/login-fix --to release/1.0 --mode pr
```

Single commits are cherry-picked in the order supplied.

Ranges are expanded with Git in chronological order, equivalent to:

```bash
git rev-list --reverse <range>
```

Commit mode must not guess commits from the current branch. If no commit or range is provided, the command fails with a clear message.

In PR mode, Git Spread creates a propagation branch for each target, cherry-picks the selected commits, pushes that branch, and opens a pull request.

### Pull Request

Pull request propagation treats an existing pull request as a set of commits.

```bash
git spread pr 123 --to release/1.0,release/1.1
git spread pr https://github.com/acme/app/pull/123 --to release/*
```

Git Spread reads the commits from the source PR, preserves their order, and applies them to each target.

PR mode for pull request propagation creates a propagation branch per target, pushes it, and opens a pull request to the target branch.

Pull request input is required. Git Spread v1 does not guess the current branch's associated pull request.

## Target Resolution

Targets may be explicit branch names or simple patterns:

```bash
--to release/1.0,release/1.1,main
--to release/*
```

Patterns resolve against branches from the configured remote. `git spread plan` must show the exact branches before execution:

```text
Targets:
  release/1.0
  release/1.1
  main
```

## Configuration

The repository config file is `.git-spread.yml`.

```yaml
version: 1

defaults:
  mode: direct
  remote: origin
  workspace: isolated
  workspaceDir: .spread
  editor: auto
  github:
    collaboration: auto

tasks:
  release:
    type: branch
    from: develop
    to:
      - release/*
      - main

  backport:
    type: commit
    to:
      - release/*
    mode: pr

  backport-pr:
    type: pr
    to:
      - release/*
    mode: pr
```

`git spread run <task>` loads the task, then CLI flags may override task values:

```bash
git spread run release --mode pr
git spread commit abc123 --task backport
git spread pr 123 --task backport-pr
```

## Execution Modes

### Direct Mode

Direct mode is the default.

For each target branch, Git Spread applies the requested changes in an isolated workspace and pushes the target branch to the configured remote.

If the push fails because of branch protection, missing permission, or remote rejection, the run pauses. The TUI and CLI should show the rejected target and offer PR mode for that target. Git Spread should not silently fallback to PR mode.

### PR Mode

PR mode opens GitHub pull requests.

Branch propagation uses the source branch as the pull request head. If the source branch is local-only, Git Spread pushes it to the selected head repository before opening the pull request.

Commit and pull request propagation create a propagation branch per target, because a GitHub pull request head must be a branch.

GitHub collaboration mode:

- `shared`: push heads to the same repository
- `fork`: push heads to the user's fork
- `auto`: use shared repository when push is possible, otherwise use a fork

The user-facing explanation is:

```text
Use the shared repository when possible. Otherwise use your fork.
```

## Workspace Model

The default workspace mode is `isolated`.

Git Spread creates target workspaces under `.spread/`:

```text
.spread/
  release-1.0/
  release-1.1/
  main/
```

These workspaces share Git object storage with the main repository. They are not full clones.

Normal users should not need to know that the implementation uses Git worktrees. They only need the conflict instruction:

```text
Resolve the conflict here:
  .spread/release-1.1
```

An advanced `current` workspace mode may exist for users who want operations to run in the current checkout, but the primary path is isolated.

## TUI Design

The TUI is a propagation control panel, not a merge editor.

It shows:

- selected task or command
- source input
- mode
- target list
- per-target status
- current conflict workspace
- conflicted file summary
- available actions

Example:

```text
Git Spread - release

Source: develop                  Mode: direct
Workspace: isolated              Editor: auto

Targets
  done       release/1.0        pushed
> conflict   release/1.1        2 files
  pending    main

Conflict summary for release/1.1
  Workspace: .spread/release-1.1
  Files:     user.go, config.yaml

Actions
  o   open workspace in editor
  r   refresh status
  c   continue
  p   create PR instead
  a   abort run
```

The TUI should not ask users to resolve conflicts file by file. It should open the whole conflicted workspace in the configured editor.

## Editor Handoff

On conflict, Git Spread can open the conflicted workspace:

```bash
git spread open
git spread open --editor code
git spread open --editor idea
git spread open --editor cursor
```

Editor config:

```yaml
defaults:
  editor: auto
```

`auto` may try common editors and otherwise print the path only.

Users resolve conflicts using their normal IDE Git flow. Git Spread does not implement its own merge UI.

## Continue Behavior

`git spread continue` can be run from any directory inside the repository or its isolated workspaces.

When continuing a paused target:

- if unmerged files remain, stay paused and show the files
- if changes are staged but not committed, finish the merge or cherry-pick commit
- if the user already committed from the editor and the workspace is clean, mark the target complete
- after the target is complete, continue to the next target

This supports users who stage in the terminal and users who commit from VS Code, IDEA, or Cursor.

## State

Git Spread stores run state locally so `status`, `continue`, and `abort` do not depend on the current shell directory or active branch.

State records include:

- run id
- task or command input
- resolved targets
- propagation type
- execution mode
- per-target workspace path
- per-target status
- conflicted files
- Git operation in progress
- created branch names
- pushed refs
- pull request URLs

The state format is an implementation detail, but it should be inspectable enough for debugging.

## Error Handling

Common failures should produce actionable messages:

- dirty source repository: explain what must be clean before starting
- missing target branch: show the unresolved target
- empty target pattern: show the pattern and configured remote
- merge or cherry-pick conflict: pause and show workspace plus files
- push rejected: pause and offer PR mode
- GitHub authentication missing: explain the missing auth requirement
- fork unavailable: explain how to configure `github.collaboration`

Errors should preserve run state unless the user runs `abort`.

## User Burden

Zero-config examples:

```bash
git spread branch --to release/1.0 --mode pr
git spread branch develop --to release/1.0,main
git spread commit abc123 --to release/1.0
git spread pr 123 --to release/*
```

Team examples:

```bash
git spread run release
git spread commit abc123 --task backport
git spread pr 123 --task backport-pr
```

Conflict examples:

```bash
git spread
# press o to open the conflicted workspace
# resolve in the editor
git spread continue
```

The main burden is understanding that conflicts are resolved in an isolated workspace rather than the current checkout. The TUI and command output should repeat the exact path every time a run is paused.

## Testing Strategy

The implementation should be tested with local temporary Git repositories.

Required test areas:

- config parsing and CLI override precedence
- current-branch default for branch mode
- required input validation for commit and pull request modes
- commit list and range expansion order
- target pattern expansion
- direct branch merge propagation
- commit cherry-pick propagation
- pull request commit extraction through a mocked GitHub layer
- isolated workspace creation and reuse
- conflict pause state
- continue after staged resolution
- continue after user-created commit
- abort cleanup behavior
- push rejection pause behavior
- PR branch naming and PR creation through a mocked GitHub layer

GitHub calls should sit behind an interface so tests can run without network access.

## Design Decision Summary

Git Spread v1 should be complete enough to feel like a product:

- full propagation units: branch, commit, pull request
- full local lifecycle: plan, run, status, continue, abort
- human-friendly TUI
- isolated workspaces
- editor-based conflict resolution
- GitHub direct and PR workflows

The design still keeps a tight boundary: it propagates changes between Git branches. It does not manage release approvals or deployments.
