package okaon

// ModelAggregate는 모델별 집계 결과
type ModelAggregate struct {
	Provider string
	Model    string

	TotalRuns    int64
	SuccessRuns  int64
	FailureRuns  int64
	AvgLatencyMs float64
	AvgReward01  float64
	TotalCostUSD float64
}

// CategoryAggregate는 카테고리별 집계 결과
type CategoryAggregate struct {
	Category string

	TotalRuns    int64
	UniqueModels int64
	AvgReward01  float64
}
