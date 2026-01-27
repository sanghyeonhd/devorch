package router

import (
	"math/rand"
	"time"
)

type Bandit interface {
	Choose(exploitIndex int, n int) int
}

type EpsilonGreedy struct {
	Epsilon float64
	rng     *rand.Rand
}

func NewEpsilonGreedy() *EpsilonGreedy {
	return &EpsilonGreedy{
		Epsilon: 0.10,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *EpsilonGreedy) Choose(exploitIndex int, n int) int {
	if n <= 0 {
		return -1
	}
	if n == 1 {
		return 0
	}
	if b.rng.Float64() < b.Epsilon {
		return b.rng.Intn(n)
	}
	return exploitIndex
}
