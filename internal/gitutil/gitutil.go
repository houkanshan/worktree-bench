package gitutil

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// MainWorktreeRoot returns the root of the main worktree for the repository.
func MainWorktreeRoot(path string) (string, error) {
	commonDir, err := GitCommonDir(path)
	if err != nil {
		return "", err
	}
	commonDir = filepath.Clean(commonDir)
	if filepath.Base(commonDir) == ".git" {
		return filepath.Dir(commonDir), nil
	}
	sep := string(filepath.Separator)
	needle := sep + ".git" + sep
	if idx := strings.Index(commonDir, needle); idx != -1 {
		root := commonDir[:idx]
		if root == "" {
			root = sep
		}
		return root, nil
	}
	return path, nil
}

func Branch(path string) (string, error) {
	return BranchOrShortHEAD(path)
}

func BranchOrShortHEAD(path string) (string, error) {
	args := []string{"-C", path, "symbolic-ref", "--short", "-q", "HEAD"}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	args = []string{"-C", path, "rev-parse", "--short", "HEAD"}
	out, err = exec.Command("git", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args[2:], " "), detail)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args[2:], " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func GitCommonDir(path string) (string, error) {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", gitOutputError("rev-parse --git-common-dir", err)
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
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return 0, 0, fmt.Errorf("git %s: %s", strings.Join(args[2:], " "), detail)
		}
		return 0, 0, fmt.Errorf("git %s: %w", strings.Join(args[2:], " "), err)
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
	return runGit(path, "fetch", "--all", "--prune")
}

// ResolvePrimaryBranch returns the best default base branch for a repository.
// It prefers master when present for backward compatibility, then falls back to main.
func ResolvePrimaryBranch(path string) (string, error) {
	candidates := []string{"master", "main"}
	for _, branch := range candidates {
		exists, err := gitRefExists(path, branch)
		if err != nil {
			return "", err
		}
		if exists {
			return branch, nil
		}

		exists, err = gitRefExists(path, "origin/"+branch)
		if err != nil {
			return "", err
		}
		if exists {
			return branch, nil
		}
	}

	return "master", nil
}

func CheckoutBranch(path, branch string) error {
	if err := runGit(path, "checkout", branch); err == nil {
		return nil
	}
	if err := runGit(path, "checkout", "-b", branch, "origin/"+branch); err == nil {
		return nil
	}
	return runGit(path, "checkout", "-b", branch)
}

func CheckoutNewBranch(path, branch, base string) error {
	if base == "" {
		return runGit(path, "checkout", "-b", branch)
	}
	if err := runGit(path, "checkout", "-b", branch, base); err == nil {
		return nil
	} else if strings.HasPrefix(base, "origin/") {
		return err
	}
	if err := runGit(path, "checkout", "-b", branch, "origin/"+base); err == nil {
		return nil
	} else {
		return err
	}
}

func gitRefExists(path, ref string) (bool, error) {
	args := []string{"rev-parse", "--verify", "--quiet", ref + "^{commit}"}
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	detail := strings.TrimSpace(string(out))
	if detail != "" {
		return false, fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return false, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

func WorktreeAdd(repoRoot, path, branch, base string) error {
	attempt := func(baseRef string) error {
		args := []string{"worktree", "add"}
		if branch != "" {
			args = append(args, "-b", branch)
		}
		args = append(args, path)
		if baseRef != "" {
			args = append(args, baseRef)
		}
		return runGit(repoRoot, args...)
	}
	if err := attempt(base); err == nil {
		return nil
	} else if base == "" || strings.HasPrefix(base, "origin/") {
		return err
	}
	if err := attempt("origin/" + base); err == nil {
		return nil
	} else {
		return err
	}
}

func WorktreeMove(repoRoot, oldPath, newPath string) error {
	return runGit(repoRoot, "worktree", "move", oldPath, newPath)
}

func WorktreeRemove(repoRoot, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	return runGit(repoRoot, args...)
}

func DeleteBranch(repoRoot, branch string) error {
	return runGit(repoRoot, "branch", "-D", branch)
}

// LastCommitTime returns the author date of the most recent commit in the repo at path.
func LastCommitTime(path string) (time.Time, error) {
	t, _, err := LastCommitInfo(path)
	return t, err
}

// LastCommitSubject returns the first line of the most recent commit message.
func LastCommitSubject(path string) (string, error) {
	_, subject, err := LastCommitInfo(path)
	return subject, err
}

func LastCommitInfo(path string) (time.Time, string, error) {
	args := []string{"-C", path, "log", "-1", "--format=%aI%x00%s"}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return time.Time{}, "", fmt.Errorf("git %s: %s", strings.Join(args[2:], " "), detail)
		}
		return time.Time{}, "", fmt.Errorf("git %s: %w", strings.Join(args[2:], " "), err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\x00", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("git log: unexpected output")
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
	if err != nil {
		return time.Time{}, "", err
	}
	return t, firstLine(parts[1]), nil
}

// BranchDescription returns the configured branch description, if any.
func BranchDescription(path, branch string) (string, error) {
	if branch == "" {
		return "", nil
	}
	key := fmt.Sprintf("branch.%s.description", branch)
	args := []string{"-C", path, "config", "--get", key}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args[2:], " "), detail)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args[2:], " "), err)
	}
	return firstLine(strings.TrimSpace(string(out))), nil
}

func firstLine(text string) string {
	parts := strings.SplitN(text, "\n", 2)
	return strings.TrimSpace(parts[0])
}

// gitOutputError wraps an error from exec.Command().Output() with the git
// subcommand name and any stderr the process produced.
func gitOutputError(subcmd string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		detail := strings.TrimSpace(string(exitErr.Stderr))
		if detail != "" {
			return fmt.Errorf("git %s: %s", subcmd, detail)
		}
	}
	return fmt.Errorf("git %s: %w", subcmd, err)
}

// runGit executes a git command with -C path, capturing stderr for error context.
func runGit(path string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
		}
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
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
