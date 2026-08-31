package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"worktree-bench/internal/config"
)

func directSelectionForTest(t *testing.T, flags []string, args []string) (directSelection, bool, error) {
	t.Helper()
	command := &cobra.Command{}
	addNoTUISelectionFlags(command)
	if err := command.Flags().Parse(flags); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return parseDirectSelection(command, args)
}

func TestParseDirectSelectionJSONExisting(t *testing.T) {
	selection, direct, err := directSelectionForTest(t, []string{"--json", "--init-cmd", "gnm"}, []string{"wb-123"})
	if err != nil {
		t.Fatal(err)
	}
	if !direct || !selection.JSON || selection.BenchID != "wb-123" || selection.CreateNew {
		t.Fatalf("unexpected selection: %+v direct=%t", selection, direct)
	}
}

func TestParseDirectSelectionJSONNew(t *testing.T) {
	selection, direct, err := directSelectionForTest(t, []string{"--new", "--type", "large", "--json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !direct || !selection.JSON || !selection.CreateNew || selection.Type != "large" {
		t.Fatalf("unexpected selection: %+v direct=%t", selection, direct)
	}
}

func TestParseDirectSelectionCarriesCheckoutTarget(t *testing.T) {
	selection, direct, err := directSelectionForTest(t, []string{"--json", "--checkout", "#42"}, []string{"wb-123"})
	if err != nil {
		t.Fatal(err)
	}
	if !direct || selection.CheckoutTarget != "#42" {
		t.Fatalf("unexpected selection: %+v direct=%t", selection, direct)
	}
	if _, _, err := directSelectionForTest(t, []string{"--checkout", "feature", "--init-cmd", "gnm"}, []string{"wb-123"}); err == nil {
		t.Fatal("expected checkout and init-cmd to conflict")
	}
}

func TestParseDirectSelectionCarriesSafetyBoundary(t *testing.T) {
	selection, direct, err := directSelectionForTest(t, []string{"--json", "--require-reusable", "--allowed-root", "/tmp/one", "--allowed-root", "/tmp/two"}, []string{"wb-123"})
	if err != nil {
		t.Fatal(err)
	}
	if !direct || !selection.RequireReusable || len(selection.AllowedRoots) != 2 || selection.AllowedRoots[1] != "/tmp/two" {
		t.Fatalf("unexpected selection: %+v direct=%t", selection, direct)
	}
}

func TestParseDirectSelectionFlagsRequireTarget(t *testing.T) {
	for _, flags := range [][]string{{"--json"}, {"--require-reusable"}, {"--checkout", "feature"}, {"--allowed-root", "/tmp"}} {
		if _, _, err := directSelectionForTest(t, flags, nil); err == nil {
			t.Fatalf("expected %v without a direct target to fail", flags)
		}
	}
}

func TestDirectSelectionRechecksReusableBeforeInit(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	tracked := filepath.Join(repo, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "tracked.txt"}, {"commit", "-m", "initial"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	remote := filepath.Join(filepath.Dir(repo), "origin.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init remote: %s", out)
	}
	for _, args := range [][]string{{"remote", "add", "origin", remote}, {"push", "-u", "origin", "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	marker := filepath.Join(repo, "initialized")
	settings := config.Settings{InitCmd: "touch " + marker}
	pool := config.Pool{Benches: []config.Workbench{{ID: "wb-123", Name: "test", Type: "large", Path: repo}}}
	direct := directSelection{BenchID: "wb-123", RequireReusable: true, AllowedRoots: []string{repo}, JSON: true}
	if err := runDirectSelection(&cobra.Command{}, repo, settings, pool, dashboardOptions{}, direct); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("init command did not run: %v", err)
	}

	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runDirectSelection(&cobra.Command{}, repo, settings, pool, dashboardOptions{}, direct); err == nil {
		t.Fatal("expected dirty workbench to fail reusable check")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("init command ran after failed reusable check: %v", err)
	}
}

func TestDirectSelectionCheckoutReplacesInit(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "master"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "tracked.txt"}, {"commit", "-m", "initial"}, {"branch", "feature"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	marker := filepath.Join(repo, "initialized")
	settings := config.Settings{InitCmd: "touch " + marker}
	pool := config.Pool{Benches: []config.Workbench{{ID: "wb-123", Name: "test", Type: "large", Path: repo}}}
	direct := directSelection{BenchID: "wb-123", CheckoutTarget: "feature", AllowedRoots: []string{repo}, JSON: true}
	if err := runDirectSelection(&cobra.Command{}, repo, settings, pool, dashboardOptions{}, direct); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("init command ran during checkout: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "branch", "--show-current").CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "feature" {
		t.Fatalf("current branch = %q, err=%v", out, err)
	}
}

func TestSelectionJSONContract(t *testing.T) {
	payload, err := json.Marshal(selectionJSON{
		BenchID: "wb-123",
		Name:    "repo-l-1",
		Type:    "large",
		Path:    "/tmp/repo-l-1",
		Created: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"benchId":"wb-123","name":"repo-l-1","type":"large","path":"/tmp/repo-l-1","created":true}`
	if string(payload) != want {
		t.Fatalf("unexpected JSON: %s", payload)
	}
}
