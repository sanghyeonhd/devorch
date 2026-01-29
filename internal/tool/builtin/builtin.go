// Package builtin provides built-in tools for AI agents
package builtin

import (
	"os"

	"devorch/internal/tool"
)

// Singleton instances for stateful tools
var (
	taskTool        *TaskTool
	todoTool        *TodoTool
	planTool        *PlanTool
	agentTool       *AgentTool
	diagnosticsTool *DiagnosticsTool
	questionTool    *QuestionTool
	skillTool       *SkillTool
	codeSearchTool  *CodeSearchTool
	lspTool         *LSPTool
)

func init() {
	taskTool = NewTaskTool()
	todoTool = NewTodoTool()
	planTool = NewPlanTool()
	diagnosticsTool = NewDiagnosticsTool()
	questionTool = NewQuestionTool()
	skillTool = NewSkillTool()
	codeSearchTool = NewCodeSearchTool()
	// lspTool is initialized in DefaultTools with rootDir
}

// DefaultTools returns all built-in tools with default configuration
func DefaultTools(rootDir string) []tool.Tool {
	// Create read-only tools for agent sub-tool
	readOnlyTools := []tool.Tool{
		&GlobTool{RootDir: rootDir},
		&GrepTool{RootDir: rootDir},
		&LsTool{RootDir: rootDir},
		NewViewTool(),
		&SourcegraphTool{},
	}

	// Initialize agent tool with limited access
	agentTool = NewAgentTool(readOnlyTools)

	// Initialize LSP tool with rootDir
	lspTool = NewLSPTool(rootDir)

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

		// View tool (enhanced read with line numbers)
		NewViewTool(),

		// Patch tool (unified diff application)
		NewPatchTool(),

		// Web tools
		&WebFetchTool{},
		&WebSearchTool{
			BraveAPIKey: os.Getenv("BRAVE_API_KEY"),
			SerpAPIKey:  os.Getenv("SERP_API_KEY"),
		},

		// Fetch tool (generic web fetching)
		&FetchTool{},

		// Code search tools
		&SourcegraphTool{},

		// Task management tools (stateful singletons)
		taskTool,
		todoTool,
		planTool,

		// Agent sub-tool
		agentTool,

		// Diagnostics tool (LSP)
		diagnosticsTool,

		// Interactive question tool
		questionTool,

		// Skill invocation tool
		skillTool,

		// Code search tool
		codeSearchTool,

		// LSP operations tool
		lspTool,
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
		"view",
		"patch",
		// Web
		"webfetch",
		"websearch",
		"fetch",
		// Code search
		"sourcegraph",
		"codesearch",
		// Task management
		"task",
		"todo",
		"plan",
		// Agent
		"agent",
		// Diagnostics
		"diagnostics",
		// Interactive
		"question",
		// Skills
		"skill",
		// LSP
		"lsp",
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

// GetAgentTool returns the singleton agent tool instance
func GetAgentTool() *AgentTool {
	return agentTool
}

// GetDiagnosticsTool returns the singleton diagnostics tool instance
func GetDiagnosticsTool() *DiagnosticsTool {
	return diagnosticsTool
}

// GetQuestionTool returns the singleton question tool instance
func GetQuestionTool() *QuestionTool {
	return questionTool
}

// GetSkillTool returns the singleton skill tool instance
func GetSkillTool() *SkillTool {
	return skillTool
}

// GetCodeSearchTool returns the singleton code search tool instance
func GetCodeSearchTool() *CodeSearchTool {
	return codeSearchTool
}

// GetLSPTool returns the singleton LSP tool instance
func GetLSPTool() *LSPTool {
	return lspTool
}

// CreateBatchTool creates a batch tool with the given registry
func CreateBatchTool(registry tool.Registry) *BatchTool {
	return NewBatchTool(registry)
}
