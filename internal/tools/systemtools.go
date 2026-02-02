// Package tools provides advanced file and system tools for DevOrch CLI
package tools

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SystemTools provides system integration tools like bash, glob, grep
type SystemTools struct {
	workingDir string
}

// NewSystemTools creates a new system tools instance
func NewSystemTools() *SystemTools {
	wd, _ := os.Getwd()
	return &SystemTools{
		workingDir: wd,
	}
}

// BashCommand executes a shell command and returns output
func (st *SystemTools) BashCommand(command string) (string, error) {
	fmt.Printf("🔧 Executing: %s\n", command)

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = st.workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	result := string(output)
	fmt.Printf("✅ Command completed successfully\n")
	if len(result) > 0 {
		fmt.Printf("Output:\n%s\n", result)
	}

	return result, nil
}

// GlobSearch finds files matching a glob pattern
func (st *SystemTools) GlobSearch(pattern string) ([]string, error) {
	fmt.Printf("🔍 Searching for pattern: %s\n", pattern)

	// Handle recursive patterns with **
	if strings.Contains(pattern, "**") {
		return st.recursiveGlob(pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern error: %w", err)
	}

	fmt.Printf("✅ Found %d matches\n", len(matches))
	for _, match := range matches {
		fmt.Printf("  📄 %s\n", match)
	}

	return matches, nil
}

// recursiveGlob handles recursive glob patterns with **
func (st *SystemTools) recursiveGlob(pattern string) ([]string, error) {
	var matches []string

	// Split pattern at **
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ** pattern: %s", pattern)
	}

	baseDir := parts[0]
	if baseDir == "" {
		baseDir = "."
	}

	suffix := parts[1]
	if strings.HasPrefix(suffix, "/") {
		suffix = suffix[1:]
	}

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if suffix == "" {
			matches = append(matches, path)
			return nil
		}

		matched, _ := filepath.Match(suffix, filepath.Base(path))
		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}

// GrepSearch searches for text patterns in files
func (st *SystemTools) GrepSearch(pattern string, files []string, options GrepOptions) (*GrepResult, error) {
	fmt.Printf("🔍 Searching for pattern: %s\n", pattern)

	result := &GrepResult{
		Pattern: pattern,
		Files:   make(map[string][]GrepMatch),
		Options: options,
	}

	var regex *regexp.Regexp
	var err error

	if options.UseRegex {
		flags := ""
		if options.IgnoreCase {
			flags += "(?i)"
		}
		regex, err = regexp.Compile(flags + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	for _, file := range files {
		matches, err := st.searchInFile(file, pattern, regex, options)
		if err != nil {
			fmt.Printf("⚠️ Error searching %s: %v\n", file, err)
			continue
		}

		if len(matches) > 0 {
			result.Files[file] = matches
			result.TotalMatches += len(matches)
		}
	}

	// Print results
	fmt.Printf("✅ Search completed: %d matches in %d files\n", result.TotalMatches, len(result.Files))
	for file, matches := range result.Files {
		fmt.Printf("\n📄 %s (%d matches):\n", file, len(matches))
		for _, match := range matches {
			if options.ShowLineNumbers {
				fmt.Printf("  %d: %s\n", match.LineNumber, strings.TrimSpace(match.Line))
			} else {
				fmt.Printf("  %s\n", strings.TrimSpace(match.Line))
			}
		}
	}

	return result, nil
}

// searchInFile searches for pattern in a single file
func (st *SystemTools) searchInFile(filePath, pattern string, regex *regexp.Regexp, options GrepOptions) ([]GrepMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []GrepMatch
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		var found bool
		if regex != nil {
			found = regex.MatchString(line)
		} else {
			searchText := line
			searchPattern := pattern

			if options.IgnoreCase {
				searchText = strings.ToLower(searchText)
				searchPattern = strings.ToLower(searchPattern)
			}

			if options.WholeWord {
				// Simple whole word matching
				words := strings.Fields(searchText)
				for _, word := range words {
					if word == searchPattern {
						found = true
						break
					}
				}
			} else {
				found = strings.Contains(searchText, searchPattern)
			}
		}

		if found {
			matches = append(matches, GrepMatch{
				Line:       line,
				LineNumber: lineNumber,
				MatchStart: strings.Index(line, pattern),
			})
		}
	}

	return matches, scanner.Err()
}

// ListDirectory lists directory contents with detailed info
func (st *SystemTools) ListDirectory(path string, options LsOptions) (*LsResult, error) {
	fmt.Printf("📂 Listing directory: %s\n", path)

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	result := &LsResult{
		Path:    path,
		Entries: make([]LsEntry, 0, len(entries)),
	}

	for _, entry := range entries {
		if !options.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if !options.ShowDirectories && entry.IsDir() {
			continue
		}

		if !options.ShowFiles && !entry.IsDir() {
			continue
		}

		lsEntry := LsEntry{
			Name:     entry.Name(),
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			FullPath: filepath.Join(path, entry.Name()),
		}

		result.Entries = append(result.Entries, lsEntry)
	}

	// Print results
	fmt.Printf("✅ Found %d entries\n", len(result.Entries))
	for _, entry := range result.Entries {
		if options.ShowDetails {
			fmt.Printf("  %s %8d %s %s\n",
				entry.Mode.String(),
				entry.Size,
				entry.ModTime.Format("2006-01-02 15:04"),
				entry.Name)
		} else {
			if entry.IsDir {
				fmt.Printf("  📁 %s/\n", entry.Name)
			} else {
				fmt.Printf("  📄 %s\n", entry.Name)
			}
		}
	}

	return result, nil
}

// GrepOptions configures grep search behavior
type GrepOptions struct {
	IgnoreCase      bool
	UseRegex        bool
	WholeWord       bool
	ShowLineNumbers bool
	MaxMatches      int
}

// DefaultGrepOptions returns sensible defaults for grep
func DefaultGrepOptions() GrepOptions {
	return GrepOptions{
		IgnoreCase:      false,
		UseRegex:        false,
		WholeWord:       false,
		ShowLineNumbers: true,
		MaxMatches:      100,
	}
}

// GrepResult contains the results of a grep search
type GrepResult struct {
	Pattern      string
	Files        map[string][]GrepMatch
	TotalMatches int
	Options      GrepOptions
}

// GrepMatch represents a single match
type GrepMatch struct {
	Line       string
	LineNumber int
	MatchStart int
}

// LsOptions configures directory listing behavior
type LsOptions struct {
	ShowHidden      bool
	ShowDirectories bool
	ShowFiles       bool
	ShowDetails     bool
	Recursive       bool
	DirsOnly        bool
}

// DefaultLsOptions returns sensible defaults for ls
func DefaultLsOptions() LsOptions {
	return LsOptions{
		ShowHidden:      false,
		ShowDirectories: true,
		ShowFiles:       true,
		ShowDetails:     false,
		Recursive:       false,
	}
}

// LsResult contains directory listing results
type LsResult struct {
	Path    string
	Entries []LsEntry
}

// LsEntry represents a directory entry
type LsEntry struct {
	Name     string
	IsDir    bool
	Size     int64
	ModTime  time.Time
	Mode     os.FileMode
	FullPath string
}
