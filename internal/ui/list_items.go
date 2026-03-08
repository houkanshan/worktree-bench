package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"worktree-bench/internal/config"
)

type benchItem struct {
	bench config.Workbench
	label string
	desc  string
	isNew bool
}

func (b benchItem) Title() string       { return b.label }
func (b benchItem) Description() string { return b.desc }
func (b benchItem) FilterValue() string { return b.label }

func newBenchItem(bench config.Workbench, desc string) list.Item {
	return benchItem{bench: bench, label: fmt.Sprintf("%s (%s)", bench.Name, bench.Type), desc: desc}
}

func newNewBenchItem() list.Item {
	return benchItem{label: "➕ Create new workbench", desc: "Create a fresh worktree", isNew: true}
}

func isNewItem(item list.Item) bool {
	if bench, ok := item.(benchItem); ok {
		return bench.isNew
	}
	return false
}

func itemWorkbench(item list.Item) (config.Workbench, bool) {
	bench, ok := item.(benchItem)
	if !ok {
		return config.Workbench{}, false
	}
	return bench.bench, true
}
