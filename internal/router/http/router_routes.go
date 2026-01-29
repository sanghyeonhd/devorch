package http

import (
	"encoding/json"
	"net/http"

	"devorch/internal/router"
)

type RouterRoutes struct {
	PolicyRouter *router.PolicyRouter
	PolicyStore  *router.PolicyStore
}

func NewRouterRoutes(pr *router.PolicyRouter, ps *router.PolicyStore) *RouterRoutes {
	return &RouterRoutes{
		PolicyRouter: pr,
		PolicyStore:  ps,
	}
}

func (r *RouterRoutes) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/router/route", r.route)
	mux.HandleFunc("GET /api/router/policies", r.listPolicies)
}

func (r *RouterRoutes) route(w http.ResponseWriter, req *http.Request) {
	scenario := req.URL.Query().Get("scenario")
	if scenario == "" {
		scenario = "chat"
	}

	result, err := r.PolicyRouter.Route(req.Context(), scenario, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"provider": result.Provider,
		"model":    result.Model,
		"weight":   result.Weight,
	})
}

func (r *RouterRoutes) listPolicies(w http.ResponseWriter, req *http.Request) {
	policies, err := r.PolicyStore.All(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policies)
}
