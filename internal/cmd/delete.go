package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
)

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a workbench",
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

			result, err := ui.RunDelete(pool.Benches)
			if err != nil {
				return err
			}
			if result.Cancelled {
				return nil
			}

			force, _ := cmd.Flags().GetBool("force")
			updated, message, err := runDeleteFlow(repoRoot, pool, result.BenchID, force)
			if err != nil {
				return err
			}
			if err := config.SavePool(repoRoot, updated); err != nil {
				return err
			}
			invalidateStatusCacheEntries(repoRoot, result.BenchID)
			if message != "" {
				fmt.Println(message)
			}
			return nil
		},
	}

	cmd.Flags().Bool("force", false, "force remove worktree directory")
	return cmd
}
