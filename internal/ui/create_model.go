package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"worktree-bench/internal/config"
)

const (
	createStepSelect = iota
	createStepInput
	createStepDone
)

const (
	baseBranchMaster  = "master"
	baseBranchCurrent = "current"
)

type baseBranchOption struct {
	label string
	value string
}

type CreateResult struct {
	Type        string
	UseExisting bool
	BenchID     string
	Name        string
	BaseBranch  string
	Cancelled   bool
}

type createModel struct {
	step        int
	lists       map[string]list.Model
	types       []string
	tabIndex    int
	lockType    bool
	selected    *config.Workbench
	nameInput   textinput.Model
	baseOptions []baseBranchOption
	baseIndex   int
	focusIndex  int
	result      CreateResult
}

func RunCreate(benches []config.Workbench) (CreateResult, error) {
	return RunCreateWithType(benches, "")
}

func RunCreateWithType(benches []config.Workbench, fixedType string) (CreateResult, error) {
	statuses := LoadBenchStatuses(benches)
	model := newCreateModel(benches, statuses, fixedType)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return CreateResult{}, err
	}
	m := final.(createModel)
	return m.result, nil
}

func newCreateModel(benches []config.Workbench, statuses map[string]BenchStatus, fixedType string) createModel {
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

	nameInput := textinput.New()
	nameInput.Placeholder = "auto"
	nameInput.CharLimit = 64
	nameInput.Blur()

	baseOptions := []baseBranchOption{
		{label: "master/main", value: baseBranchMaster},
		{label: "current branch", value: baseBranchCurrent},
	}
	baseIndex := 0

	tabIndex := 0
	lockType := false
	step := createStepSelect
	if fixedType != "" {
		for i, value := range types {
			if value == fixedType {
				tabIndex = i
				lockType = true
				step = createStepInput
				break
			}
		}
	}

	return createModel{
		step:        step,
		lists:       lists,
		types:       types,
		tabIndex:    tabIndex,
		lockType:    lockType,
		nameInput:   nameInput,
		baseOptions: baseOptions,
		baseIndex:   baseIndex,
		focusIndex:  1,
	}
}

func (m createModel) Init() tea.Cmd { return nil }

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.step == createStepSelect && !m.lockType {
				m.tabIndex = (m.tabIndex + 1) % len(m.types)
				return m, nil
			}
			if m.step == createStepInput {
				m.focusIndex = (m.focusIndex + 1) % 2
				if m.focusIndex == 0 {
					m.nameInput.Focus()
				} else {
					m.nameInput.Blur()
				}
				return m, nil
			}
		case "enter":
			if m.step == createStepSelect {
				listModel := m.lists[m.types[m.tabIndex]]
				item := listModel.SelectedItem()
				if item == nil {
					return m, nil
				}
				if isNewItem(item) {
					m.selected = nil
				} else if bench, ok := itemWorkbench(item); ok {
					m.selected = &bench
				}
				m.step = createStepInput
				m.focusIndex = 1
				m.nameInput.Blur()
				return m, nil
			}
			if m.step == createStepInput {
				m.result = CreateResult{
					Type:        m.types[m.tabIndex],
					UseExisting: m.selected != nil,
					BaseBranch:  m.baseOptions[m.baseIndex].value,
					Name:        m.nameInput.Value(),
				}
				if m.selected != nil {
					m.result.BenchID = m.selected.ID
				}
				m.step = createStepDone
				return m, tea.Quit
			}
		case "up", "k", "left", "h":
			if m.step == createStepInput && m.focusIndex == 1 {
				m.baseIndex = (m.baseIndex + len(m.baseOptions) - 1) % len(m.baseOptions)
				return m, nil
			}
		case "down", "j", "right", "l":
			if m.step == createStepInput && m.focusIndex == 1 {
				m.baseIndex = (m.baseIndex + 1) % len(m.baseOptions)
				return m, nil
			}
		}
	}

	if m.step == createStepSelect {
		listModel := m.lists[m.types[m.tabIndex]]
		updated, cmd := listModel.Update(msg)
		m.lists[m.types[m.tabIndex]] = updated
		return m, cmd
	}

	if m.step == createStepInput {
		if m.focusIndex == 0 {
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m createModel) View() string {
	if m.step == createStepInput {
		header := renderHeader("Create workbench")
		nameLabel := "Workbench name (optional):"
		if m.selected != nil {
			nameLabel = "Workbench name (ignored for reuse):"
		}
		baseOptions := renderBaseBranchOptions(m.baseOptions, m.baseIndex)
		return fmt.Sprintf("%s\n\nType: %s\n\n%s\n%s\n\nBase branch:\n%s\n\n(tab to switch input, arrows to change base, enter to confirm)",
			header,
			m.types[m.tabIndex],
			nameLabel,
			m.nameInput.View(),
			baseOptions,
		)
	}

	return renderTabs(m.types, m.tabIndex, m.lists[m.types[m.tabIndex]].View())
}

func renderBaseBranchOptions(options []baseBranchOption, active int) string {
	parts := make([]string, 0, len(options))
	for i, option := range options {
		style := inactiveTabStyle
		if i == active {
			style = activeTabStyle
		}
		parts = append(parts, style.Render(option.label))
	}
	return strings.Join(parts, " | ")
}
