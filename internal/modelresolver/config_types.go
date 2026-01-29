package modelresolver

// DevOrchModelConfig는 모델 선택에 필요한 설정 구조체
type DevOrchModelConfig struct {
	SystemDefaultModel string `json:"systemDefaultModel,omitempty"`

	Providers ProvidersConfig `json:"providers,omitempty"`

	Categories map[string]CategoryConfig `json:"categories,omitempty"`
	Agents     map[string]AgentConfig    `json:"agents,omitempty"`
}

// ProvidersConfig는 프로바이더 설정
type ProvidersConfig struct {
	Priority   []string          `json:"priority,omitempty"`
	Namespaces map[string]string `json:"namespaces,omitempty"`
}

// CategoryConfig는 카테고리별 모델 설정
type CategoryConfig struct {
	Model     string   `json:"model,omitempty"`
	Providers []string `json:"providers,omitempty"`
}

// AgentConfig는 에이전트별 모델 설정
type AgentConfig struct {
	Model     string   `json:"model,omitempty"`
	Providers []string `json:"providers,omitempty"`
}

// DefaultModelConfig는 기본 모델 설정을 반환합니다
func DefaultModelConfig() DevOrchModelConfig {
	return DevOrchModelConfig{
		SystemDefaultModel: "claude-sonnet",
		Providers: ProvidersConfig{
			Priority: []string{"anthropic", "openai", "google", "opencode", "copilot", "openrouter", "ollama"},
			Namespaces: map[string]string{
				"anthropic":  "anthropic/",
				"openai":     "openai/",
				"google":     "google/",
				"opencode":   "opencode/",
				"copilot":    "github-copilot/",
				"openrouter": "openrouter/",
				"ollama":     "ollama/",
			},
		},
		Categories: map[string]CategoryConfig{
			"quick":              {Model: "claude-haiku", Providers: []string{"anthropic", "opencode", "openai", "google"}},
			"ultrabrain":         {Model: "gpt-4o-mini", Providers: []string{"openai", "anthropic", "google"}},
			"visual-engineering": {Model: "gemini-3-flash", Providers: []string{"google", "openai", "anthropic"}},
			"unspecified-high":   {Model: "claude-opus", Providers: []string{"anthropic", "openai"}},
			"unspecified-low":    {Model: "claude-sonnet", Providers: []string{"anthropic", "openai"}},
		},
		Agents: map[string]AgentConfig{},
	}
}
