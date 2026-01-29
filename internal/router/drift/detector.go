package drift

import (
	"math"
	"time"
)

type Decision struct {
	HasBaseline    bool
	ShouldRollback bool

	Baseline WindowStats
	Recent   WindowStats

	Score     float64
	Threshold float64

	Reason string
}

type Detector struct {
	Cfg            Config
	lastRollbackAt map[string]time.Time
}

func NewDetector(cfg Config) *Detector {
	return &Detector{
		Cfg:            cfg,
		lastRollbackAt: map[string]time.Time{},
	}
}

func scopeKey(scopeType, scopeID, os, arch, scenario string) string {
	return scopeType + "|" + scopeID + "|" + os + "|" + arch + "|" + scenario
}

func (d *Detector) Evaluate(scopeType, scopeID, os, arch, scenario string, baseline, recent WindowStats) Decision {
	dec := Decision{
		Baseline:  baseline,
		Recent:    recent,
		Threshold: d.Cfg.ScoreThreshold,
	}

	// 기준선 부족
	if baseline.Samples < d.Cfg.BaselineMinSamples || recent.Samples < d.Cfg.RecentSamples {
		dec.HasBaseline = false
		dec.ShouldRollback = false
		dec.Reason = "insufficient_samples"
		return dec
	}
	dec.HasBaseline = true

	// 드리프트 구성요소
	successDrop := baseline.SuccessRate - recent.SuccessRate
	latencyIncreaseRatio := ratioIncrease(baseline.AvgLatencyMs, recent.AvgLatencyMs)
	costIncreaseRatio := ratioIncrease(baseline.AvgCost, recent.AvgCost)

	// 규칙 기반 컷
	ruleTriggered := false
	if successDrop >= d.Cfg.MinSuccessDrop {
		ruleTriggered = true
	}
	if latencyIncreaseRatio >= d.Cfg.MaxLatencyIncreaseRatio {
		ruleTriggered = true
	}
	if costIncreaseRatio >= d.Cfg.MaxCostIncreaseRatio {
		ruleTriggered = true
	}

	// 간단 합성 점수
	score := 0.0
	score += driftClamp01(successDrop / d.Cfg.MinSuccessDrop)
	score += driftClamp01(latencyIncreaseRatio / d.Cfg.MaxLatencyIncreaseRatio)
	score += driftClamp01(costIncreaseRatio / d.Cfg.MaxCostIncreaseRatio)

	dec.Score = score

	if !ruleTriggered && score < d.Cfg.ScoreThreshold {
		dec.ShouldRollback = false
		dec.Reason = "no_drift"
		return dec
	}

	// cooldown 체크
	k := scopeKey(scopeType, scopeID, os, arch, scenario)
	if t, ok := d.lastRollbackAt[k]; ok {
		if time.Since(t) < d.Cfg.Cooldown {
			dec.ShouldRollback = false
			dec.Reason = "cooldown_active"
			return dec
		}
	}

	dec.ShouldRollback = true
	dec.Reason = "drift_detected"
	return dec
}

func (d *Detector) MarkRollback(scopeType, scopeID, os, arch, scenario string) {
	d.lastRollbackAt[scopeKey(scopeType, scopeID, os, arch, scenario)] = time.Now()
}

func ratioIncrease(base, now float64) float64 {
	if base <= 0 {
		if now <= 0 {
			return 0
		}
		return 1
	}
	return math.Max(0, (now-base)/base)
}

func driftClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
