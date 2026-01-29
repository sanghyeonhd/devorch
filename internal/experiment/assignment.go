package experiment

import "sort"

type WeightedKey struct {
	Key    string
	Weight float64
}

func ChooseVariant(seed string, subject string, traffic float64, weights []WeightedKey) (string, bool) {
	// traffic gate
	r := hashToFloat01(seed, subject)
	if r > traffic {
		return "", false // not in experiment
	}

	// normalize weights
	total := 0.0
	for _, w := range weights {
		if w.Weight > 0 {
			total += w.Weight
		}
	}
	if total <= 0 {
		return "", false
	}

	normalized := make([]WeightedKey, 0, len(weights))
	for _, w := range weights {
		if w.Weight <= 0 {
			continue
		}
		normalized = append(normalized, WeightedKey{Key: w.Key, Weight: w.Weight / total})
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Key < normalized[j].Key })

	x := hashToFloat01(seed+"|variant", subject)
	acc := 0.0
	for _, w := range normalized {
		acc += w.Weight
		if x <= acc {
			return w.Key, true
		}
	}
	return normalized[len(normalized)-1].Key, true
}
