//! Candidate evaluation.
//!
//! This is the part of Devorch that decides which agent's work wins, and it is
//! built around one rule the development simulation proved the hard way:
//!
//! > A failing deterministic gate cannot win because a language model liked the
//! > answer.
//!
//! Scoring happens in two stages that cannot be traded off against each other.
//! First the **hard gate**: build, tests, and an inventory free of files the
//! ticket did not authorize. A candidate that fails the hard gate is out, no
//! matter what else it scored. Only survivors are ranked, and they are ranked on
//! observable facts — how many files were touched, how much churn, how many
//! advisory checks failed — never on the agent's own confidence.

use std::path::PathBuf;

use devorch_protocol::change::{check_inventory, InventoryViolation};
use devorch_protocol::{AgentKind, ChangeInventory};
use devorch_verify::VerificationReport;
use serde::{Deserialize, Serialize};

/// One agent's attempt at a task.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Candidate {
    pub agent: AgentKind,
    pub workspace: String,
    /// What the agent actually changed, from git — not what it said it changed.
    pub inventory: ChangeInventory,
    /// What the deterministic checks found.
    pub verification: VerificationReport,
    /// How long the agent took.
    pub duration_ms: u64,
    /// Whether the agent's own stream ended in a success claim. Recorded for
    /// the record, and deliberately not used in scoring.
    pub agent_claimed_success: bool,
}

/// Why a candidate was disqualified.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Disqualification {
    /// A required verification check failed.
    VerificationFailed {
        check: String,
        exit_code: Option<i32>,
    },
    /// The candidate was never verified at all.
    NotVerified,
    /// The candidate changed nothing.
    NoChanges,
    /// The change inventory contained something the ticket did not authorize.
    InventoryViolation { violation: InventoryViolation },
}

impl Disqualification {
    /// A one-line explanation for the compare view.
    pub fn summary(&self) -> String {
        match self {
            Disqualification::VerificationFailed { check, exit_code } => match exit_code {
                Some(code) => format!("`{check}` failed with status {code}"),
                None => format!("`{check}` could not be run"),
            },
            Disqualification::NotVerified => "no verification evidence".to_string(),
            Disqualification::NoChanges => "changed nothing".to_string(),
            Disqualification::InventoryViolation { violation } => match violation {
                InventoryViolation::UnexpectedBinary { path } => {
                    format!("added a binary file: {}", path.display())
                }
                InventoryViolation::UnexpectedGenerated { path } => {
                    format!("added build output: {}", path.display())
                }
                InventoryViolation::OutsideOwnedPaths { path } => {
                    format!("touched a path the ticket does not own: {}", path.display())
                }
                InventoryViolation::PossibleSecret { path } => {
                    format!(
                        "added something that looks like a credential: {}",
                        path.display()
                    )
                }
            },
        }
    }
}

/// A candidate after evaluation.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Evaluation {
    pub candidate: Candidate,
    /// Empty when the candidate passed the hard gate.
    pub disqualifications: Vec<Disqualification>,
    /// Lower is better. Only meaningful for candidates that passed the gate.
    pub score: Score,
}

impl Evaluation {
    /// Whether this candidate is eligible to win.
    pub fn passed(&self) -> bool {
        self.disqualifications.is_empty()
    }

    /// Why it was rejected, if it was.
    pub fn rejection_summary(&self) -> Option<String> {
        (!self.passed()).then(|| {
            self.disqualifications
                .iter()
                .map(Disqualification::summary)
                .collect::<Vec<_>>()
                .join("; ")
        })
    }
}

/// The observable qualities a passing candidate is ranked on.
///
/// The field order is the comparison order, and every field is something
/// Devorch measured. There is no field an agent can talk its way into.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub struct Score {
    /// Advisory checks that failed (format, lint). Fewer is better.
    pub advisory_failures: usize,
    /// Distinct files touched, untracked included. Smaller scope wins.
    pub touched_paths: usize,
    /// Added plus removed lines. A smaller diff that passes is the better fix.
    pub churn: u64,
    /// Wall-clock milliseconds. Only ever a tie-breaker.
    pub duration_ms: u64,
}

/// Evaluate one candidate against the ticket's owned paths.
///
/// An empty `owned_paths` means the ticket did not restrict paths; the binary,
/// generated and secret checks still run.
pub fn evaluate(candidate: Candidate, owned_paths: &[PathBuf]) -> Evaluation {
    let mut disqualifications = Vec::new();

    // Hard gate 1 — deterministic verification.
    if candidate.verification.checks.is_empty() {
        disqualifications.push(Disqualification::NotVerified);
    }
    for failure in candidate.verification.failures() {
        disqualifications.push(Disqualification::VerificationFailed {
            check: failure.name.clone(),
            exit_code: failure.exit_code,
        });
    }

    // Hard gate 2 — the change inventory. This is the gate `git diff` alone
    // cannot implement, because it never sees untracked files.
    if candidate.inventory.is_empty() {
        disqualifications.push(Disqualification::NoChanges);
    }
    for violation in check_inventory(&candidate.inventory, owned_paths) {
        disqualifications.push(Disqualification::InventoryViolation { violation });
    }

    let score = Score {
        advisory_failures: candidate.verification.advisory_failures(),
        touched_paths: candidate.inventory.touched_count(),
        churn: candidate.inventory.churn,
        duration_ms: candidate.duration_ms,
    };

    Evaluation {
        candidate,
        disqualifications,
        score,
    }
}

/// The outcome of comparing every candidate for a task.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Comparison {
    /// Every candidate, winner first, then the rest of the passing candidates
    /// in rank order, then the rejected ones.
    pub evaluations: Vec<Evaluation>,
}

impl Comparison {
    /// The winning candidate, if any passed.
    pub fn winner(&self) -> Option<&Evaluation> {
        self.evaluations.first().filter(|e| e.passed())
    }

    /// Candidates that passed the hard gate but were not selected.
    pub fn runners_up(&self) -> Vec<&Evaluation> {
        self.evaluations
            .iter()
            .skip(1)
            .filter(|e| e.passed())
            .collect()
    }

    /// Candidates that failed the hard gate.
    pub fn rejected(&self) -> Vec<&Evaluation> {
        self.evaluations.iter().filter(|e| !e.passed()).collect()
    }
}

/// Evaluate and rank every candidate.
///
/// Passing candidates always outrank rejected ones, so a rejected candidate can
/// never be selected however small its diff. Among equals the agent name breaks
/// the tie, which keeps the result deterministic — the same inputs must always
/// produce the same winner, or a comparison cannot be reproduced from the audit
/// record.
pub fn compare(candidates: Vec<Candidate>, owned_paths: &[PathBuf]) -> Comparison {
    let mut evaluations: Vec<_> = candidates
        .into_iter()
        .map(|c| evaluate(c, owned_paths))
        .collect();

    evaluations.sort_by(|a, b| {
        b.passed()
            .cmp(&a.passed())
            .then_with(|| a.score.cmp(&b.score))
            .then_with(|| a.candidate.agent.as_str().cmp(b.candidate.agent.as_str()))
    });

    Comparison { evaluations }
}

#[cfg(test)]
mod tests {
    use super::*;
    use devorch_verify::{CheckOutcome, VerificationReport};

    fn passing_checks() -> VerificationReport {
        VerificationReport {
            checks: vec![CheckOutcome {
                name: "tests".into(),
                command: "python -m unittest".into(),
                required: true,
                exit_code: Some(0),
                passed: true,
                output_tail: "OK".into(),
                duration_ms: 100,
            }],
        }
    }

    fn failing_checks() -> VerificationReport {
        VerificationReport {
            checks: vec![CheckOutcome {
                name: "tests".into(),
                command: "python -m unittest".into(),
                required: true,
                exit_code: Some(1),
                passed: false,
                output_tail: "FAILED (failures=3)".into(),
                duration_ms: 100,
            }],
        }
    }

    fn candidate(
        agent: AgentKind,
        inventory: ChangeInventory,
        verification: VerificationReport,
    ) -> Candidate {
        Candidate {
            agent,
            workspace: agent.as_str().to_string(),
            inventory,
            verification,
            duration_ms: 1_000,
            agent_claimed_success: true,
        }
    }

    fn edited(churn: u64) -> ChangeInventory {
        ChangeInventory {
            tracked_modified: vec![PathBuf::from("calculator.py")],
            churn,
            ..Default::default()
        }
    }

    /// The exact four-candidate scenario the development simulation validated.
    fn simulation_candidates() -> Vec<Candidate> {
        let gemini_inventory = ChangeInventory {
            tracked_modified: vec![PathBuf::from("calculator.py")],
            untracked: vec![PathBuf::from("NOTES.md")],
            churn: 4,
            ..Default::default()
        };
        vec![
            candidate(AgentKind::Codex, edited(2), passing_checks()),
            candidate(AgentKind::Claude, edited(3), passing_checks()),
            // Grok returned `a * b`: the tests fail.
            candidate(AgentKind::Grok, edited(2), failing_checks()),
            candidate(AgentKind::Gemini, gemini_inventory, passing_checks()),
        ]
    }

    #[test]
    fn the_simulation_result_is_reproduced() {
        let comparison = compare(simulation_candidates(), &[]);

        let winner = comparison.winner().expect("a candidate should win");
        assert_eq!(winner.candidate.agent, AgentKind::Codex);

        // Grok is rejected on failing tests; gemini is valid but wider in scope.
        let rejected = comparison.rejected();
        assert_eq!(rejected.len(), 1);
        assert_eq!(rejected[0].candidate.agent, AgentKind::Grok);

        let runners_up: Vec<_> = comparison
            .runners_up()
            .iter()
            .map(|e| e.candidate.agent)
            .collect();
        assert_eq!(runners_up, vec![AgentKind::Claude, AgentKind::Gemini]);
    }

    #[test]
    fn an_untracked_file_widens_scope_enough_to_lose() {
        // Same tracked edit, same churn, same passing tests. The only
        // difference is a file `git diff --numstat` would never have shown.
        let lean = candidate(AgentKind::Codex, edited(2), passing_checks());
        let wide = candidate(
            AgentKind::Claude,
            ChangeInventory {
                tracked_modified: vec![PathBuf::from("calculator.py")],
                untracked: vec![PathBuf::from("NOTES.md")],
                churn: 2,
                ..Default::default()
            },
            passing_checks(),
        );

        let comparison = compare(vec![wide, lean], &[]);
        assert_eq!(
            comparison.winner().unwrap().candidate.agent,
            AgentKind::Codex
        );
    }

    #[test]
    fn a_failing_gate_cannot_win_however_small_the_diff() {
        // The failing candidate has the smallest possible diff and the fastest
        // time. It must still lose to a slower, larger, passing one.
        let mut tiny_but_broken = candidate(AgentKind::Grok, edited(1), failing_checks());
        tiny_but_broken.duration_ms = 1;

        let mut large_but_correct = candidate(AgentKind::Codex, edited(500), passing_checks());
        large_but_correct.duration_ms = 90_000;

        let comparison = compare(vec![tiny_but_broken, large_but_correct], &[]);
        assert_eq!(
            comparison.winner().unwrap().candidate.agent,
            AgentKind::Codex
        );
    }

    #[test]
    fn an_agents_own_success_claim_does_not_influence_the_outcome() {
        // Grok says it succeeded and the tests say otherwise. The tests win.
        let mut liar = candidate(AgentKind::Grok, edited(1), failing_checks());
        liar.agent_claimed_success = true;

        let mut honest = candidate(AgentKind::Codex, edited(2), passing_checks());
        honest.agent_claimed_success = false;

        let comparison = compare(vec![liar, honest], &[]);
        assert_eq!(
            comparison.winner().unwrap().candidate.agent,
            AgentKind::Codex
        );
        assert!(comparison.rejected()[0].candidate.agent_claimed_success);
    }

    #[test]
    fn an_unverified_candidate_cannot_win() {
        let unverified = candidate(AgentKind::Codex, edited(1), VerificationReport::default());
        let evaluation = evaluate(unverified, &[]);

        assert!(!evaluation.passed());
        assert!(evaluation
            .disqualifications
            .contains(&Disqualification::NotVerified));
    }

    #[test]
    fn a_candidate_that_changed_nothing_cannot_win() {
        let idle = candidate(
            AgentKind::Codex,
            ChangeInventory::default(),
            passing_checks(),
        );
        let evaluation = evaluate(idle, &[]);

        assert!(!evaluation.passed());
        assert!(evaluation
            .disqualifications
            .contains(&Disqualification::NoChanges));
    }

    #[test]
    fn build_output_and_credentials_disqualify_even_with_passing_tests() {
        let messy = candidate(
            AgentKind::Gemini,
            ChangeInventory {
                tracked_modified: vec![PathBuf::from("calculator.py")],
                untracked: vec![
                    PathBuf::from("__pycache__/calculator.pyc"),
                    PathBuf::from(".env"),
                ],
                churn: 2,
                ..Default::default()
            },
            passing_checks(),
        );

        let evaluation = evaluate(messy, &[]);
        assert!(!evaluation.passed());

        let summary = evaluation.rejection_summary().unwrap();
        assert!(summary.contains("build output"), "{summary}");
        assert!(summary.contains("credential"), "{summary}");
    }

    #[test]
    fn touching_a_path_the_ticket_does_not_own_disqualifies() {
        let overreaching = candidate(
            AgentKind::Claude,
            ChangeInventory {
                tracked_modified: vec![
                    PathBuf::from("crates/devorch-git/src/lib.rs"),
                    PathBuf::from("Cargo.toml"),
                ],
                churn: 10,
                ..Default::default()
            },
            passing_checks(),
        );

        let evaluation = evaluate(overreaching, &[PathBuf::from("crates/devorch-git")]);
        assert!(!evaluation.passed());
        assert!(evaluation
            .rejection_summary()
            .unwrap()
            .contains("Cargo.toml"));
    }

    #[test]
    fn advisory_failures_outrank_a_smaller_diff() {
        let mut clean = candidate(AgentKind::Codex, edited(10), passing_checks());
        clean.verification.checks.push(CheckOutcome {
            name: "lint".into(),
            command: "clippy".into(),
            required: false,
            exit_code: Some(0),
            passed: true,
            output_tail: String::new(),
            duration_ms: 10,
        });

        let mut lint_failing = candidate(AgentKind::Claude, edited(2), passing_checks());
        lint_failing.verification.checks.push(CheckOutcome {
            name: "lint".into(),
            command: "clippy".into(),
            required: false,
            exit_code: Some(1),
            passed: false,
            output_tail: "warning".into(),
            duration_ms: 10,
        });

        let comparison = compare(vec![lint_failing, clean], &[]);
        assert_eq!(
            comparison.winner().unwrap().candidate.agent,
            AgentKind::Codex
        );
    }

    #[test]
    fn comparison_is_deterministic_regardless_of_input_order() {
        // The same inputs must always yield the same winner, or a decision
        // cannot be reproduced from the audit record.
        let forward = compare(simulation_candidates(), &[]);
        let mut reversed = simulation_candidates();
        reversed.reverse();
        let backward = compare(reversed, &[]);

        let order = |c: &Comparison| -> Vec<AgentKind> {
            c.evaluations.iter().map(|e| e.candidate.agent).collect()
        };
        assert_eq!(order(&forward), order(&backward));
    }

    #[test]
    fn identical_candidates_are_broken_by_a_stable_tie_break() {
        let a = candidate(AgentKind::Gemini, edited(2), passing_checks());
        let b = candidate(AgentKind::Claude, edited(2), passing_checks());

        let comparison = compare(vec![a, b], &[]);
        // Alphabetical by agent name: claude before gemini.
        assert_eq!(
            comparison.winner().unwrap().candidate.agent,
            AgentKind::Claude
        );
    }

    #[test]
    fn when_every_candidate_fails_there_is_no_winner() {
        let comparison = compare(
            vec![
                candidate(AgentKind::Codex, edited(1), failing_checks()),
                candidate(AgentKind::Grok, edited(1), failing_checks()),
            ],
            &[],
        );
        assert!(comparison.winner().is_none());
        assert_eq!(comparison.rejected().len(), 2);
    }
}
