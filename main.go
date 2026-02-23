package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ekz/claude-ctx/internal/session"
	"github.com/ekz/claude-ctx/internal/ui"
)

func main() {
	sessionID := flag.String("session", "", "session UUID to open directly")
	latest := flag.Bool("latest", false, "open the most recent session")
	flag.Parse()

	if *latest {
		runLatest()
		return
	}

	if *sessionID != "" {
		runSession(*sessionID)
		return
	}

	runPicker()
}

func runPicker() {
	sessions, err := session.ListAllSessionsWithActive()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No Claude Code sessions found in ~/.claude/projects/")
		os.Exit(1)
	}

	if len(sessions) > 50 {
		sessions = sessions[:50]
	}
	m := ui.NewModel(sessions)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runLatest() {
	sessions, err := session.ListAllSessionsWithActive()
	if err != nil || len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions found")
		os.Exit(1)
	}

	runWithSession(sessions[0])
}

func runSession(id string) {
	sessions, err := session.ListAllSessionsWithActive()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, s := range sessions {
		if s.SessionID == id {
			runWithSession(s)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Session %s not found\n", id)
	os.Exit(1)
}

func runWithSession(s session.Session) {
	m := ui.NewModelWithSession(s)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
