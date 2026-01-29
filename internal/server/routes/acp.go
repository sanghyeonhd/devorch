package routes

import (
	"net/http"
	"time"

	"devorch/internal/bus"
)

// ACPRoutes provides a minimal SSE-based ACP stream for IDEs.
type ACPRoutes struct {
	Hub *bus.Hub
}

// RegisterACP registers ACP SSE endpoint.
func (ar *ACPRoutes) RegisterACP(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/acp/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		sub := ar.Hub.Subscribe("acp-"+time.Now().Format("150405.000"), bus.TopicSystem, bus.TopicSessionStream)
		defer sub.Close()

		bus.WriteSSEComment(w, "connected")
		for ev := range sub.Ch {
			_ = bus.WriteSSE(w, ev)
		}
	})
}
