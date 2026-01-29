package routes

import (
	"net/http"
	"time"

	"devorch/internal/bus"
)

type StreamRoutes struct {
	Hub *bus.Hub
}

func (r *StreamRoutes) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/stream", r.handleStream) // query: workspace=...&session_id=... (선택)
}

func (r *StreamRoutes) handleStream(w http.ResponseWriter, req *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	workspace := req.URL.Query().Get("workspace")
	sessionID := req.URL.Query().Get("session_id")
	taskID := req.URL.Query().Get("task_id")

	// TODO: auth middleware에서 workspace 접근권한 확인 (8단계 강화)
	sub := r.Hub.Subscribe("sse-"+time.Now().Format("150405.000"), bus.TopicSessionStream, bus.TopicSystem)
	defer sub.Close()

	bus.WriteSSEComment(w, "connected")
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-req.Context().Done():
			return
		case <-tick.C:
			bus.WriteSSEComment(w, "ping")
		case ev, ok := <-sub.Ch:
			if !ok {
				return
			}
			// workspace 필터
			if workspace != "" && ev.Workspace != workspace {
				continue
			}
			// session/task 필터(옵션)
			if sessionID != "" && ev.SessionID != sessionID {
				continue
			}
			if taskID != "" && ev.TaskID != taskID {
				continue
			}
			_ = bus.WriteSSE(w, ev)
		}
	}
}
