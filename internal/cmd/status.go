package cmd

import (
	"encoding/json"
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
			allowedRoots, _ := cmd.Flags().GetStringArray("allowed-root")
			if jsonOutput {
				return writeStatusJSON(repoRoot, settings, pool, fast, allowedRoots)
			}

			return ui.RunStatus(pool.Benches)
		},
	}
	cmd.Flags().Bool("json", false, "print workbench status as JSON")
	cmd.Flags().Bool("fast", false, "reuse cached status for JSON output and skip expensive diff on cache misses")
	cmd.Flags().StringArray("allowed-root", nil, "limit JSON status checks to paths under these roots")
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

func writeStatusJSON(repoRoot string, settings config.Settings, pool config.Pool, fast bool, allowedRoots []string) error {
	statusBenches, err := filterAllowedBenches(pool.Benches, allowedRoots)
	if err != nil {
		return err
	}
	var statuses map[string]ui.BenchStatus
	if fast {
		statuses = ui.LoadBenchStatusesCached(repoRoot, statusBenches, ui.StatusOptions{UseCache: true, Fast: true})
	} else {
		statuses = ui.LoadBenchStatuses(statusBenches)
	}
	var reuseStatuses map[string]gitutil.ReuseStatus
	if !fast {
		reuseStatuses = loadReuseStatuses(statusBenches)
	}
	benches := make([]statusBenchJSON, 0, len(pool.Benches))
	for _, workbench := range pool.Benches {
		benchJSON := statusBenchJSON{
			ID:   workbench.ID,
			Name: workbench.Name,
			Type: workbench.Type,
			Path: workbench.Path,
		}
		if status, ok := statuses[workbench.ID]; ok {
			benchJSON.Description = ui.FormatStatusLine(status)
			benchJSON.DirMissing = status.DirMissing
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

func filterAllowedBenches(benches []config.Workbench, allowedRoots []string) ([]config.Workbench, error) {
	authorized := make([]config.Workbench, 0, len(benches))
	for _, workbench := range benches {
		allowed, err := pathAllowed(workbench.Path, allowedRoots)
		if err != nil {
			return nil, err
		}
		if allowed {
			authorized = append(authorized, workbench)
		}
	}
	return authorized, nil
}

func loadReuseStatuses(authorized []config.Workbench) map[string]gitutil.ReuseStatus {
	const workers = 4
	result := make(map[string]gitutil.ReuseStatus, len(authorized))
	jobs := make(chan config.Workbench)
	var mutex sync.Mutex
	var group sync.WaitGroup
	workerCount := workers
	if len(authorized) < workerCount {
		workerCount = len(authorized)
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
	for _, workbench := range authorized {
		jobs <- workbench
	}
	close(jobs)
	group.Wait()
	return result
}
