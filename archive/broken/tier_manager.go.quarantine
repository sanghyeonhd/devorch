package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TierManager는 AI OS의 4단계 티어 시스템을 관리합니다
// Anthropic API 키 문제로 OpenAI, OpenRouter, Gemini, Copilot, Ollama만 사용
type TierManager struct {
	enabledProviders map[string]bool

	// 4단계 AI 티어
	tier1Orchestrator *AITier // 최고 지휘관 (전략적 의사결정)
	tier2Senior       *AITier // 고급 개발자 (복잡한 설계)
	tier3Regular      *AITier // 일반 개발자 (기본 구현)
	tier4Background   *AITier // 백그라운드 작업자 (모니터링)

	// 시스템 상태
	activeModels map[string]*ModelInstance
	costTracker  *CostTracker
	mu           sync.RWMutex

	// 하드웨어 최적화 설정
	hardwareTier string // ultra, high, mid, low, minimal
}

// AITier는 각 티어별 AI 모델 그룹을 관리합니다
type AITier struct {
	Level        int              `json:"level"`
	Name         string           `json:"name"`
	Purpose      string           `json:"purpose"`
	Models       []*ModelInstance `json:"models"`
	CostRange    CostRange        `json:"cost_range"`
	UsagePattern string           `json:"usage_pattern"`
	Active       bool             `json:"active"`
}

// ModelInstance는 실제 사용 중인 모델 인스턴스입니다
type ModelInstance struct {
	Provider   string    `json:"provider"`
	ModelName  string    `json:"model_name"`
	Tier       int       `json:"tier"`
	CostPer1K  float64   `json:"cost_per_1k"`
	Status     string    `json:"status"` // active, idle, error
	LastUsed   time.Time `json:"last_used"`
	UsageCount int       `json:"usage_count"`
	TotalCost  float64   `json:"total_cost"`
}

// CostRange는 티어별 비용 범위를 정의합니다
type CostRange struct {
	Min    float64 `json:"min"`    // 최저 비용 ($/1K tokens)
	Max    float64 `json:"max"`    // 최고 비용 ($/1K tokens)
	Budget float64 `json:"budget"` // 시간당 예산
}

// CostTracker는 전체 비용을 추적합니다
type CostTracker struct {
	HourlyCost float64   `json:"hourly_cost"`
	DailyCost  float64   `json:"daily_cost"`
	TotalCost  float64   `json:"total_cost"`
	Budget     float64   `json:"budget"`
	StartTime  time.Time `json:"start_time"`
	mu         sync.RWMutex
}

// NewTierManager creates a new tier manager with enabled providers
func NewTierManager(enabledProviders map[string]bool) *TierManager {
	tm := &TierManager{
		enabledProviders: enabledProviders,
		activeModels:     make(map[string]*ModelInstance),
		costTracker:      &CostTracker{StartTime: time.Now()},
	}

	tm.initializeTiers()
	return tm
}

// ConfigureForHardware는 하드웨어 티어에 맞게 모델 구성을 최적화합니다
func (tm *TierManager) ConfigureForHardware(hwConfig interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 하드웨어 티어 감지 (autosetup에서 받은 정보 활용)
	tm.hardwareTier = tm.detectHardwareTier(hwConfig)

	// 하드웨어 티어에 맞는 모델 선택
	return tm.selectOptimalModels()
}

// initializeTiers는 4단계 AI 티어 시스템을 초기화합니다
func (tm *TierManager) initializeTiers() {
	// Tier 1: AI OS Orchestrator (최고 지휘관)
	tm.tier1Orchestrator = &AITier{
		Level:   1,
		Name:    "AI OS Orchestrator",
		Purpose: "전략적 의사결정, 프로젝트 총괄 관리, 아키텍처 설계",
		Models:  tm.getTier1Models(),
		CostRange: CostRange{
			Min:    0.10, // $0.10/1K tokens
			Max:    0.30, // $0.30/1K tokens
			Budget: 5.00, // $5/hour
		},
		UsagePattern: "중요 결정 시점 (1-2시간 간격)",
		Active:       false,
	}

	// Tier 2: Senior AI Developers (고급 개발진)
	tm.tier2Senior = &AITier{
		Level:   2,
		Name:    "Senior AI Developers",
		Purpose: "복잡한 설계, 핵심 로직 구현, 코드 리뷰",
		Models:  tm.getTier2Models(),
		CostRange: CostRange{
			Min:    0.05, // $0.05/1K tokens
			Max:    0.15, // $0.15/1K tokens
			Budget: 3.00, // $3/hour
		},
		UsagePattern: "복잡한 작업 집중 투입 (4-6시간)",
		Active:       false,
	}

	// Tier 3: Regular AI Developers (일반 개발진)
	tm.tier3Regular = &AITier{
		Level:   3,
		Name:    "Regular AI Developers",
		Purpose: "기본 구현, 테스트 코드 작성, 문서화",
		Models:  tm.getTier3Models(),
		CostRange: CostRange{
			Min:    0.01, // $0.01/1K tokens
			Max:    0.05, // $0.05/1K tokens
			Budget: 2.00, // $2/hour
		},
		UsagePattern: "지속적 개발 작업 (8-12시간)",
		Active:       false,
	}

	// Tier 4: Background AI Workers (백그라운드 작업자)
	tm.tier4Background = &AITier{
		Level:   4,
		Name:    "Background AI Workers",
		Purpose: "지속적 모니터링, 기본 작업, 상태 체크",
		Models:  tm.getTier4Models(),
		CostRange: CostRange{
			Min:    0.00, // Free models
			Max:    0.01, // $0.01/1K tokens
			Budget: 0.50, // $0.50/hour
		},
		UsagePattern: "24시간 상시 가동",
		Active:       false,
	}
}

// getTier1Models returns Tier 1 models (excluding Anthropic)
func (tm *TierManager) getTier1Models() []*ModelInstance {
	models := []*ModelInstance{}

	// OpenAI 최고 성능 모델들 (직접 접근)
	if tm.enabledProviders["openai"] {
		models = append(models, &ModelInstance{
			Provider:  "openai",
			ModelName: "gpt-4o",
			Tier:      1,
			CostPer1K: 0.15,
			Status:    "available",
		})

		// GPT-5 시리즈 (사용 가능 시)
		models = append(models, &ModelInstance{
			Provider:  "openai",
			ModelName: "gpt-5.2",
			Tier:      1,
			CostPer1K: 0.30,
			Status:    "available",
		})

		// O1/O3 추론 모델들
		models = append(models, &ModelInstance{
			Provider:  "openai",
			ModelName: "o1",
			Tier:      1,
			CostPer1K: 0.25,
			Status:    "available",
		})
	}

	// OpenRouter를 통한 고급 모델 접근 (Claude 대체)
	if tm.enabledProviders["openrouter"] {
		// Claude Opus 4.5 (OpenRouter 경유)
		models = append(models, &ModelInstance{
			Provider:  "openrouter",
			ModelName: "anthropic/claude-opus-4.5",
			Tier:      1,
			CostPer1K: 0.20,
			Status:    "available",
		})

		// 최신 중국 모델들
		models = append(models, &ModelInstance{
			Provider:  "openrouter",
			ModelName: "qwen/qwen3-max",
			Tier:      1,
			CostPer1K: 0.10,
			Status:    "available",
		})

		// X.AI Grok 시리즈
		models = append(models, &ModelInstance{
			Provider:  "openrouter",
			ModelName: "x-ai/grok-4",
			Tier:      1,
			CostPer1K: 0.18,
			Status:    "available",
		})
	}

	// 로컬 최고 성능 모델 (하드웨어 허용 시)
	if tm.enabledProviders["ollama"] {
		models = append(models, &ModelInstance{
			Provider:  "ollama",
			ModelName: "qwen2.5:32b",
			Tier:      1,
			CostPer1K: 0.00, // 전력비만
			Status:    "local",
		})
	}

	return models
}

// getTier2Models returns Tier 2 models
func (tm *TierManager) getTier2Models() []*ModelInstance {
	models := []*ModelInstance{}

	// OpenAI 고급 모델들
	if tm.enabledProviders["openai"] {
		models = append(models,
			&ModelInstance{Provider: "openai", ModelName: "gpt-4o", Tier: 2, CostPer1K: 0.10, Status: "available"},
			&ModelInstance{Provider: "openai", ModelName: "gpt-4.1", Tier: 2, CostPer1K: 0.08, Status: "available"},
		)
	}

	// OpenRouter 고급 모델들
	if tm.enabledProviders["openrouter"] {
		models = append(models,
			&ModelInstance{Provider: "openrouter", ModelName: "anthropic/claude-3.5-sonnet", Tier: 2, CostPer1K: 0.12, Status: "available"},
			&ModelInstance{Provider: "openrouter", ModelName: "anthropic/claude-3.7-sonnet", Tier: 2, CostPer1K: 0.15, Status: "available"},
			&ModelInstance{Provider: "openrouter", ModelName: "qwen/qwen-2.5-72b-instruct", Tier: 2, CostPer1K: 0.06, Status: "available"},
			&ModelInstance{Provider: "openrouter", ModelName: "meta-llama/llama-3.1-70b-instruct", Tier: 2, CostPer1K: 0.05, Status: "available"},
		)
	}

	// Gemini 고급 모델
	if tm.enabledProviders["gemini"] {
		models = append(models,
			&ModelInstance{Provider: "gemini", ModelName: "gemini-1.5-pro", Tier: 2, CostPer1K: 0.07, Status: "available"},
		)
	}

	// 로컬 고성능 모델
	if tm.enabledProviders["ollama"] {
		models = append(models,
			&ModelInstance{Provider: "ollama", ModelName: "qwen2.5:14b", Tier: 2, CostPer1K: 0.00, Status: "local"},
			&ModelInstance{Provider: "ollama", ModelName: "qwen2.5-coder:14b", Tier: 2, CostPer1K: 0.00, Status: "local"},
		)
	}

	return models
}

// getTier3Models returns Tier 3 models
func (tm *TierManager) getTier3Models() []*ModelInstance {
	models := []*ModelInstance{}

	// OpenAI 경제적 모델들
	if tm.enabledProviders["openai"] {
		models = append(models,
			&ModelInstance{Provider: "openai", ModelName: "gpt-4o-mini", Tier: 3, CostPer1K: 0.03, Status: "available"},
			&ModelInstance{Provider: "openai", ModelName: "gpt-3.5-turbo", Tier: 3, CostPer1K: 0.02, Status: "available"},
		)
	}

	// OpenRouter 중간 성능 모델들
	if tm.enabledProviders["openrouter"] {
		models = append(models,
			&ModelInstance{Provider: "openrouter", ModelName: "anthropic/claude-3.5-haiku", Tier: 3, CostPer1K: 0.04, Status: "available"},
			&ModelInstance{Provider: "openrouter", ModelName: "qwen/qwen-2.5-32b-instruct", Tier: 3, CostPer1K: 0.03, Status: "available"},
		)
	}

	// Gemini 기본 모델
	if tm.enabledProviders["gemini"] {
		models = append(models,
			&ModelInstance{Provider: "gemini", ModelName: "gemini-1.5-flash", Tier: 3, CostPer1K: 0.02, Status: "available"},
		)
	}

	// Copilot 개발 특화
	if tm.enabledProviders["copilot"] {
		models = append(models,
			&ModelInstance{Provider: "copilot", ModelName: "copilot-chat", Tier: 3, CostPer1K: 0.00, Status: "oauth"},
		)
	}

	// 로컬 중간 성능 모델
	if tm.enabledProviders["ollama"] {
		models = append(models,
			&ModelInstance{Provider: "ollama", ModelName: "qwen2.5:7b", Tier: 3, CostPer1K: 0.00, Status: "local"},
			&ModelInstance{Provider: "ollama", ModelName: "qwen2.5-coder:7b", Tier: 3, CostPer1K: 0.00, Status: "local"},
		)
	}

	return models
}

// getTier4Models returns Tier 4 models (mostly free/local)
func (tm *TierManager) getTier4Models() []*ModelInstance {
	models := []*ModelInstance{}

	// OpenRouter 무료 모델들
	if tm.enabledProviders["openrouter"] {
		models = append(models,
			&ModelInstance{Provider: "openrouter", ModelName: "qwen/qwen-2.5-7b-instruct:free", Tier: 4, CostPer1K: 0.00, Status: "free"},
			&ModelInstance{Provider: "openrouter", ModelName: "meta-llama/llama-3.2-3b-instruct:free", Tier: 4, CostPer1K: 0.00, Status: "free"},
			&ModelInstance{Provider: "openrouter", ModelName: "microsoft/phi-3-mini-128k-instruct:free", Tier: 4, CostPer1K: 0.00, Status: "free"},
		)
	}

	// 로컬 경량 모델들 (24시간 가동 가능)
	if tm.enabledProviders["ollama"] {
		models = append(models,
			&ModelInstance{Provider: "ollama", ModelName: "qwen2.5:3b", Tier: 4, CostPer1K: 0.00, Status: "local"},
			&ModelInstance{Provider: "ollama", ModelName: "phi3:mini", Tier: 4, CostPer1K: 0.00, Status: "local"},
			&ModelInstance{Provider: "ollama", ModelName: "gemma:2b", Tier: 4, CostPer1K: 0.00, Status: "local"},
		)
	}

	return models
}

// GetAvailableModel returns the best available model for the specified tier
func (tm *TierManager) GetAvailableModel(tier Tier) (ModelConfig, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var aiTier *AITier
	switch tier {
	case TierOrchestrator:
		aiTier = tm.tier1Orchestrator
	case TierSenior:
		aiTier = tm.tier2Senior
	case TierRegular:
		aiTier = tm.tier3Regular
	case TierBackground:
		aiTier = tm.tier4Background
	default:
		return ModelConfig{}, fmt.Errorf("unknown tier: %d", tier)
	}

	// 사용 가능한 첫 번째 모델 반환
	for _, model := range aiTier.Models {
		if model.Status == "available" || model.Status == "local" || model.Status == "free" || model.Status == "oauth" {
			return ModelConfig{
				Provider: model.Provider,
				Model:    model.ModelName,
				Tier:     int(tier),
			}, nil
		}
	}

	return ModelConfig{}, fmt.Errorf("no available models for tier %d", tier)
}

// ActivateAllTiers activates all AI tiers for autonomous operation
func (tm *TierManager) ActivateAllTiers(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 각 티어 활성화
	tiers := []*AITier{tm.tier1Orchestrator, tm.tier2Senior, tm.tier3Regular, tm.tier4Background}

	for _, tier := range tiers {
		if err := tm.activateTier(tier); err != nil {
			return fmt.Errorf("failed to activate tier %d: %w", tier.Level, err)
		}
	}

	return nil
}

// ShutdownAll safely shuts down all AI tiers
func (tm *TierManager) ShutdownAll() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 모든 티어 비활성화
	tm.tier1Orchestrator.Active = false
	tm.tier2Senior.Active = false
	tm.tier3Regular.Active = false
	tm.tier4Background.Active = false

	return nil
}

// GetTierStatus returns the current status of all tiers
func (tm *TierManager) GetTierStatus() map[int]*AITier {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return map[int]*AITier{
		1: tm.tier1Orchestrator,
		2: tm.tier2Senior,
		3: tm.tier3Regular,
		4: tm.tier4Background,
	}
}

// GetCostSummary returns current cost tracking information
func (tm *TierManager) GetCostSummary() *CostTracker {
	return tm.costTracker
}

// Helper methods
func (tm *TierManager) detectHardwareTier(hwConfig interface{}) string {
	// autosetup에서 받은 하드웨어 정보 분석
	// 임시로 "mid" 반환 (실제로는 hwConfig 파싱)
	return "mid"
}

func (tm *TierManager) selectOptimalModels() error {
	// 하드웨어 티어에 따른 최적 모델 선택
	switch tm.hardwareTier {
	case "ultra":
		// 모든 고성능 모델 활용
	case "high":
		// 대부분의 모델 활용
	case "mid":
		// 균형잡힌 모델 선택
	case "low":
		// 경량 모델 위주
	case "minimal":
		// 로컬 모델만
	}
	return nil
}

func (tm *TierManager) activateTier(tier *AITier) error {
	// 티어 활성화 로직
	tier.Active = true
	return nil
}
