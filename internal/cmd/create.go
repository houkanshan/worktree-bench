package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
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

			result, err := ui.RunCreate(pool.Benches)
			if err != nil {
				return err
			}
			if result.Cancelled {
				return nil
			}

			input := bench.CreateInput{
				Type:        result.Type,
				UseExisting: result.UseExisting,
				BenchID:     result.BenchID,
				Name:        result.Name,
				Ref:         result.Ref,
			}

			pool, created, err := bench.CreateWorkbench(repoRoot, settings, pool, input)
			if err != nil {
				return err
			}
			if err := config.SavePool(repoRoot, pool); err != nil {
				return err
			}

			if created != nil {
				fmt.Fprintf(os.Stdout, "Created workbench %s at %s\n", created.Name, created.Path)
			}
			return nil
		},
	}
}
