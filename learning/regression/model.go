package regression

import "math"

// 아주 단순한 선형모델: y = w·x + b
// feature는 이미 정규화되어 들어온다고 가정(예: 0..1)
type LinearModel struct {
	W []float64
	B float64
}

func NewLinearModel(dim int) *LinearModel {
	if dim <= 0 {
		dim = 1
	}
	return &LinearModel{
		W: make([]float64, dim),
		B: 0,
	}
}

func (m *LinearModel) Predict(x []float64) float64 {
	if len(x) == 0 || len(m.W) == 0 {
		return regClamp01(m.B)
	}
	n := len(m.W)
	if len(x) < n {
		n = len(x)
	}
	y := m.B
	for i := 0; i < n; i++ {
		y += m.W[i] * x[i]
	}
	return regClamp01(y)
}

func regClamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
