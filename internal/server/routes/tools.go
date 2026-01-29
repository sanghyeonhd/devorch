package routes

import (
	"encoding/json"
	"net/http"

	"devorch/internal/tool"
)

// ToolsRoutes provides HTTP endpoints to list and execute tools.
type ToolsRoutes struct {
	Registry *tool.Registry
}

// RegisterTools registers tool endpoints.
func (tr *ToolsRoutes) RegisterTools(mux *http.ServeMux) {
	// List tools
	mux.HandleFunc("GET /api/tools", func(w http.ResponseWriter, r *http.Request) {
		infos := tr.Registry.List()
		writeJSON(w, http.StatusOK, infos)
	})

	// Execute tool
	mux.HandleFunc("POST /api/tools/execute", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name      string          `json:"name"`
			Workspace string          `json:"workspace,omitempty"`
			SessionID string          `json:"session_id,omitempty"`
			TaskID    string          `json:"task_id,omitempty"`
			Params    json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		out, err := tr.Registry.Execute(r.Context(), req.Workspace, req.SessionID, req.TaskID, req.Name, req.Params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Result can be *tool.Result or any
		writeJSON(w, http.StatusOK, out)
	})
}
