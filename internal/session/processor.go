package session

import (
	"bytes"
	"context"

	"devorch/internal/bench"
	"devorch/internal/bus"
	"devorch/internal/provider"
	"devorch/internal/quality"
)

type Processor struct {
	Providers *provider.Registry
	Bench     *bench.Recorder
	Hub       *bus.Hub
	Quality   *quality.Recorder
}

func (p *Processor) RunChat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	var full bytes.Buffer

	call := func() (provider.ChatResponse, error) {
		c, ok := p.Providers.Get(req.Provider)
		if !ok {
			return provider.ChatResponse{}, provider.ErrProviderNotFound
		}

		if req.Stream {
			resp, err := c.ChatStream(ctx, req, func(ch provider.StreamChunk) error {
				if ch.TextDelta != "" {
					full.WriteString(ch.TextDelta)
					if p.Hub != nil {
						p.Hub.Publish(BuildDeltaEvent(req.Workspace, req.SessionID, req.TaskID, ch.TextDelta))
					}
				}
				if ch.Done {
					if p.Hub != nil {
						p.Hub.Publish(BuildDoneEvent(req.Workspace, req.SessionID, req.TaskID, full.String()))
					}
				}
				return nil
			})
			// stream 응답은 last.Text가 비어있을 수 있으니 보정
			if resp.Text() == "" {
				resp.RawText = full.String()
				resp.Text_ = full.String()
			}
			return resp, err
		}

		resp, err := c.Chat(ctx, req)
		if err == nil {
			if p.Hub != nil {
				p.Hub.Publish(BuildDoneEvent(req.Workspace, req.SessionID, req.TaskID, resp.Text()))
			}
		} else if p.Hub != nil {
			p.Hub.Publish(BuildErrorEvent(req.Workspace, req.SessionID, req.TaskID, err.Error()))
		}
		return resp, err
	}

	// Bench wrap
	var resp provider.ChatResponse
	var err error
	if p.Bench != nil {
		resp, err = p.Bench.WrapChat(ctx, req, call)
	} else {
		resp, err = call()
	}

	// Quality eval: "작업 완료 기준" 기반 (MVP: success + error + latency + 토큰 + 선택적 tool 결과)
	if p.Quality != nil {
		qreq := quality.EvalRequest{
			Workspace: req.Workspace,
			SessionID: req.SessionID,
			TaskID:    req.TaskID,
			Provider:  req.Provider,
			Model:     req.Model,
			Answer:    resp.Text(),
			Error:     err,
		}
		_ = p.Quality.EvaluateAndRecord(ctx, qreq) // 실패해도 채팅 자체 실패로 만들지 않음
	}

	return resp, err
}
