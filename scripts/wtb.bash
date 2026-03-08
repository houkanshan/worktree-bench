# shellcheck shell=bash
# Usage: source this file, then call `wtb` to switch workbenches.
# Works for bash/zsh.

wtb() {
  local directive_file exit_code
  directive_file="$(mktemp)"

  # Ensure worktree-bench is in PATH.
  WTB_DIRECTIVE_FILE="$directive_file" worktree-bench switch "$@"
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
