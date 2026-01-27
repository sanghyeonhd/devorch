package auth

import (
	"sync"
	"time"
)

type Pending struct {
	WorkspaceID string
	Provider    Provider
	PKCE        *PKCE
	CreatedAt   time.Time
}

type StateStore struct {
	mu   sync.Mutex
	data map[string]Pending
	ttl  time.Duration
}

func NewStateStore(ttl time.Duration) *StateStore {
	return &StateStore{data: map[string]Pending{}, ttl: ttl}
}

func (s *StateStore) Put(state string, p Pending) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[state] = p
}

func (s *StateStore) Get(state string) (Pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.data[state]
	if !ok {
		return Pending{}, false
	}
	if time.Since(p.CreatedAt) > s.ttl {
		delete(s.data, state)
		return Pending{}, false
	}
	delete(s.data, state)
	return p, true
}
