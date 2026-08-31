package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func canonicalPath(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(absolute)
	if parent == absolute {
		return filepath.Clean(absolute), nil
	}
	canonicalParent, err := canonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(canonicalParent, filepath.Base(absolute)), nil
}

func pathInside(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func pathAllowed(candidate string, roots []string) (bool, error) {
	if len(roots) == 0 {
		return true, nil
	}
	canonicalCandidate, err := canonicalPath(candidate)
	if err != nil {
		return false, fmt.Errorf("resolve path %q: %w", candidate, err)
	}
	for _, root := range roots {
		canonicalRoot, err := canonicalPath(root)
		if err != nil {
			return false, fmt.Errorf("resolve allowed root %q: %w", root, err)
		}
		if pathInside(canonicalRoot, canonicalCandidate) {
			return true, nil
		}
	}
	return false, nil
}

func requireAllowedPath(candidate string, roots []string) error {
	allowed, err := pathAllowed(candidate, roots)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("path is outside allowed roots: %s", candidate)
	}
	return nil
}
