package bench

import (
	"os"
	"path/filepath"
	"testing"

	"worktree-bench/internal/config"
)

func TestAutomaticNamesFromMainAndChildWorktrees(t *testing.T) {
	repo := initializedRepo(t)
	child := filepath.Join(filepath.Dir(repo), "unrelated-child")
	gitTest(t, repo, "worktree", "add", "-b", "child", child)
	settings := config.Settings{WorktreesDir: filepath.Join(filepath.Dir(repo), "benches")}
	for _, root := range []string{repo, child} {
		for _, test := range []struct{ kind, prefix string }{
			{config.TypeLarge, "repo-"}, {config.TypeMedium, "repo-m-"}, {config.TypeSmall, "repo-s-"},
		} {
			want := test.prefix + "1"
			if root == child {
				want = test.prefix + "2"
			}
			_, created, _, err := CreateWorkbench(root, settings, CreateInput{Type: test.kind, BaseBranch: "master"})
			if err != nil {
				t.Fatal(err)
			}
			if created.Name != want {
				t.Fatalf("root %s: name %q, want %q", root, created.Name, want)
			}
		}
	}
	_, explicit, _, err := CreateWorkbench(child, settings, CreateInput{Type: config.TypeSmall, Name: "custom-name", BaseBranch: "master"})
	if err != nil || explicit.Name != "custom-name" {
		t.Fatalf("explicit name: %+v, %v", explicit, err)
	}
}

func TestNextNameGapsLegacyNamesAndCollisions(t *testing.T) {
	repo := initializedRepo(t)
	dir := filepath.Join(filepath.Dir(repo), "benches")
	pool := config.Pool{Benches: []config.Workbench{
		{Name: "repo-1", Type: config.TypeLarge},
		{Name: "repo-24", Type: "full"}, // Existing legacy series continues without renaming.
		{Name: "repo-l-99", Type: config.TypeLarge},
		{Name: "custom", Type: config.TypeLarge},
		{Name: "repo-m-7", Type: config.TypeSmall}, // Names are reserved regardless of type.
		{Name: "repo-s-2", Type: config.TypeSmall},
	}}
	for kind, want := range map[string]string{config.TypeLarge: "repo-25", config.TypeMedium: "repo-m-8", config.TypeSmall: "repo-s-3"} {
		got, err := nextName(repo, dir, pool, kind)
		if err != nil || got != want {
			t.Fatalf("%s: got %q, %v; want %q", kind, got, err, want)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A dangling symlink is also an occupied filesystem name.
	if err := os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "repo-26")); err != nil {
		t.Fatal(err)
	}
	// Git keeps a worktree registration even if its directory disappears.
	leftover := filepath.Join(dir, "repo-28")
	gitTest(t, repo, "worktree", "add", "--detach", leftover, "master")
	if err := os.RemoveAll(leftover); err != nil {
		t.Fatal(err)
	}
	got, err := nextName(repo, dir, pool, config.TypeLarge)
	if err != nil || got != "repo-29" {
		t.Fatalf("leftovers: got %q, %v; want repo-29", got, err)
	}
}

func TestAdoptAndDeleteUseFreshPool(t *testing.T) {
	repo := initializedRepo(t)
	settings := config.Settings{WorktreesDir: filepath.Join(filepath.Dir(repo), "benches")}
	_, created, _, err := CreateWorkbench(repo, settings, CreateInput{Type: config.TypeSmall, BaseBranch: "master"})
	if err != nil {
		t.Fatal(err)
	}
	pool, adopted, err := AdoptWorkbench(repo, settings, AdoptInput{Type: config.TypeLarge})
	if err != nil || len(pool.Benches) != 2 {
		t.Fatalf("adopt: %+v, %v", pool, err)
	}
	if _, _, err := AdoptWorkbench(repo, settings, AdoptInput{Type: config.TypeLarge}); err == nil {
		t.Fatal("duplicate adoption succeeded")
	}
	pool, _, err = DeleteWorkbench(repo, settings, DeleteInput{BenchID: created.ID})
	if err != nil || len(pool.Benches) != 1 || pool.Benches[0].ID != adopted.ID {
		t.Fatalf("delete lost independent registration: %+v, %v", pool, err)
	}
	if _, _, err := DeleteWorkbench(repo, settings, DeleteInput{BenchID: created.ID}); err == nil {
		t.Fatal("stale deletion succeeded")
	}
}
