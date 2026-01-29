package exec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"devorch/internal/hook"
	"devorch/internal/id"
	"devorch/internal/provider"
	"devorch/internal/provider/pool"
	"devorch/internal/router"
)

type BenchRecorder interface {
	// 25단계에서는 "실측 벤치 결과 저장 + arm_stats 갱신"까지 수행한다고 가정
	Record(ctx context.Context, in BenchInput) error
}

type BenchInput struct {
	RunID       string
	Fingerprint string
	Category    string

	Provider string
	Model    string

	Success      bool
	LatencyMs    int64
	CostMicroUSD int64

	Quality01 float64 // 0..1
	ErrorMsg  string
}

type RewardSink interface {
	Dispatch(ctx context.Context, in hook.DispatchInput) error
}

type Runner struct {
	Router *router.SelectRouter
	Pool   *pool.Pool

	Providers *provider.Registry

	Hooks RewardSink
	Bench BenchRecorder

	// retry 정책
	MaxAttempts int
	BaseBackoff time.Duration
}

func NewRunner(rt *router.SelectRouter, pl *pool.Pool, reg *provider.Registry, hooks RewardSink, bench BenchRecorder) *Runner {
	return &Runner{
		Router:      rt,
		Pool:        pl,
		Providers:   reg,
		Hooks:       hooks,
		Bench:       bench,
		MaxAttempts: 3,
		BaseBackoff: 250 * time.Millisecond,
	}
}

type RunInput struct {
	Fingerprint string
	Category    string

	// Router 후보 생성용(모델리졸버 결과)
	Candidates []router.SelectCandidate

	// 실제 요청(표준화된 provider request)
	Request provider.ChatRequest

	// Quality 측정이 가능하면(테스트/정답셋/정적 eval) 넣고, 없으면 0.5 기본
	QualityHint01 float64
}

type RunOutput struct {
	RunID    string
	Provider string
	Model    string

	Response provider.ChatResponse
}

func (r *Runner) RunChat(ctx context.Context, in RunInput) (RunOutput, error) {
	runID := id.NewULID()

	quality := in.QualityHint01
	if quality <= 0 {
		quality = 0.5
	}

	// 1) Router 선택
	sel, err := r.Router.Select(ctx, router.SelectInput{
		Fingerprint: in.Fingerprint,
		Category:    in.Category,
		Candidates:  in.Candidates,
	})
	if err != nil {
		return RunOutput{}, err
	}

	cur := sel.Chosen
	curCtxHash := hashContext(in.Request)

	// 2) 실제 실행(재시도 + fallback)
	var lastErr error
	attempts := clampInt(r.MaxAttempts, 1, 10)

	for attempt := 1; attempt <= attempts; attempt++ {
		start := time.Now()
		resp, costMicro, err := r.callOnce(ctx, cur.Provider, cur.Model, in.Request)
		latMs := time.Since(start).Milliseconds()

		success := (err == nil)

		// 2-a) Pool 관측 기록
		if r.Pool != nil {
			r.Pool.Observe(cur.Provider, cur.Model, success, latMs)
		}

		// 2-b) Bench 기록(실측)
		if r.Bench != nil {
			_ = r.Bench.Record(ctx, BenchInput{
				RunID:        runID,
				Fingerprint:  in.Fingerprint,
				Category:     in.Category,
				Provider:     cur.Provider,
				Model:        cur.Model,
				Success:      success,
				LatencyMs:    latMs,
				CostMicroUSD: costMicro,
				Quality01:    quality,
				ErrorMsg:     errString(err),
			})
		}

		if err == nil {
			return RunOutput{
				RunID:    runID,
				Provider: cur.Provider,
				Model:    cur.Model,
				Response: resp,
			}, nil
		}

		lastErr = err

		// 2-c) fallback: 다음 후보로 스위치 시도
		next, ok := pickNextCandidate(in.Candidates, cur)
		if ok {
			// Hook: model switch 이벤트 발행(=Reward 저장 트리거)
			if r.Hooks != nil {
				_ = r.Hooks.Dispatch(ctx, hook.DispatchInput{
					Type:        hook.EventOnModelSwitch,
					RunID:       runID,
					FromModel:   cur.Provider + "/" + cur.Model,
					ToModel:     next.Provider + "/" + next.Model,
					Reason:      "provider_error_or_retry",
					ContextHash: curCtxHash,
				})
			}
			cur = next
			// backoff는 스위치 후에도 적용
			sleepBackoff(ctx, r.BaseBackoff, attempt)
			continue
		}

		// 후보가 더 없으면 retry만
		sleepBackoff(ctx, r.BaseBackoff, attempt)
	}

	// 3) retry exhausted 이벤트
	if r.Hooks != nil {
		_ = r.Hooks.Dispatch(ctx, hook.DispatchInput{
			Type:    hook.EventOnRetryExhausted,
			RunID:   runID,
			Message: "retry_exhausted: " + errString(lastErr),
		})
	}
	return RunOutput{}, lastErr
}

func (r *Runner) callOnce(ctx context.Context, providerName, model string, req provider.ChatRequest) (provider.ChatResponse, int64, error) {
	if r.Providers == nil {
		return provider.ChatResponse{}, 0, errors.New("provider registry nil")
	}
	req.Provider = providerName
	req.Model = model
	c, ok := r.Providers.Get(providerName)
	if !ok {
		return provider.ChatResponse{}, 0, errors.New("unknown provider: " + providerName)
	}
	resp, err := c.Chat(ctx, req)
	return resp, resp.CostMicroUSD, err
}

func pickNextCandidate(list []router.SelectCandidate, cur router.SelectCandidate) (router.SelectCandidate, bool) {
	// 단순: list 순서 기준으로 현재 다음
	found := false
	for _, c := range list {
		if !found {
			if c.Provider == cur.Provider && c.Model == cur.Model {
				found = true
			}
			continue
		}
		return c, true
	}
	return router.SelectCandidate{}, false
}

func sleepBackoff(ctx context.Context, base time.Duration, attempt int) {
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	d := time.Duration(attempt) * base
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return
	case <-t.C:
		return
	}
}

func hashContext(req provider.ChatRequest) string {
	// 최소: messages role/content만 해시
	h := sha256.New()
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
