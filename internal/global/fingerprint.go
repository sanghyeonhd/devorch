package global

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

// Fingerprint는 인스턴스 핑거프린트 정보입니다
type Fingerprint struct {
	InstallID       string    `json:"install_id"`
	FirstRunAt      time.Time `json:"first_run_at"`
	SchemaVersion   int       `json:"schema_version"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	RuntimeSessions int       `json:"runtime_sessions"`
}

var (
	globalFingerprint *Fingerprint
	fingerprintOnce   sync.Once
)

// GetFingerprint는 글로벌 핑거프린트를 반환합니다
func GetFingerprint() *Fingerprint {
	fingerprintOnce.Do(func() {
		host, _ := os.Hostname()
		installID := MakeFingerprint(runtime.GOOS, runtime.GOARCH, host)

		globalFingerprint = &Fingerprint{
			InstallID:       installID[:16], // 처음 16자만 사용
			FirstRunAt:      time.Now(),
			SchemaVersion:   1,
			LastSeenAt:      time.Now(),
			RuntimeSessions: 0,
		}
	})

	globalFingerprint.LastSeenAt = time.Now()
	return globalFingerprint
}

// IncrementSessions는 세션 카운트를 증가시킵니다
func IncrementSessions() {
	fp := GetFingerprint()
	fp.RuntimeSessions++
}

// DefaultFingerprint는 HTTP 요청 컨텍스트에서 기본 fingerprint를 생성합니다
func DefaultFingerprint(r *http.Request) string {
	host, _ := os.Hostname()
	raw := runtime.GOOS + "|" + runtime.GOARCH + "|" + host

	// 프로젝트별로 분리하고 싶으면 헤더에서 추출
	if project := r.Header.Get("X-DevOrch-Project"); project != "" {
		raw += "|" + project
	}

	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// MakeFingerprint는 주어진 값들로 fingerprint를 생성합니다
func MakeFingerprint(goos, arch, hostname string) string {
	raw := goos + "|" + arch + "|" + hostname
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
