package drift

type WindowStats struct {
	Samples      int
	SuccessRate  float64
	AvgLatencyMs float64
	AvgCost      float64
}
