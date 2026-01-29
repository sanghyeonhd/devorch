package modelresolver

// ParseRequirement는 요청 컨텍스트에서 Requirement를 파싱합니다
func ParseRequirement(reqCtx map[string]any) Requirement {
	req := Requirement{}

	if agentID, ok := reqCtx["agent_id"].(string); ok {
		req.AgentID = agentID
	}
	if category, ok := reqCtx["category"].(string); ok {
		req.Category = category
	}
	if model, ok := reqCtx["model"].(string); ok {
		req.Model = model
	}

	if contextLen, ok := reqCtx["context_length"].(int); ok {
		req.ContextLength = contextLen
	}
	if maxTokens, ok := reqCtx["max_tokens"].(int); ok {
		req.MaxTokens = maxTokens
	}
	if needVision, ok := reqCtx["need_vision"].(bool); ok {
		req.NeedVision = needVision
	}
	if needTool, ok := reqCtx["need_tool"].(bool); ok {
		req.NeedTool = needTool
	}

	if forcedProvider, ok := reqCtx["forced_provider"].(string); ok {
		req.ForcedProvider = forcedProvider
	}

	return req
}

// IsSatisfiedBy는 Resolution이 Requirement를 만족하는지 검사합니다
func (req Requirement) IsSatisfiedBy(res Resolution) bool {
	// 강제 프로바이더 지정 시 확인
	if req.ForcedProvider != "" && res.Provider != req.ForcedProvider {
		return false
	}

	// 모델 명시적 지정 시 확인
	if req.Model != "" && res.Model != req.Model {
		return false
	}

	// 컨텍스트 길이 요구사항 (실제 모델 스펙 확인은 추후 구현)
	// Vision, Tool 지원 확인도 추후 구현

	return true
}

// RequirementBuilder는 Requirement를 빌드하는 헬퍼입니다
type RequirementBuilder struct {
	req Requirement
}

// NewRequirementBuilder는 새 RequirementBuilder를 생성합니다
func NewRequirementBuilder() *RequirementBuilder {
	return &RequirementBuilder{req: Requirement{}}
}

// WithAgent는 AgentID를 설정합니다
func (b *RequirementBuilder) WithAgent(agentID string) *RequirementBuilder {
	b.req.AgentID = agentID
	return b
}

// WithCategory는 Category를 설정합니다
func (b *RequirementBuilder) WithCategory(category string) *RequirementBuilder {
	b.req.Category = category
	return b
}

// WithModel는 Model을 설정합니다
func (b *RequirementBuilder) WithModel(model string) *RequirementBuilder {
	b.req.Model = model
	return b
}

// WithContextLength는 ContextLength를 설정합니다
func (b *RequirementBuilder) WithContextLength(length int) *RequirementBuilder {
	b.req.ContextLength = length
	return b
}

// WithMaxTokens는 MaxTokens를 설정합니다
func (b *RequirementBuilder) WithMaxTokens(tokens int) *RequirementBuilder {
	b.req.MaxTokens = tokens
	return b
}

// WithVision는 NeedVision을 설정합니다
func (b *RequirementBuilder) WithVision(need bool) *RequirementBuilder {
	b.req.NeedVision = need
	return b
}

// WithTool는 NeedTool을 설정합니다
func (b *RequirementBuilder) WithTool(need bool) *RequirementBuilder {
	b.req.NeedTool = need
	return b
}

// WithProvider는 ForcedProvider를 설정합니다
func (b *RequirementBuilder) WithProvider(provider string) *RequirementBuilder {
	b.req.ForcedProvider = provider
	return b
}

// Build는 Requirement를 반환합니다
func (b *RequirementBuilder) Build() Requirement {
	return b.req
}
