package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"worktree-bench/internal/config"
)

const (
	adoptStepSelect = iota
	adoptStepInput
	adoptStepConfirm
	adoptStepDone
)

type AdoptResult struct {
	Type      string
	Name      string
	RunSetup  bool
	RunDev    bool
	RunInit   bool
	Cancelled bool
}

type adoptModel struct {
	step        int
	types       []string
	tabIndex    int
	lockType    bool
	nameInput   textinput.Model
	defaultName string
	currentPath string
	setupCmd    string
	devCmd      string
	initCmd     string
	runSetup    bool
	runDev      bool
	runInit     bool
	result      AdoptResult
}

func RunAdopt(currentPath string, defaultName string, setupCmd string, devCmd string, initCmd string) (AdoptResult, error) {
	return RunAdoptWithType(currentPath, defaultName, setupCmd, devCmd, initCmd, "")
}

func RunAdoptWithType(currentPath string, defaultName string, setupCmd string, devCmd string, initCmd string, fixedType string) (AdoptResult, error) {
	model := newAdoptModel(currentPath, defaultName, setupCmd, devCmd, initCmd, fixedType)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return AdoptResult{}, err
	}
	m := final.(adoptModel)
	return m.result, nil
}

func newAdoptModel(currentPath string, defaultName string, setupCmd string, devCmd string, initCmd string, fixedType string) adoptModel {
	types := config.WorkbenchTypes()
	nameInput := textinput.New()
	nameInput.Placeholder = defaultName
	nameInput.CharLimit = 64
	nameInput.Focus()

	tabIndex := 0
	lockType := false
	step := adoptStepSelect
	if fixedType != "" {
		for i, value := range types {
			if value == fixedType {
				tabIndex = i
				lockType = true
				step = adoptStepInput
				break
			}
		}
	}

	return adoptModel{
		step:        step,
		types:       types,
		tabIndex:    tabIndex,
		lockType:    lockType,
		nameInput:   nameInput,
		defaultName: defaultName,
		currentPath: currentPath,
		setupCmd:    setupCmd,
		devCmd:      devCmd,
		initCmd:     initCmd,
		runInit:     strings.TrimSpace(initCmd) != "",
	}
}

func (m adoptModel) Init() tea.Cmd { return nil }

func (m adoptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.result.Cancelled = true
			return m, tea.Quit
		case "tab":
			if m.step == adoptStepSelect && !m.lockType {
				m.tabIndex = (m.tabIndex + 1) % len(m.types)
				return m, nil
			}
		case "enter":
			switch m.step {
			case adoptStepSelect:
				m.step = adoptStepInput
				m.nameInput.Focus()
				return m, nil
			case adoptStepInput:
				m.step = adoptStepConfirm
				m.ensureEligibility()
				return m, nil
			case adoptStepConfirm:
				m.result = AdoptResult{
					Type:     m.types[m.tabIndex],
					Name:     m.resolveName(),
					RunSetup: m.runSetup,
					RunDev:   m.runDev,
					RunInit:  m.runInit,
				}
				m.step = adoptStepDone
				return m, tea.Quit
			}
		case "s":
			if m.step == adoptStepConfirm && m.setupEligible() {
				m.runSetup = !m.runSetup
				return m, nil
			}
		case "d":
			if m.step == adoptStepConfirm && m.devEligible() {
				m.runDev = !m.runDev
				return m, nil
			}
		case "i":
			if m.step == adoptStepConfirm && m.initEligible() {
				m.runInit = !m.runInit
				return m, nil
			}
		}
	}

	if m.step == adoptStepInput {
		updated, cmd := m.nameInput.Update(msg)
		m.nameInput = updated
		return m, cmd
	}

	return m, nil
}

func (m adoptModel) View() string {
	switch m.step {
	case adoptStepInput:
		header := renderHeader("Adopt current worktree")
		body := fmt.Sprintf("Current: %s\nType: %s\n\nWorkbench name (default: %s):\n%s\n\n(enter to continue)", m.currentPath, strings.ToUpper(m.types[m.tabIndex]), m.defaultName, m.nameInput.View())
		return fmt.Sprintf("%s\n\n%s", header, body)
	case adoptStepConfirm:
		header := renderHeader("Confirm adoption")
		lines := []string{
			fmt.Sprintf("Type: %s", strings.ToUpper(m.types[m.tabIndex])),
			fmt.Sprintf("Name: %s", m.resolveName()),
		}
		if m.setupEligible() {
			status := "no"
			if m.runSetup {
				status = "yes"
			}
			lines = append(lines, fmt.Sprintf("Setup (s): %s (%s)", status, m.setupCmd))
		} else {
			lines = append(lines, "Setup: n/a")
		}
		if m.devEligible() {
			status := "no"
			if m.runDev {
				status = "yes"
			}
			lines = append(lines, fmt.Sprintf("Dev server (d): %s (%s)", status, m.devCmd))
		} else {
			lines = append(lines, "Dev server: n/a")
		}
		if m.initEligible() {
			status := "no"
			if m.runInit {
				status = "yes"
			}
			lines = append(lines, fmt.Sprintf("Init (i): %s (%s)", status, m.initCmd))
		} else {
			lines = append(lines, "Init: n/a")
		}
		lines = append(lines, "\n(enter to confirm)")
		return fmt.Sprintf("%s\n\n%s", header, strings.Join(lines, "\n"))
	default:
		body := fmt.Sprintf("Current: %s\n\n(tab to switch type, enter to continue)", m.currentPath)
		return renderTabs(m.types, m.tabIndex, body)
	}
}

func (m adoptModel) resolveName() string {
	name := strings.TrimSpace(m.nameInput.Value())
	if name == "" {
		return m.defaultName
	}
	return name
}

func (m adoptModel) setupEligible() bool {
	return m.setupCmd != "" && m.types[m.tabIndex] != config.TypeSmall
}

func (m adoptModel) devEligible() bool {
	return m.devCmd != "" && m.types[m.tabIndex] == config.TypeLarge
}

func (m adoptModel) initEligible() bool {
	return strings.TrimSpace(m.initCmd) != ""
}

func (m *adoptModel) ensureEligibility() {
	if !m.setupEligible() {
		m.runSetup = false
	}
	if !m.devEligible() {
		m.runDev = false
	}
	if !m.initEligible() {
		m.runInit = false
	}
}
