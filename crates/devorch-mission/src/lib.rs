//! Mission execution.
//!
//! A mission is: create isolated worktrees, run the chosen agents in them,
//! verify each result deterministically, compare, merge the winner, re-run the
//! gate on the merge, and clean up. This crate is where the plan's whole flow
//! becomes one function.
//!
//! Three properties are load-bearing and each has a test:
//!
//! - **The merged result is verified again.** A candidate that passes alone can
//!   still break the integration branch, so the gate runs a second time on the
//!   merge result and the merge is rolled back if it fails.
//! - **Losing worktrees are removed, their records are not.** The evidence
//!   lives in the journal.
//! - **State survives a restart.** The mission record and its candidates are
//!   persisted before any cleanup happens.

use std::path::PathBuf;
use std::time::Duration;

use devorch_agent::adapter::ExecuteRequest;
use devorch_agent::run_agent;
use devorch_config::{Config, TaskRisk};
use devorch_evaluate::{compare, Candidate, Comparison};
use devorch_git::{GitError, MergeOutcome, Repository};
use devorch_protocol::{AgentEvent, AgentKind};
use devorch_router::{AgentAvailability, Router, RoutingPlan};
use devorch_store::{Store, StoreError};
use devorch_verify::{verify, Check, VerificationReport};
use devorch_workspace::{CreateWorkspace, WorkspaceError, WorkspaceManager};

pub mod fake;
pub mod record;

pub use fake::{FakeAgent, FakeBehavior, FakeExecutor};
pub use record::{MissionRecord, MissionStatus, MissionStore};

/// Failure modes of a mission.
#[derive(Debug, thiserror::Error)]
pub enum MissionError {
    #[error("no agent is available to run this mission")]
    NoAgents,

    #[error("every candidate failed the deterministic gate")]
    NoWinner,

    #[error("the winning candidate could not be merged: {0}")]
    MergeFailed(String),

    #[error("the merged result failed the gate; the merge was rolled back")]
    MergeRegressed,

    #[error(transparent)]
    Workspace(#[from] WorkspaceError),

    #[error(transparent)]
    Git(#[from] GitError),

    #[error(transparent)]
    Store(#[from] StoreError),
}

/// What a mission is asked to do.
#[derive(Debug, Clone)]
pub struct MissionRequest {
    /// What the agents are asked to accomplish.
    pub goal: String,
    /// Repository to work in.
    pub repo: PathBuf,
    /// How much scrutiny the task warrants.
    pub risk: TaskRisk,
    /// Agents to use. Empty means let the router decide.
    pub agents: Vec<AgentKind>,
    /// Deterministic checks. Empty means detect them from the project.
    pub checks: Vec<Check>,
    /// Paths the task is authorized to touch. Empty means unrestricted.
    pub owned_paths: Vec<PathBuf>,
    /// Per-agent wall-clock ceiling.
    pub agent_timeout: Option<Duration>,
    /// Branch the winner is merged into. Defaults to a mission branch.
    pub integration_branch: Option<String>,
    /// Stop after comparing; do not merge.
    pub dry_run: bool,
}

impl MissionRequest {
    /// A mission to pursue `goal` in `repo`.
    pub fn new(goal: impl Into<String>, repo: impl Into<PathBuf>) -> Self {
        Self {
            goal: goal.into(),
            repo: repo.into(),
            risk: TaskRisk::Normal,
            agents: Vec::new(),
            checks: Vec::new(),
            owned_paths: Vec::new(),
            agent_timeout: None,
            integration_branch: None,
            dry_run: false,
        }
    }

    /// Set how much scrutiny the task warrants.
    pub fn risk(mut self, risk: TaskRisk) -> Self {
        self.risk = risk;
        self
    }

    /// Force a specific set of agents instead of letting the router choose.
    pub fn agents(mut self, agents: Vec<AgentKind>) -> Self {
        self.agents = agents;
        self
    }

    /// Compare candidates but do not merge.
    pub fn dry_run(mut self) -> Self {
        self.dry_run = true;
        self
    }
}

/// What a mission produced.
#[derive(Debug, Clone)]
pub struct MissionOutcome {
    pub record: MissionRecord,
    pub plan: RoutingPlan,
    pub comparison: Comparison,
    /// The commit the winner was merged as, when a merge happened.
    pub merge_commit: Option<String>,
    /// The gate re-run on the merged result.
    pub post_merge: Option<VerificationReport>,
}

impl MissionOutcome {
    /// The agent whose work was selected.
    pub fn winner(&self) -> Option<AgentKind> {
        self.comparison.winner().map(|e| e.candidate.agent)
    }
}

/// Runs one candidate.
///
/// A mission takes this as a parameter so tests can substitute deterministic
/// fakes for real CLIs. Every control-flow path — comparison, merge, rollback,
/// cleanup, restart — is therefore testable without a subscription, a network
/// or a wall-clock dependency.
#[async_trait::async_trait]
pub trait CandidateExecutor: Send + Sync {
    /// Run `agent` against `worktree` and return the events it produced.
    ///
    /// An executor that cannot run the agent returns an empty event list; the
    /// candidate is then judged on what is actually in the worktree, which is
    /// nothing, and loses on the inventory gate.
    async fn execute(
        &self,
        agent: AgentKind,
        worktree: &std::path::Path,
        prompt: &str,
        timeout: Option<Duration>,
    ) -> Vec<AgentEvent>;
}

/// Drives the real vendor CLIs.
pub struct VendorExecutor;

#[async_trait::async_trait]
impl CandidateExecutor for VendorExecutor {
    async fn execute(
        &self,
        agent: AgentKind,
        worktree: &std::path::Path,
        prompt: &str,
        timeout: Option<Duration>,
    ) -> Vec<AgentEvent> {
        let Some(mut adapter) = devorch_agents::adapter_for(agent) else {
            tracing::warn!(agent = %agent, "no adapter available");
            return Vec::new();
        };

        let mut request = ExecuteRequest::new(prompt, worktree);
        if let Some(timeout) = timeout {
            request = request.timeout(timeout);
        }

        match run_agent(adapter.as_mut(), &request, |_| {}).await {
            Ok(run) => run.events,
            Err(e) => {
                tracing::warn!(agent = %agent, error = %e, "agent run failed");
                Vec::new()
            }
        }
    }
}

/// Runs missions.
pub struct MissionRunner<'a> {
    store: &'a Store,
    config: Config,
    executor: Box<dyn CandidateExecutor>,
}

impl<'a> MissionRunner<'a> {
    /// A runner driving the real vendor CLIs.
    pub fn new(store: &'a Store, config: Config) -> Self {
        Self {
            store,
            config,
            executor: Box::new(VendorExecutor),
        }
    }

    /// A runner with a substituted executor, for tests and dry runs.
    pub fn with_executor(
        store: &'a Store,
        config: Config,
        executor: Box<dyn CandidateExecutor>,
    ) -> Self {
        Self {
            store,
            config,
            executor,
        }
    }

    /// Run a mission end to end.
    pub async fn run<F>(
        &self,
        request: MissionRequest,
        mut on_event: F,
    ) -> Result<MissionOutcome, MissionError>
    where
        F: FnMut(MissionProgress<'_>),
    {
        let repo = Repository::open(&request.repo).await?;
        let base_commit = repo.rev_parse("HEAD").await?;

        let mut record = MissionRecord::new(&request.goal, repo.root(), &base_commit);
        MissionStore::new(self.store).insert(&record)?;

        let plan = self.plan(&request).await;
        on_event(MissionProgress::Planned(&plan));

        if plan.candidates.is_empty() {
            record.fail("no agent available");
            MissionStore::new(self.store).update(&record)?;
            return Err(MissionError::NoAgents);
        }

        let checks = if request.checks.is_empty() {
            devorch_verify::detect_checks(repo.root())
        } else {
            request.checks.clone()
        };

        let root = self
            .config
            .worktree_root()
            .unwrap_or_else(|_| repo.root().join(".devorch/worktrees"));
        let manager = WorkspaceManager::new(self.store, &root);

        // Run candidates in waves so peak process count stays capped.
        let mut candidates = Vec::new();
        let mut created = Vec::new();
        for wave in plan.wave_groups() {
            for agent in wave {
                let name = format!("{}-{}", record.short_id(), agent.as_str());
                let workspace = manager
                    .create(CreateWorkspace {
                        name: name.clone(),
                        repo_path: repo.root().to_path_buf(),
                        base: Some(base_commit.clone()),
                        agent: Some(agent),
                    })
                    .await?;
                created.push(name.clone());
                on_event(MissionProgress::CandidateStarted(agent, &name));

                let candidate = self
                    .run_candidate(
                        agent,
                        &workspace.worktree_path,
                        &request,
                        &checks,
                        &mut on_event,
                    )
                    .await;
                on_event(MissionProgress::CandidateFinished(&candidate));
                candidates.push(candidate);
            }
        }

        let comparison = compare(candidates, &request.owned_paths);
        on_event(MissionProgress::Compared(&comparison));

        // Persist the comparison before any cleanup: the evidence must outlive
        // the worktrees it came from.
        record.candidates = comparison
            .evaluations
            .iter()
            .map(record::CandidateRecord::from_evaluation)
            .collect();
        MissionStore::new(self.store).update(&record)?;

        let Some(winner) = comparison.winner() else {
            record.fail("no candidate passed the gate");
            MissionStore::new(self.store).update(&record)?;
            self.cleanup(&manager, &created, None).await;
            return Err(MissionError::NoWinner);
        };
        let winner_agent = winner.candidate.agent;
        let winner_workspace = winner.candidate.workspace.clone();

        if request.dry_run {
            record.succeed(Some(winner_agent), None);
            MissionStore::new(self.store).update(&record)?;
            return Ok(MissionOutcome {
                record,
                plan,
                comparison,
                merge_commit: None,
                post_merge: None,
            });
        }

        // Merge the winner onto an integration branch, never onto whatever the
        // operator has checked out.
        let integration = request
            .integration_branch
            .clone()
            .unwrap_or_else(|| format!("devorch/mission/{}", record.short_id()));
        let winner_branch = format!("devorch/{winner_workspace}");

        repo.commit_all(
            &root.join(&winner_workspace),
            &format!("devorch: {} ({})", request.goal, winner_agent),
        )
        .await?;
        repo.branch_create(&integration, &base_commit).await?;

        let outcome = repo
            .merge_into(
                &integration,
                &winner_branch,
                &format!("devorch: merge {winner_agent} candidate"),
            )
            .await?;

        let merge_commit = match &outcome {
            MergeOutcome::Merged { commit } => commit.clone(),
            MergeOutcome::Conflict { conflicts, .. } => {
                let detail = conflicts
                    .iter()
                    .map(|p| p.display().to_string())
                    .collect::<Vec<_>>()
                    .join(", ");
                record.fail(&format!("merge conflict: {detail}"));
                MissionStore::new(self.store).update(&record)?;
                self.cleanup(&manager, &created, Some(&winner_workspace))
                    .await;
                return Err(MissionError::MergeFailed(detail));
            }
        };
        on_event(MissionProgress::Merged(&merge_commit));

        // Re-run the gate on the merged result. Passing alone is not the same
        // as passing together.
        let post_merge = self.verify_merge(&repo, &integration, &checks).await?;
        on_event(MissionProgress::PostMergeVerified(&post_merge));

        if !post_merge.passed() {
            // Roll the integration branch back to base; the mission failed but
            // the repository is untouched.
            repo.branch_create(&integration, &base_commit).await?;
            record.fail("the merged result failed the gate");
            MissionStore::new(self.store).update(&record)?;
            self.cleanup(&manager, &created, None).await;
            return Err(MissionError::MergeRegressed);
        }

        record.succeed(Some(winner_agent), Some(&merge_commit));
        MissionStore::new(self.store).update(&record)?;

        // Only now remove the losers. The winner's worktree is kept so the
        // operator can inspect what landed.
        self.cleanup(&manager, &created, Some(&winner_workspace))
            .await;

        Ok(MissionOutcome {
            record,
            plan,
            comparison,
            merge_commit: Some(merge_commit),
            post_merge: Some(post_merge),
        })
    }

    /// Decide which agents run.
    async fn plan(&self, request: &MissionRequest) -> RoutingPlan {
        if !request.agents.is_empty() {
            let parallelism = self
                .config
                .runtime
                .max_parallel_agents
                .min(request.agents.len().max(1));
            return RoutingPlan {
                candidates: request.agents.clone(),
                parallelism,
                confidence: devorch_router::Confidence::Low,
                reasoning: vec!["agents were named explicitly".to_string()],
            };
        }

        let available: Vec<_> = devorch_agent::inspect_all()
            .await
            .into_iter()
            .map(|caps| {
                if caps.installed {
                    AgentAvailability::installed(caps.id, caps.has_structured_output())
                } else {
                    AgentAvailability::missing(caps.id)
                }
            })
            .collect();

        Router::new(self.config.clone()).plan(request.risk, &available)
    }

    /// Run one agent in its worktree and verify what it produced.
    async fn run_candidate<F>(
        &self,
        agent: AgentKind,
        worktree: &std::path::Path,
        request: &MissionRequest,
        checks: &[Check],
        on_event: &mut F,
    ) -> Candidate
    where
        F: FnMut(MissionProgress<'_>),
    {
        let workspace = worktree
            .file_name()
            .map(|n| n.to_string_lossy().into_owned())
            .unwrap_or_default();

        let started = std::time::Instant::now();

        let events = self
            .executor
            .execute(agent, worktree, &prompt_for(request), request.agent_timeout)
            .await;
        for event in &events {
            on_event(MissionProgress::AgentEvent(agent, event));
        }

        // What the agent said about itself. Recorded, never scored.
        let claimed_success = matches!(events.last(), Some(AgentEvent::Completed(_)));

        // The inventory comes from git, not from what the agent said it did.
        let inventory = devorch_git::change_inventory(worktree)
            .await
            .unwrap_or_default();

        let verification = verify(worktree, checks, request.agent_timeout)
            .await
            .unwrap_or_default();

        Candidate {
            agent,
            workspace,
            inventory,
            verification,
            duration_ms: started.elapsed().as_millis() as u64,
            agent_claimed_success: claimed_success,
        }
    }

    /// Run the gate against the merged integration branch.
    async fn verify_merge(
        &self,
        repo: &Repository,
        branch: &str,
        checks: &[Check],
    ) -> Result<VerificationReport, MissionError> {
        let scratch = repo
            .root()
            .join(".git/devorch-verify")
            .join(format!("{}", std::process::id()));
        if scratch.exists() {
            let _ = repo.worktree_remove(&scratch, true).await;
        }
        if let Some(parent) = scratch.parent() {
            let _ = std::fs::create_dir_all(parent);
        }

        repo.worktree_add_detached(&scratch, branch).await?;
        let report = verify(&scratch, checks, None).await.unwrap_or_default();
        let _ = repo.worktree_remove(&scratch, true).await;
        Ok(report)
    }

    /// Remove every workspace except `keep`.
    ///
    /// Cleanup failures are logged, never propagated: a mission that produced a
    /// verified merge has succeeded even if one directory could not be removed.
    async fn cleanup(&self, manager: &WorkspaceManager<'_>, names: &[String], keep: Option<&str>) {
        for name in names {
            if Some(name.as_str()) == keep {
                continue;
            }
            if let Err(e) = manager.remove(name, true).await {
                tracing::warn!(workspace = %name, error = %e, "could not remove workspace");
            }
        }
    }
}

/// The instruction given to an agent.
///
/// The scope rules are stated to the agent as well as enforced by policy —
/// an agent told what the boundary is fails less often than one that discovers
/// it by being denied.
fn prompt_for(request: &MissionRequest) -> String {
    let mut prompt = request.goal.clone();
    if !request.owned_paths.is_empty() {
        let paths = request
            .owned_paths
            .iter()
            .map(|p| p.display().to_string())
            .collect::<Vec<_>>()
            .join(", ");
        prompt.push_str(&format!("\n\nOnly modify these paths: {paths}."));
    }
    prompt.push_str(
        "\n\nWork only inside this directory. Do not create files the task did not ask for, \
         and do not commit build output, caches or credentials.",
    );
    prompt
}

/// Progress reported while a mission runs.
#[derive(Debug)]
pub enum MissionProgress<'a> {
    Planned(&'a RoutingPlan),
    CandidateStarted(AgentKind, &'a str),
    AgentEvent(AgentKind, &'a AgentEvent),
    CandidateFinished(&'a Candidate),
    Compared(&'a Comparison),
    Merged(&'a str),
    PostMergeVerified(&'a VerificationReport),
}
