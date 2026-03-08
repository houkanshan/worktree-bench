# worktree-bench

A Bubble Tea TUI for managing a pool of git worktree workbenches.

## Features
- **Workbench pool** with three types:
  - **full**: setup + dev server
  - **light**: setup only
  - **minimal**: plain worktree
- **TUI create / switch / status / adopt / delete** with tabbed workbench types
- **Status line** shows branch, PR (via `gh`), and uncommitted line counts
- **Shell directives** for `cd` so switching updates your shell session

## How it works (ASCII diagram)
```
Developer workflow
──────────────────

┌───────────────┐   create / adopt / switch   ┌────────────────────────┐
│ developer     │ ──────────────────────────> │ worktree-bench (TUI)   │
│ (current cwd) │                              └────────────┬──────────┘
└───────────────┘                                           │
                                                           │ switch anytime if scope grows
                                                           ▼
                     workbenches (more minimal > light > full)

  ┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
  │ minimal/<name>       │  │ minimal/<name>       │  │ minimal/<name>       │
  │ PR review / reading  │  │ tiny change          │  │ temporary use        │
  │ fast + cheap         │  │ fast + cheap         │  │ fast + cheap         │
  └──────────────────────┘  └──────────────────────┘  └──────────────────────┘

        ┌──────────────────────┐      ┌──────────────────────┐
        │ light/<name>         │      │ light/<name>         │
        │ small fixes          │      │ quick changes        │
        │ setup only           │      │ setup only           │
        └──────────────────────┘      └──────────────────────┘

                 ┌──────────────────────┐
                 │ full/<name>          │
                 │ heavy features       │
                 │ setup + dev server   │
                 └──────────────────────┘

Notes:
- Pick minimal for review/reading or tiny changes.
- Pick light for small fixes.
- Pick full for heavy feature work.
- When full/light are busy, use a temporary minimal/light bench.
```

## Requirements
- `git`
- `gh` (GitHub CLI) for PR checkout and PR status
- Go 1.21+ for building

## Install
```bash
cd /Users/mhou/code/worktree-bench

go build ./cmd/worktree-bench
# Optional: move binary into PATH
# mv worktree-bench /usr/local/bin/
```

## Usage
### Default dashboard (no args)
```bash
./worktree-bench
```
Shortcuts:
- `tab`: switch type tab
- `enter`: create new ("+ new") or switch to selected workbench
- `d`: delete selected workbench (confirm y/n)

### Create a workbench
```bash
./worktree-bench create
```
If the current worktree is not registered, you'll be asked whether to adopt it or create a new worktree.

### View status
```bash
./worktree-bench status
```

### Adopt current worktree
```bash
./worktree-bench adopt
```

### Delete a workbench
```bash
./worktree-bench delete
```

### Switch workbench (bash/zsh)
Option A: eval directly
```bash
eval "$(./worktree-bench switch)"
```

Option B: source the helper function (opens the dashboard)
```bash
source ./scripts/wtb.bash
wtb
```

### Switch workbench (fish)
```fish
source ./scripts/wtb.fish
wtb
```

## Shell directives
`worktree-bench switch` writes `cd 'path'` to a directive file (env `WTB_DIRECTIVE_FILE`) or prints to stdout. The provided wrapper functions consume that output and `eval` it in your shell so the working directory changes in the current session.

## Configuration
Settings and pool data are stored in the repo root under `.worktree-bench/`:
- `config.json`: `worktrees_dir`, `setup_cmd`, `dev_cmd`
- `pool.json`: workbench metadata

Defaults are inferred from lockfiles and `package.json` (when available).

## Development
```bash
go build ./cmd/worktree-bench
```

## Roadmap ideas
- Workbench cleanup / archive command
- Per-bench tags and notes
- Background dev-server supervision
