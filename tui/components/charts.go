package components

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TagChartItem represents a tag with its duration for chart display.
type TagChartItem struct {
	Tag      string
	Duration time.Duration
	Percent  float64
}

// RenderTagChart renders a horizontal bar chart showing tag distribution.
func RenderTagChart(totals map[string]time.Duration, width, height int, chartBarStyle, chartLabelStyle, chartPercentStyle, boxStyle lipgloss.Style, getTagColor func(string) lipgloss.Color, formatDurationShort func(time.Duration) string) string {
	if len(totals) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("No tags tracked."))
	}

	// Convert to slice and sort
	var items []TagChartItem
	var maxDuration time.Duration
	for tag, duration := range totals {
		items = append(items, TagChartItem{Tag: tag, Duration: duration})
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	// Calculate percentages
	for i := range items {
		if maxDuration > 0 {
			items[i].Percent = float64(items[i].Duration) / float64(maxDuration)
		}
	}

	// Sort by duration (descending), then alphabetically by tag
	sort.Slice(items, func(i, j int) bool {
		if items[i].Duration != items[j].Duration {
			return items[i].Duration > items[j].Duration
		}
		return items[i].Tag < items[j].Tag
	})

	// Limit to available height
	maxLines := height - 2
	if len(items) > maxLines {
		items = items[:maxLines]
	}

	var lines []string
	tagNameWidth := 20 // Increased width for tag names with # prefix
	percentWidth := 6  // Space for percentage (e.g., "100%")
	barWidth := width - tagNameWidth - percentWidth - 2 // Leave space for tag name, percentage, and padding

	for _, item := range items {
		filled := int(float64(barWidth) * item.Percent)
		if filled < 0 {
			filled = 0
		}
		if filled > barWidth {
			filled = barWidth
		}

		bar := ""
		for i := 0; i < filled; i++ {
			bar += "█"
		}

		tagColor := getTagColor(item.Tag)
		tagStyle := chartLabelStyle.Copy().Foreground(tagColor)
		// Add # prefix to match UI style
		tagNameWithPrefix := "#" + item.Tag
		tagName := tagStyle.Render(tagNameWithPrefix)
		// Truncate if needed (accounting for # prefix and ANSI codes)
		visibleWidth := lipgloss.Width(tagName)
		if visibleWidth > tagNameWidth {
			// Truncate the tag name (without #) and add back the prefix
			maxTagLen := tagNameWidth - 1 // -1 for the # character
			if maxTagLen > 0 && len(item.Tag) > maxTagLen {
				truncatedTag := item.Tag[:maxTagLen-3] + "..."
				tagName = tagStyle.Render("#" + truncatedTag)
			}
		}

		percentNum := int(item.Percent * 100)
		percentText := chartPercentStyle.Render(fmt.Sprintf("%d%%", percentNum))
		barStyled := chartBarStyle.Render(bar)

		line := lipgloss.JoinHorizontal(lipgloss.Left,
			lipgloss.NewStyle().Width(tagNameWidth).Render(tagName),
			barStyled,
			percentText,
		)
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return boxStyle.Width(width).Height(height).Render(content)
}
