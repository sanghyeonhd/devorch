package builtin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/tool"
)

// WriteParams defines parameters for writing files
type WriteParams struct {
	FilePath string `json:"filePath"`
	Content  string `json:"content"`
}

// WriteTool writes content to files
type WriteTool struct {
	// RootDir restricts writing to this directory
	RootDir string

	// AllowExternal allows writing files outside RootDir
	AllowExternal bool

	// AllowOverwrite allows overwriting existing files
	AllowOverwrite bool

	// CreateDirs automatically creates parent directories
	CreateDirs bool
}

// Info returns tool metadata
func (t *WriteTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "write",
		Description: `Write content to a file.
Creates a new file or overwrites an existing file with the provided content.
Parent directories are created automatically if they don't exist.
The file content should be complete - this tool replaces the entire file.
For partial edits, use the 'edit' tool instead.`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"filePath": {
					"type": "string",
					"description": "Path to the file to write"
				},
				"content": {
					"type": "string",
					"description": "Complete content to write to the file"
				}
			},
			"required": ["filePath", "content"]
		}`),
	}
}

// Execute writes the file
func (t *WriteTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p WriteParams
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
	existed := false
	var oldContent []byte
	if info, err := os.Stat(filePath); err == nil {
		existed = true
		if info.IsDir() {
			return nil, fmt.Errorf("cannot write to directory: %s", p.FilePath)
		}
		if !t.AllowOverwrite {
			return nil, fmt.Errorf("file already exists: %s (set AllowOverwrite to true)", p.FilePath)
		}
		oldContent, _ = os.ReadFile(filePath)
	}

	// Create parent directories if needed
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if !t.CreateDirs {
			return nil, fmt.Errorf("directory does not exist: %s", dir)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("cannot create directory: %w", err)
		}
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(p.Content), 0644); err != nil {
		return nil, fmt.Errorf("cannot write file: %w", err)
	}

	// Calculate hash
	hash := sha256.Sum256([]byte(p.Content))
	hashStr := hex.EncodeToString(hash[:8]) // short hash

	// Build output
	var output strings.Builder
	relPath, _ := filepath.Rel(ctx.WorkDir, filePath)
	if relPath == "" {
		relPath = filepath.Base(filePath)
	}

	if existed {
		output.WriteString(fmt.Sprintf("Updated file: %s\n", relPath))
		output.WriteString(fmt.Sprintf("Bytes written: %d\n", len(p.Content)))
		if len(oldContent) > 0 {
			output.WriteString(fmt.Sprintf("Previous size: %d bytes\n", len(oldContent)))
		}
	} else {
		output.WriteString(fmt.Sprintf("Created file: %s\n", relPath))
		output.WriteString(fmt.Sprintf("Bytes written: %d\n", len(p.Content)))
	}

	// Count lines
	lines := strings.Count(p.Content, "\n") + 1
	output.WriteString(fmt.Sprintf("Lines: %d\n", lines))
	output.WriteString(fmt.Sprintf("Hash: %s", hashStr))

	return &tool.Result{
		Output: output.String(),
		Title:  relPath,
		Metadata: map[string]any{
			"file_path":     filePath,
			"bytes_written": len(p.Content),
			"lines":         lines,
			"existed":       existed,
			"hash":          hashStr,
		},
	}, nil
}

// Name returns tool name for legacy interface
func (t *WriteTool) Name() string {
	return "write"
}
