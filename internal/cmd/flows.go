package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
	"worktree-bench/internal/config"
	"worktree-bench/internal/ui"
)

type createFlowResult struct {
	pool    config.Pool
	changed bool
	message string
}

func runCreateFlow(repoRoot string, settings config.Settings, pool config.Pool) (createFlowResult, error) {
	result, err := ui.RunCreate(pool.Benches)
	if err != nil {
		return createFlowResult{}, err
	}
	if result.Cancelled {
		return createFlowResult{pool: pool}, nil
	}

	if result.UseExisting {
		input := bench.CreateInput{
			Type:        result.Type,
			UseExisting: true,
			BenchID:     result.BenchID,
			Ref:         result.Ref,
		}
		updated, _, err := bench.CreateWorkbench(repoRoot, settings, pool, input)
		return createFlowResult{pool: updated}, err
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
			adoptResult, err := ui.RunAdoptWithType(repoRoot, defaultName, settings.SetupCmd, settings.DevCmd, result.Type)
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
			}
			updated, adopted, err := bench.AdoptWorkbench(repoRoot, settings, pool, input)
			message := ""
			if adopted != nil {
				message = fmt.Sprintf("Adopted workbench %s", adopted.Name)
			}
			return createFlowResult{pool: updated, changed: adopted != nil, message: message}, err
		}
	}

	input := bench.CreateInput{
		Type:        result.Type,
		UseExisting: false,
		Name:        result.Name,
		Ref:         result.Ref,
	}
	updated, created, err := bench.CreateWorkbench(repoRoot, settings, pool, input)
	message := ""
	if created != nil {
		message = fmt.Sprintf("Created workbench %s at %s", created.Name, created.Path)
	}
	return createFlowResult{pool: updated, changed: created != nil, message: message}, err
}

func runCreateFromDashboard(repoRoot string, settings config.Settings, pool config.Pool, benchType string) (createFlowResult, error) {
	if benchType == "" {
		benchType = config.TypeFull
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
			adoptResult, err := ui.RunAdoptWithType(repoRoot, defaultName, settings.SetupCmd, settings.DevCmd, benchType)
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
			}
			updated, adopted, err := bench.AdoptWorkbench(repoRoot, settings, pool, input)
			message := ""
			if adopted != nil {
				message = fmt.Sprintf("Adopted workbench %s", adopted.Name)
			}
			return createFlowResult{pool: updated, changed: adopted != nil, message: message}, err
		}
	}

	result, err := ui.RunCreateWithType(pool.Benches, benchType)
	if err != nil {
		return createFlowResult{}, err
	}
	if result.Cancelled {
		return createFlowResult{pool: pool}, nil
	}

	input := bench.CreateInput{
		Type:        benchType,
		UseExisting: false,
		Name:        result.Name,
		Ref:         result.Ref,
	}
	updated, created, err := bench.CreateWorkbench(repoRoot, settings, pool, input)
	message := ""
	if created != nil {
		message = fmt.Sprintf("Created workbench %s at %s", created.Name, created.Path)
	}
	return createFlowResult{pool: updated, changed: created != nil, message: message}, err
}

func runDeleteFlow(repoRoot string, pool config.Pool, benchID string, force bool) (config.Pool, string, error) {
	updated, removed, err := bench.DeleteWorkbench(repoRoot, pool, bench.DeleteInput{BenchID: benchID, Force: force})
	if err != nil {
		return pool, "", err
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
		return appendDirective(directiveFile, targetPath)
	}
	fmt.Printf("cd '%s'\n", targetPath)
	return nil
}
