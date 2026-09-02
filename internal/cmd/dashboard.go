package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
)

func tracef(format string, args ...any) {
	if os.Getenv("WTB_TRACE") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[wtb] "+format+"\n", args...)
}

type dashboardOptions struct {
	swapOnSelect bool
	swapOnCreate bool
}

type directSelection struct {
	BenchID         string
	CreateNew       bool
	Type            string
	Name            string
	Base            string
	JSON            bool
	RequireReusable bool
	CheckoutTarget  string
	AllowedRoots    []string
}

type selectionJSON struct {
	BenchID string `json:"benchId"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

func addNoTUISelectionFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("new", false, "create a new workbench without opening the selector")
	cmd.Flags().Bool("json", false, "print the selected workbench as JSON (requires --new or a workbench id)")
	cmd.Flags().String("type", config.TypeLarge, "workbench type for --new (large, medium, small)")
	cmd.Flags().String("name", "", "workbench name for --new (defaults to the next pool name)")
	cmd.Flags().String("base", baseBranchMaster, "base branch for --new (master, current, or a branch name)")
	cmd.Flags().String("init-cmd", "", "override init_cmd for this no-TUI action (pass '-' or an empty value to disable)")
	cmd.Flags().String("checkout", "", "checkout a PR number, GitHub PR URL, or branch instead of running init_cmd")
	cmd.Flags().String("directive-file", "", "write cd directives to a file (for shell wrappers)")
	cmd.Flags().Bool("require-reusable", false, "fail unless the selected existing workbench is freshly classified safe")
	cmd.Flags().StringArray("allowed-root", nil, "limit direct selection to paths under these roots")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	return runDashboardWithOptions(cmd, args, dashboardOptions{})
}

func runSwapDashboard(cmd *cobra.Command, args []string) error {
	return runDashboardWithOptions(cmd, args, dashboardOptions{swapOnSelect: true, swapOnCreate: true})
}

func runDashboardWithOptions(cmd *cobra.Command, args []string, opts dashboardOptions) error {
	repoRoot, err := gitutil.RepoRoot()
	if err != nil {
		return err
	}
	settings, err := bench.EnsureSettings(repoRoot)
	if err != nil {
		return err
	}
	if value, changed := stringFlag(cmd, "init-cmd"); changed {
		settings.InitCmd = normalizeInitCmd(value)
	}
	pool, err := config.LoadPool(repoRoot, settings)
	if err != nil {
		return err
	}

	direct, directMode, err := parseDirectSelection(cmd, args)
	if err != nil {
		return err
	}
	if directMode {
		return runDirectSelection(cmd, repoRoot, settings, pool, opts, direct)
	}
	result, err := ui.RunDashboard(pool.Benches)
	if err != nil {
		return err
	}
	if result.Cancelled {
		return nil
	}

	tracef("dashboard: action=%d benchID=%s type=%s", result.Action, result.BenchID, result.Type)
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
			invalidateStatusCacheEntries(repoRoot, flow.touchedIDs...)
			invalidateStatusCachePaths(repoRoot, flow.touchedPaths...)
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
				invalidateStatusCacheForPaths(repoRoot, flow.pool, repoRoot, flow.targetPath)
			}
			return emitDirective(cmd, targetPath)
		}
		return nil
	case ui.DashboardActionSwitch:
		selected := findBenchByID(pool, result.BenchID)
		if selected == nil {
			return fmt.Errorf("workbench not found")
		}
		tracef("dashboard-switch: bench=%s path=%s initCmd=%q swapOnSelect=%t", selected.Name, selected.Path, settings.InitCmd, opts.swapOnSelect)
		if settings.InitCmd != "" {
			tracef("dashboard-switch: running init_cmd %q in %s", settings.InitCmd, selected.Path)
			if err := bench.RunInitCmd(selected.Path, settings.InitCmd); err != nil {
				return err
			}
		}
		if !opts.swapOnSelect {
			return emitDirective(cmd, selected.Path)
		}
		targetPath, err := bench.Switch(repoRoot, selected.Path, true)
		if err != nil {
			return err
		}
		invalidateStatusCacheForPaths(repoRoot, pool, repoRoot, selected.Path)
		return emitDirective(cmd, targetPath)
	case ui.DashboardActionDelete:
		updated, message, err := runDeleteFlow(repoRoot, pool, result.BenchID, false)
		if err != nil {
			return err
		}
		if err := config.SavePool(repoRoot, updated); err != nil {
			return err
		}
		invalidateStatusCacheEntries(repoRoot, result.BenchID)
		if message != "" {
			fmt.Fprintln(os.Stdout, message)
		}
		return nil
	default:
		return nil
	}
}

func normalizeInitCmd(value string) string {
	if strings.TrimSpace(value) == "-" {
		return ""
	}
	return value
}

func parseDirectSelection(cmd *cobra.Command, args []string) (directSelection, bool, error) {
	createNew, _ := boolFlag(cmd, "new")
	jsonOutput, _ := boolFlag(cmd, "json")
	requireReusable, _ := boolFlag(cmd, "require-reusable")
	checkoutTarget, checkoutChanged := stringFlag(cmd, "checkout")
	checkoutTarget = strings.TrimSpace(checkoutTarget)
	allowedRoots, _ := cmd.Flags().GetStringArray("allowed-root")
	if createNew && len(args) > 0 {
		return directSelection{}, false, fmt.Errorf("workbench id cannot be combined with --new")
	}
	if len(args) > 1 {
		return directSelection{}, false, fmt.Errorf("expected at most one workbench id")
	}
	if !createNew && len(args) == 0 {
		if jsonOutput || requireReusable || checkoutChanged || len(allowedRoots) > 0 {
			return directSelection{}, false, fmt.Errorf("--json, --require-reusable, --checkout, and --allowed-root require --new or a workbench id")
		}
		return directSelection{}, false, nil
	}

	benchType, _ := stringFlag(cmd, "type")
	if benchType == "" {
		benchType = config.TypeLarge
	}
	benchType = config.NormalizeWorkbenchType(benchType)
	if createNew && !config.IsWorkbenchType(benchType) {
		return directSelection{}, false, fmt.Errorf("unsupported workbench type %q", benchType)
	}

	name, _ := stringFlag(cmd, "name")
	base, _ := stringFlag(cmd, "base")
	if base == "" {
		base = baseBranchMaster
	}

	if createNew && requireReusable {
		return directSelection{}, false, fmt.Errorf("--require-reusable applies only to an existing workbench")
	}
	_, initChanged := stringFlag(cmd, "init-cmd")
	if checkoutChanged && checkoutTarget == "" {
		return directSelection{}, false, fmt.Errorf("--checkout requires a PR number or branch")
	}
	if checkoutTarget != "" && initChanged {
		return directSelection{}, false, fmt.Errorf("--checkout cannot be combined with --init-cmd")
	}
	direct := directSelection{
		CreateNew:       createNew,
		Type:            benchType,
		Name:            name,
		Base:            base,
		JSON:            jsonOutput,
		RequireReusable: requireReusable,
		CheckoutTarget:  checkoutTarget,
		AllowedRoots:    allowedRoots,
	}
	if len(args) == 1 {
		direct.BenchID = strings.TrimSpace(args[0])
		if direct.BenchID == "" {
			return directSelection{}, false, fmt.Errorf("missing workbench id")
		}
	}
	return direct, true, nil
}

func runDirectSelection(cmd *cobra.Command, repoRoot string, settings config.Settings, pool config.Pool, opts dashboardOptions, direct directSelection) error {
	if direct.CreateNew {
		if err := requireAllowedPath(settings.WorktreesDir, direct.AllowedRoots); err != nil {
			return err
		}
		baseBranch, err := resolveBaseBranch(repoRoot, direct.Base)
		if err != nil {
			return err
		}
		input := bench.CreateInput{
			Type:           direct.Type,
			Name:           direct.Name,
			BaseBranch:     baseBranch,
			RunInit:        direct.CheckoutTarget == "" && strings.TrimSpace(settings.InitCmd) != "",
			CheckoutTarget: direct.CheckoutTarget,
			QuietOutput:    direct.JSON,
			ValidatePath:   func(candidate string) error { return requireAllowedPath(candidate, direct.AllowedRoots) },
		}
		updated, created, err := bench.CreateWorkbench(repoRoot, settings, pool, input)
		if err != nil {
			return err
		}
		if created == nil {
			return nil
		}
		if err := config.SavePool(repoRoot, updated); err != nil {
			return err
		}
		invalidateStatusCacheEntries(repoRoot, created.ID)
		invalidateStatusCachePaths(repoRoot, created.Path)
		if !direct.JSON {
			fmt.Fprintf(os.Stdout, "Created workbench %s at %s\n", created.Name, created.Path)
		}

		targetPath := created.Path
		if opts.swapOnCreate {
			if targetPath, err = bench.Switch(repoRoot, targetPath, true); err != nil {
				return err
			}
			invalidateStatusCacheForPaths(repoRoot, updated, repoRoot, created.Path)
		}
		return emitSelection(cmd, *created, targetPath, true, direct.JSON)
	}

	selected := findBenchByID(pool, direct.BenchID)
	if selected == nil {
		return fmt.Errorf("workbench %q not found", direct.BenchID)
	}
	if err := requireAllowedPath(selected.Path, direct.AllowedRoots); err != nil {
		return err
	}
	if direct.RequireReusable {
		status := gitutil.InspectReuseStatus(selected.Path)
		if status.Severity != "safe" {
			return fmt.Errorf("workbench %q is not reusable: %s", selected.Name, status.Label)
		}
	}
	tracef("direct-switch: bench=%s path=%s initCmd=%q checkout=%q swapOnSelect=%t", selected.Name, selected.Path, settings.InitCmd, direct.CheckoutTarget, opts.swapOnSelect)
	targetPath := selected.Path
	if direct.CheckoutTarget != "" {
		checkout, err := bench.CheckoutTarget(selected.Path, direct.CheckoutTarget)
		if err != nil {
			return err
		}
		if err := requireAllowedPath(checkout.Path, direct.AllowedRoots); err != nil {
			return err
		}
		targetPath = checkout.Path
	} else if strings.TrimSpace(settings.InitCmd) != "" {
		tracef("direct-switch: running init_cmd %q in %s", settings.InitCmd, selected.Path)
		if err := bench.RunInitCmdWithOutput(selected.Path, settings.InitCmd, !direct.JSON); err != nil {
			return err
		}
	}
	if !opts.swapOnSelect {
		return emitSelection(cmd, *selected, targetPath, false, direct.JSON)
	}
	if targetPath != selected.Path {
		return fmt.Errorf("cannot swap to a checkout target owned by another worktree")
	}
	targetPath, err := bench.Switch(repoRoot, selected.Path, true)
	if err != nil {
		return err
	}
	invalidateStatusCacheForPaths(repoRoot, pool, repoRoot, selected.Path)
	return emitSelection(cmd, *selected, targetPath, false, direct.JSON)
}

func emitSelection(cmd *cobra.Command, selected config.Workbench, targetPath string, created bool, jsonOutput bool) error {
	if !jsonOutput {
		return emitDirective(cmd, targetPath)
	}
	return json.NewEncoder(os.Stdout).Encode(selectionJSON{
		BenchID: selected.ID,
		Name:    selected.Name,
		Type:    selected.Type,
		Path:    targetPath,
		Created: created,
	})
}

func boolFlag(cmd *cobra.Command, name string) (bool, bool) {
	if cmd == nil || cmd.Flags().Lookup(name) == nil {
		return false, false
	}
	value, _ := cmd.Flags().GetBool(name)
	return value, cmd.Flags().Lookup(name).Changed
}

func stringFlag(cmd *cobra.Command, name string) (string, bool) {
	if cmd == nil || cmd.Flags().Lookup(name) == nil {
		return "", false
	}
	value, _ := cmd.Flags().GetString(name)
	return value, cmd.Flags().Lookup(name).Changed
}
