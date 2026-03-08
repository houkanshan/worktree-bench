package ui

import (
	"fmt"
	"strings"
)

func renderTabs(labels []string, active int, body string) string {
	var rendered []string
	for i, label := range labels {
		style := inactiveTabStyle
		if i == active {
			style = activeTabStyle
		}
		rendered = append(rendered, style.Render(strings.ToUpper(label)))
	}
	return fmt.Sprintf("%s\n\n%s", strings.Join(rendered, " | "), body)
}
