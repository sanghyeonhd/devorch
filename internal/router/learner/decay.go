package learner

import "time"

// DecayFactor: 오래된 샘플은 영향력 감소
func DecayFactor(updatedAt time.Time) float64 {
	days := time.Since(updatedAt).Hours() / 24.0
	if days <= 1 {
		return 1.0
	}
	// 30일 지나면 영향력 거의 0
	f := 1.0 / (1.0 + days/5.0)
	if f < 0.05 {
		return 0.05
	}
	return f
}
