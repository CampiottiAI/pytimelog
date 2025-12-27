package components

import (
	"lazytime/storage"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// removeTags removes #tag patterns from text.
func removeTags(text string) string {
	words := strings.Fields(text)
	var cleaned []string
	for _, word := range words {
		if !strings.HasPrefix(word, "#") {
			cleaned = append(cleaned, word)
		}
	}
	return strings.Join(cleaned, " ")
}

// RenderHero renders the hero section with large timer and current task info.
func RenderHero(entries []storage.Entry, now time.Time, width int, borderIdle, borderRunning, styleIdle, heroTimerStyle, heroTaskStyle, heroTagStyle lipgloss.Style, getTagColor func(string) lipgloss.Color, formatDuration, formatDurationShort func(time.Duration) string) string {
	idx := storage.FindOpen(entries)

	var lines []string

	if idx == -1 {
		// No active task - show idle state
		lines = append(lines, lipgloss.Place(width-4, 1, lipgloss.Center, lipgloss.Center, styleIdle.Render("IDLE")))
	} else {
		// Active task - show compact status with elapsed time and task description
		entry := entries[idx]
		elapsed := entry.Duration(now)

		// Format elapsed time in big bold green
		timerText := formatDuration(elapsed)
		styledTimer := heroTimerStyle.Render(timerText)

		// Get task description without tags
		taskDescription := removeTags(entry.Text)
		styledTask := heroTaskStyle.Render(taskDescription)

		// Create horizontal layout: [ELAPSED TIME] [TASK DESCRIPTION]
		// Account for border padding (2 chars on each side = 4 total)
		availableWidth := width - 4
		timerWidth := lipgloss.Width(styledTimer)
		taskWidth := lipgloss.Width(styledTask)
		spacing := 2 // Space between timer and task

		// If content fits, use simple layout
		if timerWidth+spacing+taskWidth <= availableWidth {
			content := styledTimer + strings.Repeat(" ", spacing) + styledTask
			lines = append(lines, content)
		} else {
			// If task is too long, truncate it
			maxTaskWidth := availableWidth - timerWidth - spacing
			if maxTaskWidth > 0 {
				// Truncate task description to fit
				truncatedTask := lipgloss.Place(maxTaskWidth, 1, lipgloss.Left, lipgloss.Top, styledTask)
				content := styledTimer + strings.Repeat(" ", spacing) + truncatedTask
				lines = append(lines, content)
			} else {
				// If even timer doesn't fit, just show timer
				lines = append(lines, styledTimer)
			}
		}
	}

	// Determine border style based on state
	var borderStyle lipgloss.Style
	if idx == -1 {
		borderStyle = borderIdle
	} else {
		borderStyle = borderRunning
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return borderStyle.Width(width).Render(content)
}
