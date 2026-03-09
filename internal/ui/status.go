package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
)

type BenchStatus struct {
	DirMissing   bool
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

func LoadBenchStatuses(benches []config.Workbench) map[string]BenchStatus {
	statuses := make(map[string]BenchStatus, len(benches))
	for _, bench := range benches {
		status := BenchStatus{}

		// Check if the worktree directory still exists on disk.
		if _, err := os.Stat(bench.Path); os.IsNotExist(err) {
			status.DirMissing = true
			statuses[bench.ID] = status
			continue
		}

		branch, err := gitutil.Branch(bench.Path)
		if err == nil {
			status.Branch = branch
		}

		changes, err := gitutil.DiffLines(bench.Path)
		status.ChangesLines = changes
		status.ChangesErr = err

		if t, err := gitutil.LastCommitTime(bench.Path); err == nil {
			status.LastCommit = t
		}

		// GitHub PR lookup commented out — too slow.
		// if pr, err := ghPRView(bench.Path); err == nil {
		// 	status.PRNumber = pr.Number
		// 	status.PRTitle = pr.Title
		// 	status.PRURL = pr.URL
		// }

		statuses[bench.ID] = status
	}
	return statuses
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
