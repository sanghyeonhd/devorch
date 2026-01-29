package learner

type Aggregate struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	SampleCount  int
	SuccessCount int
	AvgLatency   float64
	AvgCost      float64
}

// 여러 Feature로부터 Aggregate 생성
func AggregateFeatures(features []Features) []Aggregate {
	type key struct {
		st, sid, os, arch, scenario, provider, model string
	}
	m := make(map[key]*Aggregate)

	for _, f := range features {
		k := key{f.ScopeType, f.ScopeID, f.OS, f.Arch, f.Scenario, f.Provider, f.Model}
		agg, ok := m[k]
		if !ok {
			agg = &Aggregate{
				ScopeType: f.ScopeType,
				ScopeID:   f.ScopeID,
				OS:        f.OS,
				Arch:      f.Arch,
				Scenario:  f.Scenario,
				Provider:  f.Provider,
				Model:     f.Model,
			}
			m[k] = agg
		}
		agg.SampleCount++
		if f.Success {
			agg.SuccessCount++
		}
		agg.AvgLatency += float64(f.LatencyMs)
		agg.AvgCost += f.Cost
	}

	out := make([]Aggregate, 0, len(m))
	for _, agg := range m {
		if agg.SampleCount > 0 {
			agg.AvgLatency /= float64(agg.SampleCount)
			agg.AvgCost /= float64(agg.SampleCount)
		}
		out = append(out, *agg)
	}
	return out
}
