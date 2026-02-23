package ui

import (
	"fmt"
	"sort"
	"strings"
)

type toolEntry struct {
	Name  string
	Count int
}

// RenderTools renders tool usage statistics with bars.
func RenderTools(tools map[string]int, maxBarWidth int) string {
	var sb strings.Builder

	entries := make([]toolEntry, 0, len(tools))
	totalCalls := 0
	for name, count := range tools {
		entries = append(entries, toolEntry{Name: name, Count: count})
		totalCalls += count
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})

	stats := fmt.Sprintf("  %d tools · %s total calls",
		len(entries), formatNumber(totalCalls))
	sb.WriteString(subtitleStyle.Render(stats))
	sb.WriteString("\n\n")

	// Find max for bar scaling
	maxCount := 0
	if len(entries) > 0 {
		maxCount = entries[0].Count
	}

	// Find max name length for alignment
	maxNameLen := 0
	for _, e := range entries {
		if len(e.Name) > maxNameLen {
			maxNameLen = len(e.Name)
		}
	}

	for _, e := range entries {
		padding := maxNameLen - len(e.Name) + 2
		bar := barBlock(e.Count, maxCount, maxBarWidth)
		coloredBar := colorBar(bar, e.Count, maxCount)

		line := fmt.Sprintf("  %s%s%s %s",
			fileStyle.Render(e.Name),
			strings.Repeat(" ", padding),
			coloredBar,
			linesStyle.Render(fmt.Sprintf("%d", e.Count)))

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}
