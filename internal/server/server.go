package server

import (
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
}

func New(hub *bus.Hub, proc *session.Processor) *Server {
	mux := http.NewServeMux()

	// stream routes
	sr := &routes.StreamRoutes{Hub: hub}
	sr.Register(mux)

	// tools registry and routes
	rootDir, _ := os.Getwd()
	reg := tool.NewRegistry()
	reg.WorkDir = rootDir
	reg.RegisterAll(builtin.DefaultTools(rootDir))

	tr := &routes.ToolsRoutes{Registry: reg}
	tr.RegisterTools(mux)

	// ACP SSE stream
	ar := &routes.ACPRoutes{Hub: hub}
	ar.RegisterACP(mux)

	return &Server{Mux: mux, Sessions: proc, Hub: hub, Tools: reg}
}

func (s *Server) Handler() http.Handler {
	return s.Mux
}
