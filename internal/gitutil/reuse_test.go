package gitutil

import (
	"os"
	"path/filepath"
	"sync"
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
  *"status --porcelain") if [ "${GIT_SCENARIO:-}" = dirty ] || { [ -n "${DIRTY_MARKER:-}" ] && [ -f "${DIRTY_MARKER:-}" ]; }; then echo " M .mcp.json"; fi; exit 0 ;;
  *"branch --show-current") echo feature/test; exit 0 ;;
  *"symbolic-ref --quiet refs/remotes/origin/HEAD") exit 1 ;;
  *"for-each-ref --contains=HEAD"*)
    [ "${GIT_SCENARIO:-}" = default ] || [ "${GIT_SCENARIO:-}" = default-ahead ] && echo refs/heads/master
    [ "${GIT_SCENARIO:-}" = warning ] && echo warning >&2
    exit 0 ;;
  *"rev-list --count"*)
    [ "${GIT_SCENARIO:-}" = unpushed ] || [ "${GIT_SCENARIO:-}" = default-ahead ] && echo 2 || echo 0
    exit 0 ;;
  *"rev-parse HEAD") echo abc123; exit 0 ;;
  *"show -s --format=%cI abc123") echo 2026-01-02T01:00:00Z; exit 0 ;;
esac
exit 2
`)
	writeExecutable(t, filepath.Join(root, "tools", "branch-pr-status"), `[ -z "${MAKE_DIRTY:-}" ] || touch "$MAKE_DIRTY"
printf '%s\n' "${PR_OUTPUT:-}"`)
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
	if status.Kind != "uncommitted" || status.Severity != "warning" || status.Dirty == nil || !*status.Dirty {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusRejectsUnpushedCommitsBeforePR(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "unpushed")
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":42,"headSha":"abc123","headRefOid":"abc123","stale":false}`)
	status := InspectReuseStatus(worktree)
	if status.Kind != "unpushed" || status.Unpushed == nil || *status.Unpushed != 2 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusRejectsAheadLocalDefaultBranch(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "default-ahead")
	status := InspectReuseStatus(worktree)
	if status.Kind != "unpushed" || status.Severity != "warning" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusIgnoresSuccessfulGitStderr(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "warning")
	status := InspectReuseStatus(worktree)
	if status.Severity == "safe" {
		t.Fatalf("stderr warning authorized reuse: %+v", status)
	}
}

func TestInspectReuseStatusAcceptsDefaultBranchOrMergedPR(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "default")
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":41,"headSha":"abc123","headRefOid":"abc123","updatedAt":"2026-01-02T02:00:00Z","stale":false}`)
	status := InspectReuseStatus(worktree)
	if status.Kind != "no-change" || status.Severity != "safe" {
		t.Fatalf("unexpected default-branch status: %+v", status)
	}
	if status.Activity.CommitAt != "2026-01-02T01:00:00Z" || status.Activity.PRAt != "2026-01-02T02:00:00Z" {
		t.Fatalf("unexpected default-branch activity: %+v", status.Activity)
	}

	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":42,"headSha":"abc123","headRefOid":"abc123","updatedAt":"2026-01-02T02:00:00Z","mergedAt":"2026-01-02T03:00:00Z","closedAt":"2026-01-02T02:30:00Z","stale":false}`)
	status = InspectReuseStatus(worktree)
	if status.Kind != "pr-merged" || status.Severity != "safe" || status.PR == nil || status.PR.State != "merged" {
		t.Fatalf("unexpected merged-PR status: %+v", status)
	}
	if status.Activity.CommitAt != "2026-01-02T01:00:00Z" || status.Activity.PRAt != "2026-01-02T03:00:00Z" {
		t.Fatalf("unexpected activity: %+v", status.Activity)
	}
}

func TestInspectReuseStatusAcceptsSyncedOpenPR(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("PR_OUTPUT", `{"state":"open","number":43,"headSha":"abc123","headRefOid":"abc123","stale":false}`)
	status := InspectReuseStatus(worktree)
	if status.Kind != "pr-open" || status.Severity != "safe" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusRejectsOpenPRWhoseRemoteHeadMoved(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("PR_OUTPUT", `{"state":"open","number":43,"headSha":"abc123","headRefOid":"newer","stale":false}`)
	status := InspectReuseStatus(worktree)
	if status.Kind != "pr-open" || status.Severity != "warning" || status.PR == nil || status.PR.Number != float64(43) {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestInspectReuseStatusRechecksCleanlinessAfterPRLookup(t *testing.T) {
	worktree := setupReuseCommands(t)
	marker := filepath.Join(worktree, "dirty-marker")
	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("DIRTY_MARKER", marker)
	t.Setenv("MAKE_DIRTY", marker)
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":42,"headSha":"abc123","headRefOid":"abc123","stale":false}`)
	status := InspectReuseStatus(worktree)
	if status.Kind != "unknown" || status.Severity != "muted" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestBranchPRStatusCallsAreSerializedByFileLock(t *testing.T) {
	worktree := setupReuseCommands(t)
	home := os.Getenv("HOME")
	guard := filepath.Join(home, "helper-running")
	writeExecutable(t, filepath.Join(home, "tools", "branch-pr-status"), `
if ! mkdir "$HELPER_GUARD" 2>/dev/null; then exit 9; fi
sleep 0.1
rmdir "$HELPER_GUARD"
echo '{"state":"none"}'
`)
	t.Setenv("HELPER_GUARD", guard)
	var group sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := runBranchPRStatus(home, worktree, "feature/test")
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent helper failed: %v", err)
		}
	}
}

func TestInspectReuseStatusRejectsMergedPRForAnOlderHead(t *testing.T) {
	worktree := setupReuseCommands(t)
	t.Setenv("GIT_SCENARIO", "pushed")
	t.Setenv("PR_OUTPUT", `{"state":"merged","number":42,"headSha":"abc123","headRefOid":"older","stale":false}`)
	status := InspectReuseStatus(worktree)
	if status.Kind != "unknown" || status.Severity != "muted" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
