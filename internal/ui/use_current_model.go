package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type UseCurrentResult struct {
	UseCurrent bool
	Cancelled  bool
}

type useCurrentModel struct {
	list        list.Model
	currentPath string
	result      UseCurrentResult
}

type useCurrentItem struct {
	label string
	use   bool
}

func (u useCurrentItem) Title() string       { return u.label }
func (u useCurrentItem) Description() string { return "" }
func (u useCurrentItem) FilterValue() string { return u.label }

func RunUseCurrentPrompt(currentPath string) (UseCurrentResult, error) {
	items := []list.Item{
		useCurrentItem{label: "Use current worktree", use: true},
		useCurrentItem{label: "Create a new worktree", use: false},
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Current worktree not registered"
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(false)
	applyListTheme(&l)

	model := useCurrentModel{list: l, currentPath: currentPath}
	prog := tea.NewProgram(model, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return UseCurrentResult{}, err
	}
	m := final.(useCurrentModel)
	return m.result, nil
}

func (m useCurrentModel) Init() tea.Cmd { return nil }

func (m useCurrentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.result.Cancelled = true
			return m, tea.Quit
		case "enter":
			item := m.list.SelectedItem()
			if item == nil {
				return m, nil
			}
			choice, ok := item.(useCurrentItem)
			if !ok {
				return m, nil
			}
			m.result.UseCurrent = choice.use
			return m, tea.Quit
		}
	}

	updated, cmd := m.list.Update(msg)
	m.list = updated
	return m, cmd
}

func (m useCurrentModel) View() string {
	header := renderHeader("Current worktree not registered")
	body := fmt.Sprintf("Current: %s\n\nChoose an option:\n\n%s", m.currentPath, m.list.View())
	return fmt.Sprintf("%s\n\n%s", header, body)
}
