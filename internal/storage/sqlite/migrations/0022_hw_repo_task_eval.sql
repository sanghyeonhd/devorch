-- Phase 33/34: HW Profile, Repo Fingerprint, and Task Evaluation columns
-- Migration: 0022_hw_repo_task_eval.sql

BEGIN;

-- Add hw_profile_hash to okaon_runs if not exists
ALTER TABLE okaon_runs ADD COLUMN hw_profile_hash TEXT;
ALTER TABLE okaon_runs ADD COLUMN hw_tier TEXT;
ALTER TABLE okaon_runs ADD COLUMN repo_fingerprint TEXT;
ALTER TABLE okaon_runs ADD COLUMN repo_lang TEXT;
ALTER TABLE okaon_runs ADD COLUMN task_type TEXT;
ALTER TABLE okaon_runs ADD COLUMN evaluator_name TEXT;
ALTER TABLE okaon_runs ADD COLUMN eval_signals_json TEXT;

-- Index for hw-based queries
CREATE INDEX IF NOT EXISTS idx_okaon_runs_hw ON okaon_runs(hw_profile_hash);
CREATE INDEX IF NOT EXISTS idx_okaon_runs_repo ON okaon_runs(repo_fingerprint);
CREATE INDEX IF NOT EXISTS idx_okaon_runs_task_type ON okaon_runs(task_type);

-- Table for storing evaluator results separately (for detailed analysis)
CREATE TABLE IF NOT EXISTS okaon_eval_results (
    eval_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    evaluator_name TEXT NOT NULL,
    quality_score REAL NOT NULL,
    signals_json TEXT,
    notes_json TEXT,
    created_at INTEGER NOT NULL,
    FOREIGN KEY(run_id) REFERENCES okaon_runs(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_eval_run ON okaon_eval_results(run_id);
CREATE INDEX IF NOT EXISTS idx_okaon_eval_evaluator ON okaon_eval_results(evaluator_name);

COMMIT;
