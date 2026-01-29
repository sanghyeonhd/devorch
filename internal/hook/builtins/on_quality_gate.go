package builtins

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/id"
	"devorch/internal/okaon"
	"devorch/learning"
)

// 품질 게이트 결과 -> OkAON 저장 + Learner reward feed
type QualityGateInput struct {
	RunID     string
	Evaluator string
	Notes     string

	QualityScore float64 // 0..1
	Correctness  float64 // 0..1
	Style        float64 // 0..1
	Safety       float64 // 0..1

	// 학습 연결용
	ContextHash string
	Model       string

	// reward 구성 파라미터(일단 단순)
	RewardWeight float64
}

type QualityStore interface {
	InsertQuality(ctx context.Context, q okaon.Quality) error
	InsertReward(ctx context.Context, rw okaon.Reward) error
}

type QualityLearner interface {
	ObserveReward(ctx context.Context, contextHash, model string, reward01 float64) error
}

func OnQualityGate(ctx context.Context, store QualityStore, learner QualityLearner, in QualityGateInput) error {
	now := time.Now().UTC()

	// Build score JSON from detailed scores
	scoreData := map[string]float64{
		"quality":     qualityClamp01(in.QualityScore),
		"correctness": qualityClamp01(in.Correctness),
		"style":       qualityClamp01(in.Style),
		"safety":      qualityClamp01(in.Safety),
	}
	scoreJSON, _ := json.Marshal(scoreData)

	q := okaon.Quality{
		ID:          id.NewULID(),
		RunID:       in.RunID,
		QualityBin:  int(qualityClamp01(in.QualityScore) * 10),
		ScoreJSON:   string(scoreJSON),
		EvaluatedAt: now,
	}
	if err := store.InsertQuality(ctx, q); err != nil {
		return err
	}

	// reward: 우선은 qualityScore를 그대로 사용 (0..1)
	reward01 := qualityClamp01(in.QualityScore)

	rw := okaon.Reward{
		ID:        id.NewULID(),
		RunID:     in.RunID,
		Reward01:  reward01,
		CreatedAt: now,
	}
	if err := store.InsertReward(ctx, rw); err != nil {
		return err
	}

	// learner feed
	if learner != nil {
		_ = learner.ObserveReward(ctx, in.ContextHash, in.Model, reward01)
	}

	// (선택) ArmKey 계산/설명은 learning.ArmKey로 가능
	_ = learning.ArmKey(in.ContextHash, in.Model)

	return nil
}

func qualityClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
