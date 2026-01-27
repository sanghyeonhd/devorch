package tenancy

import (
	"context"
	"sync"
)

type Store struct {
	mu  sync.Mutex
	cur Workspace
}

func NewStore() *Store {
	return &Store{cur: DefaultWorkspace()}
}

func (s *Store) Current(ctx context.Context) Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *Store) SetCurrent(ctx context.Context, ws Workspace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cur = ws
}
