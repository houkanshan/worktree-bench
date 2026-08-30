package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/statuscache"
)

func TestLoadBenchStatusesCachedHitAvoidsGitLoader(t *testing.T) {
	repoRoot := t.TempDir()
	runCmd(t, repoRoot, "git", "init")
	benchPath := filepath.Join(t.TempDir(), "fake-wt")
	if err := os.MkdirAll(filepath.Join(benchPath, ".git", "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(benchPath, ".git", "HEAD"), "ref: refs/heads/bench/test\n")
	write(t, filepath.Join(benchPath, ".git", "refs", "heads", "bench-test"), "abc\n")
	write(t, filepath.Join(benchPath, ".git", "index"), "")
	bench := config.Workbench{ID: "wb-1", Name: "one", Type: config.TypeLarge, Path: benchPath}
	fp, err := gitutil.WorktreeFingerprint(benchPath)
	if err != nil {
		t.Fatal(err)
	}
	fp = fp + ":" + bench.ID + ":" + bench.Type + ":" + bench.Name
	cache := statuscache.Cache{}
	statuscache.Put(&cache, bench, fp, statuscache.BenchStatus{Description: "cached", Branch: "bench/test"})
	if err := statuscache.Save(repoRoot, cache); err != nil {
		t.Fatal(err)
	}
	statuses := LoadBenchStatusesCached(repoRoot, []config.Workbench{bench}, StatusOptions{UseCache: true, Fast: true})
	if statuses[bench.ID].Description != "cached" {
		t.Fatalf("cache not used: %+v", statuses[bench.ID])
	}
}

func TestLoadBenchStatusesCachedMissingDir(t *testing.T) {
	repoRoot := t.TempDir()
	runCmd(t, repoRoot, "git", "init")
	bench := config.Workbench{ID: "wb-1", Name: "one", Type: config.TypeLarge, Path: filepath.Join(t.TempDir(), "missing")}
	statuses := LoadBenchStatusesCached(repoRoot, []config.Workbench{bench}, StatusOptions{UseCache: true, Fast: true})
	if !statuses[bench.ID].DirMissing {
		t.Fatalf("expected missing dir: %+v", statuses[bench.ID])
	}
	statuses = LoadBenchStatusesCached(repoRoot, []config.Workbench{bench}, StatusOptions{UseCache: true, Fast: true})
	if !statuses[bench.ID].DirMissing {
		t.Fatalf("expected cached missing dir: %+v", statuses[bench.ID])
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
