package cmd

import (
	"github.com/spf13/cobra"
)

func newSwapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "swap",
		Short: "Swap branches with another workbench",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSwapDashboard(cmd, args)
		},
	}

	cmd.Flags().String("directive-file", "", "write cd directives to a file (for shell wrappers)")
	return cmd
}
