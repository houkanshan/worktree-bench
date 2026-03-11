package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"worktree-bench/internal/config"
)

type statusModel struct {
	lists    map[string]list.Model
	types    []string
	tabIndex int
}

func RunStatus(benches []config.Workbench) error {
	statuses := LoadBenchStatuses(benches)
	model := newStatusModel(benches, statuses)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	_, err := prog.Run()
	return err
}

func newStatusModel(benches []config.Workbench, statuses map[string]BenchStatus) statusModel {
	types := config.WorkbenchTypes()
	lists := make(map[string]list.Model)
	for _, benchType := range types {
		items := []list.Item{}
		for _, bench := range benches {
			if bench.Type != benchType {
				continue
			}
			desc := FormatStatusLine(statuses[bench.ID])
			items = append(items, newBenchItem(bench, desc))
		}
		l := list.New(items, list.NewDefaultDelegate(), 0, 0)
		l.Title = fmt.Sprintf("%s workbenches", benchType)
		l.SetShowStatusBar(false)
		l.SetShowFilter(false)
		l.SetFilteringEnabled(false)
		applyListTheme(&l)
		lists[benchType] = l
	}

	return statusModel{lists: lists, types: types}
}

func (m statusModel) Init() tea.Cmd { return nil }

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		for _, benchType := range m.types {
			listModel := m.lists[benchType]
			listModel.SetSize(msg.Width, msg.Height-4)
			m.lists[benchType] = listModel
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.tabIndex = (m.tabIndex + 1) % len(m.types)
			return m, nil
		}
	}

	listModel := m.lists[m.types[m.tabIndex]]
	updated, cmd := listModel.Update(msg)
	m.lists[m.types[m.tabIndex]] = updated
	return m, cmd
}

func (m statusModel) View() string {
	return renderTabs(m.types, m.tabIndex, m.lists[m.types[m.tabIndex]].View())
}
