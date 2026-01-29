package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/tool"
)

// MultiEditParams defines parameters for multi-file editing
type MultiEditParams struct {
	Edits []SingleEdit `json:"edits"`
}

// SingleEdit represents a single file edit operation
type SingleEdit struct {
	Path     string `json:"path"`      // file path
	OldText  string `json:"old_text"`  // text to find
	NewText  string `json:"new_text"`  // replacement text
	AllMatch bool   `json:"all_match"` // replace all occurrences
}

// MultiEditTool edits multiple files in a single operation
type MultiEditTool struct {
	// RootDir is the base directory
	RootDir string

	// AllowExternal allows editing files outside RootDir
	AllowExternal bool

	// DryRun if true, shows changes without applying
	DryRun bool
}

// Info returns tool metadata
func (t *MultiEditTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "multiedit",
		Description: `Edit multiple files in a single atomic operation.
Each edit specifies a file path, text to find, and replacement text.
All edits are validated before any changes are made.
If any edit fails validation, no changes are applied.

Best for:
- Renaming variables/functions across multiple files
- Updating imports in multiple files
- Making consistent changes across a codebase

Examples:
  - Rename function: {"edits": [
      {"path": "a.go", "old_text": "func OldName", "new_text": "func NewName"},
      {"path": "b.go", "old_text": "OldName()", "new_text": "NewName()"}
    ]}
  - Update imports: {"edits": [
      {"path": "main.go", "old_text": "\"old/pkg\"", "new_text": "\"new/pkg\""},
      {"path": "util.go", "old_text": "\"old/pkg\"", "new_text": "\"new/pkg\""}
    ]}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"edits": {
					"type": "array",
					"description": "List of edit operations",
					"items": {
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "File path to edit"
							},
							"old_text": {
								"type": "string",
								"description": "Exact text to find and replace"
							},
							"new_text": {
								"type": "string",
								"description": "Replacement text"
							},
							"all_match": {
								"type": "boolean",
								"description": "Replace all occurrences (default: first only)"
							}
						},
						"required": ["path", "old_text", "new_text"]
					}
				}
			},
			"required": ["edits"]
		}`),
	}
}

// EditResult represents the result of a single edit
type EditResult struct {
	Path        string `json:"path"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Occurrences int    `json:"occurrences"`
}

// Execute performs multiple file edits atomically
func (t *MultiEditTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p MultiEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	if len(p.Edits) == 0 {
		return nil, fmt.Errorf("no edits specified")
	}

	if len(p.Edits) > 50 {
		return nil, fmt.Errorf("too many edits (max 50, got %d)", len(p.Edits))
	}

	// Phase 1: Validate all edits and prepare changes
	type pendingEdit struct {
		path        string
		oldContent  string
		newContent  string
		occurrences int
	}

	var pending []pendingEdit
	var errors []string

	for i, edit := range p.Edits {
		// Resolve path
		targetPath := edit.Path
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(t.RootDir, targetPath)
		}
		targetPath = filepath.Clean(targetPath)

		// Security check
		if !t.AllowExternal && !strings.HasPrefix(targetPath, t.RootDir) && t.RootDir != "" {
			errors = append(errors, fmt.Sprintf("[%d] %s: access denied (outside root)", i+1, edit.Path))
			continue
		}

		// Read file
		content, err := os.ReadFile(targetPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("[%d] %s: %v", i+1, edit.Path, err))
			continue
		}

		oldContent := string(content)

		// Check if old_text exists
		occurrences := strings.Count(oldContent, edit.OldText)
		if occurrences == 0 {
			errors = append(errors, fmt.Sprintf("[%d] %s: text not found", i+1, edit.Path))
			continue
		}

		// Prepare new content
		var newContent string
		if edit.AllMatch {
			newContent = strings.ReplaceAll(oldContent, edit.OldText, edit.NewText)
		} else {
			newContent = strings.Replace(oldContent, edit.OldText, edit.NewText, 1)
			occurrences = 1
		}

		pending = append(pending, pendingEdit{
			path:        targetPath,
			oldContent:  oldContent,
			newContent:  newContent,
			occurrences: occurrences,
		})
	}

	// If any validation errors, return without making changes
	if len(errors) > 0 {
		return &tool.Result{
			Output: fmt.Sprintf("Validation failed:\n%s", strings.Join(errors, "\n")),
			Title:  "multiedit failed",
			Metadata: map[string]any{
				"success": false,
				"errors":  len(errors),
			},
		}, nil
	}

	// Phase 2: Apply all edits
	var results []EditResult
	var applied int

	if t.DryRun {
		// Dry run - just report what would happen
		for i, edit := range p.Edits {
			results = append(results, EditResult{
				Path:        edit.Path,
				Success:     true,
				Message:     fmt.Sprintf("would replace %d occurrence(s)", pending[i].occurrences),
				Occurrences: pending[i].occurrences,
			})
		}
	} else {
		// Apply changes
		for i, pe := range pending {
			err := os.WriteFile(pe.path, []byte(pe.newContent), 0644)
			if err != nil {
				// Rollback previous changes
				for j := 0; j < i; j++ {
					_ = os.WriteFile(pending[j].path, []byte(pending[j].oldContent), 0644)
				}
				return nil, fmt.Errorf("failed to write %s: %w (changes rolled back)", pe.path, err)
			}

			results = append(results, EditResult{
				Path:        p.Edits[i].Path,
				Success:     true,
				Message:     fmt.Sprintf("replaced %d occurrence(s)", pe.occurrences),
				Occurrences: pe.occurrences,
			})
			applied++
		}
	}

	// Format output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Multi-edit complete: %d file(s)\n\n", applied))

	for _, r := range results {
		status := "✓"
		if !r.Success {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("%s %s: %s\n", status, r.Path, r.Message))
	}

	return &tool.Result{
		Output: sb.String(),
		Title:  fmt.Sprintf("multiedit: %d files", applied),
		Metadata: map[string]any{
			"success":      true,
			"files_edited": applied,
			"results":      results,
			"dry_run":      t.DryRun,
		},
	}, nil
}

// Ensure MultiEditTool implements Tool interface
var _ tool.Tool = (*MultiEditTool)(nil)
