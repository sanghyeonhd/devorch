package modelresolver

import (
	"math/rand"
	"sync"
	"time"
)

// SelectionPolicy는 모델 선택 정책을 정의합니다
type SelectionPolicy string

const (
	// PolicyBandit는 Thompson Sampling 기반 선택
	PolicyBandit SelectionPolicy = "bandit"
	// PolicyPriority는 우선순위 기반 선택
	PolicyPriority SelectionPolicy = "priority"
	// PolicyRoundRobin는 라운드로빈 선택
	PolicyRoundRobin SelectionPolicy = "round_robin"
	// PolicyRandom는 랜덤 선택
	PolicyRandom SelectionPolicy = "random"
)

// PolicyConfig는 정책별 설정입니다
type PolicyConfig struct {
	Policy          SelectionPolicy `json:"policy"`
	BanditExplore   float64         `json:"bandit_explore,omitempty"`
	PriorityWeights map[string]int  `json:"priority_weights,omitempty"`
}

// DefaultPolicyConfig는 기본 정책 설정을 반환합니다
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		Policy:        PolicyBandit,
		BanditExplore: 0.1,
		PriorityWeights: map[string]int{
			"anthropic":  100,
			"openai":     90,
			"google":     80,
			"opencode":   70,
			"copilot":    60,
			"openrouter": 50,
			"ollama":     40,
		},
	}
}

// PolicySelector는 정책 기반 선택을 수행합니다
type PolicySelector struct {
	config        PolicyConfig
	mu            sync.Mutex
	roundRobinIdx int
	rng           *rand.Rand
}

// NewPolicySelector는 새 PolicySelector를 생성합니다
func NewPolicySelector(config PolicyConfig) *PolicySelector {
	return &PolicySelector{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectFromChain는 체인에서 정책에 따라 선택합니다
func (ps *PolicySelector) SelectFromChain(chain []Resolution) Resolution {
	if len(chain) == 0 {
		return Resolution{}
	}

	switch ps.config.Policy {
	case PolicyPriority:
		return ps.selectByPriority(chain)
	case PolicyRoundRobin:
		return ps.selectByRoundRobin(chain)
	case PolicyRandom:
		return ps.selectByRandom(chain)
	case PolicyBandit:
		// Bandit는 외부에서 선택하므로 첫 번째 반환
		return chain[0]
	default:
		return chain[0]
	}
}

func (ps *PolicySelector) selectByPriority(chain []Resolution) Resolution {
	var best Resolution
	bestWeight := -1

	for _, res := range chain {
		if weight, ok := ps.config.PriorityWeights[res.Provider]; ok {
			if weight > bestWeight {
				bestWeight = weight
				best = res
			}
		}
	}

	if bestWeight == -1 && len(chain) > 0 {
		return chain[0]
	}
	return best
}

func (ps *PolicySelector) selectByRoundRobin(chain []Resolution) Resolution {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	idx := ps.roundRobinIdx % len(chain)
	ps.roundRobinIdx++
	return chain[idx]
}

func (ps *PolicySelector) selectByRandom(chain []Resolution) Resolution {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	idx := ps.rng.Intn(len(chain))
	return chain[idx]
}
