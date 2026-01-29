package server

import (
	"net/http"

	"devorch/internal/bus"
	"devorch/internal/server/routes"
	"devorch/internal/session"
)

type Server struct {
	Mux      *http.ServeMux
	Sessions *session.Processor
	Hub      *bus.Hub
}

func New(hub *bus.Hub, proc *session.Processor) *Server {
	mux := http.NewServeMux()

	// stream routes
	sr := &routes.StreamRoutes{Hub: hub}
	sr.Register(mux)

	return &Server{Mux: mux, Sessions: proc, Hub: hub}
}

func (s *Server) Handler() http.Handler {
	return s.Mux
}
