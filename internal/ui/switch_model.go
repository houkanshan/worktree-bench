package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"worktree-bench/internal/config"
)

const (
	switchStepSelect = iota
	switchStepConfirm
	switchStepDone
)

type SwitchResult struct {
	BenchID   string
	Swap      bool
	Cancelled bool
}

type switchModel struct {
	step     int
	lists    map[string]list.Model
	types    []string
	tabIndex int
	selected *config.Workbench
	swap     bool
	result   SwitchResult
}

func RunSwitch(benches []config.Workbench) (SwitchResult, error) {
	statuses := LoadBenchStatuses(benches)
	model := newSwitchModel(benches, statuses)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return SwitchResult{}, err
	}
	m := final.(switchModel)
	return m.result, nil
}

func newSwitchModel(benches []config.Workbench, statuses map[string]BenchStatus) switchModel {
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

	return switchModel{
		step:  switchStepSelect,
		lists: lists,
		types: types,
	}
}

func (m switchModel) Init() tea.Cmd { return nil }

func (m switchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.result.Cancelled = true
			return m, tea.Quit
		case "tab":
			if m.step == switchStepSelect {
				m.tabIndex = (m.tabIndex + 1) % len(m.types)
				return m, nil
			}
		case "enter":
			if m.step == switchStepSelect {
				listModel := m.lists[m.types[m.tabIndex]]
				item := listModel.SelectedItem()
				if item == nil {
					return m, nil
				}
				bench, ok := itemWorkbench(item)
				if !ok {
					return m, nil
				}
				m.selected = &bench
				m.step = switchStepConfirm
				return m, nil
			}
			if m.step == switchStepConfirm {
				m.result = SwitchResult{BenchID: m.selected.ID, Swap: m.swap}
				m.step = switchStepDone
				return m, tea.Quit
			}
		case "y":
			if m.step == switchStepConfirm {
				m.swap = true
				return m, nil
			}
		case "n":
			if m.step == switchStepConfirm {
				m.swap = false
				return m, nil
			}
		}
	}

	if m.step == switchStepSelect {
		listModel := m.lists[m.types[m.tabIndex]]
		updated, cmd := listModel.Update(msg)
		m.lists[m.types[m.tabIndex]] = updated
		return m, cmd
	}

	return m, nil
}

func (m switchModel) View() string {
	if m.step == switchStepConfirm {
		header := renderHeader("Switch workbench")
		body := "Swap branches with current worktree? (y/n, enter to confirm)"
		selection := "no"
		if m.swap {
			selection = "yes"
		}
		return fmt.Sprintf("%s\n\n%s\nSelected: %s", header, body, selection)
	}

	return renderTabs(m.types, m.tabIndex, m.lists[m.types[m.tabIndex]].View())
}
