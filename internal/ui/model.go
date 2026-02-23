package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ekz/claude-ctx/internal/gitdiff"
	"github.com/ekz/claude-ctx/internal/parser"
	"github.com/ekz/claude-ctx/internal/session"
	"github.com/ekz/claude-ctx/internal/watcher"
)

type viewState int

const (
	viewPicker viewState = iota
	viewTree
)

// Model is the main bubbletea model.
type Model struct {
	state    viewState
	sessions []session.Session
	cursor   int
	scroll   int

	sessionData *parser.SessionData
	diffResult  *gitdiff.DiffResult
	selected    session.Session
	watch       *watcher.Watcher
	program     *tea.Program

	width  int
	height int
	err    error
}

// SetProgram stores the program reference for watcher integration.
// Must be called before p.Run().
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// NewModel creates the initial model with session picker.
func NewModel(sessions []session.Session) *Model {
	return &Model{
		state:    viewPicker,
		sessions: sessions,
	}
}

// NewModelWithSession creates a model that skips the picker.
func NewModelWithSession(s session.Session) *Model {
	return &Model{
		state:    viewTree,
		selected: s,
	}
}

func (m *Model) Init() tea.Cmd {
	if m.state == viewTree {
		return m.loadSession()
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionLoadedMsg:
		m.sessionData = msg.data
		m.err = msg.err
		if msg.err == nil {
			m.sessionData.ResolveTotalLines()
			if m.program != nil {
				m.startWatcher()
			}
		}
		return m, m.loadDiff()

	case diffLoadedMsg:
		m.diffResult = &msg.diff
		return m, nil

	case watcher.FileChanged:
		if m.sessionData != nil {
			m.sessionData.MergeReads(msg.NewReads)
			m.sessionData.ResolveTotalLines()
		}
		// Also refresh git diff
		return m, m.loadDiff()
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.stopWatcher()
		return m, tea.Quit

	case "esc":
		if m.state == viewTree {
			m.stopWatcher()
			m.state = viewPicker
			m.sessionData = nil
			m.diffResult = nil
			return m, nil
		}
		m.stopWatcher()
		return m, tea.Quit

	case "up", "k":
		if m.state == viewPicker && m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scroll {
				m.scroll = m.cursor
			}
		}

	case "down", "j":
		if m.state == viewPicker && m.cursor < len(m.sessions)-1 {
			m.cursor++
			maxVisible := m.maxVisibleSessions()
			if m.cursor >= m.scroll+maxVisible {
				m.scroll = m.cursor - maxVisible + 1
			}
		}

	case "enter":
		if m.state == viewPicker && len(m.sessions) > 0 {
			m.selected = m.sessions[m.cursor]
			m.state = viewTree
			return m, m.loadSession()
		}
	}

	return m, nil
}

func (m *Model) View() string {
	switch m.state {
	case viewPicker:
		return m.viewPicker()
	case viewTree:
		return m.viewTree()
	}
	return ""
}

func (m *Model) viewPicker() string {
	var sb strings.Builder

	header := titleStyle.Render("claude-ctx") + "  " + subtitleStyle.Render("Select a session")
	sb.WriteString(header)
	sb.WriteString("\n\n")

	if len(m.sessions) == 0 {
		sb.WriteString(dimStyle.Render("  No sessions found"))
		sb.WriteString("\n")
	}

	maxVisible := m.maxVisibleSessions()
	end := m.scroll + maxVisible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	for i := m.scroll; i < end; i++ {
		s := m.sessions[i]
		cursor := "  "
		style := normalStyle

		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		name := s.DisplayName()
		timeAgo := dimStyle.Render(s.TimeAgo())
		project := dimStyle.Render(truncatePath(s.ProjectPath, 30))

		line := fmt.Sprintf("%s%s  %s  %s", cursor, style.Render(name), timeAgo, project)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(m.sessions) > maxVisible {
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d sessions (↑↓ to scroll)", m.cursor+1, len(m.sessions))))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  enter: select · esc/q: quit"))

	return sb.String()
}

func (m *Model) viewTree() string {
	var sb strings.Builder

	if m.err != nil {
		sb.WriteString(titleStyle.Render("claude-ctx"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("  Error: %v", m.err))
		sb.WriteString("\n\n")
		sb.WriteString(dimStyle.Render("  esc: back · q: quit"))
		return sb.String()
	}

	if m.sessionData == nil {
		sb.WriteString(titleStyle.Render("claude-ctx"))
		sb.WriteString("\n\n")
		sb.WriteString(dimStyle.Render("  Loading..."))
		return sb.String()
	}

	// Header
	model := m.sessionData.Model
	if model == "" {
		model = "unknown"
	}

	fileCount := len(m.sessionData.Files)
	totalLines := m.sessionData.TotalLines()

	header := titleStyle.Render("claude-ctx") + "  " +
		dimStyle.Render("·") + "  " +
		subtitleStyle.Render(model)
	sb.WriteString(header)
	sb.WriteString("\n")

	stats := fmt.Sprintf("Files read: %d · Lines: %s",
		fileCount, formatNumber(totalLines))
	sb.WriteString(subtitleStyle.Render(stats))
	sb.WriteString("\n\n")

	if fileCount == 0 {
		sb.WriteString(dimStyle.Render("  No files read yet. Waiting..."))
		sb.WriteString("\n")
	} else {
		files := m.sessionData.SortedFiles()
		projectPath := m.selected.ProjectPath
		if projectPath == "" && m.sessionData.Cwd != "" {
			projectPath = m.sessionData.Cwd
		}
		root := BuildTree(files, projectPath)

		barWidth := 20
		if m.width > 100 {
			barWidth = 30
		}

		tree := RenderTree(root, barWidth)
		sb.WriteString(tree)

		sb.WriteString("\n")
		summary := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Render(
			fmt.Sprintf("+%s in %d files", formatNumber(totalLines), fileCount))
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	// Git changes section
	sb.WriteString("\n")
	if m.diffResult != nil {
		barWidth := 15
		if m.width > 100 {
			barWidth = 20
		}
		sb.WriteString(RenderChanges(*m.diffResult, barWidth))
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  esc: back · q: quit"))

	return sb.String()
}

func (m *Model) startWatcher() {
	if m.selected.FullPath == "" || m.program == nil {
		return
	}
	p := m.program
	m.watch = watcher.New(m.selected.FullPath, func(fc watcher.FileChanged) {
		p.Send(fc)
	})
	m.watch.Start()
}

func (m *Model) stopWatcher() {
	if m.watch != nil {
		m.watch.Stop()
		m.watch = nil
	}
}

func (m *Model) maxVisibleSessions() int {
	if m.height <= 0 {
		return 20
	}
	visible := m.height - 6
	if visible < 5 {
		visible = 5
	}
	return visible
}

// sessionLoadedMsg is sent when session JSONL is parsed.
type sessionLoadedMsg struct {
	data *parser.SessionData
	err  error
}

// diffLoadedMsg is sent when git diff is computed.
type diffLoadedMsg struct {
	diff gitdiff.DiffResult
}

func (m *Model) loadSession() tea.Cmd {
	path := m.selected.FullPath
	return func() tea.Msg {
		data, err := parser.ParseWithSubagents(path)
		return sessionLoadedMsg{data: data, err: err}
	}
}

func (m *Model) loadDiff() tea.Cmd {
	projectPath := m.selected.ProjectPath
	// Fallback to cwd from JSONL if projectPath is empty
	if projectPath == "" && m.sessionData != nil && m.sessionData.Cwd != "" {
		projectPath = m.sessionData.Cwd
	}
	return func() tea.Msg {
		diff := gitdiff.GetDiff(projectPath)
		return diffLoadedMsg{diff: diff}
	}
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
