package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"devorch/internal/tool"
)

const (
	lsMaxEntries = 500
)

// LsParams defines parameters for directory listing
type LsParams struct {
	Path      string `json:"path"`                // directory path to list
	All       bool   `json:"all,omitempty"`       // include hidden files
	Long      bool   `json:"long,omitempty"`      // detailed listing
	Recursive bool   `json:"recursive,omitempty"` // recursive listing
	MaxDepth  int    `json:"max_depth,omitempty"` // max recursion depth (default: 3)
}

// LsTool lists directory contents
type LsTool struct {
	// RootDir is the base directory
	RootDir string
}

// Info returns tool metadata
func (t *LsTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "ls",
		Description: `List directory contents.
Returns files and subdirectories in the specified path.
Use 'all' to include hidden files (starting with .).
Use 'long' for detailed information (size, modification time).
Use 'recursive' to list subdirectories recursively.

Examples:
  - {"path": "."} - list current directory
  - {"path": "src", "all": true} - list src including hidden files
  - {"path": ".", "long": true} - detailed listing
  - {"path": ".", "recursive": true, "max_depth": 2} - recursive with depth limit`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Directory path to list (relative or absolute)"
				},
				"all": {
					"type": "boolean",
					"description": "Include hidden files (starting with .)"
				},
				"long": {
					"type": "boolean",
					"description": "Show detailed information (size, time)"
				},
				"recursive": {
					"type": "boolean",
					"description": "List subdirectories recursively"
				},
				"max_depth": {
					"type": "integer",
					"description": "Maximum recursion depth (default: 3)"
				}
			},
			"required": ["path"]
		}`),
	}
}

// Execute lists directory contents
func (t *LsTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p LsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Resolve path
	targetPath := p.Path
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(t.RootDir, targetPath)
	}
	targetPath = filepath.Clean(targetPath)

	// Security check
	if !strings.HasPrefix(targetPath, t.RootDir) && t.RootDir != "" {
		return nil, fmt.Errorf("access denied: path outside root directory")
	}

	// Check if path exists and is a directory
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("path not found: %s", p.Path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", p.Path)
	}

	// Set default max depth
	maxDepth := p.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	var entries []string
	count := 0

	if p.Recursive {
		entries, count = t.listRecursive(targetPath, p.All, p.Long, maxDepth, 0)
	} else {
		entries, count = t.listDirectory(targetPath, p.All, p.Long)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n", p.Path))
	if count > lsMaxEntries {
		sb.WriteString(fmt.Sprintf("(showing first %d of %d entries)\n", lsMaxEntries, count))
	}
	sb.WriteString("\n")
	sb.WriteString(strings.Join(entries, "\n"))

	return &tool.Result{
		Output: sb.String(),
		Title:  fmt.Sprintf("ls %s", p.Path),
		Metadata: map[string]any{
			"path":      p.Path,
			"count":     len(entries),
			"truncated": count > lsMaxEntries,
			"total":     count,
		},
	}, nil
}

func (t *LsTool) listDirectory(dir string, showAll, longFormat bool) ([]string, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("error reading directory: %v", err)}, 0
	}

	var results []string
	count := 0

	// Sort entries: directories first, then files
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless all is true
		if !showAll && strings.HasPrefix(name, ".") {
			continue
		}

		count++
		if count > lsMaxEntries {
			continue
		}

		if longFormat {
			info, err := entry.Info()
			if err != nil {
				results = append(results, fmt.Sprintf("%s (error)", name))
				continue
			}
			results = append(results, t.formatLong(name, info))
		} else {
			if entry.IsDir() {
				results = append(results, name+"/")
			} else {
				results = append(results, name)
			}
		}
	}

	return results, count
}

func (t *LsTool) listRecursive(dir string, showAll, longFormat bool, maxDepth, currentDepth int) ([]string, int) {
	if currentDepth >= maxDepth {
		return nil, 0
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}

	var results []string
	totalCount := 0

	// Sort entries
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return entries[i].Name() < entries[j].Name()
	})

	prefix := strings.Repeat("  ", currentDepth)

	for _, entry := range entries {
		name := entry.Name()

		if !showAll && strings.HasPrefix(name, ".") {
			continue
		}

		totalCount++
		if totalCount <= lsMaxEntries {
			if longFormat {
				info, _ := entry.Info()
				if info != nil {
					results = append(results, prefix+t.formatLong(name, info))
				} else {
					results = append(results, prefix+name)
				}
			} else {
				if entry.IsDir() {
					results = append(results, prefix+name+"/")
				} else {
					results = append(results, prefix+name)
				}
			}
		}

		if entry.IsDir() {
			subPath := filepath.Join(dir, name)
			subResults, subCount := t.listRecursive(subPath, showAll, longFormat, maxDepth, currentDepth+1)
			results = append(results, subResults...)
			totalCount += subCount
		}
	}

	return results, totalCount
}

func (t *LsTool) formatLong(name string, info os.FileInfo) string {
	// Format: mode size date name
	mode := info.Mode().String()
	size := t.formatSize(info.Size())
	modTime := info.ModTime().Format("Jan 02 15:04")

	if info.IsDir() {
		name += "/"
	}

	return fmt.Sprintf("%s %8s %s %s", mode, size, modTime, name)
}

func (t *LsTool) formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1fG", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1fM", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1fK", float64(size)/KB)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

// Ensure LsTool implements Tool interface
var _ tool.Tool = (*LsTool)(nil)

// Suppress unused import warning
var _ = time.Now
