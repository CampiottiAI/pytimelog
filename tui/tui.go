package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"lazytime/storage"
)

// TerminalUI manages the terminal user interface.
type TerminalUI struct {
	screen        tcell.Screen
	entries       []storage.Entry
	message       string
	messageError  bool
	focusSection  string // "day" or "week"
	dayOffset     int
	weekOffset    int
	timeout       int
}

// NewTerminalUI creates a new TerminalUI instance.
func NewTerminalUI(s tcell.Screen) *TerminalUI {
	ui := &TerminalUI{
		screen:       s,
		entries:     []storage.Entry{},
		focusSection: "day",
		timeout:     1000, // 1 second
	}

	s.SetStyle(tcell.StyleDefault)
	s.EnableMouse()
	s.EnablePaste()
	s.Clear()

	// Initialize color pairs
	s.SetStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorBlack))

	ui.reloadEntries()
	return ui
}

// reloadEntries reloads entries from the log file.
func (ui *TerminalUI) reloadEntries() {
	entries, err := storage.ReadEntries("")
	if err != nil {
		ui.message = fmt.Sprintf("Error reading log: %v", err)
		ui.messageError = true
		ui.entries = []storage.Entry{}
		return
	}
	ui.entries = entries
}

// formatDuration formats duration as HH:MM.
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// clampDuration calculates overlap duration within a time range.
func clampDuration(entry storage.Entry, start, end, now time.Time) time.Duration {
	entryEnd := now
	if entry.End != nil {
		entryEnd = *entry.End
	}

	latestStart := entry.Start
	if start.After(latestStart) {
		latestStart = start
	}

	earliestEnd := entryEnd
	if end.Before(earliestEnd) {
		earliestEnd = end
	}

	if earliestEnd.Before(latestStart) || earliestEnd.Equal(latestStart) {
		return 0
	}

	return earliestEnd.Sub(latestStart)
}

// drawBox draws a box with a title at the specified position.
func (ui *TerminalUI) drawBox(y, x, height, width int, title string, highlight bool) {
	if height < 2 || width < 2 {
		return
	}

	style := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
	if !highlight {
		style = tcell.StyleDefault
	}

	// Draw corners and borders
	ul := '┌'
	ur := '┐'
	ll := '└'
	lr := '┘'
	hline := '─'
	vline := '│'

	// Top border
	ui.screen.SetContent(x, y, ul, nil, style)
	for col := x + 1; col < x+width-1; col++ {
		ui.screen.SetContent(col, y, hline, nil, style)
	}
	ui.screen.SetContent(x+width-1, y, ur, nil, style)

	// Title
	titleText := fmt.Sprintf(" %s ", title)
	if len(titleText) < width-2 {
		ui.drawString(y, x+2, titleText, style)
	}

	// Side borders
	for row := y + 1; row < y+height-1; row++ {
		ui.screen.SetContent(x, row, vline, nil, style)
		ui.screen.SetContent(x+width-1, row, vline, nil, style)
	}

	// Bottom border
	ui.screen.SetContent(x, y+height-1, ll, nil, style)
	for col := x + 1; col < x+width-1; col++ {
		ui.screen.SetContent(col, y+height-1, hline, nil, style)
	}
	ui.screen.SetContent(x+width-1, y+height-1, lr, nil, style)
}

// drawWindowBox draws a box on a sub-window (for prompts).
func (ui *TerminalUI) drawWindowBox(win tcell.Screen, height, width int, title string) {
	if height < 2 || width < 2 {
		return
	}

	style := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)

	ul := '┌'
	ur := '┐'
	ll := '└'
	lr := '┘'
	hline := '─'
	vline := '│'

	// Top border with title
	win.SetContent(0, 0, ul, nil, style)
	titleText := fmt.Sprintf(" %s ", title)
	titleLen := len(titleText)
	availableWidth := width - 2

	if titleLen < availableWidth {
		remaining := availableWidth - titleLen
		leftHlines := remaining / 2

		for col := 1; col < 1+leftHlines; col++ {
			win.SetContent(col, 0, hline, nil, style)
		}

		ui.drawStringOnScreen(win, 0, 1+leftHlines, titleText, style)

		for col := 1 + leftHlines + titleLen; col < width-1; col++ {
			win.SetContent(col, 0, hline, nil, style)
		}
	} else {
		for col := 1; col < width-1; col++ {
			win.SetContent(col, 0, hline, nil, style)
		}
	}
	win.SetContent(width-1, 0, ur, nil, style)

	// Side borders
	for row := 1; row < height-1; row++ {
		win.SetContent(0, row, vline, nil, style)
		win.SetContent(width-1, row, vline, nil, style)
	}

	// Bottom border
	win.SetContent(0, height-1, ll, nil, style)
	for col := 1; col < width-1; col++ {
		win.SetContent(col, height-1, hline, nil, style)
	}
	win.SetContent(width-1, height-1, lr, nil, style)
}

// drawString draws a string at the specified position.
func (ui *TerminalUI) drawString(y, x int, text string, style tcell.Style) {
	for i, r := range text {
		ui.screen.SetContent(x+i, y, r, nil, style)
	}
}

// drawStringOnScreen draws a string on a specific screen.
func (ui *TerminalUI) drawStringOnScreen(s tcell.Screen, y, x int, text string, style tcell.Style) {
	for i, r := range text {
		s.SetContent(x+i, y, r, nil, style)
	}
}

// addstr safely adds a string with width constraint.
func (ui *TerminalUI) addstr(y, x int, text string, style tcell.Style, width int) {
	totalWidth, height := ui.screen.Size()
	if y < 0 || y >= height {
		return
	}
	cappedWidth := width
	if x+cappedWidth > totalWidth {
		cappedWidth = totalWidth - x
	}
	if cappedWidth <= 0 {
		return
	}

	text = text[:min(len(text), cappedWidth)]
	for i, r := range text {
		if x+i < totalWidth {
			ui.screen.SetContent(x+i, y, r, nil, style)
		}
	}
	// Fill remaining space
	for i := len(text); i < cappedWidth; i++ {
		ui.screen.SetContent(x+i, y, ' ', nil, style)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// dayWindow returns the UTC time range for today.
func (ui *TerminalUI) dayWindow(now time.Time) (time.Time, time.Time) {
	tz := now.Location()
	today := now
	startLocal := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, tz)
	endLocal := startLocal.AddDate(0, 0, 1)
	return startLocal.UTC(), endLocal.UTC()
}

// weekWindow returns the UTC time range for this week (Mon-Sun).
func (ui *TerminalUI) weekWindow(now time.Time) (time.Time, time.Time) {
	tz := now.Location()
	today := now
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekday-- // Monday = 0
	weekStart := today.AddDate(0, 0, -weekday)
	startLocal := time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, tz)
	endLocal := startLocal.AddDate(0, 0, 7)
	return startLocal.UTC(), endLocal.UTC()
}

// totalForRange calculates total duration within a time range.
func (ui *TerminalUI) totalForRange(startUTC, endUTC, now time.Time) time.Duration {
	var total time.Duration
	for _, entry := range ui.entries {
		total += clampDuration(entry, startUTC, endUTC, now)
	}
	return total
}

// summarizeEntries returns entries within a range with their local times and durations.
func (ui *TerminalUI) summarizeEntries(startUTC, endUTC, now time.Time) []struct {
	startLocal time.Time
	endLocal   time.Time
	duration   time.Duration
	text       string
} {
	tz := now.Location()
	var rows []struct {
		startLocal time.Time
		endLocal   time.Time
		duration   time.Duration
		text       string
	}

	for _, entry := range ui.entries {
		if entry.End == nil {
			continue
		}
		overlap := clampDuration(entry, startUTC, endUTC, now)
		if overlap <= 0 {
			continue
		}
		localStart := entry.Start
		if startUTC.After(localStart) {
			localStart = startUTC
		}
		localStart = localStart.In(tz)

		localEnd := *entry.End
		if endUTC.Before(localEnd) {
			localEnd = endUTC
		}
		localEnd = localEnd.In(tz)

		rows = append(rows, struct {
			startLocal time.Time
			endLocal   time.Time
			duration   time.Duration
			text       string
		}{
			startLocal: localStart,
			endLocal:   localEnd,
			duration:   overlap,
			text:       entry.Text,
		})
	}

	// Sort by start time, descending
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].startLocal.After(rows[j].startLocal)
	})

	return rows
}

// topTasksForRange returns top tags by duration within a range.
func (ui *TerminalUI) topTasksForRange(startUTC, endUTC, now time.Time, limit int) []struct {
	tag      string
	duration time.Duration
} {
	totals := make(map[string]time.Duration)
	for _, entry := range ui.entries {
		duration := clampDuration(entry, startUTC, endUTC, now)
		if duration <= 0 {
			continue
		}
		tags := entry.Tags()
		if len(tags) == 0 {
			tags = []string{"(untagged)"}
		}
		for _, tag := range tags {
			totals[tag] += duration
		}
	}

	type tagItem struct {
		tag      string
		duration time.Duration
	}
	var ordered []tagItem
	for tag, duration := range totals {
		ordered = append(ordered, tagItem{tag: tag, duration: duration})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].duration > ordered[j].duration
	})

	var result []struct {
		tag      string
		duration time.Duration
	}
	for i := 0; i < limit && i < len(ordered); i++ {
		result = append(result, struct {
			tag      string
			duration time.Duration
		}{
			tag:      ordered[i].tag,
			duration: ordered[i].duration,
		})
	}
	return result
}

// allocateLeftHeights allocates height across day/week/top sections.
func (ui *TerminalUI) allocateLeftHeights(total int) (int, int, int) {
	if total <= 0 {
		return 0, 0, 0
	}
	mins := []int{3, 5, 4} // day, week, top
	baseSum := 0
	for _, m := range mins {
		baseSum += m
	}
	if total <= baseSum {
		shrunk := ui.shrinkToTotal(mins, total)
		return shrunk[0], shrunk[1], shrunk[2]
	}
	remaining := total - baseSum
	weights := []int{2, 3, 3}
	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	extraDay := remaining * weights[0] / totalWeight
	extraWeek := remaining * weights[1] / totalWeight
	extraTop := remaining - extraDay - extraWeek
	return mins[0] + extraDay, mins[1] + extraWeek, mins[2] + extraTop
}

func (ui *TerminalUI) shrinkToTotal(values []int, total int) []int {
	if total <= 0 {
		return []int{0, 0, 0}
	}
	result := make([]int, len(values))
	copy(result, values)
	for sum(result) > total {
		allOne := true
		for _, v := range result {
			if v > 1 {
				allOne = false
				break
			}
		}
		if allOne {
			break
		}
		for i := range result {
			if sum(result) <= total {
				break
			}
			if result[i] > 1 {
				result[i]--
			}
		}
	}
	return result
}

func sum(values []int) int {
	s := 0
	for _, v := range values {
		s += v
	}
	return s
}

// Draw renders the entire UI.
func (ui *TerminalUI) Draw() {
	ui.screen.Clear()
	now := storage.UTCNow()
	width, height := ui.screen.Size()
	footerHeight := 2
	contentHeight := max(0, height-footerHeight)
	leftWidth := max(int(float64(width)*0.38), 28)
	rightWidth := max(width-leftWidth-1, 20)
	statusHeight := 3

	leftRemaining := max(contentHeight-statusHeight, 0)
	dayHeight, weekHeight, topHeight := ui.allocateLeftHeights(leftRemaining)

	currentY := 0
	ui.drawStatusBox(currentY, 0, statusHeight, leftWidth, now)
	currentY += statusHeight
	ui.drawDaySummary(currentY, 0, dayHeight, leftWidth, now)
	currentY += dayHeight
	ui.drawWeekSummary(currentY, 0, weekHeight, leftWidth, now)
	currentY += weekHeight
	ui.drawTopTasks(currentY, 0, topHeight, leftWidth, now)

	rightX := leftWidth + 1
	minCurrentHeight := 6
	commentBoxHeight := min(topHeight, max(contentHeight-minCurrentHeight, 0))
	currentTaskHeight := max(contentHeight-commentBoxHeight, minCurrentHeight)

	ui.drawCurrentTaskBox(0, rightX, currentTaskHeight, rightWidth, now)
	ui.drawCommentLog(currentTaskHeight, rightX, commentBoxHeight, rightWidth, now)
	ui.drawFooter(contentHeight, width)
	ui.screen.Show()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// drawStatusBox draws the status box.
func (ui *TerminalUI) drawStatusBox(y, x, height, width int, now time.Time) {
	if height < 2 {
		return
	}
	idx := storage.FindOpen(ui.entries)
	var status string
	var statusStyle tcell.Style
	if idx == -1 {
		status = "IDLE"
		statusStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
	} else {
		status = "WORKING"
		statusStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
	}
	title := "[1]-Status"
	ui.drawBox(y, x, height, width, title, false)
	ui.addstr(y+1, x+2, status, statusStyle, width-3)
}

// drawDaySummary draws the day summary pane.
func (ui *TerminalUI) drawDaySummary(y, x, height, width int, now time.Time) {
	if height < 2 {
		return
	}
	startUTC, endUTC := ui.dayWindow(now)
	summaries := ui.summarizeEntries(startUTC, endUTC, now)
	title := "[2]-Day summary"
	highlight := ui.focusSection == "day"
	if highlight {
		title += " (scroll)"
	}
	ui.drawBox(y, x, height, width, title, highlight)
	innerWidth := width - 2
	maxLines := max(0, height-2)
	if maxLines <= 0 {
		return
	}
	clampedOffset := min(ui.dayOffset, max(0, len(summaries)-maxLines))
	ui.dayOffset = clampedOffset
	view := summaries[clampedOffset:]
	if len(view) > maxLines {
		view = view[:maxLines]
	}
	for idx, row := range view {
		line := fmt.Sprintf("%s-%s %s %s",
			row.startLocal.Format("15:04"),
			row.endLocal.Format("15:04"),
			formatDuration(row.duration),
			row.text)
		ui.addstr(y+1+idx, x+1, line, tcell.StyleDefault, innerWidth)
	}
	if len(summaries) == 0 {
		ui.addstr(y+1, x+1, "No entries yet today.", tcell.StyleDefault.Dim(true), innerWidth)
	}
}

// drawWeekSummary draws the week summary pane.
func (ui *TerminalUI) drawWeekSummary(y, x, height, width int, now time.Time) {
	if height < 2 {
		return
	}
	startUTC, endUTC := ui.weekWindow(now)
	summaries := ui.summarizeEntries(startUTC, endUTC, now)
	title := "[3]-Week summary"
	highlight := ui.focusSection == "week"
	if highlight {
		title += " (scroll)"
	}
	ui.drawBox(y, x, height, width, title, highlight)
	innerWidth := width - 2
	maxLines := max(0, height-2)
	if maxLines <= 0 {
		return
	}
	clampedOffset := min(ui.weekOffset, max(0, len(summaries)-maxLines))
	ui.weekOffset = clampedOffset
	view := summaries[clampedOffset:]
	if len(view) > maxLines {
		view = view[:maxLines]
	}
	for idx, row := range view {
		line := fmt.Sprintf("%s %s-%s %s %s",
			row.startLocal.Format("Mon 15:04"),
			row.startLocal.Format("15:04"),
			row.endLocal.Format("15:04"),
			formatDuration(row.duration),
			row.text)
		ui.addstr(y+1+idx, x+1, line, tcell.StyleDefault, innerWidth)
	}
	if len(summaries) == 0 {
		ui.addstr(y+1, x+1, "No entries this week yet.", tcell.StyleDefault.Dim(true), innerWidth)
	}
}

// drawTopTasks draws the top tags pane.
func (ui *TerminalUI) drawTopTasks(y, x, height, width int, now time.Time) {
	if height < 2 {
		return
	}
	weekStart, weekEnd := ui.weekWindow(now)
	top := ui.topTasksForRange(weekStart, weekEnd, now, 6)
	ui.drawBox(y, x, height, width, "[4]-Top tags", false)
	innerWidth := width - 2
	maxLines := max(0, height-2)
	for idx, item := range top {
		if idx >= maxLines {
			break
		}
		line := fmt.Sprintf("%s %s", formatDuration(item.duration), item.tag)
		ui.addstr(y+1+idx, x+1, line, tcell.StyleDefault, innerWidth)
	}
	if len(top) == 0 {
		ui.addstr(y+1, x+1, "No tracked time yet.", tcell.StyleDefault.Dim(true), innerWidth)
	}
}

// drawCurrentTaskBox draws the current task box.
func (ui *TerminalUI) drawCurrentTaskBox(y, x, height, width int, now time.Time) {
	if height < 2 {
		return
	}
	ui.drawBox(y, x, height, width, "[0]-Current task", false)
	innerWidth := max(0, width-2)
	innerHeight := max(0, height-2)
	idx := storage.FindOpen(ui.entries)
	tz := now.Location()
	var lines []string
	if idx == -1 {
		lines = []string{"No active task."}
		// Find last closed entry
		var closed *storage.Entry
		for i := len(ui.entries) - 1; i >= 0; i-- {
			if ui.entries[i].End != nil {
				closed = &ui.entries[i]
				break
			}
		}
		if closed != nil {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Last: %s", closed.Text))
			lines = append(lines, fmt.Sprintf("Ended: %s", closed.End.In(tz).Format("2006-01-02 15:04")))
			lines = append(lines, fmt.Sprintf("Length: %s", formatDuration(closed.Duration(*closed.End))))
		}
	} else {
		entry := ui.entries[idx]
		startLocal := entry.Start.In(tz)
		elapsed := entry.Duration(now)
		lines = []string{
			fmt.Sprintf("Task: %s", entry.Text),
			fmt.Sprintf("Start: %s", startLocal.Format("2006-01-02 15:04")),
			fmt.Sprintf("Elapsed: %s", formatDuration(elapsed)),
		}
		tags := entry.Tags()
		if len(tags) == 0 {
			lines = append(lines, "Tags: (untagged)")
		} else {
			lines = append(lines, fmt.Sprintf("Tags: %s", strings.Join(tags, ", ")))
		}
	}
	ui.renderWrapped(lines, y+1, x+1, innerHeight, innerWidth, tcell.StyleDefault)
}

// renderWrapped renders text with word wrapping.
func (ui *TerminalUI) renderWrapped(lines []string, y, x, maxHeight, maxWidth int, style tcell.Style) {
	if maxHeight <= 0 || maxWidth <= 0 {
		return
	}
	currentRow := y
	for _, line := range lines {
		words := strings.Fields(line)
		currentLine := ""
		for _, word := range words {
			testLine := currentLine
			if testLine != "" {
				testLine += " "
			}
			testLine += word
			if len(testLine) > maxWidth {
				if currentLine != "" {
					ui.addstr(currentRow, x, currentLine, style, maxWidth)
					currentRow++
					if currentRow >= y+maxHeight {
						return
					}
				}
				currentLine = word
			} else {
				currentLine = testLine
			}
		}
		if currentLine != "" {
			ui.addstr(currentRow, x, currentLine, style, maxWidth)
			currentRow++
			if currentRow >= y+maxHeight {
				return
			}
		}
	}
}

// drawCommentLog draws the comment log pane.
func (ui *TerminalUI) drawCommentLog(y, x, height, width int, now time.Time) {
	if height < 2 {
		return
	}
	ui.drawBox(y, x, height, width, "Comment log", false)
	todayStart, todayEnd := ui.dayWindow(now)
	weekStart, weekEnd := ui.weekWindow(now)
	todayTotal := ui.totalForRange(todayStart, todayEnd, now)
	weekTotal := ui.totalForRange(weekStart, weekEnd, now)
	targetToday := 8 * time.Hour
	targetWeek := 40 * time.Hour
	deltaToday := targetToday - todayTotal
	deltaWeek := targetWeek - weekTotal

	var todayStyle, weekStyle tcell.Style
	if deltaToday > 0 {
		todayStyle = tcell.StyleDefault.Foreground(tcell.ColorRed)
	} else {
		todayStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	}
	if deltaWeek > 0 {
		weekStyle = tcell.StyleDefault.Foreground(tcell.ColorRed)
	} else {
		weekStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	}

	var todayText, weekText string
	if deltaToday > 0 {
		todayText = fmt.Sprintf("Remaining today: %s to hit 08:00", formatDuration(deltaToday))
	} else {
		todayText = fmt.Sprintf("Remaining today: +%s over 08:00", formatDuration(-deltaToday))
	}
	if deltaWeek > 0 {
		weekText = fmt.Sprintf("Remaining week: %s to hit 40:00", formatDuration(deltaWeek))
	} else {
		weekText = fmt.Sprintf("Remaining week: +%s over 40:00", formatDuration(-deltaWeek))
	}

	innerWidth := width - 2
	ui.addstr(y+2, x+1, todayText, todayStyle, innerWidth)
	if height >= 3 {
		ui.addstr(y+4, x+1, weekText, weekStyle, innerWidth)
	}
	if height >= 4 {
		var messageStyle tcell.Style
		if ui.messageError {
			messageStyle = tcell.StyleDefault.Foreground(tcell.ColorRed)
		} else {
			messageStyle = tcell.StyleDefault.Foreground(tcell.ColorGreen)
		}
		ui.addstr(y+3, x+1, ui.message, messageStyle, innerWidth)
	}
}

// drawFooter draws the footer with help text.
func (ui *TerminalUI) drawFooter(y, width int) {
	helpLine := "[Tab] switch scroll target  [↑/↓] scroll  [n] start  [x] stop  [r] reload  [q] quit"
	actionLine := "Actions: start new task, stop current, reload log"
	ui.addstr(y, 0, helpLine, tcell.StyleDefault.Dim(true), width)
	ui.addstr(y+1, 0, actionLine, tcell.StyleDefault.Dim(true), width)
}

// Prompt shows a modal input dialog and returns the entered text and whether it was cancelled.
func (ui *TerminalUI) Prompt(promptText string) (string, bool) {
	width, height := ui.screen.Size()
	buffer := []rune{}
	boxWidth := max(50, min(width-4, len(promptText)+50))
	boxHeight := 5
	startY := max((height-boxHeight)/2, 0)
	startX := max((width-boxWidth)/2, 0)

	title := strings.TrimSpace(strings.TrimSuffix(promptText, ":"))
	if title == "" {
		title = "Input"
	}

	ui.screen.SetCursorStyle(tcell.CursorStyleBlinkingBlock)
	ui.screen.ShowCursor(startX+2, startY+2)

	for {
		// Draw on main screen
		ui.Draw()

		// Draw prompt box on main screen
		ui.drawBox(startY, startX, boxHeight, boxWidth, title, true)
		display := string(buffer)
		displayText := display
		if len(displayText) > boxWidth-4 {
			displayText = displayText[len(displayText)-(boxWidth-4):]
		}
		ui.addstr(startY+2, startX+2, displayText, tcell.StyleDefault, boxWidth-4)
		cursorX := startX + 2 + min(len(displayText), boxWidth-4)
		ui.screen.ShowCursor(cursorX, startY+2)
		ui.screen.Show()

		ev := ui.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				ui.screen.HideCursor()
				return "", true
			}
			if ev.Key() == tcell.KeyEnter {
				ui.screen.HideCursor()
				return strings.TrimSpace(display), false
			}
			if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 || ev.Key() == tcell.KeyDelete {
				if len(buffer) > 0 {
					buffer = buffer[:len(buffer)-1]
				}
				continue
			}
			if ev.Key() == tcell.KeyRune {
				buffer = append(buffer, ev.Rune())
			}
		}
	}
}

// notify sets a message to display.
func (ui *TerminalUI) notify(text string, error bool) {
	ui.message = text
	ui.messageError = error
}

// parseTimeOverrides extracts @HH:MM tokens from text.
// Returns cleaned text, optional start time, optional end time.
func (ui *TerminalUI) parseTimeOverrides(rawText string, now time.Time) (string, *time.Time, *time.Time, error) {
	re := regexp.MustCompile(`@(\d{1,2}:\d{2})`)
	matches := re.FindAllStringSubmatch(rawText, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(rawText), nil, nil, nil
	}
	if len(matches) > 2 {
		return "", nil, nil, fmt.Errorf("specify at most two times (start and optional end)")
	}

	tz := now.Location()
	today := now
	var parsedTimes []time.Time
	for _, match := range matches {
		timeText := match[1]
		hour, minute, err := storage.ParseTimeOfDay(timeText)
		if err != nil {
			return "", nil, nil, fmt.Errorf("invalid time: %s", timeText)
		}
		parsedTimes = append(parsedTimes, time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, tz))
	}

	// Remove @HH:MM from text
	cleaned := re.ReplaceAllString(rawText, "")
	cleaned = strings.TrimSpace(strings.Join(strings.Fields(cleaned), " "))

	startLocal := parsedTimes[0]
	var endLocal *time.Time
	if len(parsedTimes) == 2 {
		endLocal = &parsedTimes[1]
	}

	return cleaned, &startLocal, endLocal, nil
}

// startEntry handles starting a new entry.
func (ui *TerminalUI) startEntry() {
	text, cancelled := ui.Prompt("Start entry:")
	if cancelled {
		ui.notify("Start cancelled.", false)
		return
	}

	nowLocal := storage.LocalNow()
	entries, err := storage.ReadEntries("")
	if err != nil {
		ui.notify(fmt.Sprintf("Error: %v", err), true)
		return
	}

	if storage.FindOpen(entries) != -1 {
		ui.notify("An entry is already running.", true)
		return
	}

	cleanText, startOverride, endOverride, err := ui.parseTimeOverrides(text, nowLocal)
	if err != nil {
		ui.notify(err.Error(), true)
		return
	}
	if cleanText == "" {
		ui.notify("Please enter a description.", true)
		return
	}

	if endOverride != nil {
		// Adding a completed entry
		startLocal := startOverride
		if startLocal == nil {
			startLocal = &nowLocal
		}
		endLocal := endOverride

		if !endLocal.After(*startLocal) {
			ui.notify("End time must be after start time.", true)
			return
		}
		if endLocal.After(nowLocal) {
			ui.notify("End time cannot be in the future.", true)
			return
		}

		newEntry := storage.Entry{
			Start: storage.ToUTC(*startLocal),
			End:   func() *time.Time { t := storage.ToUTC(*endLocal); return &t }(),
			Text:  cleanText,
		}

		overlapEntry, overlapDuration, hasOverlap := storage.CheckOverlap(entries, newEntry, *newEntry.End)
		if hasOverlap {
			otherLocal := overlapEntry.Start.In(nowLocal.Location())
			ui.notify(
				fmt.Sprintf("Overlaps with entry at %s (%s)",
					otherLocal.Format("2006-01-02 15:04"),
					formatDuration(overlapDuration)),
				true,
			)
			return
		}

		if err := storage.AppendEntry(newEntry, ""); err != nil {
			ui.notify(fmt.Sprintf("Error: %v", err), true)
			return
		}

		ui.reloadEntries()
		ui.notify(fmt.Sprintf("Added: %s %s-%s", cleanText, startLocal.Format("15:04"), endLocal.Format("15:04")), false)
		return
	}

	// Starting a new open entry
	startLocal := startOverride
	if startLocal == nil {
		startLocal = &nowLocal
	}

	if startLocal.After(nowLocal) {
		ui.notify("Start time cannot be in the future.", true)
		return
	}

	if err := storage.AppendEntry(storage.Entry{
		Start: storage.ToUTC(*startLocal),
		End:   nil,
		Text:  cleanText,
	}, ""); err != nil {
		ui.notify(fmt.Sprintf("Error: %v", err), true)
		return
	}

	ui.reloadEntries()
	suffix := ""
	if startOverride != nil {
		suffix = fmt.Sprintf(" @ %s", startLocal.Format("15:04"))
	}
	ui.notify(fmt.Sprintf("Started: %s%s", cleanText, suffix), false)
}

// stopEntry handles stopping the active entry.
func (ui *TerminalUI) stopEntry() {
	entries, err := storage.ReadEntries("")
	if err != nil {
		ui.notify(fmt.Sprintf("Error: %v", err), true)
		return
	}

	idx := storage.FindOpen(entries)
	if idx == -1 {
		ui.notify("No active entry to stop.", true)
		return
	}

	now := storage.ToUTC(storage.LocalNow())
	openEntry := entries[idx]
	if now.Before(openEntry.Start) || now.Equal(openEntry.Start) {
		now = openEntry.Start.Add(time.Minute)
	}

	entries[idx] = storage.Entry{
		Start: openEntry.Start,
		End:   &now,
		Text:  openEntry.Text,
	}

	if err := storage.WriteEntries(entries, ""); err != nil {
		ui.notify(fmt.Sprintf("Error: %v", err), true)
		return
	}

	ui.reloadEntries()
	ui.notify(fmt.Sprintf("Stopped: %s", openEntry.Text), false)
}

// scrollActive scrolls the active pane.
func (ui *TerminalUI) scrollActive(delta int) {
	if ui.focusSection == "day" {
		ui.dayOffset = max(0, ui.dayOffset+delta)
	} else {
		ui.weekOffset = max(0, ui.weekOffset+delta)
	}
}

// Loop runs the main event loop.
func (ui *TerminalUI) Loop() {
	ui.screen.SetStyle(tcell.StyleDefault)
	ui.screen.EnableMouse()

	for {
		ui.Draw()

		ev := ui.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Rune() == 'q' {
				return
			}
			if ev.Key() == tcell.KeyUp || ev.Rune() == 'k' {
				ui.scrollActive(-1)
			} else if ev.Key() == tcell.KeyDown || ev.Rune() == 'j' {
				ui.scrollActive(1)
			} else if ev.Key() == tcell.KeyTab {
				if ui.focusSection == "day" {
					ui.focusSection = "week"
				} else {
					ui.focusSection = "day"
				}
			} else if ev.Rune() == 'n' {
				ui.startEntry()
			} else if ev.Rune() == 'x' {
				ui.stopEntry()
			} else if ev.Rune() == 'r' {
				ui.reloadEntries()
				ui.notify("Reloaded log.", false)
			}
		case *tcell.EventResize:
			ui.screen.Sync()
		}
	}
}

// LaunchTUI initializes and launches the terminal UI.
func LaunchTUI() error {
	s, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create screen: %w", err)
	}
	if err := s.Init(); err != nil {
		return fmt.Errorf("failed to initialize screen: %w", err)
	}
	defer s.Fini()

	ui := NewTerminalUI(s)
	ui.Loop()
	return nil
}

