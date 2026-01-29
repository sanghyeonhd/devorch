// Package webui provides a web-based user interface for DevOrch.
package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

// Server is the web UI server.
type Server struct {
	mu          sync.RWMutex
	httpServer  *http.Server
	mux         *http.ServeMux
	sseClients  map[string]chan SSEEvent
	handlers    map[string]APIHandler
	middlewares []Middleware
}

// APIHandler is a function that handles API requests.
type APIHandler func(ctx context.Context, req *APIRequest) (*APIResponse, error)

// Middleware is a function that wraps handlers.
type Middleware func(APIHandler) APIHandler

// APIRequest represents an API request.
type APIRequest struct {
	Method  string
	Path    string
	Body    json.RawMessage
	Query   map[string]string
	Headers map[string]string
	UserID  string
}

// APIResponse represents an API response.
type APIResponse struct {
	Status  int
	Body    any
	Headers map[string]string
}

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
	ID    string `json:"id,omitempty"`
}

// Config for the web UI server.
type Config struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	EnableCORS     bool
	CORSOrigins    []string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Addr:           ":8080",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
		EnableCORS:     true,
		CORSOrigins:    []string{"*"},
	}
}

// NewServer creates a new web UI server.
func NewServer(config Config) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		sseClients: make(map[string]chan SSEEvent),
		handlers:   make(map[string]APIHandler),
	}

	// Setup routes
	s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:           config.Addr,
		Handler:        s.corsMiddleware(s.mux, config),
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}

	return s
}

func (s *Server) setupRoutes() {
	// Static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	s.mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// API routes
	s.mux.HandleFunc("/api/", s.handleAPI)

	// SSE endpoint
	s.mux.HandleFunc("/events", s.handleSSE)

	// Health check
	s.mux.HandleFunc("/health", s.handleHealth)
}

func (s *Server) corsMiddleware(h http.Handler, config Config) http.Handler {
	if !config.EnableCORS {
		return h
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false

		for _, o := range config.CORSOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}

// Start starts the web server.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// Close all SSE connections
	s.mu.Lock()
	for _, ch := range s.sseClients {
		close(ch)
	}
	s.sseClients = make(map[string]chan SSEEvent)
	s.mu.Unlock()

	return s.httpServer.Shutdown(ctx)
}

// RegisterHandler registers an API handler.
func (s *Server) RegisterHandler(path string, handler APIHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[path] = handler
}

// AddMiddleware adds a middleware.
func (s *Server) AddMiddleware(m Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = append(s.middlewares, m)
}

// Broadcast sends an event to all SSE clients.
func (s *Server) Broadcast(event SSEEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.sseClients {
		select {
		case ch <- event:
		default:
			// Client is slow, skip
		}
	}
}

// SendTo sends an event to a specific client.
func (s *Server) SendTo(clientID string, event SSEEvent) {
	s.mu.RLock()
	ch, ok := s.sseClients[clientID]
	s.mu.RUnlock()

	if ok {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")

	// Parse request
	var body json.RawMessage
	if r.Body != nil && r.ContentLength > 0 {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "Failed to read request body")
			return
		}
		body = bodyBytes
	}

	// Build query map
	query := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	// Build headers map
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	req := &APIRequest{
		Method:  r.Method,
		Path:    path,
		Body:    body,
		Query:   query,
		Headers: headers,
	}

	// Find handler
	s.mu.RLock()
	handler, ok := s.handlers[path]
	middlewares := s.middlewares
	s.mu.RUnlock()

	if !ok {
		// Try to match with method prefix
		key := r.Method + " " + path
		s.mu.RLock()
		handler, ok = s.handlers[key]
		s.mu.RUnlock()
	}

	if !ok {
		s.writeError(w, http.StatusNotFound, "Not found")
		return
	}

	// Apply middlewares
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	// Execute handler
	resp, err := handler(r.Context(), req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Write response
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)

	if resp.Body != nil {
		json.NewEncoder(w).Encode(resp.Body)
	}
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create client channel
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	eventChan := make(chan SSEEvent, 100)

	s.mu.Lock()
	s.sseClients[clientID] = eventChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sseClients, clientID)
		s.mu.Unlock()
		close(eventChan)
	}()

	// Send initial connection event
	s.writeSSEEvent(w, SSEEvent{
		Event: "connected",
		Data:  map[string]string{"client_id": clientID},
	})

	// Flush to ensure headers are sent
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Stream events
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			s.writeSSEEvent(w, event)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) writeSSEEvent(w http.ResponseWriter, event SSEEvent) {
	if event.ID != "" {
		fmt.Fprintf(w, "id: %s\n", event.ID)
	}
	if event.Event != "" {
		fmt.Fprintf(w, "event: %s\n", event.Event)
	}

	data, _ := json.Marshal(event.Data)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
