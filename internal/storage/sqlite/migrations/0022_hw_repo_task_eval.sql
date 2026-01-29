-- Phase 33/34: HW Profile, Repo Fingerprint, and Task Evaluation columns
-- Migration: 0022_hw_repo_task_eval.sql

-- SQLite does not support IF NOT EXISTS for ALTER TABLE ADD COLUMN.
-- Using a new table instead to avoid conflicts with existing okaon_runs schema.

-- Table for HW profile and repo context (linked to runs)
CREATE TABLE IF NOT EXISTS okaon_run_context (
    run_id TEXT PRIMARY KEY,
    hw_profile_hash TEXT,
    hw_tier TEXT,
    repo_fingerprint TEXT,
    repo_lang TEXT,
    evaluator_name TEXT,
    eval_signals_json TEXT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

-- Index for hw-based queries
CREATE INDEX IF NOT EXISTS idx_okaon_run_context_hw ON okaon_run_context(hw_profile_hash);
CREATE INDEX IF NOT EXISTS idx_okaon_run_context_repo ON okaon_run_context(repo_fingerprint);

-- Table for storing evaluator results separately (for detailed analysis)
CREATE TABLE IF NOT EXISTS okaon_eval_results (
    eval_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    evaluator_name TEXT NOT NULL,
    quality_score REAL NOT NULL,
    signals_json TEXT,
    notes_json TEXT,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_eval_run ON okaon_eval_results(run_id);
CREATE INDEX IF NOT EXISTS idx_okaon_eval_evaluator ON okaon_eval_results(evaluator_name);
