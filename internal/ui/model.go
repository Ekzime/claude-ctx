package ui

import (
	"fmt"
	"strings"
	"time"

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
	viewTabs
)

type tabID int

const (
	tabContext tabID = iota
	tabChanges
	tabTools
	tabCount // total number of tabs
)

// Model is the main bubbletea model.
type Model struct {
	state    viewState
	sessions []session.Session
	cursor   int
	scroll   int

	// Tab view
	activeTab   tabID
	tabScroll   [3]int // scroll offset per tab
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
		state:    viewTabs,
		selected: s,
	}
}

func (m *Model) Init() tea.Cmd {
	if m.state == viewTabs {
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
		// Load diff if on changes tab
		if m.activeTab == tabChanges {
			return m, m.loadDiff()
		}
		return m, nil

	case diffLoadedMsg:
		m.diffResult = &msg.diff
		if m.activeTab == tabChanges {
			return m, m.scheduleDiffRefresh()
		}
		return m, nil

	case diffTickMsg:
		if m.state == viewTabs && m.activeTab == tabChanges {
			return m, m.loadDiff()
		}
		return m, nil

	case watcher.FileChanged:
		if m.sessionData != nil {
			m.sessionData.MergeReads(msg.NewReads)
			m.sessionData.MergeTools(msg.NewTools)
			m.sessionData.ResolveTotalLines()
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.stopWatcher()
		return m, tea.Quit

	case "esc":
		if m.state == viewTabs {
			m.stopWatcher()
			m.state = viewPicker
			m.sessionData = nil
			m.diffResult = nil
			return m, nil
		}
		m.stopWatcher()
		return m, tea.Quit

	case "tab", "right", "l":
		if m.state == viewTabs {
			return m.switchTab(1)
		}

	case "shift+tab", "left", "h":
		if m.state == viewTabs {
			return m.switchTab(-1)
		}

	case "up", "k":
		if m.state == viewPicker {
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scroll {
					m.scroll = m.cursor
				}
			}
		} else if m.state == viewTabs {
			if m.tabScroll[m.activeTab] > 0 {
				m.tabScroll[m.activeTab]--
			}
		}

	case "down", "j":
		if m.state == viewPicker {
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				maxVisible := m.maxVisibleSessions()
				if m.cursor >= m.scroll+maxVisible {
					m.scroll = m.cursor - maxVisible + 1
				}
			}
		} else if m.state == viewTabs {
			m.tabScroll[m.activeTab]++
			// Clamping happens in render
		}

	case "enter":
		if m.state == viewPicker && len(m.sessions) > 0 {
			m.selected = m.sessions[m.cursor]
			m.state = viewTabs
			m.activeTab = tabContext
			return m, m.loadSession()
		}
	}

	return m, nil
}

func (m *Model) switchTab(dir int) (tea.Model, tea.Cmd) {
	prevTab := m.activeTab

	next := int(m.activeTab) + dir
	if next < 0 {
		next = int(tabCount) - 1
	} else if next >= int(tabCount) {
		next = 0
	}
	m.activeTab = tabID(next)

	if m.activeTab == prevTab {
		return m, nil
	}

	// Start polling when switching to Changes
	if m.activeTab == tabChanges {
		return m, m.loadDiff()
	}

	return m, nil
}

func (m *Model) View() string {
	switch m.state {
	case viewPicker:
		return m.viewPicker()
	case viewTabs:
		return m.viewTabbed()
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

func (m *Model) viewTabbed() string {
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

	// === Fixed header ===
	model := m.sessionData.Model
	if model == "" {
		model = "unknown"
	}

	header := titleStyle.Render("claude-ctx") + "  " +
		subtitleStyle.Render(model)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Context bar
	contextBar := renderContextBar(m.sessionData, m.width-4)
	if contextBar != "" {
		sb.WriteString(contextBar)
		sb.WriteString("\n")
	}

	// Tab bar
	sb.WriteString(m.renderTabBar())
	sb.WriteString("\n")
	sb.WriteString(separator(m.width - 4))
	sb.WriteString("\n")

	// === Scrollable body ===
	headerLines := 5 // title + context + tabs + separator + footer
	footerLines := 2
	bodyHeight := m.height - headerLines - footerLines
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	var bodyContent string
	switch m.activeTab {
	case tabContext:
		bodyContent = m.renderContextTab()
	case tabChanges:
		bodyContent = m.renderChangesTab()
	case tabTools:
		bodyContent = m.renderToolsTab()
	}

	// Apply scroll
	lines := strings.Split(bodyContent, "\n")
	totalLines := len(lines)

	// Clamp scroll
	maxScroll := totalLines - bodyHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.tabScroll[m.activeTab] > maxScroll {
		m.tabScroll[m.activeTab] = maxScroll
	}

	scrollOff := m.tabScroll[m.activeTab]
	end := scrollOff + bodyHeight
	if end > totalLines {
		end = totalLines
	}

	visible := lines[scrollOff:end]
	sb.WriteString(strings.Join(visible, "\n"))
	sb.WriteString("\n")

	// Scroll indicator
	if totalLines > bodyHeight {
		remaining := totalLines - end
		if remaining > 0 {
			sb.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more lines", remaining)))
		} else {
			sb.WriteString(dimStyle.Render("  ── end ──"))
		}
		sb.WriteString("\n")
	}

	// === Fixed footer ===
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  tab: switch · j/k: scroll · esc: back · q: quit"))

	return sb.String()
}

func (m *Model) renderTabBar() string {
	tabs := []struct {
		id   tabID
		name string
	}{
		{tabContext, "Context"},
		{tabChanges, "Changes"},
		{tabTools, "Tools"},
	}

	var parts []string
	for _, t := range tabs {
		if t.id == m.activeTab {
			style := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7aa2f7")).
				Background(lipgloss.Color("#292e42")).
				Padding(0, 1)
			parts = append(parts, style.Render(t.name))
		} else {
			parts = append(parts, dimStyle.Render("  "+t.name+"  "))
		}
	}

	hint := dimStyle.Render("(tab to cycle)")
	return "  " + strings.Join(parts, " ") + "  " + hint
}

func (m *Model) renderContextTab() string {
	var sb strings.Builder

	fileCount := len(m.sessionData.Files)
	totalLines := m.sessionData.TotalLines()

	stats := fmt.Sprintf("  %d files · %s lines read",
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

		barWidth := 15
		if m.width > 100 {
			barWidth = 25
		}

		tree := RenderTree(root, barWidth)
		sb.WriteString(tree)
	}

	return sb.String()
}

func (m *Model) renderToolsTab() string {
	if m.sessionData == nil || len(m.sessionData.Tools) == 0 {
		return dimStyle.Render("  No tool calls yet.")
	}

	barWidth := 15
	if m.width > 100 {
		barWidth = 25
	}

	return RenderTools(m.sessionData.Tools, barWidth)
}

func (m *Model) renderChangesTab() string {
	if m.diffResult == nil {
		return dimStyle.Render("  Loading git diff...")
	}

	barWidth := 12
	if m.width > 100 {
		barWidth = 18
	}

	return RenderChanges(*m.diffResult, barWidth)
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

// diffTickMsg triggers periodic git diff refresh.
type diffTickMsg struct{}

func (m *Model) scheduleDiffRefresh() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return diffTickMsg{}
	})
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
