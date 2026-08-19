//! UI state and the commands that change it.
//!
//! State lives here rather than inside the render functions so it can be tested
//! without a window. Every mutation the UI performs goes through [`apply`],
//! which is a pure function of state and command — the render code never
//! reaches past it into the core.

use std::path::PathBuf;

use devorch_agent::AgentCapabilities;
use devorch_config::TaskRisk;
use devorch_mission::{MissionRecord, MissionStatus};
use devorch_protocol::AgentKind;
use devorch_store::WorkspaceRecord;

/// Which screen is showing.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Screen {
    /// Missions, their candidates and their verdicts.
    #[default]
    MissionControl,
    /// The four agents and what each installed binary can do.
    AgentBoard,
    /// Live worktrees.
    Workspaces,
    /// Side-by-side candidate evidence for one mission.
    Compare,
    /// Policy, in the form it actually evaluates.
    Policy,
    /// Configuration and health.
    Settings,
}

impl Screen {
    /// Every screen, in sidebar order.
    pub const ALL: [Screen; 6] = [
        Screen::MissionControl,
        Screen::AgentBoard,
        Screen::Workspaces,
        Screen::Compare,
        Screen::Policy,
        Screen::Settings,
    ];

    /// The sidebar label.
    pub fn label(&self) -> &'static str {
        match self {
            Screen::MissionControl => "Missions",
            Screen::AgentBoard => "Agents",
            Screen::Workspaces => "Workspaces",
            Screen::Compare => "Compare",
            Screen::Policy => "Policy",
            Screen::Settings => "Settings",
        }
    }
}

/// How a running mission is progressing, as far as the UI knows.
#[derive(Debug, Clone, Default)]
pub struct RunProgress {
    /// Lines the runner has reported, newest last.
    pub log: Vec<String>,
    /// Whether a mission is currently running.
    pub running: bool,
    /// The last error, if the run failed.
    pub error: Option<String>,
}

impl RunProgress {
    /// Cap on retained log lines.
    ///
    /// A long mission can emit thousands of events; the UI keeps a bounded tail
    /// so a run cannot grow the window's memory without limit.
    pub const MAX_LINES: usize = 500;

    /// Append a line, discarding the oldest if the cap is reached.
    pub fn push(&mut self, line: impl Into<String>) {
        self.log.push(line.into());
        if self.log.len() > Self::MAX_LINES {
            let excess = self.log.len() - Self::MAX_LINES;
            self.log.drain(..excess);
        }
    }
}

/// The form backing the "new mission" panel.
#[derive(Debug, Clone, PartialEq)]
pub struct MissionForm {
    pub goal: String,
    pub repo: String,
    pub risk: TaskRisk,
    /// Agents the operator ticked. Empty means let the router decide.
    pub agents: Vec<AgentKind>,
    pub dry_run: bool,
}

impl Default for MissionForm {
    fn default() -> Self {
        Self {
            goal: String::new(),
            repo: std::env::current_dir()
                .map(|d| d.display().to_string())
                .unwrap_or_default(),
            risk: TaskRisk::Normal,
            agents: Vec::new(),
            dry_run: false,
        }
    }
}

impl MissionForm {
    /// Why the mission cannot be started yet, if it cannot.
    ///
    /// The button is disabled rather than the error being discovered after a
    /// worktree has already been created.
    pub fn blocker(&self) -> Option<&'static str> {
        if self.goal.trim().is_empty() {
            return Some("a goal is required");
        }
        if self.repo.trim().is_empty() {
            return Some("a repository path is required");
        }
        if !PathBuf::from(self.repo.trim()).join(".git").exists() {
            return Some("that path is not a git repository");
        }
        None
    }

    /// Whether the mission can be started.
    pub fn is_valid(&self) -> bool {
        self.blocker().is_none()
    }

    /// Toggle whether an agent is explicitly selected.
    pub fn toggle_agent(&mut self, agent: AgentKind) {
        match self.agents.iter().position(|a| *a == agent) {
            Some(index) => {
                self.agents.remove(index);
            }
            None => self.agents.push(agent),
        }
    }
}

/// Everything the window draws.
#[derive(Debug, Default)]
pub struct UiState {
    pub screen: Screen,
    pub missions: Vec<MissionRecord>,
    pub workspaces: Vec<WorkspaceRecord>,
    pub agents: Vec<AgentCapabilities>,
    /// The mission shown in the Compare view.
    pub selected_mission: Option<usize>,
    pub form: MissionForm,
    pub progress: RunProgress,
    /// Whatever went wrong last, shown in a banner until dismissed.
    pub error: Option<String>,
}

impl UiState {
    /// The mission currently selected, if any.
    pub fn selected(&self) -> Option<&MissionRecord> {
        self.selected_mission.and_then(|i| self.missions.get(i))
    }

    /// How many missions succeeded.
    pub fn succeeded_count(&self) -> usize {
        self.missions
            .iter()
            .filter(|m| m.status == MissionStatus::Succeeded)
            .count()
    }

    /// Agents that are installed.
    pub fn installed_agents(&self) -> usize {
        self.agents.iter().filter(|a| a.installed).count()
    }
}

/// Something the operator asked for.
///
/// Commands are data, so a click is separable from its effect: the view emits
/// one, and the app applies it. Nothing is mutated inside a paint callback.
#[derive(Debug, Clone, PartialEq)]
pub enum Command {
    /// Show a different screen.
    Show(Screen),
    /// Re-read missions, workspaces and agents from the store.
    Refresh,
    /// Start the mission described by the form.
    RunMission,
    /// Select a mission for the Compare view.
    SelectMission(usize),
    /// Replace the new-mission form with the operator's edits.
    UpdateForm(MissionForm),
    /// Remove a workspace and its branch.
    RemoveWorkspace(String),
    /// Dismiss the error banner.
    ClearError,
}

/// Apply a command to the state.
///
/// Returns whether the caller needs to perform I/O — loading from the store or
/// starting a mission — because those cannot happen inside the render pass.
pub fn apply(state: &mut UiState, command: Command) -> Effect {
    match command {
        Command::Show(screen) => {
            state.screen = screen;
            Effect::None
        }
        Command::Refresh => Effect::Reload,
        Command::RunMission => {
            if !state.form.is_valid() {
                state.error = state.form.blocker().map(str::to_string);
                return Effect::None;
            }
            if state.progress.running {
                // Refuse rather than queue: two concurrent missions on one
                // repository would fight over worktrees.
                state.error = Some("a mission is already running".to_string());
                return Effect::None;
            }
            state.progress = RunProgress {
                running: true,
                ..Default::default()
            };
            state.error = None;
            Effect::StartMission
        }
        Command::SelectMission(index) => {
            state.selected_mission = Some(index);
            state.screen = Screen::Compare;
            Effect::None
        }
        Command::UpdateForm(form) => {
            state.form = form;
            Effect::None
        }
        Command::RemoveWorkspace(name) => Effect::RemoveWorkspace(name),
        Command::ClearError => {
            state.error = None;
            Effect::None
        }
    }
}

/// I/O the app must perform after a command.
#[derive(Debug, Clone, PartialEq)]
pub enum Effect {
    /// Nothing to do.
    None,
    /// Re-read everything from the store.
    Reload,
    /// Start the mission the form describes.
    StartMission,
    /// Remove this workspace.
    RemoveWorkspace(String),
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn switching_screens_needs_no_io() {
        let mut state = UiState::default();
        assert_eq!(
            apply(&mut state, Command::Show(Screen::AgentBoard)),
            Effect::None
        );
        assert_eq!(state.screen, Screen::AgentBoard);
    }

    #[test]
    fn a_mission_without_a_goal_cannot_start() {
        let mut state = UiState::default();
        state.form.goal = "  ".to_string();

        assert_eq!(apply(&mut state, Command::RunMission), Effect::None);
        assert!(state.error.unwrap().contains("goal"));
        assert!(!state.progress.running);
    }

    #[test]
    fn a_path_that_is_not_a_repository_is_refused_before_anything_is_created() {
        let dir = tempfile::tempdir().unwrap();
        let mut form = MissionForm {
            goal: "fix it".into(),
            repo: dir.path().display().to_string(),
            ..Default::default()
        };
        assert_eq!(form.blocker(), Some("that path is not a git repository"));

        std::fs::create_dir(dir.path().join(".git")).unwrap();
        form.repo = dir.path().display().to_string();
        assert!(form.is_valid());
    }

    #[test]
    fn a_second_mission_is_refused_rather_than_queued() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::create_dir(dir.path().join(".git")).unwrap();

        let mut state = UiState {
            form: MissionForm {
                goal: "fix it".into(),
                repo: dir.path().display().to_string(),
                ..Default::default()
            },
            ..Default::default()
        };

        assert_eq!(apply(&mut state, Command::RunMission), Effect::StartMission);
        assert!(state.progress.running);

        assert_eq!(apply(&mut state, Command::RunMission), Effect::None);
        assert!(state.error.unwrap().contains("already running"));
    }

    #[test]
    fn selecting_a_mission_switches_to_the_compare_view() {
        let mut state = UiState::default();
        apply(&mut state, Command::SelectMission(2));

        assert_eq!(state.selected_mission, Some(2));
        assert_eq!(state.screen, Screen::Compare);
    }

    #[test]
    fn the_progress_log_is_bounded() {
        let mut progress = RunProgress::default();
        for i in 0..(RunProgress::MAX_LINES + 250) {
            progress.push(format!("line {i}"));
        }

        assert_eq!(progress.log.len(), RunProgress::MAX_LINES);
        // The tail is kept, not the head.
        assert!(progress
            .log
            .last()
            .unwrap()
            .contains(&format!("line {}", RunProgress::MAX_LINES + 249)));
    }

    #[test]
    fn agent_selection_toggles() {
        let mut form = MissionForm::default();
        assert!(form.agents.is_empty(), "empty means let the router decide");

        form.toggle_agent(AgentKind::Codex);
        form.toggle_agent(AgentKind::Grok);
        assert_eq!(form.agents, vec![AgentKind::Codex, AgentKind::Grok]);

        form.toggle_agent(AgentKind::Codex);
        assert_eq!(form.agents, vec![AgentKind::Grok]);
    }

    #[test]
    fn form_edits_are_applied_without_io() {
        let mut state = UiState::default();
        let edited = MissionForm {
            goal: "fix the login regression".into(),
            ..state.form.clone()
        };

        assert_eq!(
            apply(&mut state, Command::UpdateForm(edited.clone())),
            Effect::None
        );
        assert_eq!(state.form, edited);
    }

    #[test]
    fn errors_can_be_dismissed() {
        let mut state = UiState {
            error: Some("boom".into()),
            ..Default::default()
        };
        apply(&mut state, Command::ClearError);
        assert!(state.error.is_none());
    }

    #[test]
    fn every_screen_has_a_label() {
        for screen in Screen::ALL {
            assert!(!screen.label().is_empty());
        }
    }
}
