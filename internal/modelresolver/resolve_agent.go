package modelresolver

import (
	"fmt"
	"log/slog"
	"strings"
)

// AgentModelRequirement는 에이전트 요청 컨텍스트입니다
type AgentModelRequirement struct {
	AgentID       string
	TaskType      string
	RequireVision bool
	RequireTool   bool
	ContextSize   int
	MaxTokens     int
	Preferences   map[string]any
}

// ResolveForAgent는 에이전트 요청에 대한 Resolution을 반환합니다
func (mr *ModelResolver) ResolveForAgent(agentReq AgentModelRequirement) (Resolution, error) {
	mr.logger.Debug("resolving for agent",
		"agent_id", agentReq.AgentID,
		"task_type", agentReq.TaskType,
		"require_vision", agentReq.RequireVision,
	)

	// 에이전트별 설정 확인
	if agentCfg, ok := mr.config.Agents[agentReq.AgentID]; ok {
		provider := mr.selectProviderForAgent(agentCfg.Providers, agentReq)
		return Resolution{
			Model:    agentCfg.Model,
			Provider: provider,
			Reason:   fmt.Sprintf("agent config for %s", agentReq.AgentID),
		}, nil
	}

	// TaskType을 Category로 매핑
	category := mr.mapTaskTypeToCategory(agentReq.TaskType)
	if catCfg, ok := mr.config.Categories[category]; ok {
		provider := mr.selectProviderForAgent(catCfg.Providers, agentReq)
		return Resolution{
			Model:    catCfg.Model,
			Provider: provider,
			Reason:   fmt.Sprintf("category %s for task %s", category, agentReq.TaskType),
		}, nil
	}

	// 기능 기반 선택
	if agentReq.RequireVision {
		if catCfg, ok := mr.config.Categories["visual-engineering"]; ok {
			provider := mr.selectProviderForAgent(catCfg.Providers, agentReq)
			return Resolution{
				Model:    catCfg.Model,
				Provider: provider,
				Reason:   "vision capability required",
			}, nil
		}
	}

	// 기본값
	provider := mr.selectProviderForAgent(mr.config.Providers.Priority, agentReq)
	return Resolution{
		Model:    mr.config.SystemDefaultModel,
		Provider: provider,
		Reason:   "agent default",
	}, nil
}

func (mr *ModelResolver) selectProviderForAgent(providers []string, agentReq AgentModelRequirement) string {
	// Preferences에서 강제 프로바이더 확인
	if forced, ok := agentReq.Preferences["provider"].(string); ok && forced != "" {
		return forced
	}

	if len(providers) == 0 {
		return mr.config.Providers.Priority[0]
	}
	return providers[0]
}

func (mr *ModelResolver) mapTaskTypeToCategory(taskType string) string {
	taskType = strings.ToLower(taskType)

	switch {
	case strings.Contains(taskType, "quick"):
		return "quick"
	case strings.Contains(taskType, "ultra") || strings.Contains(taskType, "brain"):
		return "ultrabrain"
	case strings.Contains(taskType, "visual") || strings.Contains(taskType, "image"):
		return "visual-engineering"
	case strings.Contains(taskType, "complex") || strings.Contains(taskType, "high"):
		return "unspecified-high"
	default:
		return "unspecified-low"
	}
}

// AgentResolver는 에이전트 전용 해석기입니다
type AgentResolver struct {
	resolver *ModelResolver
	agentID  string
	logger   *slog.Logger
}

// NewAgentResolver는 새 AgentResolver를 생성합니다
func NewAgentResolver(resolver *ModelResolver, agentID string, logger *slog.Logger) *AgentResolver {
	return &AgentResolver{
		resolver: resolver,
		agentID:  agentID,
		logger:   logger,
	}
}

// Resolve는 에이전트 컨텍스트로 Resolution을 반환합니다
func (ar *AgentResolver) Resolve(taskType string, opts ...AgentResolveOption) (Resolution, error) {
	req := AgentModelRequirement{
		AgentID:  ar.agentID,
		TaskType: taskType,
	}

	for _, opt := range opts {
		opt(&req)
	}

	return ar.resolver.ResolveForAgent(req)
}

// AgentResolveOption는 에이전트 해석 옵션입니다
type AgentResolveOption func(*AgentModelRequirement)

// WithVisionRequired는 비전 요구사항을 설정합니다
func WithVisionRequired() AgentResolveOption {
	return func(req *AgentModelRequirement) {
		req.RequireVision = true
	}
}

// WithToolRequired는 도구 요구사항을 설정합니다
func WithToolRequired() AgentResolveOption {
	return func(req *AgentModelRequirement) {
		req.RequireTool = true
	}
}

// WithContextSize는 컨텍스트 크기를 설정합니다
func WithContextSize(size int) AgentResolveOption {
	return func(req *AgentModelRequirement) {
		req.ContextSize = size
	}
}

// WithMaxTokensOpt는 최대 토큰을 설정합니다
func WithMaxTokensOpt(tokens int) AgentResolveOption {
	return func(req *AgentModelRequirement) {
		req.MaxTokens = tokens
	}
}

// WithPreference는 환경설정을 설정합니다
func WithPreference(key string, value any) AgentResolveOption {
	return func(req *AgentModelRequirement) {
		if req.Preferences == nil {
			req.Preferences = make(map[string]any)
		}
		req.Preferences[key] = value
	}
}
