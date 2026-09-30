package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"worktree-bench/internal/config"
)

func TestConcurrentCLIProcessesPreserveRegistrations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "worktree-bench")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/worktree-bench")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, out)
	}
	repo := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-b", "master"}, {"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"}, {"commit", "--allow-empty", "-m", "base"},
	} {
		if out, err := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	child := filepath.Join(filepath.Dir(repo), "child")
	if out, err := exec.CommandContext(ctx, "git", "-C", repo, "worktree", "add", "-b", "child", child).CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, out)
	}
	settings := config.Settings{WorktreesDir: filepath.Join(filepath.Dir(repo), "benches")}
	if err := config.SaveSettings(repo, settings); err != nil {
		t.Fatal(err)
	}
	var processes []*exec.Cmd
	var outputs, errors []*bytes.Buffer
	_, err := config.UpdatePool(repo, settings, func(pool config.Pool) (config.Pool, error) {
		// Start both actual CLIs while the main process owns the shared lock.
		// Each must read the registry after this transaction commits.
		for _, cwd := range []string{repo, child} {
			command := exec.CommandContext(ctx, binary, "--new", "--type", "medium", "--base", "master", "--json", "--init-cmd", "-")
			command.Dir = cwd
			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			command.Stdout, command.Stderr = stdout, stderr
			if err := command.Start(); err != nil {
				return pool, err
			}
			processes = append(processes, command)
			outputs, errors = append(outputs, stdout), append(errors, stderr)
		}
		pool.Benches = append(pool.Benches, config.Workbench{ID: "legacy", Name: "project-m-4", Type: config.TypeMedium})
		return pool, nil
	})
	// Reap every started process even if another process or the transaction failed.
	for i, command := range processes {
		if waitErr := command.Wait(); waitErr != nil {
			t.Errorf("CLI %d: %v: %s", i, waitErr, errors[i])
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, output := range outputs {
		var selected selectionJSON
		if err := json.Unmarshal(output.Bytes(), &selected); err != nil {
			t.Fatalf("CLI output %q: %v", output, err)
		}
		if !selected.Created {
			t.Fatal("expected a newly created workbench")
		}
		names = append(names, selected.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "project-m-5" || names[1] != "project-m-6" {
		t.Fatalf("concurrent names: %v", names)
	}
	pool, err := config.LoadPool(child, settings)
	if err != nil || len(pool.Benches) != 3 || pool.Benches[0].ID != "legacy" {
		t.Fatalf("lost registration: %+v, %v", pool, err)
	}
}
