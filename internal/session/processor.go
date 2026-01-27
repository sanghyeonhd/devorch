package session

import (
	"context"

	"devorch/internal/bench"
	"devorch/internal/provider"
)

type Processor struct {
	Providers *provider.Registry
	Bench     *bench.Recorder
	// Router/Scheduler etc...
}

func (p *Processor) RunChat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	call := func() (provider.ChatResponse, error) {
		c, ok := p.Providers.Get(req.Provider)
		if !ok {
			return provider.ChatResponse{}, provider.ErrProviderNotFound
		}
		if req.Stream {
			return c.ChatStream(ctx, req, func(ch provider.StreamChunk) error {
				// TODO: bus/SSE로 UI에 흘려보내기 (7단계에서 session.stream.go와 연결)
				return nil
			})
		}
		return c.Chat(ctx, req)
	}

	if p.Bench != nil {
		return p.Bench.WrapChat(ctx, req, call)
	}
	return call()
}
