// Package billing provides pricing tables and cost calculation utilities.
// Phase 32: Provider/Model pricing table with overrides
package billing

import (
	"strings"
	"sync"
)

// TokenPricing represents per-token pricing for a model.
type TokenPricing struct {
	USDPer1KInput  float64
	USDPer1KOutput float64
}

// PricingTable manages pricing for providers and models.
type PricingTable struct {
	mu sync.RWMutex

	// Key formats:
	// - "provider/model" (exact match)
	// - "provider/*"     (provider default)
	// - "*/*"            (global default)
	prices map[string]TokenPricing
}

// NewPricingTable creates a new pricing table with default values.
func NewPricingTable() *PricingTable {
	t := &PricingTable{
		prices: make(map[string]TokenPricing),
	}
	t.loadDefaults()
	return t
}

func (t *PricingTable) loadDefaults() {
	// Local providers are free
	t.prices["ollama/*"] = TokenPricing{USDPer1KInput: 0, USDPer1KOutput: 0}
	t.prices["local/*"] = TokenPricing{USDPer1KInput: 0, USDPer1KOutput: 0}

	// OpenAI (approximate)
	t.prices["openai/*"] = TokenPricing{USDPer1KInput: 0.002, USDPer1KOutput: 0.004}
	t.prices["openai/gpt-4"] = TokenPricing{USDPer1KInput: 0.03, USDPer1KOutput: 0.06}
	t.prices["openai/gpt-4-turbo"] = TokenPricing{USDPer1KInput: 0.01, USDPer1KOutput: 0.03}
	t.prices["openai/gpt-3.5-turbo"] = TokenPricing{USDPer1KInput: 0.0005, USDPer1KOutput: 0.0015}

	// Anthropic (approximate)
	t.prices["anthropic/*"] = TokenPricing{USDPer1KInput: 0.003, USDPer1KOutput: 0.015}
	t.prices["anthropic/claude-3-opus"] = TokenPricing{USDPer1KInput: 0.015, USDPer1KOutput: 0.075}
	t.prices["anthropic/claude-3-sonnet"] = TokenPricing{USDPer1KInput: 0.003, USDPer1KOutput: 0.015}
	t.prices["anthropic/claude-3-haiku"] = TokenPricing{USDPer1KInput: 0.00025, USDPer1KOutput: 0.00125}

	// Google (approximate)
	t.prices["google/*"] = TokenPricing{USDPer1KInput: 0.0015, USDPer1KOutput: 0.003}

	// OpenRouter (average)
	t.prices["openrouter/*"] = TokenPricing{USDPer1KInput: 0.002, USDPer1KOutput: 0.006}

	// Global default
	t.prices["*/*"] = TokenPricing{USDPer1KInput: 0.002, USDPer1KOutput: 0.004}
}

// Upsert adds or updates pricing for a provider/model combination.
func (t *PricingTable) Upsert(provider, model string, p TokenPricing) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := normalizeKey(provider, model)
	t.prices[k] = p
}

// Resolve returns the pricing for a provider/model combination.
// Resolution order: exact match -> provider default -> global default.
func (t *PricingTable) Resolve(provider, model string) TokenPricing {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 1) Exact provider/model match
	if p, ok := t.prices[normalizeKey(provider, model)]; ok {
		return p
	}

	// 2) Provider default (provider/*)
	if p, ok := t.prices[normalizeKey(provider, "*")]; ok {
		return p
	}

	// 3) Global default (*/*) - always exists
	return t.prices["*/*"]
}

// EstimateCost calculates the cost for given token counts.
func (t *PricingTable) EstimateCost(provider, model string, tokensIn, tokensOut int64) float64 {
	p := t.Resolve(provider, model)
	cIn := (float64(tokensIn) / 1000.0) * p.USDPer1KInput
	cOut := (float64(tokensOut) / 1000.0) * p.USDPer1KOutput
	return cIn + cOut
}

func normalizeKey(provider, model string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	model = strings.TrimSpace(strings.ToLower(model))
	if provider == "" {
		provider = "*"
	}
	if model == "" {
		model = "*"
	}
	return provider + "/" + model
}
