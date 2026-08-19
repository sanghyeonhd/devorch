//! Workspace manager.
//!
//! Binds a git worktree on disk to a persisted record, so a candidate's
//! isolation is both real (a separate working directory and branch) and
//! recoverable (the record survives a daemon restart, and survives removal of
//! the directory itself).

use std::path::{Path, PathBuf};

use chrono::Utc;
use devorch_git::{change_inventory, GitError, Repository};
use devorch_protocol::{AgentKind, ChangeInventory, WorkspaceId};
use devorch_store::{Store, StoreError, WorkspaceRecord, WorkspaceState};

/// Failure modes of workspace management.
#[derive(Debug, thiserror::Error)]
pub enum WorkspaceError {
    #[error(transparent)]
    Git(#[from] GitError),

    #[error(transparent)]
    Store(#[from] StoreError),

    #[error("workspace name `{0}` is not usable: names may contain only letters, digits, '-', '_' and '.'")]
    InvalidName(String),

    #[error("a workspace named `{0}` already exists")]
    NameTaken(String),

    #[error("no workspace named `{0}`")]
    NotFound(String),

    #[error("the worktree directory {path} already exists")]
    PathOccupied { path: PathBuf },

    #[error("could not prepare {path}: {source}")]
    Io {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },
}

/// What to create a workspace from.
#[derive(Debug, Clone)]
pub struct CreateWorkspace {
    /// Unique workspace name; also the branch and directory suffix.
    pub name: String,
    /// Any path inside the repository to branch from.
    pub repo_path: PathBuf,
    /// Revision to branch from. Defaults to `HEAD`.
    pub base: Option<String>,
    /// The agent that will work here, when one is already chosen.
    pub agent: Option<AgentKind>,
}

/// Creates, lists and removes workspaces.
pub struct WorkspaceManager<'a> {
    store: &'a Store,
    worktree_root: PathBuf,
}

impl<'a> WorkspaceManager<'a> {
    /// Manage workspaces under `worktree_root`, recording them in `store`.
    pub fn new(store: &'a Store, worktree_root: impl AsRef<Path>) -> Self {
        Self {
            store,
            worktree_root: worktree_root.as_ref().to_path_buf(),
        }
    }

    /// Create a worktree and its record.
    ///
    /// The record is written only after git reports the worktree exists, so a
    /// failed `git worktree add` cannot leave a row claiming a directory that
    /// was never created.
    pub async fn create(
        &self,
        request: CreateWorkspace,
    ) -> Result<WorkspaceRecord, WorkspaceError> {
        validate_name(&request.name)?;

        if let Some(existing) = self.store.workspaces().find_by_name(&request.name)? {
            if existing.state == WorkspaceState::Active {
                return Err(WorkspaceError::NameTaken(request.name));
            }
        }

        let repo = Repository::open(&request.repo_path).await?;
        let base_rev = request.base.as_deref().unwrap_or("HEAD");
        let base_commit = repo.rev_parse(base_rev).await?;

        let path = self.worktree_root.join(&request.name);
        if path.exists() {
            return Err(WorkspaceError::PathOccupied { path });
        }
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).map_err(|source| WorkspaceError::Io {
                path: parent.to_path_buf(),
                source,
            })?;
        }

        let branch = format!("devorch/{}", request.name);
        repo.worktree_add(&path, &branch, &base_commit).await?;

        let record = WorkspaceRecord {
            id: WorkspaceId::new(),
            name: request.name,
            repo_root: repo.root().to_path_buf(),
            worktree_path: path,
            branch,
            base_commit,
            agent: request.agent,
            state: WorkspaceState::Active,
            created_at: Utc::now(),
            removed_at: None,
        };
        self.store.workspaces().insert(&record)?;
        Ok(record)
    }

    /// Active workspaces, or all of them including removed history.
    pub fn list(&self, include_removed: bool) -> Result<Vec<WorkspaceRecord>, WorkspaceError> {
        Ok(self.store.workspaces().list(include_removed)?)
    }

    /// Look up an active workspace by name.
    pub fn get(&self, name: &str) -> Result<WorkspaceRecord, WorkspaceError> {
        self.store
            .workspaces()
            .find_by_name(name)?
            .filter(|r| r.state == WorkspaceState::Active)
            .ok_or_else(|| WorkspaceError::NotFound(name.to_string()))
    }

    /// Remove a workspace's worktree and branch, keeping its record as history.
    ///
    /// `force` discards uncommitted work. A losing candidate is removed this
    /// way once its evaluation has been persisted — the evidence lives in the
    /// journal, not in the directory.
    pub async fn remove(&self, name: &str, force: bool) -> Result<WorkspaceRecord, WorkspaceError> {
        let record = self.get(name)?;
        let repo = Repository::open(&record.repo_root).await?;

        // A directory the user already deleted is not an error: prune the git
        // administrative record and carry on to the branch and the row.
        if record.worktree_path.exists() {
            repo.worktree_remove(&record.worktree_path, force).await?;
        } else {
            repo.worktree_prune().await?;
        }

        // A branch that is already gone must not block the removal either.
        if let Err(e) = repo.branch_delete(&record.branch, force).await {
            tracing::warn!(branch = %record.branch, error = %e, "branch delete skipped");
        }

        self.store.workspaces().mark_removed(&record.id)?;
        self.store.workspaces().get(&record.id).map_err(Into::into)
    }

    /// The complete change inventory of a workspace's worktree.
    pub async fn inventory(&self, name: &str) -> Result<ChangeInventory, WorkspaceError> {
        let record = self.get(name)?;
        Ok(change_inventory(&record.worktree_path).await?)
    }
}

/// Reject names that would produce awkward branches or escape the worktree root.
fn validate_name(name: &str) -> Result<(), WorkspaceError> {
    let invalid = name.is_empty()
        || name.len() > 100
        || name.starts_with('.')
        || name.starts_with('-')
        || !name
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '-' | '_' | '.'));

    if invalid {
        return Err(WorkspaceError::InvalidName(name.to_string()));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid_names_are_accepted() {
        for name in ["codex", "fix-login", "wave_r2", "v1.2"] {
            assert!(validate_name(name).is_ok(), "{name} should be valid");
        }
    }

    #[test]
    fn names_that_could_escape_the_root_are_rejected() {
        for name in ["", "..", "../evil", "a/b", ".hidden", "-flag", "has space"] {
            assert!(
                validate_name(name).is_err(),
                "{name:?} should have been rejected"
            );
        }
    }
}
