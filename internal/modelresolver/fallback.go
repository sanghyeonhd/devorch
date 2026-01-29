package modelresolver

import "log/slog"

// FallbackResolver는 폴백 로직을 처리합니다
type FallbackResolver struct {
	config DevOrchModelConfig
	logger *slog.Logger
}

// NewFallbackResolver는 새 FallbackResolver를 생성합니다
func NewFallbackResolver(config DevOrchModelConfig, logger *slog.Logger) *FallbackResolver {
	return &FallbackResolver{
		config: config,
		logger: logger,
	}
}

// ResolveFallback는 폴백 Resolution을 반환합니다
func (fr *FallbackResolver) ResolveFallback(req Requirement, failedProvider string) (Resolution, bool) {
	// 카테고리 기반 폴백 탐색
	if req.Category != "" {
		if catCfg, ok := fr.config.Categories[req.Category]; ok {
			for _, prov := range catCfg.Providers {
				if prov != failedProvider {
					fr.logger.Info("fallback provider found",
						"category", req.Category,
						"failed", failedProvider,
						"fallback", prov,
					)
					return Resolution{
						Model:    catCfg.Model,
						Provider: prov,
						Reason:   "fallback from " + failedProvider,
					}, true
				}
			}
		}
	}

	// 전역 프로바이더 우선순위 기반 폴백
	for _, prov := range fr.config.Providers.Priority {
		if prov != failedProvider {
			fr.logger.Info("global fallback provider",
				"failed", failedProvider,
				"fallback", prov,
			)
			return Resolution{
				Model:    fr.config.SystemDefaultModel,
				Provider: prov,
				Reason:   "global fallback from " + failedProvider,
			}, true
		}
	}

	return Resolution{}, false
}

// BuildFallbackChain는 폴백 체인을 구성합니다
func (fr *FallbackResolver) BuildFallbackChain(req Requirement) []Resolution {
	var chain []Resolution

	// 에이전트 특정 설정이 있으면 먼저
	if req.AgentID != "" {
		if agentCfg, ok := fr.config.Agents[req.AgentID]; ok {
			for _, prov := range agentCfg.Providers {
				chain = append(chain, Resolution{
					Model:    agentCfg.Model,
					Provider: prov,
					Reason:   "agent config",
				})
			}
		}
	}

	// 카테고리 설정
	if req.Category != "" {
		if catCfg, ok := fr.config.Categories[req.Category]; ok {
			for _, prov := range catCfg.Providers {
				// 중복 제거
				if !containsProvider(chain, prov) {
					chain = append(chain, Resolution{
						Model:    catCfg.Model,
						Provider: prov,
						Reason:   "category config",
					})
				}
			}
		}
	}

	// 전역 기본값
	for _, prov := range fr.config.Providers.Priority {
		if !containsProvider(chain, prov) {
			chain = append(chain, Resolution{
				Model:    fr.config.SystemDefaultModel,
				Provider: prov,
				Reason:   "system default",
			})
		}
	}

	return chain
}

func containsProvider(chain []Resolution, provider string) bool {
	for _, res := range chain {
		if res.Provider == provider {
			return true
		}
	}
	return false
}
