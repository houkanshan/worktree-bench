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
			pool, removed, err := bench.DeleteWorkbench(repoRoot, pool, bench.DeleteInput{BenchID: result.BenchID, Force: force})
			if err != nil {
				return err
			}
			if err := config.SavePool(repoRoot, pool); err != nil {
				return err
			}

			if removed != nil {
				fmt.Printf("Deleted workbench %s\n", removed.Name)
			}
			return nil
		},
	}

	cmd.Flags().Bool("force", false, "force remove worktree directory")
	return cmd
}
