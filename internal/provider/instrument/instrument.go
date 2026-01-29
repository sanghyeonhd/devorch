package instrument

import (
	"context"
	"time"

	"devorch/internal/provider/observer"
)

// Instrumenter: provider 호출을 감싸서 관측 데이터 수집
type Instrumenter struct {
	Observer observer.Observer

	OS                 string
	Arch               string
	MachineFingerprint string
}

func NewInstrumenter(obs observer.Observer, osName, arch, machine string) *Instrumenter {
	return &Instrumenter{
		Observer:           obs,
		OS:                 osName,
		Arch:               arch,
		MachineFingerprint: machine,
	}
}

// Wrap: 함수 호출을 감싸서 시간/결과 기록
func (i *Instrumenter) Wrap(ctx context.Context, provider, model string, scenario Scenario, fn func() error) error {
	start := time.Now()
	err := fn()
	latency := time.Since(start).Milliseconds()

	call := observer.ProviderCall{
		CreatedAt:          start,
		Provider:           provider,
		Model:              model,
		Scenario:           string(scenario),
		OS:                 i.OS,
		Arch:               i.Arch,
		MachineFingerprint: i.MachineFingerprint,
		LatencyMs:          latency,
		Success:            err == nil,
	}
	if err != nil {
		call.Error = err.Error()
	}

	if i.Observer != nil {
		_ = i.Observer.OnComplete(ctx, call)
	}
	return err
}

// WrapWithTokens: 토큰 정보 포함
func (i *Instrumenter) WrapWithTokens(
	ctx context.Context,
	provider, model string,
	scenario Scenario,
	fn func() (inputTokens, outputTokens int64, err error),
) error {
	start := time.Now()
	inputTokens, outputTokens, err := fn()
	latency := time.Since(start).Milliseconds()

	call := observer.ProviderCall{
		CreatedAt:          start,
		Provider:           provider,
		Model:              model,
		Scenario:           string(scenario),
		OS:                 i.OS,
		Arch:               i.Arch,
		MachineFingerprint: i.MachineFingerprint,
		InputTokens:        inputTokens,
		OutputTokens:       outputTokens,
		LatencyMs:          latency,
		Success:            err == nil,
	}
	if err != nil {
		call.Error = err.Error()
	}

	if i.Observer != nil {
		_ = i.Observer.OnComplete(ctx, call)
	}
	return err
}
