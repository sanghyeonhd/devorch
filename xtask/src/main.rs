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
        Some(other) => {
            bail!("unknown task `{other}`; expected sync-agent-rules or verify-agent-rules")
        }
        None => {
            eprintln!("usage: cargo xtask <sync-agent-rules|verify-agent-rules>");
            std::process::exit(2);
        }
    }
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
