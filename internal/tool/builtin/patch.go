// Package tools provides the patch tool implementation
package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"devorch/internal/tool"
)

// PatchParams represents parameters for the patch tool
type PatchParams struct {
	Patch string `json:"patch"` // The patch content in unified diff format
}

// PatchTool implements applying patches to files
type PatchTool struct{}

const PatchToolName = "patch"

// PatchOperation represents a single file operation in a patch
type PatchOperation struct {
	Type     string // "update", "add", "delete"
	FilePath string
	Hunks    []Hunk
	Content  string // For new files
}

// Hunk represents a single hunk in a patch
type Hunk struct {
	ContextLine string   // Line to search for context
	RemoveLines []string // Lines to remove (prefixed with -)
	AddLines    []string // Lines to add (prefixed with +)
	KeepLines   []string // Lines to keep (prefixed with space)
}

// NewPatchTool creates a new patch tool
func NewPatchTool() *PatchTool {
	return &PatchTool{}
}

// Info returns tool information
func (t *PatchTool) Info() tool.ToolInfo {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{
				"type":        "string",
				"description": "The patch content to apply",
			},
		},
		"required": []string{"patch"},
	}
	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name: PatchToolName,
		Description: `Applies patches to files using a unified diff-like format.

The patch text must follow this format:
*** Begin Patch
*** Update File: /path/to/file
@@ Context line (unique within the file)
 Line to keep
-Line to remove
+Line to add
 Line to keep
*** Add File: /path/to/new/file
+Content of the new file
+More content
*** Delete File: /path/to/file/to/delete
*** End Patch

Before using this tool:
1. Use the View tool to understand the files' contents and context
2. Verify all file paths are correct (use the ls tool)

CRITICAL REQUIREMENTS:
1. UNIQUENESS: Context lines MUST uniquely identify the specific sections you want to change
2. PRECISION: All whitespace, indentation, and surrounding code must match exactly
3. VALIDATION: Ensure edits result in idiomatic, correct code
4. PATHS: Always use absolute file paths (starting with /)

The tool will apply all changes in a single atomic operation.`,
		Parameters: schemaJSON,
	}
}

// Execute runs the patch tool
func (t *PatchTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p PatchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if p.Patch == "" {
		return &tool.Result{
			Output:   "patch content is required",
			ExitCode: 1,
		}, nil
	}

	// Parse the patch
	operations, err := parsePatch(p.Patch, ctx.WorkDir)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing patch: %s", err),
			ExitCode: 1,
		}, nil
	}

	if len(operations) == 0 {
		return &tool.Result{
			Output:   "No operations found in patch",
			ExitCode: 1,
		}, nil
	}

	// Validate all operations first
	var validationErrors []string
	for _, op := range operations {
		if err := validateOperation(op); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("%s: %s", op.FilePath, err))
		}
	}

	if len(validationErrors) > 0 {
		return &tool.Result{
			Output:   fmt.Sprintf("Validation errors:\n%s", strings.Join(validationErrors, "\n")),
			ExitCode: 1,
		}, nil
	}

	// Apply operations
	var results []string
	for _, op := range operations {
		result, err := applyOperation(op)
		if err != nil {
			return &tool.Result{
				Output:   fmt.Sprintf("Error applying patch to %s: %s", op.FilePath, err),
				ExitCode: 1,
			}, nil
		}
		results = append(results, result)
	}

	return &tool.Result{
		Output: fmt.Sprintf("Patch applied successfully:\n%s", strings.Join(results, "\n")),
	}, nil
}

func parsePatch(patch string, workingDir string) ([]PatchOperation, error) {
	var operations []PatchOperation

	lines := strings.Split(patch, "\n")
	var currentOp *PatchOperation
	var currentHunk *Hunk
	inPatch := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.Contains(line, "*** Begin Patch") || strings.Contains(line, "***Begin Patch") {
			inPatch = true
			continue
		}

		if strings.Contains(line, "*** End Patch") || strings.Contains(line, "***End Patch") {
			if currentOp != nil {
				if currentHunk != nil {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				operations = append(operations, *currentOp)
			}
			break
		}

		if !inPatch {
			continue
		}

		// Check for file operations
		if strings.HasPrefix(line, "*** Update File:") || strings.HasPrefix(line, "***Update File:") {
			if currentOp != nil {
				if currentHunk != nil {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				operations = append(operations, *currentOp)
			}

			filePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "*** Update File:"), "***Update File:"))
			if !filepath.IsAbs(filePath) && workingDir != "" {
				filePath = filepath.Join(workingDir, filePath)
			}

			currentOp = &PatchOperation{
				Type:     "update",
				FilePath: filePath,
				Hunks:    []Hunk{},
			}
			currentHunk = nil
			continue
		}

		if strings.HasPrefix(line, "*** Add File:") || strings.HasPrefix(line, "***Add File:") {
			if currentOp != nil {
				if currentHunk != nil {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				operations = append(operations, *currentOp)
			}

			filePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "*** Add File:"), "***Add File:"))
			if !filepath.IsAbs(filePath) && workingDir != "" {
				filePath = filepath.Join(workingDir, filePath)
			}

			currentOp = &PatchOperation{
				Type:     "add",
				FilePath: filePath,
			}
			currentHunk = nil
			continue
		}

		if strings.HasPrefix(line, "*** Delete File:") || strings.HasPrefix(line, "***Delete File:") {
			if currentOp != nil {
				if currentHunk != nil {
					currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
				}
				operations = append(operations, *currentOp)
			}

			filePath := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "*** Delete File:"), "***Delete File:"))
			if !filepath.IsAbs(filePath) && workingDir != "" {
				filePath = filepath.Join(workingDir, filePath)
			}

			currentOp = &PatchOperation{
				Type:     "delete",
				FilePath: filePath,
			}
			operations = append(operations, *currentOp)
			currentOp = nil
			currentHunk = nil
			continue
		}

		// Handle hunk header (context line)
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil && currentOp != nil {
				currentOp.Hunks = append(currentOp.Hunks, *currentHunk)
			}

			// Extract context line
			contextLine := strings.TrimPrefix(line, "@@")
			contextLine = strings.TrimSpace(contextLine)

			currentHunk = &Hunk{
				ContextLine: contextLine,
			}
			continue
		}

		if currentOp == nil {
			continue
		}

		// Handle file content for new files
		if currentOp.Type == "add" {
			if strings.HasPrefix(line, "+") {
				currentOp.Content += strings.TrimPrefix(line, "+") + "\n"
			}
			continue
		}

		// Handle hunk content
		if currentHunk != nil {
			if strings.HasPrefix(line, "-") {
				currentHunk.RemoveLines = append(currentHunk.RemoveLines, strings.TrimPrefix(line, "-"))
			} else if strings.HasPrefix(line, "+") {
				currentHunk.AddLines = append(currentHunk.AddLines, strings.TrimPrefix(line, "+"))
			} else if strings.HasPrefix(line, " ") {
				currentHunk.KeepLines = append(currentHunk.KeepLines, strings.TrimPrefix(line, " "))
			}
		}
	}

	return operations, nil
}

func validateOperation(op PatchOperation) error {
	switch op.Type {
	case "update":
		if _, err := os.Stat(op.FilePath); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist")
		}
		if len(op.Hunks) == 0 {
			return fmt.Errorf("no hunks specified for update")
		}
	case "add":
		if _, err := os.Stat(op.FilePath); err == nil {
			return fmt.Errorf("file already exists")
		}
	case "delete":
		if _, err := os.Stat(op.FilePath); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist")
		}
	}
	return nil
}

func applyOperation(op PatchOperation) (string, error) {
	switch op.Type {
	case "update":
		return applyUpdateOperation(op)
	case "add":
		return applyAddOperation(op)
	case "delete":
		return applyDeleteOperation(op)
	default:
		return "", fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

func applyUpdateOperation(op PatchOperation) (string, error) {
	content, err := os.ReadFile(op.FilePath)
	if err != nil {
		return "", err
	}

	fileContent := string(content)

	for _, hunk := range op.Hunks {
		// Find the context line
		if hunk.ContextLine != "" {
			contextPattern := regexp.QuoteMeta(hunk.ContextLine)
			re := regexp.MustCompile("(?m)" + contextPattern)
			loc := re.FindStringIndex(fileContent)
			if loc == nil {
				return "", fmt.Errorf("context line not found: %s", hunk.ContextLine)
			}
		}

		// Apply removals and additions
		for _, removeLine := range hunk.RemoveLines {
			if !strings.Contains(fileContent, removeLine) {
				return "", fmt.Errorf("line to remove not found: %s", removeLine)
			}
			fileContent = strings.Replace(fileContent, removeLine+"\n", "", 1)
		}

		// For additions, we need to find the right place
		for i, addLine := range hunk.AddLines {
			if i == 0 && hunk.ContextLine != "" {
				// Add after context line
				fileContent = strings.Replace(fileContent, hunk.ContextLine+"\n",
					hunk.ContextLine+"\n"+addLine+"\n", 1)
			} else if i > 0 {
				// Add after previous added line
				prevLine := hunk.AddLines[i-1]
				fileContent = strings.Replace(fileContent, prevLine+"\n",
					prevLine+"\n"+addLine+"\n", 1)
			}
		}
	}

	if err := os.WriteFile(op.FilePath, []byte(fileContent), 0644); err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Updated: %s", op.FilePath), nil
}

func applyAddOperation(op PatchOperation) (string, error) {
	// Create parent directories if needed
	dir := filepath.Dir(op.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(op.FilePath, []byte(op.Content), 0644); err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Created: %s", op.FilePath), nil
}

func applyDeleteOperation(op PatchOperation) (string, error) {
	if err := os.Remove(op.FilePath); err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Deleted: %s", op.FilePath), nil
}
