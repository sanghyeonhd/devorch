package okaon

import "time"

// RetentionPolicy는 데이터 보존 정책
type RetentionPolicy struct {
	// 일반 데이터 최대 보존 기간
	MaxAge time.Duration

	// Arm 통계 최대 보존 기간
	ArmStatsMaxAge time.Duration
}

// DefaultRetentionPolicy는 기본 보존 정책 (90일)
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		MaxAge:         90 * 24 * time.Hour,
		ArmStatsMaxAge: 180 * 24 * time.Hour,
	}
}
