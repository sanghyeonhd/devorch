package learner

import "math"

// 낮을수록 좋은 것: latency, cost
// 높을수록 좋은 것: success_rate
func ComputeWeight(successRate float64, avgLatency float64, avgCost float64) float64 {
	// 실패율 강한 페널티
	failPenalty := math.Pow(1.0-successRate, 2) * 5.0

	// latency/cost 정규화(러프)
	latScore := 1.0 / (1.0 + avgLatency/1000.0)
	costScore := 1.0 / (1.0 + avgCost)

	weight := successRate*3.0 + latScore*2.0 + costScore*2.0 - failPenalty

	if weight < 0 {
		return 0
	}
	return weight
}
