package learning

import (
	"context"
	"math/rand"
	"time"

	"devorch/learning/bandit"
)

// ThompsonSamplingBandit implements Thompson Sampling algorithm for multi-armed bandit
type ThompsonSamplingBandit struct {
	store       bandit.Store
	rng         *rand.Rand
	ctx         context.Context
	fingerprint string
	category    string
}

// NewThompsonSamplingBandit creates a new Thompson Sampling bandit
func NewThompsonSamplingBandit(okaonStore OkAONStore) *ThompsonSamplingBandit {
	return &ThompsonSamplingBandit{
		store: &BanditStore{
			Ctx:         context.Background(),
			OkStore:     okaonStore,
			Fingerprint: "default",
			Category:    "agent-orchestration",
		},
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		ctx:         context.Background(),
		fingerprint: "default",
		category:    "agent-orchestration",
	}
}

// Select chooses the best arm using Thompson Sampling
func (ts *ThompsonSamplingBandit) Select(arms []bandit.ArmKey) (bandit.ArmKey, error) {
	if len(arms) == 0 {
		return "", bandit.ErrNoArms
	}

	if len(arms) == 1 {
		return arms[0], nil
	}

	bestArm := arms[0]
	bestSample := 0.0

	for _, arm := range arms {
		// Load arm statistics
		stats, ok, err := ts.store.Load(arm)
		if err != nil {
			return "", err
		}

		if !ok {
			// New arm - use optimistic initialization
			stats = bandit.ArmStats{
				Alpha:      1.0,
				Beta:       1.0,
				Pulls:      0,
				LastReward: 0.5,
			}
		}

		// Sample from Beta distribution Beta(alpha, beta)
		sample := ts.betaSample(stats.Alpha, stats.Beta)

		if sample > bestSample {
			bestSample = sample
			bestArm = arm
		}
	}

	return bestArm, nil
}

// Update updates the arm statistics with observed reward
func (ts *ThompsonSamplingBandit) Update(chosen bandit.ArmKey, reward float64) error {
	// Load current statistics
	stats, ok, err := ts.store.Load(chosen)
	if err != nil {
		return err
	}

	if !ok {
		// New arm
		stats = bandit.ArmStats{
			Alpha:      1.0,
			Beta:       1.0,
			Pulls:      0,
			LastReward: 0.0,
		}
	}

	// Update Beta distribution parameters
	stats.Alpha += reward
	stats.Beta += (1.0 - reward)
	stats.Pulls++
	stats.LastReward = reward

	// Save updated statistics
	return ts.store.Save(chosen, stats)
}

// betaSample generates a sample from Beta(alpha, beta) distribution
func (ts *ThompsonSamplingBandit) betaSample(alpha, beta float64) float64 {
	if alpha <= 0 || beta <= 0 {
		return 0.5 // Default fallback
	}

	// Simple beta sampling using gamma distribution
	// Beta(α,β) = Gamma(α,1) / (Gamma(α,1) + Gamma(β,1))
	x := ts.gammaSample(alpha, 1.0)
	y := ts.gammaSample(beta, 1.0)

	if x+y == 0 {
		return 0.5
	}

	return x / (x + y)
}

// gammaSample generates a sample from Gamma(shape, scale) distribution
func (ts *ThompsonSamplingBandit) gammaSample(shape, scale float64) float64 {
	if shape < 1 {
		// Use rejection sampling for shape < 1
		return ts.gammaSample(shape+1, scale) * ts.rng.Float64()
	}

	// Marsaglia and Tsang's method for shape >= 1
	d := shape - 1.0/3.0
	c := 1.0 / (3.0 * d)
	c = 1.0 / c
	c = c * c * c

	for {
		x := ts.rng.NormFloat64()
		v := 1.0 + c*x
		if v <= 0 {
			continue
		}

		v = v * v * v
		u := ts.rng.Float64()

		if u < 1.0-0.0331*(x*x*x*x) {
			return d * v * scale
		}

		if u < 0.5*x*x+d*(1.0-v+v) {
			return d * v * scale
		}
	}
}
