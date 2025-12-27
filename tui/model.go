package tui

import (
	"fmt"
	"lazytime/storage"
	"lazytime/tui/components"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ViewMode represents the active view (Today, Week, Month).
type ViewMode int

const (
	ViewToday ViewMode = iota
	ViewWeek
)

// Model represents the application state.
type Model struct {
	entries      []storage.Entry
	now          time.Time
	viewMode     ViewMode
	message      string
	messageError bool
	messageTimer *time.Timer

	// Modal state
	showModal        bool
	modalType        string // "new" or "help"
	modalInput       string
	modalSuggestions []string
	modalSelected    int

	// Hero section
	activeEntryIndex int

	// Goals (hardcoded for now)
	targetToday time.Duration
	targetWeek  time.Duration

	// Window size
	width  int
	height int

	// Scroll state
	scrollOffset int
}

// NewModel creates a new model instance.
func NewModel() *Model {
	m := &Model{
		viewMode:         ViewToday,
		targetToday:      8 * time.Hour,
		targetWeek:       40 * time.Hour,
		activeEntryIndex: -1,
		width:            120,
		height:           40,
		scrollOffset:     0,
	}
	m.reloadEntries()
	return m
}

// reloadEntries reloads entries from storage.
func (m *Model) reloadEntries() error {
	entries, err := storage.ReadEntries("")
	if err != nil {
		m.message = "Error reading log: " + err.Error()
		m.messageError = true
		m.entries = []storage.Entry{}
		return err
	}
	m.entries = entries
	m.now = storage.UTCNow()

	// Find active entry
	m.activeEntryIndex = storage.FindOpen(m.entries)
	return nil
}

// Init initializes the model (required by Bubbletea).
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		loadEntriesCmd(),
	)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showModal {
			return m.handleModalKey(msg)
		}

		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "1":
			m.viewMode = ViewToday
			m.scrollOffset = 0 // Reset scroll when switching views
		case "2":
			m.viewMode = ViewWeek
			m.scrollOffset = 0 // Reset scroll when switching views
		case "up", "k":
			// Scroll up (only in Today or Week view)
			if m.viewMode == ViewToday || m.viewMode == ViewWeek {
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			}
			return m, nil
		case "down", "j":
			// Scroll down (only in Today or Week view)
			if m.viewMode == ViewToday || m.viewMode == ViewWeek {
				m.scrollOffset++
				// Will be clamped in render functions based on actual content
			}
			return m, nil
		case "n":
			m.showModal = true
			m.modalType = "new"
			m.modalInput = ""
			m.modalSuggestions = []string{}
			m.modalSelected = 0
		case "x":
			return m, m.stopEntry()
		case "r":
			return m, loadEntriesCmd()
		case "e", "?":
			m.showModal = true
			m.modalType = "help"
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		m.now = storage.UTCNow()
		m.activeEntryIndex = storage.FindOpen(m.entries)
		return m, tickCmd()
	case entriesLoadedMsg:
		m.entries = msg.entries
		m.now = storage.UTCNow()
		m.activeEntryIndex = storage.FindOpen(m.entries)
	case entryStoppedMsg:
		if msg.err != nil {
			m.message = "Error: " + msg.err.Error()
			m.messageError = true
		} else {
			m.message = "Stopped: " + msg.text
			m.messageError = false
		}
		m.setMessageTimer()
		return m, loadEntriesCmd()
	case entryStartedMsg:
		if msg.err != nil {
			m.message = "Error: " + msg.err.Error()
			m.messageError = true
		} else {
			m.message = "Started: " + msg.text
			m.messageError = false
		}
		m.setMessageTimer()
		// Close modal and reset state
		m.showModal = false
		m.modalInput = ""
		m.modalSuggestions = []string{}
		m.modalSelected = 0
		return m, loadEntriesCmd()
	}

	return m, tea.Batch(cmds...)
}

// setMessageTimer sets a timer to clear the message after 3 seconds.
func (m *Model) setMessageTimer() {
	if m.messageTimer != nil {
		m.messageTimer.Stop()
	}
	m.messageTimer = time.NewTimer(3 * time.Second)
	go func() {
		<-m.messageTimer.C
		m.message = ""
	}()
}

// Messages for Bubbletea
type tickMsg time.Time
type entriesLoadedMsg struct {
	entries []storage.Entry
}
type entryStoppedMsg struct {
	text string
	err  error
}
type entryStartedMsg struct {
	text string
	err  error
}

// Commands
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadEntriesCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := storage.ReadEntries("")
		if err != nil {
			return entriesLoadedMsg{entries: []storage.Entry{}}
		}
		return entriesLoadedMsg{entries: entries}
	}
}

func (m *Model) stopEntry() tea.Cmd {
	return func() tea.Msg {
		entries, err := storage.ReadEntries("")
		if err != nil {
			return entryStoppedMsg{err: err}
		}

		idx := storage.FindOpen(entries)
		if idx == -1 {
			return entryStoppedMsg{err: &noActiveEntryError{}}
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
			return entryStoppedMsg{err: err}
		}

		return entryStoppedMsg{text: openEntry.Text}
	}
}

type noActiveEntryError struct{}

func (e *noActiveEntryError) Error() string {
	return "no active entry to stop"
}

// View renders the UI (required by Bubbletea).
func (m Model) View() string {
	if m.showModal {
		return renderModalView(m)
	}
	return renderMainView(m)
}

// handleModalKey handles keyboard input when modal is shown.
func (m Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	// Handle e/q specially for help modal only
	if (key == "e" || key == "q") && m.modalType == "help" {
		m.showModal = false
		return m, nil
	}
	
	switch key {
	case "esc":
		m.showModal = false
		m.modalInput = ""
		m.modalSuggestions = []string{}
		return m, nil
	case "enter":
		if m.modalType == "new" {
			return m, m.startEntry()
		} else if m.modalType == "help" {
			m.showModal = false
			return m, nil
		}
	case "up":
		if len(m.modalSuggestions) > 0 {
			m.modalSelected = max(0, m.modalSelected-1)
		}
		return m, nil
	case "down":
		if len(m.modalSuggestions) > 0 {
			m.modalSelected = min(len(m.modalSuggestions)-1, m.modalSelected+1)
		}
		return m, nil
	default:
		// Handle text input (including e/q for new entry modal)
		if m.modalType == "new" {
			if msg.Type == tea.KeyRunes {
				m.modalInput += string(msg.Runes)
				// Update tag suggestions if user is typing a tag
				tagInput := extractCurrentTagInput(m.modalInput)
				if tagInput != "" {
					allTags := GetUniqueTags(m.entries)
					m.modalSuggestions = components.GetFuzzySuggestions(tagInput, allTags, 5)
				} else {
					m.modalSuggestions = []string{}
				}
			} else if msg.Type == tea.KeySpace {
				m.modalInput += " "
				// Update tag suggestions if user is typing a tag
				tagInput := extractCurrentTagInput(m.modalInput)
				if tagInput != "" {
					allTags := GetUniqueTags(m.entries)
					m.modalSuggestions = components.GetFuzzySuggestions(tagInput, allTags, 5)
				} else {
					m.modalSuggestions = []string{}
				}
			} else if msg.Type == tea.KeyBackspace {
				if len(m.modalInput) > 0 {
					m.modalInput = m.modalInput[:len(m.modalInput)-1]
					// Update tag suggestions if user is typing a tag
					tagInput := extractCurrentTagInput(m.modalInput)
					if tagInput != "" {
						allTags := GetUniqueTags(m.entries)
						m.modalSuggestions = components.GetFuzzySuggestions(tagInput, allTags, 5)
					} else {
						m.modalSuggestions = []string{}
					}
				}
			}
		}
	}
	return m, nil
}

// extractCurrentTagInput extracts the current tag being typed (text after the last #).
func extractCurrentTagInput(input string) string {
	lastHash := strings.LastIndex(input, "#")
	if lastHash == -1 {
		return ""
	}
	// Get text after the last #
	tagPart := input[lastHash+1:]
	// Extract only the current word (up to space or end)
	words := strings.Fields(tagPart)
	if len(words) > 0 {
		return words[0]
	}
	return tagPart
}

// startEntry starts a new entry (command).
func (m *Model) startEntry() tea.Cmd {
	return func() tea.Msg {
		text := m.modalInput
		if text == "" {
			return entryStartedMsg{err: &emptyTextError{}}
		}

		nowLocal := storage.LocalNow()
		entries, err := storage.ReadEntries("")
		if err != nil {
			return entryStartedMsg{err: err}
		}

		if storage.FindOpen(entries) != -1 {
			return entryStartedMsg{err: &entryAlreadyRunningError{}}
		}

		// Parse time overrides (@HH:MM)
		cleanText, startOverride, endOverride, err := parseTimeOverrides(text, nowLocal)
		if err != nil {
			return entryStartedMsg{err: err}
		}
		if cleanText == "" {
			return entryStartedMsg{err: &emptyTextError{}}
		}

		if endOverride != nil {
			// Adding a completed entry
			startLocal := startOverride
			if startLocal == nil {
				startLocal = &nowLocal
			}
			endLocal := endOverride

			if !endLocal.After(*startLocal) {
				return entryStartedMsg{err: &invalidTimeError{msg: "end time must be after start time"}}
			}
			if endLocal.After(nowLocal) {
				return entryStartedMsg{err: &invalidTimeError{msg: "end time cannot be in the future"}}
			}

			newEntry := storage.Entry{
				Start: storage.ToUTC(*startLocal),
				End:   func() *time.Time { t := storage.ToUTC(*endLocal); return &t }(),
				Text:  cleanText,
			}

			overlapEntry, overlapDuration, hasOverlap := storage.CheckOverlap(entries, newEntry, *newEntry.End)
			if hasOverlap {
				return entryStartedMsg{err: &overlapError{
					entry:    overlapEntry,
					duration: overlapDuration,
				}}
			}

			if err := storage.AppendEntry(newEntry, ""); err != nil {
				return entryStartedMsg{err: err}
			}

			return entryStartedMsg{text: cleanText}
		}

		// Starting a new open entry
		startLocal := startOverride
		if startLocal == nil {
			startLocal = &nowLocal
		}

		if startLocal.After(nowLocal) {
			return entryStartedMsg{err: &invalidTimeError{msg: "start time cannot be in the future"}}
		}

		if err := storage.AppendEntry(storage.Entry{
			Start: storage.ToUTC(*startLocal),
			End:   nil,
			Text:  cleanText,
		}, ""); err != nil {
			return entryStartedMsg{err: err}
		}

		return entryStartedMsg{text: cleanText}
	}
}

// Error types
type emptyTextError struct{}

func (e *emptyTextError) Error() string {
	return "please enter a description"
}

type entryAlreadyRunningError struct{}

func (e *entryAlreadyRunningError) Error() string {
	return "an entry is already running"
}

type invalidTimeError struct {
	msg string
}

func (e *invalidTimeError) Error() string {
	return e.msg
}

type overlapError struct {
	entry    storage.Entry
	duration time.Duration
}

func (e *overlapError) Error() string {
	return "entry overlaps with existing entry"
}

// parseTimeOverrides extracts @HH:MM tokens from text.
// Returns cleaned text, optional start time, optional end time.
func parseTimeOverrides(rawText string, now time.Time) (string, *time.Time, *time.Time, error) {
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
