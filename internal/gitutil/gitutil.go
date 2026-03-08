package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

func Branch(path string) (string, error) {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func GitCommonDir(path string) (string, error) {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(string(out))
	if filepath.IsAbs(common) {
		return common, nil
	}
	return filepath.Join(path, common), nil
}

func StatusNumstat(path string, staged bool) (int, int, error) {
	args := []string{"-C", path, "diff", "--numstat"}
	if staged {
		args = append(args, "--cached")
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return 0, 0, err
	}
	added, deleted := parseNumstat(out)
	return added, deleted, nil
}

func parseNumstat(out []byte) (int, int) {
	lines := bytes.Split(out, []byte{'\n'})
	added := 0
	deleted := 0
	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "-" || fields[1] == "-" {
			continue
		}
		addVal := atoi(fields[0])
		delVal := atoi(fields[1])
		added += addVal
		deleted += delVal
	}
	return added, deleted
}

func atoi(value string) int {
	var n int
	fmt.Sscanf(value, "%d", &n)
	return n
}

func Fetch(path string) error {
	cmd := exec.Command("git", "-C", path, "fetch", "--all", "--prune")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func CheckoutBranch(path, branch string) error {
	cmd := exec.Command("git", "-C", path, "checkout", branch)
	if err := cmd.Run(); err == nil {
		return nil
	}
	cmd = exec.Command("git", "-C", path, "checkout", "-b", branch, "origin/"+branch)
	if err := cmd.Run(); err == nil {
		return nil
	}
	cmd = exec.Command("git", "-C", path, "checkout", "-b", branch)
	return cmd.Run()
}

func WorktreeAdd(repoRoot, path, branch string) error {
	args := []string{"-C", repoRoot, "worktree", "add", path}
	if branch != "" {
		args = append(args, branch)
	}
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

func WorktreeMove(repoRoot, oldPath, newPath string) error {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "move", oldPath, newPath)
	return cmd.Run()
}

func WorktreeRemove(repoRoot, path string, force bool) error {
	args := []string{"-C", repoRoot, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

func DiffLines(path string) (int, error) {
	added1, deleted1, err := StatusNumstat(path, false)
	if err != nil {
		return 0, err
	}
	added2, deleted2, err := StatusNumstat(path, true)
	if err != nil {
		return 0, err
	}
	return added1 + deleted1 + added2 + deleted2, nil
}
