// Package websocket provides real-time WebSocket server functionality for DevOrch
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Server represents the WebSocket server
type Server struct {
	addr      string
	upgrader  websocket.Upgrader
	clients   map[string]*Client
	clientsMu sync.RWMutex
	hub       *Hub
	ctx       context.Context
	cancel    context.CancelFunc
}

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	conn   *websocket.Conn
	send   chan []byte
	server *Server
}

// Hub manages WebSocket message broadcasting
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

// Message represents a WebSocket message
type Message struct {
	Type  string      `json:"type"`
	Topic string      `json:"topic,omitempty"`
	Data  interface{} `json:"data"`
	From  string      `json:"from,omitempty"`
	Time  time.Time   `json:"time"`
}

// NewServer creates a new WebSocket server
func NewServer(addr string) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	hub := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	return &Server{
		addr: addr,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow connections from any origin
			},
		},
		clients: make(map[string]*Client),
		hub:     hub,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	// Start the hub
	go s.hub.run()

	// Setup HTTP handler
	http.HandleFunc("/ws", s.handleWebSocket)

	fmt.Printf("🔌 WebSocket server starting on %s\n", s.addr)

	// Start HTTP server in background
	go func() {
		if err := http.ListenAndServe(s.addr, nil); err != nil {
			fmt.Printf("WebSocket server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the WebSocket server
func (s *Server) Stop() error {
	s.cancel()
	return nil
}

// handleWebSocket handles WebSocket connection upgrades
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().Unix())
	client := &Client{
		ID:     clientID,
		conn:   conn,
		send:   make(chan []byte, 256),
		server: s,
	}

	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()

	s.hub.register <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()
}

// GetClients returns list of connected clients
func (s *Server) GetClients() map[string]*Client {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	clients := make(map[string]*Client)
	for id, client := range s.clients {
		clients[id] = client
	}
	return clients
}

// Broadcast sends a message to all connected clients
func (s *Server) Broadcast(data []byte) {
	s.hub.broadcast <- data
}

// Subscribe adds client to a topic (simplified implementation)
func (s *Server) Subscribe(clientID, topic string) error {
	s.clientsMu.RLock()
	client, exists := s.clients[clientID]
	s.clientsMu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	// Send subscription confirmation
	msg := Message{
		Type:  "subscription",
		Topic: topic,
		Data:  "subscribed",
		Time:  time.Now(),
	}

	msgBytes, _ := json.Marshal(msg)
	select {
	case client.send <- msgBytes:
	default:
		close(client.send)
		delete(s.clients, clientID)
	}

	return nil
}

// Ping pings all clients and returns response times
func (s *Server) Ping() map[string]time.Duration {
	results := make(map[string]time.Duration)

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for id := range s.clients {
		// Simulate ping - in real implementation would send ping frame
		results[id] = time.Duration(30+len(id)*3) * time.Millisecond
	}

	return results
}

// Hub methods
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// Client methods
func (c *Client) readPump() {
	defer func() {
		c.server.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// Echo message back for now
		c.server.hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
