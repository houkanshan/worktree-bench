package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/statuscache"
)

type BenchStatus struct {
	DirMissing   bool
	Description  string
	Branch       string
	LastCommit   time.Time
	ChangesLines int
	ChangesErr   error

	// GitHub PR fields — commented out because gh calls are too slow.
	// PRNumber int
	// PRTitle  string
	// PRURL    string
}

// type prView struct {
// 	Number int    `json:"number"`
// 	Title  string `json:"title"`
// 	URL    string `json:"url"`
// }

type StatusOptions struct {
	UseCache bool
	Fast     bool
}

func LoadBenchStatuses(benches []config.Workbench) map[string]BenchStatus {
	return loadBenchStatuses(benches, StatusOptions{})
}

func LoadBenchStatusesCached(repoRoot string, benches []config.Workbench, opts StatusOptions) map[string]BenchStatus {
	if !opts.UseCache {
		return loadBenchStatuses(benches, opts)
	}
	start := time.Now()
	cache, _ := statuscache.Load(repoRoot)
	statuses := make(map[string]BenchStatus, len(benches))
	updated := false
	hits := 0
	misses := 0
	for _, bench := range benches {
		fingerprint, status := benchFingerprint(bench)
		if cached, ok := statuscache.Get(cache, bench, fingerprint); ok {
			statuses[bench.ID] = fromCachedStatus(cached)
			hits++
			continue
		}
		misses++
		if fingerprint == "missing" {
			statuscache.Put(&cache, bench, fingerprint, toCachedStatus(status))
			statuses[bench.ID] = status
			updated = true
			continue
		}
		slowStart := time.Now()
		status = loadBenchStatus(bench, opts)
		tracef("status bench=%s slow_path=%s", bench.ID, time.Since(slowStart))
		statuscache.Put(&cache, bench, fingerprint, toCachedStatus(status))
		statuses[bench.ID] = status
		updated = true
	}
	if updated {
		_ = statuscache.Save(repoRoot, cache)
	}
	tracef("status total=%s hits=%d misses=%d updated=%t", time.Since(start), hits, misses, updated)
	return statuses
}

func loadBenchStatuses(benches []config.Workbench, opts StatusOptions) map[string]BenchStatus {
	statuses := make(map[string]BenchStatus, len(benches))
	for _, bench := range benches {
		statuses[bench.ID] = loadBenchStatus(bench, opts)
	}
	return statuses
}

func loadBenchStatus(bench config.Workbench, opts StatusOptions) BenchStatus {
	status := BenchStatus{}
	if _, err := os.Stat(bench.Path); os.IsNotExist(err) {
		status.DirMissing = true
		return status
	}
	branch, err := gitutil.BranchOrShortHEAD(bench.Path)
	if err == nil {
		status.Branch = branch
		if branch != "" && branch != "HEAD" {
			if desc, err := gitutil.BranchDescription(bench.Path, branch); err == nil {
				status.Description = desc
			}
		}
	}
	if t, subject, err := gitutil.LastCommitInfo(bench.Path); err == nil {
		status.LastCommit = t
		if status.Description == "" {
			status.Description = subject
		}
	}
	if !opts.Fast {
		changes, err := gitutil.DiffLines(bench.Path)
		status.ChangesLines = changes
		status.ChangesErr = err
	}
	return status
}

func benchFingerprint(bench config.Workbench) (string, BenchStatus) {
	status := BenchStatus{}
	if _, err := os.Stat(bench.Path); os.IsNotExist(err) {
		status.DirMissing = true
		return "missing", status
	}
	fingerprint, err := gitutil.WorktreeFingerprint(bench.Path)
	if err != nil {
		return "error:" + err.Error(), status
	}
	return fingerprint + ":" + bench.ID + ":" + bench.Type + ":" + bench.Name, status
}

func toCachedStatus(status BenchStatus) statuscache.BenchStatus {
	cached := statuscache.BenchStatus{DirMissing: status.DirMissing, Description: status.Description, Branch: status.Branch, LastCommit: status.LastCommit, ChangesLines: status.ChangesLines}
	if status.ChangesErr != nil {
		cached.ChangesErr = status.ChangesErr.Error()
	}
	return cached
}

func fromCachedStatus(status statuscache.BenchStatus) BenchStatus {
	cached := BenchStatus{DirMissing: status.DirMissing, Description: status.Description, Branch: status.Branch, LastCommit: status.LastCommit, ChangesLines: status.ChangesLines}
	if status.ChangesErr != "" {
		cached.ChangesErr = fmt.Errorf("%s", status.ChangesErr)
	}
	return cached
}

func tracef(format string, args ...any) {
	if os.Getenv("WTB_TRACE") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// ghPRView commented out — too slow.
// func ghPRView(path string) (prView, error) {
// 	cmd := exec.Command("gh", "pr", "view", "--json", "number,title,url")
// 	cmd.Dir = path
// 	out, err := cmd.Output()
// 	if err != nil {
// 		return prView{}, err
// 	}
// 	var payload prView
// 	if err := json.Unmarshal(out, &payload); err != nil {
// 		return prView{}, err
// 	}
// 	return payload, nil
// }

// relativeTime formats a duration since t as a compact human string.
// Examples: "1m", "2h3m", "1d", "1w", "3w2d".
func relativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	case d < 7*24*time.Hour:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd", days)
	default:
		weeks := int(d.Hours()) / (7 * 24)
		days := (int(d.Hours()) / 24) % 7
		if days == 0 {
			return fmt.Sprintf("%dw", weeks)
		}
		return fmt.Sprintf("%dw%dd", weeks, days)
	}
}

func FormatStatusLine(status BenchStatus) string {
	if status.DirMissing {
		return "⚠ directory missing"
	}
	parts := []string{}
	if status.Description != "" {
		parts = append(parts, status.Description)
	}
	if status.Branch != "" {
		parts = append(parts, fmt.Sprintf("branch: %s", status.Branch))
	}
	if !status.LastCommit.IsZero() {
		parts = append(parts, fmt.Sprintf("commit: %s ago", relativeTime(status.LastCommit)))
	}
	// PR line commented out — gh calls removed.
	// if status.PRNumber != 0 {
	// 	parts = append(parts, fmt.Sprintf("PR #%d: %s", status.PRNumber, status.PRTitle))
	// }
	if status.ChangesErr == nil && status.ChangesLines > 0 {
		parts = append(parts, fmt.Sprintf("Δ %d lines", status.ChangesLines))
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, " | ")
}
