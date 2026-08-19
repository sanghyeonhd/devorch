//! Mission records.
//!
//! A mission's record outlives the worktrees it created. That is the point: a
//! losing candidate's directory is deleted, but what it did, what it changed and
//! why it lost stays queryable after a restart.

use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use devorch_evaluate::Evaluation;
use devorch_protocol::{AgentKind, MissionId};
use devorch_store::{Store, StoreError};
use serde::{Deserialize, Serialize};

/// Where a mission is in its lifecycle.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MissionStatus {
    Running,
    Succeeded,
    Failed,
}

impl MissionStatus {
    /// The string stored in the database.
    pub fn as_str(&self) -> &'static str {
        match self {
            MissionStatus::Running => "running",
            MissionStatus::Succeeded => "succeeded",
            MissionStatus::Failed => "failed",
        }
    }

    /// Parse a stored value, defaulting to `Running` for anything unknown.
    pub fn parse(raw: &str) -> Self {
        match raw {
            "succeeded" => MissionStatus::Succeeded,
            "failed" => MissionStatus::Failed,
            _ => MissionStatus::Running,
        }
    }
}

/// One candidate's outcome, flattened for storage and display.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CandidateRecord {
    pub agent: AgentKind,
    pub workspace: String,
    pub passed: bool,
    /// Why it was rejected, when it was.
    pub rejection: Option<String>,
    pub touched_paths: usize,
    pub churn: u64,
    pub untracked: Vec<PathBuf>,
    pub duration_ms: u64,
    /// What the agent said about itself, kept for the record and never scored.
    pub agent_claimed_success: bool,
}

impl CandidateRecord {
    /// Flatten an evaluation for storage.
    pub fn from_evaluation(evaluation: &Evaluation) -> Self {
        Self {
            agent: evaluation.candidate.agent,
            workspace: evaluation.candidate.workspace.clone(),
            passed: evaluation.passed(),
            rejection: evaluation.rejection_summary(),
            touched_paths: evaluation.score.touched_paths,
            churn: evaluation.score.churn,
            untracked: evaluation.candidate.inventory.untracked.clone(),
            duration_ms: evaluation.score.duration_ms,
            agent_claimed_success: evaluation.candidate.agent_claimed_success,
        }
    }
}

/// A persisted mission.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct MissionRecord {
    pub id: MissionId,
    pub goal: String,
    pub repo_root: PathBuf,
    pub base_commit: String,
    pub status: MissionStatus,
    pub winner: Option<AgentKind>,
    pub merge_commit: Option<String>,
    pub failure: Option<String>,
    pub candidates: Vec<CandidateRecord>,
    pub created_at: DateTime<Utc>,
    pub finished_at: Option<DateTime<Utc>>,
}

impl MissionRecord {
    /// A new running mission.
    pub fn new(goal: &str, repo_root: &Path, base_commit: &str) -> Self {
        Self {
            id: MissionId::new(),
            goal: goal.to_string(),
            repo_root: repo_root.to_path_buf(),
            base_commit: base_commit.to_string(),
            status: MissionStatus::Running,
            winner: None,
            merge_commit: None,
            failure: None,
            candidates: Vec::new(),
            created_at: Utc::now(),
            finished_at: None,
        }
    }

    /// A short identifier for branch and directory names.
    pub fn short_id(&self) -> String {
        self.id
            .as_str()
            .rsplit('_')
            .next()
            .unwrap_or(self.id.as_str())
            .chars()
            .rev()
            .take(8)
            .collect::<Vec<_>>()
            .into_iter()
            .rev()
            .collect::<String>()
            .to_lowercase()
    }

    /// Mark the mission successful.
    pub fn succeed(&mut self, winner: Option<AgentKind>, merge_commit: Option<&str>) {
        self.status = MissionStatus::Succeeded;
        self.winner = winner;
        self.merge_commit = merge_commit.map(str::to_string);
        self.finished_at = Some(Utc::now());
    }

    /// Mark the mission failed, recording why.
    pub fn fail(&mut self, reason: &str) {
        self.status = MissionStatus::Failed;
        self.failure = Some(reason.to_string());
        self.finished_at = Some(Utc::now());
    }

    /// How long the mission took, if it finished.
    pub fn duration(&self) -> Option<chrono::Duration> {
        self.finished_at.map(|end| end - self.created_at)
    }
}

/// Reads and writes over the `missions` table.
pub struct MissionStore<'a> {
    store: &'a Store,
}

impl<'a> MissionStore<'a> {
    /// Missions in `store`.
    pub fn new(store: &'a Store) -> Self {
        Self { store }
    }

    /// Insert a new mission.
    pub fn insert(&self, record: &MissionRecord) -> Result<(), StoreError> {
        self.store
            .put_document("mission", record.id.as_str(), record)
    }

    /// Overwrite an existing mission.
    pub fn update(&self, record: &MissionRecord) -> Result<(), StoreError> {
        self.store
            .put_document("mission", record.id.as_str(), record)
    }

    /// Fetch one mission.
    pub fn get(&self, id: &MissionId) -> Result<Option<MissionRecord>, StoreError> {
        self.store.get_document("mission", id.as_str())
    }

    /// Every mission, newest first.
    pub fn list(&self) -> Result<Vec<MissionRecord>, StoreError> {
        self.store.list_documents("mission")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_new_mission_starts_running_with_no_winner() {
        let record = MissionRecord::new("fix the login bug", Path::new("/repo"), "abc123");
        assert_eq!(record.status, MissionStatus::Running);
        assert!(record.winner.is_none());
        assert!(record.duration().is_none());
    }

    #[test]
    fn short_ids_are_short_and_lowercase() {
        let record = MissionRecord::new("x", Path::new("/repo"), "abc");
        let short = record.short_id();
        assert_eq!(short.len(), 8);
        assert_eq!(short, short.to_lowercase());
    }

    #[test]
    fn succeeding_records_the_winner_and_the_merge() {
        let mut record = MissionRecord::new("x", Path::new("/repo"), "abc");
        record.succeed(Some(AgentKind::Codex), Some("def456"));

        assert_eq!(record.status, MissionStatus::Succeeded);
        assert_eq!(record.winner, Some(AgentKind::Codex));
        assert_eq!(record.merge_commit.as_deref(), Some("def456"));
        assert!(record.duration().is_some());
    }

    #[test]
    fn failing_records_the_reason() {
        let mut record = MissionRecord::new("x", Path::new("/repo"), "abc");
        record.fail("no candidate passed the gate");

        assert_eq!(record.status, MissionStatus::Failed);
        assert!(record.failure.unwrap().contains("no candidate"));
    }
}
