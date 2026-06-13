# Git Spread

Git Spread propagates branch, commit, and pull request changes across Git branches.

## Examples

```bash
git spread init --print
git spread branch develop --to release/1.0,main --no-tui
git spread commit abc123 --to release/1.0 --mode pr --no-tui
git spread pr 123 --to release/* --mode pr --no-tui
git spread
```

Conflicts are resolved in isolated workspaces under `.spread/`. Open the workspace in your editor, resolve the conflict, then run:

```bash
git spread continue
```

## Design

The design spec lives in `docs/superpowers/specs/2026-06-13-git-spread-design.md`.
