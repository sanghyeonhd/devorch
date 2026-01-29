package bandit

import (
	"errors"
	"math/rand"
	"time"
)

type Thompson struct {
	store Store
}

func NewThompson(store Store) *Thompson {
	rand.Seed(time.Now().UnixNano())
	return &Thompson{store: store}
}

func (t *Thompson) Select(arms []ArmKey) (ArmKey, error) {
	if len(arms) == 0 {
		return "", errors.New("no arms")
	}

	best := arms[0]
	bestScore := -1.0

	for _, a := range arms {
		st, ok, err := t.store.Load(a)
		if err != nil {
			return "", err
		}
		if !ok {
			st = ArmStats{Alpha: 1, Beta: 1, Pulls: 0}
		}
		s := SampleBeta(st.Alpha, st.Beta)
		if s > bestScore {
			bestScore = s
			best = a
		}
	}
	return best, nil
}

func (t *Thompson) Update(chosen ArmKey, reward float64) error {
	st, ok, err := t.store.Load(chosen)
	if err != nil {
		return err
	}
	if !ok {
		st = ArmStats{Alpha: 1, Beta: 1, Pulls: 0}
	}

	r := clamp01(reward)
	st.Alpha += r
	st.Beta += (1.0 - r)
	st.Pulls++
	st.LastReward = reward

	return t.store.Save(chosen, st)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
