package main

import (
	"os"

	"worktree-bench/internal/cmd"
)

func main() {
	rootCmd := cmd.NewRootCommand()
	rootCmd.SetArgs(cmd.NormalizeRootArgs(rootCmd, os.Args[1:]))
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
