package sqlite

import (
	"context"

	"devorch/internal/okaon"
)

// InsertWork inserts a work record
func (s *Store) InsertWork(ctx context.Context, w okaon.Work) error {
	const q = `
		INSERT INTO okaon_works (
			id, project_id, workspace_id, user_id,
			task_type, category, agent, prompt_hash, context_hash,
			os, arch, fingerprint, tags_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, q,
		w.ID, w.ProjectID, w.WorkspaceID, w.UserID,
		w.TaskType, w.Category, w.Agent, w.PromptHash, w.ContextHash,
		w.OS, w.Arch, w.Fingerprint, w.TagsJSON, w.CreatedAt.Unix(),
	)
	return err
}

// InsertRun inserts a run record
func (s *Store) InsertRun(ctx context.Context, r okaon.Run) error {
	const q = `
		INSERT INTO okaon_runs (
			id, work_id, provider, model,
			route_policy, was_fallback, retry_count,
			latency_ms, input_tokens, output_tokens, cost_micro_usd,
			error_code, error_msg, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	fallback := 0
	if r.WasFallback {
		fallback = 1
	}
	_, err := s.db.ExecContext(ctx, q,
		r.ID, r.WorkID, r.Provider, r.Model,
		r.RoutePolicy, fallback, r.RetryCount,
		r.LatencyMs, r.InputTokens, r.OutputTokens, r.CostMicroUSD,
		r.ErrorCode, r.ErrorMsg, r.CreatedAt.Unix(),
	)
	return err
}

// InsertQuality inserts a quality record
func (s *Store) InsertQuality(ctx context.Context, q okaon.Quality) error {
	const query = `
		INSERT INTO okaon_qualities (
			id, run_id, quality_bin, score_json, evaluated_at
		) VALUES (?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query,
		q.ID, q.RunID, q.QualityBin, q.ScoreJSON, q.EvaluatedAt.Unix(),
	)
	return err
}

// InsertReward inserts a reward record
func (s *Store) InsertReward(ctx context.Context, r okaon.Reward) error {
	const q = `
		INSERT INTO okaon_rewards (
			id, run_id, reward_01, created_at
		) VALUES (?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, q,
		r.ID, r.RunID, r.Reward01, r.CreatedAt.Unix(),
	)
	return err
}
