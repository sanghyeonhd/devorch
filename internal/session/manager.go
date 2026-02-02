// Package session provides session management functionality.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"devorch/internal/id"
)

// Manager manages multiple sessions and their persistence
type Manager struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	storePath     string
	activeSession *Session
}

// NewManager creates a new session manager
func NewManager(storePath string) (*Manager, error) {
	if storePath == "" {
		storePath = os.ExpandEnv("$HOME/.devorch/sessions")
	}

	// Ensure directory exists
	if err := os.MkdirAll(storePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session store directory: %w", err)
	}

	manager := &Manager{
		sessions:  make(map[string]*Session),
		storePath: storePath,
	}

	// Load existing sessions
	if err := manager.loadExistingSessions(); err != nil {
		return nil, fmt.Errorf("failed to load existing sessions: %w", err)
	}

	return manager, nil
}

// CreateSession creates a new session
func (m *Manager) CreateSession(name, description string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := id.NewULID()
	session := NewSession(sessionID, name, "")

	// Add description to metadata
	if description != "" {
		session.Metadata["description"] = description
	}

	m.sessions[sessionID] = session

	// Save to disk
	if err := m.saveSession(session); err != nil {
		delete(m.sessions, sessionID)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListSessions returns all sessions
func (m *Manager) ListSessions() ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteSession deletes a session
func (m *Manager) DeleteSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Remove from memory
	delete(m.sessions, sessionID)

	// Remove from disk
	sessionPath := filepath.Join(m.storePath, sessionID+".json")
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		// Add back to memory if disk removal failed
		m.sessions[sessionID] = session
		return fmt.Errorf("failed to remove session file: %w", err)
	}

	return nil
}

// GetActiveSession returns the currently active session
func (m *Manager) GetActiveSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeSession
}

// SetActiveSession sets the active session
func (m *Manager) SetActiveSession(session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeSession = session
}

// Shutdown gracefully shuts down the session manager
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Save all sessions
	var errors []error
	for _, session := range m.sessions {
		if err := m.saveSession(session); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to save some sessions during shutdown: %v", errors)
	}

	return nil
}

// saveSession saves a session to disk
func (m *Manager) saveSession(session *Session) error {
	sessionPath := filepath.Join(m.storePath, session.ID+".json")

	// Create a copy for saving (to avoid holding lock during I/O)
	session.mu.RLock()
	sessionData := *session
	session.mu.RUnlock()

	// Save to temporary file first, then rename (atomic operation)
	tempPath := sessionPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(sessionData); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode session: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tempPath, sessionPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// loadExistingSessions loads sessions from disk
func (m *Manager) loadExistingSessions() error {
	files, err := filepath.Glob(filepath.Join(m.storePath, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list session files: %w", err)
	}

	for _, file := range files {
		if filepath.Base(file) == "config.json" {
			continue // Skip config files
		}

		session, err := m.loadSessionFromFile(file)
		if err != nil {
			// Log error but continue loading other sessions
			fmt.Printf("Warning: failed to load session %s: %v\n", file, err)
			continue
		}

		m.sessions[session.ID] = session
	}

	return nil
}

// loadSessionFromFile loads a session from a specific file
func (m *Manager) loadSessionFromFile(filePath string) (*Session, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	var session Session
	if err := json.NewDecoder(file).Decode(&session); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	return &session, nil
}

// Description returns the description from metadata
func (s *Session) Description() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if desc, ok := s.Metadata["description"].(string); ok {
		return desc
	}
	return ""
}
