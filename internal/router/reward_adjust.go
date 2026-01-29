package router

import "math"

// quality(↑), latency(↓), cost(↓)를 하나의 reward01(0..1)로 합성하는 유틸
// - 지금은 단순 합성(추후 정책/가중치 학습 가능)
type RewardWeights struct {
	Quality float64
	Latency float64
	Cost    float64
}

func DefaultRewardWeights() RewardWeights {
	return RewardWeights{Quality: 0.7, Latency: 0.2, Cost: 0.1}
}

// latencyMsNorm: 0..1 (0 빠름, 1 느림)
// costNorm: 0..1 (0 저렴, 1 비쌈)
// quality: 0..1 (0 나쁨, 1 좋음)
func ComposeReward01(w RewardWeights, quality, latencyMsNorm, costNorm float64) float64 {
	q := rewardClamp01(quality)
	l := rewardClamp01(1.0 - latencyMsNorm)
	c := rewardClamp01(1.0 - costNorm)

	sum := w.Quality + w.Latency + w.Cost
	if sum <= 0 {
		return q
	}
	r := (w.Quality*q + w.Latency*l + w.Cost*c) / sum
	return rewardClamp01(r)
}

func rewardClamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
