package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// NormalizeRootArgs lets the root command accept a workbench id as a positional
// argument even though cobra would otherwise treat an unknown first positional
// token as a subcommand name. Known subcommands are left untouched.
func NormalizeRootArgs(command *cobra.Command, args []string) []string {
	command.InitDefaultHelpCmd()
	command.InitDefaultCompletionCmd()

	out := append([]string(nil), args...)
	skipValue := false
	for i, arg := range out {
		if skipValue {
			skipValue = false
			continue
		}
		if arg == "--" {
			return out
		}
		if strings.HasPrefix(arg, "-") {
			skipValue = rootFlagNeedsSeparateValue(command, arg)
			continue
		}
		if isRootSubcommand(command, arg) {
			return out
		}
		return moveDirectArgsAfterFlags(command, out, i)
	}
	return out
}

func moveDirectArgsAfterFlags(command *cobra.Command, args []string, directIndex int) []string {
	flags := append([]string(nil), args[:directIndex]...)
	positionals := []string{args[directIndex]}
	skipValue := false
	for _, arg := range args[directIndex+1:] {
		if skipValue {
			flags = append(flags, arg)
			skipValue = false
			continue
		}
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			skipValue = rootFlagNeedsSeparateValue(command, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	withTerminator := make([]string, 0, len(args)+1)
	withTerminator = append(withTerminator, flags...)
	withTerminator = append(withTerminator, "--")
	withTerminator = append(withTerminator, positionals...)
	return withTerminator
}

func isRootSubcommand(command *cobra.Command, arg string) bool {
	for _, subcommand := range command.Commands() {
		if subcommand.Name() == arg || subcommand.HasAlias(arg) {
			return true
		}
	}
	return false
}

func rootFlagNeedsSeparateValue(command *cobra.Command, arg string) bool {
	if strings.HasPrefix(arg, "--") {
		name, _, hasInlineValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		flag := command.Flags().Lookup(name)
		return flag != nil && !hasInlineValue && flag.NoOptDefVal == ""
	}
	if len(arg) == 2 {
		flag := command.Flags().ShorthandLookup(arg[1:])
		return flag != nil && flag.NoOptDefVal == ""
	}
	return false
}
