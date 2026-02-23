package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ekz/claude-ctx/internal/parser"
)

// treeNode represents a node in the file tree.
type treeNode struct {
	name       string
	isDir      bool
	lines      int
	totalLines int // actual file size (0 if unknown/deleted)
	reads      int
	children   []*treeNode
}

// BuildTree constructs a tree from file read info.
func BuildTree(files []*parser.FileReadInfo, projectPath string) *treeNode {
	root := &treeNode{name: "", isDir: true}

	for _, fi := range files {
		relPath := fi.FilePath
		if projectPath != "" {
			if rel, err := filepath.Rel(projectPath, fi.FilePath); err == nil {
				relPath = rel
			}
		}

		parts := strings.Split(relPath, "/")
		insertNode(root, parts, fi.Lines, fi.TotalLines, fi.ReadCount)
	}

	sortTree(root)
	return root
}

func insertNode(parent *treeNode, parts []string, lines, totalLines, reads int) {
	if len(parts) == 0 {
		return
	}

	name := parts[0]

	var child *treeNode
	for _, c := range parent.children {
		if c.name == name {
			child = c
			break
		}
	}

	if child == nil {
		child = &treeNode{
			name:  name,
			isDir: len(parts) > 1,
		}
		parent.children = append(parent.children, child)
	}

	if len(parts) == 1 {
		child.lines = lines
		child.totalLines = totalLines
		child.reads = reads
	} else {
		insertNode(child, parts[1:], lines, totalLines, reads)
	}
}

func sortTree(node *treeNode) {
	sort.Slice(node.children, func(i, j int) bool {
		if node.children[i].isDir != node.children[j].isDir {
			return node.children[i].isDir
		}
		return node.children[i].name < node.children[j].name
	})

	for _, child := range node.children {
		if child.isDir {
			sortTree(child)
		}
	}
}

// RenderTree renders the file tree with bars as a string.
func RenderTree(root *treeNode, maxBarWidth int) string {
	if maxBarWidth <= 0 {
		maxBarWidth = 20
	}

	maxLines := findMaxLines(root)
	maxNameLen := findMaxNameLen(root, 0)

	var sb strings.Builder
	for i, child := range root.children {
		isLast := i == len(root.children)-1
		renderNode(&sb, child, "", isLast, maxLines, maxNameLen, maxBarWidth)
	}

	return sb.String()
}

var (
	totalLinesStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
)

func renderNode(sb *strings.Builder, node *treeNode, prefix string, isLast bool, maxLines, maxNameLen, maxBarWidth int) {
	connector := treeChars.Tee
	if isLast {
		connector = treeChars.Corner
	}

	if node.isDir {
		line := fmt.Sprintf("%s%s %s", prefix, connector, dirStyle.Render(node.name+"/"))
		sb.WriteString(line)
		sb.WriteString("\n")

		childPrefix := prefix
		if isLast {
			childPrefix += treeChars.Space + " "
		} else {
			childPrefix += treeChars.Pipe + "  "
		}

		for i, child := range node.children {
			childIsLast := i == len(node.children)-1
			renderNode(sb, child, childPrefix, childIsLast, maxLines, maxNameLen, maxBarWidth)
		}
	} else {
		name := fileStyle.Render(node.name)

		padding := maxNameLen - visibleLen(node.name, prefix, connector) + 2
		if padding < 2 {
			padding = 2
		}

		bar := barBlock(node.lines, maxLines, maxBarWidth)
		coloredBar := colorBar(bar, node.lines, maxLines)

		// Format: +190/563 or +190 (if file doesn't exist)
		var linesStr string
		if node.totalLines > 0 {
			readLines := node.lines
			if readLines > node.totalLines {
				readLines = node.totalLines
			}
			read := linesStyle.Render(fmt.Sprintf("+%d", readLines))
			total := totalLinesStyle.Render(fmt.Sprintf("/%d", node.totalLines))
			linesStr = read + total
		} else {
			linesStr = linesStyle.Render(fmt.Sprintf("+%d", node.lines))
		}

		line := fmt.Sprintf("%s%s %s%s%s  %s",
			prefix, connector, name,
			strings.Repeat(" ", padding),
			coloredBar, linesStr)

		sb.WriteString(line)
		sb.WriteString("\n")
	}
}

func findMaxLines(node *treeNode) int {
	max := node.lines
	for _, child := range node.children {
		if childMax := findMaxLines(child); childMax > max {
			max = childMax
		}
	}
	return max
}

func findMaxNameLen(node *treeNode, depth int) int {
	nameLen := depth*4 + len(node.name)
	if node.isDir {
		nameLen++
	}

	max := nameLen
	for _, child := range node.children {
		if childMax := findMaxNameLen(child, depth+1); childMax > max {
			max = childMax
		}
	}
	return max
}

func visibleLen(name, prefix, connector string) int {
	return len(prefix) + len(connector) + 1 + len(name)
}
