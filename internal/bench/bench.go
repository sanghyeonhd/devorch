package bench

import (
	"context"
	"time"

	"devorch/internal/provider"
)

type Store interface {
	Insert(ctx context.Context, br provider.BenchResult) (string, error)
}

type Recorder struct {
	Store Store
}

func (r *Recorder) WrapChat(ctx context.Context, req provider.ChatRequest, fn func() (provider.ChatResponse, error)) (provider.ChatResponse, error) {
	start := time.Now()
	resp, err := fn()
	end := time.Now()

	br := provider.BenchResult{
		Workspace: req.Workspace,
		SessionID: req.SessionID,
		TaskID:    req.TaskID,
		Provider:  req.Provider,
		Model:     req.Model,
		StartedAt: start,
		EndedAt:   end,
		LatencyMs: end.Sub(start).Milliseconds(),
	}
	if err != nil {
		br.Error = err.Error()
	} else {
		br.PromptTok = resp.PromptTokens
		br.CompletionTok = resp.CompletionTokens
		br.TotalTok = resp.TotalTokens
	}
	if r.Store != nil {
		_, _ = r.Store.Insert(ctx, br) // bench 저장 실패는 본 요청 실패로 만들지 않음
	}
	return resp, err
}
