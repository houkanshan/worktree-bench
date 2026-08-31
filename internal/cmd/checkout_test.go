package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutCommandRejectsUnauthorizedPathBeforeCheckout(t *testing.T) {
	allowed := t.TempDir()
	outside := filepath.Join(filepath.Dir(allowed), "outside")
	command := newCheckoutCommand()
	command.SilenceErrors = true
	command.SetArgs([]string{"feature", "--path", outside, "--allowed-root", allowed})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "outside allowed roots") {
		t.Fatalf("expected authorization failure, got %v", err)
	}
}
