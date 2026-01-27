package provider

import "sync"

type ProviderInfo struct {
	Name string
	Kind string
}

type Registry struct {
	mu   sync.RWMutex
	m    map[string]Provider
	list []ProviderInfo
}

func NewRegistry() *Registry {
	return &Registry{
		m: make(map[string]Provider),
	}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[p.Name()] = p
	r.rebuildList()
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.m[name]
	return p, ok
}

func (r *Registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderInfo, len(r.list))
	copy(out, r.list)
	return out
}

func (r *Registry) rebuildList() {
	r.list = r.list[:0]
	for name, p := range r.m {
		r.list = append(r.list, ProviderInfo{Name: name, Kind: p.Kind()})
	}
}
