//! The application shell.
//!
//! This is the only place in the UI that performs I/O. It owns the store, runs
//! missions on a background runtime, and drains their progress into state.
//! Views stay pure; `state.rs` stays testable; everything that could block a
//! frame lives here.

use std::sync::mpsc::{Receiver, Sender};
use std::sync::{Arc, Mutex};

use devorch_config::Config;
use devorch_mission::{MissionRequest, MissionRunner, MissionStore};
use devorch_store::Store;
use egui::{Context, RichText};

use crate::state::{apply, Command, Effect, Screen, UiState};
use crate::{theme, views};

/// A message from a background mission.
enum Update {
    /// A line of progress.
    Log(String),
    /// Assistant text, which continues the previous line when it is the same
    /// agent still speaking.
    Text { prefix: String, text: String },
    /// The mission ended; the payload is the error, if it failed.
    Finished(Option<String>),
}

/// The Devorch desktop application.
pub struct DevorchApp {
    state: UiState,
    config: Config,
    /// `None` when the database could not be opened; the UI still runs and
    /// says so rather than refusing to start.
    store: Option<Arc<Mutex<Store>>>,
    runtime: Option<tokio::runtime::Runtime>,
    updates: (Sender<Update>, Receiver<Update>),
}

impl Default for DevorchApp {
    fn default() -> Self {
        Self::new()
    }
}

impl DevorchApp {
    /// Build the app, loading config and opening the store.
    ///
    /// A failure to open either is shown in the error banner rather than
    /// aborting: an operator whose database is unreadable still needs a window
    /// that tells them so.
    pub fn new() -> Self {
        let mut state = UiState::default();

        let config = Config::default_path()
            .and_then(Config::load)
            .unwrap_or_else(|e| {
                state.error = Some(format!("config could not be loaded: {e}"));
                Config::default()
            });

        let store = match config.database_path().and_then(|path| {
            Store::open(&path).map_err(|e| devorch_config::ConfigError::Read {
                path,
                source: std::io::Error::other(e.to_string()),
            })
        }) {
            Ok(store) => Some(Arc::new(Mutex::new(store))),
            Err(e) => {
                state.error = Some(format!("database could not be opened: {e}"));
                None
            }
        };

        let runtime = tokio::runtime::Builder::new_multi_thread()
            .enable_all()
            .build()
            .ok();

        let mut app = Self {
            state,
            config,
            store,
            runtime,
            updates: std::sync::mpsc::channel(),
        };
        app.reload();
        app
    }

    /// Re-read missions, workspaces and agent capabilities.
    fn reload(&mut self) {
        let Some(store) = &self.store else { return };
        let Ok(store) = store.lock() else {
            self.state.error = Some("the store lock was poisoned".into());
            return;
        };

        match MissionStore::new(&store).list() {
            Ok(missions) => self.state.missions = missions,
            Err(e) => self.state.error = Some(format!("missions could not be read: {e}")),
        }
        match store.workspaces().list(false) {
            Ok(workspaces) => self.state.workspaces = workspaces,
            Err(e) => self.state.error = Some(format!("workspaces could not be read: {e}")),
        }

        // Inspecting four CLIs runs four processes, so it happens on the
        // runtime rather than blocking the frame.
        if let Some(runtime) = &self.runtime {
            self.state.agents = runtime.block_on(devorch_agent::inspect_all());
        }
    }

    /// Start the mission the form describes, on the background runtime.
    fn start_mission(&mut self, ctx: &Context) {
        let (Some(store), Some(runtime)) = (self.store.clone(), self.runtime.as_ref()) else {
            self.state.error = Some("no store or runtime available".into());
            self.state.progress.running = false;
            return;
        };

        let form = self.state.form.clone();
        let config = self.config.clone();
        let sender = self.updates.0.clone();
        let ctx = ctx.clone();
        let handle = runtime.handle().clone();

        // The mission runs on its own OS thread rather than as a spawned task.
        // A SQLite connection is not `Sync`, so its guard cannot be held across
        // an await point on a work-stealing runtime; owning it on one thread and
        // blocking there is both simpler and correct.
        std::thread::spawn(move || {
            let request = MissionRequest {
                agents: form.agents.clone(),
                dry_run: form.dry_run,
                ..MissionRequest::new(form.goal, form.repo).risk(form.risk)
            };

            let guard = match store.lock() {
                Ok(guard) => guard,
                Err(_) => {
                    let _ =
                        sender.send(Update::Finished(Some("the store lock was poisoned".into())));
                    ctx.request_repaint();
                    return;
                }
            };

            let runner = MissionRunner::new(&guard, config);
            let outcome = handle.block_on(runner.run(request, |progress| {
                if let Some(update) = describe(&progress) {
                    let _ = sender.send(update);
                    ctx.request_repaint();
                }
            }));

            let _ = sender.send(Update::Finished(outcome.err().map(|e| e.to_string())));
            ctx.request_repaint();
        });
    }

    /// Remove a workspace on the background runtime.
    fn remove_workspace(&mut self, name: &str) {
        let (Some(store), Some(runtime)) = (&self.store, &self.runtime) else {
            return;
        };
        let Ok(guard) = store.lock() else {
            self.state.error = Some("the store lock was poisoned".into());
            return;
        };

        let root = match self.config.worktree_root() {
            Ok(root) => root,
            Err(e) => {
                self.state.error = Some(format!("worktree root could not be resolved: {e}"));
                return;
            }
        };

        let manager = devorch_workspace::WorkspaceManager::new(&guard, &root);
        if let Err(e) = runtime.block_on(manager.remove(name, true)) {
            self.state.error = Some(format!("{name} could not be removed: {e}"));
        }
        drop(guard);
        self.reload();
    }

    /// Drain background updates into state.
    fn drain_updates(&mut self) {
        let mut finished = false;
        while let Ok(update) = self.updates.1.try_recv() {
            match update {
                Update::Log(line) => {
                    self.state.progress.end_line();
                    self.state.progress.push(line);
                }
                Update::Text { prefix, text } => self.state.progress.push_text(&prefix, &text),
                Update::Finished(error) => {
                    self.state.progress.running = false;
                    if let Some(error) = error {
                        self.state.progress.error = Some(error.clone());
                        self.state.error = Some(format!("mission failed: {error}"));
                    }
                    finished = true;
                }
            }
        }
        if finished {
            self.reload();
        }
    }

    /// Apply a command and perform whatever I/O it implies.
    fn handle(&mut self, command: Command, ctx: &Context) {
        match apply(&mut self.state, command) {
            Effect::None => {}
            Effect::Reload => self.reload(),
            Effect::StartMission => self.start_mission(ctx),
            Effect::RemoveWorkspace(name) => self.remove_workspace(&name),
        }
    }
}

impl eframe::App for DevorchApp {
    fn update(&mut self, ctx: &Context, _frame: &mut eframe::Frame) {
        self.drain_updates();

        let mut commands = Vec::new();

        egui::SidePanel::left("sidebar")
            .exact_width(180.0)
            .show(ctx, |ui| views::sidebar(ui, &self.state, &mut commands));

        egui::TopBottomPanel::top("banner").show_animated(ctx, self.state.error.is_some(), |ui| {
            if let Some(error) = self.state.error.clone() {
                ui.horizontal(|ui| {
                    ui.label(RichText::new(error).color(theme::FAIL));
                    if ui.small_button("Dismiss").clicked() {
                        commands.push(Command::ClearError);
                    }
                });
            }
        });

        egui::CentralPanel::default().show(ctx, |ui| match self.state.screen {
            Screen::MissionControl => views::mission_control(ui, &self.state, &mut commands),
            Screen::AgentBoard => views::agent_board(ui, &self.state),
            Screen::Workspaces => views::workspaces(ui, &self.state, &mut commands),
            Screen::Compare => views::compare(ui, &self.state),
            Screen::Policy => views::policy(ui),
            Screen::Settings => views::settings(ui, &self.state, &self.config),
        });

        for command in commands {
            self.handle(command, ctx);
        }

        // A running mission produces updates from another thread; keep painting
        // so they appear without the user having to move the mouse.
        if self.state.progress.running {
            ctx.request_repaint_after(std::time::Duration::from_millis(200));
        }
    }
}

/// Render mission progress as a log update, or `None` for events the log does
/// not show.
///
/// Reasoning text is deliberately excluded: it is the model's private thinking,
/// not its answer, and the UI must not present it as one.
fn describe(progress: &devorch_mission::MissionProgress<'_>) -> Option<Update> {
    use devorch_mission::MissionProgress;
    use devorch_protocol::AgentEvent;

    // Assistant text arrives a word at a time and continues the line it
    // started; everything else is a whole line of its own.
    if let MissionProgress::AgentEvent(agent, AgentEvent::TextDelta(t)) = progress {
        if t.reasoning || t.text.trim().is_empty() {
            return None;
        }
        return Some(Update::Text {
            prefix: format!("[{agent}] "),
            text: t.text.clone(),
        });
    }

    Some(Update::Log(match progress {
        MissionProgress::Planned(plan) => {
            let names: Vec<_> = plan.candidates.iter().map(|a| a.as_str()).collect();
            format!(
                "plan: [{}], {} at a time — {}",
                names.join(", "),
                plan.parallelism,
                plan.reasoning.join("; ")
            )
        }
        MissionProgress::CandidateStarted(agent, workspace) => {
            format!("{agent} started in {workspace}")
        }
        MissionProgress::AgentEvent(agent, event) => match event {
            AgentEvent::ToolStarted(t) => format!("[{agent}] {}", t.name),
            AgentEvent::Failed(f) => format!("[{agent}] failed: {}", f.message),
            _ => return None,
        },
        MissionProgress::CandidateFinished(candidate) => format!(
            "{} finished: {} file(s), {} churn, tests {}",
            candidate.agent,
            candidate.inventory.touched_count(),
            candidate.inventory.churn,
            if candidate.verification.passed() {
                "passed"
            } else {
                "failed"
            }
        ),
        MissionProgress::Compared(comparison) => match comparison.winner() {
            Some(winner) => format!("winner: {}", winner.candidate.agent),
            None => "no candidate passed the gate".to_string(),
        },
        MissionProgress::Merged(commit) => {
            format!("merged as {}", &commit[..12.min(commit.len())])
        }
        MissionProgress::PostMergeVerified(report) => format!(
            "post-merge gate: {}",
            if report.passed() { "passed" } else { "FAILED" }
        ),
    }))
}
