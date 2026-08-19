//! The screens.
//!
//! Every function here takes `&UiState` and pushes [`Command`]s. None of them
//! mutate state or touch the store, which is what keeps the interesting logic
//! in `state.rs` where it can be tested without a window.

use devorch_config::TaskRisk;
use devorch_mission::{MissionRecord, MissionStatus};
use devorch_protocol::AgentKind;
use egui::{Color32, RichText, Ui};

use crate::graph3d::{self, Camera, Constellation};
use crate::state::{Command, CompareMode, Screen, UiState};
use crate::theme;

/// The left navigation rail.
pub fn sidebar(ui: &mut Ui, state: &UiState, commands: &mut Vec<Command>) {
    ui.add_space(8.0);
    ui.heading(RichText::new("Devorch").strong());
    ui.label(
        RichText::new(format!("v{}", env!("CARGO_PKG_VERSION")))
            .small()
            .color(theme::MUTED),
    );
    ui.add_space(12.0);

    for screen in Screen::ALL {
        let selected = state.screen == screen;
        if ui.selectable_label(selected, screen.label()).clicked() {
            commands.push(Command::Show(screen));
        }
    }

    ui.add_space(16.0);
    ui.separator();
    ui.add_space(8.0);

    ui.label(
        RichText::new(format!("{} agents installed", state.installed_agents()))
            .small()
            .color(theme::MUTED),
    );
    ui.label(
        RichText::new(format!(
            "{}/{} missions succeeded",
            state.succeeded_count(),
            state.missions.len()
        ))
        .small()
        .color(theme::MUTED),
    );

    ui.add_space(8.0);
    if ui.button("Refresh").clicked() {
        commands.push(Command::Refresh);
    }
}

/// Missions, plus the form that starts a new one.
pub fn mission_control(ui: &mut Ui, state: &UiState, commands: &mut Vec<Command>) {
    ui.heading("Mission Control");
    ui.add_space(6.0);

    ui.group(|ui| {
        ui.label(RichText::new("New mission").strong());
        ui.add_space(4.0);

        // The form is drawn from a copy: views never mutate state directly, so
        // edits are collected and applied by the app.
        let mut form = state.form.clone();
        ui.horizontal(|ui| {
            ui.label("Goal");
            ui.add(
                egui::TextEdit::singleline(&mut form.goal)
                    .desired_width(f32::INFINITY)
                    .hint_text("Fix the seeded authentication regression"),
            );
        });
        ui.horizontal(|ui| {
            ui.label("Repo");
            ui.add(egui::TextEdit::singleline(&mut form.repo).desired_width(420.0));
        });

        ui.horizontal(|ui| {
            ui.label("Risk");
            for (risk, label, hint) in [
                (TaskRisk::Low, "Low", "1 candidate"),
                (TaskRisk::Normal, "Normal", "2 candidates"),
                (TaskRisk::High, "High", "up to 4, 2 at a time"),
                (TaskRisk::ExplicitCompareAll, "Compare all", "all 4"),
            ] {
                ui.selectable_value(&mut form.risk, risk, label)
                    .on_hover_text(hint);
            }
        });

        ui.horizontal(|ui| {
            ui.label("Agents");
            for agent in AgentKind::ALL {
                let installed = state.agents.iter().any(|c| c.id == agent && c.installed);
                let mut selected = form.agents.contains(&agent);

                ui.add_enabled_ui(installed, |ui| {
                    if ui
                        .checkbox(&mut selected, agent.as_str())
                        .on_hover_text(if installed {
                            "tick to force this agent; leave all unticked to let the router decide"
                        } else {
                            "not installed"
                        })
                        .changed()
                    {
                        form.toggle_agent(agent);
                    }
                });
            }
            ui.checkbox(&mut form.dry_run, "Dry run")
                .on_hover_text("compare candidates but do not merge");
        });

        ui.add_space(4.0);
        ui.horizontal(|ui| {
            let blocker = form.blocker();
            ui.add_enabled_ui(blocker.is_none() && !state.progress.running, |ui| {
                if ui.button(RichText::new("Run mission").strong()).clicked() {
                    commands.push(Command::RunMission);
                }
            });

            if let Some(reason) = blocker {
                ui.label(RichText::new(reason).color(theme::WAITING).small());
            } else if state.progress.running {
                ui.label(RichText::new("running…").color(theme::ACTIVE).small());
            }
        });

        if form != state.form {
            commands.push(Command::UpdateForm(form));
        }
    });

    if state.progress.running || !state.progress.log.is_empty() {
        ui.add_space(8.0);
        ui.group(|ui| {
            ui.label(RichText::new("Progress").strong());
            egui::ScrollArea::vertical()
                .max_height(180.0)
                .stick_to_bottom(true)
                .show(ui, |ui| {
                    for line in &state.progress.log {
                        ui.label(RichText::new(line).monospace().small());
                    }
                });
        });
    }

    ui.add_space(10.0);
    ui.label(RichText::new("Missions").strong());
    ui.add_space(4.0);

    if state.missions.is_empty() {
        ui.label(RichText::new("No missions yet.").color(theme::MUTED));
        return;
    }

    egui::ScrollArea::vertical().show(ui, |ui| {
        for (index, mission) in state.missions.iter().enumerate() {
            mission_row(ui, index, mission, commands);
        }
    });
}

/// One mission in the list.
fn mission_row(ui: &mut Ui, index: usize, mission: &MissionRecord, commands: &mut Vec<Command>) {
    ui.group(|ui| {
        ui.horizontal(|ui| {
            let (color, label) = status_badge(mission.status);
            ui.label(RichText::new(label).color(color).strong());
            ui.label(RichText::new(&mission.goal).strong());
        });

        ui.horizontal(|ui| {
            ui.label(
                RichText::new(match mission.winner {
                    Some(agent) => format!("winner: {agent}"),
                    None => "winner: none".to_string(),
                })
                .small()
                .color(if mission.winner.is_some() {
                    theme::WINNER
                } else {
                    theme::MUTED
                }),
            );
            ui.label(
                RichText::new(format!("{} candidates", mission.candidates.len()))
                    .small()
                    .color(theme::MUTED),
            );
            if let Some(commit) = &mission.merge_commit {
                ui.label(
                    RichText::new(format!("merged {}", &commit[..12.min(commit.len())]))
                        .small()
                        .monospace()
                        .color(theme::MUTED),
                );
            }
            if ui.small_button("Compare").clicked() {
                commands.push(Command::SelectMission(index));
            }
        });

        if let Some(failure) = &mission.failure {
            ui.label(RichText::new(failure).small().color(theme::FAIL));
        }
    });
}

/// The color and word for a mission status.
fn status_badge(status: MissionStatus) -> (Color32, &'static str) {
    match status {
        MissionStatus::Running => (theme::ACTIVE, "RUNNING"),
        MissionStatus::Succeeded => (theme::PASS, "SUCCEEDED"),
        MissionStatus::Failed => (theme::FAIL, "FAILED"),
    }
}

/// The four agents and what each installed binary can actually do.
pub fn agent_board(ui: &mut Ui, state: &UiState) {
    ui.heading("Agent Board");
    ui.label(
        RichText::new("Capabilities are read from the installed binaries at runtime, not assumed.")
            .small()
            .color(theme::MUTED),
    );
    ui.add_space(8.0);

    if state.agents.is_empty() {
        ui.label(RichText::new("Not inspected yet — press Refresh.").color(theme::MUTED));
        return;
    }

    for caps in &state.agents {
        ui.group(|ui| {
            ui.horizontal(|ui| {
                ui.label(RichText::new(caps.id.as_str()).strong());
                if caps.installed {
                    ui.label(
                        RichText::new(caps.version.as_deref().unwrap_or("installed"))
                            .small()
                            .color(theme::PASS),
                    );
                } else {
                    ui.label(RichText::new("not installed").small().color(theme::MUTED));
                }
            });

            if !caps.installed {
                return;
            }

            ui.horizontal_wrapped(|ui| {
                ui.label(RichText::new("mode").small().color(theme::MUTED));
                ui.label(
                    RichText::new(
                        caps.preferred_mode()
                            .map(|m| format!("{m:?}"))
                            .unwrap_or_else(|| "none".into()),
                    )
                    .small(),
                );

                ui.label(RichText::new("formats").small().color(theme::MUTED));
                ui.label(
                    RichText::new(if caps.structured_formats.is_empty() {
                        "-".to_string()
                    } else {
                        caps.structured_formats.join(", ")
                    })
                    .small(),
                );

                ui.label(RichText::new("protocols").small().color(theme::MUTED));
                ui.label(
                    RichText::new(if caps.protocols.is_empty() {
                        "-".to_string()
                    } else {
                        caps.protocols.join(", ")
                    })
                    .small(),
                );
            });

            ui.horizontal(|ui| {
                flag(ui, "resume", caps.resume);
                flag(ui, "tool permissions", caps.tool_permissions);
                flag(ui, "structured output", caps.has_structured_output());
            });
        });
    }
}

/// A yes/no capability, labelled as well as colored.
fn flag(ui: &mut Ui, label: &str, on: bool) {
    ui.label(
        RichText::new(format!("{label}: {}", if on { "yes" } else { "no" }))
            .small()
            .color(if on { theme::PASS } else { theme::MUTED }),
    );
}

/// Live worktrees.
pub fn workspaces(ui: &mut Ui, state: &UiState, commands: &mut Vec<Command>) {
    ui.heading("Workspaces");
    ui.label(
        RichText::new("Each candidate works in its own git worktree and branch.")
            .small()
            .color(theme::MUTED),
    );
    ui.add_space(8.0);

    if state.workspaces.is_empty() {
        ui.label(RichText::new("No active workspaces.").color(theme::MUTED));
        return;
    }

    egui::ScrollArea::vertical().show(ui, |ui| {
        for workspace in &state.workspaces {
            ui.group(|ui| {
                ui.horizontal(|ui| {
                    ui.label(RichText::new(&workspace.name).strong());
                    ui.label(
                        RichText::new(workspace.agent.map(|a| a.as_str()).unwrap_or("-"))
                            .small()
                            .color(theme::MUTED),
                    );
                    if ui
                        .small_button("Remove")
                        .on_hover_text("deletes the worktree and branch; the record is kept")
                        .clicked()
                    {
                        commands.push(Command::RemoveWorkspace(workspace.name.clone()));
                    }
                });
                ui.label(
                    RichText::new(workspace.worktree_path.display().to_string())
                        .small()
                        .monospace()
                        .color(theme::MUTED),
                );
                ui.label(
                    RichText::new(format!(
                        "{} @ {}",
                        workspace.branch,
                        &workspace.base_commit[..12.min(workspace.base_commit.len())]
                    ))
                    .small()
                    .monospace()
                    .color(theme::MUTED),
                );
            });
        }
    });
}

/// Side-by-side candidate evidence for one mission.
///
/// Deterministic evidence is shown first and largest, because that is what
/// decided the outcome.
pub fn compare(ui: &mut Ui, state: &UiState, camera: &mut Camera, commands: &mut Vec<Command>) {
    ui.horizontal(|ui| {
        ui.heading("Compare");
        ui.add_space(12.0);
        for (mode, label, hint) in [
            (CompareMode::Evidence, "Evidence", "what was measured"),
            (
                CompareMode::Constellation,
                "Constellation",
                "how the mission was shaped",
            ),
        ] {
            if ui
                .selectable_label(state.compare_mode == mode, label)
                .on_hover_text(hint)
                .clicked()
            {
                commands.push(Command::SetCompareMode(mode));
            }
        }
    });

    let Some(mission) = state.selected() else {
        ui.label(RichText::new("Select a mission from Mission Control.").color(theme::MUTED));
        return;
    };

    ui.label(RichText::new(&mission.goal).strong());
    ui.label(
        RichText::new(format!(
            "base {} · {} candidates",
            &mission.base_commit[..12.min(mission.base_commit.len())],
            mission.candidates.len()
        ))
        .small()
        .monospace()
        .color(theme::MUTED),
    );
    ui.add_space(8.0);

    if state.compare_mode == CompareMode::Constellation {
        graph3d::show(ui, &Constellation::from_mission(mission), camera);
        return;
    }

    egui::ScrollArea::vertical().show(ui, |ui| {
        egui::Grid::new("compare-grid")
            .num_columns(6)
            .striped(true)
            .spacing([16.0, 6.0])
            .show(ui, |ui| {
                for header in ["agent", "verdict", "files", "churn", "time", "notes"] {
                    ui.label(RichText::new(header).small().strong().color(theme::MUTED));
                }
                ui.end_row();

                for candidate in &mission.candidates {
                    let is_winner = mission.winner == Some(candidate.agent);
                    ui.label(RichText::new(candidate.agent.as_str()).strong().color(
                        if is_winner {
                            theme::WINNER
                        } else {
                            Color32::GRAY
                        },
                    ));
                    ui.label(
                        RichText::new(theme::verdict_label(candidate.passed))
                            .color(theme::verdict_color(candidate.passed)),
                    );
                    ui.label(candidate.touched_paths.to_string());
                    ui.label(candidate.churn.to_string());
                    ui.label(format!("{}ms", candidate.duration_ms));

                    let mut notes = Vec::new();
                    if !candidate.untracked.is_empty() {
                        notes.push(format!("{} untracked", candidate.untracked.len()));
                    }
                    if let Some(rejection) = &candidate.rejection {
                        notes.push(rejection.clone());
                    }
                    // Shown as a fact, never as a reason: the agent's own claim
                    // has no weight in the verdict.
                    if candidate.agent_claimed_success && !candidate.passed {
                        notes.push("claimed success".to_string());
                    }
                    ui.label(RichText::new(notes.join("; ")).small());
                    ui.end_row();
                }
            });
    });
}

/// The shipped policy, in the form it actually evaluates.
pub fn policy(ui: &mut Ui) {
    use devorch_policy::{Capability, Policy, RiskLevel, RISK_LEVELS};

    ui.heading("Policy");
    ui.label(
        RichText::new(
            "Devorch owns the final permission decision. Vendor approval flags are inputs, \
             not authority.",
        )
        .small()
        .color(theme::MUTED),
    );
    ui.add_space(8.0);

    let policy = Policy::default();

    ui.label(RichText::new("By risk level").strong());
    egui::Grid::new("policy-risk")
        .num_columns(2)
        .striped(true)
        .spacing([24.0, 4.0])
        .show(ui, |ui| {
            for risk in RISK_LEVELS {
                ui.label(RichText::new(format!("{risk:?}")).small().monospace());
                let decision = policy
                    .by_risk
                    .get(&risk)
                    .map(|d| format!("{d:?}"))
                    .unwrap_or_else(|| "Deny (no entry — fails closed)".into());
                ui.label(RichText::new(decision).small().color(risk_color(risk)));
                ui.end_row();
            }
        });

    ui.add_space(10.0);
    ui.label(RichText::new("Capability rules").strong());
    egui::Grid::new("policy-rules")
        .num_columns(3)
        .striped(true)
        .spacing([24.0, 4.0])
        .show(ui, |ui| {
            for rule in &policy.rules {
                ui.label(
                    RichText::new(format!("{:?}", rule.capability))
                        .small()
                        .monospace(),
                );
                ui.label(
                    RichText::new(if rule.within_workspace_only {
                        "inside the worktree"
                    } else {
                        "anywhere"
                    })
                    .small()
                    .color(theme::MUTED),
                );
                ui.label(RichText::new(format!("{:?}", rule.decision)).small());
                ui.end_row();
            }
        });

    ui.add_space(10.0);
    ui.label(
        RichText::new(format!(
            "Native computer control is {:?} in this release — there is no computer runtime \
             behind it yet.",
            policy
                .rules
                .iter()
                .find(|r| r.capability == Capability::ComputerControl)
                .map(|r| r.decision)
                .unwrap_or(devorch_policy::Decision::Deny)
        ))
        .small()
        .color(theme::WAITING),
    );

    let _ = RiskLevel::L0Observe;
}

/// The color for a risk level: quiet at the bottom, loud at the top.
fn risk_color(risk: devorch_policy::RiskLevel) -> Color32 {
    use devorch_policy::RiskLevel;
    match risk {
        RiskLevel::L0Observe | RiskLevel::L1ReversibleLocal => theme::PASS,
        RiskLevel::L2LocalModification => theme::ACTIVE,
        RiskLevel::L3ExternalSideEffect | RiskLevel::L4Sensitive => theme::WAITING,
        RiskLevel::L5Destructive => theme::FAIL,
    }
}

/// Configuration and health.
pub fn settings(ui: &mut Ui, state: &UiState, config: &devorch_config::Config) {
    ui.heading("Settings");
    ui.add_space(8.0);

    egui::Grid::new("settings-grid")
        .num_columns(2)
        .striped(true)
        .spacing([24.0, 6.0])
        .show(ui, |ui| {
            let mut row = |label: &str, value: String| {
                ui.label(RichText::new(label).small().color(theme::MUTED));
                ui.label(RichText::new(value).small().monospace());
                ui.end_row();
            };

            row(
                "home",
                config
                    .home_dir()
                    .map(|p| p.display().to_string())
                    .unwrap_or_else(|_| "unavailable".into()),
            );
            row(
                "database",
                config
                    .database_path()
                    .map(|p| p.display().to_string())
                    .unwrap_or_else(|_| "unavailable".into()),
            );
            row(
                "max parallel agents",
                config.runtime.max_parallel_agents.to_string(),
            );
            row(
                "idle session ttl",
                format!("{}s", config.runtime.idle_session_ttl_seconds),
            );
            row(
                "agents installed",
                format!("{}/4", state.installed_agents()),
            );
            row("missions", state.missions.len().to_string());
            row("active workspaces", state.workspaces.len().to_string());
        });

    ui.add_space(10.0);
    ui.label(
        RichText::new(
            "Devorch keeps no idle agent processes. Agents start when a mission needs them \
             and exit when it is done.",
        )
        .small()
        .color(theme::MUTED),
    );
}
