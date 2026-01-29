// Package builtin provides built-in tools for AI agents
package builtin

import (
	"devorch/internal/tool"
)

// DefaultTools returns all built-in tools with default configuration
func DefaultTools(rootDir string) []tool.Tool {
	return []tool.Tool{
		&BashTool{
			DefaultWorkDir: rootDir,
		},
		&ReadTool{
			RootDir:       rootDir,
			AllowExternal: false,
		},
		&WriteTool{
			RootDir:        rootDir,
			AllowExternal:  false,
			AllowOverwrite: true,
			CreateDirs:     true,
		},
		&EditTool{
			RootDir:       rootDir,
			AllowExternal: false,
		},
		&GlobTool{
			RootDir: rootDir,
		},
		&GrepTool{
			RootDir: rootDir,
		},
	}
}

// ToolNames returns names of all built-in tools
func ToolNames() []string {
	return []string{
		"bash",
		"read",
		"write",
		"edit",
		"glob",
		"grep",
	}
}
