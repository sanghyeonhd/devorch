// Package history provides file change history tracking
package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ChangeType represents the type of file change
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeModify ChangeType = "modify"
	ChangeTypeDelete ChangeType = "delete"
	ChangeTypeRename ChangeType = "rename"
)

// FileChange represents a single file change
type FileChange struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	OldPath      string     `json:"old_path,omitempty"` // For renames
	Type         ChangeType `json:"type"`
	Timestamp    time.Time  `json:"timestamp"`
	Hash         string     `json:"hash"`
	PreviousHash string     `json:"previous_hash,omitempty"`
	Size         int64      `json:"size"`
	Tool         string     `json:"tool,omitempty"`        // Which tool made the change
	Description  string     `json:"description,omitempty"` // Change description
	Backup       string     `json:"backup,omitempty"`      // Backup file path
}

// FileState represents the current state of a tracked file
type FileState struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// Tracker tracks file changes
type Tracker struct {
	historyDir string
	backupDir  string
	changes    []FileChange
	changesMu  sync.RWMutex
	fileStates map[string]*FileState
	statesMu   sync.RWMutex
	maxBackups int
	enabled    bool
}

// NewTracker creates a new file change tracker
func NewTracker(historyDir string, maxBackups int) (*Tracker, error) {
	if historyDir == "" {
		homeDir, _ := os.UserHomeDir()
		historyDir = filepath.Join(homeDir, ".devorch", "history")
	}

	backupDir := filepath.Join(historyDir, "backups")

	// Create directories
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, err
	}

	if maxBackups <= 0 {
		maxBackups = 100
	}

	t := &Tracker{
		historyDir: historyDir,
		backupDir:  backupDir,
		changes:    make([]FileChange, 0),
		fileStates: make(map[string]*FileState),
		maxBackups: maxBackups,
		enabled:    true,
	}

	// Load existing history
	t.Load()

	return t, nil
}

// RecordChange records a file change
func (t *Tracker) RecordChange(path string, changeType ChangeType, tool, description string) (*FileChange, error) {
	if !t.enabled {
		return nil, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Get file info and hash
	var hash string
	var size int64

	if changeType != ChangeTypeDelete {
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, err
		}
		size = info.Size()

		hash, err = t.hashFile(absPath)
		if err != nil {
			return nil, err
		}
	}

	// Get previous state
	t.statesMu.RLock()
	prevState, hasPrev := t.fileStates[absPath]
	t.statesMu.RUnlock()

	var prevHash string
	if hasPrev {
		prevHash = prevState.Hash
	}

	// Create backup for modifications
	var backupPath string
	if changeType == ChangeTypeModify && hasPrev {
		backupPath, err = t.createBackup(absPath, prevHash)
		if err != nil {
			// Non-fatal error, continue without backup
			backupPath = ""
		}
	}

	// Create change record
	change := FileChange{
		ID:           generateID(),
		Path:         absPath,
		Type:         changeType,
		Timestamp:    now,
		Hash:         hash,
		PreviousHash: prevHash,
		Size:         size,
		Tool:         tool,
		Description:  description,
		Backup:       backupPath,
	}

	// Update state
	t.statesMu.Lock()
	if changeType == ChangeTypeDelete {
		delete(t.fileStates, absPath)
	} else {
		t.fileStates[absPath] = &FileState{
			Path:         absPath,
			Hash:         hash,
			Size:         size,
			LastModified: now,
		}
	}
	t.statesMu.Unlock()

	// Add to history
	t.changesMu.Lock()
	t.changes = append(t.changes, change)
	t.changesMu.Unlock()

	// Save history
	t.Save()

	return &change, nil
}

// RecordRename records a file rename
func (t *Tracker) RecordRename(oldPath, newPath, tool, description string) (*FileChange, error) {
	if !t.enabled {
		return nil, nil
	}

	absOldPath, _ := filepath.Abs(oldPath)
	absNewPath, _ := filepath.Abs(newPath)

	now := time.Now()

	// Get file info
	info, err := os.Stat(absNewPath)
	if err != nil {
		return nil, err
	}

	hash, err := t.hashFile(absNewPath)
	if err != nil {
		return nil, err
	}

	// Get previous state
	t.statesMu.RLock()
	prevState, hasPrev := t.fileStates[absOldPath]
	t.statesMu.RUnlock()

	var prevHash string
	if hasPrev {
		prevHash = prevState.Hash
	}

	change := FileChange{
		ID:           generateID(),
		Path:         absNewPath,
		OldPath:      absOldPath,
		Type:         ChangeTypeRename,
		Timestamp:    now,
		Hash:         hash,
		PreviousHash: prevHash,
		Size:         info.Size(),
		Tool:         tool,
		Description:  description,
	}

	// Update state
	t.statesMu.Lock()
	delete(t.fileStates, absOldPath)
	t.fileStates[absNewPath] = &FileState{
		Path:         absNewPath,
		Hash:         hash,
		Size:         info.Size(),
		LastModified: now,
	}
	t.statesMu.Unlock()

	// Add to history
	t.changesMu.Lock()
	t.changes = append(t.changes, change)
	t.changesMu.Unlock()

	t.Save()

	return &change, nil
}

// GetHistory returns the change history
func (t *Tracker) GetHistory(limit int) []FileChange {
	t.changesMu.RLock()
	defer t.changesMu.RUnlock()

	if limit <= 0 || limit > len(t.changes) {
		limit = len(t.changes)
	}

	// Return most recent changes first
	result := make([]FileChange, limit)
	for i := 0; i < limit; i++ {
		result[i] = t.changes[len(t.changes)-1-i]
	}

	return result
}

// GetFileHistory returns history for a specific file
func (t *Tracker) GetFileHistory(path string) []FileChange {
	t.changesMu.RLock()
	defer t.changesMu.RUnlock()

	absPath, _ := filepath.Abs(path)

	var result []FileChange
	for _, change := range t.changes {
		if change.Path == absPath || change.OldPath == absPath {
			result = append(result, change)
		}
	}

	// Reverse to show most recent first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// Revert reverts a file to a previous state
func (t *Tracker) Revert(changeID string) error {
	t.changesMu.RLock()
	var change *FileChange
	for i := range t.changes {
		if t.changes[i].ID == changeID {
			change = &t.changes[i]
			break
		}
	}
	t.changesMu.RUnlock()

	if change == nil {
		return fmt.Errorf("change not found: %s", changeID)
	}

	if change.Backup == "" {
		return fmt.Errorf("no backup available for change: %s", changeID)
	}

	// Check backup exists
	if _, err := os.Stat(change.Backup); err != nil {
		return fmt.Errorf("backup file not found: %s", change.Backup)
	}

	// Read backup
	data, err := os.ReadFile(change.Backup)
	if err != nil {
		return err
	}

	// Restore file
	if err := os.WriteFile(change.Path, data, 0644); err != nil {
		return err
	}

	// Record the revert as a new change
	t.RecordChange(change.Path, ChangeTypeModify, "revert", fmt.Sprintf("Reverted from change %s", changeID))

	return nil
}

// GetFileState returns the tracked state of a file
func (t *Tracker) GetFileState(path string) (*FileState, bool) {
	t.statesMu.RLock()
	defer t.statesMu.RUnlock()

	absPath, _ := filepath.Abs(path)
	state, ok := t.fileStates[absPath]
	return state, ok
}

// HasChanged checks if a file has changed since last tracked
func (t *Tracker) HasChanged(path string) (bool, error) {
	absPath, _ := filepath.Abs(path)

	state, exists := t.GetFileState(absPath)
	if !exists {
		// Never tracked, consider it changed
		return true, nil
	}

	hash, err := t.hashFile(absPath)
	if err != nil {
		return false, err
	}

	return hash != state.Hash, nil
}

func (t *Tracker) hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (t *Tracker) createBackup(path, hash string) (string, error) {
	// Generate backup filename
	filename := fmt.Sprintf("%s_%s", filepath.Base(path), hash[:12])
	backupPath := filepath.Join(t.backupDir, filename)

	// Check if backup already exists
	if _, err := os.Stat(backupPath); err == nil {
		return backupPath, nil // Already backed up
	}

	// Read original file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}

	// Cleanup old backups
	t.cleanupBackups()

	return backupPath, nil
}

func (t *Tracker) cleanupBackups() {
	entries, err := os.ReadDir(t.backupDir)
	if err != nil {
		return
	}

	if len(entries) <= t.maxBackups {
		return
	}

	// Sort by modification time and delete oldest
	type backupInfo struct {
		path    string
		modTime time.Time
	}

	var backups []backupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupInfo{
			path:    filepath.Join(t.backupDir, e.Name()),
			modTime: info.ModTime(),
		})
	}

	// Sort by time (oldest first)
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].modTime.After(backups[j].modTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// Delete oldest backups
	toDelete := len(backups) - t.maxBackups
	for i := 0; i < toDelete; i++ {
		os.Remove(backups[i].path)
	}
}

// Load loads history from disk
func (t *Tracker) Load() error {
	historyFile := filepath.Join(t.historyDir, "history.json")

	data, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var saved struct {
		Changes    []FileChange          `json:"changes"`
		FileStates map[string]*FileState `json:"file_states"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	t.changesMu.Lock()
	t.changes = saved.Changes
	t.changesMu.Unlock()

	t.statesMu.Lock()
	t.fileStates = saved.FileStates
	t.statesMu.Unlock()

	return nil
}

// Save saves history to disk
func (t *Tracker) Save() error {
	t.changesMu.RLock()
	t.statesMu.RLock()
	defer t.changesMu.RUnlock()
	defer t.statesMu.RUnlock()

	saved := struct {
		Changes    []FileChange          `json:"changes"`
		FileStates map[string]*FileState `json:"file_states"`
	}{
		Changes:    t.changes,
		FileStates: t.fileStates,
	}

	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}

	historyFile := filepath.Join(t.historyDir, "history.json")
	return os.WriteFile(historyFile, data, 0644)
}

// SetEnabled enables or disables tracking
func (t *Tracker) SetEnabled(enabled bool) {
	t.enabled = enabled
}

// Clear clears all history
func (t *Tracker) Clear() error {
	t.changesMu.Lock()
	t.statesMu.Lock()
	defer t.changesMu.Unlock()
	defer t.statesMu.Unlock()

	t.changes = make([]FileChange, 0)
	t.fileStates = make(map[string]*FileState)

	// Remove backup files
	os.RemoveAll(t.backupDir)
	os.MkdirAll(t.backupDir, 0755)

	return t.Save()
}

func generateID() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
