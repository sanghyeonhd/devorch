package server

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"devorch/internal/bus"
	"devorch/internal/server/routes"
	"devorch/internal/session"
	"devorch/internal/tool"
	"devorch/internal/tool/builtin"
)

type Server struct {
	Mux      *http.ServeMux
	Sessions *session.Processor
	Hub      *bus.Hub
	Tools    *tool.Registry
	WSHub    *routes.WebSocketHub
}

func New(hub *bus.Hub, proc *session.Processor) *Server {
	mux := http.NewServeMux()

	// stream routes
	sr := &routes.StreamRoutes{Hub: hub}
	sr.Register(mux)

	// tools registry and routes
	rootDir, _ := os.Getwd()
	reg, err := tool.NewRegistry()
	if err != nil {
		// Log error and continue with nil registry
		fmt.Printf("Warning: failed to create tool registry: %v\n", err)
		reg = nil
	} else {
		reg.WorkDir = rootDir
		reg.RegisterAll(builtin.DefaultTools(rootDir))
	}

	tr := &routes.ToolsRoutes{Registry: reg}
	tr.RegisterTools(mux)

	// ACP SSE stream
	ar := &routes.ACPRoutes{Hub: hub}
	ar.RegisterACP(mux)

	// ACP WebSocket
	wsHub := routes.NewWebSocketHub(hub)
	aw := &routes.ACPWebSocket{Hub: wsHub}
	aw.RegisterACPWebSocket(mux)

	return &Server{Mux: mux, Sessions: proc, Hub: hub, Tools: reg, WSHub: wsHub}
}

// RunWebSocketHub starts the WebSocket hub in a goroutine
func (s *Server) RunWebSocketHub(ctx context.Context) {
	if s.WSHub != nil {
		go s.WSHub.Run(ctx)
	}
}

func (s *Server) Handler() http.Handler {
	return s.Mux
}
