package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"worktree-bench/internal/version"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "worktree-bench",
		Short: "Manage a pool of git worktree workbenches",
	}

	rootCmd.AddCommand(newCreateCommand())
	rootCmd.AddCommand(newSwitchCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newAdoptCommand())
	rootCmd.AddCommand(newDeleteCommand())
	rootCmd.AddCommand(newVersionCommand())

	return rootCmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.String())
		},
	}
}
