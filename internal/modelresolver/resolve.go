package modelresolver

import "log/slog"

// Resolver는 모델 해석기 인터페이스입니다
type Resolver interface {
	Resolve(req Requirement) (Resolution, error)
	ResolveWithFallback(req Requirement, failedProvider string) (Resolution, error)
}

// ModelResolver는 Requirement를 Resolution으로 변환합니다
type ModelResolver struct {
	config           DevOrchModelConfig
	policyConfig     PolicyConfig
	fallbackResolver *FallbackResolver
	policySelector   *PolicySelector
	logger           *slog.Logger
}

// NewModelResolver는 새 ModelResolver를 생성합니다
func NewModelResolver(config DevOrchModelConfig, policyConfig PolicyConfig, logger *slog.Logger) *ModelResolver {
	return &ModelResolver{
		config:           config,
		policyConfig:     policyConfig,
		fallbackResolver: NewFallbackResolver(config, logger),
		policySelector:   NewPolicySelector(policyConfig),
		logger:           logger,
	}
}

// Resolve는 Requirement에서 Resolution을 반환합니다
func (mr *ModelResolver) Resolve(req Requirement) (Resolution, error) {
	mr.logger.Debug("resolving model",
		"agent_id", req.AgentID,
		"category", req.Category,
		"model", req.Model,
	)

	// 1. 명시적 모델 지정 시
	if req.Model != "" && req.ForcedProvider != "" {
		return Resolution{
			Model:    req.Model,
			Provider: req.ForcedProvider,
			Reason:   "explicit model and provider",
		}, nil
	}

	// 2. 에이전트 특정 설정 확인
	if req.AgentID != "" {
		if agentCfg, ok := mr.config.Agents[req.AgentID]; ok {
			provider := mr.selectProvider(agentCfg.Providers, req.ForcedProvider)
			return Resolution{
				Model:    agentCfg.Model,
				Provider: provider,
				Reason:   "agent specific config",
			}, nil
		}
	}

	// 3. 카테고리 기반 해석
	if req.Category != "" {
		if catCfg, ok := mr.config.Categories[req.Category]; ok {
			provider := mr.selectProvider(catCfg.Providers, req.ForcedProvider)
			return Resolution{
				Model:    catCfg.Model,
				Provider: provider,
				Reason:   "category config",
			}, nil
		}
	}

	// 4. 명시적 모델만 지정된 경우
	if req.Model != "" {
		provider := mr.selectProvider(mr.config.Providers.Priority, req.ForcedProvider)
		return Resolution{
			Model:    req.Model,
			Provider: provider,
			Reason:   "explicit model",
		}, nil
	}

	// 5. 시스템 기본값
	provider := mr.selectProvider(mr.config.Providers.Priority, req.ForcedProvider)
	return Resolution{
		Model:    mr.config.SystemDefaultModel,
		Provider: provider,
		Reason:   "system default",
	}, nil
}

// ResolveWithFallback는 실패한 프로바이더를 제외하고 Resolution을 반환합니다
func (mr *ModelResolver) ResolveWithFallback(req Requirement, failedProvider string) (Resolution, error) {
	res, ok := mr.fallbackResolver.ResolveFallback(req, failedProvider)
	if !ok {
		// 폴백 실패 시 시스템 기본값
		return Resolution{
			Model:    mr.config.SystemDefaultModel,
			Provider: mr.config.Providers.Priority[0],
			Reason:   "no fallback available, using system default",
		}, nil
	}
	return res, nil
}

// GetFallbackChain는 요청에 대한 폴백 체인을 반환합니다
func (mr *ModelResolver) GetFallbackChain(req Requirement) []Resolution {
	return mr.fallbackResolver.BuildFallbackChain(req)
}

func (mr *ModelResolver) selectProvider(providers []string, forced string) string {
	if forced != "" {
		return forced
	}
	if len(providers) == 0 {
		if len(mr.config.Providers.Priority) > 0 {
			return mr.config.Providers.Priority[0]
		}
		return "anthropic" // 최종 기본값
	}
	return providers[0]
}
