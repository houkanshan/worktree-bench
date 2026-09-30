package gitutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMainWorktreeRootWithSeparateGitDir(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "project")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "git", "init", "--separate-git-dir", filepath.Join(root, "metadata"))
	run(t, repo, "git", "config", "core.worktree", "../project")
	run(t, repo, "git", "config", "user.email", "test@example.com")
	run(t, repo, "git", "config", "user.name", "Test")
	run(t, repo, "git", "commit", "--allow-empty", "-m", "base")
	child := filepath.Join(root, "child")
	run(t, repo, "git", "worktree", "add", "-b", "child", child)
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{repo, child} {
		got, err := MainWorktreeRoot(path)
		if err != nil || got != want {
			t.Fatalf("main root from %s: got %q, %v; want %q", path, got, err, want)
		}
	}
}
