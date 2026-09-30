package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/gitutil"
	"worktree-bench/internal/ui"
)

type createFlowResult struct {
	pool         config.Pool
	changed      bool
	message      string
	targetPath   string
	touchedIDs   []string
	touchedPaths []string
}

func runCreateFlow(repoRoot string, settings config.Settings, pool config.Pool) (createFlowResult, error) {
	result, err := ui.RunCreate(pool.Benches, settings.InitCmd)
	if err != nil {
		return createFlowResult{}, err
	}
	if result.Cancelled {
		return createFlowResult{pool: pool}, nil
	}

	if result.UseExisting {
		baseBranch, err := resolveBaseBranch(repoRoot, result.BaseBranch)
		if err != nil {
			return createFlowResult{}, err
		}
		input := bench.CreateInput{
			Type:        result.Type,
			UseExisting: true,
			BenchID:     result.BenchID,
			BaseBranch:  baseBranch,
			RunInit:     strings.TrimSpace(settings.InitCmd) != "",
		}
		updated, touched, targetPath, err := bench.CreateWorkbench(repoRoot, settings, input)
		flow := createFlowResult{pool: updated, changed: touched != nil, targetPath: targetPath}
		if touched != nil {
			flow.touchedIDs = []string{touched.ID}
			flow.touchedPaths = []string{touched.Path}
		}
		return flow, err
	}

	if bench.FindWorkbenchByPath(pool, repoRoot) == nil {
		choice, err := ui.RunUseCurrentPrompt(repoRoot)
		if err != nil {
			return createFlowResult{}, err
		}
		if choice.Cancelled {
			return createFlowResult{pool: pool}, nil
		}
		if choice.UseCurrent {
			defaultName := result.Name
			if defaultName == "" {
				defaultName = filepath.Base(repoRoot)
			}
			adoptResult, err := ui.RunAdoptWithType(repoRoot, defaultName, settings.SetupCmd, settings.DevCmd, settings.InitCmd, result.Type)
			if err != nil {
				return createFlowResult{}, err
			}
			if adoptResult.Cancelled {
				return createFlowResult{pool: pool}, nil
			}
			input := bench.AdoptInput{
				Type:     adoptResult.Type,
				Name:     adoptResult.Name,
				RunSetup: adoptResult.RunSetup,
				RunDev:   adoptResult.RunDev,
				RunInit:  adoptResult.RunInit,
			}
			updated, adopted, err := bench.AdoptWorkbench(repoRoot, settings, input)
			message := ""
			if adopted != nil {
				message = fmt.Sprintf("Adopted workbench %s", adopted.Name)
			}
			flow := createFlowResult{pool: updated, changed: adopted != nil, message: message}
			if adopted != nil {
				flow.touchedIDs = []string{adopted.ID}
				flow.touchedPaths = []string{adopted.Path}
			}
			return flow, err
		}
	}

	baseBranch, err := resolveBaseBranch(repoRoot, result.BaseBranch)
	if err != nil {
		return createFlowResult{}, err
	}
	input := bench.CreateInput{
		Type:        result.Type,
		UseExisting: false,
		Name:        result.Name,
		BaseBranch:  baseBranch,
		RunInit:     result.RunInit,
	}
	updated, created, targetPath, err := bench.CreateWorkbench(repoRoot, settings, input)
	message := ""
	if created != nil {
		message = fmt.Sprintf("Created workbench %s at %s", created.Name, created.Path)
	}
	flow := createFlowResult{pool: updated, changed: created != nil, message: message, targetPath: targetPath}
	if created != nil {
		flow.touchedIDs = []string{created.ID}
		flow.touchedPaths = []string{created.Path}
	}
	return flow, err
}

func runCreateFromDashboard(repoRoot string, settings config.Settings, pool config.Pool, benchType string) (createFlowResult, error) {
	if benchType == "" {
		benchType = config.TypeLarge
	}

	if bench.FindWorkbenchByPath(pool, repoRoot) == nil {
		choice, err := ui.RunUseCurrentPrompt(repoRoot)
		if err != nil {
			return createFlowResult{}, err
		}
		if choice.Cancelled {
			return createFlowResult{pool: pool}, nil
		}
		if choice.UseCurrent {
			defaultName := filepath.Base(repoRoot)
			adoptResult, err := ui.RunAdoptWithType(repoRoot, defaultName, settings.SetupCmd, settings.DevCmd, settings.InitCmd, benchType)
			if err != nil {
				return createFlowResult{}, err
			}
			if adoptResult.Cancelled {
				return createFlowResult{pool: pool}, nil
			}
			input := bench.AdoptInput{
				Type:     adoptResult.Type,
				Name:     adoptResult.Name,
				RunSetup: adoptResult.RunSetup,
				RunDev:   adoptResult.RunDev,
				RunInit:  adoptResult.RunInit,
			}
			updated, adopted, err := bench.AdoptWorkbench(repoRoot, settings, input)
			message := ""
			if adopted != nil {
				message = fmt.Sprintf("Adopted workbench %s", adopted.Name)
			}
			flow := createFlowResult{pool: updated, changed: adopted != nil, message: message}
			if adopted != nil {
				flow.touchedIDs = []string{adopted.ID}
				flow.touchedPaths = []string{adopted.Path}
			}
			return flow, err
		}
	}

	result, err := ui.RunCreateWithType(pool.Benches, benchType, settings.InitCmd)
	if err != nil {
		return createFlowResult{}, err
	}
	if result.Cancelled {
		return createFlowResult{pool: pool}, nil
	}

	baseBranch, err := resolveBaseBranch(repoRoot, result.BaseBranch)
	if err != nil {
		return createFlowResult{}, err
	}
	input := bench.CreateInput{
		Type:        benchType,
		UseExisting: false,
		Name:        result.Name,
		BaseBranch:  baseBranch,
		RunInit:     result.RunInit,
	}
	updated, created, targetPath, err := bench.CreateWorkbench(repoRoot, settings, input)
	message := ""
	if created != nil {
		message = fmt.Sprintf("Created workbench %s at %s", created.Name, created.Path)
	}
	flow := createFlowResult{pool: updated, changed: created != nil, message: message, targetPath: targetPath}
	if created != nil {
		flow.touchedIDs = []string{created.ID}
		flow.touchedPaths = []string{created.Path}
	}
	return flow, err
}

func runDeleteFlow(repoRoot string, settings config.Settings, benchID string, force bool) (config.Pool, string, error) {
	updated, removed, err := bench.DeleteWorkbench(repoRoot, settings, bench.DeleteInput{BenchID: benchID, Force: force})
	if err != nil {
		return updated, "", err
	}
	message := ""
	if removed != nil {
		message = fmt.Sprintf("Deleted workbench %s", removed.Name)
	}
	return updated, message, nil
}

const directiveEnv = "WTB_DIRECTIVE_FILE"

func emitDirective(cmd *cobra.Command, targetPath string) error {
	directiveFile := ""
	if cmd != nil {
		directiveFile, _ = cmd.Flags().GetString("directive-file")
	}
	if directiveFile == "" {
		directiveFile = os.Getenv(directiveEnv)
	}
	if directiveFile != "" {
		if os.Getenv("WTB_TRACE") != "" {
			fmt.Fprintf(os.Stderr, "[wtb] directive-file=%s cd '%s'\n", directiveFile, targetPath)
		}
		return appendDirective(directiveFile, targetPath)
	}
	if os.Getenv("WTB_TRACE") != "" {
		fmt.Fprintf(os.Stderr, "[wtb] directive-stdout cd '%s'\n", targetPath)
	}
	fmt.Printf("cd '%s'\n", targetPath)
	return nil
}

func resolveBaseBranch(repoRoot string, selection string) (string, error) {
	value := strings.TrimSpace(selection)
	if value == "" {
		value = baseBranchMaster
	}
	if value == baseBranchMaster {
		return gitutil.ResolvePrimaryBranch(repoRoot)
	}
	if value == baseBranchCurrent {
		branch, err := gitutil.Branch(repoRoot)
		if err != nil {
			return "", err
		}
		if branch == "HEAD" {
			return "", fmt.Errorf("current worktree is in detached HEAD state")
		}
		return branch, nil
	}
	return value, nil
}

const (
	baseBranchMaster  = "master"
	baseBranchCurrent = "current"
)
