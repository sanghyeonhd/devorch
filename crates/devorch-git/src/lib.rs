//! Git plumbing for Devorch.
//!
//! Two responsibilities: managing the worktrees that isolate candidates, and
//! producing a *complete* [`ChangeInventory`] for a worktree. The second one is
//! the reason this crate exists as its own layer — the development simulation
//! showed that `git diff --numstat` silently omits untracked files, so a
//! candidate can add an unrequested file and still look minimal.

use std::path::{Path, PathBuf};

use devorch_process::{run, ProcessError, ProcessSpec};
use devorch_protocol::ChangeInventory;

/// Failure modes of a git operation.
#[derive(Debug, thiserror::Error)]
pub enum GitError {
    #[error("git is not installed or not on PATH")]
    GitNotFound,

    #[error("`git {args}` failed with status {code:?}: {stderr}")]
    Command {
        args: String,
        code: Option<i32>,
        stderr: String,
    },

    #[error("{path} is not inside a git repository")]
    NotARepository { path: PathBuf },

    #[error(transparent)]
    Process(#[from] ProcessError),
}

/// A git repository Devorch operates on.
#[derive(Debug, Clone)]
pub struct Repository {
    root: PathBuf,
}

impl Repository {
    /// Open the repository containing `path`, resolving to its top level.
    pub async fn open(path: impl AsRef<Path>) -> Result<Self, GitError> {
        let path = path.as_ref();
        let out = git(path, ["rev-parse", "--show-toplevel"]).await;
        match out {
            Ok(text) => Ok(Self {
                root: PathBuf::from(text.trim()),
            }),
            Err(GitError::Command { .. }) => Err(GitError::NotARepository {
                path: path.to_path_buf(),
            }),
            Err(other) => Err(other),
        }
    }

    /// The repository's top-level directory.
    pub fn root(&self) -> &Path {
        &self.root
    }

    /// Resolve `rev` to a full commit SHA.
    pub async fn rev_parse(&self, rev: &str) -> Result<String, GitError> {
        Ok(git(&self.root, ["rev-parse", rev])
            .await?
            .trim()
            .to_string())
    }

    /// The current branch name, or `None` on a detached HEAD.
    pub async fn current_branch(&self) -> Result<Option<String>, GitError> {
        let name = git(&self.root, ["rev-parse", "--abbrev-ref", "HEAD"])
            .await?
            .trim()
            .to_string();
        Ok((name != "HEAD").then_some(name))
    }

    /// Create a worktree at `path` on a new branch `branch`, based on `base`.
    pub async fn worktree_add(
        &self,
        path: &Path,
        branch: &str,
        base: &str,
    ) -> Result<(), GitError> {
        git(
            &self.root,
            [
                "worktree",
                "add",
                "-b",
                branch,
                &path.to_string_lossy(),
                base,
            ],
        )
        .await?;
        Ok(())
    }

    /// Remove the worktree at `path`.
    ///
    /// `force` discards uncommitted changes; losing candidates are removed this
    /// way once their evaluation has been persisted.
    pub async fn worktree_remove(&self, path: &Path, force: bool) -> Result<(), GitError> {
        let mut args = vec!["worktree".to_string(), "remove".to_string()];
        if force {
            args.push("--force".to_string());
        }
        args.push(path.to_string_lossy().into_owned());
        git(&self.root, args).await?;
        Ok(())
    }

    /// Delete a branch, discarding unmerged commits when `force` is set.
    pub async fn branch_delete(&self, branch: &str, force: bool) -> Result<(), GitError> {
        let flag = if force { "-D" } else { "-d" };
        git(&self.root, ["branch", flag, branch]).await?;
        Ok(())
    }

    /// Paths of the worktrees currently registered on this repository.
    pub async fn worktree_list(&self) -> Result<Vec<PathBuf>, GitError> {
        let out = git(&self.root, ["worktree", "list", "--porcelain"]).await?;
        Ok(out
            .lines()
            .filter_map(|line| line.strip_prefix("worktree "))
            .map(PathBuf::from)
            .collect())
    }

    /// Drop administrative records for worktrees whose directories are gone.
    pub async fn worktree_prune(&self) -> Result<(), GitError> {
        git(&self.root, ["worktree", "prune"]).await?;
        Ok(())
    }
}

/// Collect the complete change inventory of the worktree at `dir`.
///
/// All four commands from the validated plan are run, because no single one of
/// them sees every category:
///
/// - `git status --porcelain=v1`      — modified, staged, deleted, untracked
/// - `git diff --numstat`             — unstaged churn, and binary detection
/// - `git diff --cached --numstat`    — staged churn
/// - `git ls-files --others --exclude-standard` — untracked, authoritative
pub async fn change_inventory(dir: &Path) -> Result<ChangeInventory, GitError> {
    let mut inv = ChangeInventory::default();

    let status = git(dir, ["status", "--porcelain=v1"]).await?;
    for line in status.lines() {
        if line.len() < 4 {
            continue;
        }
        let (code, rest) = line.split_at(2);
        let path = PathBuf::from(unquote_path(rest.trim()));
        let mut bytes = code.bytes();
        let index = bytes.next().unwrap_or(b' ');
        let worktree = bytes.next().unwrap_or(b' ');

        if index == b'?' && worktree == b'?' {
            inv.untracked.push(path);
            continue;
        }
        if index == b'D' || worktree == b'D' {
            inv.deleted.push(path);
            continue;
        }
        if index != b' ' && index != b'?' {
            inv.staged.push(path.clone());
        }
        if worktree != b' ' && worktree != b'?' {
            inv.tracked_modified.push(path);
        }
    }

    // `git ls-files --others` is the authoritative untracked list; status can
    // collapse an untracked directory into a single entry.
    let untracked = git(dir, ["ls-files", "--others", "--exclude-standard"]).await?;
    inv.untracked = untracked
        .lines()
        .filter(|l| !l.trim().is_empty())
        .map(|l| PathBuf::from(unquote_path(l)))
        .collect();

    for args in [
        vec!["diff", "--numstat"],
        vec!["diff", "--cached", "--numstat"],
    ] {
        let numstat = git(dir, args).await?;
        for line in numstat.lines() {
            let mut fields = line.split('\t');
            let (Some(added), Some(removed), Some(path)) =
                (fields.next(), fields.next(), fields.next())
            else {
                continue;
            };
            // git writes "-\t-" for binary files.
            if added == "-" || removed == "-" {
                let path = PathBuf::from(unquote_path(path));
                if !inv.binary.contains(&path) {
                    inv.binary.push(path);
                }
                continue;
            }
            inv.churn += added.parse::<u64>().unwrap_or(0) + removed.parse::<u64>().unwrap_or(0);
        }
    }

    inv.generated = inv
        .all_paths()
        .into_iter()
        .filter(|p| devorch_protocol::change::is_generated(p))
        .map(Path::to_path_buf)
        .collect();

    dedup(&mut inv.tracked_modified);
    dedup(&mut inv.staged);
    dedup(&mut inv.deleted);
    dedup(&mut inv.untracked);

    Ok(inv)
}

fn dedup(paths: &mut Vec<PathBuf>) {
    paths.sort();
    paths.dedup();
}

/// Strip the surrounding quotes git adds to paths containing unusual bytes.
fn unquote_path(raw: &str) -> String {
    let raw = raw.trim();
    match raw.strip_prefix('"').and_then(|r| r.strip_suffix('"')) {
        Some(inner) => inner.replace("\\\"", "\"").replace("\\\\", "\\"),
        None => raw.to_string(),
    }
}

/// Run a git command in `dir`, returning stdout on success.
async fn git<I, S>(dir: impl AsRef<Path>, args: I) -> Result<String, GitError>
where
    I: IntoIterator<Item = S>,
    S: AsRef<std::ffi::OsStr>,
{
    let spec = ProcessSpec::new("git").args(args).cwd(dir);
    let out = match run(&spec).await {
        Ok(out) => out,
        Err(ProcessError::NotFound { .. }) => return Err(GitError::GitNotFound),
        Err(e) => return Err(e.into()),
    };

    if !out.success() {
        return Err(GitError::Command {
            args: spec.display(),
            code: out.exit_code,
            stderr: out.stderr.trim().to_string(),
        });
    }
    Ok(out.stdout)
}
