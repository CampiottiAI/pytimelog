package tui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Color palette for tags (16 colors that work well in terminals)
var tagColorPalette = []string{
	"#00ff00", // green
	"#00ffff", // cyan
	"#ff00ff", // magenta
	"#ffff00", // yellow
	"#ff8800", // orange
	"#0088ff", // blue
	"#ff0088", // pink
	"#88ff00", // lime
	"#00ff88", // spring green
	"#8800ff", // purple
	"#ff8800", // orange
	"#0088ff", // light blue
	"#ff0088", // hot pink
	"#88ff00", // chartreuse
	"#00ff88", // aquamarine
	"#8800ff", // violet
}

// Style definitions
var (
	// Status colors
	StyleRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true)
	StyleIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	StylePaused  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffff00"))

	// Border styles
	BorderRunning = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00ff00")).
			Padding(1, 2)
	BorderIdle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#888888")).
			Padding(1, 2)
	BorderPaused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ffff00")).
			Padding(1, 2)

	// Hero section
	HeroTimerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00ff00"))
	HeroTaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff"))
	HeroTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	// Progress bars
	ProgressOnTrack = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	ProgressBehind  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00"))
	ProgressAhead   = lipgloss.NewStyle().Foreground(lipgloss.Color("#0088ff"))

	// Tabs
	TabActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#0088ff")).
			Padding(0, 2).
			Bold(true)
	TabInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 2)

	// Tree view
	TreeTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true)
	TreeTaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cccccc"))
	TreeDurationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Charts
	ChartBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0088ff"))
	ChartLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff"))
	ChartPercentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1, 2)

	// Footer
	FooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	// Error/Success messages
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00"))
)

// GetTagColor returns a consistent color for a tag.
func GetTagColor(tag string) lipgloss.Color {
	hash := fnv.New32a()
	hash.Write([]byte(strings.ToLower(tag)))
	index := hash.Sum32() % uint32(len(tagColorPalette))
	return lipgloss.Color(tagColorPalette[index])
}

// GetProgressColor returns the appropriate color for progress status.
func GetProgressColor(current, target time.Duration) lipgloss.Style {
	if current >= target {
		return ProgressAhead
	}
	percent := float64(current) / float64(target)
	if percent >= 0.8 {
		return ProgressOnTrack
	}
	return ProgressBehind
}

// FormatDuration formats duration as HH:MM:SS or HH:MM.
func FormatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		if seconds > 0 {
			return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		}
		return fmt.Sprintf("%02d:%02d", hours, minutes)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// FormatDurationShort formats duration as compact string (e.g., "4h 30m").
func FormatDurationShort(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", totalSeconds)
}

