package drift

import "time"

type Config struct {
	Enabled bool

	// 기준선/최근 비교 윈도우
	BaselineMinSamples int
	RecentSamples      int

	// 임계값
	MinSuccessDrop          float64
	MaxLatencyIncreaseRatio float64
	MaxCostIncreaseRatio    float64

	// drift 점수 임계값
	ScoreThreshold float64

	// 감지 주기
	Cooldown time.Duration
}

func DefaultConfig() Config {
	return Config{
		Enabled: true,

		BaselineMinSamples: 100,
		RecentSamples:      30,

		MinSuccessDrop:          0.08,
		MaxLatencyIncreaseRatio: 0.25,
		MaxCostIncreaseRatio:    0.30,
		ScoreThreshold:          1.0,
		Cooldown:                10 * time.Minute,
	}
}
