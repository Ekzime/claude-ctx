package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ekz/claude-ctx/internal/gitdiff"
)

// RenderChanges renders the git diff section with add/remove bars.
func RenderChanges(diff gitdiff.DiffResult, maxBarWidth int) string {
	var sb strings.Builder

	sb.WriteString(sectionStyle.Render("Changes"))
	sb.WriteString("\n")

	if !diff.HasGit {
		sb.WriteString(dimStyle.Render("  Git not initialized"))
		sb.WriteString("\n")
		return sb.String()
	}

	if len(diff.Files) == 0 {
		sb.WriteString(dimStyle.Render("  No changes"))
		sb.WriteString("\n")
		return sb.String()
	}

	// Sort by total changes (added + removed) descending
	files := make([]gitdiff.FileDiff, len(diff.Files))
	copy(files, diff.Files)
	sort.Slice(files, func(i, j int) bool {
		return (files[i].Added + files[i].Removed) > (files[j].Added + files[j].Removed)
	})

	// Find max for bar scaling
	maxChange := 0
	for _, f := range files {
		if f.Added > maxChange {
			maxChange = f.Added
		}
		if f.Removed > maxChange {
			maxChange = f.Removed
		}
	}

	// Find max filename length for alignment
	maxNameLen := 0
	for _, f := range files {
		if len(f.FilePath) > maxNameLen {
			maxNameLen = len(f.FilePath)
		}
	}

	addBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(addedBarColor))
	rmBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(removedBarColor))

	for _, f := range files {
		name := fileStyle.Render(f.FilePath)

		padding := maxNameLen - len(f.FilePath) + 2
		if padding < 2 {
			padding = 2
		}

		var parts []string

		// Added bar
		if f.Added > 0 {
			bar := barBlock(f.Added, maxChange, maxBarWidth)
			parts = append(parts, addBarStyle.Render(bar))
			parts = append(parts, addedStyle.Render(fmt.Sprintf("+%d", f.Added)))
		}

		// Removed bar
		if f.Removed > 0 {
			bar := barBlock(f.Removed, maxChange, maxBarWidth)
			parts = append(parts, rmBarStyle.Render(bar))
			parts = append(parts, removedStyle.Render(fmt.Sprintf("-%d", f.Removed)))
		}

		line := fmt.Sprintf("  %s%s%s",
			name,
			strings.Repeat(" ", padding),
			strings.Join(parts, "  "))

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("\n")
	totalAdded := addedStyle.Render(fmt.Sprintf("+%d", diff.TotalAdded()))
	totalRemoved := removedStyle.Render(fmt.Sprintf("-%d", diff.TotalRemoved()))
	sb.WriteString(fmt.Sprintf("  %s %s in %d files", totalAdded, totalRemoved, len(files)))
	sb.WriteString("\n")

	return sb.String()
}
