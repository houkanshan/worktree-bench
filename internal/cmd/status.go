package cmd

import (
	"encoding/json"
	"os"

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
			if jsonOutput {
				return writeStatusJSON(repoRoot, settings, pool, fast)
			}

			return ui.RunStatus(pool.Benches)
		},
	}
	cmd.Flags().Bool("json", false, "print workbench status as JSON")
	cmd.Flags().Bool("fast", false, "reuse cached status for JSON output and skip expensive diff on cache misses")
	return cmd
}

type statusJSON struct {
	Types        []string          `json:"types"`
	WorktreesDir string            `json:"worktrees_dir"`
	Benches      []statusBenchJSON `json:"benches"`
}

type statusBenchJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Description string `json:"description"`
	DirMissing  bool   `json:"dir_missing"`
}

func writeStatusJSON(repoRoot string, settings config.Settings, pool config.Pool, fast bool) error {
	var statuses map[string]ui.BenchStatus
	if fast {
		statuses = ui.LoadBenchStatusesCached(repoRoot, pool.Benches, ui.StatusOptions{UseCache: true, Fast: true})
	} else {
		statuses = ui.LoadBenchStatuses(pool.Benches)
	}
	benches := make([]statusBenchJSON, 0, len(pool.Benches))
	for _, workbench := range pool.Benches {
		status := statuses[workbench.ID]
		benches = append(benches, statusBenchJSON{
			ID:          workbench.ID,
			Name:        workbench.Name,
			Type:        workbench.Type,
			Path:        workbench.Path,
			Description: ui.FormatStatusLine(status),
			DirMissing:  status.DirMissing,
		})
	}
	return json.NewEncoder(os.Stdout).Encode(statusJSON{
		Types:        config.WorkbenchTypes(),
		WorktreesDir: settings.WorktreesDir,
		Benches:      benches,
	})
}
