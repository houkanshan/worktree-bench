package gitutil

import (
	"os"
	"path/filepath"
	"testing"
)

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func setupReuseCommands(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	writeExecutable(t, filepath.Join(bin, "git"), `
case "$*" in
  *"status --porcelain") [ "${GIT_SCENARIO:-}" = dirty ] && echo " M .mcp.json"; exit 0 ;;
  *"branch --show-current") echo feature/test; exit 0 ;;
  *"symbolic-ref --quiet refs/remotes/origin/HEAD") exit 1 ;;
  *"for-each-ref --contains=HEAD"*) [ "${GIT_SCENARIO:-}" = default ] && echo refs/heads/master; exit 0 ;;
  *"rev-list --count"*) [ "${GIT_SCENARIO:-}" = unpushed ] && echo 2 || echo 0; exit 0 ;;
esac
exit 2
`)
	writeExecutable(t, filepath.Join(root, "tools", "branch-pr-status"), `printf '%s\n' "${PR_OUTPUT:-}"`)
	t.Setenv("HOME", root)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	worktree := filepath.Join(root, "worktree")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func TestInspectReuseStatusRejectsEveryDirtyFile(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "dirty")
	status := InspectReuseStatus(worktree)
	if status.State != "uncommitted" || status.Severity != "warning" || status.Dirty == nil || !*status.Dirty {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusRejectsUnpushedCommitsBeforePR(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "unpushed")
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":42}`)
	status := InspectReuseStatus(worktree)
	if status.State != "unpushed" || status.Unpushed == nil || *status.Unpushed != 2 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusAcceptsDefaultBranchOrMergedPR(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "default")
	status := InspectReuseStatus(worktree)
	if status.State != "no-change" || status.Severity != "safe" {
		t.Fatalf("unexpected default-branch status: %+v", status)
	}

	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":42}`)
	status = InspectReuseStatus(worktree)
	if status.State != "pr-merged" || status.Severity != "safe" || status.PR == nil || status.PR.State != "merged" {
		t.Fatalf("unexpected merged-PR status: %+v", status)
	}
}

func TestInspectReuseStatusRejectsOpenPR(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("PR_OUTPUT", `{"state":"open","number":43}`)
	status := InspectReuseStatus(worktree)
	if status.State != "pr-open" || status.Severity != "warning" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
