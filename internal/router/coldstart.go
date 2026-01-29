package router

import "time"

type ColdstartPolicy struct {
	// 데이터 없으면 기본 점수(중립)
	DefaultScore float64

	// 최근 데이터가 이 시간보다 오래되면 coldstart처럼 취급
	StaleAfter time.Duration
}

func DefaultColdstart() ColdstartPolicy {
	return ColdstartPolicy{
		DefaultScore: 0.5,
		StaleAfter:   21 * 24 * time.Hour,
	}
}
