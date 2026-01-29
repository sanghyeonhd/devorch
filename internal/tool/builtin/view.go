// Package tools provides the view tool implementation
package builtin

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/tool"
)

// ViewParams represents parameters for the view tool
type ViewParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"` // 0-based line number to start from
	Limit    int    `json:"limit"`  // number of lines to read
}

// ViewResponseMetadata contains metadata about the view response
type ViewResponseMetadata struct {
	FilePath   string `json:"file_path"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Truncated  bool   `json:"truncated"`
}

// ViewTool implements file viewing with line numbers
type ViewTool struct{}

const (
	ViewToolName     = "view"
	MaxViewSize      = 250 * 1024 // 250KB max file size
	DefaultViewLimit = 2000       // default lines to read
	MaxLineLength    = 2000       // max characters per line
)

var imageExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".bmp":  "image/bmp",
}

// NewViewTool creates a new view tool
func NewViewTool() *ViewTool {
	return &ViewTool{}
}

// Info returns tool information
func (t *ViewTool) Info() tool.ToolInfo {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "The line number to start reading from (0-based)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "The number of lines to read (defaults to 2000)",
			},
		},
		"required": []string{"file_path"},
	}
	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name: ViewToolName,
		Description: `File viewing tool that reads and displays the contents of files with line numbers.

WHEN TO USE THIS TOOL:
- Use when you need to examine the contents of a file
- Helpful for reviewing code, logs, or configuration files
- Use this instead of 'cat' or 'head' bash commands

HOW TO USE:
- Provide the file path to read
- Optionally specify offset (0-based line number) to start from a specific line
- Optionally specify limit to control how many lines to read

FEATURES:
- Displays content with line numbers for easy reference
- Supports reading specific sections of large files
- Handles various file encodings
- Can display image files as base64

TIPS:
- For code exploration, first use grep to find relevant files, then view to examine them
- When viewing large files, use the offset parameter to read specific sections`,
		Parameters: schemaJSON,
	}
}

// Execute runs the view tool
func (t *ViewTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p ViewParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if p.FilePath == "" {
		return &tool.Result{
			Output:   "file_path is required",
			ExitCode: 1,
		}, nil
	}

	// Handle relative paths
	filePath := p.FilePath
	if !filepath.IsAbs(filePath) {
		if ctx.WorkDir != "" {
			filePath = filepath.Join(ctx.WorkDir, filePath)
		}
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to offer suggestions
			suggestions := findSimilarFiles(filePath)
			if len(suggestions) > 0 {
				return &tool.Result{
					Output:   fmt.Sprintf("File not found: %s\n\nDid you mean:\n%s", filePath, strings.Join(suggestions, "\n")),
					ExitCode: 1,
				}, nil
			}
			return &tool.Result{
				Output:   fmt.Sprintf("File not found: %s", filePath),
				ExitCode: 1,
			}, nil
		}
		return &tool.Result{
			Output:   fmt.Sprintf("Error accessing file: %s", err),
			ExitCode: 1,
		}, nil
	}

	if fileInfo.IsDir() {
		return &tool.Result{
			Output:   fmt.Sprintf("Path is a directory, not a file: %s\nUse the 'ls' tool to list directory contents", filePath),
			ExitCode: 1,
		}, nil
	}

	// Check for image files
	ext := strings.ToLower(filepath.Ext(filePath))
	if mimeType, isImage := imageExtensions[ext]; isImage {
		return t.handleImageFile(filePath, mimeType)
	}

	// Set default limit
	limit := p.Limit
	if limit <= 0 {
		limit = DefaultViewLimit
	}

	// Read the file
	content, totalLines, err := readFileWithLineNumbers(filePath, p.Offset, limit)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error reading file: %s", err),
			ExitCode: 1,
		}, nil
	}

	endLine := p.Offset + limit
	if endLine > totalLines {
		endLine = totalLines
	}

	truncated := endLine < totalLines

	metadata := ViewResponseMetadata{
		FilePath:   filePath,
		TotalLines: totalLines,
		StartLine:  p.Offset + 1, // 1-based for display
		EndLine:    endLine,
		Truncated:  truncated,
	}

	var output strings.Builder
	output.WriteString(content)

	if truncated {
		output.WriteString(fmt.Sprintf("\n\n[Truncated: showing lines %d-%d of %d total lines. Use offset parameter to view more.]",
			metadata.StartLine, metadata.EndLine, metadata.TotalLines))
	}

	return &tool.Result{
		Output:   output.String(),
		Metadata: map[string]any{"view": metadata},
	}, nil
}

func (t *ViewTool) handleImageFile(filePath, mimeType string) (*tool.Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error reading image file: %s", err),
			ExitCode: 1,
		}, nil
	}

	base64Data := base64.StdEncoding.EncodeToString(data)

	return &tool.Result{
		Output: fmt.Sprintf("[Image file: %s, type: %s, size: %d bytes]", filePath, mimeType, len(data)),
		Attachments: []tool.Attachment{
			{
				Type:     "image",
				MIME:     mimeType,
				URL:      fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
				FileName: filepath.Base(filePath),
			},
		},
	}, nil
}

func readFileWithLineNumbers(filePath string, offset, limit int) (string, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	// Check file size
	stat, err := file.Stat()
	if err != nil {
		return "", 0, err
	}

	if stat.Size() > MaxViewSize {
		// For large files, we still try to read but may truncate
	}

	var output strings.Builder
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	linesWritten := 0
	totalLines := 0

	for scanner.Scan() {
		totalLines++
		lineNum++

		if lineNum <= offset {
			continue
		}

		if linesWritten >= limit {
			// Continue counting total lines
			continue
		}

		line := scanner.Text()

		// Truncate very long lines
		if len(line) > MaxLineLength {
			line = line[:MaxLineLength] + "... [truncated]"
		}

		output.WriteString(fmt.Sprintf("%6d │ %s\n", lineNum, line))
		linesWritten++
	}

	if err := scanner.Err(); err != nil {
		// If we got an error but already read some content, return what we have
		if output.Len() > 0 {
			return output.String(), totalLines, nil
		}
		return "", 0, err
	}

	return output.String(), totalLines, nil
}

func findSimilarFiles(filePath string) []string {
	dir := filepath.Dir(filePath)
	base := strings.ToLower(filepath.Base(filePath))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		// Simple similarity check
		if strings.Contains(name, base) || strings.Contains(base, name) ||
			levenshteinDistance(name, base) <= 3 {
			suggestions = append(suggestions, filepath.Join(dir, entry.Name()))
			if len(suggestions) >= 5 {
				break
			}
		}
	}

	return suggestions
}

func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}

	prevRow := make([]int, len(s1)+1)
	for i := range prevRow {
		prevRow[i] = i
	}

	for j := 1; j <= len(s2); j++ {
		currRow := make([]int, len(s1)+1)
		currRow[0] = j

		for i := 1; i <= len(s1); i++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			currRow[i] = min(
				prevRow[i]+1,
				currRow[i-1]+1,
				prevRow[i-1]+cost,
			)
		}
		prevRow = currRow
	}

	return prevRow[len(s1)]
}

// LineScanner handles scanning lines with custom buffer
type LineScanner struct {
	reader  *bufio.Reader
	line    string
	err     error
	maxSize int
}

// NewLineScanner creates a new line scanner
func NewLineScanner(r io.Reader) *LineScanner {
	return &LineScanner{
		reader:  bufio.NewReaderSize(r, 64*1024),
		maxSize: MaxLineLength,
	}
}

// Scan reads the next line
func (s *LineScanner) Scan() bool {
	line, err := s.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		s.err = err
		return false
	}
	if err == io.EOF && len(line) == 0 {
		return false
	}
	s.line = strings.TrimSuffix(line, "\n")
	return true
}

// Text returns the current line
func (s *LineScanner) Text() string {
	return s.line
}

// Err returns any error
func (s *LineScanner) Err() error {
	return s.err
}
