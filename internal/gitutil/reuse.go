package gitutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ReuseStatus is the fresh, machine-readable safety status used when selecting
// a workbench for reuse. Safe means that switching away cannot lose local work:
// HEAD is on the default branch, matches a pushed open/closed PR head, or is
// contained in a merged PR head (proved using local Git objects only).
type ReuseStatus struct {
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Label    string         `json:"label"`
	Branch   string         `json:"branch,omitempty"`
	Dirty    *bool          `json:"dirty"`
	Unpushed *int           `json:"unpushed"`
	PR       *ReusePRStatus `json:"pr,omitempty"`
	Activity ReuseActivity  `json:"activity"`
}

type ReuseActivity struct {
	CommitAt string `json:"commitAt,omitempty"`
	PRAt     string `json:"prAt,omitempty"`
}

type ReusePRStatus struct {
	Number any    `json:"number"`
	State  string `json:"state"`
}

type branchPRStatus struct {
	Number     any    `json:"number"`
	State      string `json:"state"`
	Error      string `json:"error"`
	HeadSHA    string `json:"headSha"`
	HeadRefOID string `json:"headRefOid"`
	UpdatedAt  string `json:"updatedAt"`
	MergedAt   string `json:"mergedAt"`
	ClosedAt   string `json:"closedAt"`
	Stale      bool   `json:"stale"`
}

func boolPtr(value bool) *bool { return &value }
func intPtr(value int) *int    { return &value }

func reuseStatus(state, severity, label, branch string, dirty *bool, unpushed *int, pr *ReusePRStatus) ReuseStatus {
	return ReuseStatus{Kind: state, Severity: severity, Label: label, Branch: branch, Dirty: dirty, Unpushed: unpushed, PR: pr}
}

func latestTimestamp(values ...string) string {
	var latest time.Time
	var result string
	for _, value := range values {
		parsed, err := time.Parse(time.RFC3339, value)
		if err == nil && parsed.After(latest) {
			latest = parsed
			result = value
		}
	}
	return result
}

// InspectReuseStatus performs fresh checks. It deliberately does not use the
// display-status cache because stale safety data must never authorize reuse.
func InspectReuseStatus(path string) ReuseStatus {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return reuseStatus("missing", "muted", "missing", "", nil, nil, nil)
	}

	runGit := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		commandArgs := append([]string{"-C", path}, args...)
		cmd := exec.CommandContext(ctx, "git", commandArgs...)
		// Safety inspection must not fetch missing objects from promisor remotes.
		cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail != "" {
				return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
			}
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	status, err := runGit("status", "--porcelain")
	if err != nil {
		return reuseStatus("unknown", "muted", "unknown", "", nil, nil, nil)
	}
	if status != "" {
		return reuseStatus("uncommitted", "warning", "uncommitted", "", boolPtr(true), nil, nil)
	}

	branch, err := runGit("branch", "--show-current")
	if err != nil || branch == "" {
		return reuseStatus("unknown", "muted", "unknown", "", boolPtr(false), nil, nil)
	}
	head, err := runGit("rev-parse", "HEAD")
	if err != nil {
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), nil, nil)
	}
	commitAt, _ := runGit("show", "-s", "--format=%cI", head)
	confirmSafe := func(candidate ReuseStatus) ReuseStatus {
		finalHead, headErr := runGit("rev-parse", "HEAD")
		finalStatus, statusErr := runGit("status", "--porcelain")
		if headErr != nil || statusErr != nil || finalHead != head || finalStatus != "" {
			return reuseStatus("unknown", "muted", "unknown", branch, nil, nil, nil)
		}
		candidate.Activity.CommitAt = commitAt
		return candidate
	}

	var unpushed *int
	upstreamKnown := false
	if output, err := runGit("rev-list", "--count", "@{upstream}..HEAD"); err == nil {
		if count, parseErr := strconv.Atoi(output); parseErr == nil {
			unpushed = intPtr(count)
			upstreamKnown = true
			if count > 0 {
				return reuseStatus("unpushed", "warning", fmt.Sprintf("%d unpushed", count), branch, boolPtr(false), unpushed, nil)
			}
		}
	}

	defaultSafe := false
	defaultRefs := []string{"refs/heads/main", "refs/heads/master", "refs/remotes/origin/main", "refs/remotes/origin/master"}
	if remoteDefault, err := runGit("symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil && remoteDefault != "" {
		defaultRefs = []string{remoteDefault}
	}
	containsArgs := append([]string{"for-each-ref", "--contains=HEAD", "--format=%(refname)"}, defaultRefs...)
	if containing, err := runGit(containsArgs...); err == nil && containing != "" {
		remotePreserved := false
		for _, ref := range strings.Split(containing, "\n") {
			if strings.HasPrefix(strings.TrimSpace(ref), "refs/remotes/") {
				remotePreserved = true
				break
			}
		}
		defaultSafe = upstreamKnown || remotePreserved
	}
	defaultStatus := func(prAt string) ReuseStatus {
		candidate := reuseStatus("no-change", "safe", "no change", branch, boolPtr(false), unpushed, nil)
		candidate.Activity.PRAt = prAt
		return confirmSafe(candidate)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		if defaultSafe {
			return defaultStatus("")
		}
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
	out, err := runBranchPRStatus(home, path, branch)
	if err != nil {
		if defaultSafe {
			return defaultStatus("")
		}
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
	var payload branchPRStatus
	payloadValid := json.Unmarshal(out, &payload) == nil && payload.Error == "" && !payload.Stale && payload.HeadSHA == head
	if defaultSafe {
		prAt := ""
		if payloadValid {
			prAt = latestTimestamp(payload.UpdatedAt, payload.MergedAt, payload.ClosedAt)
		}
		return defaultStatus(prAt)
	}
	if !payloadValid {
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
	state := strings.ToLower(payload.State)
	if state == "" || state == "none" || payload.Number == nil {
		return reuseStatus("no-pr", "muted", "no PR", branch, boolPtr(false), unpushed, nil)
	}
	prAt := latestTimestamp(payload.UpdatedAt, payload.MergedAt, payload.ClosedAt)
	pr := &ReusePRStatus{Number: payload.Number, State: state}
	label := fmt.Sprintf("#%v %s", payload.Number, state)
	switch state {
	case "merged":
		if payload.HeadRefOID != head {
			// Automation may append commits before merging. Preserve the exact-head
			// fast path; otherwise require local proof that all local history is
			// in the merged PR. Missing objects or any Git failure stay unknown.
			if _, err := runGit("merge-base", "--is-ancestor", head, payload.HeadRefOID); err != nil {
				return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
			}
		}
		candidate := reuseStatus("pr-merged", "safe", label, branch, boolPtr(false), unpushed, pr)
		candidate.Activity.PRAt = prAt
		return confirmSafe(candidate)
	case "closed":
		if payload.HeadRefOID != head {
			return reuseStatus("pr-closed", "warning", fmt.Sprintf("#%v closed · remote changed", payload.Number), branch, boolPtr(false), unpushed, pr)
		}
		candidate := reuseStatus("pr-closed", "safe", label, branch, boolPtr(false), unpushed, pr)
		candidate.Activity.PRAt = prAt
		return confirmSafe(candidate)
	case "open":
		if payload.HeadRefOID != head {
			return reuseStatus("pr-open", "warning", fmt.Sprintf("#%v open · remote changed", payload.Number), branch, boolPtr(false), unpushed, pr)
		}
		candidate := reuseStatus("pr-open", "safe", fmt.Sprintf("#%v open", payload.Number), branch, boolPtr(false), unpushed, pr)
		candidate.Activity.PRAt = prAt
		return confirmSafe(candidate)
	default:
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
}

func runBranchPRStatus(home, worktree, branch string) ([]byte, error) {
	cacheRoot := os.Getenv("XDG_CACHE_HOME")
	if cacheRoot == "" {
		cacheRoot = filepath.Join(home, ".cache")
	}
	lockDir := filepath.Join(cacheRoot, "branch-pr-status")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create branch-pr-status lock directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(lockDir, "wtb.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open branch-pr-status lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return nil, fmt.Errorf("lock branch-pr-status cache: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck

	// Start the helper timeout only after this process owns the cross-process lock.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(home, "tools", "branch-pr-status"), branch, "--wait")
	cmd.Dir = worktree
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("branch-pr-status: %s", detail)
		}
		return nil, fmt.Errorf("branch-pr-status: %w", err)
	}
	return out, nil
}
