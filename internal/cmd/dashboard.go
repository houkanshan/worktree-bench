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

type dashboardOptions struct {
	swapOnSelect bool
	swapOnCreate bool
}

func runDashboard(cmd *cobra.Command, args []string) error {
	return runDashboardWithOptions(cmd, dashboardOptions{})
}

func runSwapDashboard(cmd *cobra.Command, args []string) error {
	return runDashboardWithOptions(cmd, dashboardOptions{swapOnSelect: true, swapOnCreate: true})
}

func runDashboardWithOptions(cmd *cobra.Command, opts dashboardOptions) error {
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

	result, err := ui.RunDashboard(pool.Benches)
	if err != nil {
		return err
	}
	if result.Cancelled {
		return nil
	}

	switch result.Action {
	case ui.DashboardActionCreate:
		flow, err := runCreateFromDashboard(repoRoot, settings, pool, result.Type)
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
		if flow.targetPath != "" {
			targetPath := flow.targetPath
			if opts.swapOnCreate {
				if targetPath, err = bench.Switch(repoRoot, targetPath, true); err != nil {
					return err
				}
			}
			return emitDirective(cmd, targetPath)
		}
		return nil
	case ui.DashboardActionSwitch:
		selected := findBenchByID(pool, result.BenchID)
		if selected == nil {
			return fmt.Errorf("workbench not found")
		}
		if !opts.swapOnSelect {
			return emitDirective(cmd, selected.Path)
		}
		targetPath, err := bench.Switch(repoRoot, selected.Path, true)
		if err != nil {
			return err
		}
		return emitDirective(cmd, targetPath)
	case ui.DashboardActionDelete:
		updated, message, err := runDeleteFlow(repoRoot, pool, result.BenchID, false)
		if err != nil {
			return err
		}
		if err := config.SavePool(repoRoot, updated); err != nil {
			return err
		}
		if message != "" {
			fmt.Fprintln(os.Stdout, message)
		}
		return nil
	default:
		return nil
	}
}
