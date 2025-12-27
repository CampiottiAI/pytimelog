package tui

import (
	"lazytime/storage"
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
	case ViewMonth:
		activeView = components.ViewMonth
	}
	tabsSection := components.RenderTabs(activeView, width, TabActive, TabInactive)

	// Main content area (left) and sidebar (right)
	leftWidth := int(float64(width) * 0.65)
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
	case ViewMonth:
		monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, tz)
		monthEnd := monthStart.AddDate(0, 1, 0)
		startUTC = storage.ToUTC(monthStart)
		endUTC = storage.ToUTC(monthEnd)
	}

	// Main content (tree view or heatmap)
	var mainContent string
	if m.viewMode == ViewMonth {
		mainContent = components.RenderMonthHeatmap(m.entries, m.now, leftWidth, mainHeight, clampDuration, BoxStyle)
	} else if m.viewMode == ViewWeek {
		// For week view, show both tree and heatmap
		treeHeight := mainHeight / 2
		heatmapHeight := mainHeight - treeHeight - 1
		if treeHeight < 3 {
			treeHeight = 3
		}
		if heatmapHeight < 3 {
			heatmapHeight = 3
		}
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
		treeView := components.RenderTree(compGroups, leftWidth, treeHeight, TreeTagStyle, TreeTaskStyle, TreeDurationStyle, BoxStyle, GetTagColor, FormatDurationShort)
		heatmapView := components.RenderWeekHeatmap(m.entries, m.now, leftWidth, heatmapHeight, clampDuration, BoxStyle)
		mainContent = lipgloss.JoinVertical(lipgloss.Left, treeView, heatmapView)
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
	goalsHeight := mainHeight / 2
	tagsHeight := mainHeight - goalsHeight - 1
	if goalsHeight < 3 {
		goalsHeight = 3
	}
	if tagsHeight < 3 {
		tagsHeight = 3
	}

	goalsSection := components.RenderGoalProgress(m.entries, m.now, m.targetToday, m.targetWeek, rightWidth, clampDuration, GetProgressColor, FormatDurationShort)
	goalsBox := BoxStyle.Width(rightWidth).Height(goalsHeight).Render(goalsSection)

	tagTotals := CalculateTagTotals(m.entries, startUTC, endUTC, m.now)
	tagsSection := components.RenderTagChart(tagTotals, rightWidth, tagsHeight, ChartBarStyle, ChartLabelStyle, ChartPercentStyle, BoxStyle, GetTagColor, FormatDurationShort)

	sidebar := lipgloss.JoinVertical(lipgloss.Left, goalsBox, tagsSection)

	// Combine main content and sidebar
	contentRow := lipgloss.JoinHorizontal(lipgloss.Left, mainContent, " ", sidebar)

	// Footer
	footer := renderFooter(width)

	// Message (if any)
	var messageLine string
	if m.message != "" {
		msgStyle := SuccessStyle
		if m.messageError {
			msgStyle = ErrorStyle
		}
		messageLine = msgStyle.Render(m.message)
		if len(messageLine) > width {
			messageLine = messageLine[:width]
		}
		messageLine = lipgloss.Place(width, 1, lipgloss.Center, lipgloss.Top, messageLine)
	}

	// Combine everything
	return lipgloss.JoinVertical(lipgloss.Left,
		heroSection,
		tabsSection,
		contentRow,
		messageLine,
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

// renderFooter renders the footer with help text.
func renderFooter(width int) string {
	helpLine := "[1/2/3] Views  [n] New  [x] Stop  [r] Reload  [e/?] Help  [q] Quit"
	return FooterStyle.Width(width).Render(helpLine)
}
