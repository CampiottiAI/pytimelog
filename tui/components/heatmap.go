package components

import (
	"lazytime/storage"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RenderWeekHeatmap renders a calendar heatmap for the week (7 days).
func RenderWeekHeatmap(entries []storage.Entry, now time.Time, width, height int, clampDuration func(storage.Entry, time.Time, time.Time, time.Time) time.Duration, boxStyle lipgloss.Style) string {
	tz := now.Location()
	today := now

	// Calculate week start (Monday)
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekday-- // Monday = 0
	weekStart := today.AddDate(0, 0, -weekday)

	// Calculate daily totals
	dailyTotals := make([]time.Duration, 7)
	var maxDuration time.Duration

	for i := 0; i < 7; i++ {
		dayStart := weekStart.AddDate(0, 0, i)
		dayStartLocal := time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, tz)
		dayEndLocal := dayStartLocal.AddDate(0, 0, 1)
		dayStartUTC := storage.ToUTC(dayStartLocal)
		dayEndUTC := storage.ToUTC(dayEndLocal)

		var total time.Duration
		for _, entry := range entries {
			total += clampDuration(entry, dayStartUTC, dayEndUTC, now)
		}
		dailyTotals[i] = total
		if total > maxDuration {
			maxDuration = total
		}
	}

	// Render squares
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var lines []string

	// Header
	header := "Week Heatmap"
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(header))
	lines = append(lines, "")

	// Squares row
	var squares []string
	for i, total := range dailyTotals {
		intensity := 0.0
		if maxDuration > 0 {
			intensity = float64(total) / float64(maxDuration)
		}

		// Choose color based on intensity
		var color lipgloss.Color
		if intensity == 0 {
			color = lipgloss.Color("#333333")
		} else if intensity < 0.25 {
			color = lipgloss.Color("#005500")
		} else if intensity < 0.5 {
			color = lipgloss.Color("#00aa00")
		} else if intensity < 0.75 {
			color = lipgloss.Color("#00ff00")
		} else {
			color = lipgloss.Color("#88ff88")
		}

		square := lipgloss.NewStyle().
			Background(color).
			Foreground(color).
			Width(2).
			Height(1).
			Render("██")

		squares = append(squares, square)
		if i < len(dayNames) {
			// Add day name below
			dayName := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(dayNames[i])
			squares = append(squares, "\n"+dayName)
		}
	}

	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, squares...))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return boxStyle.Width(width).Height(height).Render(content)
}

// RenderMonthHeatmap renders a calendar heatmap for the month.
func RenderMonthHeatmap(entries []storage.Entry, now time.Time, width, height int, clampDuration func(storage.Entry, time.Time, time.Time, time.Time) time.Duration, boxStyle lipgloss.Style) string {
	// Simplified month view - show last 30 days
	tz := now.Location()
	today := now

	// Calculate daily totals for last 30 days
	dailyTotals := make([]time.Duration, 30)
	var maxDuration time.Duration

	for i := 0; i < 30; i++ {
		dayStart := today.AddDate(0, 0, -29+i)
		dayStartLocal := time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, tz)
		dayEndLocal := dayStartLocal.AddDate(0, 0, 1)
		dayStartUTC := storage.ToUTC(dayStartLocal)
		dayEndUTC := storage.ToUTC(dayEndLocal)

		var total time.Duration
		for _, entry := range entries {
			total += clampDuration(entry, dayStartUTC, dayEndUTC, now)
		}
		dailyTotals[i] = total
		if total > maxDuration {
			maxDuration = total
		}
	}

	// Calculate available space (accounting for box padding: 1 top/bottom, 2 left/right)
	boxPaddingH := 2 * 2 // left + right padding
	boxPaddingV := 1 * 2 // top + bottom padding
	headerHeight := 2     // "Last 30 Days" + empty line
	spacing := 1          // spacing between squares

	availableWidth := width - boxPaddingH
	availableHeight := height - boxPaddingV - headerHeight

	// Calculate optimal grid layout
	// Try different column counts to maximize square size
	const numDays = 30
	bestCols := 6
	bestSquareWidth := 2
	bestSquareHeight := 2

	// Try column counts from 5 to 10
	for cols := 5; cols <= 10; cols++ {
		rows := (numDays + cols - 1) / cols // ceil division

		// Calculate square dimensions that fit
		// Available width: (cols * squareWidth) + ((cols - 1) * spacing) <= availableWidth
		// squareWidth <= (availableWidth - (cols-1)*spacing) / cols
		squareWidth := (availableWidth - (cols-1)*spacing) / cols
		if squareWidth < 2 {
			squareWidth = 2
		}

		// Available height: (rows * squareHeight) + ((rows - 1) * spacing) <= availableHeight
		// squareHeight <= (availableHeight - (rows-1)*spacing) / rows
		squareHeight := (availableHeight - (rows-1)*spacing) / rows
		if squareHeight < 2 {
			squareHeight = 2
		}

		// Check if this layout fits
		neededWidth := (cols * squareWidth) + ((cols - 1) * spacing)
		neededHeight := (rows * squareHeight) + ((rows - 1) * spacing)

		if neededWidth <= availableWidth && neededHeight <= availableHeight {
			// Prefer larger squares
			if squareWidth*squareHeight > bestSquareWidth*bestSquareHeight {
				bestCols = cols
				bestSquareWidth = squareWidth
				bestSquareHeight = squareHeight
			}
		}
	}

	// Ensure minimum size
	if bestSquareWidth < 2 {
		bestSquareWidth = 2
	}
	if bestSquareHeight < 2 {
		bestSquareHeight = 2
	}

	rows := (numDays + bestCols - 1) / bestCols

	// Render header
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Last 30 Days"))
	lines = append(lines, "")

	// Render grid with calculated dimensions
	for row := 0; row < rows; row++ {
		var squareRows []string // Multiple lines per row of squares
		for lineInRow := 0; lineInRow < bestSquareHeight; lineInRow++ {
			var squares []string
			for col := 0; col < bestCols; col++ {
				idx := row*bestCols + col
				if idx >= numDays {
					// Empty space for incomplete last row
					emptySquare := lipgloss.NewStyle().
						Width(bestSquareWidth).
						Height(1).
						Render(strings.Repeat(" ", bestSquareWidth))
					squares = append(squares, emptySquare)
					if col < bestCols-1 {
						squares = append(squares, strings.Repeat(" ", spacing))
					}
					continue
				}

				total := dailyTotals[idx]
				intensity := 0.0
				if maxDuration > 0 {
					intensity = float64(total) / float64(maxDuration)
				}

				var color lipgloss.Color
				if intensity == 0 {
					color = lipgloss.Color("#333333")
				} else if intensity < 0.25 {
					color = lipgloss.Color("#005500")
				} else if intensity < 0.5 {
					color = lipgloss.Color("#00aa00")
				} else if intensity < 0.75 {
					color = lipgloss.Color("#00ff00")
				} else {
					color = lipgloss.Color("#88ff88")
				}

				// Create square with proper width
				squareContent := strings.Repeat("█", bestSquareWidth)
				square := lipgloss.NewStyle().
					Background(color).
					Foreground(color).
					Width(bestSquareWidth).
					Height(1).
					Render(squareContent)

				squares = append(squares, square)
				if col < bestCols-1 {
					squares = append(squares, strings.Repeat(" ", spacing))
				}
			}
			squareRows = append(squareRows, lipgloss.JoinHorizontal(lipgloss.Left, squares...))
		}
		// Add all lines for this row of squares
		lines = append(lines, squareRows...)
		// Add spacing between rows (except after last row)
		if row < rows-1 {
			lines = append(lines, "")
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return boxStyle.Width(width).Height(height).Render(content)
}
