//! Executor selection.
//!
//! The router answers two questions: how many candidates should run, and which
//! agents should they be. The first question matters more than the second.
//!
//! Having four agents installed is not a reason to run four. Four CLIs plus a
//! browser dwarf the orchestrator's own footprint, so candidate count is the
//! largest resource decision Devorch makes — larger than anything that could be
//! micro-optimized inside the daemon. Low-risk work gets one agent.
//!
//! Selection is transparent by construction: every choice carries the reasoning
//! that produced it, so the UI can explain *why* an agent was picked without
//! ever exposing a model's private reasoning.

use devorch_config::{Config, TaskRisk};
use devorch_protocol::AgentKind;
use serde::{Deserialize, Serialize};

/// What is known about an agent when the router runs.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AgentAvailability {
    pub agent: AgentKind,
    pub installed: bool,
    /// Whether the installed binary offers structured headless output.
    pub structured: bool,
    /// Historical success rate on comparable work, if OkAON has enough data.
    /// `None` means unknown — which is never treated as zero.
    pub success_rate: Option<f64>,
    /// Attempts the success rate is based on.
    pub sample_size: u32,
}

impl AgentAvailability {
    /// An installed agent with no history yet.
    pub fn installed(agent: AgentKind, structured: bool) -> Self {
        Self {
            agent,
            installed: true,
            structured,
            success_rate: None,
            sample_size: 0,
        }
    }

    /// An agent that is not installed.
    pub fn missing(agent: AgentKind) -> Self {
        Self {
            agent,
            installed: false,
            structured: false,
            success_rate: None,
            sample_size: 0,
        }
    }
}

/// How confident the router is that one agent will succeed alone.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Confidence {
    /// Enough history, and it is good. One candidate is enough.
    High,
    /// Some history, or a mixed record. Two candidates.
    Medium,
    /// No history, or a poor record. Widen the field.
    Low,
}

/// Minimum attempts before a success rate is treated as meaningful.
///
/// Below this, one lucky run would otherwise convince the router to stop
/// comparing — which is exactly when comparison is most valuable.
const MIN_SAMPLE: u32 = 5;

/// The router's plan for one task.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct RoutingPlan {
    /// The agents to run, in preference order.
    pub candidates: Vec<AgentKind>,
    /// How many may run at once.
    pub parallelism: usize,
    pub confidence: Confidence,
    /// Why this plan, in words a user can read.
    pub reasoning: Vec<String>,
}

impl RoutingPlan {
    /// How many waves the candidates will run in.
    pub fn waves(&self) -> usize {
        if self.parallelism == 0 || self.candidates.is_empty() {
            return 0;
        }
        self.candidates.len().div_ceil(self.parallelism)
    }

    /// The candidates grouped into the waves they will run in.
    pub fn wave_groups(&self) -> Vec<Vec<AgentKind>> {
        if self.parallelism == 0 {
            return Vec::new();
        }
        self.candidates
            .chunks(self.parallelism)
            .map(<[AgentKind]>::to_vec)
            .collect()
    }
}

/// Chooses candidates for a task.
#[derive(Debug, Clone)]
pub struct Router {
    config: Config,
}

impl Router {
    /// A router bound to `config`.
    pub fn new(config: Config) -> Self {
        Self { config }
    }

    /// Plan a task of the given risk against the agents that are available.
    pub fn plan(&self, risk: TaskRisk, available: &[AgentAvailability]) -> RoutingPlan {
        let mut reasoning = Vec::new();

        let enabled = self.config.enabled_agents();
        let mut usable: Vec<_> = available
            .iter()
            .filter(|a| a.installed && enabled.contains(&a.agent))
            .collect();

        if usable.len() < available.len() {
            let excluded: Vec<_> = available
                .iter()
                .filter(|a| !a.installed)
                .map(|a| a.agent.as_str())
                .collect();
            if !excluded.is_empty() {
                reasoning.push(format!("not installed: {}", excluded.join(", ")));
            }
        }

        // Best first: proven success, then a structured protocol, then name for
        // determinism. An unknown success rate ranks below a proven one but
        // above a proven-bad one — unknown is not zero.
        usable.sort_by(|a, b| {
            rank(b)
                .partial_cmp(&rank(a))
                .unwrap_or(std::cmp::Ordering::Equal)
                .then_with(|| b.structured.cmp(&a.structured))
                .then_with(|| a.agent.as_str().cmp(b.agent.as_str()))
        });

        let confidence = confidence_of(usable.first().copied());
        reasoning.push(match confidence {
            Confidence::High => "a proven agent leads on comparable work".to_string(),
            Confidence::Medium => "some history, but not enough to stop comparing".to_string(),
            Confidence::Low => "no usable history yet, so candidates are compared".to_string(),
        });

        // Risk sets the ceiling; confidence may lower it but never raises it
        // past what the operator asked for.
        let mut wanted = self.config.candidate_count(risk);
        if risk != TaskRisk::ExplicitCompareAll {
            let by_confidence = match confidence {
                Confidence::High => 1,
                Confidence::Medium => 2,
                Confidence::Low => wanted,
            };
            if by_confidence < wanted {
                reasoning.push(format!(
                    "narrowed from {wanted} to {by_confidence} candidates on confidence"
                ));
                wanted = by_confidence;
            }
        } else {
            reasoning.push("explicit compare-all requested".to_string());
        }

        let candidates: Vec<_> = usable.iter().take(wanted).map(|a| a.agent).collect();
        let parallelism = self
            .config
            .runtime
            .max_parallel_agents
            .min(candidates.len().max(1));

        if candidates.len() > parallelism {
            reasoning.push(format!(
                "{} candidates run {parallelism} at a time, in {} waves",
                candidates.len(),
                candidates.len().div_ceil(parallelism)
            ));
        }

        RoutingPlan {
            candidates,
            parallelism,
            confidence,
            reasoning,
        }
    }
}

/// A sortable quality score. Unknown history sits between proven-good and
/// proven-bad rather than at the bottom.
fn rank(a: &AgentAvailability) -> f64 {
    match a.success_rate {
        Some(rate) if a.sample_size >= MIN_SAMPLE => rate,
        _ => 0.5,
    }
}

/// How much the leading agent's record justifies running it alone.
fn confidence_of(best: Option<&AgentAvailability>) -> Confidence {
    match best {
        Some(a) if a.sample_size >= MIN_SAMPLE => match a.success_rate {
            Some(rate) if rate >= 0.9 => Confidence::High,
            Some(rate) if rate >= 0.6 => Confidence::Medium,
            _ => Confidence::Low,
        },
        _ => Confidence::Low,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn all_installed() -> Vec<AgentAvailability> {
        AgentKind::ALL
            .iter()
            .map(|a| AgentAvailability::installed(*a, true))
            .collect()
    }

    fn proven(agent: AgentKind, rate: f64) -> AgentAvailability {
        AgentAvailability {
            success_rate: Some(rate),
            sample_size: 20,
            ..AgentAvailability::installed(agent, true)
        }
    }

    #[test]
    fn low_risk_work_runs_exactly_one_agent() {
        let router = Router::new(Config::default());
        let plan = router.plan(TaskRisk::Low, &all_installed());

        assert_eq!(plan.candidates.len(), 1);
        assert_eq!(plan.parallelism, 1);
        assert_eq!(plan.waves(), 1);
    }

    #[test]
    fn high_risk_work_compares_four_candidates_in_two_waves() {
        // Four candidates, but never four processes: this is the whole point.
        let router = Router::new(Config::default());
        let plan = router.plan(TaskRisk::High, &all_installed());

        assert_eq!(plan.candidates.len(), 4);
        assert_eq!(plan.parallelism, 2);
        assert_eq!(plan.waves(), 2);
        assert_eq!(plan.wave_groups().len(), 2);
        assert_eq!(plan.wave_groups()[0].len(), 2);
    }

    #[test]
    fn a_proven_agent_lets_the_router_stop_comparing() {
        let router = Router::new(Config::default());
        let mut available = all_installed();
        available[0] = proven(AgentKind::Codex, 0.95);

        let plan = router.plan(TaskRisk::High, &available);
        assert_eq!(plan.confidence, Confidence::High);
        assert_eq!(plan.candidates, vec![AgentKind::Codex]);
        assert!(plan.reasoning.iter().any(|r| r.contains("narrowed")));
    }

    #[test]
    fn one_lucky_run_is_not_enough_to_stop_comparing() {
        let router = Router::new(Config::default());
        let mut available = all_installed();
        available[0] = AgentAvailability {
            success_rate: Some(1.0),
            sample_size: 1,
            ..AgentAvailability::installed(AgentKind::Codex, true)
        };

        let plan = router.plan(TaskRisk::High, &available);
        assert_eq!(plan.confidence, Confidence::Low);
        assert_eq!(
            plan.candidates.len(),
            4,
            "a sample of 1 must not narrow the field"
        );
    }

    #[test]
    fn a_poor_record_widens_the_field_rather_than_narrowing_it() {
        let router = Router::new(Config::default());
        let mut available = all_installed();
        available[0] = proven(AgentKind::Codex, 0.2);

        let plan = router.plan(TaskRisk::High, &available);
        assert_eq!(plan.confidence, Confidence::Low);
        assert_eq!(plan.candidates.len(), 4);
    }

    #[test]
    fn explicit_compare_all_is_never_narrowed_by_confidence() {
        let router = Router::new(Config::default());
        let mut available = all_installed();
        available[0] = proven(AgentKind::Codex, 0.99);

        let plan = router.plan(TaskRisk::ExplicitCompareAll, &available);
        assert_eq!(plan.candidates.len(), 4, "the operator asked for all four");
    }

    #[test]
    fn uninstalled_agents_are_excluded_and_the_reason_is_recorded() {
        let router = Router::new(Config::default());
        let available = vec![
            AgentAvailability::installed(AgentKind::Codex, true),
            AgentAvailability::missing(AgentKind::Claude),
            AgentAvailability::missing(AgentKind::Grok),
            AgentAvailability::missing(AgentKind::Gemini),
        ];

        let plan = router.plan(TaskRisk::High, &available);
        assert_eq!(plan.candidates, vec![AgentKind::Codex]);
        assert!(plan
            .reasoning
            .iter()
            .any(|r| r.contains("not installed") && r.contains("claude")));
    }

    #[test]
    fn an_unknown_record_outranks_a_proven_bad_one() {
        let router = Router::new(Config::default());
        let available = vec![
            proven(AgentKind::Codex, 0.1),
            AgentAvailability::installed(AgentKind::Claude, true),
        ];

        let plan = router.plan(TaskRisk::Low, &available);
        assert_eq!(plan.candidates, vec![AgentKind::Claude]);
    }

    #[test]
    fn structured_output_breaks_a_tie_against_pty_only() {
        let router = Router::new(Config::default());
        let available = vec![
            AgentAvailability::installed(AgentKind::Codex, false),
            AgentAvailability::installed(AgentKind::Claude, true),
        ];

        let plan = router.plan(TaskRisk::Low, &available);
        assert_eq!(plan.candidates, vec![AgentKind::Claude]);
    }

    #[test]
    fn planning_with_nothing_installed_yields_no_candidates() {
        let router = Router::new(Config::default());
        let available: Vec<_> = AgentKind::ALL
            .iter()
            .map(|a| AgentAvailability::missing(*a))
            .collect();

        let plan = router.plan(TaskRisk::High, &available);
        assert!(plan.candidates.is_empty());
        assert_eq!(plan.waves(), 0);
    }

    #[test]
    fn every_plan_explains_itself() {
        let router = Router::new(Config::default());
        for risk in [
            TaskRisk::Low,
            TaskRisk::Normal,
            TaskRisk::High,
            TaskRisk::ExplicitCompareAll,
        ] {
            let plan = router.plan(risk, &all_installed());
            assert!(!plan.reasoning.is_empty(), "{risk:?} produced no reasoning");
        }
    }
}
