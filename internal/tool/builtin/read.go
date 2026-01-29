package builtin

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/tool"
)

const (
	readDefaultLimit  = 2000 // lines
	readMaxLineLength = 2000 // characters
	readMaxBytes      = 50 * 1024
)

// ReadParams defines parameters for reading files
type ReadParams struct {
	FilePath string `json:"filePath"`
	Offset   int    `json:"offset,omitempty"` // 0-based line offset
	Limit    int    `json:"limit,omitempty"`  // number of lines to read
}

// ReadTool reads file contents
type ReadTool struct {
	// RootDir restricts reading to this directory
	RootDir string

	// AllowExternal allows reading files outside RootDir
	AllowExternal bool
}

// Info returns tool metadata
func (t *ReadTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "read",
		Description: `Read file contents.
Returns file contents with line numbers.
For large files, use offset and limit parameters to read specific sections.
Supports text files, images (returned as base64), and PDFs.
Binary files are not supported.`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"filePath": {
					"type": "string",
					"description": "Path to the file to read"
				},
				"offset": {
					"type": "number",
					"description": "Line number to start reading from (0-based)"
				},
				"limit": {
					"type": "number",
					"description": "Number of lines to read (default: 2000)"
				}
			},
			"required": ["filePath"]
		}`),
	}
}

// Execute reads the file
func (t *ReadTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p ReadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// Resolve path
	filePath := p.FilePath
	if !filepath.IsAbs(filePath) {
		base := ctx.WorkDir
		if base == "" {
			base = t.RootDir
		}
		if base == "" {
			wd, _ := os.Getwd()
			base = wd
		}
		filePath = filepath.Join(base, filePath)
	}

	// Clean path
	filePath = filepath.Clean(filePath)

	// Security check
	if t.RootDir != "" && !t.AllowExternal {
		if !strings.HasPrefix(filePath, filepath.Clean(t.RootDir)) {
			return nil, fmt.Errorf("access denied: %s is outside project directory", p.FilePath)
		}
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		suggestions := t.findSimilarFiles(filePath)
		if len(suggestions) > 0 {
			return nil, fmt.Errorf("file not found: %s\n\nDid you mean one of these?\n%s",
				p.FilePath, strings.Join(suggestions, "\n"))
		}
		return nil, fmt.Errorf("file not found: %s", p.FilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot read directory: %s", p.FilePath)
	}

	// Check file type
	mime := detectMIME(filePath)

	// Handle images
	if strings.HasPrefix(mime, "image/") && mime != "image/svg+xml" {
		return t.readImage(ctx, filePath, mime)
	}

	// Handle PDFs
	if mime == "application/pdf" {
		return t.readPDF(ctx, filePath)
	}

	// Check if binary
	if isBinaryFile(filePath) {
		return nil, fmt.Errorf("cannot read binary file: %s", p.FilePath)
	}

	// Read text file
	return t.readText(ctx, filePath, p.Offset, p.Limit)
}

// readText reads a text file with line numbers
func (t *ReadTool) readText(ctx *tool.Context, filePath string, offset, limit int) (*tool.Result, error) {
	if limit <= 0 {
		limit = readDefaultLimit
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read lines
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

	var lines []string
	lineNum := 0
	totalBytes := 0
	truncatedByBytes := false

	for scanner.Scan() {
		if lineNum < offset {
			lineNum++
			continue
		}

		if len(lines) >= limit {
			break
		}

		line := scanner.Text()
		if len(line) > readMaxLineLength {
			line = line[:readMaxLineLength] + "..."
		}

		lineBytes := len(line) + 1 // +1 for newline
		if totalBytes+lineBytes > readMaxBytes {
			truncatedByBytes = true
			break
		}

		lines = append(lines, line)
		totalBytes += lineBytes
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Format output with line numbers
	var sb strings.Builder
	sb.WriteString("<file>\n")
	for i, line := range lines {
		lineNum := offset + i + 1
		sb.WriteString(fmt.Sprintf("%05d| %s\n", lineNum, line))
	}

	// Count total lines
	file.Seek(0, 0)
	totalLines := 0
	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		totalLines++
	}

	lastReadLine := offset + len(lines)
	hasMoreLines := totalLines > lastReadLine
	truncated := hasMoreLines || truncatedByBytes

	if truncatedByBytes {
		sb.WriteString(fmt.Sprintf("\n(Output truncated at %d bytes. Use 'offset' parameter to read beyond line %d)\n",
			readMaxBytes, lastReadLine))
	} else if hasMoreLines {
		sb.WriteString(fmt.Sprintf("\n(File has more lines. Use 'offset' parameter to read beyond line %d)\n", lastReadLine))
	} else {
		sb.WriteString(fmt.Sprintf("\n(End of file - total %d lines)\n", totalLines))
	}
	sb.WriteString("</file>")

	// Generate preview
	preview := ""
	if len(lines) > 0 {
		previewLines := lines
		if len(previewLines) > 20 {
			previewLines = previewLines[:20]
		}
		preview = strings.Join(previewLines, "\n")
	}

	relPath, _ := filepath.Rel(ctx.WorkDir, filePath)
	if relPath == "" {
		relPath = filepath.Base(filePath)
	}

	return &tool.Result{
		Output:    sb.String(),
		Title:     relPath,
		Truncated: truncated,
		Metadata: map[string]any{
			"preview":     preview,
			"total_lines": totalLines,
			"read_lines":  len(lines),
			"truncated":   truncated,
		},
	}, nil
}

// readImage reads an image file as base64 attachment
func (t *ReadTool) readImage(ctx *tool.Context, filePath, mime string) (*tool.Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64Data)

	relPath, _ := filepath.Rel(ctx.WorkDir, filePath)
	if relPath == "" {
		relPath = filepath.Base(filePath)
	}

	return &tool.Result{
		Output: "Image read successfully",
		Title:  relPath,
		Metadata: map[string]any{
			"preview":   "Image read successfully",
			"truncated": false,
			"mime":      mime,
		},
		Attachments: []tool.Attachment{
			{
				SessionID: ctx.SessionID,
				MessageID: ctx.MessageID,
				Type:      "file",
				MIME:      mime,
				URL:       dataURL,
				FileName:  filepath.Base(filePath),
			},
		},
	}, nil
}

// readPDF reads a PDF file as base64 attachment
func (t *ReadTool) readPDF(ctx *tool.Context, filePath string) (*tool.Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:application/pdf;base64,%s", base64Data)

	relPath, _ := filepath.Rel(ctx.WorkDir, filePath)
	if relPath == "" {
		relPath = filepath.Base(filePath)
	}

	return &tool.Result{
		Output: "PDF read successfully",
		Title:  relPath,
		Metadata: map[string]any{
			"preview":   "PDF read successfully",
			"truncated": false,
			"mime":      "application/pdf",
		},
		Attachments: []tool.Attachment{
			{
				SessionID: ctx.SessionID,
				MessageID: ctx.MessageID,
				Type:      "file",
				MIME:      "application/pdf",
				URL:       dataURL,
				FileName:  filepath.Base(filePath),
			},
		},
	}, nil
}

// findSimilarFiles finds files with similar names
func (t *ReadTool) findSimilarFiles(filePath string) []string {
	dir := filepath.Dir(filePath)
	base := strings.ToLower(filepath.Base(filePath))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if strings.Contains(name, base) || strings.Contains(base, name) {
			suggestions = append(suggestions, filepath.Join(dir, entry.Name()))
			if len(suggestions) >= 3 {
				break
			}
		}
	}

	return suggestions
}

// Name returns tool name for legacy interface
func (t *ReadTool) Name() string {
	return "read"
}

// detectMIME detects file MIME type
func detectMIME(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	default:
		// Try to detect from content
		file, err := os.Open(filePath)
		if err != nil {
			return "application/octet-stream"
		}
		defer file.Close()

		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		return http.DetectContentType(buf[:n])
	}
}

// isBinaryFile checks if file is binary
func isBinaryFile(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 8KB
	buf := make([]byte, 8192)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}

	// Check for null bytes
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	return false
}
