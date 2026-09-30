# Architecture

## Overview
`worktree-bench` manages a pool of git worktrees (workbenches) so you can reuse pre‑setup environments. It offers a Bubble Tea TUI for creating and switching workbenches, plus a status view.

## Key concepts
- **Workbench types**
  - **large**: setup + dev server
  - **medium**: setup only
  - **small**: plain worktree
- **Pool metadata**
  - Stored in `.worktree-bench/pool.json` under the repo root.
  - Each workbench tracks `id`, `name`, `type`, `path`, `created_at`, `last_setup`, and optional dev server metadata.
- **Settings**
  - Stored in `.worktree-bench/config.json`.
  - Includes `worktrees_dir`, `setup_cmd`, `dev_cmd`, `init_cmd`, `branch_prefix`.
  - Defaults are inferred from lockfiles / `package.json` and can be edited.

## Pool mutation ownership
- `internal/bench` public create/adopt/delete operations own persistence through `config.UpdatePool`; CLI and dashboard callers never save a previously loaded snapshot.
- `UpdatePool` takes a repository-scoped advisory `flock` on `.worktree-bench/pool.lock`, reloads the pool, performs the mutation, and atomically saves before releasing the lock. The lock file is retained so concurrent processes share one inode. The OS releases the lock on exit, including crashes.
- Prompts finish before transactions start. Setup/dev/init side effects remain inside the transaction; other mutations wait for those commands. Read-only pool loads never create or overwrite registry files.
- Main and child worktrees share the main worktree's configuration directory. Git's first porcelain worktree entry identifies ordinary main roots; `core.worktree` identifies submodules and configured separate gitdirs.
- Automatic names are `<main-project>-{n}` (large), `<main-project>-m-{n}` (medium), and `<main-project>-s-{n}` (small), with independent current maximum suffixes across registry names, filesystem entries, and Git worktree registrations. Existing and explicit names remain unchanged.
- The lock protects cooperating WTB registry writers, not external Git commands or worktree reuse/checkout leases. A crash can leave an unregistered worktree; subsequent automatic naming avoids its name.

## Runtime flow
- **dashboard (no args)**
  - Tabbed list with "+ new" plus workbenches.
  - `enter` switches/creates, `d` deletes with confirm.
- **create**
  - TUI lets you pick a type and reuse an existing bench or create a new one.
  - Runs `git worktree add`, optionally `gh pr checkout`.
  - Runs setup/dev based on type, then optional `init_cmd` for all types.
- **switch**
  - TUI lets you pick a workbench by type.
  - Optionally swaps current worktree with the target (via `git worktree move`).
  - Emits shell directives (`cd 'path'`) to stdout or a directive file.
- **checkout / resolve-target**
  - Resolves branches and pull requests to an immutable commit and branch name.
  - Enumerates Git worktrees structurally; checkout returns an existing authorized worktree instead of failing when it already owns the target branch at the resolved commit. Detached worktrees at the same commit do not own the branch and are never reuse candidates.
  - Rechecks worktree ownership after a checkout failure to reconcile races without parsing Git stderr.
- **status**
  - TUI shows branch, PR (via `gh`), and uncommitted line counts.
- **adopt**
  - Registers the current worktree as a workbench.
  - Prompts for type, name, and whether to run setup/dev/init.
- **delete**
  - Removes a workbench and its worktree directory.

## Packages
- `internal/config`: settings + pool persistence
- `internal/gitutil`: git helpers (branch, diff, worktree add/move)
- `internal/bench`: core operations (create, switch, target resolution/checkout, setup/dev)
- `internal/ui`: Bubble Tea models for create/switch/status
- `internal/cmd`: cobra CLI commands

## Shell directives
`switch` writes `cd 'path'` to a directive file (env `WTB_DIRECTIVE_FILE`) or prints to stdout for `eval` usage in bash/zsh.

## External dependencies
- `git` for worktrees
- `gh` for PR lookup and checkout
- `Bubble Tea` + `Bubbles` + `Lip Gloss` for the TUI
- `Cobra` for CLI wiring
