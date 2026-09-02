package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePRNumberSupportsGitHubURLs(t *testing.T) {
	for _, test := range []struct {
		target string
		want   int
		ok     bool
	}{
		{target: "42", want: 42, ok: true},
		{target: "#42", want: 42, ok: true},
		{target: "pr:42", want: 42, ok: true},
		{target: "https://github.com/owner/repo/pull/42", want: 42, ok: true},
		{target: "https://github.com/owner/repo/pull/42/files?diff=split", want: 42, ok: true},
		{target: "https://example.com/owner/repo/pull/42"},
		{target: "feature/42"},
		{target: "0"},
	} {
		t.Run(test.target, func(t *testing.T) {
			got, ok := parsePRNumber(test.target)
			if got != test.want || ok != test.ok {
				t.Fatalf("parsePRNumber(%q) = (%d, %t), want (%d, %t)", test.target, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestCheckoutTargetSupportsBranchAndPRNumber(t *testing.T) {
	repo := initializedRepo(t)
	gitTest(t, repo, "branch", "feature")
	if _, err := CheckoutTarget(repo, "feature"); err != nil {
		t.Fatal(err)
	}
	if branch := gitTest(t, repo, "branch", "--show-current"); branch != "feature" {
		t.Fatalf("current branch = %q, want feature", branch)
	}
	if _, err := CheckoutTarget(repo, "missing-branch"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a missing branch error, got %v", err)
	}
	if branches := gitTest(t, repo, "branch", "--format=%(refname:short)"); strings.Contains(branches, "missing-branch") {
		t.Fatalf("checkout created a missing branch: %s", branches)
	}

	remote := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare remote: %v\n%s", err, out)
	}
	gitTest(t, repo, "remote", "add", "origin", remote)
	gitTest(t, repo, "branch", "remote-only", "master")
	gitTest(t, repo, "push", "origin", "remote-only")
	gitTest(t, repo, "branch", "-D", "remote-only")
	gitTest(t, repo, "update-ref", "-d", "refs/remotes/origin/remote-only")
	if _, err := CheckoutTarget(repo, "remote-only"); err != nil {
		t.Fatal(err)
	}
	if branch := gitTest(t, repo, "branch", "--show-current"); branch != "remote-only" {
		t.Fatalf("current branch = %q, want remote-only", branch)
	}

	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "gh.log")
	ghPath := filepath.Join(binDir, "gh")
	commit := gitTest(t, repo, "rev-parse", "HEAD")
	script := `#!/bin/sh
if [ "$1 $2" = "pr view" ]; then
  printf '{"headRefName":"feature","headRefOid":"%s","isCrossRepository":false,"number":42,"url":"https://github.com/owner/repo/pull/42"}\n' "$WTB_HEAD"
elif [ "$1 $2" = "repo view" ]; then
  printf 'owner/repo\n'
elif [ "$1 $2" = "pr checkout" ]; then
  printf '%s\n' "$PWD" "$*" > "$WTB_GH_LOG"
fi
`
	if err := os.WriteFile(ghPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WTB_GH_LOG", logPath)
	t.Setenv("WTB_HEAD", commit)
	for _, target := range []string{"#42", "https://github.com/owner/repo/pull/42/files"} {
		if _, err := CheckoutTarget(repo, target); err != nil {
			t.Fatal(err)
		}
		payload, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.TrimSpace(string(payload)), repo+"\npr checkout 42"; got != want {
			t.Fatalf("gh invocation for %q = %q, want %q", target, got, want)
		}
	}
}

func TestResolveTargetIgnoresPrunableWorktrees(t *testing.T) {
	repo := initializedRepo(t)
	gitTest(t, repo, "branch", "feature")
	stalePath := filepath.Join(t.TempDir(), "stale-worktree")
	gitTest(t, repo, "worktree", "add", stalePath, "feature")
	if err := os.RemoveAll(stalePath); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveTarget(repo, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ExistingPath != "" {
		t.Fatalf("prunable worktree was returned: %s", resolved.ExistingPath)
	}
}

func TestCheckoutTargetReusesBranchOwnedByAnotherWorktree(t *testing.T) {
	repo := initializedRepo(t)
	gitTest(t, repo, "branch", "feature")
	otherWorktree := filepath.Join(t.TempDir(), "feature-worktree")
	gitTest(t, repo, "worktree", "add", otherWorktree, "feature")

	result, err := CheckoutTarget(repo, "feature")
	if err != nil {
		t.Fatal(err)
	}
	wantPath, err := filepath.EvalSymlinks(otherWorktree)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != wantPath || result.Disposition != "existing" {
		t.Fatalf("checkout result = %#v, want existing path %s", result, wantPath)
	}
	if branch := gitTest(t, repo, "branch", "--show-current"); branch != "master" {
		t.Fatalf("checkout changed the caller to %q", branch)
	}
}
