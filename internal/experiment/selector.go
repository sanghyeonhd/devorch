package experiment

import (
	"math"
	"strconv"
)

type DecisionRule struct {
	MinSamplesPerVariant int
	MinUtilityDelta      float64
}

func DefaultDecisionRule() DecisionRule {
	return DecisionRule{
		MinSamplesPerVariant: 30,
		MinUtilityDelta:      0.03,
	}
}

func PickWinner(evals []Eval, rule DecisionRule) (winner string, ok bool, reason string) {
	if len(evals) < 2 {
		return "", false, "need >=2 variants"
	}
	// ensure min samples
	for _, e := range evals {
		if e.N < rule.MinSamplesPerVariant {
			return "", false, "insufficient samples"
		}
	}

	// pick best utility
	best := evals[0]
	second := Eval{Utility: -math.MaxFloat64}
	for _, e := range evals {
		if e.Utility > best.Utility {
			second = best
			best = e
		} else if e.Utility > second.Utility && e.VariantKey != best.VariantKey {
			second = e
		}
	}

	delta := best.Utility - second.Utility
	if delta < rule.MinUtilityDelta {
		return "", false, "delta too small"
	}

	reason = "winner=" + best.VariantKey +
		" utility=" + strconv.FormatFloat(best.Utility, 'f', 3, 64) +
		" second=" + second.VariantKey +
		" delta=" + strconv.FormatFloat(delta, 'f', 3, 64)
	return best.VariantKey, true, reason
}
