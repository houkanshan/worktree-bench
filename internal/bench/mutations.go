package bench

import "worktree-bench/internal/config"

// CreateWorkbench creates or reuses a bench and persists its fresh pool under
// the repository lock. Callers must finish collecting user input first.
func CreateWorkbench(repoRoot string, settings config.Settings, input CreateInput) (config.Pool, *config.Workbench, string, error) {
	var selected *config.Workbench
	var target string
	pool, err := config.UpdatePool(repoRoot, settings, func(pool config.Pool) (config.Pool, error) {
		updated, bench, path, err := createWorkbench(repoRoot, settings, pool, input)
		selected, target = bench, path
		return updated, err
	})
	if err != nil {
		return pool, nil, "", err
	}
	return pool, selected, target, nil
}

// AdoptWorkbench registers the current worktree without overwriting changes
// made by another process while the user was answering prompts.
func AdoptWorkbench(repoRoot string, settings config.Settings, input AdoptInput) (config.Pool, *config.Workbench, error) {
	var adopted *config.Workbench
	pool, err := config.UpdatePool(repoRoot, settings, func(pool config.Pool) (config.Pool, error) {
		updated, bench, err := adoptWorkbench(repoRoot, settings, pool, input)
		adopted = bench
		return updated, err
	})
	if err != nil {
		return pool, nil, err
	}
	return pool, adopted, nil
}

// DeleteWorkbench revalidates the selected ID against the current pool before
// removing the worktree and its registration in the same transaction.
func DeleteWorkbench(repoRoot string, settings config.Settings, input DeleteInput) (config.Pool, *config.Workbench, error) {
	var removed *config.Workbench
	pool, err := config.UpdatePool(repoRoot, settings, func(pool config.Pool) (config.Pool, error) {
		updated, bench, err := deleteWorkbench(repoRoot, pool, input)
		removed = bench
		return updated, err
	})
	if err != nil {
		return pool, nil, err
	}
	return pool, removed, nil
}
