package tui

import (
	"lazytime/storage"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"lazytime/tui/components"
)

// renderMainView renders the main application view.
func renderMainView(m Model) string {
	width := m.width
	height := m.height
	if width < 80 {
		width = 80
	}
	if height < 24 {
		height = 24
	}

	footerHeight := 2
	contentHeight := height - footerHeight

	// Hero section (full width at top)
	heroHeight := 8
	heroSection := components.RenderHero(m.entries, m.now, width,
		BorderIdle, BorderRunning, StyleIdle, HeroTimerStyle, HeroTaskStyle, HeroTagStyle,
		GetTagColor, FormatDuration, FormatDurationShort)

	// Remaining space for main content
	mainHeight := contentHeight - heroHeight
	if mainHeight < 5 {
		mainHeight = 5
	}

	// Tabs - convert ViewMode to components.ViewMode
	var activeView components.ViewMode
	switch m.viewMode {
	case ViewToday:
		activeView = components.ViewToday
	case ViewWeek:
		activeView = components.ViewWeek
	}
	tabsSection := components.RenderTabs(activeView, width, TabActive, TabInactive)

	// Main content area (left) and sidebar (right)
	leftWidth := int(float64(width) * 0.50)
	rightWidth := width - leftWidth - 1

	// Calculate time ranges based on view mode
	var startUTC, endUTC time.Time
	tz := m.now.Location()
	today := m.now

	switch m.viewMode {
	case ViewToday:
		todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, tz)
		todayEnd := todayStart.AddDate(0, 0, 1)
		startUTC = storage.ToUTC(todayStart)
		endUTC = storage.ToUTC(todayEnd)
	case ViewWeek:
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		weekday-- // Monday = 0
		weekStart := today.AddDate(0, 0, -weekday)
		weekStartLocal := time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, tz)
		weekEndLocal := weekStartLocal.AddDate(0, 0, 7)
		startUTC = storage.ToUTC(weekStartLocal)
		endUTC = storage.ToUTC(weekEndLocal)
	}

	// Main content (tree view)
	var mainContent string
	if m.viewMode == ViewWeek {
		mainContent = renderWeekView(m.entries, startUTC, endUTC, m.now, leftWidth, mainHeight, m.scrollOffset)
	} else if m.viewMode == ViewToday {
		mainContent = renderTodayView(m.entries, startUTC, endUTC, m.now, leftWidth, mainHeight, m.scrollOffset)
	} else {
		groups := GroupByTag(m.entries, startUTC, endUTC, m.now)
		// Convert to components.TagGroup
		compGroups := make([]components.TagGroup, len(groups))
		for i, g := range groups {
			compGroups[i] = components.TagGroup{
				Tag:      g.Tag,
				Duration: g.Duration,
				Entries:  g.Entries,
				Tasks:    g.Tasks,
				TaskList: make([]components.TaskItem, len(g.TaskList)),
			}
			for j, t := range g.TaskList {
				compGroups[i].TaskList[j] = components.TaskItem{
					Text:     t.Text,
					Duration: t.Duration,
					Start:    t.Start,
					End:      t.End,
				}
			}
		}
		mainContent = components.RenderTree(compGroups, leftWidth, mainHeight, TreeTagStyle, TreeTaskStyle, TreeDurationStyle, BoxStyle, GetTagColor, FormatDurationShort)
	}

	// Sidebar: Goals and Tags
	goalsHeight := 6 // Fixed smaller height for goals box
	tagsHeight := mainHeight - goalsHeight - 1
	if goalsHeight < 3 {
		goalsHeight = 3
	}
	if tagsHeight < 3 {
		tagsHeight = 3
	}

	goalsSection := components.RenderGoalProgress(m.entries, m.now, m.targetToday, m.targetWeek, rightWidth, clampDuration, GetProgressColor, FormatDurationShort)
	goalsBox := BoxStyle.Width(rightWidth).Height(goalsHeight).Render(goalsSection)

	heatmapSection := components.RenderMonthHeatmap(m.entries, m.now, rightWidth, tagsHeight, clampDuration, BoxStyle)

	sidebar := lipgloss.JoinVertical(lipgloss.Left, goalsBox, heatmapSection)

	// Combine main content and sidebar
	contentRow := lipgloss.JoinHorizontal(lipgloss.Left, mainContent, " ", sidebar)

	// Footer
	footer := renderFooter(width)

	// Combine everything
	return lipgloss.JoinVertical(lipgloss.Left,
		heroSection,
		tabsSection,
		contentRow,
		footer,
	)
}

// renderModalView renders the modal overlay.
func renderModalView(m Model) string {
	width := m.width
	height := m.height
	if width < 80 {
		width = 80
	}
	if height < 24 {
		height = 24
	}

	// Get tag suggestions if needed
	var suggestions []string
	if m.modalType == "new" {
		// Extract current tag input from the single field
		tagInput := extractCurrentTagInput(m.modalInput)
		if tagInput != "" {
			allTags := GetUniqueTags(m.entries)
			suggestions = components.GetFuzzySuggestions(tagInput, allTags, 5)
			m.modalSuggestions = suggestions
		} else {
			suggestions = m.modalSuggestions
		}
	}

	// Render main view first (dimmed)
	mainView := renderMainView(m)
	dimmed := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render(mainView)

	// Render modal on top
	modal := components.RenderModal(m.modalType, m.modalInput, suggestions, m.modalSelected, width, height, BoxStyle, TabActive, TabInactive, FooterStyle)

	// Combine (modal should overlay)
	return lipgloss.JoinVertical(lipgloss.Left, dimmed, modal)
}

// renderTodayView renders a flat list of today's tasks sorted by completion time (most recent first).
func renderTodayView(entries []storage.Entry, startUTC, endUTC, now time.Time, width, height, scrollOffset int) string {
	// Filter entries for today
	var todayEntries []storage.Entry
	for _, entry := range entries {
		if clampDuration(entry, startUTC, endUTC, now) > 0 {
			todayEntries = append(todayEntries, entry)
		}
	}

	if len(todayEntries) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("No entries today."))
	}

	// Sort by end time (descending - most recent first)
	// For open entries, use 'now' as the end time for sorting
	sort.Slice(todayEntries, func(i, j int) bool {
		endI := now
		if todayEntries[i].End != nil {
			endI = *todayEntries[i].End
		}
		endJ := now
		if todayEntries[j].End != nil {
			endJ = *todayEntries[j].End
		}
		return endI.After(endJ)
	})

	// Convert UTC times to local timezone for display
	tz := now.Location()

	// Build all lines first (without height limit)
	var allLines []string

	for _, entry := range todayEntries {

		// Convert start/end times to local timezone
		startLocal := entry.Start.In(tz)
		endLocal := now
		if entry.End != nil {
			endLocal = entry.End.In(tz)
		}

		// Format time range
		timeRange := startLocal.Format("15:04") + " - " + endLocal.Format("15:04")

		// Extract task text without tags
		taskText := removeTags(entry.Text)

		// Extract tags
		tags := entry.Tags()

		// Build the line: "- (HH:MM - HH:MM) <task> <tag1> <tag2>"
		prefix := "- (" + timeRange + ") " + taskText

		// Render tags with colors
		var tagParts []string
		for _, tag := range tags {
			tagColor := GetTagColor(tag)
			tagStyle := lipgloss.NewStyle().Foreground(tagColor)
			tagParts = append(tagParts, tagStyle.Render("#"+tag))
		}
		tagsStr := strings.Join(tagParts, " ")

		// Calculate available width for the line
		// Account for box padding (2 chars on each side = 4 total)
		availableWidth := width - 4

		// Get visible widths (accounting for ANSI escape codes)
		prefixVisible := lipgloss.Width(prefix)
		tagsVisible := lipgloss.Width(tagsStr)

		var line string
		if len(tags) > 0 {
			if prefixVisible+tagsVisible+1 <= availableWidth {
				// Tags fit on the same line - align to right
				spacesNeeded := availableWidth - prefixVisible - tagsVisible
				line = prefix + strings.Repeat(" ", spacesNeeded) + tagsStr
			} else {
				// Tags don't fit - put them after task text with a space
				line = prefix + " " + tagsStr
			}
		} else {
			// No tags
			line = prefix
		}

		// Truncate if line exceeds available width
		if lipgloss.Width(line) > availableWidth {
			// Use lipgloss to truncate while preserving ANSI codes
			line = lipgloss.Place(availableWidth, 1, lipgloss.Left, lipgloss.Top, line)
		}

		allLines = append(allLines, line)
	}

	// Calculate visible lines and apply scroll offset
	visibleLines := height - 2
	maxScrollOffset := 0
	if len(allLines) > visibleLines {
		maxScrollOffset = len(allLines) - visibleLines
	}

	// Clamp scroll offset
	if scrollOffset > maxScrollOffset {
		scrollOffset = maxScrollOffset
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Apply scroll offset and limit to visible lines
	startIdx := scrollOffset
	endIdx := scrollOffset + visibleLines
	if endIdx > len(allLines) {
		endIdx = len(allLines)
	}

	var lines []string
	if startIdx < len(allLines) {
		lines = allLines[startIdx:endIdx]
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return BoxStyle.Width(width).Height(height).Render(content)
}

// renderWeekView renders a list of the week's tasks grouped by day of the week.
func renderWeekView(entries []storage.Entry, startUTC, endUTC, now time.Time, width, height, scrollOffset int) string {
	// Filter entries for the week
	var weekEntries []storage.Entry
	for _, entry := range entries {
		if clampDuration(entry, startUTC, endUTC, now) > 0 {
			weekEntries = append(weekEntries, entry)
		}
	}

	if len(weekEntries) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("No entries this week."))
	}

	// Convert UTC times to local timezone for grouping
	tz := now.Location()

	// Group entries by day of the week
	// Map: day index (0=Monday, 6=Sunday) -> entries for that day
	dayGroups := make(map[int][]storage.Entry)

	for _, entry := range weekEntries {
		// Determine which day this entry belongs to (use start time)
		startLocal := entry.Start.In(tz)
		dayIndex := int(startLocal.Weekday())
		if dayIndex == 0 {
			dayIndex = 7 // Sunday = 7, but we want it to be index 6
		}
		dayIndex-- // Monday = 0, Sunday = 6

		dayGroups[dayIndex] = append(dayGroups[dayIndex], entry)
	}

	// Day names in order (Monday to Sunday)
	dayNames := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

	// Sort entries within each day by end time (descending - most recent first)
	for dayIndex := range dayGroups {
		sort.Slice(dayGroups[dayIndex], func(i, j int) bool {
			endI := now
			if dayGroups[dayIndex][i].End != nil {
				endI = *dayGroups[dayIndex][i].End
			}
			endJ := now
			if dayGroups[dayIndex][j].End != nil {
				endJ = *dayGroups[dayIndex][j].End
			}
			return endI.After(endJ)
		})
	}

	// Build all lines first (without height limit)
	var allLines []string

	// Determine today's day index (0=Monday, 6=Sunday)
	todayLocal := now.In(tz)
	todayDayIndex := int(todayLocal.Weekday())
	if todayDayIndex == 0 {
		todayDayIndex = 7 // Sunday = 7
	}
	todayDayIndex-- // Monday = 0, Sunday = 6

	// Create descending day order starting from today going backwards
	// If today is Friday (4), order is: 4, 3, 2, 1, 0, 6, 5
	var dayOrder []int
	for i := 0; i < 7; i++ {
		dayIndex := (todayDayIndex - i + 7) % 7
		dayOrder = append(dayOrder, dayIndex)
	}

	for _, dayIndex := range dayOrder {
		dayEntries, hasEntries := dayGroups[dayIndex]
		if !hasEntries || len(dayEntries) == 0 {
			continue // Skip days with no entries
		}

		// Day header: "> monday" (styled like tree headers)
		dayHeader := "> " + TreeTagStyle.Render(dayNames[dayIndex])
		allLines = append(allLines, dayHeader)

		// Add tasks for this day
		for _, entry := range dayEntries {

			// Convert start/end times to local timezone
			startLocal := entry.Start.In(tz)
			endLocal := now
			if entry.End != nil {
				endLocal = entry.End.In(tz)
			}

			// Format time range
			timeRange := startLocal.Format("15:04") + " - " + endLocal.Format("15:04")

			// Extract task text without tags
			taskText := removeTags(entry.Text)

			// Extract tags
			tags := entry.Tags()

			// Build the line: "- (HH:MM - HH:MM) <task> <tag1> <tag2>"
			prefix := "- (" + timeRange + ") " + taskText

			// Render tags with colors
			var tagParts []string
			for _, tag := range tags {
				tagColor := GetTagColor(tag)
				tagStyle := lipgloss.NewStyle().Foreground(tagColor)
				tagParts = append(tagParts, tagStyle.Render("#"+tag))
			}
			tagsStr := strings.Join(tagParts, " ")

			// Calculate available width for the line
			// Account for box padding (2 chars on each side = 4 total)
			availableWidth := width - 4

			// Get visible widths (accounting for ANSI escape codes)
			prefixVisible := lipgloss.Width(prefix)
			tagsVisible := lipgloss.Width(tagsStr)

			var line string
			if len(tags) > 0 {
				if prefixVisible+tagsVisible+1 <= availableWidth {
					// Tags fit on the same line - align to right
					spacesNeeded := availableWidth - prefixVisible - tagsVisible
					line = prefix + strings.Repeat(" ", spacesNeeded) + tagsStr
				} else {
					// Tags don't fit - put them after task text with a space
					line = prefix + " " + tagsStr
				}
			} else {
				// No tags
				line = prefix
			}

			// Truncate if line exceeds available width
			if lipgloss.Width(line) > availableWidth {
				// Use lipgloss to truncate while preserving ANSI codes
				line = lipgloss.Place(availableWidth, 1, lipgloss.Left, lipgloss.Top, line)
			}

			allLines = append(allLines, line)
		}
	}

	if len(allLines) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("No entries this week."))
	}

	// Calculate visible lines and apply scroll offset
	visibleLines := height - 2
	maxScrollOffset := 0
	if len(allLines) > visibleLines {
		maxScrollOffset = len(allLines) - visibleLines
	}

	// Clamp scroll offset
	if scrollOffset > maxScrollOffset {
		scrollOffset = maxScrollOffset
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Apply scroll offset and limit to visible lines
	startIdx := scrollOffset
	endIdx := scrollOffset + visibleLines
	if endIdx > len(allLines) {
		endIdx = len(allLines)
	}

	var lines []string
	if startIdx < len(allLines) {
		lines = allLines[startIdx:endIdx]
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return BoxStyle.Width(width).Height(height).Render(content)
}

// renderFooter renders the footer with help text.
func renderFooter(width int) string {
	helpLine := "[1/2] Views  [n] New  [x] Stop  [r] Reload  [e/?] Help  [q] Quit"
	return FooterStyle.Width(width).Render(helpLine)
}
