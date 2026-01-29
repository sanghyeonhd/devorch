package tool

import (
	"context"
)

// Context provides execution context for tools
type Context struct {
	// Standard Go context
	Ctx context.Context

	// Session information
	SessionID   string
	MessageID   string
	WorkspaceID string

	// Working directory for file operations
	WorkDir string

	// Abort signal for long-running operations
	Abort <-chan struct{}

	// Extra parameters passed from caller
	Extra map[string]any
}

// IsAborted returns true if the operation has been aborted
func (c *Context) IsAborted() bool {
	select {
	case <-c.Abort:
		return true
	default:
		return false
	}
}

// Value returns extra parameter by key
func (c *Context) Value(key string) any {
	if c.Extra == nil {
		return nil
	}
	return c.Extra[key]
}
