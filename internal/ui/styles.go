package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Header styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7aa2f7"))

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89"))

	// Tree styles
	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7dcfff")).
			Bold(true)

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5"))

	// Bar colors - 3 levels: low, medium, high
	barColorLow  = "#7aa2f7" // blue - small files
	barColorMid  = "#9ece6a" // green - medium files
	barColorHigh = "#ff9e64" // orange - large files

	linesStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a"))

	// Session picker styles
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7aa2f7")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a9b1d6"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89"))

	// Git diff styles
	addedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a")) // green

	removedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f7768e")) // red

	addedBarColor = "#9ece6a"
	removedBarColor = "#f7768e"

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#bb9af7")) // purple

	// Tree branch chars
	treeChars = struct {
		Pipe   string
		Tee    string
		Corner string
		Dash   string
		Space  string
	}{
		Pipe:   "│",
		Tee:    "├──",
		Corner: "└──",
		Dash:   "─",
		Space:  "   ",
	}
)

// barBlock returns a horizontal bar using Unicode block characters.
func barBlock(value, maxValue, maxWidth int) string {
	if maxValue == 0 || value == 0 {
		return ""
	}

	// Scale value to bar width
	width := float64(value) / float64(maxValue) * float64(maxWidth)
	if width < 1 {
		width = 1
	}

	fullBlocks := int(width)
	fraction := width - float64(fullBlocks)

	blocks := []rune("▏▎▍▌▋▊▉█")

	var bar string
	for i := 0; i < fullBlocks; i++ {
		bar += string(blocks[7]) // full block █
	}

	if fraction > 0.125 && fullBlocks < maxWidth {
		idx := int(fraction * 8)
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		bar += string(blocks[idx])
	}

	return bar
}

// colorBar applies 3-level color to bar based on relative size.
func colorBar(bar string, value, maxValue int) string {
	if bar == "" {
		return ""
	}

	ratio := float64(value) / float64(maxValue)

	var color string
	switch {
	case ratio > 0.66:
		color = barColorHigh // orange - large
	case ratio > 0.33:
		color = barColorMid // green - medium
	default:
		color = barColorLow // blue - small
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return style.Render(bar)
}
