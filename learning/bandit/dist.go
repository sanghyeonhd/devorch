package bandit

import (
	"math"
	"math/rand"
)

// BetaDist represents a Beta distribution
type BetaDist struct {
	Alpha float64
	Beta  float64
}

// NewBetaDist creates a new Beta distribution with given parameters
func NewBetaDist(alpha, beta float64) *BetaDist {
	if alpha <= 0 {
		alpha = 1
	}
	if beta <= 0 {
		beta = 1
	}
	return &BetaDist{Alpha: alpha, Beta: beta}
}

// Sample draws a sample from the Beta distribution
func (d *BetaDist) Sample(rng *rand.Rand) float64 {
	x := gammaSample(rng, d.Alpha)
	y := gammaSample(rng, d.Beta)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

// Mean returns the mean of the distribution
func (d *BetaDist) Mean() float64 {
	return d.Alpha / (d.Alpha + d.Beta)
}

// SampleBeta samples from Beta(alpha, beta) distribution
func SampleBeta(alpha, beta float64) float64 {
	x := sampleGamma(alpha)
	y := sampleGamma(beta)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

// gammaSample generates a sample from Gamma(alpha, 1) using Marsaglia and Tsang's method
func gammaSample(rng *rand.Rand, alpha float64) float64 {
	if alpha < 1 {
		return gammaSample(rng, alpha+1) * math.Pow(rng.Float64(), 1.0/alpha)
	}

	d := alpha - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		var x, v float64
		for {
			x = rng.NormFloat64()
			v = 1 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng.Float64()

		if u < 1-0.0331*(x*x)*(x*x) {
			return d * v
		}

		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

// sampleGamma generates a sample from Gamma(k, 1) using global rand
func sampleGamma(k float64) float64 {
	if k <= 0 {
		return 0
	}
	if k < 1 {
		u := rand.Float64()
		return sampleGamma(k+1) * math.Pow(u, 1.0/k)
	}

	d := k - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)

	for {
		x := rand.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rand.Float64()

		if u < 1-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
