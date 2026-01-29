// Package router provides scoring utilities for model selection.
// Phase 32: Multi-factor scoring (quality, latency, cost, reliability, learning)
package router

import "math"

// ScoringCandidate represents a candidate model for scoring evaluation.
type ScoringCandidate struct {
	Provider string
	Model    string

	// Real-time state
	HealthScore     float64 // 0..1 (provider/pool health)
	Reliability     float64 // 0..1 (success rate / recent failure decay)
	ExpectedLatency float64 // ms (predicted)
	ExpectedQuality float64 // 0..1 (learning/bench based)
	ExpectedCostUSD float64 // $

	// Learning bonus (bandit/learner)
	LearningBonus float64 // -1..+1
}

// ScoringWeights configures the relative importance of each factor.
type ScoringWeights struct {
	WHealth   float64
	WReliab   float64
	WQuality  float64
	WLatency  float64
	WCost     float64
	WLearning float64
}

// DefaultScoringWeights returns reasonable default weights.
func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		WHealth:   1.0,
		WReliab:   1.0,
		WQuality:  2.0,
		WLatency:  1.0,
		WCost:     1.0,
		WLearning: 0.8,
	}
}

// ScoreCandidateV2 computes a score (higher is better) for a candidate.
func ScoreCandidateV2(c ScoringCandidate, w ScoringWeights) float64 {
	s := 0.0

	// Health and reliability are directly added
	s += w.WHealth * clamp01(c.HealthScore)
	s += w.WReliab * clamp01(c.Reliability)

	// Quality is weighted
	s += w.WQuality * clamp01(c.ExpectedQuality)

	// Latency penalty (lower latency is better)
	s -= w.WLatency * latencyPenaltyV2(c.ExpectedLatency)

	// Cost penalty (lower cost is better)
	s -= w.WCost * costPenaltyV2(c.ExpectedCostUSD)

	// Learning bonus
	s += w.WLearning * clampRange(c.LearningBonus, -1, 1)

	return s
}

func latencyPenaltyV2(ms float64) float64 {
	// 0~500ms: almost 0, grows after 2s, strong after 10s
	if ms <= 500 {
		return 0
	}
	x := (ms - 500) / 5000.0 // 0~1 over 5.5 second range
	return clamp01(x)
}

func costPenaltyV2(usd float64) float64 {
	// $0.00~$0.05: almost 0, strong penalty at $0.50+
	if usd <= 0.05 {
		return 0
	}
	x := (usd - 0.05) / 0.45
	return clamp01(x)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampRange(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// CandidateToScoringCandidate converts a router.Candidate to ScoringCandidate
// with additional context for scoring.
func CandidateToScoringCandidate(c Candidate, health, reliability, learningBonus float64) ScoringCandidate {
	return ScoringCandidate{
		Provider:        c.Provider,
		Model:           c.Model,
		HealthScore:     health,
		Reliability:     reliability,
		ExpectedLatency: float64(c.EstLatencyMS),
		ExpectedQuality: c.EstQuality,
		ExpectedCostUSD: float64(c.EstCostMicroUSD) / 1_000_000.0,
		LearningBonus:   learningBonus,
	}
}
