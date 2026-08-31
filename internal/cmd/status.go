package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "View workbench status",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := gitutil.RepoRoot()
			if err != nil {
				return err
			}
			settings, err := bench.EnsureSettings(repoRoot)
			if err != nil {
				return err
			}
			pool, err := config.LoadPool(repoRoot, settings)
			if err != nil {
				return err
			}

			jsonOutput, _ := cmd.Flags().GetBool("json")
			fast, _ := cmd.Flags().GetBool("fast")
			selection, _ := cmd.Flags().GetBool("selection")
			if selection && !jsonOutput {
				return fmt.Errorf("--selection requires --json")
			}
			if jsonOutput {
				return writeStatusJSON(repoRoot, settings, pool, fast, selection)
			}

			return ui.RunStatus(pool.Benches)
		},
	}
	cmd.Flags().Bool("json", false, "print workbench status as JSON")
	cmd.Flags().Bool("fast", false, "reuse cached status for JSON output and skip expensive diff on cache misses")
	cmd.Flags().Bool("selection", false, "include fresh reusable-safety status in JSON output")
	return cmd
}

type statusJSON struct {
	Types        []string          `json:"types"`
	WorktreesDir string            `json:"worktrees_dir"`
	Benches      []statusBenchJSON `json:"benches"`
}

type statusBenchJSON struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Path        string               `json:"path"`
	Description string               `json:"description"`
	DirMissing  bool                 `json:"dir_missing"`
	Git         *gitutil.ReuseStatus `json:"git,omitempty"`
}

func writeStatusJSON(repoRoot string, settings config.Settings, pool config.Pool, fast, selection bool) error {
	var statuses map[string]ui.BenchStatus
	if fast || selection {
		statuses = ui.LoadBenchStatusesCached(repoRoot, pool.Benches, ui.StatusOptions{UseCache: true, Fast: true})
	} else {
		statuses = ui.LoadBenchStatuses(pool.Benches)
	}
	var reuseStatuses map[string]gitutil.ReuseStatus
	if selection {
		reuseStatuses = loadReuseStatuses(pool.Benches)
	}
	benches := make([]statusBenchJSON, 0, len(pool.Benches))
	for _, workbench := range pool.Benches {
		status := statuses[workbench.ID]
		benchJSON := statusBenchJSON{
			ID:          workbench.ID,
			Name:        workbench.Name,
			Type:        workbench.Type,
			Path:        workbench.Path,
			Description: ui.FormatStatusLine(status),
			DirMissing:  status.DirMissing,
		}
		if reuse, ok := reuseStatuses[workbench.ID]; ok {
			benchJSON.Git = &reuse
		}
		benches = append(benches, benchJSON)
	}
	return json.NewEncoder(os.Stdout).Encode(statusJSON{
		Types:        config.WorkbenchTypes(),
		WorktreesDir: settings.WorktreesDir,
		Benches:      benches,
	})
}

func loadReuseStatuses(benches []config.Workbench) map[string]gitutil.ReuseStatus {
	const workers = 4
	result := make(map[string]gitutil.ReuseStatus, len(benches))
	jobs := make(chan config.Workbench)
	var mutex sync.Mutex
	var group sync.WaitGroup
	workerCount := workers
	if len(benches) < workerCount {
		workerCount = len(benches)
	}
	group.Add(workerCount)
	for range workerCount {
		go func() {
			defer group.Done()
			for workbench := range jobs {
				status := gitutil.InspectReuseStatus(workbench.Path)
				mutex.Lock()
				result[workbench.ID] = status
				mutex.Unlock()
			}
		}()
	}
	for _, workbench := range benches {
		jobs <- workbench
	}
	close(jobs)
	group.Wait()
	return result
}
