package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"worktree-bench/internal/config"
)

const (
	deleteStepSelect = iota
	deleteStepConfirm
	deleteStepDone
)

type DeleteResult struct {
	BenchID   string
	Cancelled bool
}

type deleteModel struct {
	step     int
	lists    map[string]list.Model
	types    []string
	tabIndex int
	selected *config.Workbench
	result   DeleteResult
}

func RunDelete(benches []config.Workbench) (DeleteResult, error) {
	statuses := LoadBenchStatuses(benches)
	model := newDeleteModel(benches, statuses)
	prog := tea.NewProgram(model)
	final, err := prog.Run()
	if err != nil {
		return DeleteResult{}, err
	}
	m := final.(deleteModel)
	return m.result, nil
}

func newDeleteModel(benches []config.Workbench, statuses map[string]BenchStatus) deleteModel {
	types := []string{config.TypeFull, config.TypeLight, config.TypeMinimal}
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

	return deleteModel{step: deleteStepSelect, lists: lists, types: types}
}

func (m deleteModel) Init() tea.Cmd { return nil }

func (m deleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.step == deleteStepSelect {
				m.tabIndex = (m.tabIndex + 1) % len(m.types)
				return m, nil
			}
		case "enter":
			if m.step == deleteStepSelect {
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
				m.step = deleteStepConfirm
				return m, nil
			}
		case "y":
			if m.step == deleteStepConfirm {
				m.result = DeleteResult{BenchID: m.selected.ID}
				m.step = deleteStepDone
				return m, tea.Quit
			}
		case "n":
			if m.step == deleteStepConfirm {
				m.result.Cancelled = true
				return m, tea.Quit
			}
		}
	}

	if m.step == deleteStepSelect {
		listModel := m.lists[m.types[m.tabIndex]]
		updated, cmd := listModel.Update(msg)
		m.lists[m.types[m.tabIndex]] = updated
		return m, cmd
	}

	return m, nil
}

func (m deleteModel) View() string {
	if m.step == deleteStepConfirm {
		header := renderHeader("Delete workbench")
		body := fmt.Sprintf("Delete %s? This removes the worktree directory. (y/n)", m.selected.Name)
		return fmt.Sprintf("%s\n\n%s", header, body)
	}

	return renderTabs(m.types, m.tabIndex, m.lists[m.types[m.tabIndex]].View())
}
