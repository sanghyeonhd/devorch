//! The policy engine.
//!
//! Two principles decide everything in this crate.
//!
//! **Devorch owns the final decision.** Vendors ship their own approval and
//! sandbox switches. Those are inputs, not authority: an agent that was started
//! with a permissive vendor flag still has to pass this engine.
//!
//! **Capabilities, not categories.** "allow shell" is not a policy — it is a
//! surrender. Policy is written against a specific verb, a specific resource
//! and a specific scope: `process.run("cargo test", workspace)` is allowed,
//! `fs.delete` outside the workspace is not.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

/// How much damage an action can do.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RiskLevel {
    /// Reading only.
    L0Observe,
    /// Local and trivially reversible.
    L1ReversibleLocal,
    /// Modifies local state.
    L2LocalModification,
    /// Has an effect outside this machine.
    L3ExternalSideEffect,
    /// Touches credentials or personal data.
    L4Sensitive,
    /// Destroys something.
    L5Destructive,
}

/// What the policy engine decided.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Decision {
    /// Proceed.
    Allow,
    /// Proceed, but only within the scope the request named.
    AllowScope,
    /// Proceed this one time; do not treat it as standing permission.
    AllowOnce,
    /// Stop and ask a human.
    Ask,
    /// Stop and ask a human, with the consequences spelled out.
    AskStrong,
    /// Refuse.
    Deny,
}

impl Decision {
    /// Whether the action may proceed without asking a human.
    pub fn is_permitted(&self) -> bool {
        matches!(
            self,
            Decision::Allow | Decision::AllowScope | Decision::AllowOnce
        )
    }

    /// Whether a human has to answer before anything happens.
    pub fn needs_human(&self) -> bool {
        matches!(self, Decision::Ask | Decision::AskStrong)
    }
}

/// The verb of a capability.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Capability {
    /// Read a file.
    FsRead,
    /// Create or modify a file.
    FsWrite,
    /// Delete a file.
    FsDelete,
    /// Run a subprocess.
    ProcessRun,
    /// Read repository state.
    GitRead,
    /// Commit inside a worktree.
    GitCommit,
    /// Push to a remote.
    GitPush,
    /// Reach the network.
    NetworkAccess,
    /// Send a message to a person or service.
    SendMessage,
    /// Read or change a credential.
    CredentialAccess,
    /// Drive the mouse, keyboard or screen.
    ComputerControl,
}

impl Capability {
    /// The risk this verb carries before scope is considered.
    pub fn base_risk(&self) -> RiskLevel {
        match self {
            Capability::FsRead | Capability::GitRead => RiskLevel::L0Observe,
            Capability::ProcessRun => RiskLevel::L2LocalModification,
            Capability::FsWrite | Capability::GitCommit => RiskLevel::L2LocalModification,
            Capability::FsDelete => RiskLevel::L5Destructive,
            Capability::NetworkAccess => RiskLevel::L3ExternalSideEffect,
            Capability::GitPush | Capability::SendMessage => RiskLevel::L3ExternalSideEffect,
            Capability::CredentialAccess => RiskLevel::L4Sensitive,
            Capability::ComputerControl => RiskLevel::L4Sensitive,
        }
    }
}

/// A specific action a policy is asked about.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ActionRequest {
    pub capability: Capability,
    /// What is being acted on: a path, a command line, a remote name.
    pub resource: String,
    /// The worktree this action belongs to, when it has one.
    pub workspace: Option<PathBuf>,
    /// The path the action targets, when it targets one.
    pub path: Option<PathBuf>,
}

impl ActionRequest {
    /// An action on `resource` using `capability`.
    pub fn new(capability: Capability, resource: impl Into<String>) -> Self {
        Self {
            capability,
            resource: resource.into(),
            workspace: None,
            path: None,
        }
    }

    /// Bind the action to a workspace.
    pub fn in_workspace(mut self, workspace: impl Into<PathBuf>) -> Self {
        self.workspace = Some(workspace.into());
        self
    }

    /// Name the path the action targets.
    pub fn on_path(mut self, path: impl Into<PathBuf>) -> Self {
        self.path = Some(path.into());
        self
    }

    /// Whether the target path lies inside the declared workspace.
    ///
    /// A path outside the workspace is treated as outside scope even when it
    /// merely shares a name prefix: `/repo/../elsewhere` is not `/repo`.
    pub fn within_workspace(&self) -> bool {
        let (Some(workspace), Some(path)) = (&self.workspace, &self.path) else {
            return false;
        };
        let normalized = normalize(path);
        normalized.starts_with(normalize(workspace))
    }
}

/// Resolve `.` and `..` lexically, without touching the filesystem.
///
/// The filesystem is not consulted because the path may not exist yet — an
/// agent asking to create `../../etc/hosts` must be judged before the write,
/// not after.
fn normalize(path: &Path) -> PathBuf {
    let mut out = PathBuf::new();
    for part in path.components() {
        match part {
            std::path::Component::ParentDir => {
                out.pop();
            }
            std::path::Component::CurDir => {}
            other => out.push(other.as_os_str()),
        }
    }
    out
}

/// The decision and the reasoning behind it, for the audit record.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct PolicyVerdict {
    pub decision: Decision,
    pub risk: RiskLevel,
    /// Why, in one sentence, phrased for a human reading an approval prompt.
    pub reason: String,
}

/// A policy: a decision for each risk level, plus explicit capability rules.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Policy {
    pub name: String,
    /// Decision per risk level.
    pub by_risk: BTreeMap<RiskLevel, Decision>,
    /// Capability rules that override the risk table.
    pub rules: Vec<Rule>,
}

/// An explicit rule for one capability.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Rule {
    pub capability: Capability,
    /// Only applies when the action stays inside its workspace.
    pub within_workspace_only: bool,
    pub decision: Decision,
}

impl Default for Policy {
    /// The default policy from the plan:
    ///
    /// ```text
    /// L0 allow
    /// L1 allow inside mission scope
    /// L2 allow inside owned worktree
    /// L3 ask
    /// L4 ask_strong
    /// L5 deny
    /// ```
    fn default() -> Self {
        let mut by_risk = BTreeMap::new();
        by_risk.insert(RiskLevel::L0Observe, Decision::Allow);
        by_risk.insert(RiskLevel::L1ReversibleLocal, Decision::AllowScope);
        by_risk.insert(RiskLevel::L2LocalModification, Decision::AllowScope);
        by_risk.insert(RiskLevel::L3ExternalSideEffect, Decision::Ask);
        by_risk.insert(RiskLevel::L4Sensitive, Decision::AskStrong);
        by_risk.insert(RiskLevel::L5Destructive, Decision::Deny);

        Self {
            name: "default".to_string(),
            by_risk,
            rules: vec![
                // Editing and committing are ordinary work — inside the
                // candidate's own worktree and nowhere else.
                Rule {
                    capability: Capability::FsWrite,
                    within_workspace_only: true,
                    decision: Decision::AllowScope,
                },
                Rule {
                    capability: Capability::GitCommit,
                    within_workspace_only: true,
                    decision: Decision::AllowScope,
                },
                Rule {
                    capability: Capability::ProcessRun,
                    within_workspace_only: true,
                    decision: Decision::AllowScope,
                },
                // Deleting inside your own worktree is reversible — the branch
                // still holds the file. Deleting anywhere else is not.
                Rule {
                    capability: Capability::FsDelete,
                    within_workspace_only: true,
                    decision: Decision::AllowScope,
                },
                // Native control has no runtime in this release, so it cannot
                // be permitted by any policy that ships with it.
                Rule {
                    capability: Capability::ComputerControl,
                    within_workspace_only: false,
                    decision: Decision::Deny,
                },
            ],
        }
    }
}

impl Policy {
    /// A policy that permits nothing. Useful as a dry-run baseline.
    pub fn deny_all() -> Self {
        Self {
            name: "deny-all".to_string(),
            by_risk: RISK_LEVELS
                .iter()
                .map(|risk| (*risk, Decision::Deny))
                .collect(),
            rules: Vec::new(),
        }
    }

    /// Decide whether `request` may proceed.
    pub fn evaluate(&self, request: &ActionRequest) -> PolicyVerdict {
        let within = request.within_workspace();
        let risk = self.risk_of(request, within);

        // An explicit capability rule beats the risk table, but a rule that is
        // scoped to a workspace does not apply outside one.
        for rule in &self.rules {
            if rule.capability != request.capability {
                continue;
            }
            if rule.within_workspace_only && !within {
                continue;
            }
            return PolicyVerdict {
                decision: rule.decision,
                risk,
                reason: format!(
                    "{:?} on `{}` matched an explicit rule{}",
                    request.capability,
                    request.resource,
                    if rule.within_workspace_only {
                        " for actions inside the workspace"
                    } else {
                        ""
                    }
                ),
            };
        }

        let decision = self
            .by_risk
            .get(&risk)
            .copied()
            // A risk level with no entry is refused, not permitted. Policy
            // gaps must fail closed.
            .unwrap_or(Decision::Deny);

        PolicyVerdict {
            decision,
            risk,
            reason: format!(
                "{:?} on `{}` is {:?}{}",
                request.capability,
                request.resource,
                risk,
                if within {
                    " inside the workspace"
                } else {
                    " outside any workspace"
                }
            ),
        }
    }

    /// The risk of an action, escalated when it leaves its workspace.
    fn risk_of(&self, request: &ActionRequest, within: bool) -> RiskLevel {
        let base = request.capability.base_risk();

        // Scope is what separates ordinary work from damage. The same write is
        // routine inside a candidate's worktree and destructive outside it.
        if within {
            return base;
        }
        match request.capability {
            Capability::FsWrite | Capability::GitCommit | Capability::ProcessRun
                if request.path.is_some() =>
            {
                RiskLevel::L4Sensitive
            }
            _ => base,
        }
    }
}

/// Every risk level, for iteration.
pub const RISK_LEVELS: [RiskLevel; 6] = [
    RiskLevel::L0Observe,
    RiskLevel::L1ReversibleLocal,
    RiskLevel::L2LocalModification,
    RiskLevel::L3ExternalSideEffect,
    RiskLevel::L4Sensitive,
    RiskLevel::L5Destructive,
];

#[cfg(test)]
mod tests {
    use super::*;

    fn workspace_action(capability: Capability, path: &str) -> ActionRequest {
        ActionRequest::new(capability, path)
            .in_workspace("/repo/.devorch/codex")
            .on_path(format!("/repo/.devorch/codex/{path}"))
    }

    #[test]
    fn the_four_decisions_from_the_simulation_are_reproduced() {
        // The validation report ran exactly these four actions and expected
        // exactly these four decisions.
        let policy = Policy::default();

        let read = workspace_action(Capability::FsRead, "calculator.py");
        assert_eq!(policy.evaluate(&read).decision, Decision::Allow);

        let edit = workspace_action(Capability::FsWrite, "calculator.py");
        assert_eq!(policy.evaluate(&edit).decision, Decision::AllowScope);

        let push = ActionRequest::new(Capability::GitPush, "origin main");
        assert_eq!(policy.evaluate(&push).decision, Decision::Ask);

        let delete = ActionRequest::new(Capability::FsDelete, "/Users/me/Documents")
            .in_workspace("/repo/.devorch/codex")
            .on_path("/Users/me/Documents");
        assert_eq!(policy.evaluate(&delete).decision, Decision::Deny);
    }

    #[test]
    fn the_same_write_is_routine_inside_the_worktree_and_refused_outside_it() {
        let policy = Policy::default();

        let inside = workspace_action(Capability::FsWrite, "src/lib.rs");
        assert!(policy.evaluate(&inside).decision.is_permitted());

        let outside = ActionRequest::new(Capability::FsWrite, "/etc/hosts")
            .in_workspace("/repo/.devorch/codex")
            .on_path("/etc/hosts");
        let verdict = policy.evaluate(&outside);
        assert!(!verdict.decision.is_permitted());
        assert_eq!(verdict.risk, RiskLevel::L4Sensitive);
    }

    #[test]
    fn a_path_cannot_escape_the_workspace_with_dot_dot() {
        let escape = ActionRequest::new(Capability::FsWrite, "escape")
            .in_workspace("/repo/.devorch/codex")
            .on_path("/repo/.devorch/codex/../../../etc/hosts");
        assert!(!escape.within_workspace());
        assert!(!Policy::default().evaluate(&escape).decision.is_permitted());
    }

    #[test]
    fn a_sibling_worktree_with_a_shared_prefix_is_still_outside() {
        // `/repo/wt-codex-2` must not count as inside `/repo/wt-codex`.
        let sneaky = ActionRequest::new(Capability::FsWrite, "x")
            .in_workspace("/repo/wt-codex")
            .on_path("/repo/wt-codex-2/x");
        assert!(!sneaky.within_workspace());
    }

    #[test]
    fn external_side_effects_ask_and_credentials_ask_strongly() {
        let policy = Policy::default();
        assert_eq!(
            policy
                .evaluate(&ActionRequest::new(Capability::SendMessage, "email"))
                .decision,
            Decision::Ask
        );
        assert_eq!(
            policy
                .evaluate(&ActionRequest::new(
                    Capability::CredentialAccess,
                    "keychain"
                ))
                .decision,
            Decision::AskStrong
        );
    }

    #[test]
    fn native_computer_control_is_denied_by_the_shipped_policy() {
        // This release has no computer runtime; no policy that ships with it
        // may permit one.
        let verdict = Policy::default().evaluate(&ActionRequest::new(
            Capability::ComputerControl,
            "click Run Tests",
        ));
        assert_eq!(verdict.decision, Decision::Deny);
    }

    #[test]
    fn a_policy_with_a_missing_risk_entry_fails_closed() {
        let mut policy = Policy::default();
        policy.by_risk.remove(&RiskLevel::L3ExternalSideEffect);
        policy.rules.clear();

        let verdict = policy.evaluate(&ActionRequest::new(Capability::GitPush, "origin"));
        assert_eq!(
            verdict.decision,
            Decision::Deny,
            "gaps must deny, not allow"
        );
    }

    #[test]
    fn deny_all_permits_nothing_not_even_reading() {
        let policy = Policy::deny_all();
        let read = workspace_action(Capability::FsRead, "a.txt");
        assert_eq!(policy.evaluate(&read).decision, Decision::Deny);
    }

    #[test]
    fn decisions_classify_themselves_correctly() {
        assert!(Decision::Allow.is_permitted());
        assert!(Decision::AllowOnce.is_permitted());
        assert!(!Decision::Ask.is_permitted());
        assert!(Decision::Ask.needs_human());
        assert!(Decision::AskStrong.needs_human());
        assert!(!Decision::Deny.needs_human());
    }

    #[test]
    fn a_policy_round_trips_through_json() {
        let policy = Policy::default();
        let json = serde_json::to_string(&policy).unwrap();
        let back: Policy = serde_json::from_str(&json).unwrap();
        assert_eq!(policy, back);
    }
}
