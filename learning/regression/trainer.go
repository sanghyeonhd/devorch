package regression

// Online SGD trainer (MSE loss)
type Trainer struct {
	LR       float64 // learning rate
	L2       float64 // weight decay
	ClipGrad float64
}

func DefaultTrainer() Trainer {
	return Trainer{
		LR:       0.05,
		L2:       0.0001,
		ClipGrad: 1.0,
	}
}

func (t Trainer) Update(m *LinearModel, x []float64, yTrue float64) {
	if m == nil || len(m.W) == 0 {
		return
	}

	yPred := m.Predict(x)
	err := (yPred - yTrue) // d/dy (1/2 (yPred-yTrue)^2)

	// bias update
	gb := err
	gb = clip(gb, t.ClipGrad)
	m.B -= t.LR * gb

	// weight update
	n := len(m.W)
	if len(x) < n {
		n = len(x)
	}
	for i := 0; i < n; i++ {
		gi := err*x[i] + t.L2*m.W[i]
		gi = clip(gi, t.ClipGrad)
		m.W[i] -= t.LR * gi
	}
}

func clip(v, c float64) float64 {
	if c <= 0 {
		return v
	}
	if v > c {
		return c
	}
	if v < -c {
		return -c
	}
	return v
}
