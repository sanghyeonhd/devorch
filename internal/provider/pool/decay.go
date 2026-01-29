package pool

import (
	"math"
	"time"
)

func DecayFactor(last time.Time, halfLife time.Duration) float64 {
	if last.IsZero() {
		return 0.0
	}
	if halfLife <= 0 {
		halfLife = 24 * time.Hour
	}
	age := time.Since(last)
	if age <= 0 {
		return 1.0
	}
	return math.Exp(-0.6931471805599453 * float64(age) / float64(halfLife))
}
