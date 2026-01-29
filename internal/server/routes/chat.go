package routes

import (
	"encoding/json"
	"net/http"
)

// ChatHandler는 채팅 핸들러입니다
func ChatHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.SessionManager == nil {
			writeError(w, http.StatusServiceUnavailable, "session manager not available")
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Message == "" {
			writeError(w, http.StatusBadRequest, "message is required")
			return
		}

		resp, err := deps.SessionManager.Chat(req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// ChatStreamHandler는 스트리밍 채팅 핸들러입니다
func ChatStreamHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.SessionManager == nil {
			writeError(w, http.StatusServiceUnavailable, "session manager not available")
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Message == "" {
			writeError(w, http.StatusBadRequest, "message is required")
			return
		}

		// SSE 헤더 설정
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		err := deps.SessionManager.ChatStream(req, func(chunk string) error {
			_, err := w.Write([]byte("data: " + chunk + "\n\n"))
			if err != nil {
				return err
			}
			flusher.Flush()
			return nil
		})

		if err != nil {
			// 에러 이벤트 전송
			w.Write([]byte("event: error\ndata: " + err.Error() + "\n\n"))
			flusher.Flush()
			return
		}

		// 완료 이벤트
		w.Write([]byte("event: done\ndata: {}\n\n"))
		flusher.Flush()
	}
}
