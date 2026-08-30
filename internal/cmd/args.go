package cmd

import "strings"

var rootSubcommands = map[string]struct{}{
	"adopt":      {},
	"completion": {},
	"create":     {},
	"delete":     {},
	"help":       {},
	"status":     {},
	"swap":       {},
	"version":    {},
}

var rootStringFlags = map[string]struct{}{
	"--base":           {},
	"--directive-file": {},
	"--init-cmd":       {},
	"--name":           {},
	"--type":           {},
}

// NormalizeRootArgs lets the root command accept a workbench id as a positional
// argument even though cobra would otherwise treat an unknown first positional
// token as a subcommand name. Known subcommands are left untouched.
func NormalizeRootArgs(args []string) []string {
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
		if strings.HasPrefix(arg, "--") {
			name := arg
			if before, _, ok := strings.Cut(arg, "="); ok {
				name = before
			}
			if _, ok := rootStringFlags[name]; ok && !strings.Contains(arg, "=") {
				skipValue = true
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if _, ok := rootSubcommands[arg]; ok {
			return out
		}
		return moveDirectArgsAfterFlags(out, i)
	}
	return out
}

func moveDirectArgsAfterFlags(args []string, directIndex int) []string {
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
		if strings.HasPrefix(arg, "--") {
			flags = append(flags, arg)
			name := arg
			if before, _, ok := strings.Cut(arg, "="); ok {
				name = before
			}
			if _, ok := rootStringFlags[name]; ok && !strings.Contains(arg, "=") {
				skipValue = true
			}
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
