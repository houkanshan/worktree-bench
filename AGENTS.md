# AGENTS

## Project intent
`worktree-bench` provides a workbench pool manager for git worktrees with a Bubble Tea TUI.

## Coding conventions
- Prefer small, focused packages under `internal/`.
- Use `config.Settings` + `config.Pool` for persistence.
- Keep shelling out to `git`/`gh` inside `internal/gitutil` and `internal/bench`.
- Always wrap external command errors with context: include the command name and stderr output. Use `CombinedOutput()` + `fmt.Errorf` instead of bare `cmd.Run()`.
- Always run `go build ./cmd/worktree-bench` after making changes.

## UI behavior
- Tabs switch workbench types with `tab`.
- `create`/`switch`/`adopt`/`delete` run via Bubble Tea; avoid interactive prompts outside the TUI.

## Extending
- Add new bench types by updating `config` constants and UI tabs.
- Add extra status fields by extending `ui.BenchStatus` and `FormatStatusLine`.

## Tooling
- `go test` currently not defined; add tests under `internal/...` as needed.
- Build: `go build ./cmd/worktree-bench`.
