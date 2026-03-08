package ui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	accentColor = lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"}
	mutedColor  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}

	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(mutedColor).Padding(0, 1)
	listTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(0, 1)
	listTitleBar     = lipgloss.NewStyle().Padding(0, 0, 1, 1)
)

func applyListTheme(l *list.Model) {
	styles := list.DefaultStyles()
	styles.Title = listTitleStyle
	styles.TitleBar = listTitleBar
	styles.StatusBar = styles.StatusBar.Foreground(mutedColor)
	styles.StatusEmpty = styles.StatusEmpty.Foreground(mutedColor)
	styles.PaginationStyle = styles.PaginationStyle.Foreground(mutedColor)
	styles.HelpStyle = styles.HelpStyle.Foreground(mutedColor)
	styles.ActivePaginationDot = lipgloss.NewStyle().Foreground(accentColor).SetString("•")
	styles.InactivePaginationDot = lipgloss.NewStyle().Foreground(mutedColor).SetString("•")
	styles.DividerDot = lipgloss.NewStyle().Foreground(mutedColor).SetString(" • ")
	l.Styles = styles
}

func renderHeader(title string) string {
	return headerStyle.Render(title)
}
