package agent

import (
	"fmt"
	"sync"
)

// Registry manages agent registration and lookup
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent
}

// NewRegistry creates a new agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// Register adds an agent to the registry
func (r *Registry) Register(a *Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if a.Name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}

	r.agents[a.Name] = a
	return nil
}

// RegisterAll adds multiple agents
func (r *Registry) RegisterAll(agents []*Agent) error {
	for _, a := range agents {
		if err := r.Register(a); err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves an agent by name
func (r *Registry) Get(name string) (*Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.agents[name]
	if !ok {
		return nil, false
	}
	return a.Clone(), true
}

// List returns all registered agents
func (r *Registry) List() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agents = append(agents, a.Clone())
	}
	return agents
}

// Names returns all registered agent names
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Remove unregisters an agent
func (r *Registry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.agents[name]; !ok {
		return false
	}
	delete(r.agents, name)
	return true
}

// Count returns the number of registered agents
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// GetByRole returns agents with a specific role metadata
func (r *Registry) GetByRole(role string) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []*Agent
	for _, a := range r.agents {
		if roleVal, ok := a.Metadata["role"].(string); ok && roleVal == role {
			agents = append(agents, a.Clone())
		}
	}
	return agents
}

// DefaultRegistry returns a registry with default agents
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.RegisterAll(DefaultAgents())
	return r
}
