package routes

import (
	"net/http"
	"runtime"
	"time"

	"devorch/internal/global"
	platform "devorch/platform/detect"
)

var startTime = time.Now()

// HealthHandler는 헬스체크 핸들러입니다
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"uptime": time.Since(startTime).String(),
	})
}

// SystemInfoHandler는 시스템 정보 핸들러입니다
func SystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	profile := platform.Detect()

	writeJSON(w, http.StatusOK, map[string]any{
		"os":         profile.OS,
		"arch":       profile.Arch,
		"num_cpu":    profile.NumCPU,
		"go_version": runtime.Version(),
		"memory_gb":  profile.MemoryGB(),
		"has_gpu":    profile.HasGPU(),
		"uptime":     time.Since(startTime).String(),
	})
}

// FingerprintHandler는 환경 핑거프린트 핸들러입니다
func FingerprintHandler(w http.ResponseWriter, r *http.Request) {
	fp := global.GetFingerprint()
	envFp := platform.GenerateFingerprint()

	writeJSON(w, http.StatusOK, map[string]any{
		"install_id":       fp.InstallID,
		"env_hash":         envFp.Hash,
		"platform":         envFp.Platform,
		"features":         envFp.Features,
		"first_run_at":     fp.FirstRunAt,
		"schema_version":   fp.SchemaVersion,
		"last_seen_at":     fp.LastSeenAt,
		"runtime_sessions": fp.RuntimeSessions,
	})
}
