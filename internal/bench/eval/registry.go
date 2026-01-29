// Package eval provides evaluator registry.
// Phase 33: Evaluator registry for pluggable evaluators
package eval

import "context"

// Registry manages a collection of evaluators.
type Registry struct {
	list []Evaluator
}

// NewRegistry creates a new evaluator registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds an evaluator to the registry.
func (r *Registry) Register(e Evaluator) {
	r.list = append(r.list, e)
}

// Evaluate runs all applicable evaluators and averages their scores.
func (r *Registry) Evaluate(ctx context.Context, in Input) (Result, error) {
	var (
		sum float64
		n   float64
		out Result
	)
	out.Signals = make(map[string]float64)

	for _, e := range r.list {
		if !e.Supports(in.TaskType) {
			continue
		}
		res, err := e.Evaluate(ctx, in)
		if err != nil {
			out.Notes = append(out.Notes, e.Name()+":error")
			continue
		}
		sum += res.QualityScore
		n++

		// Merge signals with evaluator prefix
		for k, v := range res.Signals {
			out.Signals[e.Name()+"."+k] = v
		}
		out.Notes = append(out.Notes, e.Name()+":ok")
	}

	if n == 0 {
		out.QualityScore = 0.5 // Unknown baseline
		return out, nil
	}
	out.QualityScore = sum / n
	return out, nil
}

// EvaluateWithWeights runs evaluators with custom weights.
func (r *Registry) EvaluateWithWeights(ctx context.Context, in Input, weights map[string]float64) (Result, error) {
	var (
		weightedSum float64
		totalWeight float64
		out         Result
	)
	out.Signals = make(map[string]float64)

	for _, e := range r.list {
		if !e.Supports(in.TaskType) {
			continue
		}
		res, err := e.Evaluate(ctx, in)
		if err != nil {
			out.Notes = append(out.Notes, e.Name()+":error")
			continue
		}

		w := 1.0
		if wt, ok := weights[e.Name()]; ok {
			w = wt
		}

		weightedSum += res.QualityScore * w
		totalWeight += w

		for k, v := range res.Signals {
			out.Signals[e.Name()+"."+k] = v
		}
		out.Notes = append(out.Notes, e.Name()+":ok")
	}

	if totalWeight == 0 {
		out.QualityScore = 0.5
		return out, nil
	}
	out.QualityScore = weightedSum / totalWeight
	return out, nil
}

// List returns all registered evaluators.
func (r *Registry) List() []Evaluator {
	return r.list
}

// DefaultRegistry creates a registry with default evaluators.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(BasicText{})
	return reg
}
