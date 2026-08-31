package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathAllowedUsesCanonicalBoundaries(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "benches", "one")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	allowed, err := pathAllowed(inside, []string{root})
	if err != nil || !allowed {
		t.Fatalf("inside path rejected: allowed=%t err=%v", allowed, err)
	}
	allowed, err = pathAllowed(root+"-outside", []string{root})
	if err != nil || allowed {
		t.Fatalf("prefix sibling accepted: allowed=%t err=%v", allowed, err)
	}
}

func TestPathAllowedRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	allowed, err := pathAllowed(link, []string{root})
	if err != nil || allowed {
		t.Fatalf("escaping symlink accepted: allowed=%t err=%v", allowed, err)
	}
}
