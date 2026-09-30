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
	cmd := &cobra.Command{
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
				invalidateStatusCacheEntries(repoRoot, flow.touchedIDs...)
				invalidateStatusCachePaths(repoRoot, flow.touchedPaths...)
			}
			if flow.message != "" {
				fmt.Fprintln(os.Stdout, flow.message)
			}
			if flow.targetPath != "" {
				return emitDirective(cmd, flow.targetPath)
			}
			return nil
		},
	}

	cmd.Flags().String("directive-file", "", "write cd directives to a file (for shell wrappers)")
	return cmd
}
