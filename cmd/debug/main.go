package main

import (
	"fmt"
	"os"

	"github.com/ekz/claude-ctx/internal/parser"
	"github.com/ekz/claude-ctx/internal/session"
	"github.com/ekz/claude-ctx/internal/ui"
)

func main() {
	sessions, err := session.ListAllSessionsWithActive()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) > 5 {
		sessions = sessions[:5]
	}
	fmt.Printf("Found %d recent sessions:\n\n", len(sessions))
	for i, s := range sessions {
		fmt.Printf("%d. %s\n   Project: %s\n   Modified: %s (%s)\n   Path: %s\n\n",
			i+1, s.DisplayName(), s.ProjectPath, s.ModifiedStr, s.TimeAgo(), s.FullPath)
	}

	if len(sessions) == 0 {
		return
	}

	// Try to find a session with actual Read calls
	var latest session.Session
	for _, s := range sessions {
		d, err := parser.ParseWithSubagents(s.FullPath)
		if err == nil && len(d.Files) > 0 {
			latest = s
			break
		}
	}
	if latest.FullPath == "" {
		// Fallback: try all sessions
		allSessions, _ := session.ListAllSessions()
		for _, s := range allSessions {
			d, err := parser.ParseWithSubagents(s.FullPath)
			if err == nil && len(d.Files) > 0 {
				latest = s
				break
			}
		}
	}
	if latest.FullPath == "" {
		fmt.Println("No sessions with Read calls found")
		return
	}
	fmt.Printf("--- Parsing session: %s ---\n\n", latest.SessionID)

	data, err := parser.ParseWithSubagents(latest.FullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	data.ResolveTotalLines()
	fmt.Printf("Model: %s\n", data.Model)
	fmt.Printf("Files read: %d\n", len(data.Files))
	fmt.Printf("Total lines: %d\n\n", data.TotalLines())

	files := data.SortedFiles()
	root := ui.BuildTree(files, latest.ProjectPath)
	tree := ui.RenderTree(root, 25)
	fmt.Println(tree)

	fmt.Printf("+%d in %d files\n", data.TotalLines(), len(data.Files))

	// Tool usage
	if len(data.Tools) > 0 {
		fmt.Printf("\nTool usage:\n")
		fmt.Println(ui.RenderTools(data.Tools, 20))
	}
}
