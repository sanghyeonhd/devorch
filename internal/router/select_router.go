package router

import (
	"context"
	"math"
	"time"

	"devorch/internal/provider/pool"
)

// SelectCandidate는 SelectRouter에서 사용하는 후보
type SelectCandidate struct {
	Provider string
	Model    string
	// 정책/요구사항상 기본 점수(0..1)
	BaseScore float64
}

type RouteFeedback interface {
	QueueDepth() int64
	Congestion() int64
}

type SelectRouter struct {
	Coldstart ColdstartPolicy

	Pool *pool.Pool

	Learning *LearningAdapter
	Feedback RouteFeedback

	// 혼잡 페널티 최대치(0..1)
	MaxCongestionPenalty float64
}

func NewSelectRouter(p *pool.Pool, la *LearningAdapter, fb RouteFeedback) *SelectRouter {
	return &SelectRouter{
		Coldstart:            DefaultColdstart(),
		Pool:                 p,
		Learning:             la,
		Feedback:             fb,
		MaxCongestionPenalty: 0.15,
	}
}

type SelectInput struct {
	Fingerprint string
	Category    string

	// 후보군(모델리졸버/요구사항에서 생성)
	Candidates []SelectCandidate
}

type SelectOutput struct {
	Chosen  SelectCandidate
	Explain Explain
}

func (r *SelectRouter) Select(ctx context.Context, in SelectInput) (SelectOutput, error) {
	exp := Explain{}
	bestScore := -1.0
	var best SelectCandidate

	congPenalty := r.congestionPenalty01()
	now := time.Now()

	for _, c := range in.Candidates {
		base := c.BaseScore
		if base <= 0 {
			base = r.Coldstart.DefaultScore
		}
		score := selClamp01(base)

		// 1) learning_adapter 보정
		if r.Learning != nil {
			s2, reason := r.Learning.AdjustScore(ctx, score, in.Fingerprint, in.Category, c.Provider, c.Model)
			if s2 != score {
				exp.Add("learning", reason, s2-score)
			}
			score = s2
		}

		// 2) reliability + decay
		if r.Pool != nil {
			snap := r.Pool.Snapshot(c.Provider, c.Model)
			if snap.Decay > 0 {
				// 성공률이 낮으면 페널티, 높으면 소폭 가산
				relDelta := (snap.SuccessEMA - 0.5) * 0.20 * snap.Decay // 최대 ±0.1 정도
				score = selClamp01(score + relDelta)

				// latency는 느릴수록 페널티(EMA ms를 0..1로 약식 정규화)
				latNorm := normalizeLatency01(snap.LatencyEMA)
				latDelta := -(latNorm * 0.10 * snap.Decay)
				score = selClamp01(score + latDelta)

				if !snap.Last.IsZero() {
					_ = now // (추후 stale 판단 강화에 사용)
				}
			}
		}

		// 3) 혼잡 페널티
		if congPenalty > 0 {
			score = selClamp01(score - congPenalty)
		}

		if score > bestScore {
			bestScore = score
			best = c
		}
	}

	exp.ChosenProvider = best.Provider
	exp.ChosenModel = best.Model
	exp.BaseScore = selClamp01(best.BaseScore)
	exp.FinalScore = selClamp01(bestScore)
	return SelectOutput{Chosen: best, Explain: exp}, nil
}

func (r *SelectRouter) congestionPenalty01() float64 {
	if r.Feedback == nil {
		return 0
	}
	// 0..100
	c := float64(r.Feedback.Congestion())
	if c <= 0 {
		return 0
	}
	if c > 100 {
		c = 100
	}
	return (c / 100.0) * r.MaxCongestionPenalty
}

func normalizeLatency01(latencyMs float64) float64 {
	// 매우 단순: 0ms -> 0, 3000ms -> 1
	if math.IsNaN(latencyMs) || latencyMs <= 0 {
		return 0
	}
	if latencyMs >= 3000 {
		return 1
	}
	return latencyMs / 3000.0
}

func selClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
