package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"worktree-bench/internal/bench"
)

func newCheckoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "checkout <pr-number|branch|github-url>",
		Short:        "Checkout a PR or branch in an authorized working tree",
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
			return bench.CheckoutTarget(workingTree, target)
		},
	}
	cmd.Flags().String("path", ".", "working tree to update")
	cmd.Flags().StringArray("allowed-root", nil, "limit checkout to paths under these roots")
	return cmd
}
