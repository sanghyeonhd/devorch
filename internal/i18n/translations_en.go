package i18n

// English translations
var englishTranslations = map[string]string{
	// General
	"app.name":        "DevOrch",
	"app.description": "AI-powered development orchestration tool",
	"app.version":     "Version %s",

	// Commands
	"cmd.help":    "Show help",
	"cmd.version": "Show version",
	"cmd.config":  "Configure settings",
	"cmd.chat":    "Start chat session",
	"cmd.run":     "Run a task",
	"cmd.init":    "Initialize project",

	// Chat
	"chat.welcome":     "Welcome to DevOrch! Type your message or /help for commands.",
	"chat.thinking":    "Thinking...",
	"chat.typing":      "Typing...",
	"chat.error":       "Error: %s",
	"chat.exit":        "Goodbye!",
	"chat.clear":       "Chat cleared.",
	"chat.saved":       "Chat saved to %s",
	"chat.loaded":      "Chat loaded from %s",
	"chat.new_session": "Starting new session...",

	// Tools
	"tool.executing":  "Executing %s...",
	"tool.completed":  "Completed %s",
	"tool.failed":     "Failed: %s",
	"tool.permission": "Permission required for %s",
	"tool.approve":    "Approve",
	"tool.deny":       "Deny",
	"tool.always":     "Always allow",

	// Files
	"file.reading":  "Reading %s...",
	"file.writing":  "Writing %s...",
	"file.creating": "Creating %s...",
	"file.deleting": "Deleting %s...",
	"file.modified": "File modified: %s",
	"file.created":  "File created: %s",
	"file.deleted":  "File deleted: %s",
	"file.notfound": "File not found: %s",

	// Git
	"git.staging":    "Staging changes...",
	"git.committing": "Committing...",
	"git.pushing":    "Pushing...",
	"git.pulling":    "Pulling...",
	"git.status":     "Git status",
	"git.diff":       "Git diff",

	// Providers
	"provider.connecting": "Connecting to %s...",
	"provider.connected":  "Connected to %s",
	"provider.error":      "Provider error: %s",
	"provider.ratelimit":  "Rate limit reached. Waiting...",
	"provider.retry":      "Retrying in %d seconds...",

	// Settings
	"settings.saved":   "Settings saved",
	"settings.loaded":  "Settings loaded",
	"settings.reset":   "Settings reset to defaults",
	"settings.invalid": "Invalid setting: %s",

	// Errors
	"error.generic":     "An error occurred: %s",
	"error.network":     "Network error: %s",
	"error.permission":  "Permission denied: %s",
	"error.notfound":    "Not found: %s",
	"error.invalid":     "Invalid input: %s",
	"error.timeout":     "Operation timed out",
	"error.interrupted": "Operation interrupted",

	// Confirmations
	"confirm.yes":     "Yes",
	"confirm.no":      "No",
	"confirm.cancel":  "Cancel",
	"confirm.proceed": "Do you want to proceed?",
	"confirm.delete":  "Are you sure you want to delete %s?",
	"confirm.reset":   "Are you sure you want to reset?",

	// Status
	"status.ready":      "Ready",
	"status.busy":       "Busy",
	"status.waiting":    "Waiting...",
	"status.processing": "Processing...",
	"status.complete":   "Complete",
	"status.failed":     "Failed",

	// TUI
	"tui.input.placeholder": "Type a message...",
	"tui.sidebar.chats":     "Chats",
	"tui.sidebar.tools":     "Tools",
	"tui.sidebar.settings":  "Settings",
	"tui.help.title":        "Keyboard Shortcuts",
	"tui.help.quit":         "Quit",
	"tui.help.send":         "Send message",
	"tui.help.newline":      "New line",
	"tui.help.clear":        "Clear chat",
	"tui.help.copy":         "Copy last response",

	// Web UI
	"web.title":      "DevOrch Web UI",
	"web.connect":    "Connect",
	"web.disconnect": "Disconnect",
	"web.settings":   "Settings",
	"web.history":    "History",
	"web.new_chat":   "New Chat",
	"web.export":     "Export",
	"web.import":     "Import",

	// Memory
	"memory.title":    "Project Memory",
	"memory.saved":    "Memory saved",
	"memory.loaded":   "Memory loaded",
	"memory.cleared":  "Memory cleared",
	"memory.notfound": "No memory found",

	// Session
	"session.new":       "New session created",
	"session.restored":  "Session restored",
	"session.compacted": "Session compacted: %d messages",
	"session.expired":   "Session expired",
}
