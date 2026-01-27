package provider

import (
	"context"
	"sync"
)

type ProviderInfo struct {
	Name string
	Kind string
}

type Registry struct {
	mu      sync.RWMutex
	m       map[string]Provider
	clients map[string]ChatClient
	list    []ProviderInfo
}

func NewRegistry() *Registry {
	return &Registry{
		m:       make(map[string]Provider),
		clients: make(map[string]ChatClient),
	}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[p.Name()] = p
	r.rebuildList()
}

func (r *Registry) RegisterClient(name string, c ChatClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = c
}

func (r *Registry) Get(name string) (ChatClient, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// ChatClient 우선
	if c, ok := r.clients[name]; ok {
		return c, true
	}
	// Legacy Provider는 adapter로 반환
	if p, ok := r.m[name]; ok {
		return &providerAdapter{p: p}, true
	}
	return nil, false
}

func (r *Registry) GetProvider(name string) (Provider, bool) {
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

func (r *Registry) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	c, ok := r.Get(req.Provider)
	if !ok {
		return ChatResponse{}, ErrProviderNotFound
	}
	if req.Stream {
		return c.ChatStream(ctx, req, func(_ StreamChunk) error { return nil })
	}
	return c.Chat(ctx, req)
}

// providerAdapter wraps legacy Provider as ChatClient
type providerAdapter struct {
	p Provider
}

func (a *providerAdapter) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	return a.p.Chat(ctx, req)
}

func (a *providerAdapter) ChatStream(ctx context.Context, req ChatRequest, yield func(StreamChunk) error) (ChatResponse, error) {
	// Legacy providers don't support streaming, just call Chat
	resp, err := a.p.Chat(ctx, req)
	if err != nil {
		return resp, err
	}
	_ = yield(StreamChunk{TextDelta: resp.Text(), Done: true})
	return resp, nil
}
