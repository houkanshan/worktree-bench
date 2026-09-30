package gitutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
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

// ErrMainWorktreeUnknown means Git metadata cannot identify the main working
// tree. Callers must not choose another configuration location in this case.
var ErrMainWorktreeUnknown = errors.New("cannot identify main worktree; configure core.worktree in the common Git config")

// MainWorktreeRoot returns the root of the main worktree for the repository.
func MainWorktreeRoot(path string) (string, error) {
	commonDir, err := GitCommonDir(path)
	if err != nil {
		return "", err
	}
	// Submodules (and configured separate gitdirs) record the main worktree
	// relative to their common gitdir. Git's worktree list reports the gitdir
	// itself for these repositories, not the working tree.
	out, err := exec.Command("git", "--git-dir", commonDir, "config", "--get", "core.worktree").CombinedOutput()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if !filepath.IsAbs(root) {
			root = filepath.Join(commonDir, root)
		}
		commonInfo, commonErr := os.Stat(commonDir)
		rootInfo, rootErr := os.Stat(root)
		if commonErr == nil && rootErr == nil && os.SameFile(commonInfo, rootInfo) {
			return "", ErrMainWorktreeUnknown
		}
		return filepath.Clean(root), nil
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		return "", fmt.Errorf("git config --get core.worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// Ordinary repositories list the main worktree first, even from a child.
	out, err = exec.Command("git", "-C", path, "worktree", "list", "--porcelain", "-z").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree list: %w: %s", err, strings.TrimSpace(string(out)))
	}
	field, _, _ := strings.Cut(string(out), "\x00")
	root, ok := strings.CutPrefix(field, "worktree ")
	if !ok || root == "" {
		return "", fmt.Errorf("git worktree list: missing main worktree")
	}
	commonInfo, commonErr := os.Stat(commonDir)
	rootInfo, rootErr := os.Stat(root)
	if commonErr == nil && rootErr == nil && os.SameFile(commonInfo, rootInfo) {
		// An unconfigured separate gitdir reports metadata as its main entry.
		// Only a main-worktree invocation can recover the real working tree:
		// its gitdir equals the common gitdir, unlike a linked child's gitdir.
		out, err = exec.Command("git", "-C", path, "rev-parse", "--absolute-git-dir").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%w: git rev-parse --absolute-git-dir: %v: %s", ErrMainWorktreeUnknown, err, strings.TrimSpace(string(out)))
		}
		gitInfo, err := os.Stat(strings.TrimSpace(string(out)))
		if err != nil || !os.SameFile(commonInfo, gitInfo) {
			return "", ErrMainWorktreeUnknown
		}
		out, err = exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%w: git rev-parse --show-toplevel: %v: %s", ErrMainWorktreeUnknown, err, strings.TrimSpace(string(out)))
		}
		root = strings.TrimSpace(string(out))
		rootInfo, err = os.Stat(root)
		if err != nil || os.SameFile(commonInfo, rootInfo) {
			return "", ErrMainWorktreeUnknown
		}
	}
	return filepath.Clean(root), nil
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
	startPoint, local, err := ResolveBranch(path, branch)
	if err != nil {
		return err
	}
	if local {
		return runGit(path, "checkout", branch)
	}
	remote, _, ok := strings.Cut(startPoint, "/")
	if !ok {
		return fmt.Errorf("branch %q has invalid remote start point %q", branch, startPoint)
	}
	return runGit(path, "checkout", "-b", branch, "--track", remote+"/"+branch)
}

// ResolveBranch validates a branch and fetches its remote-tracking ref when the
// branch exists only on a remote. It returns a commit-ish start point and
// whether the branch already exists locally, without changing any worktree.
func ResolveBranch(path, branch string) (string, bool, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.HasPrefix(branch, "-") {
		return "", false, fmt.Errorf("invalid branch %q", branch)
	}
	if err := runGit(path, "check-ref-format", "--branch", branch); err != nil {
		return "", false, err
	}

	localRef := "refs/heads/" + branch
	local, err := gitRefExists(path, localRef)
	if err != nil {
		return "", false, err
	}
	if local {
		return branch, true, nil
	}

	remotes, err := checkoutRemotes(path)
	if err != nil {
		return "", false, err
	}
	for _, remote := range remotes {
		exists, err := remoteBranchExists(path, remote, branch)
		if err != nil || !exists {
			continue
		}
		remoteRef := "refs/remotes/" + remote + "/" + branch
		refspec := "+refs/heads/" + branch + ":" + remoteRef
		if err := runGit(path, "fetch", remote, refspec); err != nil {
			return "", false, err
		}
		return remote + "/" + branch, false, nil
	}
	return "", false, fmt.Errorf("branch %q not found", branch)
}

func ResolveCommit(path, ref string) (string, error) {
	args := []string{"-C", path, "rev-parse", "--verify", ref + "^{commit}"}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return "", fmt.Errorf("git rev-parse --verify: %s", detail)
		}
		return "", fmt.Errorf("git rev-parse --verify: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func checkoutRemotes(path string) ([]string, error) {
	args := []string{"-C", path, "remote"}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return nil, gitOutputError("remote", err)
	}
	seen := make(map[string]bool)
	remotes := make([]string, 0)
	add := func(remote string) {
		remote = strings.TrimSpace(remote)
		if remote != "" && !seen[remote] {
			seen[remote] = true
			remotes = append(remotes, remote)
		}
	}
	upstreamArgs := []string{"-C", path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"}
	if upstream, upstreamErr := exec.Command("git", upstreamArgs...).CombinedOutput(); upstreamErr == nil {
		if remote, _, ok := strings.Cut(strings.TrimSpace(string(upstream)), "/"); ok {
			add(remote)
		}
	}
	add("origin")
	for _, remote := range strings.Fields(string(out)) {
		add(remote)
	}
	return remotes, nil
}

func remoteBranchExists(path, remote, branch string) (bool, error) {
	args := []string{"-C", path, "ls-remote", "--exit-code", "--heads", remote, "refs/heads/" + branch}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)) != "", nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return false, nil
	}
	detail := strings.TrimSpace(string(out))
	if detail != "" {
		return false, fmt.Errorf("git ls-remote --heads %s: %s", remote, detail)
	}
	return false, fmt.Errorf("git ls-remote --heads %s: %w", remote, err)
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
