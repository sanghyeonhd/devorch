// Package session provides session management.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store manages session persistence.
type Store struct {
	mu       sync.RWMutex
	dir      string
	sessions map[string]*Session
}

// NewStore creates a new session store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	s := &Store{
		dir:      dir,
		sessions: make(map[string]*Session),
	}

	if err := s.loadAll(); err != nil {
		return nil, err
	}

	return s, nil
}

// loadAll loads all sessions from disk.
func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		session, err := FromJSON(data)
		if err != nil {
			continue // Skip invalid files
		}

		s.sessions[session.ID] = session
	}

	return nil
}

// Create creates a new session.
func (s *Store) Create(ctx context.Context, id, name, workspace string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; exists {
		return nil, fmt.Errorf("session already exists: %s", id)
	}

	session := NewSession(id, name, workspace)
	s.sessions[id] = session

	if err := s.saveSession(session); err != nil {
		delete(s.sessions, id)
		return nil, err
	}

	return session, nil
}

// Get retrieves a session by ID.
func (s *Store) Get(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// Update updates a session.
func (s *Store) Update(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[session.ID]; !ok {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	session.UpdatedAt = time.Now()
	s.sessions[session.ID] = session

	return s.saveSession(session)
}

// Delete removes a session.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	delete(s.sessions, id)

	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// List returns all sessions for a workspace.
func (s *Store) List(ctx context.Context, workspace string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Session
	for _, session := range s.sessions {
		if workspace == "" || session.Workspace == workspace {
			result = append(result, session)
		}
	}

	// Sort by updated time, newest first
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result, nil
}

// Fork creates a fork of an existing session.
func (s *Store) Fork(ctx context.Context, sourceID, newID, newName string, fromMessageIdx int) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, ok := s.sessions[sourceID]
	if !ok {
		return nil, fmt.Errorf("source session not found: %s", sourceID)
	}

	if _, exists := s.sessions[newID]; exists {
		return nil, fmt.Errorf("session already exists: %s", newID)
	}

	fork := source.Fork(newID, newName, fromMessageIdx)
	s.sessions[newID] = fork

	if err := s.saveSession(fork); err != nil {
		delete(s.sessions, newID)
		return nil, err
	}

	return fork, nil
}

// Search searches sessions by query.
func (s *Store) Search(ctx context.Context, workspace, query string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Session
	for _, session := range s.sessions {
		if workspace != "" && session.Workspace != workspace {
			continue
		}

		// Search in name, tags, and messages
		if matchesQuery(session, query) {
			result = append(result, session)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result, nil
}

func matchesQuery(session *Session, query string) bool {
	// Simple contains matching
	if contains(session.Name, query) {
		return true
	}

	for _, tag := range session.Tags {
		if contains(tag, query) {
			return true
		}
	}

	for _, msg := range session.Messages {
		if contains(msg.Content, query) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Export exports a session to a file.
func (s *Store) Export(ctx context.Context, id, format, path string) error {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	var data []byte
	var err error

	switch format {
	case "json":
		data, err = session.ToJSON()
	case "markdown":
		data, err = exportMarkdown(session)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Import imports a session from a file.
func (s *Store) Import(ctx context.Context, path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	session, err := FromJSON(data)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; exists {
		return nil, fmt.Errorf("session already exists: %s", session.ID)
	}

	s.sessions[session.ID] = session

	if err := s.saveSession(session); err != nil {
		delete(s.sessions, session.ID)
		return nil, err
	}

	return session, nil
}

// saveSession saves a session to disk.
func (s *Store) saveSession(session *Session) error {
	data, err := session.ToJSON()
	if err != nil {
		return err
	}

	path := filepath.Join(s.dir, session.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// exportMarkdown exports a session as Markdown.
func exportMarkdown(session *Session) ([]byte, error) {
	var sb stringBuilder

	sb.WriteString("# ")
	sb.WriteString(session.Name)
	sb.WriteString("\n\n")

	sb.WriteString("**Created:** ")
	sb.WriteString(session.CreatedAt.Format(time.RFC3339))
	sb.WriteString("\n")

	sb.WriteString("**Updated:** ")
	sb.WriteString(session.UpdatedAt.Format(time.RFC3339))
	sb.WriteString("\n\n")

	if len(session.Tags) > 0 {
		sb.WriteString("**Tags:** ")
		for i, tag := range session.Tags {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tag)
		}
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n\n")

	for _, msg := range session.Messages {
		switch msg.Role {
		case RoleUser:
			sb.WriteString("### 👤 User\n\n")
		case RoleAssistant:
			sb.WriteString("### 🤖 Assistant\n\n")
		case RoleSystem:
			sb.WriteString("### ⚙️ System\n\n")
		case RoleTool:
			sb.WriteString("### 🔧 Tool\n\n")
		}

		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")

		if len(msg.ToolCalls) > 0 {
			sb.WriteString("**Tool Calls:**\n")
			for _, tc := range msg.ToolCalls {
				sb.WriteString("- `")
				sb.WriteString(tc.Name)
				sb.WriteString("`: ")
				sb.WriteString(tc.Arguments)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.Bytes(), nil
}

type stringBuilder struct {
	data []byte
}

func (sb *stringBuilder) WriteString(s string) {
	sb.data = append(sb.data, s...)
}

func (sb *stringBuilder) Bytes() []byte {
	return sb.data
}

// SessionStats represents session statistics.
type SessionStats struct {
	TotalSessions         int       `json:"total_sessions"`
	TotalMessages         int       `json:"total_messages"`
	TotalTokens           int       `json:"total_tokens"`
	AvgMessagesPerSession float64   `json:"avg_messages_per_session"`
	OldestSession         time.Time `json:"oldest_session"`
	NewestSession         time.Time `json:"newest_session"`
}

// Stats returns statistics about all sessions.
func (s *Store) Stats(ctx context.Context, workspace string) (*SessionStats, error) {
	sessions, err := s.List(ctx, workspace)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return &SessionStats{}, nil
	}

	stats := &SessionStats{
		TotalSessions: len(sessions),
		OldestSession: sessions[len(sessions)-1].CreatedAt,
		NewestSession: sessions[0].CreatedAt,
	}

	for _, session := range sessions {
		stats.TotalMessages += len(session.Messages)
		stats.TotalTokens += session.TotalTokens
	}

	stats.AvgMessagesPerSession = float64(stats.TotalMessages) / float64(stats.TotalSessions)

	return stats, nil
}

// SessionSummary is a lightweight session summary.
type SessionSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Messages  int       `json:"messages"`
	Tokens    int       `json:"tokens"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListSummaries returns lightweight session summaries.
func (s *Store) ListSummaries(ctx context.Context, workspace string) ([]SessionSummary, error) {
	sessions, err := s.List(ctx, workspace)
	if err != nil {
		return nil, err
	}

	summaries := make([]SessionSummary, len(sessions))
	for i, session := range sessions {
		summaries[i] = SessionSummary{
			ID:        session.ID,
			Name:      session.Name,
			Messages:  len(session.Messages),
			Tokens:    session.TotalTokens,
			UpdatedAt: session.UpdatedAt,
		}
	}

	return summaries, nil
}

// AutoSave configuration
type AutoSaveConfig struct {
	Enabled  bool
	Interval time.Duration
}

// StartAutoSave starts automatic periodic saving.
func (s *Store) StartAutoSave(ctx context.Context, config AutoSaveConfig) {
	if !config.Enabled {
		return
	}

	go func() {
		ticker := time.NewTicker(config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.saveAllSessions()
			}
		}
	}()
}

func (s *Store) saveAllSessions() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		_ = s.saveSession(session)
	}
}

// Backup creates a backup of all sessions.
func (s *Store) Backup(ctx context.Context, path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backup := make(map[string]*Session)
	for id, session := range s.sessions {
		backup[id] = session
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Restore restores sessions from a backup.
func (s *Store) Restore(ctx context.Context, path string, overwrite bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var backup map[string]*Session
	if err := json.Unmarshal(data, &backup); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, session := range backup {
		if !overwrite {
			if _, exists := s.sessions[id]; exists {
				continue
			}
		}

		s.sessions[id] = session
		_ = s.saveSession(session)
	}

	return nil
}
