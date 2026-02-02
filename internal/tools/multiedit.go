// Package tools provides advanced multi-file editing capabilities
package tools

import (
	"fmt"
)

// MultiEditTool provides batch file editing capabilities like OpenCode
type MultiEditTool struct {
	fileWriter *FileWriter
	operations []EditOperation
}

// NewMultiEditTool creates a new multi-edit tool instance
func NewMultiEditTool(fw *FileWriter) *MultiEditTool {
	return &MultiEditTool{
		fileWriter: fw,
		operations: make([]EditOperation, 0),
	}
}

// EditOperation represents a single edit operation
type EditOperation struct {
	Type       string // "create", "replace", "patch"
	FilePath   string
	OldContent string
	NewContent string
}

// MultiEditResult contains the results of a multi-edit operation
type MultiEditResult struct {
	TotalOperations      int
	SuccessfulOperations int
	FailedOperations     int
	Errors               map[string]error
}

// AddCreateOperation adds a file creation operation
func (met *MultiEditTool) AddCreateOperation(filePath, content string) {
	op := EditOperation{
		Type:       "create",
		FilePath:   filePath,
		NewContent: content,
	}
	met.operations = append(met.operations, op)
}

// AddReplaceOperation adds a text replacement operation
func (met *MultiEditTool) AddReplaceOperation(filePath, oldContent, newContent string) {
	op := EditOperation{
		Type:       "replace",
		FilePath:   filePath,
		OldContent: oldContent,
		NewContent: newContent,
	}
	met.operations = append(met.operations, op)
}

// AddPatchOperation adds a patch operation
func (met *MultiEditTool) AddPatchOperation(filePath, patch string) {
	op := EditOperation{
		Type:       "patch",
		FilePath:   filePath,
		NewContent: patch,
	}
	met.operations = append(met.operations, op)
}

// Clear clears all queued operations
func (met *MultiEditTool) Clear() {
	met.operations = make([]EditOperation, 0)
}

// ListOperations returns all queued operations
func (met *MultiEditTool) ListOperations() []EditOperation {
	return met.operations
}

// Execute executes all queued operations
func (met *MultiEditTool) Execute(dryRun bool) (*MultiEditResult, error) {
	result := &MultiEditResult{
		TotalOperations: len(met.operations),
		Errors:          make(map[string]error),
	}

	for _, op := range met.operations {
		err := met.executeOperation(op, dryRun)
		if err != nil {
			result.FailedOperations++
			result.Errors[op.FilePath] = err
		} else {
			result.SuccessfulOperations++
		}
	}

	return result, nil
}

// executeOperation executes a single operation
func (met *MultiEditTool) executeOperation(op EditOperation, dryRun bool) error {
	if dryRun {
		// Just validate, don't execute
		return nil
	}

	switch op.Type {
	case "create":
		return met.fileWriter.CreateFile(op.FilePath, op.NewContent)
	case "replace":
		return met.fileWriter.ReplaceInFile(op.FilePath, op.OldContent, op.NewContent)
	case "patch":
		return met.fileWriter.PatchFile(op.FilePath, op.NewContent)
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}
