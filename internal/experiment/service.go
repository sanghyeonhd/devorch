package experiment

import (
	"context"
	"encoding/json"

	"devorch/internal/id"
)

type Service struct {
	Store     *Store
	Evaluator *Evaluator
}

func NewService(st *Store, ev *Evaluator) *Service {
	return &Service{Store: st, Evaluator: ev}
}

type ActiveExperiment struct {
	Experiment Experiment
	Variants   []Variant
	Weights    []WeightedKey
}

func (s *Service) LoadActive(ctx context.Context) ([]ActiveExperiment, error) {
	exps, err := s.Store.GetRunningExperiments(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ActiveExperiment, 0, len(exps))
	for _, e := range exps {
		vars, err := s.Store.GetVariants(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		w := make([]WeightedKey, 0, len(vars))
		for _, v := range vars {
			w = append(w, WeightedKey{Key: v.Key, Weight: v.Weight})
		}
		out = append(out, ActiveExperiment{
			Experiment: e,
			Variants:   vars,
			Weights:    w,
		})
	}
	return out, nil
}

type Applied struct {
	ExperimentID string
	VariantKey   string
	PayloadJSON  string
}

func (s *Service) Assign(ctx context.Context, active ActiveExperiment, subject string) (Applied, bool, error) {
	// sticky from DB first
	if a, ok, err := s.Store.GetAssignment(ctx, active.Experiment.ID, subject); err != nil {
		return Applied{}, false, err
	} else if ok {
		for _, v := range active.Variants {
			if v.Key == a.VariantKey {
				return Applied{ExperimentID: active.Experiment.ID, VariantKey: v.Key, PayloadJSON: v.PayloadJSON}, true, nil
			}
		}
	}

	// choose variant
	key, inExp := ChooseVariant(active.Experiment.Seed, subject, active.Experiment.Traffic, active.Weights)
	if !inExp || key == "" {
		return Applied{}, false, nil
	}

	// store assignment
	if _, err := s.Store.InsertAssignment(ctx, active.Experiment.ID, subject, key); err != nil {
		return Applied{}, false, err
	}

	for _, v := range active.Variants {
		if v.Key == key {
			return Applied{ExperimentID: active.Experiment.ID, VariantKey: v.Key, PayloadJSON: v.PayloadJSON}, true, nil
		}
	}
	return Applied{}, false, nil
}

type OutcomeInput struct {
	ExperimentID string
	VariantKey   string

	BenchRunID string
	SessionID  string
	MessageID  string

	Success      bool
	LatencyMS    int
	CostMicroUSD int
	QualityScore float64
	TokensIn     int
	TokensOut    int
}

func (s *Service) RecordOutcome(ctx context.Context, in OutcomeInput) error {
	return s.Store.InsertOutcome(ctx, Outcome{
		ID:           id.NewULID(),
		ExperimentID: in.ExperimentID,
		VariantKey:   in.VariantKey,
		BenchRunID:   in.BenchRunID,
		SessionID:    in.SessionID,
		MessageID:    in.MessageID,
		Success:      in.Success,
		LatencyMS:    in.LatencyMS,
		CostMicroUSD: in.CostMicroUSD,
		QualityScore: in.QualityScore,
		TokensIn:     in.TokensIn,
		TokensOut:    in.TokensOut,
	})
}

func (s *Service) AutoDecide(ctx context.Context, experimentID string) (bool, string, error) {
	evals, err := s.Evaluator.Evaluate(ctx, experimentID)
	if err != nil {
		return false, "", err
	}

	stop, stopReason := ShouldStop(evals, DefaultGuard())
	if stop {
		_ = s.Store.StopExperiment(ctx, experimentID, false)
		return true, "stopped: " + stopReason, nil
	}

	winner, ok, reason := PickWinner(evals, DefaultDecisionRule())
	if !ok {
		return false, reason, nil
	}

	// persist winner
	reasonJSON, _ := json.Marshal(map[string]any{
		"rule":   "default",
		"reason": reason,
	})
	if err := s.Store.MarkWinner(ctx, experimentID, winner, string(reasonJSON)); err != nil {
		return false, "", err
	}
	_ = s.Store.StopExperiment(ctx, experimentID, true)
	return true, "completed winner=" + winner, nil
}
