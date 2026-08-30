package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func directSelectionForTest(t *testing.T, flags []string, args []string) (directSelection, bool, error) {
	t.Helper()
	command := &cobra.Command{}
	addNoTUISelectionFlags(command)
	if err := command.Flags().Parse(flags); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return parseDirectSelection(command, args)
}

func TestParseDirectSelectionJSONExisting(t *testing.T) {
	selection, direct, err := directSelectionForTest(t, []string{"--json", "--init-cmd", "gnm"}, []string{"wb-123"})
	if err != nil {
		t.Fatal(err)
	}
	if !direct || !selection.JSON || selection.BenchID != "wb-123" || selection.CreateNew {
		t.Fatalf("unexpected selection: %+v direct=%t", selection, direct)
	}
}

func TestParseDirectSelectionJSONNew(t *testing.T) {
	selection, direct, err := directSelectionForTest(t, []string{"--new", "--type", "large", "--json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !direct || !selection.JSON || !selection.CreateNew || selection.Type != "large" {
		t.Fatalf("unexpected selection: %+v direct=%t", selection, direct)
	}
}

func TestParseDirectSelectionJSONRequiresTarget(t *testing.T) {
	_, _, err := directSelectionForTest(t, []string{"--json"}, nil)
	if err == nil {
		t.Fatal("expected --json without a direct target to fail")
	}
}

func TestSelectionJSONContract(t *testing.T) {
	payload, err := json.Marshal(selectionJSON{
		BenchID: "wb-123",
		Name:    "repo-l-1",
		Type:    "large",
		Path:    "/tmp/repo-l-1",
		Created: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"benchId":"wb-123","name":"repo-l-1","type":"large","path":"/tmp/repo-l-1","created":true}`
	if string(payload) != want {
		t.Fatalf("unexpected JSON: %s", payload)
	}
}
