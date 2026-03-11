package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
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
			pool, err := config.LoadPool(repoRoot, settings)
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

			pool, adopted, err := bench.AdoptWorkbench(repoRoot, settings, pool, input)
			if err != nil {
				return err
			}
			if err := config.SavePool(repoRoot, pool); err != nil {
				return err
			}

			if adopted != nil {
				fmt.Printf("Adopted workbench %s\n", adopted.Name)
			}
			return nil
		},
	}
}
