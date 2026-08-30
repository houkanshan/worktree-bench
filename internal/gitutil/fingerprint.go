package gitutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WorktreeGitDir(path string) (string, error) {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return gitPath, nil
	}
	payload, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(payload))
	const prefix = "gitdir:"
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return "", fmt.Errorf("invalid .git file in %s", path)
	}
	gitDir := strings.TrimSpace(line[len(prefix):])
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(path, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

func WorktreeFingerprint(path string) (string, error) {
	h := sha256.New()
	addString := func(s string) { _, _ = h.Write([]byte(s)); _, _ = h.Write([]byte{0}) }
	addStat := func(name string) {
		info, err := os.Stat(name)
		if err != nil {
			addString("missing:" + name)
			return
		}
		addString(fmt.Sprintf("stat:%s:%d:%d:%t", name, info.Size(), info.ModTime().UnixNano(), info.IsDir()))
	}
	addFile := func(name string) {
		payload, err := os.ReadFile(name)
		if err != nil {
			addString("read-missing:" + name)
			return
		}
		addString("file:" + name + ":" + string(payload))
	}

	cleanPath := filepath.Clean(path)
	addString("path:" + cleanPath)
	addStat(cleanPath)
	gitDot := filepath.Join(cleanPath, ".git")
	addStat(gitDot)
	if payload, err := os.ReadFile(gitDot); err == nil {
		addString("dotgit:" + string(payload))
	}

	gitDir, err := WorktreeGitDir(cleanPath)
	if err != nil {
		return "", err
	}
	addString("gitdir:" + gitDir)
	commonDir := gitDir
	if payload, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		commonDir = strings.TrimSpace(string(payload))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(gitDir, commonDir)
		}
		commonDir = filepath.Clean(commonDir)
	}
	addString("commondir:" + commonDir)

	headPath := filepath.Join(gitDir, "HEAD")
	addFile(headPath)
	if payload, err := os.ReadFile(headPath); err == nil {
		head := strings.TrimSpace(string(payload))
		if strings.HasPrefix(head, "ref:") {
			ref := strings.TrimSpace(strings.TrimPrefix(head, "ref:"))
			addStat(filepath.Join(commonDir, filepath.FromSlash(ref)))
		} else {
			addString("detached:" + head)
		}
	}

	for _, name := range []string{
		filepath.Join(commonDir, "packed-refs"),
		filepath.Join(gitDir, "index"),
		filepath.Join(gitDir, "MERGE_HEAD"),
		filepath.Join(gitDir, "CHERRY_PICK_HEAD"),
		filepath.Join(gitDir, "REVERT_HEAD"),
		filepath.Join(gitDir, "rebase-apply"),
		filepath.Join(gitDir, "rebase-merge"),
		filepath.Join(commonDir, "config"),
		filepath.Join(gitDir, "config.worktree"),
	} {
		addStat(name)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
