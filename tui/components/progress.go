package components

import (
	"fmt"
	"lazytime/storage"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// formatDurationCompact formats duration as compact string (e.g., "5h", "8h", "30m").
func formatDurationCompact(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60

	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", totalSeconds)
}

// RenderProgressBar renders a progress bar for goal tracking.
// Uses a two-line layout: label + duration on top, bar on bottom.
func RenderProgressBar(current, target time.Duration, label string, barWidth int, progressStyle lipgloss.Style) string {
	if target <= 0 {
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff"))
		return labelStyle.Render(label + ": N/A")
	}

	percent := float64(current) / float64(target)
	if percent > 1.0 {
		percent = 1.0
	}

	// Format duration compactly (e.g., "5h/8h")
	currentStr := formatDurationCompact(current)
	targetStr := formatDurationCompact(target)
	durationText := currentStr + "/" + targetStr

	// Style the label with duration (e.g., "Today: 5h/8h")
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff"))
	durationStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	styledLabel := labelStyle.Render(label + ":")
	styledDuration := durationStyle.Render(" " + durationText)
	firstLine := styledLabel + styledDuration

	// Build progress bar
	filled := int(float64(barWidth) * percent)
	empty := barWidth - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}

	styledBar := progressStyle.Render(bar)

	// Format percentage and style it in bold
	percentText := fmt.Sprintf(" %.0f%%", percent*100)
	percentStyle := lipgloss.NewStyle().Bold(true)
	styledPercent := percentStyle.Render(percentText)

	// Join bar and percentage on the same line
	barLine := styledBar + styledPercent

	// Join label+duration (top) and bar+percentage (bottom) vertically
	return lipgloss.JoinVertical(lipgloss.Left,
		firstLine,
		barLine,
	)
}

// RenderGoalProgress renders progress bars for daily and weekly goals.
func RenderGoalProgress(entries []storage.Entry, now time.Time, targetToday, targetWeek time.Duration, width int, clampDuration func(storage.Entry, time.Time, time.Time, time.Time) time.Duration, getProgressStyle func(time.Duration, time.Duration) lipgloss.Style, formatDuration func(time.Duration) string) string {
	tz := now.Location()
	today := now

	// Calculate today's total
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, tz)
	todayEnd := todayStart.AddDate(0, 0, 1)
	todayStartUTC := storage.ToUTC(todayStart)
	todayEndUTC := storage.ToUTC(todayEnd)

	var todayTotal time.Duration
	for _, entry := range entries {
		todayTotal += clampDuration(entry, todayStartUTC, todayEndUTC, now)
	}

	// Calculate week's total
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekday-- // Monday = 0
	weekStart := today.AddDate(0, 0, -weekday)
	weekStartLocal := time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, tz)
	weekEndLocal := weekStartLocal.AddDate(0, 0, 7)
	weekStartUTC := storage.ToUTC(weekStartLocal)
	weekEndUTC := storage.ToUTC(weekEndLocal)

	var weekTotal time.Duration
	for _, entry := range entries {
		weekTotal += clampDuration(entry, weekStartUTC, weekEndUTC, now)
	}

	// Account for box padding (2 chars on each side = 4 total)
	boxPadding := 20
	availableWidth := width - boxPadding
	if availableWidth < 10 {
		availableWidth = 10
	}

	// Calculate the maximum bar width that both bars can use
	// Use the full available width for the bars to evenly space content
	barWidth := availableWidth
	if barWidth < 5 {
		barWidth = 5
	}

	// Render both bars with the same width
	todayBar := RenderProgressBar(todayTotal, targetToday, "Today", barWidth, getProgressStyle(todayTotal, targetToday))
	weekBar := RenderProgressBar(weekTotal, targetWeek, "Week", barWidth, getProgressStyle(weekTotal, targetWeek))

	return lipgloss.JoinVertical(lipgloss.Left, todayBar, weekBar)
}
