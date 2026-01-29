package builtins

import (
	"context"
	"log/slog"
	"strings"
)

// HookEventType is local alias to avoid import cycle
type HookEventType string

const (
	EventPostToolUse   HookEventType = "post_tool_use"
	EventMessageCreate HookEventType = "message_create"
)

// CommentCheckerHook detects and warns about AI slop patterns in generated code
// Inspired by oh-my-opencode's comment-checker hook
type CommentCheckerHook struct {
	// Patterns to detect (case insensitive)
	Patterns []string

	// Logger for warnings
	Logger *slog.Logger
}

// Name returns the hook name
func (h *CommentCheckerHook) Name() string {
	return "comment_checker"
}

// Events returns the events this hook handles
func (h *CommentCheckerHook) Events() []HookEventType {
	return []HookEventType{
		EventPostToolUse,
		EventMessageCreate,
	}
}

// PostToolUseData for post tool use event
type PostToolUseData struct {
	SessionID string
	ToolName  string
	Output    any
}

// MessageCreateData for message create event
type MessageCreateData struct {
	Role    string
	Content string
}

// Execute checks for AI slop patterns
func (h *CommentCheckerHook) Execute(ctx context.Context, event HookEventType, data any) (any, error) {
	patterns := h.getPatterns()

	switch event {
	case EventPostToolUse:
		if d, ok := data.(*PostToolUseData); ok {
			if d.ToolName == "write" || d.ToolName == "edit" {
				// Check output for patterns
				if output, ok := d.Output.(string); ok {
					h.checkPatterns(output, patterns)
				}
			}
		}
	case EventMessageCreate:
		if d, ok := data.(*MessageCreateData); ok {
			if d.Role == "assistant" {
				h.checkPatterns(d.Content, patterns)
			}
		}
	}

	return data, nil
}

// getPatterns returns patterns to check
func (h *CommentCheckerHook) getPatterns() []string {
	if len(h.Patterns) > 0 {
		return h.Patterns
	}

	// Default AI slop patterns
	return []string{
		"// TODO: implement",
		"// add your code here",
		"// implementation goes here",
		"// your code here",
		"/* your code here */",
		"# TODO: implement",
		"# your code here",
		"pass  # implement",
		"raise NotImplementedError",
		"throw new Error('Not implemented')",
		"// ... (existing code)",
		"// existing code remains",
		"<!-- your content here -->",
	}
}

// checkPatterns checks content for patterns and logs warnings
func (h *CommentCheckerHook) checkPatterns(content string, patterns []string) {
	lower := strings.ToLower(content)

	for _, pattern := range patterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			if h.Logger != nil {
				h.Logger.Warn("AI slop pattern detected",
					slog.String("pattern", pattern),
				)
			}
		}
	}
}

// NewCommentCheckerHook creates a new comment checker hook
func NewCommentCheckerHook(logger *slog.Logger) *CommentCheckerHook {
	return &CommentCheckerHook{
		Logger: logger,
	}
}
