package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CLIWorkModeManager manages work modes in CLI interface
type CLIWorkModeManager struct {
	current     WorkMode
	context     map[string]interface{}
	promptMod   string
	fileContext []string
	agentSteps  []string
	planTasks   []string
	initialized bool
}

// NewCLIWorkModeManager creates a new work mode manager
func NewCLIWorkModeManager() *CLIWorkModeManager {
	return &CLIWorkModeManager{
		current:     WorkModeAsk,
		context:     make(map[string]interface{}),
		fileContext: []string{},
		agentSteps:  []string{},
		planTasks:   []string{},
		initialized: true,
	}
}

// SwitchWorkMode switches to a different work mode with proper context setup
func (wm *CLIWorkModeManager) SwitchWorkMode(mode WorkMode, cliMode *CLIMode) {
	wm.current = mode
	wm.promptMod = wm.getWorkModePromptModifier()

	// Clear previous mode context
	wm.clearModeContext()

	// Mode-specific initialization
	switch mode {
	case WorkModeAsk:
		wm.initAskMode(cliMode)
	case WorkModeEdit:
		wm.initEditMode(cliMode)
	case WorkModeAgent:
		wm.initAgentMode(cliMode)
	case WorkModePlan:
		wm.initPlanMode(cliMode)
	}

	// Display mode switch confirmation
	wm.showModeStatus(cliMode)
}

// GetCurrentMode returns the current work mode
func (wm *CLIWorkModeManager) GetCurrentMode() WorkMode {
	return wm.current
}

// GetPromptModifier returns the current mode's prompt modifier
func (wm *CLIWorkModeManager) GetPromptModifier() string {
	return wm.promptMod
}

// GetContextMap returns the context as a map for external use
func (wm *CLIWorkModeManager) GetContextMap() map[string]string {
	contextMap := make(map[string]string)
	for k, v := range wm.context {
		contextMap[k] = fmt.Sprintf("%v", v)
	}
	return contextMap
}

// LoadContext loads context from external source (for preset loading)
func (wm *CLIWorkModeManager) LoadContext(context map[string]string) {
	wm.context = make(map[string]interface{})
	for k, v := range context {
		wm.context[k] = v
	}
}

// === Mode-specific initialization methods ===

func (wm *CLIWorkModeManager) initAskMode(cliMode *CLIMode) {
	wm.context["mode_desc"] = "Quick questions and answers"
	wm.context["optimized_for"] = "fast_response"
}

func (wm *CLIWorkModeManager) initEditMode(cliMode *CLIMode) {
	wm.context["mode_desc"] = "Code editing with file context"
	wm.context["optimized_for"] = "code_analysis"

	// Auto-detect project files
	files := wm.detectProjectFiles()
	wm.fileContext = files
	wm.context["file_count"] = len(files)

	if len(files) > 0 {
		fmt.Printf("🔍 Auto-detected %d project files for context\n", len(files))
		wm.showFileContext(cliMode)
	} else {
		fmt.Println("📁 No files detected. Add files manually using /context add <file>")
	}
}

func (wm *CLIWorkModeManager) initAgentMode(cliMode *CLIMode) {
	wm.context["mode_desc"] = "Autonomous multi-step task execution"
	wm.context["optimized_for"] = "task_breakdown"
	wm.agentSteps = []string{}

	fmt.Println("🤖 Agent mode initialized - ready for multi-step task execution")
	fmt.Println("   Provide complex tasks, and I'll break them down into actionable steps")
}

func (wm *CLIWorkModeManager) initPlanMode(cliMode *CLIMode) {
	wm.context["mode_desc"] = "Task analysis and planning"
	wm.context["optimized_for"] = "strategic_planning"
	wm.planTasks = []string{}

	fmt.Println("📋 Plan mode initialized - ready for strategic task planning")
	fmt.Println("   Describe your project goals for detailed planning and analysis")
}

// === Helper methods ===

func (wm *CLIWorkModeManager) clearModeContext() {
	wm.context = make(map[string]interface{})
	wm.fileContext = []string{}
	wm.agentSteps = []string{}
	wm.planTasks = []string{}
}

func (wm *CLIWorkModeManager) getWorkModePromptModifier() string {
	switch wm.current {
	case WorkModeAsk:
		return "Quick Q&A mode: Provide concise, direct answers focused on immediate questions."
	case WorkModeEdit:
		return "Code editing mode: Focus on code analysis, file modifications, and technical implementation details."
	case WorkModeAgent:
		return "Agent mode: Break down complex tasks into actionable steps and execute them systematically."
	case WorkModePlan:
		return "Planning mode: Create detailed task breakdowns, analyze requirements, and develop strategic approaches."
	default:
		return ""
	}
}

func (wm *CLIWorkModeManager) showModeStatus(cliMode *CLIMode) {
	fmt.Printf("\n%s Switched to %s Mode\n", wm.current.Icon(), wm.current.String())
	fmt.Printf("   %s\n", wm.context["mode_desc"])

	// Show mode-specific status
	switch wm.current {
	case WorkModeEdit:
		if len(wm.fileContext) > 0 {
			fmt.Printf("   📁 %d files in context\n", len(wm.fileContext))
		}
	case WorkModeAgent:
		fmt.Printf("   🎯 Ready for complex task execution\n")
	case WorkModePlan:
		fmt.Printf("   📊 Ready for strategic planning\n")
	}
	fmt.Println()
}

func (wm *CLIWorkModeManager) detectProjectFiles() []string {
	files := []string{}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return files
	}

	// Check if we're in a git repository
	if wm.isInGitRepo(cwd) {
		// Get recently modified files from git
		gitFiles := wm.getRecentlyModifiedGitFiles(cwd)
		files = append(files, gitFiles...)
	} else {
		// Fall back to scanning common project files
		commonFiles := wm.scanCommonProjectFiles(cwd)
		files = append(files, commonFiles...)
	}

	// Limit to reasonable number of files
	if len(files) > 20 {
		files = files[:20]
	}

	return files
}

func (wm *CLIWorkModeManager) isInGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	if info, err := os.Stat(gitDir); err == nil {
		return info.IsDir()
	}
	return false
}

func (wm *CLIWorkModeManager) getRecentlyModifiedGitFiles(dir string) []string {
	// This is a simplified implementation
	// In a real scenario, you'd use git commands to get recently modified files
	files := []string{}

	// Common extensions for code files
	extensions := []string{".go", ".js", ".ts", ".py", ".java", ".cpp", ".c", ".h", ".md", ".json", ".yaml", ".yml"}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}

		// Skip hidden directories and files
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip vendor, node_modules, etc.
		if info.IsDir() {
			skipDirs := []string{"vendor", "node_modules", "build", "dist", ".git"}
			for _, skip := range skipDirs {
				if info.Name() == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file has relevant extension
		ext := filepath.Ext(info.Name())
		for _, validExt := range extensions {
			if ext == validExt {
				relPath, _ := filepath.Rel(dir, path)
				files = append(files, relPath)
				break
			}
		}

		return nil
	})

	if err != nil {
		return []string{}
	}

	return files
}

func (wm *CLIWorkModeManager) scanCommonProjectFiles(dir string) []string {
	files := []string{}

	// Look for common project files
	commonFiles := []string{
		"README.md", "readme.md", "README.txt",
		"main.go", "main.py", "main.js", "index.js", "app.js",
		"package.json", "go.mod", "requirements.txt", "Cargo.toml",
		"Dockerfile", "docker-compose.yml", "Makefile",
	}

	for _, fileName := range commonFiles {
		filePath := filepath.Join(dir, fileName)
		if _, err := os.Stat(filePath); err == nil {
			files = append(files, fileName)
		}
	}

	return files
}

func (wm *CLIWorkModeManager) showFileContext(cliMode *CLIMode) {
	fmt.Println("📁 Files in context:")
	for i, file := range wm.fileContext {
		if i < 10 { // Show first 10 files
			fmt.Printf("   • %s\n", file)
		} else if i == 10 {
			fmt.Printf("   ... and %d more files\n", len(wm.fileContext)-10)
			break
		}
	}

	if len(wm.fileContext) > 0 {
		fmt.Println("\n💡 Use /context add <file> to add more files")
		fmt.Println("   Use /context list to see all files")
	}
}

// GetFileContext returns the current file context
func (wm *CLIWorkModeManager) GetFileContext() []string {
	return wm.fileContext
}

// AddFileToContext adds a file to the current context
func (wm *CLIWorkModeManager) AddFileToContext(filePath string) {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("⚠️ File not found: %s\n", filePath)
		return
	}

	// Check if already in context
	for _, existing := range wm.fileContext {
		if existing == filePath {
			fmt.Printf("ℹ️ File already in context: %s\n", filePath)
			return
		}
	}

	wm.fileContext = append(wm.fileContext, filePath)
	fmt.Printf("✓ Added to context: %s\n", filePath)

	// Update context count
	wm.context["file_count"] = len(wm.fileContext)
}

// RemoveFileFromContext removes a file from the current context
func (wm *CLIWorkModeManager) RemoveFileFromContext(filePath string) {
	for i, existing := range wm.fileContext {
		if existing == filePath {
			wm.fileContext = append(wm.fileContext[:i], wm.fileContext[i+1:]...)
			fmt.Printf("✓ Removed from context: %s\n", filePath)
			wm.context["file_count"] = len(wm.fileContext)
			return
		}
	}
	fmt.Printf("⚠️ File not found in context: %s\n", filePath)
}

// AddAgentStep adds a step to agent execution
func (wm *CLIWorkModeManager) AddAgentStep(step string) {
	wm.agentSteps = append(wm.agentSteps, step)
	fmt.Printf("🤖 Agent Step %d: %s\n", len(wm.agentSteps), step)
}

// GetAgentSteps returns current agent steps
func (wm *CLIWorkModeManager) GetAgentSteps() []string {
	return wm.agentSteps
}

// AddPlanTask adds a task to the planning list
func (wm *CLIWorkModeManager) AddPlanTask(task string) {
	wm.planTasks = append(wm.planTasks, task)
	fmt.Printf("📋 Plan Task %d: %s\n", len(wm.planTasks), task)
}

// GetPlanTasks returns current plan tasks
func (wm *CLIWorkModeManager) GetPlanTasks() []string {
	return wm.planTasks
}

// BuildModeSpecificPrompt builds a mode-specific prompt enhancement
func (wm *CLIWorkModeManager) BuildModeSpecificPrompt(userInput string) string {
	base := userInput

	switch wm.current {
	case WorkModeEdit:
		if len(wm.fileContext) > 0 {
			base += fmt.Sprintf("\n\n[Context: %d files in scope - %v]",
				len(wm.fileContext), wm.fileContext[:min(3, len(wm.fileContext))])
		}
	case WorkModeAgent:
		if len(wm.agentSteps) > 0 {
			base += fmt.Sprintf("\n\n[Previous steps: %v]", wm.agentSteps)
		}
	case WorkModePlan:
		if len(wm.planTasks) > 0 {
			base += fmt.Sprintf("\n\n[Current plan: %v]", wm.planTasks)
		}
	}

	return base
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
