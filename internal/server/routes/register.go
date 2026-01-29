package routes

import (
	"encoding/json"
	"net/http"
)

// RegisterRoutes는 모든 라우트를 등록합니다
func RegisterRoutes(mux *http.ServeMux, deps *RouteDeps) {
	// 헬스체크
	mux.HandleFunc("GET /health", HealthHandler)
	mux.HandleFunc("GET /api/health", HealthHandler)

	// 시스템 정보
	mux.HandleFunc("GET /api/system/info", SystemInfoHandler)
	mux.HandleFunc("GET /api/system/fingerprint", FingerprintHandler)

	// 채팅
	mux.HandleFunc("POST /api/chat", ChatHandler(deps))
	mux.HandleFunc("POST /api/chat/stream", ChatStreamHandler(deps))

	// OkAON 학습 데이터
	mux.HandleFunc("GET /api/learning/arms", LearningArmsHandler(deps))
	mux.HandleFunc("GET /api/learning/runs", LearningRunsHandler(deps))
	mux.HandleFunc("GET /api/learning/stats", LearningStatsHandler(deps))

	// 백그라운드 작업
	mux.HandleFunc("GET /api/tasks", TaskListHandler(deps))
	mux.HandleFunc("GET /api/tasks/{id}", TaskGetHandler(deps))
	mux.HandleFunc("POST /api/tasks/{id}/cancel", TaskCancelHandler(deps))

	// 프로바이더
	mux.HandleFunc("GET /api/providers", ProvidersListHandler(deps))
	mux.HandleFunc("GET /api/providers/{name}/models", ProviderModelsHandler(deps))
}

// RouteDeps는 라우트 의존성입니다
type RouteDeps struct {
	OkaonStore      OkaonQuerier
	BackgroundMgr   BackgroundManager
	ProviderManager ProviderLister
	SessionManager  SessionRunner
}

// OkaonQuerier는 OkAON 쿼리 인터페이스입니다
type OkaonQuerier interface {
	QueryArmStats(limit int) ([]ArmStatDTO, error)
	QueryRuns(limit int) ([]RunDTO, error)
}

// ArmStatDTO는 Arm 통계 DTO입니다
type ArmStatDTO struct {
	Key   string  `json:"key"`
	Mean  float64 `json:"mean"`
	Count int     `json:"count"`
}

// RunDTO는 Run DTO입니다
type RunDTO struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
}

// BackgroundManager는 백그라운드 관리자 인터페이스입니다
type BackgroundManager interface {
	ListTasks(limit int) ([]TaskDTO, error)
	GetTask(id string) (*TaskDTO, error)
	CancelTask(id string) error
}

// TaskDTO는 작업 DTO입니다
type TaskDTO struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
}

// ProviderLister는 프로바이더 목록 인터페이스입니다
type ProviderLister interface {
	ListProviders() []string
	ListModels(provider string) ([]string, error)
}

// SessionRunner는 세션 실행 인터페이스입니다
type SessionRunner interface {
	Chat(req ChatRequest) (*ChatResponse, error)
	ChatStream(req ChatRequest, cb func(chunk string) error) error
}

// ChatRequest는 채팅 요청입니다
type ChatRequest struct {
	Message   string `json:"message"`
	Model     string `json:"model,omitempty"`
	Provider  string `json:"provider,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse는 채팅 응답입니다
type ChatResponse struct {
	Message   string `json:"message"`
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	SessionID string `json:"session_id"`
}

// writeJSON은 JSON 응답을 작성합니다
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError는 에러 응답을 작성합니다
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
