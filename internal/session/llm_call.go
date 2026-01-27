package session

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"

	"devorch/internal/okaon"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type Caller struct{}

func NewCaller() *Caller { return &Caller{} }

func (c *Caller) CallAndRecord(
	ctx context.Context,
	reg *provider.Registry,
	rt *router.Router,
	okStore *okaon.Store,
	wl router.Workload,
	candidates []router.Candidate,
	req provider.ChatRequest,
) (provider.ChatResponse, error) {
	start := time.Now()

	dec, err := rt.Select(ctx, wl, candidates)
	if err != nil {
		return provider.ChatResponse{}, err
	}

	p, ok := reg.Get(dec.Provider)
	if !ok {
		return provider.ChatResponse{}, provider.Err("provider_not_found", dec.Provider)
	}

	req.Model = dec.Model
	resp, callErr := p.Chat(ctx, req)

	lat := int(time.Since(start).Milliseconds())

	// Step2: local runtime이면 latency 기반으로 품질에 약간 반영(아주 보수적으로)
	quality := estimateQuality(callErr == nil)
	if callErr == nil && dec.Provider == "ollama" {
		quality = qualityFromLatency(lat)
	}

	run := okaon.Run{
		RunID:     ulid.Make().String(),
		CreatedAt: time.Now().Format(time.RFC3339Nano),

		PolicyKey: router.BuildPolicyKey(wl),

		WorkspaceID: wl.WorkspaceID,
		UserID:      wl.UserID,
		ProjectID:   wl.ProjectID,

		AgentType: wl.AgentType,
		Category:  wl.Category,
		TaskType:  wl.TaskType,

		Provider: dec.Provider,
		Model:    dec.Model,
		Endpoint: dec.Endpoint,

		LatencyMS: lat,
		OK:        boolToInt(callErr == nil),
		Quality:   quality,

		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		TotalTokens:      resp.TotalTokens,
		CostMicroUSD:     resp.CostMicroUSD,
	}

	if callErr != nil {
		run.ErrorCode = "provider_call_failed"
		run.ErrorMessage = callErr.Error()
	}

	if okStore != nil {
		_ = okStore.InsertRun(ctx, run)
	}

	if callErr != nil {
		return provider.ChatResponse{}, callErr
	}
	return resp, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func estimateQuality(ok bool) float64 {
	if !ok {
		return 0.0
	}
	return 0.60
}

// latency가 너무 길면 "체감 품질"이 떨어진다는 아주 단순한 보정 (Step2 placeholder)
func qualityFromLatency(ms int) float64 {
	if ms <= 300 {
		return 0.65
	}
	if ms <= 1200 {
		return 0.62
	}
	if ms <= 3000 {
		return 0.58
	}
	return 0.52
}
