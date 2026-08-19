//! Repository automation.
//!
//! The four agent products each expect their own instruction file. Maintaining
//! four hand-written copies guarantees they drift, so `docs/agent-rules.md` is
//! the single source and the vendor files are generated from it.

use std::path::{Path, PathBuf};

use anyhow::{bail, Context, Result};

/// The canonical rules document every vendor file is generated from.
const SOURCE: &str = "docs/agent-rules.md";

/// Generated file → the agent products that read it.
const TARGETS: &[(&str, &str)] = &[
    ("AGENTS.md", "Codex and Grok"),
    ("CLAUDE.md", "Claude Code"),
    ("GEMINI.md", "Gemini CLI"),
];

fn main() -> Result<()> {
    let task = std::env::args().nth(1);
    match task.as_deref() {
        Some("sync-agent-rules") => sync(false),
        Some("verify-agent-rules") => sync(true),
        Some("bundle-macos") => bundle_macos(),
        Some(other) => {
            bail!(
                "unknown task `{other}`; expected sync-agent-rules, verify-agent-rules \
                 or bundle-macos"
            )
        }
        None => {
            eprintln!("usage: cargo xtask <sync-agent-rules|verify-agent-rules|bundle-macos>");
            std::process::exit(2);
        }
    }
}

/// Wrap the release UI binary in a macOS `.app` bundle.
///
/// A bare Mach-O binary runs, but macOS treats it as a faceless process: no
/// Dock entry, no menu bar, and no application identity for the accessibility
/// and screen-recording permissions the OS grants per app. The bundle is what
/// makes it an application rather than a program that happens to open a window.
fn bundle_macos() -> Result<()> {
    if !cfg!(target_os = "macos") {
        bail!("bundle-macos only runs on macOS");
    }

    let root = repo_root()?;
    let binary = root.join("target/release/devorch-ui");
    if !binary.exists() {
        bail!(
            "{} does not exist; run `cargo build --release -p devorch-ui-bin` first",
            binary.display()
        );
    }

    let app = root.join("target/release/Devorch.app");
    let macos_dir = app.join("Contents/MacOS");
    if app.exists() {
        std::fs::remove_dir_all(&app).with_context(|| format!("clear {}", app.display()))?;
    }
    std::fs::create_dir_all(&macos_dir)?;
    std::fs::copy(&binary, macos_dir.join("devorch-ui"))
        .with_context(|| format!("copy {}", binary.display()))?;

    let version = env!("CARGO_PKG_VERSION");
    std::fs::write(
        app.join("Contents/Info.plist"),
        format!(
            r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>Devorch</string>
    <key>CFBundleDisplayName</key>
    <string>Devorch</string>
    <key>CFBundleIdentifier</key>
    <string>dev.devorch.desktop</string>
    <key>CFBundleVersion</key>
    <string>{version}</string>
    <key>CFBundleShortVersionString</key>
    <string>{version}</string>
    <key>CFBundleExecutable</key>
    <string>devorch-ui</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
"#
        ),
    )
    .with_context(|| "write Info.plist")?;

    println!("bundled {}", app.display());
    Ok(())
}

/// Generate the vendor rule files, or in `check` mode verify they are current.
fn sync(check: bool) -> Result<()> {
    let root = repo_root()?;
    let source_path = root.join(SOURCE);
    let source = std::fs::read_to_string(&source_path)
        .with_context(|| format!("read {}", source_path.display()))?;

    let mut stale = Vec::new();
    for (file, audience) in TARGETS {
        let path = root.join(file);
        let wanted = render(&source, file, audience);

        if check {
            let current = std::fs::read_to_string(&path).unwrap_or_default();
            if current != wanted {
                stale.push(*file);
            }
        } else {
            std::fs::write(&path, wanted).with_context(|| format!("write {}", path.display()))?;
            println!("wrote {file}");
        }
    }

    if check && !stale.is_empty() {
        bail!(
            "{} out of date; run `cargo xtask sync-agent-rules`",
            stale.join(", ")
        );
    }
    if check {
        println!("agent rules are up to date");
    }
    Ok(())
}

/// Wrap the canonical rules in a generated-file header for one vendor.
fn render(source: &str, file: &str, audience: &str) -> String {
    format!(
        "<!-- GENERATED FILE — DO NOT EDIT.\n     Source: {SOURCE}\n     Regenerate: cargo xtask sync-agent-rules\n     Read by: {audience} -->\n\n# Devorch agent rules ({file})\n\n{source}"
    )
}

/// Walk up from the current directory to the directory holding the source doc.
fn repo_root() -> Result<PathBuf> {
    let mut dir: &Path = &std::env::current_dir()?;
    loop {
        if dir.join(SOURCE).exists() {
            return Ok(dir.to_path_buf());
        }
        match dir.parent() {
            Some(parent) => dir = parent,
            None => bail!("could not find {SOURCE}; run this from inside the repository"),
        }
    }
}
