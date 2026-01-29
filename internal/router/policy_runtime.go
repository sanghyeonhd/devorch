package router

import (
	"context"
	"math/rand"
	"runtime"
	"time"

	"devorch/internal/router/scope"
)

// PolicyRouter: Policy 기반 provider/model 선택
type PolicyRouter struct {
	Store *PolicyStore
}

func NewPolicyRouter(store *PolicyStore) *PolicyRouter {
	return &PolicyRouter{Store: store}
}

type RouteResult struct {
	Provider string
	Model    string
	Weight   float64
}

// Route: 현재 context에서 가장 적절한 provider/model 선택
// fallback candidates = 정책이 없을 때 쓸 후보 목록
func (r *PolicyRouter) Route(ctx context.Context, scenario string, fallback []RouteResult) (RouteResult, error) {
	scopes := scope.Candidates(ctx)
	osName := runtime.GOOS
	archName := runtime.GOARCH

	for _, sk := range scopes {
		policies, err := r.Store.Query(ctx, string(sk.Type), sk.ID, osName, archName, scenario)
		if err != nil {
			return RouteResult{}, err
		}
		if len(policies) == 0 {
			continue
		}

		// decay 적용 후 weighted selection
		selected := r.weightedSelect(policies)
		if selected.Provider != "" {
			return selected, nil
		}
	}

	if len(fallback) == 0 {
		return RouteResult{}, nil
	}
	return fallback[rand.Intn(len(fallback))], nil
}

// decayFactor: 오래된 샘플은 영향력 감소 (learner와 중복이지만 cycle 방지)
func decayFactor(updatedAt time.Time) float64 {
	days := time.Since(updatedAt).Hours() / 24.0
	if days <= 1 {
		return 1.0
	}
	f := 1.0 / (1.0 + days/5.0)
	if f < 0.05 {
		return 0.05
	}
	return f
}

func (r *PolicyRouter) weightedSelect(policies []Policy) RouteResult {
	var total float64
	now := time.Now()

	for i := range policies {
		decay := decayFactor(policies[i].UpdatedAt)
		policies[i].Weight *= decay
		total += policies[i].Weight
	}

	if total <= 0 {
		return RouteResult{}
	}

	_ = now // 향후 추가 로직용

	pick := rand.Float64() * total
	var cum float64
	for _, p := range policies {
		cum += p.Weight
		if pick <= cum {
			return RouteResult{
				Provider: p.Provider,
				Model:    p.Model,
				Weight:   p.Weight,
			}
		}
	}

	// 마지막 하나 리턴
	last := policies[len(policies)-1]
	return RouteResult{
		Provider: last.Provider,
		Model:    last.Model,
		Weight:   last.Weight,
	}
}
