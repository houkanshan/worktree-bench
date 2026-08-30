package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWorktreeFingerprintChanges(t *testing.T) {
	repo := initRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	run(t, repo, "git", "worktree", "add", "-b", "bench/test", wt)

	base := fingerprint(t, wt)
	run(t, wt, "git", "checkout", "-b", "bench/other")
	branch := fingerprint(t, wt)
	if branch == base {
		t.Fatalf("branch switch did not change fingerprint")
	}

	writeFile(t, filepath.Join(wt, "file.txt"), "changed\n")
	run(t, wt, "git", "add", "file.txt")
	staged := fingerprint(t, wt)
	if staged == branch {
		t.Fatalf("staged change did not change fingerprint")
	}

	run(t, wt, "git", "commit", "-m", "change")
	committed := fingerprint(t, wt)
	if committed == staged || committed == branch {
		t.Fatalf("new commit did not change fingerprint")
	}
}

func TestWorktreeGitDirParsesFile(t *testing.T) {
	repo := initRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	run(t, repo, "git", "worktree", "add", "-b", "bench/test", wt)
	gitDir, err := WorktreeGitDir(wt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); err != nil {
		t.Fatalf("resolved gitdir missing HEAD: %v", err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test User")
	writeFile(t, filepath.Join(dir, "file.txt"), "base\n")
	run(t, dir, "git", "add", "file.txt")
	run(t, dir, "git", "commit", "-m", "base")
	return dir
}

func fingerprint(t *testing.T, path string) string {
	t.Helper()
	fp, err := WorktreeFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure filesystems with coarse mtimes produce distinct stat metadata.
	time.Sleep(10 * time.Millisecond)
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
