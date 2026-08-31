package cmd

import (
	"reflect"
	"testing"
)

func TestRootCommandConsumesRepeatedAllowedRootsAfterWorkbenchID(t *testing.T) {
	command := NewRootCommand()
	normalized := NormalizeRootArgs(command, []string{
		"wb-9999999999999999999",
		"--require-reusable",
		"--allowed-root", "/tmp/one",
		"--allowed-root", "/tmp/two",
		"--init-cmd", "gnm",
		"--json",
	})
	if err := command.ParseFlags(normalized); err != nil {
		t.Fatalf("parse normalized arguments: %v", err)
	}
	if got, want := command.Flags().Args(), []string{"wb-9999999999999999999"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("positional arguments = %q, want %q", got, want)
	}
	roots, err := command.Flags().GetStringArray("allowed-root")
	if err != nil {
		t.Fatalf("read allowed roots: %v", err)
	}
	if want := []string{"/tmp/one", "/tmp/two"}; !reflect.DeepEqual(roots, want) {
		t.Fatalf("allowed roots = %q, want %q", roots, want)
	}
}

func TestNormalizeRootArgsDerivesValueTakingFlagsFromCommand(t *testing.T) {
	command := NewRootCommand()
	command.Flags().String("future-value", "", "test value-taking flag")
	normalized := NormalizeRootArgs(command, []string{"wb-999", "--future-value", "value"})
	if err := command.ParseFlags(normalized); err != nil {
		t.Fatalf("parse normalized arguments: %v", err)
	}
	if got, want := command.Flags().Args(), []string{"wb-999"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("positional arguments = %q, want %q", got, want)
	}
	if got, _ := command.Flags().GetString("future-value"); got != "value" {
		t.Fatalf("future-value = %q, want value", got)
	}
}
