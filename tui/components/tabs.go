package components

import (
	"github.com/charmbracelet/lipgloss"
)

// ViewMode type for tabs (defined in tui package, passed as int)
type ViewMode int

const (
	ViewToday ViewMode = iota
	ViewWeek
)

// RenderTabs renders the tab navigation bar.
func RenderTabs(activeView ViewMode, width int, tabActive, tabInactive lipgloss.Style) string {
	tabs := []string{"Today", "Week"}
	var renderedTabs []string

	for i, tab := range tabs {
		if ViewMode(i) == activeView {
			renderedTabs = append(renderedTabs, tabActive.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, tabInactive.Render(tab))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...)
}
