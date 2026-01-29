// Package bench provides reward shaping utilities.
// Phase 31: Reward Shaping - combines quality, latency, and cost
package bench

import "math"

// RewardInput represents input for reward computation.
type RewardInput struct {
	QualityScore float64
	LatencyMs    int64
	CostUSD      float64
	Success      bool
}

// ComputeReward calculates reward based on quality, latency, and cost.
// Returns a value typically in [-2, 2] range.
func ComputeReward(in RewardInput) float64 {
	if !in.Success {
		return -1.0
	}

	// Quality weighted: [0..2]
	r := in.QualityScore * 2.0

	// Latency penalty (after 10 seconds)
	if in.LatencyMs > 10000 {
		penalty := math.Min(1.0, float64(in.LatencyMs-10000)/20000.0)
		r -= penalty
	}

	// Cost penalty ($1 = -1 reward)
	r -= in.CostUSD

	// Bound the result
	if r < -2 {
		r = -2
	}
	if r > 2 {
		r = 2
	}

	return r
}

// NormalizeReward converts reward from [-2, 2] to [0, 1] range.
func NormalizeReward(reward float64) float64 {
	// Map [-2, 2] to [0, 1]
	normalized := (reward + 2.0) / 4.0
	if normalized < 0 {
		normalized = 0
	}
	if normalized > 1 {
		normalized = 1
	}
	return normalized
}
