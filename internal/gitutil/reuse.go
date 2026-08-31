package gitutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReuseStatus is the fresh, machine-readable safety status used when selecting
// a workbench for reuse. Safe means that discarding the branch cannot lose
// local work: HEAD is on the default branch or its PR has been merged.
type ReuseStatus struct {
	State    string         `json:"state"`
	Severity string         `json:"severity"`
	Label    string         `json:"label"`
	Branch   string         `json:"branch,omitempty"`
	Dirty    *bool          `json:"dirty"`
	Unpushed *int           `json:"unpushed"`
	PR       *ReusePRStatus `json:"pr,omitempty"`
}

type ReusePRStatus struct {
	Number any    `json:"number"`
	State  string `json:"state"`
}

type branchPRStatus struct {
	Number any    `json:"number"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

func boolPtr(value bool) *bool { return &value }
func intPtr(value int) *int    { return &value }

func reuseStatus(state, severity, label, branch string, dirty *bool, unpushed *int, pr *ReusePRStatus) ReuseStatus {
	return ReuseStatus{State: state, Severity: severity, Label: label, Branch: branch, Dirty: dirty, Unpushed: unpushed, PR: pr}
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
		out, err := exec.CommandContext(ctx, "git", commandArgs...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
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

	defaultRefs := []string{"refs/heads/main", "refs/heads/master", "refs/remotes/origin/main", "refs/remotes/origin/master"}
	if remoteDefault, err := runGit("symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil && remoteDefault != "" {
		defaultRefs = []string{remoteDefault}
	}
	containsArgs := append([]string{"for-each-ref", "--contains=HEAD", "--format=%(refname)"}, defaultRefs...)
	if containing, err := runGit(containsArgs...); err == nil && containing != "" {
		return reuseStatus("no-change", "safe", "no change", branch, boolPtr(false), intPtr(0), nil)
	}

	var unpushed *int
	if output, err := runGit("rev-list", "--count", "@{upstream}..HEAD"); err == nil {
		if count, parseErr := strconv.Atoi(output); parseErr == nil {
			unpushed = intPtr(count)
			if count > 0 {
				return reuseStatus("unpushed", "warning", fmt.Sprintf("%d unpushed", count), branch, boolPtr(false), unpushed, nil)
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(home, "tools", "branch-pr-status"), branch)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
	var payload branchPRStatus
	if err := json.Unmarshal(out, &payload); err != nil || payload.Error != "" {
		return reuseStatus("unknown", "muted", "unknown", branch, boolPtr(false), unpushed, nil)
	}
	state := strings.ToLower(payload.State)
	if state == "" || state == "none" || payload.Number == nil {
		return reuseStatus("no-pr", "muted", "no PR", branch, boolPtr(false), unpushed, nil)
	}
	pr := &ReusePRStatus{Number: payload.Number, State: state}
	label := fmt.Sprintf("#%v %s", payload.Number, state)
	switch state {
	case "merged":
		return reuseStatus("pr-merged", "safe", label, branch, boolPtr(false), unpushed, pr)
	case "closed":
		return reuseStatus("pr-closed", "warning", label, branch, boolPtr(false), unpushed, pr)
	default:
		return reuseStatus("pr-open", "warning", fmt.Sprintf("#%v open", payload.Number), branch, boolPtr(false), unpushed, pr)
	}
}
