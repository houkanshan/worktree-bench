package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
)

func newSwitchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch to a workbench",
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

			result, err := ui.RunSwitch(pool.Benches)
			if err != nil {
				return err
			}
			if result.Cancelled {
				return nil
			}

			selected := findBenchByID(pool, result.BenchID)
			if selected == nil {
				return errors.New("workbench not found")
			}

			if !result.Swap {
				return emitDirective(cmd, selected.Path)
			}

			targetPath, err := bench.Switch(repoRoot, selected.Path, true)
			if err != nil {
				return err
			}
			invalidateStatusCacheForPaths(repoRoot, pool, repoRoot, selected.Path)

			return emitDirective(cmd, targetPath)
		},
	}

	cmd.Flags().String("directive-file", "", "write cd directives to a file (for shell wrappers)")
	return cmd
}

func appendDirective(path, target string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "cd '%s'\n", target)
	return err
}

func findBenchByID(pool config.Pool, id string) *config.Workbench {
	for i := range pool.Benches {
		if pool.Benches[i].ID == id {
			return &pool.Benches[i]
		}
	}
	return nil
}
