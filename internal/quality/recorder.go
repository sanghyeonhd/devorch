package quality

import (
	"context"
	"time"

	"devorch/internal/storage/sqlite"
)

type Store interface {
	Insert(ctx context.Context, r sqlite.QualityRun) (string, error)
}

type AggReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.QualityAgg, error)
}

type Recorder struct {
	Eval  *Evaluator
	Store Store
}

func NewRecorder(store Store) *Recorder {
	return &Recorder{
		Eval:  NewEvaluator(),
		Store: store,
	}
}

func (r *Recorder) EvaluateAndRecord(ctx context.Context, req EvalRequest) error {
	start := time.Now()
	// latency/token은 6단계 bench에서 받는 게 정석이지만, MVP로는 여기서 측정값 0 처리 가능
	lat := time.Since(start)

	totalTok := 0
	res := r.Eval.Evaluate(req, lat, totalTok)

	run := sqlite.QualityRun{
		Workspace: req.Workspace,
		SessionID: req.SessionID,
		TaskID:    req.TaskID,
		Provider:  req.Provider,
		Model:     req.Model,
		CreatedAt: time.Now(),
		Success:   res.Success,
		Score:     res.Score,
		LatencyMs: lat.Milliseconds(),
		TotalTok:  totalTok,
		Error:     res.Error,
		Notes:     res.Notes,
	}
	if r.Store != nil {
		_, err := r.Store.Insert(ctx, run)
		return err
	}
	return nil
}
