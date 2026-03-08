package ui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
)

type BenchStatus struct {
	Branch       string
	PRNumber     int
	PRTitle      string
	PRURL        string
	ChangesLines int
	ChangesErr   error
}

type prView struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

func LoadBenchStatuses(benches []config.Workbench) map[string]BenchStatus {
	statuses := make(map[string]BenchStatus, len(benches))
	for _, bench := range benches {
		status := BenchStatus{}
		branch, err := gitutil.Branch(bench.Path)
		if err == nil {
			status.Branch = branch
		}

		changes, err := gitutil.DiffLines(bench.Path)
		status.ChangesLines = changes
		status.ChangesErr = err

		if pr, err := ghPRView(bench.Path); err == nil {
			status.PRNumber = pr.Number
			status.PRTitle = pr.Title
			status.PRURL = pr.URL
		}

		statuses[bench.ID] = status
	}
	return statuses
}

func ghPRView(path string) (prView, error) {
	cmd := exec.Command("gh", "pr", "view", "--json", "number,title,url")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return prView{}, err
	}
	var payload prView
	if err := json.Unmarshal(out, &payload); err != nil {
		return prView{}, err
	}
	return payload, nil
}

func FormatStatusLine(status BenchStatus) string {
	parts := []string{}
	if status.Branch != "" {
		parts = append(parts, fmt.Sprintf("branch: %s", status.Branch))
	}
	if status.PRNumber != 0 {
		parts = append(parts, fmt.Sprintf("PR #%d: %s", status.PRNumber, status.PRTitle))
	}
	if status.ChangesErr == nil && status.ChangesLines > 0 {
		parts = append(parts, fmt.Sprintf("Δ %d lines", status.ChangesLines))
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, " | ")
}
