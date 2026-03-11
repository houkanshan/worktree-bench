# shellcheck shell=bash
# Usage: source this file, then call `wtb` to open or swap workbenches.
# Works for bash/zsh.

wtb() {
  local directive_file exit_code bin repo_root
  directive_file="$(mktemp)"

  bin="${WTB_BIN:-worktree-bench}"
  if [ -z "${WTB_BIN:-}" ]; then
    if repo_root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
      if [ -x "$repo_root/worktree-bench" ]; then
        bin="$repo_root/worktree-bench"
      fi
    fi
  fi

  WTB_DIRECTIVE_FILE="$directive_file" "$bin" "$@"
  exit_code=$?

  if [ -s "$directive_file" ]; then
    # Directive lines are emitted by worktree-bench (cd ...). Keep formats in sync.
    eval "$(cat "$directive_file")"
    if [ $exit_code -eq 0 ]; then
      exit_code=$?
    fi
  fi

  rm -f "$directive_file"
  return $exit_code
}
