# worktree-bench

A Bubble Tea TUI for managing a pool of git worktree workbenches.

## Features
- **Workbench pool** with three types:
  - **large**: setup + dev server
  - **medium**: setup only
  - **small**: plain worktree
- **TUI create / swap / status / adopt / delete** with tabbed workbench types
- **Status line** shows branch, PR (via `gh`), and uncommitted line counts
- **Shell directives** for `cd` so changing workbenches updates your shell session

## How it works (ASCII diagram)
```
Developer workflow
──────────────────

┌───────────────┐   create / adopt / swap     ┌────────────────────────┐
│ developer     │ ──────────────────────────> │ worktree-bench (TUI)   │
│ (current cwd) │                              └────────────┬──────────┘
└───────────────┘                                           │
                                                           │ swap anytime if scope grows
                                                           ▼
                     workbenches (more small > medium > large)

  ┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
  │ small/<name>         │  │ small/<name>         │  │ small/<name>         │
  │ PR review / reading  │  │ tiny change          │  │ temporary use        │
  │ fast + cheap         │  │ fast + cheap         │  │ fast + cheap         │
  └──────────────────────┘  └──────────────────────┘  └──────────────────────┘

        ┌──────────────────────┐      ┌──────────────────────┐
        │ medium/<name>        │      │ medium/<name>        │
        │ small fixes          │      │ quick changes        │
        │ setup only           │      │ setup only           │
        └──────────────────────┘      └──────────────────────┘

                 ┌──────────────────────┐
                 │ large/<name>         │
                 │ heavy features       │
                 │ setup + dev server   │
                 └──────────────────────┘

Notes:
- Pick small for review/reading or tiny changes.
- Pick medium for small fixes.
- Pick large for heavy feature work.
- When large/medium are busy, use a temporary small/medium bench.
```

## Requirements
- `git`
- `gh` (GitHub CLI) for PR checkout and PR status
- Go 1.21+ for building

## Install
```bash
cd /Users/mhou/code/worktree-bench

go build ./cmd/worktree-bench
ln -s "$(pwd)/worktree-bench" /usr/local/bin/worktree-bench
# Or move it for a one-off install
# mv worktree-bench /usr/local/bin/
```

## Shell setup (recommended)
### bash/zsh
```bash
mkdir -p ~/.config/worktree-bench
cp ./scripts/wtb.bash ~/.config/worktree-bench/wtb.bash
# Add to your ~/.bashrc or ~/.zshrc:
source ~/.config/worktree-bench/wtb.bash
```

### fish
```fish
mkdir -p ~/.config/fish/functions
cp ./scripts/wtb.fish ~/.config/fish/functions/wtb.fish
# New shells will auto-load; run `source ~/.config/fish/functions/wtb.fish` to load now.
```

## Usage
### Default dashboard (no args)
```bash
./worktree-bench
```
Shortcuts:
- `tab`: switch type tab
- `enter`: create new ("+ new") or cd to selected workbench
- `d`: delete selected workbench (confirm y/n)


### No-TUI selection
```bash
# cd to an existing workbench by id
./worktree-bench wb-123

# create a fresh workbench without opening the selector
./worktree-bench --new --type medium

# one-shot init_cmd override (`-` or empty disables init for this action)
./worktree-bench --init-cmd "pnpm install" wb-123
./worktree-bench --new --type small --init-cmd -

# machine-readable direct selection (stdout is one JSON document)
./worktree-bench wb-123 --init-cmd gnm --json
./worktree-bench --new --type large --init-cmd gnm --json
```

JSON selection output has the shape:

```json
{"benchId":"wb-123","name":"repo-l-1","type":"large","path":"/path/to/repo-l-1","created":false}
```

### Create a workbench
```bash
./worktree-bench create
```
If the current worktree is not registered, you'll be asked whether to adopt it or create a new worktree.

### View status
```bash
./worktree-bench status

# full machine-readable status with fresh Git/PR reuse safety
./worktree-bench status --json

# cached display metadata without reuse safety
./worktree-bench status --json --fast

# constrain command execution to an authorized filesystem root
./worktree-bench status --json --allowed-root /path/to/worktrees
```

Full JSON status adds a `git` object to every authorized bench. Its `severity` is `safe` only when the worktree is clean and HEAD is contained by the default branch or exactly matches the head of a merged pull request. Dirty worktrees, unpushed commits, and open, closed, missing, stale, or unknown pull requests are never safe. `--fast` explicitly skips these fresh reuse checks. Repeat `--allowed-root` when a caller has multiple authorized roots; benches outside those roots remain listed without a `git` object.

Automation can require a second fresh check immediately before initialization:

```bash
./worktree-bench wb-123 --require-reusable --allowed-root /path/to/worktrees --init-cmd gnm --json
```

### Adopt current worktree
```bash
./worktree-bench adopt
```

### Delete a workbench
```bash
./worktree-bench delete
```

### Swap workbench (bash/zsh)
Option A: eval directly
```bash
eval "$(./worktree-bench swap)"
```

Option B: source the helper function (opens the dashboard)
```bash
source ./scripts/wtb.bash
wtb swap
```
If you followed the shell setup above, you can just run `wtb swap`.

### Swap workbench (fish)
```fish
source ./scripts/wtb.fish
wtb swap
```
If you followed the shell setup above, you can just run `wtb swap`.

## Shell directives
`worktree-bench` (and `worktree-bench swap`) writes `cd 'path'` to a directive file (env `WTB_DIRECTIVE_FILE`) or prints to stdout. The provided wrapper functions consume that output and `eval` it in your shell so the working directory changes in the current session.

## Configuration
Settings and pool data are stored in the repo root under `.worktree-bench/`:
- `config.json`: `worktrees_dir`, `setup_cmd`, `dev_cmd`, `init_cmd`, `branch_prefix`, `worktree_name_prefix`
- `pool.json`: workbench metadata

Defaults are inferred from lockfiles and `package.json` (when available).
Auto-generated workbench names use `{worktree_name_prefix}{size-short}-{number}`, where size short is `l`, `m`, or `s`.


## Pi extension
A pi extension is provided at `~/.pi/agent/extensions/wtb.ts`. After `/reload`, use `/wtb` to open a worktree-bench selector inside pi. It shows the same workbench labels and status descriptions as the TUI, asks whether to run `gnm` (`~/tools/gnm`) in the selected worktree, then runs `worktree-bench` in no-TUI mode with the selected workbench id or `--new --type <type>`. Choosing gnm passes `--init-cmd "gnm"`; choosing no passes `--init-cmd "-"` so no configured init command runs. Any arguments passed to `/wtb` are still forwarded to `worktree-bench`.

## Development
```bash
go build ./cmd/worktree-bench
```

## Roadmap ideas
- Workbench cleanup / archive command
- Per-bench tags and notes
- Background dev-server supervision
