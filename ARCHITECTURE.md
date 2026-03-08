# Architecture

## Overview
`worktree-bench` manages a pool of git worktrees (workbenches) so you can reuse pre‑setup environments. It offers a Bubble Tea TUI for creating and switching workbenches, plus a status view.

## Key concepts
- **Workbench types**
  - **full**: setup + dev server
  - **light**: setup only
  - **minimal**: plain worktree
- **Pool metadata**
  - Stored in `.worktree-bench/pool.json` under the repo root.
  - Each workbench tracks `id`, `name`, `type`, `path`, `created_at`, `last_setup`, and optional dev server metadata.
- **Settings**
  - Stored in `.worktree-bench/config.json`.
  - Includes `worktrees_dir`, `setup_cmd`, `dev_cmd`.
  - Defaults are inferred from lockfiles / `package.json` and can be edited.

## Runtime flow
- **create**
  - TUI lets you pick a type and reuse an existing bench or create a new one.
  - Runs `git worktree add`, optionally `gh pr checkout`.
  - Runs setup and/or dev server based on type.
- **switch**
  - TUI lets you pick a workbench by type.
  - Optionally swaps current worktree with the target (via `git worktree move`).
  - Emits shell directives (`cd 'path'`) to stdout or a directive file.
- **status**
  - TUI shows branch, PR (via `gh`), and uncommitted line counts.
- **adopt**
  - Registers the current worktree as a workbench.
  - Prompts for type, name, and whether to run setup/dev.
- **delete**
  - Removes a workbench and its worktree directory.

## Packages
- `internal/config`: settings + pool persistence
- `internal/gitutil`: git helpers (branch, diff, worktree add/move)
- `internal/bench`: core operations (create, switch, setup/dev)
- `internal/ui`: Bubble Tea models for create/switch/status
- `internal/cmd`: cobra CLI commands

## Shell directives
`switch` writes `cd 'path'` to a directive file (env `WTB_DIRECTIVE_FILE`) or prints to stdout for `eval` usage in bash/zsh.

## External dependencies
- `git` for worktrees
- `gh` for PR lookup and checkout
- `Bubble Tea` + `Bubbles` + `Lip Gloss` for the TUI
- `Cobra` for CLI wiring
