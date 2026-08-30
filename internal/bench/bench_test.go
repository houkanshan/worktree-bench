package bench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"worktree-bench/internal/config"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func initializedRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "init", "-b", "master")
	gitTest(t, repo, "config", "user.email", "test@example.com")
	gitTest(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", "README.md")
	gitTest(t, repo, "commit", "-m", "base")
	return repo
}

func TestCreateWorkbenchFailureRemovesWorktreeAndGeneratedBranch(t *testing.T) {
	for _, test := range []struct {
		name      string
		benchType string
		setupCmd  string
		initCmd   string
		runInit   bool
	}{
		{name: "setup", benchType: config.TypeMedium, setupCmd: "false"},
		{name: "init", benchType: config.TypeSmall, initCmd: "false", runInit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := initializedRepo(t)
			worktreesDir := filepath.Join(filepath.Dir(repo), "worktrees")
			settings := config.Settings{
				WorktreesDir: worktreesDir,
				SetupCmd:     test.setupCmd,
				InitCmd:      test.initCmd,
				BranchPrefix: "bench/",
			}
			pool := config.Pool{Version: 1, RepoRoot: repo, WorktreesDir: worktreesDir, Benches: []config.Workbench{}}
			updated, created, err := CreateWorkbench(repo, settings, pool, CreateInput{
				Type:       test.benchType,
				Name:       "failed-bench",
				BaseBranch: "master",
				RunInit:    test.runInit,
			})
			if err == nil || !strings.Contains(err.Error(), "false") {
				t.Fatalf("expected original command error, got %v", err)
			}
			if created != nil || len(updated.Benches) != 0 {
				t.Fatalf("failed creation changed the pool: created=%+v pool=%+v", created, updated.Benches)
			}
			if _, err := os.Stat(filepath.Join(worktreesDir, "failed-bench")); !os.IsNotExist(err) {
				t.Fatalf("failed worktree still exists: %v", err)
			}
			if branches := gitTest(t, repo, "for-each-ref", "--format=%(refname:short)", "refs/heads/bench"); branches != "" {
				t.Fatalf("generated branch leaked: %s", branches)
			}
			lines := strings.Split(gitTest(t, repo, "worktree", "list", "--porcelain"), "\n")
			count := 0
			for _, line := range lines {
				if strings.HasPrefix(line, "worktree ") {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("expected only the main worktree, got %d", count)
			}
		})
	}
}

func TestCreateWorkbenchInitFailureStopsDevProcess(t *testing.T) {
	repo := initializedRepo(t)
	root := filepath.Dir(repo)
	worktreesDir := filepath.Join(root, "worktrees")
	pidFile := filepath.Join(root, "dev.pid")
	settings := config.Settings{
		WorktreesDir: worktreesDir,
		DevCmd:       fmt.Sprintf("echo $$ > %q; exec sleep 60", pidFile),
		InitCmd:      "sleep 0.1; false",
		BranchPrefix: "bench/",
	}
	pool := config.Pool{Version: 1, RepoRoot: repo, WorktreesDir: worktreesDir, Benches: []config.Workbench{}}
	_, _, err := CreateWorkbench(repo, settings, pool, CreateInput{
		Type:       config.TypeLarge,
		Name:       "failed-dev-bench",
		BaseBranch: "master",
		RunInit:    true,
	})
	if err == nil {
		t.Fatal("expected init failure")
	}
	payload, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("dev process did not record its pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscall.Signal(0)); err == nil {
		t.Fatalf("dev process %d survived failed creation", pid)
	}
}
