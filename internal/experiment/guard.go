package experiment

type Guard struct {
	MinSuccessRate float64
	MinAvgQuality  float64
	MaxAvgLatency  float64
}

func DefaultGuard() Guard {
	return Guard{
		MinSuccessRate: 0.80,
		MinAvgQuality:  0.55,
		MaxAvgLatency:  8000,
	}
}

func ShouldStop(evals []Eval, g Guard) (bool, string) {
	for _, e := range evals {
		if e.SuccessRate < g.MinSuccessRate {
			return true, "success_rate too low for variant " + e.VariantKey
		}
		if e.AvgQuality < g.MinAvgQuality {
			return true, "quality too low for variant " + e.VariantKey
		}
		if e.AvgLatencyMS > g.MaxAvgLatency {
			return true, "latency too high for variant " + e.VariantKey
		}
	}
	return false, ""
}
