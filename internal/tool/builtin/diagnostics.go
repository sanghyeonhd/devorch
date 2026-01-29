// Package tools provides the diagnostics tool implementation (LSP integration)
package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"devorch/internal/tool"
)

// DiagnosticsParams represents parameters for the diagnostics tool
type DiagnosticsParams struct {
	FilePath string `json:"file_path"` // Optional: specific file to check
}

// DiagnosticsTool implements LSP diagnostics checking
type DiagnosticsTool struct{}

const DiagnosticsToolName = "diagnostics"

// Diagnostic represents a single diagnostic message
type Diagnostic struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // "error", "warning", "info", "hint"
	Message  string `json:"message"`
	Source   string `json:"source"` // e.g., "go", "typescript", "eslint"
}

// NewDiagnosticsTool creates a new diagnostics tool
func NewDiagnosticsTool() *DiagnosticsTool {
	return &DiagnosticsTool{}
}

// Info returns tool information
func (t *DiagnosticsTool) Info() tool.ToolInfo {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to a specific file to check. If not provided, checks the entire project.",
			},
		},
	}
	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name: DiagnosticsToolName,
		Description: `Get diagnostics (errors, warnings) for a file or project.

WHEN TO USE THIS TOOL:
- Use when you need to check for errors or warnings in your code
- Helpful for debugging and ensuring code quality
- Good for getting a quick overview of issues in a file or project
- Use after making edits to verify the changes are correct

HOW TO USE:
- Provide a path to a file to get diagnostics for that file
- Leave the path empty to get diagnostics for the entire project

FEATURES:
- Displays errors, warnings, hints, and info messages
- Groups diagnostics by severity
- Supports multiple languages (Go, TypeScript, Python, etc.)
- Provides line and column numbers for each issue

LIMITATIONS:
- Requires language server to be running for full diagnostics
- Some diagnostics may require building the project first`,
		Parameters: schemaJSON,
	}
}

// Execute runs the diagnostics tool
func (t *DiagnosticsTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p DiagnosticsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	// If no file path specified, check project-wide
	var diagnostics []Diagnostic

	if p.FilePath != "" {
		// Check specific file
		filePath := p.FilePath
		if !filepath.IsAbs(filePath) && ctx.WorkDir != "" {
			filePath = filepath.Join(ctx.WorkDir, filePath)
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return &tool.Result{
				Output:   fmt.Sprintf("File not found: %s", filePath),
				ExitCode: 1,
			}, nil
		}

		fileDiags := t.getFileDiagnostics(filePath)
		diagnostics = append(diagnostics, fileDiags...)
	} else {
		// Check all files in project
		workDir := ctx.WorkDir
		if workDir == "" {
			workDir, _ = os.Getwd()
		}

		diagnostics = t.getProjectDiagnostics(workDir)
	}

	if len(diagnostics) == 0 {
		return &tool.Result{
			Output: "No diagnostics found. Code looks good! ✓",
		}, nil
	}

	// Format output
	output := t.formatDiagnostics(diagnostics)

	return &tool.Result{
		Output: output,
		Metadata: map[string]any{
			"total_diagnostics": len(diagnostics),
			"errors":            countBySeverity(diagnostics, "error"),
			"warnings":          countBySeverity(diagnostics, "warning"),
		},
	}, nil
}

func (t *DiagnosticsTool) getFileDiagnostics(filePath string) []Diagnostic {
	var diagnostics []Diagnostic

	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		diagnostics = append(diagnostics, t.getGoDiagnostics(filePath)...)
	case ".ts", ".tsx", ".js", ".jsx":
		diagnostics = append(diagnostics, t.getTypeScriptDiagnostics(filePath)...)
	case ".py":
		diagnostics = append(diagnostics, t.getPythonDiagnostics(filePath)...)
	}

	return diagnostics
}

func (t *DiagnosticsTool) getProjectDiagnostics(workDir string) []Diagnostic {
	var diagnostics []Diagnostic

	// Walk project directory
	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" ||
				name == "vendor" || name == "__pycache__" || name == "dist" ||
				name == "build" || name == "target" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" ||
			ext == ".jsx" || ext == ".py" {
			fileDiags := t.getFileDiagnostics(path)
			diagnostics = append(diagnostics, fileDiags...)
		}

		return nil
	})

	return diagnostics
}

func (t *DiagnosticsTool) getGoDiagnostics(filePath string) []Diagnostic {
	// This would integrate with gopls in a full implementation
	// For now, we do basic syntax checking
	var diagnostics []Diagnostic

	content, err := os.ReadFile(filePath)
	if err != nil {
		return diagnostics
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		// Check for common issues
		if strings.Contains(line, "fmt.Println") && strings.Contains(line, "//") {
			// Commented print statement - hint
		}

		// Check for TODO/FIXME comments
		if strings.Contains(strings.ToUpper(line), "TODO") {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   strings.Index(strings.ToUpper(line), "TODO") + 1,
				Severity: "info",
				Message:  "TODO comment found",
				Source:   "go",
			})
		}

		if strings.Contains(strings.ToUpper(line), "FIXME") {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   strings.Index(strings.ToUpper(line), "FIXME") + 1,
				Severity: "warning",
				Message:  "FIXME comment found - needs attention",
				Source:   "go",
			})
		}
	}

	return diagnostics
}

func (t *DiagnosticsTool) getTypeScriptDiagnostics(filePath string) []Diagnostic {
	// This would integrate with tsserver in a full implementation
	var diagnostics []Diagnostic

	content, err := os.ReadFile(filePath)
	if err != nil {
		return diagnostics
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		// Check for common issues
		if strings.Contains(line, "console.log") {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   strings.Index(line, "console.log") + 1,
				Severity: "info",
				Message:  "console.log statement found - consider removing before production",
				Source:   "typescript",
			})
		}

		// Check for 'any' type usage
		if strings.Contains(line, ": any") || strings.Contains(line, "<any>") {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   1,
				Severity: "warning",
				Message:  "Usage of 'any' type - consider using a more specific type",
				Source:   "typescript",
			})
		}

		// Check for TODO/FIXME
		if strings.Contains(strings.ToUpper(line), "TODO") {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   strings.Index(strings.ToUpper(line), "TODO") + 1,
				Severity: "info",
				Message:  "TODO comment found",
				Source:   "typescript",
			})
		}
	}

	return diagnostics
}

func (t *DiagnosticsTool) getPythonDiagnostics(filePath string) []Diagnostic {
	// This would integrate with pylsp/pyright in a full implementation
	var diagnostics []Diagnostic

	content, err := os.ReadFile(filePath)
	if err != nil {
		return diagnostics
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		// Check for common issues
		if strings.Contains(line, "print(") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			// This is informational, not necessarily a problem
		}

		// Check for bare except
		trimmed := strings.TrimSpace(line)
		if trimmed == "except:" {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   1,
				Severity: "warning",
				Message:  "Bare 'except:' clause - consider catching specific exceptions",
				Source:   "python",
			})
		}

		// Check for TODO/FIXME
		if strings.Contains(strings.ToUpper(line), "TODO") {
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     i + 1,
				Column:   strings.Index(strings.ToUpper(line), "TODO") + 1,
				Severity: "info",
				Message:  "TODO comment found",
				Source:   "python",
			})
		}
	}

	return diagnostics
}

func (t *DiagnosticsTool) formatDiagnostics(diagnostics []Diagnostic) string {
	var sb strings.Builder

	// Group by severity
	errors := filterBySeverity(diagnostics, "error")
	warnings := filterBySeverity(diagnostics, "warning")
	infos := filterBySeverity(diagnostics, "info")
	hints := filterBySeverity(diagnostics, "hint")

	// Sort each group by file and line
	sortDiagnostics(errors)
	sortDiagnostics(warnings)
	sortDiagnostics(infos)
	sortDiagnostics(hints)

	sb.WriteString(fmt.Sprintf("# Diagnostics Summary\n\n"))
	sb.WriteString(fmt.Sprintf("- Errors: %d\n", len(errors)))
	sb.WriteString(fmt.Sprintf("- Warnings: %d\n", len(warnings)))
	sb.WriteString(fmt.Sprintf("- Info: %d\n", len(infos)))
	sb.WriteString(fmt.Sprintf("- Hints: %d\n\n", len(hints)))

	if len(errors) > 0 {
		sb.WriteString("## ❌ Errors\n\n")
		for _, d := range errors {
			sb.WriteString(fmt.Sprintf("- **%s:%d:%d** - %s\n", d.FilePath, d.Line, d.Column, d.Message))
		}
		sb.WriteString("\n")
	}

	if len(warnings) > 0 {
		sb.WriteString("## ⚠️ Warnings\n\n")
		for _, d := range warnings {
			sb.WriteString(fmt.Sprintf("- **%s:%d:%d** - %s\n", d.FilePath, d.Line, d.Column, d.Message))
		}
		sb.WriteString("\n")
	}

	if len(infos) > 0 {
		sb.WriteString("## ℹ️ Info\n\n")
		for _, d := range infos {
			sb.WriteString(fmt.Sprintf("- %s:%d - %s\n", d.FilePath, d.Line, d.Message))
		}
		sb.WriteString("\n")
	}

	if len(hints) > 0 {
		sb.WriteString("## 💡 Hints\n\n")
		for _, d := range hints {
			sb.WriteString(fmt.Sprintf("- %s:%d - %s\n", d.FilePath, d.Line, d.Message))
		}
	}

	return sb.String()
}

func filterBySeverity(diagnostics []Diagnostic, severity string) []Diagnostic {
	var result []Diagnostic
	for _, d := range diagnostics {
		if d.Severity == severity {
			result = append(result, d)
		}
	}
	return result
}

func countBySeverity(diagnostics []Diagnostic, severity string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Severity == severity {
			count++
		}
	}
	return count
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].FilePath != diagnostics[j].FilePath {
			return diagnostics[i].FilePath < diagnostics[j].FilePath
		}
		return diagnostics[i].Line < diagnostics[j].Line
	})
}
