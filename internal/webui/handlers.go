// Package webui provides API handlers.
package webui

import (
	"context"
	"devorch/internal/tui"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ChatService handles chat functionality.
type ChatService struct {
	mu       sync.RWMutex
	sessions map[string]*ChatSession
	server   *Server
}

// ChatSession represents a chat session.
type ChatSession struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ChatMessage represents a chat message.
type ChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Tokens    int       `json:"tokens,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// NewChatService creates a new chat service.
func NewChatService(server *Server) *ChatService {
	s := &ChatService{
		sessions: make(map[string]*ChatSession),
		server:   server,
	}
	s.registerHandlers()
	return s
}

func (s *ChatService) registerHandlers() {
	s.server.RegisterHandler("/sessions", s.handleSessions)
	s.server.RegisterHandler("GET /sessions", s.handleListSessions)
	s.server.RegisterHandler("POST /sessions", s.handleCreateSession)
	s.server.RegisterHandler("/chat", s.handleChat)
}

func (s *ChatService) handleSessions(ctx context.Context, req *APIRequest) (*APIResponse, error) {
	switch req.Method {
	case "GET":
		return s.handleListSessions(ctx, req)
	case "POST":
		return s.handleCreateSession(ctx, req)
	default:
		return &APIResponse{
			Status: http.StatusMethodNotAllowed,
			Body:   map[string]string{"error": "Method not allowed"},
		}, nil
	}
}

func (s *ChatService) handleListSessions(ctx context.Context, req *APIRequest) (*APIResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*ChatSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	return &APIResponse{
		Status: http.StatusOK,
		Body:   sessions,
	}, nil
}

func (s *ChatService) handleCreateSession(ctx context.Context, req *APIRequest) (*APIResponse, error) {
	var input struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal(req.Body, &input); err != nil {
		return &APIResponse{
			Status: http.StatusBadRequest,
			Body:   map[string]string{"error": "Invalid request body"},
		}, nil
	}

	if input.Name == "" {
		input.Name = "New Session"
	}

	session := &ChatSession{
		ID:        generateID(),
		Name:      input.Name,
		Messages:  []ChatMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	return &APIResponse{
		Status: http.StatusCreated,
		Body:   session,
	}, nil
}

func (s *ChatService) handleChat(ctx context.Context, req *APIRequest) (*APIResponse, error) {
	if req.Method != "POST" {
		return &APIResponse{
			Status: http.StatusMethodNotAllowed,
			Body:   map[string]string{"error": "Method not allowed"},
		}, nil
	}

	var input struct {
		SessionID   string  `json:"session_id"`
		Message     string  `json:"message"`
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
	}

	if err := json.Unmarshal(req.Body, &input); err != nil {
		return &APIResponse{
			Status: http.StatusBadRequest,
			Body:   map[string]string{"error": "Invalid request body"},
		}, nil
	}

	// Get or create session
	s.mu.Lock()
	session, ok := s.sessions[input.SessionID]
	if !ok {
		session = &ChatSession{
			ID:        generateID(),
			Name:      "Session",
			Messages:  []ChatMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		s.sessions[session.ID] = session
	}

	// Add user message
	userMsg := ChatMessage{
		ID:        generateID(),
		Role:      "user",
		Content:   input.Message,
		CreatedAt: time.Now(),
	}
	session.Messages = append(session.Messages, userMsg)
	session.UpdatedAt = time.Now()
	s.mu.Unlock()

	// Generate response via actual LLM (supports Ollama/Anthropic/OpenAI/etc)
	response := s.generateResponse(ctx, input.Message, input.Model, input.Temperature)

	// Add assistant message
	s.mu.Lock()
	assistantMsg := ChatMessage{
		ID:        generateID(),
		Role:      "assistant",
		Content:   response,
		CreatedAt: time.Now(),
	}
	session.Messages = append(session.Messages, assistantMsg)
	session.UpdatedAt = time.Now()
	s.mu.Unlock()

	// Broadcast response via SSE
	s.server.Broadcast(SSEEvent{
		Event: "message",
		Data: map[string]any{
			"type":       "assistant",
			"session_id": session.ID,
			"content":    response,
			"tokens":     len(response) / 4, // Rough estimate
		},
	})

	return &APIResponse{
		Status: http.StatusOK,
		Body: map[string]any{
			"session_id": session.ID,
			"message":    assistantMsg,
		},
	}, nil
}

func (s *ChatService) generateResponse(ctx context.Context, message, model string, temperature float64) string {
	// Use actual LLM provider via tui.ChatWithProvider

	// Determine provider from model name
	provider := "ollama"
	if strings.HasPrefix(model, "claude") || strings.HasPrefix(model, "anthropic") {
		provider = "anthropic"
	} else if strings.HasPrefix(model, "gpt") || strings.HasPrefix(model, "o1") {
		provider = "openai"
	} else if strings.HasPrefix(model, "gemini") {
		provider = "google"
	} else if strings.Contains(model, "/") {
		provider = "openrouter"
	}

	// Check if Ollama is available, otherwise use available cloud provider
	if provider == "ollama" {
		// Check if any cloud API keys are set
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			provider = "anthropic"
			model = "claude-sonnet-4-20250514"
		} else if os.Getenv("OPENAI_API_KEY") != "" {
			provider = "openai"
			model = "gpt-4o"
		} else if os.Getenv("GOOGLE_API_KEY") != "" {
			provider = "google"
			model = "gemini-2.0-flash"
		}
	}

	// Build messages
	messages := []tui.Message{
		{Role: "user", Content: message},
	}

	// Call LLM
	response, err := tui.ChatWithProvider(ctx, provider, model, messages)
	if err != nil {
		return "Error: " + err.Error() + "\n\nMake sure Ollama is running (ollama serve) or set an API key for a cloud provider."
	}

	return response
}

// Helper functions

func generateID() string {
	// Simple ID generation - would use UUID in production
	now := time.Now().UnixNano()
	return intToHex(now)
}

func intToHex(n int64) string {
	const hexChars = "0123456789abcdef"
	if n == 0 {
		return "0"
	}

	var result strings.Builder
	for n > 0 {
		result.WriteByte(hexChars[n%16])
		n /= 16
	}

	// Reverse
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
