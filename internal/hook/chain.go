package hook

import (
	"context"

	"devorch/internal/hook/builtins"
)

// 실제 구현에서는 okAON.Reward를 직접 받도록 하는게 정석이지만,
// 여기서는 hook 모듈이 okAON에 강결합되지 않게 "어댑터"로 처리합니다.
type RewardStore interface {
	builtins.SwitchStore
	builtins.RetryStore
}

type Chain struct {
	Store RewardStore
}

type DispatchInput struct {
	Type EventType

	// 공통
	RunID string

	// 모델 스위치
	FromModel   string
	ToModel     string
	Reason      string
	ContextHash string

	// 재시도 소진
	Message string
}

func (c *Chain) Dispatch(ctx context.Context, in DispatchInput) error {
	switch in.Type {
	case EventOnModelSwitch:
		return builtins.OnModelSwitch(ctx, c.Store, builtins.ModelSwitchInput{
			RunID:       in.RunID,
			FromModel:   in.FromModel,
			ToModel:     in.ToModel,
			Reason:      in.Reason,
			ContextHash: in.ContextHash,
		})
	case EventOnRetryExhausted:
		return builtins.OnRetryExhausted(ctx, c.Store, builtins.RetryExhaustedInput{
			RunID:   in.RunID,
			Message: in.Message,
		})
	default:
		return nil
	}
}
