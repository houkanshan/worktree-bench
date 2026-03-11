package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"worktree-bench/internal/config"
)

const (
	dashboardStepList = iota
	dashboardStepConfirmDelete
	dashboardStepDone
)

type DashboardAction string

const (
	DashboardActionNone   DashboardAction = "none"
	DashboardActionCreate DashboardAction = "create"
	DashboardActionSwitch DashboardAction = "switch"
	DashboardActionDelete DashboardAction = "delete"
)

type DashboardResult struct {
	Action    DashboardAction
	Type      string
	BenchID   string
	Cancelled bool
}

type dashboardModel struct {
	step     int
	lists    map[string]list.Model
	types    []string
	tabIndex int
	selected *config.Workbench
	result   DashboardResult
}

func RunDashboard(benches []config.Workbench) (DashboardResult, error) {
	statuses := LoadBenchStatuses(benches)
	model := newDashboardModel(benches, statuses)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return DashboardResult{}, err
	}
	m := final.(dashboardModel)
	return m.result, nil
}

func newDashboardModel(benches []config.Workbench, statuses map[string]BenchStatus) dashboardModel {
	types := config.WorkbenchTypes()
	lists := make(map[string]list.Model)

	for _, benchType := range types {
		items := []list.Item{newNewBenchItem()}
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

	return dashboardModel{step: dashboardStepList, lists: lists, types: types}
}

func (m dashboardModel) Init() tea.Cmd { return nil }

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.step == dashboardStepList {
				m.tabIndex = (m.tabIndex + 1) % len(m.types)
				return m, nil
			}
		case "enter":
			if m.step == dashboardStepList {
				listModel := m.lists[m.types[m.tabIndex]]
				item := listModel.SelectedItem()
				if item == nil {
					return m, nil
				}
				if isNewItem(item) {
					m.result.Action = DashboardActionCreate
					m.result.Type = m.types[m.tabIndex]
					m.step = dashboardStepDone
					return m, tea.Quit
				}
				bench, ok := itemWorkbench(item)
				if !ok {
					return m, nil
				}
				m.result = DashboardResult{Action: DashboardActionSwitch, BenchID: bench.ID}
				m.step = dashboardStepDone
				return m, tea.Quit
			}
			if m.step == dashboardStepConfirmDelete {
				if m.selected == nil {
					return m, nil
				}
				m.result = DashboardResult{Action: DashboardActionDelete, BenchID: m.selected.ID}
				m.step = dashboardStepDone
				return m, tea.Quit
			}
		case "d":
			if m.step == dashboardStepList {
				listModel := m.lists[m.types[m.tabIndex]]
				item := listModel.SelectedItem()
				if item == nil || isNewItem(item) {
					return m, nil
				}
				bench, ok := itemWorkbench(item)
				if !ok {
					return m, nil
				}
				m.selected = &bench
				m.step = dashboardStepConfirmDelete
				return m, nil
			}
		case "y":
			if m.step == dashboardStepConfirmDelete {
				if m.selected == nil {
					return m, nil
				}
				m.result = DashboardResult{Action: DashboardActionDelete, BenchID: m.selected.ID}
				m.step = dashboardStepDone
				return m, tea.Quit
			}
		case "n":
			if m.step == dashboardStepConfirmDelete {
				m.step = dashboardStepList
				m.selected = nil
				return m, nil
			}
		}
	}

	if m.step == dashboardStepList {
		listModel := m.lists[m.types[m.tabIndex]]
		updated, cmd := listModel.Update(msg)
		m.lists[m.types[m.tabIndex]] = updated
		return m, cmd
	}

	return m, nil
}

func (m dashboardModel) View() string {
	if m.step == dashboardStepConfirmDelete {
		header := renderHeader("Delete workbench")
		return fmt.Sprintf("%s\n\nDelete %s? (y/n)", header, m.selected.Name)
	}

	return renderTabs(m.types, m.tabIndex, m.lists[m.types[m.tabIndex]].View())
}
