package routes

import (
	"encoding/json"
	"net/http"

	"devorch/internal/provider"
	"devorch/internal/server/middleware"
)

type ProviderProxy struct {
	Registry *provider.Registry
	Injector *provider.AuthInjector
}

func (pp *ProviderProxy) HandleChat(w http.ResponseWriter, r *http.Request) {
	ws := middleware.WorkspaceFrom(r.Context())
	_ = ws // 추후 workspace별 라우팅 및 제한 적용

	provName := r.Header.Get("X-DevOrch-Provider")
	if provName == "" {
		provName = "github"
	}

	if pp.Injector != nil {
		if err := pp.Injector.Inject(r.Context(), r, provName); err != nil {
			http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
	}

	p, ok := pp.Registry.Get(provName)
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	var req provider.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	resp, err := p.Chat(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
