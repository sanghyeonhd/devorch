package routes

import (
	"net/http"
)

// ProvidersListHandler는 프로바이더 목록 핸들러입니다
func ProvidersListHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.ProviderManager == nil {
			writeError(w, http.StatusServiceUnavailable, "provider manager not available")
			return
		}

		providers := deps.ProviderManager.ListProviders()

		writeJSON(w, http.StatusOK, map[string]any{
			"providers": providers,
			"count":     len(providers),
		})
	}
}

// ProviderModelsHandler는 프로바이더 모델 목록 핸들러입니다
func ProviderModelsHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.ProviderManager == nil {
			writeError(w, http.StatusServiceUnavailable, "provider manager not available")
			return
		}

		providerName := r.PathValue("name")
		if providerName == "" {
			writeError(w, http.StatusBadRequest, "provider name required")
			return
		}

		models, err := deps.ProviderManager.ListModels(providerName)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"provider": providerName,
			"models":   models,
			"count":    len(models),
		})
	}
}
