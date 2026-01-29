package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/tool"
)

// EditParams defines parameters for editing files
type EditParams struct {
	FilePath  string `json:"filePath"`
	OldString string `json:"oldString"`
	NewString string `json:"newString"`
}

// EditTool performs targeted edits on files
type EditTool struct {
	// RootDir restricts editing to this directory
	RootDir string

	// AllowExternal allows editing files outside RootDir
	AllowExternal bool
}

// Info returns tool metadata
func (t *EditTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "edit",
		Description: `Edit a file by replacing specific text.
This tool finds an exact match of 'oldString' in the file and replaces it with 'newString'.
The 'oldString' must match exactly including whitespace and indentation.
For best results, include a few lines of context before and after the target text.
Only the first occurrence is replaced. Use multiple edits for multiple replacements.
For complete file rewrites, use the 'write' tool instead.`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"filePath": {
					"type": "string",
					"description": "Path to the file to edit"
				},
				"oldString": {
					"type": "string",
					"description": "Exact text to find and replace (include context lines)"
				},
				"newString": {
					"type": "string",
					"description": "Text to replace oldString with"
				}
			},
			"required": ["filePath", "oldString", "newString"]
		}`),
	}
}

// Execute performs the edit
func (t *EditTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p EditParams
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
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", p.FilePath)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	contentStr := string(content)

	// Find the old string
	count := strings.Count(contentStr, p.OldString)
	if count == 0 {
		// Try to provide helpful error
		return nil, t.noMatchError(p.FilePath, p.OldString, contentStr)
	}

	if count > 1 {
		return nil, fmt.Errorf("oldString matches %d locations in %s - please include more context to make it unique",
			count, p.FilePath)
	}

	// Perform replacement
	newContent := strings.Replace(contentStr, p.OldString, p.NewString, 1)

	// Write file
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("cannot write file: %w", err)
	}

	// Calculate diff summary
	oldLines := strings.Count(p.OldString, "\n")
	newLines := strings.Count(p.NewString, "\n")
	linesDiff := newLines - oldLines

	relPath, _ := filepath.Rel(ctx.WorkDir, filePath)
	if relPath == "" {
		relPath = filepath.Base(filePath)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Edited file: %s\n", relPath))
	output.WriteString(fmt.Sprintf("Replaced %d characters with %d characters\n",
		len(p.OldString), len(p.NewString)))

	if linesDiff > 0 {
		output.WriteString(fmt.Sprintf("Added %d lines", linesDiff))
	} else if linesDiff < 0 {
		output.WriteString(fmt.Sprintf("Removed %d lines", -linesDiff))
	} else {
		output.WriteString("Line count unchanged")
	}

	return &tool.Result{
		Output: output.String(),
		Title:  relPath,
		Metadata: map[string]any{
			"file_path":     filePath,
			"old_length":    len(p.OldString),
			"new_length":    len(p.NewString),
			"lines_changed": linesDiff,
			"total_lines":   strings.Count(newContent, "\n") + 1,
		},
	}, nil
}

// noMatchError provides helpful error message when old string not found
func (t *EditTool) noMatchError(filePath, oldString, content string) error {
	// Try to find similar content
	oldLines := strings.Split(oldString, "\n")
	if len(oldLines) == 0 {
		return fmt.Errorf("oldString not found in %s", filePath)
	}

	// Check first line
	firstLine := strings.TrimSpace(oldLines[0])
	if firstLine != "" && strings.Contains(content, firstLine) {
		// First line exists but full match doesn't - likely whitespace issue
		return fmt.Errorf("oldString not found in %s (first line exists - check whitespace/indentation)", filePath)
	}

	// Try to find any matching line
	for _, line := range oldLines {
		line = strings.TrimSpace(line)
		if line != "" && len(line) > 10 && strings.Contains(content, line) {
			return fmt.Errorf("oldString not found in %s (partial match found - check context)", filePath)
		}
	}

	return fmt.Errorf("oldString not found in %s", filePath)
}

// Name returns tool name for legacy interface
func (t *EditTool) Name() string {
	return "edit"
}
