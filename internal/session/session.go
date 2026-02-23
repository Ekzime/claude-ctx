package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Session represents a Claude Code session.
type Session struct {
	SessionID   string    `json:"sessionId"`
	FullPath    string    `json:"fullPath"`
	FirstPrompt string    `json:"firstPrompt"`
	Summary     string    `json:"summary"`
	MsgCount    int       `json:"messageCount"`
	Created     time.Time `json:"-"`
	Modified    time.Time `json:"-"`
	GitBranch   string    `json:"gitBranch"`
	ProjectPath string    `json:"projectPath"`
	CreatedStr  string    `json:"created"`
	ModifiedStr string    `json:"modified"`
}

type sessionsIndex struct {
	Version      int             `json:"version"`
	OriginalPath string          `json:"originalPath"`
	Entries      []sessionEntry  `json:"entries"`
}

type sessionEntry struct {
	SessionID   string `json:"sessionId"`
	FullPath    string `json:"fullPath"`
	FirstPrompt string `json:"firstPrompt"`
	Summary     string `json:"summary"`
	MsgCount    int    `json:"messageCount"`
	Created     string `json:"created"`
	Modified    string `json:"modified"`
	GitBranch   string `json:"gitBranch"`
	ProjectPath string `json:"projectPath"`
}

// ClaudeDir returns the path to ~/.claude/projects/.
func ClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

// ListAllSessions scans all projects and returns all sessions, sorted by modified time (newest first).
func ListAllSessions() ([]Session, error) {
	projectsDir := ClaudeDir()

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var sessions []Session

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		indexPath := filepath.Join(projectsDir, entry.Name(), "sessions-index.json")
		ss, err := parseSessionsIndex(indexPath)
		if err != nil {
			continue
		}
		sessions = append(sessions, ss...)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})

	return sessions, nil
}

// ListRecentSessions returns the N most recent sessions.
func ListRecentSessions(n int) ([]Session, error) {
	all, err := ListAllSessions()
	if err != nil {
		return nil, err
	}
	if len(all) < n {
		return all, nil
	}
	return all[:n], nil
}

func parseSessionsIndex(path string) ([]Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var idx sessionsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	var sessions []Session
	for _, e := range idx.Entries {
		s := Session{
			SessionID:   e.SessionID,
			FullPath:    e.FullPath,
			FirstPrompt: e.FirstPrompt,
			Summary:     e.Summary,
			MsgCount:    e.MsgCount,
			GitBranch:   e.GitBranch,
			ProjectPath: e.ProjectPath,
			CreatedStr:  e.Created,
			ModifiedStr: e.Modified,
		}

		if t, err := time.Parse(time.RFC3339, e.Created); err == nil {
			s.Created = t
		}
		if t, err := time.Parse(time.RFC3339, e.Modified); err == nil {
			s.Modified = t
		}

		// Verify JSONL file exists
		if _, err := os.Stat(s.FullPath); err != nil {
			continue
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// FindActiveSessions finds JSONL files that are being written to recently (likely active sessions).
func FindActiveSessions() ([]Session, error) {
	projectsDir := ClaudeDir()
	var sessions []Session

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for _, projEntry := range entries {
		if !projEntry.IsDir() {
			continue
		}

		projDir := filepath.Join(projectsDir, projEntry.Name())
		files, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}

		// Read original path from sessions-index if available
		originalPath := ""
		idxPath := filepath.Join(projDir, "sessions-index.json")
		if data, err := os.ReadFile(idxPath); err == nil {
			var idx sessionsIndex
			if json.Unmarshal(data, &idx) == nil {
				originalPath = idx.OriginalPath
			}
		}

		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}

			info, err := f.Info()
			if err != nil {
				continue
			}

			// Consider "active" if modified within last 2 hours
			if now.Sub(info.ModTime()) > 2*time.Hour {
				continue
			}

			sessionID := f.Name()[:len(f.Name())-6] // strip .jsonl

			s := Session{
				SessionID:   sessionID,
				FullPath:    filepath.Join(projDir, f.Name()),
				FirstPrompt: "(active session)",
				ProjectPath: originalPath,
				Modified:    info.ModTime(),
			}
			sessions = append(sessions, s)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})

	return sessions, nil
}

// ListAllSessionsWithActive includes both indexed and active sessions.
func ListAllSessionsWithActive() ([]Session, error) {
	indexed, err := ListAllSessions()
	if err != nil {
		indexed = nil
	}

	active, _ := FindActiveSessions()

	// Merge: active first, then indexed (dedup by SessionID)
	seen := make(map[string]bool)
	var result []Session

	for _, s := range active {
		if !seen[s.SessionID] {
			seen[s.SessionID] = true
			result = append(result, s)
		}
	}

	for _, s := range indexed {
		if !seen[s.SessionID] {
			seen[s.SessionID] = true
			result = append(result, s)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Modified.After(result[j].Modified)
	})

	return result, nil
}

// DisplayName returns a short display string for session picker.
func (s Session) DisplayName() string {
	name := s.FirstPrompt
	if s.Summary != "" {
		name = s.Summary
	}
	if len(name) > 60 {
		name = name[:57] + "..."
	}
	return name
}

// TimeAgo returns a human-readable relative time.
func (s Session) TimeAgo() string {
	d := time.Since(s.Modified)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}
