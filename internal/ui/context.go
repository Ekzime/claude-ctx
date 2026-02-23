package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ekz/claude-ctx/internal/parser"
)

// Model context window limits (tokens)
var contextLimits = map[string]int{
	"claude-opus-4-6":          200000,
	"claude-opus-4-5-20251101": 200000,
	"claude-sonnet-4-6":        200000,
	"claude-sonnet-4-5-20251014": 200000,
	"claude-haiku-4-5-20251001": 200000,
}

const defaultContextLimit = 200000

var (
	ctxUsedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	ctxFreeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#292e42"))
	ctxWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	ctxDangerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	ctxTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
)

func renderContextBar(data *parser.SessionData, maxWidth int) string {
	if data == nil {
		return ""
	}

	usage := data.LastUsage
	contextUsed := usage.TotalContext()
	if contextUsed == 0 {
		return ""
	}

	limit := defaultContextLimit
	if l, ok := contextLimits[data.Model]; ok {
		limit = l
	}

	if contextUsed > limit {
		contextUsed = limit
	}

	percent := float64(contextUsed) / float64(limit) * 100

	// Bar width
	barWidth := maxWidth - 25 // leave room for text
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 40 {
		barWidth = 40
	}

	// Build the bar
	filledCount := int(float64(barWidth) * float64(contextUsed) / float64(limit))
	if filledCount > barWidth {
		filledCount = barWidth
	}

	// Choose color based on usage level
	var barStyle lipgloss.Style
	switch {
	case percent > 80:
		barStyle = ctxDangerStyle
	case percent > 60:
		barStyle = ctxWarnStyle
	default:
		barStyle = ctxUsedStyle
	}

	filled := strings.Repeat("▰", filledCount)
	empty := strings.Repeat("▱", barWidth-filledCount)

	bar := barStyle.Render(filled) + ctxFreeStyle.Render(empty)

	// Format tokens as "74k/200k"
	usedStr := formatTokens(contextUsed)
	limitStr := formatTokens(limit)

	text := ctxTextStyle.Render(fmt.Sprintf("%s/%s (%.0f%%)", usedStr, limitStr, percent))

	return fmt.Sprintf("  %s %s", bar, text)
}

func formatTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}
