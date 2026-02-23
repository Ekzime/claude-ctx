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
			Foreground(lipgloss.Color("#7dcfff"))

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a9b1d6"))

	// Tree connectors - dim so they don't compete with content
	connectorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3b4261"))

	// Bar colors - 3 levels: low, medium, high
	barColorLow  = "#3d59a1" // dim blue - small files
	barColorMid  = "#449dab" // teal - medium files
	barColorHigh = "#ff9e64" // orange - large files

	linesStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#73daca"))

	totalLinesStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3b4261"))

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
			Foreground(lipgloss.Color("#9ece6a"))

	removedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f7768e"))

	addedBarColor   = "#9ece6a"
	removedBarColor = "#f7768e"

	sectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89"))

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#292e42"))

	// Tree branch chars
	treeChars = struct {
		Pipe   string
		Tee    string
		Corner string
		Dash   string
		Space  string
	}{
		Pipe:   "│",
		Tee:    "├─",
		Corner: "└─",
		Dash:   "─",
		Space:  "  ",
	}
)

// barBlock returns a horizontal bar using Unicode block characters.
func barBlock(value, maxValue, maxWidth int) string {
	if maxValue == 0 || value == 0 {
		return ""
	}

	width := float64(value) / float64(maxValue) * float64(maxWidth)
	if width < 0.5 {
		width = 0.5
	}

	fullBlocks := int(width)
	fraction := width - float64(fullBlocks)

	blocks := []rune("▏▎▍▌▋▊▉█")

	var bar string
	for i := 0; i < fullBlocks; i++ {
		bar += string(blocks[7])
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
		color = barColorHigh
	case ratio > 0.33:
		color = barColorMid
	default:
		color = barColorLow
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	return style.Render(bar)
}

// separator returns a horizontal line.
func separator(width int) string {
	if width <= 0 {
		width = 40
	}
	s := ""
	for i := 0; i < width; i++ {
		s += "─"
	}
	return separatorStyle.Render(s)
}
