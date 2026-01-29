// Package builtin provides built-in tools for AI agents
package builtin

import (
	"os"

	"devorch/internal/tool"
)

// Singleton instances for stateful tools
var (
	taskTool *TaskTool
	todoTool *TodoTool
	planTool *PlanTool
)

func init() {
	taskTool = NewTaskTool()
	todoTool = NewTodoTool()
	planTool = NewPlanTool()
}

// DefaultTools returns all built-in tools with default configuration
func DefaultTools(rootDir string) []tool.Tool {
	return []tool.Tool{
		// File system tools
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
		&LsTool{
			RootDir: rootDir,
		},
		&MultiEditTool{
			RootDir:       rootDir,
			AllowExternal: false,
		},

		// Web tools
		&WebFetchTool{},
		&WebSearchTool{
			BraveAPIKey: os.Getenv("BRAVE_API_KEY"),
			SerpAPIKey:  os.Getenv("SERP_API_KEY"),
		},

		// Task management tools (stateful singletons)
		taskTool,
		todoTool,
		planTool,
	}
}

// ToolNames returns names of all built-in tools
func ToolNames() []string {
	return []string{
		// File system
		"bash",
		"read",
		"write",
		"edit",
		"glob",
		"grep",
		"ls",
		"multiedit",
		// Web
		"webfetch",
		"websearch",
		// Task management
		"task",
		"todo",
		"plan",
	}
}

// GetTaskTool returns the singleton task tool instance
func GetTaskTool() *TaskTool {
	return taskTool
}

// GetTodoTool returns the singleton todo tool instance
func GetTodoTool() *TodoTool {
	return todoTool
}

// GetPlanTool returns the singleton plan tool instance
func GetPlanTool() *PlanTool {
	return planTool
}
