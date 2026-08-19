//! The Devorch desktop UI.
//!
//! The UI is a **client of core state**, never a second implementation of it.
//! A click produces a [`Command`]; the core produces state; the UI renders it.
//! That indirection is what keeps the CLI, this window and any future remote
//! surface sharing one behavior — and what keeps the interesting logic testable
//! without a window.
//!
//! Concretely: nothing in this crate creates a worktree, scores a candidate or
//! decides a merge. It asks [`devorch_mission`] to, and draws the answer.

pub mod app;
pub mod graph3d;
pub mod state;
pub mod theme;
pub mod views;

pub use app::DevorchApp;
pub use state::{Command, Screen, UiState};

/// Launch the desktop application.
///
/// The window title carries the version so a bug report screenshot identifies
/// the build without the reporter having to look it up.
pub fn run() -> eframe::Result<()> {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1440.0, 900.0])
            .with_min_inner_size([960.0, 600.0])
            .with_title(format!("Devorch {}", env!("CARGO_PKG_VERSION"))),
        ..Default::default()
    };

    eframe::run_native(
        "devorch",
        options,
        Box::new(|cc| {
            theme::install(&cc.egui_ctx);
            Ok(Box::new(DevorchApp::new()))
        }),
    )
}
