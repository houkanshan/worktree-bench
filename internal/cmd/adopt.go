package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
)

func newAdoptCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "adopt",
		Short: "Register the current worktree as a workbench",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := gitutil.RepoRoot()
			if err != nil {
				return err
			}
			settings, err := bench.EnsureSettings(repoRoot)
			if err != nil {
				return err
			}

			defaultName := filepath.Base(repoRoot)
			result, err := ui.RunAdopt(repoRoot, defaultName, settings.SetupCmd, settings.DevCmd, settings.InitCmd)
			if err != nil {
				return err
			}
			if result.Cancelled {
				return nil
			}

			input := bench.AdoptInput{
				Type:     result.Type,
				Name:     result.Name,
				RunSetup: result.RunSetup,
				RunDev:   result.RunDev,
				RunInit:  result.RunInit,
			}

			_, adopted, err := bench.AdoptWorkbench(repoRoot, settings, input)
			if err != nil {
				return err
			}
			if adopted != nil {
				invalidateStatusCacheEntries(repoRoot, adopted.ID)
				invalidateStatusCachePaths(repoRoot, adopted.Path)
			}

			if adopted != nil {
				fmt.Printf("Adopted workbench %s\n", adopted.Name)
			}
			return nil
		},
	}
}
