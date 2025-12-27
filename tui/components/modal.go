package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FuzzyMatch performs a simple fuzzy match (substring, case-insensitive).
func FuzzyMatch(pattern, text string) bool {
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)
	return strings.Contains(text, pattern)
}

// GetFuzzySuggestions returns tag suggestions based on fuzzy matching.
func GetFuzzySuggestions(input string, allTags []string, limit int) []string {
	if input == "" {
		if len(allTags) > limit {
			return allTags[:limit]
		}
		return allTags
	}

	var matches []string
	for _, tag := range allTags {
		if FuzzyMatch(input, tag) {
			matches = append(matches, tag)
		}
	}

	if len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

// RenderModal renders a modal dialog for input.
func RenderModal(modalType, input string, suggestions []string, selected int, width, height int, boxStyle, tabActive, tabInactive, footerStyle lipgloss.Style) string {
	modalWidth := min(60, width-4)
	modalHeight := 12
	if modalType == "help" {
		modalHeight = min(25, height-4)
	}

	startX := (width - modalWidth) / 2
	startY := (height - modalHeight) / 2

	if modalType == "help" {
		return renderHelpModal(modalWidth, modalHeight, startX, startY, boxStyle)
	}

	// New entry modal
	var lines []string
	lines = append(lines, boxStyle.Bold(true).Render("Start New Entry"))
	lines = append(lines, "")
	lines = append(lines, input+"_") // Cursor indicator
	lines = append(lines, "")
	lines = append(lines, "Tips:")
	lines = append(lines, "  • Tags: use #tag format (e.g., #project #work)")
	lines = append(lines, "  • Time: use @HH:MM for start time, @HH:MM @HH:MM for completed entry")

	// Show suggestions if available
	if len(suggestions) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Suggestions:")
		for i, sug := range suggestions {
			if i >= 5 {
				break
			}
			style := tabInactive
			if i == selected {
				style = tabActive
			}
			lines = append(lines, "  "+style.Render("#"+sug))
		}
	}

	lines = append(lines, "")
	lines = append(lines, footerStyle.Render("Enter: Confirm  Esc: Cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	modal := boxStyle.
		Width(modalWidth).
		Height(modalHeight).
		BorderForeground(lipgloss.Color("#00ff00")).
		Render(content)

	// Position the modal
	positioned := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
	return positioned
}

// renderHelpModal renders the help modal.
func renderHelpModal(width, height, startX, startY int, boxStyle lipgloss.Style) string {
	helpText := []string{
		"TUI Usage:",
		"",
		"Navigation:",
		"  1/2/3    - Switch view (Today/Week/Month)",
		"  Tab      - Switch scroll target",
		"  ↑/↓/k/j - Scroll active pane",
		"",
		"Actions:",
		"  n        - Start new entry",
		"  x        - Stop current entry",
		"  r        - Reload log file",
		"  q/Esc    - Quit",
		"  e/?      - Show this help",
		"",
		"Time Overrides:",
		"  @HH:MM   - Backdate start time for today",
		"  @HH:MM @HH:MM - Add completed entry",
		"  Example: \"Task @09:00\" or \"Task @09:00 @10:30\"",
		"",
		"Tags & Labels:",
		"  Tags are words starting with # in entry text",
		"  Example: \"Write docs #project #writing\"",
		"  Multiple tags allowed per entry",
		"",
		"Press Esc/q/e to close",
	}

	content := lipgloss.JoinVertical(lipgloss.Left, helpText...)
	modal := boxStyle.
		Width(width).
		Height(height).
		BorderForeground(lipgloss.Color("#00ff00")).
		Render(content)

	return modal
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
