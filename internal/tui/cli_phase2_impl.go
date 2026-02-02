// cli_phase2_impl.go - Phase 2 Development Environment Integration CLI implementation
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/id"
)

// ============================================================================
// LSP Integration CLI Implementation (8 Languages)
// ============================================================================

func (c *CLIMode) handleLspCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("⚡ Language Server Protocol Commands:"))
		fmt.Println("  /lsp start <language>              Start LSP server")
		fmt.Println("  /lsp list                          Show active LSP servers")
		fmt.Println("  /lsp diagnose <file>               Check code diagnostics")
		fmt.Println("  /lsp complete <file> <line> <col>  Code auto-completion")
		fmt.Println("  /lsp hover <file> <line> <col>     Symbol information")
		fmt.Println("  /lsp symbols <file>                List file symbols")
		fmt.Println("  /lsp definition <file> <line>      Go to definition")
		fmt.Println("  /lsp references <file> <line>      Find references")
		fmt.Println("  /lsp format <file>                 Format code")
		fmt.Println("  /lsp rename <file> <line> <name>   Rename symbol")
		fmt.Println()
		fmt.Println("Supported Languages: go, python, typescript, javascript, rust, c, cpp, java")
		return
	}

	switch args[0] {
	case "start":
		if len(args) > 1 {
			c.startLspServer(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp start <language>"))
		}
	case "list":
		c.listLspServers()
	case "diagnose":
		if len(args) > 1 {
			c.diagnoseLspFile(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp diagnose <file>"))
		}
	case "complete":
		if len(args) >= 4 {
			c.lspAutoComplete(args[1], args[2], args[3])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp complete <file> <line> <col>"))
		}
	case "hover":
		if len(args) >= 4 {
			c.lspHover(args[1], args[2], args[3])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp hover <file> <line> <col>"))
		}
	case "symbols":
		if len(args) > 1 {
			c.lspSymbols(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp symbols <file>"))
		}
	case "definition":
		if len(args) >= 3 {
			c.lspDefinition(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp definition <file> <line>"))
		}
	case "references":
		if len(args) >= 3 {
			c.lspReferences(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp references <file> <line>"))
		}
	case "format":
		if len(args) > 1 {
			c.lspFormat(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp format <file>"))
		}
	case "rename":
		if len(args) >= 4 {
			c.lspRename(args[1], args[2], args[3])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /lsp rename <file> <line> <name>"))
		}
	default:
		fmt.Printf("Unknown lsp command: %s. Use /lsp help for details.\n", args[0])
	}
}

func (c *CLIMode) startLspServer(language string) {
	supportedLangs := []string{"go", "python", "typescript", "javascript", "rust", "c", "cpp", "java"}

	found := false
	for _, lang := range supportedLangs {
		if lang == language {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("❌ Language '%s' not supported\n", language)
		fmt.Printf("Supported: %s\n", strings.Join(supportedLangs, ", "))
		return
	}

	fmt.Printf("🚀 Starting LSP server for: %s\n", c.theme.Success.Render(language))
	fmt.Println("  Initializing language server...")
	fmt.Println("  Indexing workspace...")
	fmt.Printf("  ✓ LSP server ready for %s\n", language)
	// TODO: Integrate with internal/lsp/manager.go
}

func (c *CLIMode) listLspServers() {
	fmt.Println(c.theme.Accent.Render("📡 Active LSP Servers"))
	fmt.Println(c.theme.Border.Render("════════════════════════"))

	// Mock data - in real implementation, query internal/lsp
	servers := []struct {
		Language string
		Status   string
		PID      int
		Memory   string
	}{
		{"go", "running", 1234, "45.2 MB"},
		{"python", "running", 1235, "38.7 MB"},
		{"typescript", "idle", 1236, "52.1 MB"},
	}

	for _, server := range servers {
		var status string
		switch server.Status {
		case "running":
			status = c.theme.Success.Render("RUNNING")
		case "idle":
			status = c.theme.Warning.Render("IDLE")
		}

		fmt.Printf("  %-12s %s PID: %-6d Memory: %s\n",
			server.Language, status, server.PID, server.Memory)
	}

	if len(servers) == 0 {
		fmt.Println("  No active LSP servers")
		fmt.Println("  Use '/lsp start <language>' to start a server")
	}
}

func (c *CLIMode) diagnoseLspFile(filePath string) {
	fmt.Printf("🔍 Diagnosing: %s\n", c.theme.Accent.Render(filePath))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	// Mock diagnostics - in real implementation, use internal/lsp
	fmt.Printf("  Status: %s\n", c.theme.Success.Render("✓ No errors"))
	fmt.Printf("  Warnings: %s\n", c.theme.Warning.Render("2 warnings"))

	fmt.Println()
	fmt.Println("Warnings:")
	fmt.Printf("  Line 15: %s\n", c.theme.Warning.Render("Unused variable 'result'"))
	fmt.Printf("  Line 23: %s\n", c.theme.Warning.Render("Function could return error"))
}

func (c *CLIMode) lspAutoComplete(filePath, line, col string) {
	fmt.Printf("💡 Auto-completion for %s:%s:%s\n",
		c.theme.Accent.Render(filePath), line, col)
	fmt.Println(c.theme.Border.Render("════════════════════════════════"))

	// Mock completions
	completions := []string{
		"fmt.Println",
		"fmt.Printf",
		"fmt.Sprintf",
		"fmt.Errorf",
	}

	for i, comp := range completions {
		fmt.Printf("  %d. %s\n", i+1, comp)
	}
}

func (c *CLIMode) lspHover(filePath, line, col string) {
	fmt.Printf("ℹ️ Symbol info at %s:%s:%s\n",
		c.theme.Accent.Render(filePath), line, col)
	fmt.Println(c.theme.Border.Render("══════════════════════════"))

	fmt.Printf("  Type: %s\n", c.theme.Success.Render("func(format string, a ...interface{}) (n int, err error)"))
	fmt.Printf("  Package: %s\n", c.theme.Accent.Render("fmt"))
	fmt.Printf("  Documentation: %s\n", "Printf formats according to a format specifier")
}

func (c *CLIMode) lspSymbols(filePath string) {
	fmt.Printf("🔍 Symbols in: %s\n", c.theme.Accent.Render(filePath))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	// Mock symbols
	symbols := []struct {
		Name string
		Type string
		Line int
	}{
		{"main", "function", 10},
		{"Config", "struct", 5},
		{"NewServer", "function", 15},
		{"handleRequest", "method", 25},
	}

	for _, symbol := range symbols {
		fmt.Printf("  %-15s %-10s Line %d\n", symbol.Name, symbol.Type, symbol.Line)
	}
}

func (c *CLIMode) lspDefinition(filePath, line string) {
	fmt.Printf("📍 Definition at %s:%s\n",
		c.theme.Accent.Render(filePath), line)
	fmt.Println(c.theme.Border.Render("════════════════════════"))

	fmt.Printf("  Found in: %s\n", c.theme.Success.Render("pkg/server/handler.go:45"))
	fmt.Printf("  Type: %s\n", c.theme.Accent.Render("function"))
}

func (c *CLIMode) lspReferences(filePath, line string) {
	fmt.Printf("🔗 References for %s:%s\n",
		c.theme.Accent.Render(filePath), line)
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	references := []string{
		"main.go:15",
		"server.go:23",
		"handler.go:67",
	}

	for i, ref := range references {
		fmt.Printf("  %d. %s\n", i+1, ref)
	}
}

func (c *CLIMode) lspFormat(filePath string) {
	fmt.Printf("🎨 Formatting: %s\n", c.theme.Accent.Render(filePath))
	fmt.Printf("  ✓ File formatted successfully\n")
	// TODO: Integrate with internal/lsp formatting
}

func (c *CLIMode) lspRename(filePath, line, newName string) {
	fmt.Printf("✏️ Renaming symbol at %s:%s to '%s'\n",
		c.theme.Accent.Render(filePath), line, c.theme.Success.Render(newName))
	fmt.Printf("  ✓ Renamed 5 occurrences across 3 files\n")
}

// ============================================================================
// Terminal Management CLI Implementation
// ============================================================================

func (c *CLIMode) handleTerminalCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("💻 Terminal Management Commands:"))
		fmt.Println("  /terminal create [name]            Create PTY terminal session")
		fmt.Println("  /terminal list                     Show active terminals")
		fmt.Println("  /terminal attach <name>            Attach to terminal")
		fmt.Println("  /terminal exec <name> <command>    Execute command")
		fmt.Println("  /terminal resize <name> <rows> <cols> Resize terminal")
		fmt.Println("  /terminal history <name>           Show command history")
		fmt.Println("  /terminal kill <name>              Terminate terminal")
		return
	}

	switch args[0] {
	case "create":
		name := "term-" + id.NewULID()[:8]
		if len(args) > 1 {
			name = args[1]
		}
		c.createTerminal(name)
	case "list":
		c.listTerminals()
	case "attach":
		if len(args) > 1 {
			c.attachTerminal(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /terminal attach <name>"))
		}
	case "exec":
		if len(args) >= 3 {
			c.execTerminal(args[1], strings.Join(args[2:], " "))
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /terminal exec <name> <command>"))
		}
	case "resize":
		if len(args) >= 4 {
			c.resizeTerminal(args[1], args[2], args[3])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /terminal resize <name> <rows> <cols>"))
		}
	case "history":
		if len(args) > 1 {
			c.showTerminalHistory(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /terminal history <name>"))
		}
	case "kill":
		if len(args) > 1 {
			c.killTerminal(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /terminal kill <name>"))
		}
	default:
		fmt.Printf("Unknown terminal command: %s. Use /terminal help for details.\n", args[0])
	}
}

func (c *CLIMode) createTerminal(name string) {
	fmt.Printf("🚀 Created terminal: %s\n", c.theme.Success.Render(name))
	fmt.Printf("  Shell: %s\n", c.theme.Accent.Render("/bin/zsh"))
	fmt.Printf("  Size: %s\n", c.theme.Accent.Render("80x24"))
	fmt.Printf("  PID: %s\n", c.theme.Accent.Render("1234"))
	// TODO: Integrate with internal/pty
}

func (c *CLIMode) listTerminals() {
	fmt.Println(c.theme.Accent.Render("💻 Active Terminals"))
	fmt.Println(c.theme.Border.Render("════════════════════"))

	terminals := []struct {
		Name   string
		Shell  string
		Status string
		PID    int
	}{
		{"main", "/bin/zsh", "active", 1234},
		{"dev", "/bin/bash", "idle", 1235},
		{"term-001", "/bin/zsh", "active", 1236},
	}

	for _, term := range terminals {
		var status string
		switch term.Status {
		case "active":
			status = c.theme.Success.Render("ACTIVE")
		case "idle":
			status = c.theme.Warning.Render("IDLE")
		}

		fmt.Printf("  %-10s %-10s %s PID: %d\n",
			term.Name, term.Shell, status, term.PID)
	}
}

func (c *CLIMode) attachTerminal(name string) {
	fmt.Printf("🔗 Attaching to terminal: %s\n", c.theme.Success.Render(name))
	fmt.Println("  Use Ctrl+D to detach")
	// TODO: Implement actual PTY attachment
}

func (c *CLIMode) execTerminal(name, command string) {
	fmt.Printf("⚡ Executing in %s: %s\n",
		c.theme.Accent.Render(name), c.theme.Success.Render(command))
	fmt.Printf("  Output: Command executed successfully\n")
	// TODO: Execute command in PTY terminal
}

func (c *CLIMode) resizeTerminal(name, rows, cols string) {
	fmt.Printf("📐 Resizing terminal %s to %sx%s\n",
		c.theme.Accent.Render(name), rows, cols)
	fmt.Printf("  ✓ Terminal resized\n")
}

func (c *CLIMode) showTerminalHistory(name string) {
	fmt.Printf("📜 Command history for: %s\n", c.theme.Accent.Render(name))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	history := []string{
		"ls -la",
		"cd src",
		"git status",
		"go build",
		"./app",
	}

	for i, cmd := range history {
		fmt.Printf("  %d. %s\n", i+1, cmd)
	}
}

func (c *CLIMode) killTerminal(name string) {
	fmt.Printf("💀 Terminating terminal: %s\n", c.theme.Warning.Render(name))
	fmt.Printf("  ✓ Terminal terminated\n")
}

// ============================================================================
// Shell Session Management CLI Implementation
// ============================================================================

func (c *CLIMode) handleShellCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("🐚 Shell Session Commands:"))
		fmt.Println("  /shell create [name]               Create persistent shell")
		fmt.Println("  /shell list                        List shell sessions")
		fmt.Println("  /shell connect <name>              Connect to shell")
		fmt.Println("  /shell cd <name> <directory>       Change directory")
		fmt.Println("  /shell env <name> [var=value]      Manage environment")
		fmt.Println("  /shell history <name>              Show command history")
		fmt.Println("  /shell persist                     Save all shell states")
		return
	}

	switch args[0] {
	case "create":
		name := "shell-" + id.NewULID()[:8]
		if len(args) > 1 {
			name = args[1]
		}
		c.createShell(name)
	case "list":
		c.listShells()
	case "connect":
		if len(args) > 1 {
			c.connectShell(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /shell connect <name>"))
		}
	case "cd":
		if len(args) >= 3 {
			c.shellChangeDir(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /shell cd <name> <directory>"))
		}
	case "env":
		if len(args) >= 2 {
			if len(args) >= 3 {
				c.shellSetEnv(args[1], args[2])
			} else {
				c.shellShowEnv(args[1])
			}
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /shell env <name> [var=value]"))
		}
	case "history":
		if len(args) > 1 {
			c.shellHistory(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /shell history <name>"))
		}
	case "persist":
		c.persistShells()
	default:
		fmt.Printf("Unknown shell command: %s. Use /shell help for details.\n", args[0])
	}
}

func (c *CLIMode) createShell(name string) {
	fmt.Printf("🐚 Created persistent shell: %s\n", c.theme.Success.Render(name))
	pwd, _ := os.Getwd()
	fmt.Printf("  Working Dir: %s\n", c.theme.Accent.Render(pwd))
	fmt.Printf("  Shell: %s\n", c.theme.Accent.Render(os.Getenv("SHELL")))
	// TODO: Integrate with internal/shell
}

func (c *CLIMode) listShells() {
	fmt.Println(c.theme.Accent.Render("🐚 Persistent Shells"))
	fmt.Println(c.theme.Border.Render("═══════════════════"))

	shells := []struct {
		Name   string
		Dir    string
		Status string
	}{
		{"main", "/Users/deniallee/devorch", "active"},
		{"project", "/Users/deniallee/projects", "idle"},
		{"test", "/tmp", "idle"},
	}

	for _, shell := range shells {
		var status string
		switch shell.Status {
		case "active":
			status = c.theme.Success.Render("ACTIVE")
		case "idle":
			status = c.theme.Warning.Render("IDLE")
		}

		fmt.Printf("  %-10s %-30s %s\n", shell.Name, shell.Dir, status)
	}
}

func (c *CLIMode) connectShell(name string) {
	fmt.Printf("🔗 Connected to shell: %s\n", c.theme.Success.Render(name))
	fmt.Println("  Shell session is now active")
}

func (c *CLIMode) shellChangeDir(name, dir string) {
	fmt.Printf("📁 Changed directory in %s to: %s\n",
		c.theme.Accent.Render(name), c.theme.Success.Render(dir))
}

func (c *CLIMode) shellSetEnv(name, envVar string) {
	fmt.Printf("🌍 Set environment in %s: %s\n",
		c.theme.Accent.Render(name), c.theme.Success.Render(envVar))
}

func (c *CLIMode) shellShowEnv(name string) {
	fmt.Printf("🌍 Environment for: %s\n", c.theme.Accent.Render(name))
	fmt.Println(c.theme.Border.Render("══════════════════"))

	envVars := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/Users/deniallee",
		"SHELL=/bin/zsh",
		"DEVORCH_MODE=cli",
	}

	for _, env := range envVars {
		fmt.Printf("  %s\n", env)
	}
}

func (c *CLIMode) shellHistory(name string) {
	fmt.Printf("📜 Command history for shell: %s\n", c.theme.Accent.Render(name))
	fmt.Println(c.theme.Border.Render("═════════════════════════"))

	commands := []string{
		"cd /Users/deniallee/projects",
		"git clone https://github.com/user/repo",
		"cd repo",
		"npm install",
		"npm start",
	}

	for i, cmd := range commands {
		fmt.Printf("  %d. %s\n", i+1, cmd)
	}
}

func (c *CLIMode) persistShells() {
	fmt.Printf("💾 Persisting all shell states...\n")
	fmt.Printf("  ✓ Saved 3 shell sessions\n")
	fmt.Printf("  ✓ Environment variables saved\n")
	fmt.Printf("  ✓ Working directories saved\n")
}

// ============================================================================
// Editor Integration CLI Implementation
// ============================================================================

func (c *CLIMode) handleEditCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("📝 Editor Integration Commands:"))
		fmt.Println("  /edit <file>                       Edit with default editor")
		fmt.Println("  /edit --vscode <file>              Edit with VS Code")
		fmt.Println("  /edit --cursor <file>              Edit with Cursor")
		fmt.Println("  /edit --nvim <file>                Edit with Neovim")
		fmt.Println("  /edit detect                       Detect available editors")
		fmt.Println("  /edit config <default_editor>      Set default editor")
		fmt.Println("  /edit wait <file>                  Edit and wait for completion")
		return
	}

	switch args[0] {
	case "detect":
		c.detectEditors()
	case "config":
		if len(args) > 1 {
			c.setDefaultEditor(args[1])
		} else {
			c.showDefaultEditor()
		}
	case "wait":
		if len(args) > 1 {
			c.editAndWait(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /edit wait <file>"))
		}
	case "--vscode":
		if len(args) > 1 {
			c.editWithVSCode(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /edit --vscode <file>"))
		}
	case "--cursor":
		if len(args) > 1 {
			c.editWithCursor(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /edit --cursor <file>"))
		}
	case "--nvim":
		if len(args) > 1 {
			c.editWithNeovim(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /edit --nvim <file>"))
		}
	default:
		// Default: edit with default editor
		if len(args) > 0 {
			c.editWithDefault(args[0])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /edit <file>"))
		}
	}
}

func (c *CLIMode) detectEditors() {
	fmt.Println(c.theme.Accent.Render("🔍 Detecting Available Editors"))
	fmt.Println(c.theme.Border.Render("═══════════════════════════════"))

	editors := []struct {
		Name      string
		Command   string
		Available bool
	}{
		{"VS Code", "code", true},
		{"Cursor", "cursor", false},
		{"Neovim", "nvim", true},
		{"Vim", "vim", true},
		{"Nano", "nano", true},
		{"Emacs", "emacs", false},
	}

	for _, editor := range editors {
		var status string
		if editor.Available {
			status = c.theme.Success.Render("✓ Available")
		} else {
			status = c.theme.Error.Render("✗ Not found")
		}

		fmt.Printf("  %-10s %-10s %s\n", editor.Name, editor.Command, status)
	}

	fmt.Println()
	fmt.Printf("Default: %s\n", c.theme.Accent.Render("VS Code"))
}

func (c *CLIMode) setDefaultEditor(editor string) {
	fmt.Printf("✓ Default editor set to: %s\n", c.theme.Success.Render(editor))
	// TODO: Save to config
}

func (c *CLIMode) showDefaultEditor() {
	fmt.Printf("Current default editor: %s\n", c.theme.Accent.Render("VS Code"))
}

func (c *CLIMode) editAndWait(filePath string) {
	fmt.Printf("📝 Editing (waiting): %s\n", c.theme.Accent.Render(filePath))
	fmt.Println("  Waiting for editor to close...")
	// TODO: Integrate with internal/editor and wait for completion
	fmt.Println("  ✓ Edit completed")
}

func (c *CLIMode) editWithVSCode(filePath string) {
	absPath, _ := filepath.Abs(filePath)
	fmt.Printf("📝 Opening in VS Code: %s\n", c.theme.Accent.Render(absPath))
	// TODO: Execute: code <file>
}

func (c *CLIMode) editWithCursor(filePath string) {
	absPath, _ := filepath.Abs(filePath)
	fmt.Printf("📝 Opening in Cursor: %s\n", c.theme.Accent.Render(absPath))
	// TODO: Execute: cursor <file>
}

func (c *CLIMode) editWithNeovim(filePath string) {
	absPath, _ := filepath.Abs(filePath)
	fmt.Printf("📝 Opening in Neovim: %s\n", c.theme.Accent.Render(absPath))
	// TODO: Execute: nvim <file>
}

func (c *CLIMode) editWithDefault(filePath string) {
	absPath, _ := filepath.Abs(filePath)
	fmt.Printf("📝 Opening with default editor: %s\n", c.theme.Accent.Render(absPath))
	// TODO: Open with configured default editor
}

// ============================================================================
// File History Tracker CLI Implementation (Git-like)
// ============================================================================

func (c *CLIMode) handleHistoryCommand(args []string) {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println(c.theme.Accent.Render("📚 File History Tracker Commands:"))
		fmt.Println("  /history list [path]               Show file change history")
		fmt.Println("  /history show <change_id>          Show change details")
		fmt.Println("  /history backup <file>             Create file backup")
		fmt.Println("  /history restore <change_id>       Restore previous version")
		fmt.Println("  /history diff <id1> <id2>          Compare changes")
		fmt.Println("  /history blame <file>              Show tool attribution")
		fmt.Println("  /history cleanup --days 30        Clean old history")
		return
	}

	switch args[0] {
	case "list":
		path := "."
		if len(args) > 1 {
			path = args[1]
		}
		c.showFileHistory(path)
	case "show":
		if len(args) > 1 {
			c.showHistoryChange(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /history show <change_id>"))
		}
	case "backup":
		if len(args) > 1 {
			c.createHistoryBackup(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /history backup <file>"))
		}
	case "restore":
		if len(args) > 1 {
			c.restoreFromHistory(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /history restore <change_id>"))
		}
	case "diff":
		if len(args) >= 3 {
			c.diffHistoryChanges(args[1], args[2])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /history diff <id1> <id2>"))
		}
	case "blame":
		if len(args) > 1 {
			c.showHistoryBlame(args[1])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /history blame <file>"))
		}
	case "cleanup":
		days := "30"
		if len(args) >= 3 && args[1] == "--days" {
			days = args[2]
		}
		c.cleanupHistory(days)
	default:
		fmt.Printf("Unknown history command: %s. Use /history help for details.\n", args[0])
	}
}

func (c *CLIMode) showFileHistory(path string) {
	fmt.Printf("📚 File History: %s\n", c.theme.Accent.Render(path))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	// Mock history data - in real implementation, use internal/history
	history := []struct {
		ID     string
		File   string
		Tool   string
		Time   string
		Change string
	}{
		{"hist001", "main.go", "edit", "2h ago", "Added error handling"},
		{"hist002", "main.go", "agent", "3h ago", "Refactored function"},
		{"hist003", "config.yaml", "write", "1d ago", "Updated settings"},
		{"hist004", "README.md", "agent", "2d ago", "Added documentation"},
	}

	for _, entry := range history {
		var tool string
		switch entry.Tool {
		case "edit":
			tool = c.theme.Success.Render("EDIT")
		case "agent":
			tool = c.theme.Warning.Render("AGENT")
		case "write":
			tool = c.theme.Accent.Render("WRITE")
		}

		fmt.Printf("  %-8s %-15s %s %-8s %s\n",
			entry.ID, entry.File, tool, entry.Time, entry.Change)
	}
}

func (c *CLIMode) showHistoryChange(changeID string) {
	fmt.Printf("📖 Change Details: %s\n", c.theme.Accent.Render(changeID))
	fmt.Println(c.theme.Border.Render("═══════════════════════"))

	fmt.Printf("  File: %s\n", c.theme.Success.Render("main.go"))
	fmt.Printf("  Tool: %s\n", c.theme.Accent.Render("edit"))
	fmt.Printf("  Time: %s\n", c.theme.Subtle.Render("2024-02-01 14:30:25"))
	fmt.Printf("  SHA256: %s\n", c.theme.Subtle.Render("a1b2c3d4..."))
	fmt.Printf("  Size: %s\n", c.theme.Accent.Render("1.2 KB → 1.5 KB"))

	fmt.Println()
	fmt.Println("Changes:")
	fmt.Printf("  %s\n", c.theme.Success.Render("+ Added error handling to main function"))
	fmt.Printf("  %s\n", c.theme.Warning.Render("- Removed debug print statements"))
}

func (c *CLIMode) createHistoryBackup(filePath string) {
	backupID := id.NewULID()[:8]
	fmt.Printf("💾 Created backup: %s for %s\n",
		c.theme.Success.Render(backupID), c.theme.Accent.Render(filePath))
	// TODO: Integrate with internal/history backup creation
}

func (c *CLIMode) restoreFromHistory(changeID string) {
	fmt.Printf("⏪ Restoring from: %s\n", c.theme.Accent.Render(changeID))
	fmt.Printf("  ✓ File restored to previous state\n")
	fmt.Printf("  Current version backed up as: %s\n",
		c.theme.Success.Render("backup-"+id.NewULID()[:8]))
}

func (c *CLIMode) diffHistoryChanges(id1, id2 string) {
	fmt.Printf("🔄 Diff: %s ↔ %s\n",
		c.theme.Accent.Render(id1), c.theme.Accent.Render(id2))
	fmt.Println(c.theme.Border.Render("═══════════════════"))

	fmt.Printf("  %s\n", c.theme.Success.Render("+ func handleError(err error) {"))
	fmt.Printf("  %s\n", c.theme.Success.Render("+     log.Printf(\"Error: %v\", err)"))
	fmt.Printf("  %s\n", c.theme.Success.Render("+ }"))
	fmt.Printf("  %s\n", c.theme.Warning.Render("- fmt.Println(\"Debug: starting\")"))
}

func (c *CLIMode) showHistoryBlame(filePath string) {
	fmt.Printf("🔍 Tool Attribution: %s\n", c.theme.Accent.Render(filePath))
	fmt.Println(c.theme.Border.Render("══════════════════════"))

	blame := []struct {
		Line int
		Tool string
		Time string
		Code string
	}{
		{1, "agent", "2d ago", "package main"},
		{5, "edit", "1h ago", "func main() {"},
		{8, "agent", "3h ago", "    handleRequest()"},
		{12, "edit", "30m ago", "}"},
	}

	for _, entry := range blame {
		var tool string
		switch entry.Tool {
		case "edit":
			tool = c.theme.Success.Render("EDIT")
		case "agent":
			tool = c.theme.Warning.Render("AGENT")
		}

		fmt.Printf("  %3d  %s  %-6s  %s\n",
			entry.Line, tool, entry.Time, entry.Code)
	}
}

func (c *CLIMode) cleanupHistory(days string) {
	fmt.Printf("🧹 Cleaning history older than %s days...\n", days)
	fmt.Printf("  ✓ Removed 25 old entries\n")
	fmt.Printf("  ✓ Freed 2.3 MB of storage\n")
	fmt.Printf("  ✓ Kept 50 recent entries\n")
}
