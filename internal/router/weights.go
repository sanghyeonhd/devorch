package router

type Weights struct {
	Quality float64
	Latency float64
	Cost    float64
	Success float64
}

func DefaultWeights() Weights {
	return Weights{
		Quality: 0.55,
		Latency: 0.25,
		Cost:    0.10,
		Success: 0.10,
	}
}
