package bench

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/okaon"
)

// Result represents the result of a benchmark run
type Result struct {
	Provider     string
	Model        string
	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	CostMicroUSD int64
	ErrorCode    string
	ErrorMsg     string
	QualityScore float64
	Correctness  float64
	Style        float64
	Safety       float64
}

type OkStore interface {
	InsertWork(ctx context.Context, w okaon.Work) error
	InsertRun(ctx context.Context, r okaon.Run) error
	InsertQuality(ctx context.Context, q okaon.Quality) error
	InsertReward(ctx context.Context, rw okaon.Reward) error
}

type RecordInput struct {
	ProjectID   string
	WorkspaceID string
	UserID      string
	TaskType    string
	Category    string
	Agent       string

	PromptHash    string
	ContextHash   string
	OS            string
	Arch          string
	HWFingerprint string
	TagsJSON      string

	RoutePolicy string
	WasFallback bool
	RetryCount  int
}

func RecordBench(ctx context.Context, store OkStore, meta RecordInput, res Result, reward01 float64) (workID, runID string, err error) {
	now := time.Now().UTC()
	workID = id.NewULID()
	runID = id.NewULID()

	w := okaon.Work{
		ID:          workID,
		ProjectID:   meta.ProjectID,
		WorkspaceID: meta.WorkspaceID,
		UserID:      meta.UserID,
		TaskType:    meta.TaskType,
		Category:    meta.Category,
		Agent:       meta.Agent,
		PromptHash:  meta.PromptHash,
		ContextHash: meta.ContextHash,
		OS:          meta.OS,
		Arch:        meta.Arch,
		Fingerprint: meta.HWFingerprint,
		TagsJSON:    meta.TagsJSON,
		CreatedAt:   now,
	}
	if err := store.InsertWork(ctx, w); err != nil {
		return "", "", err
	}

	r := okaon.Run{
		ID:           runID,
		WorkID:       workID,
		Provider:     res.Provider,
		Model:        res.Model,
		RoutePolicy:  meta.RoutePolicy,
		WasFallback:  meta.WasFallback,
		RetryCount:   meta.RetryCount,
		LatencyMs:    res.LatencyMs,
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
		CostMicroUSD: res.CostMicroUSD,
		ErrorCode:    res.ErrorCode,
		ErrorMsg:     res.ErrorMsg,
		CreatedAt:    now,
	}
	if err := store.InsertRun(ctx, r); err != nil {
		return "", "", err
	}

	q := okaon.Quality{
		ID:          id.NewULID(),
		RunID:       runID,
		QualityBin:  int(benchClamp01(res.QualityScore) * 10),
		ScoreJSON:   "{}",
		EvaluatedAt: now,
	}
	_ = store.InsertQuality(ctx, q)

	rw := okaon.Reward{
		ID:        id.NewULID(),
		RunID:     runID,
		Reward01:  benchClamp01(reward01),
		CreatedAt: now,
	}
	_ = store.InsertReward(ctx, rw)

	return workID, runID, nil
}

func benchClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
