package statuscache

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"worktree-bench/internal/config"
)

func TestCorruptCacheIgnored(t *testing.T) {
	repoRoot := t.TempDir()
	run(t, repoRoot, "git", "init")
	configDir := filepath.Join(repoRoot, ".worktree-bench")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, FileName), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache, err := Load(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if cache.Version != Version || len(cache.Entries) != 0 {
		t.Fatalf("unexpected cache: %+v", cache)
	}
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

func TestGetPutRoundTrip(t *testing.T) {
	cache := Cache{}
	bench := config.Workbench{ID: "wb-1", Path: "/tmp/wt"}
	status := BenchStatus{Description: "desc", Branch: "bench/test", LastCommit: time.Now(), ChangesLines: 3}
	Put(&cache, bench, "fp", status)
	got, ok := Get(cache, bench, "fp")
	if !ok {
		t.Fatal("cache miss")
	}
	if got.Description != status.Description || got.Branch != status.Branch || got.ChangesLines != status.ChangesLines {
		t.Fatalf("unexpected status: %+v", got)
	}
	if _, ok := Get(cache, bench, "other"); ok {
		t.Fatal("fingerprint mismatch hit cache")
	}
}
