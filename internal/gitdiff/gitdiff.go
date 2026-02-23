package gitdiff

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FileDiff represents changes to a single file from git diff.
type FileDiff struct {
	FilePath string
	Added    int
	Removed  int
}

// DiffResult holds the result of git diff --numstat.
type DiffResult struct {
	Files   []FileDiff
	HasGit  bool
	Error   string
}

// GetDiff runs git diff --numstat in the given directory and returns results.
func GetDiff(projectPath string) DiffResult {
	if projectPath == "" {
		return DiffResult{HasGit: false, Error: "No project path"}
	}

	// Check if directory is a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = projectPath
	if _, err := cmd.Output(); err != nil {
		return DiffResult{HasGit: false, Error: "Git not initialized"}
	}

	// Get staged + unstaged changes
	result := DiffResult{HasGit: true}

	// Unstaged changes
	unstaged := runNumstat(projectPath, "git", "diff", "--numstat")
	// Staged changes
	staged := runNumstat(projectPath, "git", "diff", "--cached", "--numstat")

	// Merge staged and unstaged
	merged := make(map[string]*FileDiff)

	for _, f := range unstaged {
		merged[f.FilePath] = &FileDiff{
			FilePath: f.FilePath,
			Added:    f.Added,
			Removed:  f.Removed,
		}
	}

	for _, f := range staged {
		if existing, ok := merged[f.FilePath]; ok {
			existing.Added += f.Added
			existing.Removed += f.Removed
		} else {
			merged[f.FilePath] = &FileDiff{
				FilePath: f.FilePath,
				Added:    f.Added,
				Removed:  f.Removed,
			}
		}
	}

	// Also get untracked files
	untracked := getUntrackedFiles(projectPath)
	for _, f := range untracked {
		if _, ok := merged[f.FilePath]; !ok {
			merged[f.FilePath] = &f
		}
	}

	for _, f := range merged {
		if f.Added > 0 || f.Removed > 0 {
			result.Files = append(result.Files, *f)
		}
	}

	return result
}

func runNumstat(dir string, args ...string) []FileDiff {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseNumstat(string(out))
}

func parseNumstat(output string) []FileDiff {
	var files []FileDiff

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Binary files show "-" for added/removed
		if parts[0] == "-" || parts[1] == "-" {
			continue
		}

		added, _ := strconv.Atoi(parts[0])
		removed, _ := strconv.Atoi(parts[1])
		path := parts[2]

		// Handle renames: "old => new"
		if len(parts) > 3 {
			path = strings.Join(parts[2:], " ")
		}

		files = append(files, FileDiff{
			FilePath: path,
			Added:    added,
			Removed:  removed,
		})
	}

	return files
}

func getUntrackedFiles(dir string) []FileDiff {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var files []FileDiff
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}

		// Count lines in untracked file
		lines := countLines(filepath.Join(dir, line))
		if lines > 0 {
			files = append(files, FileDiff{
				FilePath: line,
				Added:    lines,
				Removed:  0,
			})
		}
	}

	return files
}

func countLines(path string) int {
	cmd := exec.Command("wc", "-l", path)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(parts[0])
	return n
}

// TotalAdded returns sum of all added lines.
func (r DiffResult) TotalAdded() int {
	total := 0
	for _, f := range r.Files {
		total += f.Added
	}
	return total
}

// TotalRemoved returns sum of all removed lines.
func (r DiffResult) TotalRemoved() int {
	total := 0
	for _, f := range r.Files {
		total += f.Removed
	}
	return total
}
