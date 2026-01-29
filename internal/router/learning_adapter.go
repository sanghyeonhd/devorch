package router

import (
	"context"
	"time"

	"devorch/internal/okaon"
)

type OkStats interface {
	// 최근 arm별 통계 조회(모델 선택에 쓰는 최소 API)
	// - 22단계에서 만든 okaon_arm_stats 기반 구현이 있다면 그걸 사용
	GetArmStatByKey(ctx context.Context, fingerprint, category string, provider string, model string) (okaon.ArmStat, bool, error)
}

type LearningAdapter struct {
	Store OkStats

	// 최근 데이터만 반영하도록
	StaleAfter time.Duration

	// 보정 최대 폭(±)
	MaxDelta float64
}

func DefaultLearningAdapter(store OkStats) *LearningAdapter {
	return &LearningAdapter{
		Store:      store,
		StaleAfter: 21 * 24 * time.Hour,
		MaxDelta:   0.15,
	}
}

// baseScore(0..1)에 학습 보정 적용
func (a *LearningAdapter) AdjustScore(ctx context.Context, baseScore float64, fingerprint, category, provider, model string) (final float64, reason string) {
	if a == nil || a.Store == nil {
		return baseScore, "no_learning_store"
	}
	stat, ok, err := a.Store.GetArmStatByKey(ctx, fingerprint, category, provider, model)
	if err != nil || !ok {
		return baseScore, "no_arm_stats"
	}

	age := time.Since(time.Unix(stat.UpdatedTsUTC, 0).UTC())
	if a.StaleAfter > 0 && age > a.StaleAfter {
		return baseScore, "arm_stats_stale"
	}

	// stat.MeanReward01: 0..1 로 가정
	// baseScore와 meanReward를 섞어서 보정
	mean := laClamp01(stat.MeanReward01)
	delta := (mean - 0.5) * 2 * a.MaxDelta // [-MaxDelta, +MaxDelta]

	return laClamp01(baseScore + delta), "arm_reward_adjust"
}

func laClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
