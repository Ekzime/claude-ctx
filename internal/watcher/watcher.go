package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/ekz/claude-ctx/internal/parser"
)

// FileChanged is sent when the JSONL file is updated with new data.
type FileChanged struct {
	NewReads  []*parser.FileReadInfo
	Usage     *parser.ContextUsage
	Compacted bool // context was compacted, reset file list
}

// Watcher monitors a JSONL session file for changes.
type Watcher struct {
	jsonlPath   string
	offset      int64
	stopCh      chan struct{}
	sendFn      func(FileChanged)
	subagentDir string
}

// New creates a new watcher for the given JSONL path.
func New(jsonlPath string, sendFn func(FileChanged)) *Watcher {
	base := strings.TrimSuffix(jsonlPath, ".jsonl")
	return &Watcher{
		jsonlPath:   jsonlPath,
		stopCh:      make(chan struct{}),
		sendFn:      sendFn,
		subagentDir: filepath.Join(base, "subagents"),
	}
}

// Start begins watching the JSONL file. Call Stop() to end.
func (w *Watcher) Start() {
	// Get initial offset (end of file)
	if info, err := os.Stat(w.jsonlPath); err == nil {
		w.offset = info.Size()
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher: fsnotify error: %v", err)
		return
	}

	// Watch the directory (more reliable than watching the file directly)
	dir := filepath.Dir(w.jsonlPath)
	if err := fsw.Add(dir); err != nil {
		log.Printf("watcher: add dir error: %v", err)
		fsw.Close()
		return
	}

	// Also watch subagents dir if it exists
	if _, err := os.Stat(w.subagentDir); err == nil {
		fsw.Add(w.subagentDir)
	}

	go w.loop(fsw)
}

func (w *Watcher) loop(fsw *fsnotify.Watcher) {
	defer fsw.Close()

	// Debounce: don't re-parse more often than every 500ms
	var debounceTimer *time.Timer

	for {
		select {
		case <-w.stopCh:
			return

		case event, ok := <-fsw.Events:
			if !ok {
				return
			}

			// Only care about writes to our JSONL file(s)
			if !event.Has(fsnotify.Write) {
				continue
			}

			isMainFile := event.Name == w.jsonlPath
			isSubagent := strings.HasPrefix(event.Name, w.subagentDir) &&
				strings.HasSuffix(event.Name, ".jsonl")

			if !isMainFile && !isSubagent {
				continue
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				w.checkForUpdates()
			})

		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: error: %v", err)
		}
	}
}

func (w *Watcher) checkForUpdates() {
	result, err := parser.ParseFromOffset(w.jsonlPath, w.offset)
	if err != nil {
		return
	}

	w.offset = result.NewOffset

	if len(result.Reads) > 0 || result.Usage != nil || result.Compacted {
		w.sendFn(FileChanged{
			NewReads:  result.Reads,
			Usage:     result.Usage,
			Compacted: result.Compacted,
		})
	}
}

// Stop ends the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
}
