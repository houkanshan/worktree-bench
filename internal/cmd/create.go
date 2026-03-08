package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
)

func newCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create or reuse a workbench",
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

			flow, err := runCreateFlow(repoRoot, settings, pool)
			if err != nil {
				return err
			}
			if flow.changed {
				if err := config.SavePool(repoRoot, flow.pool); err != nil {
					return err
				}
			}
			if flow.message != "" {
				fmt.Fprintln(os.Stdout, flow.message)
			}
			return nil
		},
	}
}
