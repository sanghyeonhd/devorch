// Package compaction provides session compaction/summarization
package compaction

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devorch/internal/provider"
)

// Message represents a conversation message
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Tokens    int       `json:"tokens,omitempty"`
}

// Summary represents a compacted summary of messages
type Summary struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	MessageCount int       `json:"message_count"`
	TokensSaved  int       `json:"tokens_saved"`
	CreatedAt    time.Time `json:"created_at"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	KeyTopics    []string  `json:"key_topics,omitempty"`
}

// Session represents a conversation session
type Session struct {
	ID              string    `json:"id"`
	Messages        []Message `json:"messages"`
	Summaries       []Summary `json:"summaries"`
	CreatedAt       time.Time `json:"created_at"`
	LastActivity    time.Time `json:"last_activity"`
	TotalTokens     int       `json:"total_tokens"`
	CompactedTokens int       `json:"compacted_tokens"`
}

// Compactor handles session compaction
type Compactor struct {
	provider         provider.Provider
	maxTokens        int
	compactThreshold int
	summaryRatio     float64
	savePath         string
}

// Config represents compactor configuration
type Config struct {
	MaxTokens        int     `json:"max_tokens"`
	CompactThreshold int     `json:"compact_threshold"` // Tokens before compaction
	SummaryRatio     float64 `json:"summary_ratio"`     // Target compression ratio
	SavePath         string  `json:"save_path"`
}

// NewCompactor creates a new session compactor
func NewCompactor(p provider.Provider, cfg Config) *Compactor {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 128000 // Default context window
	}
	if cfg.CompactThreshold <= 0 {
		cfg.CompactThreshold = 64000 // Compact at 50% of max
	}
	if cfg.SummaryRatio <= 0 {
		cfg.SummaryRatio = 0.25 // 4:1 compression
	}
	if cfg.SavePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.SavePath = filepath.Join(homeDir, ".devorch", "sessions")
	}

	os.MkdirAll(cfg.SavePath, 0755)

	return &Compactor{
		provider:         p,
		maxTokens:        cfg.MaxTokens,
		compactThreshold: cfg.CompactThreshold,
		summaryRatio:     cfg.SummaryRatio,
		savePath:         cfg.SavePath,
	}
}

// ShouldCompact checks if the session should be compacted
func (c *Compactor) ShouldCompact(session *Session) bool {
	return session.TotalTokens >= c.compactThreshold
}

// Compact compacts a session by summarizing older messages
func (c *Compactor) Compact(ctx context.Context, session *Session) error {
	if !c.ShouldCompact(session) {
		return nil
	}

	// Determine how many messages to compact
	// Keep recent messages, compact older ones
	messagesToCompact := c.selectMessagesToCompact(session)
	if len(messagesToCompact) < 4 {
		return nil // Not enough messages to compact
	}

	// Generate summary
	summary, err := c.generateSummary(ctx, messagesToCompact)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Calculate tokens saved
	compactedTokens := 0
	for _, msg := range messagesToCompact {
		compactedTokens += msg.Tokens
	}
	summary.TokensSaved = compactedTokens - estimateTokens(summary.Content)

	// Update session
	session.Summaries = append(session.Summaries, *summary)
	session.CompactedTokens += summary.TokensSaved

	// Remove compacted messages
	session.Messages = c.removeCompactedMessages(session.Messages, messagesToCompact)

	// Recalculate total tokens
	session.TotalTokens = c.calculateTotalTokens(session)

	return nil
}

func (c *Compactor) selectMessagesToCompact(session *Session) []Message {
	// Keep the last few messages (recent context)
	keepRecent := 10 // Keep last 10 messages

	if len(session.Messages) <= keepRecent {
		return nil
	}

	// Compact older messages
	return session.Messages[:len(session.Messages)-keepRecent]
}

func (c *Compactor) generateSummary(ctx context.Context, messages []Message) (*Summary, error) {
	// Build conversation text
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content))
	}

	prompt := fmt.Sprintf(`Summarize the following conversation, preserving:
1. Key decisions and conclusions
2. Important code changes or file modifications
3. Errors encountered and their resolutions
4. Outstanding tasks or questions
5. Technical details that may be needed for context

Conversation:
%s

Provide a concise but comprehensive summary that captures the essential context needed to continue the conversation.`, sb.String())

	resp, err := c.provider.Chat(ctx, provider.ChatRequest{
		Messages: []provider.ChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   int(float64(len(messages)*50) * c.summaryRatio), // Estimate
		Temperature: 0.3,                                             // More deterministic
	})
	if err != nil {
		return nil, err
	}

	// Extract key topics
	topics := extractKeyTopics(resp.Text())

	summary := &Summary{
		ID:           generateSummaryID(),
		Content:      resp.Text(),
		MessageCount: len(messages),
		CreatedAt:    time.Now(),
		StartTime:    messages[0].Timestamp,
		EndTime:      messages[len(messages)-1].Timestamp,
		KeyTopics:    topics,
	}

	return summary, nil
}

func (c *Compactor) removeCompactedMessages(messages, toRemove []Message) []Message {
	removeMap := make(map[time.Time]bool)
	for _, msg := range toRemove {
		removeMap[msg.Timestamp] = true
	}

	var result []Message
	for _, msg := range messages {
		if !removeMap[msg.Timestamp] {
			result = append(result, msg)
		}
	}

	return result
}

func (c *Compactor) calculateTotalTokens(session *Session) int {
	total := 0

	// Add tokens from summaries
	for _, s := range session.Summaries {
		total += estimateTokens(s.Content)
	}

	// Add tokens from messages
	for _, m := range session.Messages {
		if m.Tokens > 0 {
			total += m.Tokens
		} else {
			total += estimateTokens(m.Content)
		}
	}

	return total
}

// GetContext builds the context for a session including summaries
func (c *Compactor) GetContext(session *Session) []provider.ChatMessage {
	var messages []provider.ChatMessage

	// Add summaries first
	if len(session.Summaries) > 0 {
		var summaryContent strings.Builder
		summaryContent.WriteString("## Previous Conversation Summary\n\n")

		for i, s := range session.Summaries {
			if i > 0 {
				summaryContent.WriteString("\n---\n\n")
			}
			summaryContent.WriteString(fmt.Sprintf("### Summary %d (%s - %s)\n\n",
				i+1, s.StartTime.Format("15:04"), s.EndTime.Format("15:04")))
			summaryContent.WriteString(s.Content)
			summaryContent.WriteString("\n")
		}

		messages = append(messages, provider.ChatMessage{
			Role:    "system",
			Content: summaryContent.String(),
		})
	}

	// Add current messages
	for _, m := range session.Messages {
		messages = append(messages, provider.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return messages
}

// SaveSession saves a session to disk
func (c *Compactor) SaveSession(session *Session) error {
	path := filepath.Join(c.savePath, session.ID+".json")

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadSession loads a session from disk
func (c *Compactor) LoadSession(id string) (*Session, error) {
	path := filepath.Join(c.savePath, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// ListSessions lists all saved sessions
func (c *Compactor) ListSessions() ([]Session, error) {
	entries, err := os.ReadDir(c.savePath)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".json")
		session, err := c.LoadSession(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, *session)
	}

	return sessions, nil
}

// NewSession creates a new session
func NewSession() *Session {
	now := time.Now()
	return &Session{
		ID:           fmt.Sprintf("session_%d", now.UnixNano()),
		Messages:     make([]Message, 0),
		Summaries:    make([]Summary, 0),
		CreatedAt:    now,
		LastActivity: now,
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(role, content string) {
	tokens := estimateTokens(content)
	s.Messages = append(s.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Tokens:    tokens,
	})
	s.TotalTokens += tokens
	s.LastActivity = time.Now()
}

func estimateTokens(text string) int {
	// Rough estimation: ~4 characters per token
	return len(text) / 4
}

func extractKeyTopics(summary string) []string {
	// Simple keyword extraction
	keywords := make(map[string]int)

	// Common programming terms to look for
	terms := []string{
		"implementation", "function", "class", "error", "bug", "fix",
		"feature", "refactor", "test", "api", "database", "file",
		"config", "deploy", "build", "dependency", "module", "package",
	}

	lower := strings.ToLower(summary)
	for _, term := range terms {
		if strings.Contains(lower, term) {
			keywords[term]++
		}
	}

	var topics []string
	for k := range keywords {
		topics = append(topics, k)
	}

	// Limit to top 5
	if len(topics) > 5 {
		topics = topics[:5]
	}

	return topics
}

func generateSummaryID() string {
	return fmt.Sprintf("summary_%d", time.Now().UnixNano())
}
