// Package bench provides cost estimation utilities.
// Phase 31: Simple token-based cost estimation
// Phase 32: Extended with PricingTable support
package bench

// CostInput represents input for cost estimation.
type CostInput struct {
	Provider  string
	Model     string
	TokensIn  int64
	TokensOut int64

	// Pricing table for accurate cost calculation (Phase 32)
	Pricing PricingResolver
}

// PricingResolver resolves pricing for a provider/model combination.
type PricingResolver interface {
	Resolve(provider, model string) TokenPricing
}

// TokenPricing represents per-token pricing.
type TokenPricing struct {
	USDPer1KInput  float64
	USDPer1KOutput float64
}

// EstimateCostUSD estimates the cost in USD for a given input.
func EstimateCostUSD(in CostInput) float64 {
	// Local providers are free
	if in.Provider == "ollama" || in.Provider == "local" {
		return 0.0
	}

	if in.Pricing != nil {
		p := in.Pricing.Resolve(in.Provider, in.Model)
		cIn := (float64(in.TokensIn) / 1000.0) * p.USDPer1KInput
		cOut := (float64(in.TokensOut) / 1000.0) * p.USDPer1KOutput
		return cIn + cOut
	}

	// Fallback: rough default pricing
	total := float64(in.TokensIn + in.TokensOut)
	return (total / 1000.0) * 0.002
}

// EstimateCostMicroUSD returns cost in micro-USD (millionths of a dollar).
func EstimateCostMicroUSD(in CostInput) int64 {
	usd := EstimateCostUSD(in)
	return int64(usd * 1_000_000)
}
