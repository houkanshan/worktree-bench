package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
)

func newResolveTargetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "resolve-target <pr-number|branch|github-url>",
		Short:        "Resolve a checkout target and find its existing worktree",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.TrimSpace(args[0])
			if target == "" {
				return fmt.Errorf("missing checkout target")
			}
			workingTree, _ := cmd.Flags().GetString("path")
			workingTree, err := filepath.Abs(workingTree)
			if err != nil {
				return fmt.Errorf("resolve checkout path: %w", err)
			}
			allowedRoots, _ := cmd.Flags().GetStringArray("allowed-root")
			if err := requireAllowedPath(workingTree, allowedRoots); err != nil {
				return err
			}
			resolved, err := bench.ResolveTarget(workingTree, target)
			if err != nil {
				return err
			}
			if resolved.ExistingPath != "" {
				if err := requireAllowedPath(resolved.ExistingPath, allowedRoots); err != nil {
					return err
				}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(resolved)
		},
	}
	cmd.Flags().String("path", ".", "repository working tree used to resolve the target")
	cmd.Flags().StringArray("allowed-root", nil, "limit resolved worktrees to paths under these roots")
	return cmd
}
