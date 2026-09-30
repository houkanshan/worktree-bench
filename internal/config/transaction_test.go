package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func transactionRepo(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return repo
}

func TestMissingPoolReadDoesNotWrite(t *testing.T) {
	repo := transactionRepo(t)
	pool, err := LoadPool(repo, Settings{})
	if err != nil || len(pool.Benches) != 0 || pool.RepoRoot != repo {
		t.Fatalf("pool: %+v, %v", pool, err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".worktree-bench")); !os.IsNotExist(err) {
		t.Fatalf("read created configuration: %v", err)
	}
}

func TestPoolTransactionLockAndRelease(t *testing.T) {
	repo := transactionRepo(t)
	settings := Settings{}
	failure := errors.New("mutation failed")
	_, err := UpdatePool(repo, settings, func(pool Pool) (Pool, error) {
		lock, err := os.OpenFile(filepath.Join(repo, ".worktree-bench", "pool.lock"), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("transaction did not hold exclusive lock: %v", err)
		}
		pool.Benches = append(pool.Benches, Workbench{ID: "not-saved"})
		return pool, failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("mutation error = %v", err)
	}
	// A nonblocking probe makes a leaked lock fail rather than hang the test.
	assertUnlocked := func() {
		lock, err := os.OpenFile(filepath.Join(repo, ".worktree-bench", "pool.lock"), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatalf("lock not released: %v", err)
		}
	}
	assertUnlocked()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected callback panic")
			}
		}()
		_, _ = UpdatePool(repo, settings, func(Pool) (Pool, error) { panic("test panic") })
	}()
	assertUnlocked()
	_, err = UpdatePool(repo, settings, func(pool Pool) (Pool, error) {
		if len(pool.Benches) != 0 {
			t.Fatal("failed mutation was saved")
		}
		pool.Benches = append(pool.Benches, Workbench{ID: "saved"})
		return pool, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertUnlocked()
	pool, err := LoadPool(repo, settings)
	if err != nil || len(pool.Benches) != 1 || pool.Benches[0].ID != "saved" {
		t.Fatalf("saved pool: %+v, %v", pool, err)
	}
}

func TestRemovedNamePrefixDoesNotAffectBranchPrefix(t *testing.T) {
	repo := transactionRepo(t)
	dir := filepath.Join(repo, ".worktree-bench")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, SettingsFileName)
	if err := os.WriteFile(path, []byte(`{"worktree_name_prefix":"old-","branch_prefix":"custom/"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettings(repo)
	if err != nil || settings.BranchPrefix != "custom/" {
		t.Fatalf("settings: %+v, %v", settings, err)
	}
	if err := SaveSettings(repo, settings); err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := readJSON(path, &saved); err != nil {
		t.Fatal(err)
	}
	if _, present := saved["worktree_name_prefix"]; present {
		t.Fatal("removed setting was persisted")
	}
}
