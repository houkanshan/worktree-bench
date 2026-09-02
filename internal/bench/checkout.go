package bench

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"worktree-bench/internal/gitutil"
)

var prPattern = regexp.MustCompile(`^(#|pr:)?(\d+)$`)

type ResolvedTarget struct {
	Kind              string `json:"kind"`
	Commit            string `json:"commit"`
	BranchName        string `json:"branchName"`
	ExistingPath      string `json:"existingPath,omitempty"`
	CrossRepositoryPR bool   `json:"crossRepositoryPr,omitempty"`
}

type CheckoutResult struct {
	ResolvedTarget
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
}

type worktreeEntry struct {
	Path   string
	Commit string
	Branch string
}

func ResolveTarget(path, ref string) (ResolvedTarget, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ResolvedTarget{}, fmt.Errorf("missing checkout target")
	}

	var target ResolvedTarget
	var err error
	if number, ok := parsePRNumber(ref); ok {
		target, err = resolvePRTarget(path, ref, number)
	} else {
		target, err = resolveBranchTarget(path, ref)
	}
	if err != nil {
		return ResolvedTarget{}, err
	}
	entries, err := listWorktrees(path)
	if err != nil {
		return ResolvedTarget{}, err
	}
	target.ExistingPath = matchingWorktree(entries, target)
	return target, nil
}

func CheckoutTarget(path, ref string) (CheckoutResult, error) {
	target, err := ResolveTarget(path, ref)
	if err != nil {
		return CheckoutResult{}, err
	}
	if target.ExistingPath != "" {
		return checkoutResult(target, target.ExistingPath, "existing"), nil
	}

	if target.Kind == "pullRequest" {
		number, _ := parsePRNumber(strings.TrimSpace(ref))
		err = ghCheckoutPR(path, number)
	} else {
		err = gitutil.CheckoutBranch(path, target.BranchName)
	}
	if err == nil {
		return checkoutResult(target, path, "checkedOut"), nil
	}

	// Another process may have checked out the target after the first
	// resolution. Reconcile from Git state instead of interpreting stderr.
	reconciled, reconcileErr := ResolveTarget(path, ref)
	if reconcileErr == nil && reconciled.ExistingPath != "" {
		return checkoutResult(reconciled, reconciled.ExistingPath, "existing"), nil
	}
	return CheckoutResult{}, err
}

func checkoutResult(target ResolvedTarget, path, disposition string) CheckoutResult {
	target.ExistingPath = ""
	return CheckoutResult{ResolvedTarget: target, Path: path, Disposition: disposition}
}

func resolveBranchTarget(path, branch string) (ResolvedTarget, error) {
	startPoint, _, err := gitutil.ResolveBranch(path, branch)
	if err != nil {
		return ResolvedTarget{}, err
	}
	commit, err := gitutil.ResolveCommit(path, startPoint)
	if err != nil {
		return ResolvedTarget{}, err
	}
	return ResolvedTarget{Kind: "branch", Commit: commit, BranchName: branch}, nil
}

type prView struct {
	HeadRefName       string `json:"headRefName"`
	HeadRefOID        string `json:"headRefOid"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	Number            int    `json:"number"`
	URL               string `json:"url"`
}

func resolvePRTarget(path, original string, number int) (ResolvedTarget, error) {
	args := []string{"pr", "view", original, "--json", "headRefName,headRefOid,isCrossRepository,number,url"}
	cmd := exec.Command("gh", args...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ResolvedTarget{}, commandError("gh "+strings.Join(args, " "), out, err)
	}
	var view prView
	if err := json.Unmarshal(out, &view); err != nil {
		return ResolvedTarget{}, fmt.Errorf("gh pr view returned invalid JSON: %w", err)
	}
	if view.Number != number || strings.TrimSpace(view.HeadRefName) == "" || strings.TrimSpace(view.HeadRefOID) == "" {
		return ResolvedTarget{}, fmt.Errorf("gh pr view returned an incomplete pull request target")
	}
	if err := verifyPRRepository(path, original, view.URL); err != nil {
		return ResolvedTarget{}, err
	}

	commit, err := ensurePRCommit(path, number, view.HeadRefOID)
	if err != nil {
		return ResolvedTarget{}, err
	}
	return ResolvedTarget{
		Kind:              "pullRequest",
		Commit:            commit,
		BranchName:        view.HeadRefName,
		CrossRepositoryPR: view.IsCrossRepository,
	}, nil
}

func verifyPRRepository(path, original, resolvedURL string) error {
	parsed, err := url.Parse(original)
	if err != nil || parsed.Scheme == "" {
		return nil
	}
	requestedRepo, ok := githubRepo(parsed)
	if !ok {
		return fmt.Errorf("invalid GitHub pull request URL %q", original)
	}
	resolved, err := url.Parse(resolvedURL)
	if err != nil {
		return fmt.Errorf("gh pr view returned an invalid URL %q", resolvedURL)
	}
	resolvedRepo, ok := githubRepo(resolved)
	if !ok || !strings.EqualFold(requestedRepo, resolvedRepo) {
		return fmt.Errorf("pull request URL resolved to a different repository")
	}

	cmd := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return commandError("gh repo view", out, err)
	}
	if !strings.EqualFold(strings.TrimSpace(string(out)), requestedRepo) {
		return fmt.Errorf("pull request belongs to %s, not the current repository", requestedRepo)
	}
	return nil
}

func githubRepo(parsed *url.URL) (string, bool) {
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") || len(parts) < 4 || parts[2] != "pull" {
		return "", false
	}
	return parts[0] + "/" + parts[1], parts[0] != "" && parts[1] != ""
}

func ensurePRCommit(path string, number int, expected string) (string, error) {
	if commit, err := gitutil.ResolveCommit(path, expected); err == nil && strings.EqualFold(commit, expected) {
		return commit, nil
	}
	privateRef := fmt.Sprintf("refs/worktree-bench/pull/%d/head", number)
	refspec := fmt.Sprintf("+refs/pull/%d/head:%s", number, privateRef)
	cmd := exec.Command("git", "-C", path, "fetch", "origin", refspec)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", commandError("git fetch origin "+refspec, out, err)
	}
	commit, err := gitutil.ResolveCommit(path, privateRef)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(commit, expected) {
		return "", fmt.Errorf("fetched pull request head %s does not match GitHub head %s", commit, expected)
	}
	return commit, nil
}

func listWorktrees(path string) ([]worktreeEntry, error) {
	cmd := exec.Command("git", "-C", path, "worktree", "list", "--porcelain", "-z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, commandError("git worktree list --porcelain -z", out, err)
	}
	var entries []worktreeEntry
	var current worktreeEntry
	flush := func() {
		if current.Path != "" {
			entries = append(entries, current)
			current = worktreeEntry{}
		}
	}
	for _, field := range strings.Split(string(out), "\x00") {
		if field == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(field, " ")
		switch key {
		case "worktree":
			if current.Path != "" {
				flush()
			}
			current.Path = filepath.Clean(value)
		case "HEAD":
			current.Commit = value
		case "branch":
			current.Branch = value
		}
	}
	flush()
	return entries, nil
}

func matchingWorktree(entries []worktreeEntry, target ResolvedTarget) string {
	branchRef := "refs/heads/" + target.BranchName
	for _, entry := range entries {
		if entry.Branch == branchRef && strings.EqualFold(entry.Commit, target.Commit) {
			return entry.Path
		}
	}
	var detached string
	for _, entry := range entries {
		if entry.Branch == "" && strings.EqualFold(entry.Commit, target.Commit) {
			if detached != "" {
				return ""
			}
			detached = entry.Path
		}
	}
	return detached
}

func parsePRNumber(ref string) (int, bool) {
	if prPattern.MatchString(ref) {
		matches := prPattern.FindStringSubmatch(ref)
		number, err := strconv.Atoi(matches[2])
		return number, err == nil && number > 0
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return 0, false
	}
	_, ok := githubRepo(parsed)
	if !ok {
		return 0, false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	number, err := strconv.Atoi(parts[3])
	return number, err == nil && number > 0
}

func ghCheckoutPR(path string, number int) error {
	cmd := exec.Command("gh", "pr", "checkout", fmt.Sprintf("%d", number))
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return commandError(fmt.Sprintf("gh pr checkout %d", number), out, err)
	}
	return nil
}

func commandError(command string, out []byte, err error) error {
	detail := strings.TrimSpace(string(out))
	if detail != "" {
		return fmt.Errorf("%s: %s", command, detail)
	}
	return fmt.Errorf("%s: %w", command, err)
}
