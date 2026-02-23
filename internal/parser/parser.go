package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileReadInfo tracks how many lines were read from a file.
type FileReadInfo struct {
	FilePath   string
	Lines      int // total lines read
	ReadCount  int // how many times Read was called on this file
	TotalLines int // actual file size in lines (0 if file doesn't exist)
}

// ContextUsage tracks token usage for context indicator.
type ContextUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// TotalContext returns approximate current context size in tokens.
func (c ContextUsage) TotalContext() int {
	return c.InputTokens + c.CacheReadInputTokens + c.CacheCreationInputTokens
}

// SessionData holds parsed read data for a session.
type SessionData struct {
	SessionID    string
	Model        string
	Cwd          string
	Files        map[string]*FileReadInfo
	LastUsage    ContextUsage // usage from the most recent assistant message
	TotalOutput  int            // cumulative output tokens
}

// jsonlRecord is the top-level JSONL record.
type jsonlRecord struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Cwd     string          `json:"cwd"`
	Message json.RawMessage `json:"message"`
}

type messageEnvelope struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *usageData      `json:"usage,omitempty"`
}

type usageData struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type toolUseBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

type readInput struct {
	FilePath string `json:"file_path"`
}

// pendingRead tracks a Read tool call waiting for its result.
type pendingRead struct {
	filePath string
}

// parser holds state during JSONL parsing.
type parser struct {
	data         *SessionData
	pendingReads map[string]pendingRead // tool_use_id -> pending read info
	compacted    bool                   // true if compact_boundary was seen
}

// ParseJSONL parses a JSONL file and returns session data with all Read calls.
func ParseJSONL(path string) (*SessionData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	return ParseReader(f)
}

// ParseReader parses JSONL from a reader.
func ParseReader(r io.Reader) (*SessionData, error) {
	p := &parser{
		data: &SessionData{
			Files: make(map[string]*FileReadInfo),
		},
		pendingReads: make(map[string]pendingRead),
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		p.processLine(line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return p.data, nil
}

// IncrementalResult holds data parsed from new JSONL lines.
type IncrementalResult struct {
	Reads     []*FileReadInfo
	Usage     *ContextUsage // non-nil if usage was found in new lines
	Compacted bool          // true if a compact_boundary was encountered
	NewOffset int64
}

// ParseFromOffset reads JSONL starting at byte offset, returns incremental results.
func ParseFromOffset(path string, offset int64) (*IncrementalResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	p := &parser{
		data: &SessionData{
			Files: make(map[string]*FileReadInfo),
		},
		pendingReads: make(map[string]pendingRead),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		p.processLine(line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	newOffset, _ := f.Seek(0, io.SeekCurrent)

	var reads []*FileReadInfo
	for _, fi := range p.data.Files {
		reads = append(reads, fi)
	}

	result := &IncrementalResult{
		Reads:     reads,
		Compacted: p.compacted,
		NewOffset: newOffset,
	}

	// Pass usage if any assistant messages were parsed
	if p.data.LastUsage.TotalContext() > 0 {
		usage := p.data.LastUsage
		result.Usage = &usage
	}

	return result, nil
}

func (p *parser) processLine(line []byte) {
	var rec jsonlRecord
	if err := json.Unmarshal(line, &rec); err != nil {
		return
	}

	// Capture cwd from first record that has it
	if p.data.Cwd == "" && rec.Cwd != "" {
		p.data.Cwd = rec.Cwd
	}

	// On compaction, reset file reads — only keep post-compaction context
	if rec.Type == "system" && rec.Subtype == "compact_boundary" {
		p.data.Files = make(map[string]*FileReadInfo)
		p.pendingReads = make(map[string]pendingRead)
		p.compacted = true
		return
	}

	switch rec.Type {
	case "assistant":
		p.processAssistant(rec.Message)
	case "user":
		p.processUser(rec.Message)
	}
}

func (p *parser) processAssistant(msgRaw json.RawMessage) {
	var msg messageEnvelope
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return
	}

	if p.data.Model == "" && msg.Model != "" {
		p.data.Model = msg.Model
	}

	// Capture usage from every assistant message (last one wins)
	if msg.Usage != nil {
		p.data.LastUsage = ContextUsage{
			InputTokens:              msg.Usage.InputTokens,
			OutputTokens:             msg.Usage.OutputTokens,
			CacheCreationInputTokens: msg.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     msg.Usage.CacheReadInputTokens,
		}
		p.data.TotalOutput += msg.Usage.OutputTokens
	}

	// content is an array of blocks
	var blocks []toolUseBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return
	}

	for _, block := range blocks {
		if block.Type != "tool_use" || block.Name != "Read" {
			continue
		}

		var inp readInput
		if err := json.Unmarshal(block.Input, &inp); err != nil {
			continue
		}

		if inp.FilePath == "" || block.ID == "" {
			continue
		}

		// Store as pending, waiting for tool_result
		p.pendingReads[block.ID] = pendingRead{
			filePath: inp.FilePath,
		}
	}
}

func (p *parser) processUser(msgRaw json.RawMessage) {
	var msg messageEnvelope
	if err := json.Unmarshal(msgRaw, &msg); err != nil {
		return
	}

	// content can be a string (user text) or array (tool_result blocks)
	// Try array first
	var blocks []toolResultBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return
	}

	for _, block := range blocks {
		if block.Type != "tool_result" {
			continue
		}

		pending, ok := p.pendingReads[block.ToolUseID]
		if !ok {
			continue
		}
		delete(p.pendingReads, block.ToolUseID)

		if block.IsError {
			continue
		}

		// Count actual lines from the tool result content
		lines := countContentLines(block.Content)
		if lines == 0 {
			continue
		}

		fi, exists := p.data.Files[pending.filePath]
		if !exists {
			fi = &FileReadInfo{
				FilePath: pending.filePath,
			}
			p.data.Files[pending.filePath] = fi
		}

		fi.ReadCount++
		fi.Lines += lines
	}
}

// countContentLines counts lines in Read tool result content.
// Format: "     1\tline content\n     2\tline content\n..."
// where \t is actually a tab character (or Unicode → U+2192)
func countContentLines(content string) int {
	if content == "" {
		return 0
	}
	// Each line in the output starts with a line number prefix
	// Simply count newlines
	lines := strings.Count(content, "\n")
	// If content doesn't end with \n, add 1
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	return lines
}

// ParseWithSubagents parses main JSONL and all subagent files.
func ParseWithSubagents(jsonlPath string) (*SessionData, error) {
	data, err := ParseJSONL(jsonlPath)
	if err != nil {
		return nil, err
	}

	// Check for subagents directory
	base := strings.TrimSuffix(jsonlPath, ".jsonl")
	subagentsDir := filepath.Join(base, "subagents")

	entries, err := os.ReadDir(subagentsDir)
	if err != nil {
		return data, nil
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		subPath := filepath.Join(subagentsDir, entry.Name())
		subData, err := ParseJSONL(subPath)
		if err != nil {
			continue
		}

		for path, fi := range subData.Files {
			if existing, ok := data.Files[path]; ok {
				existing.ReadCount += fi.ReadCount
				existing.Lines += fi.Lines
			} else {
				data.Files[path] = fi
			}
		}
	}

	return data, nil
}

// SortedFiles returns files sorted by lines read (descending).
func (sd *SessionData) SortedFiles() []*FileReadInfo {
	files := make([]*FileReadInfo, 0, len(sd.Files))
	for _, fi := range sd.Files {
		files = append(files, fi)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Lines > files[j].Lines
	})
	return files
}

// TotalLines returns sum of all lines read across all files.
func (sd *SessionData) TotalLines() int {
	total := 0
	for _, fi := range sd.Files {
		total += fi.Lines
	}
	return total
}

// ResolveTotalLines counts actual file sizes for all tracked files.
func (sd *SessionData) ResolveTotalLines() {
	for _, fi := range sd.Files {
		total := countFileLines(fi.FilePath)
		if total > 0 {
			fi.TotalLines = total
		}
	}
}

func countFileLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}

// MergeReads merges new file read info into existing session data.
func (sd *SessionData) MergeReads(reads []*FileReadInfo) {
	for _, fi := range reads {
		if existing, ok := sd.Files[fi.FilePath]; ok {
			existing.ReadCount += fi.ReadCount
			existing.Lines += fi.Lines
		} else {
			sd.Files[fi.FilePath] = fi
		}
	}
}
