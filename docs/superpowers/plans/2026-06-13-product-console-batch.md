# Git Spread Product Console Batch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Git Spread from a working propagation CLI into a richer local control console with task navigation, pre-run confirmation, history, recovery, and PR-mode polish.

**Architecture:** Keep the existing command/config/state/spread/TUI boundaries. Add small state-side stores for history, extend task metadata in config, add TUI screens/actions without rewriting the executor, and expose recovery operations through CLI commands that reuse the state and git packages.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, `go.yaml.in/yaml/v3`, Git CLI, GitHub `go-gh`.

---

### Task 1: Task Metadata, Search, and Confirm Screen

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

- [ ] Write failing config tests for `description` and `group` task fields.
- [ ] Implement `Task.Description` and `Task.Group` and map them into `tui.TaskItem`.
- [ ] Write failing TUI tests for `/` search, `g/G` top/bottom navigation, grouped task labels, and pre-run confirmation.
- [ ] Implement task filtering/search state, top/bottom keys, and a confirm screen that shows the selected task plan before executing.
- [ ] Run `go test ./internal/config ./internal/cli ./internal/tui`.

### Task 2: Run History and Rerun

**Files:**
- Create: `internal/state/history.go`
- Create: `internal/state/history_test.go`
- Modify: `internal/state/store.go`
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`
- Modify: `internal/spread/executor.go`
- Modify: `internal/spread/continue.go`
- Modify: `internal/spread/*_test.go`

- [ ] Write failing state tests for appending and listing recent history entries.
- [ ] Implement JSONL history under `.git/spread/history.jsonl`.
- [ ] Write failing executor tests proving completed and paused runs are recorded with summary counts.
- [ ] Save history snapshots after execute/continue status changes.
- [ ] Add `git spread history`, `git spread run --last`, and `git spread retry` for failed/conflicted/rejected targets from the last active/history run where data is available.
- [ ] Run `go test ./internal/state ./internal/spread ./internal/cli`.

### Task 3: Recovery and Doctor

**Files:**
- Create: `internal/spread/doctor.go`
- Create: `internal/spread/doctor_test.go`
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

- [ ] Write failing tests for detecting corrupted state, missing workspaces, workspaces on wrong branches, and uncommitted workspace changes.
- [ ] Implement `spread.Doctor` returning actionable findings.
- [ ] Add `git spread doctor`.
- [ ] Extend `git spread reset` with `--target` and `--clean-worktree` for focused cleanup without deleting unrelated workspaces.
- [ ] Add TUI action text for doctor/reset guidance where an invalid active run is detected.
- [ ] Run `go test ./internal/spread ./internal/cli ./internal/tui`.

### Task 4: PR Mode Polish

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/github/client.go`
- Modify: `internal/github/client_test.go`
- Modify: `internal/spread/types.go`
- Modify: `internal/spread/executor.go`
- Modify: `internal/spread/executor_pr_test.go`
- Modify: `internal/cli/commands.go`

- [ ] Write failing config tests for PR title/body/draft/reviewer/label defaults.
- [ ] Extend request/config types for PR metadata.
- [ ] Extend GitHub create PR payload.
- [ ] Write failing PR executor tests proving title/body templates include target/source and draft/review metadata reaches the client.
- [ ] Implement template rendering and payload propagation.
- [ ] Run `go test ./internal/config ./internal/github ./internal/spread`.

### Task 5: Product Experience

**Files:**
- Modify: `README.md`
- Modify: `docs/assets/git-spread-tasks.svg`
- Modify: `docs/assets/git-spread-run.svg`
- Modify: `docs/assets/git-spread-reset.svg`
- Create or modify: shell completion files if the CLI exposes generated completions cleanly.

- [ ] Update README usage for search, confirm, history, retry, doctor, and PR templates.
- [ ] Update screenshots to match current TUI copy.
- [ ] Add `git spread examples` or README examples if a CLI command is not worth the surface area yet.
- [ ] Run `go test ./...` and `git diff --check`.
