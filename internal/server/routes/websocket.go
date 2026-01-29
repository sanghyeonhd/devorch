// Package routes provides HTTP route handlers for the DevOrch server.
package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"devorch/internal/bus"
)

// WebSocket message types
const (
	WSMsgTypeChat       = "chat"
	WSMsgTypeToolCall   = "tool_call"
	WSMsgTypeToolResult = "tool_result"
	WSMsgTypeStream     = "stream"
	WSMsgTypeError      = "error"
	WSMsgTypePing       = "ping"
	WSMsgTypePong       = "pong"
	WSMsgTypeSubscribe  = "subscribe"
	WSMsgTypeEvent      = "event"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// WSChatPayload represents a chat message payload
type WSChatPayload struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
}

// WSToolCallPayload represents a tool call payload
type WSToolCallPayload struct {
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
}

// WSStreamPayload represents a streaming chunk payload
type WSStreamPayload struct {
	Delta    string `json:"delta"`
	Done     bool   `json:"done"`
	ToolCall *struct {
		Name string `json:"name"`
		Args string `json:"args"`
	} `json:"tool_call,omitempty"`
}

// WSErrorPayload represents an error payload
type WSErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	broadcast  chan *WSMessage
	mu         sync.RWMutex
	bus        *bus.Hub
}

// WebSocketClient represents a WebSocket client connection
type WebSocketClient struct {
	hub      *WebSocketHub
	conn     *websocket.Conn
	send     chan *WSMessage
	id       string
	topics   []string
	mu       sync.Mutex
	isClosed bool
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, check for allowed origins
		// For now, allow localhost and common development hosts
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin or no origin header
		}
		// Allow localhost variants for development
		allowedOrigins := []string{
			"http://localhost",
			"http://127.0.0.1",
			"https://localhost",
			"https://127.0.0.1",
		}
		for _, allowed := range allowedOrigins {
			if strings.HasPrefix(origin, allowed) {
				return true
			}
		}
		// In production environment, be more restrictive
		if os.Getenv("DEVORCH_ENV") == "production" {
			slog.Warn("WebSocket origin rejected in production", "origin", origin)
			return false
		}
		return true
	},
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(eventBus *bus.Hub) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		broadcast:  make(chan *WSMessage, 256),
		bus:        eventBus,
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Info("WebSocket client connected", "id", client.id)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
			slog.Info("WebSocket client disconnected", "id", client.id)

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(msg *WSMessage) {
	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("broadcast channel full, dropping message")
	}
}

// ClientCount returns the number of connected clients
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS handles WebSocket requests
func (h *WebSocketHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	client := &WebSocketClient{
		hub:    h,
		conn:   conn,
		send:   make(chan *WSMessage, 256),
		id:     fmt.Sprintf("ws-%d", time.Now().UnixNano()),
		topics: []string{},
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// Close closes the client connection
func (c *WebSocketClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return
	}
	c.isClosed = true
	close(c.send)
	c.conn.Close()
}

// readPump reads messages from the WebSocket connection
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(64 * 1024) // 64KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket read error", "error", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.sendError("invalid_json", "Failed to parse message")
			continue
		}

		c.handleMessage(&msg)
	}
}

// writePump writes messages to the WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("Failed to marshal message", "error", err)
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.Error("WebSocket write error", "error", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *WebSocketClient) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case WSMsgTypePing:
		c.send <- &WSMessage{
			Type:      WSMsgTypePong,
			Timestamp: time.Now().UnixMilli(),
		}

	case WSMsgTypeSubscribe:
		var topics []string
		if err := json.Unmarshal(msg.Payload, &topics); err == nil {
			c.mu.Lock()
			c.topics = append(c.topics, topics...)
			c.mu.Unlock()

			// Subscribe to bus topics
			if c.hub.bus != nil {
				busTopics := make([]bus.Topic, len(topics))
				for i, t := range topics {
					busTopics[i] = bus.Topic(t)
				}
				sub := c.hub.bus.Subscribe(c.id, busTopics...)
				go c.forwardBusEvents(sub)
			}
		}

	case WSMsgTypeChat:
		// Forward to session processor via bus
		if c.hub.bus != nil {
			c.hub.bus.Publish(bus.Event{
				Topic:     bus.TopicSessionChat,
				Type:      "chat",
				SessionID: msg.SessionID,
				Payload:   msg.Payload,
			})
		}

	case WSMsgTypeToolCall:
		// Forward tool call to tool executor
		if c.hub.bus != nil {
			c.hub.bus.Publish(bus.Event{
				Topic:     bus.TopicToolCall,
				Type:      "tool_call",
				SessionID: msg.SessionID,
				Payload:   msg.Payload,
			})
		}

	default:
		c.sendError("unknown_type", fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// forwardBusEvents forwards events from the bus to the WebSocket client
func (c *WebSocketClient) forwardBusEvents(sub *bus.Subscription) {
	defer sub.Close()

	for ev := range sub.Ch {
		payload, _ := json.Marshal(ev.Payload)
		c.send <- &WSMessage{
			Type:      WSMsgTypeEvent,
			ID:        ev.ID,
			SessionID: ev.SessionID,
			Payload:   payload,
			Timestamp: time.Now().UnixMilli(),
		}
	}
}

// sendError sends an error message to the client
func (c *WebSocketClient) sendError(code, message string) {
	payload, _ := json.Marshal(WSErrorPayload{
		Code:    code,
		Message: message,
	})

	c.send <- &WSMessage{
		Type:      WSMsgTypeError,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}
}

// Send sends a message to the client
func (c *WebSocketClient) Send(msg *WSMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return
	}

	select {
	case c.send <- msg:
	default:
		// Buffer full
	}
}

// ACPWebSocket provides WebSocket-based ACP for IDEs
type ACPWebSocket struct {
	Hub *WebSocketHub
}

// RegisterACPWebSocket registers ACP WebSocket endpoint
func (aw *ACPWebSocket) RegisterACPWebSocket(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/acp/ws", aw.Hub.ServeWS)
}
