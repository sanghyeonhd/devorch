// Package tools provides file manipulation tools for DevOrch CLI
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileWriter provides file creation and modification capabilities
type FileWriter struct {
	basePath string
}

// NewFileWriter creates a new file writer
func NewFileWriter() *FileWriter {
	wd, _ := os.Getwd()
	return &FileWriter{
		basePath: wd,
	}
}

// CreateFile creates a new file with the given content
func (fw *FileWriter) CreateFile(path, content string) error {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Created file: %s\n", path)
	return nil
}

// UpdateFile modifies an existing file by applying a patch or replacement
func (fw *FileWriter) UpdateFile(path string, operation string, content string) error {
	switch operation {
	case "replace":
		return fw.replaceFileContent(path, content)
	case "append":
		return fw.appendToFile(path, content)
	case "prepend":
		return fw.prependToFile(path, content)
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

// replaceFileContent replaces the entire file content
func (fw *FileWriter) replaceFileContent(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to replace file content: %w", err)
	}
	fmt.Printf("✅ Updated file: %s\n", path)
	return nil
}

// appendToFile adds content to the end of the file
func (fw *FileWriter) appendToFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}

	fmt.Printf("✅ Appended to file: %s\n", path)
	return nil
}

// prependToFile adds content to the beginning of the file
func (fw *FileWriter) prependToFile(path, content string) error {
	// Read existing content
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing file: %w", err)
	}

	// Combine new and existing content
	newContent := content + string(existing)

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to prepend to file: %w", err)
	}

	fmt.Printf("✅ Prepended to file: %s\n", path)
	return nil
}

// ApplyPatch applies a unified diff patch to a file
func (fw *FileWriter) ApplyPatch(path, patch string) error {
	// This is a simplified patch application
	// For production, consider using a proper diff/patch library

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file for patching: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	patchLines := strings.Split(patch, "\n")

	// Simple patch parser (supports basic add/remove operations)
	var result []string
	lineNum := 0

	for _, patchLine := range patchLines {
		if strings.HasPrefix(patchLine, "@@") {
			// Parse line number from hunk header
			// Format: @@ -old_start,old_count +new_start,new_count @@
			continue
		} else if strings.HasPrefix(patchLine, "+") {
			// Add line
			result = append(result, strings.TrimPrefix(patchLine, "+"))
		} else if strings.HasPrefix(patchLine, "-") {
			// Remove line - skip it
			lineNum++
		} else if strings.HasPrefix(patchLine, " ") {
			// Context line - keep as is
			if lineNum < len(lines) {
				result = append(result, lines[lineNum])
				lineNum++
			}
		}
	}

	// Write the patched content
	patchedContent := strings.Join(result, "\n")
	if err := os.WriteFile(path, []byte(patchedContent), 0644); err != nil {
		return fmt.Errorf("failed to write patched file: %w", err)
	}

	fmt.Printf("✅ Applied patch to file: %s\n", path)
	return nil
}

// ListFiles lists files in a directory
func (fw *FileWriter) ListFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ReadFile reads the content of a file
func (fw *FileWriter) ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

// FileExists checks if a file exists
func (fw *FileWriter) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReplaceInFile replaces oldContent with newContent in the specified file
func (fw *FileWriter) ReplaceInFile(path, oldContent, newContent string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, oldContent) {
		return fmt.Errorf("old content not found in file")
	}

	newContentStr := strings.Replace(contentStr, oldContent, newContent, 1)

	if err := os.WriteFile(path, []byte(newContentStr), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Replaced content in file: %s\n", path)
	return nil
}

// PatchFile applies a patch to a file (simplified implementation)
func (fw *FileWriter) PatchFile(path, patch string) error {
	// This is a simplified patch implementation
	// In a real implementation, you'd parse proper patch format
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Simple append for now
	newContent := string(content) + "\n" + patch

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("✅ Applied patch to file: %s\n", path)
	return nil
}
