package cmd

import (
	"github.com/spf13/cobra"
)

func newSwapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swap [workbench-id]",
		Short: "Swap branches with another workbench",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSwapDashboard(cmd, args)
		},
	}

	addNoTUISelectionFlags(cmd)
	return cmd
}
