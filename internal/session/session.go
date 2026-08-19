// Package session provides session management functionality.
package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// MessageRole represents the role of a message sender.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	ID          string         `json:"id"`
	Role        MessageRole    `json:"role"`
	Content     string         `json:"content"`
	Tokens      int            `json:"tokens,omitempty"`
	ToolCalls   []ToolCall     `json:"tool_calls,omitempty"`
	ToolResults []ToolResult   `json:"tool_results,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// Session represents a conversation session.
type Session struct {
	mu sync.RWMutex

	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Workspace   string         `json:"workspace"`
	Messages    []Message      `json:"messages"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	ParentID    string         `json:"parent_id,omitempty"` // For forked sessions
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	TotalTokens int            `json:"total_tokens"`
}

// NewSession creates a new session.
func NewSession(id, name, workspace string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Name:      name,
		Workspace: workspace,
		Messages:  []Message{},
		Metadata:  make(map[string]any),
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ID == "" {
		msg.ID = generateMessageID(msg.Content, len(s.Messages))
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	s.Messages = append(s.Messages, msg)
	s.TotalTokens += msg.Tokens
	s.UpdatedAt = time.Now()
}

// GetMessages returns all messages.
func (s *Session) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Message, len(s.Messages))
	copy(result, s.Messages)
	return result
}

// GetRecentMessages returns the last n messages.
func (s *Session) GetRecentMessages(n int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n >= len(s.Messages) {
		result := make([]Message, len(s.Messages))
		copy(result, s.Messages)
		return result
	}

	start := len(s.Messages) - n
	result := make([]Message, n)
	copy(result, s.Messages[start:])
	return result
}

// Fork creates a fork of the session from a specific message.
func (s *Session) Fork(newID, newName string, fromMessageIdx int) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	fork := &Session{
		ID:        newID,
		Name:      newName,
		Workspace: s.Workspace,
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]any),
		Tags:      make([]string, len(s.Tags)),
		ParentID:  s.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	copy(fork.Tags, s.Tags)

	// Copy messages up to the specified index
	if fromMessageIdx > len(s.Messages) {
		fromMessageIdx = len(s.Messages)
	}
	fork.Messages = make([]Message, fromMessageIdx)
	copy(fork.Messages, s.Messages[:fromMessageIdx])

	// Recalculate tokens
	for _, msg := range fork.Messages {
		fork.TotalTokens += msg.Tokens
	}

	return fork
}

// ToJSON exports the session as JSON.
func (s *Session) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s, "", "  ")
}

// FromJSON imports a session from JSON.
func FromJSON(data []byte) (*Session, error) {
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// generateMessageID creates a unique message ID.
func generateMessageID(content string, index int) string {
	data := content + string(rune(index)) + time.Now().String()
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// Compaction strategies

// CompactionStrategy defines how to compact messages.
type CompactionStrategy int

const (
	// CompactByRemovingOld removes oldest messages keeping recent ones.
	CompactByRemovingOld CompactionStrategy = iota
	// CompactBySummarizing summarizes old messages into a system message.
	CompactBySummarizing
	// CompactByRemovingToolResults removes detailed tool results.
	CompactByRemovingToolResults
)

// CompactOptions defines options for compaction.
type CompactOptions struct {
	Strategy      CompactionStrategy
	KeepRecent    int    // Number of recent messages to keep
	TargetTokens  int    // Target token count after compaction
	SummaryPrompt string // Prompt for summarization
}

// DefaultCompactOptions returns sensible defaults.
func DefaultCompactOptions() CompactOptions {
	return CompactOptions{
		Strategy:     CompactByRemovingOld,
		KeepRecent:   10,
		TargetTokens: 4000,
	}
}

// Compact reduces the session size based on the strategy.
func (s *Session) Compact(ctx context.Context, opts CompactOptions, summarizer func(context.Context, string) (string, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch opts.Strategy {
	case CompactByRemovingOld:
		return s.compactByRemovingOld(opts)
	case CompactBySummarizing:
		return s.compactBySummarizing(ctx, opts, summarizer)
	case CompactByRemovingToolResults:
		return s.compactByRemovingToolResults(opts)
	default:
		return s.compactByRemovingOld(opts)
	}
}

func (s *Session) compactByRemovingOld(opts CompactOptions) error {
	if len(s.Messages) <= opts.KeepRecent {
		return nil
	}

	// Keep system messages and recent messages
	var systemMsgs []Message
	for _, msg := range s.Messages {
		if msg.Role == RoleSystem {
			systemMsgs = append(systemMsgs, msg)
		}
	}

	start := len(s.Messages) - opts.KeepRecent
	recentMsgs := s.Messages[start:]

	s.Messages = append(systemMsgs, recentMsgs...)
	s.recalculateTokens()
	s.UpdatedAt = time.Now()

	return nil
}

func (s *Session) compactBySummarizing(ctx context.Context, opts CompactOptions, summarizer func(context.Context, string) (string, error)) error {
	if summarizer == nil || len(s.Messages) <= opts.KeepRecent {
		return s.compactByRemovingOld(opts)
	}

	// Get messages to summarize
	toSummarize := len(s.Messages) - opts.KeepRecent
	if toSummarize <= 0 {
		return nil
	}

	// Build summary content
	var sb strings.Builder
	for _, msg := range s.Messages[:toSummarize] {
		sb.WriteString(string(msg.Role))
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}

	// Generate summary
	summary, err := summarizer(ctx, sb.String())
	if err != nil {
		return s.compactByRemovingOld(opts)
	}

	// Create summary message
	summaryMsg := Message{
		ID:        generateMessageID(summary, 0),
		Role:      RoleSystem,
		Content:   "Previous conversation summary:\n" + summary,
		CreatedAt: time.Now(),
	}

	// Keep recent messages
	start := len(s.Messages) - opts.KeepRecent
	recentMsgs := s.Messages[start:]

	s.Messages = append([]Message{summaryMsg}, recentMsgs...)
	s.Summary = summary
	s.recalculateTokens()
	s.UpdatedAt = time.Now()

	return nil
}

func (s *Session) compactByRemovingToolResults(opts CompactOptions) error {
	for i := range s.Messages {
		// Truncate long tool results
		for j := range s.Messages[i].ToolResults {
			if len(s.Messages[i].ToolResults[j].Content) > 500 {
				s.Messages[i].ToolResults[j].Content = s.Messages[i].ToolResults[j].Content[:500] + "... [truncated]"
			}
		}
	}

	s.recalculateTokens()
	s.UpdatedAt = time.Now()
	return nil
}

func (s *Session) recalculateTokens() {
	total := 0
	for _, msg := range s.Messages {
		total += msg.Tokens
	}
	s.TotalTokens = total
}

// sessionDoc is the on-disk representation of a Session. It mirrors the JSON
// fields exactly but carries no mutex, so a session can be snapshotted and
// serialized without copying its lock.
type sessionDoc struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Workspace   string         `json:"workspace"`
	Messages    []Message      `json:"messages"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	ParentID    string         `json:"parent_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	TotalTokens int            `json:"total_tokens"`
}

// doc returns a lock-free snapshot of the session for serialization.
func (s *Session) doc() sessionDoc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sessionDoc{
		ID:          s.ID,
		Name:        s.Name,
		Workspace:   s.Workspace,
		Messages:    s.Messages,
		Metadata:    s.Metadata,
		Summary:     s.Summary,
		Tags:        s.Tags,
		ParentID:    s.ParentID,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		TotalTokens: s.TotalTokens,
	}
}
