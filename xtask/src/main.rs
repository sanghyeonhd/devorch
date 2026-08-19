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
        Some("third-party-notices") => third_party_notices(),
        Some(other) => {
            bail!(
                "unknown task `{other}`; expected sync-agent-rules, verify-agent-rules, \
                 bundle-macos or third-party-notices"
            )
        }
        None => {
            eprintln!(
                "usage: cargo xtask \
                 <sync-agent-rules|verify-agent-rules|bundle-macos|third-party-notices>"
            );
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
    let source =
        read_lf(&source_path).with_context(|| format!("read {}", source_path.display()))?;

    let mut stale = Vec::new();
    for (file, audience) in TARGETS {
        let path = root.join(file);
        let wanted = render(&source, file, audience);

        if check {
            let current = read_lf(&path).unwrap_or_default();
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

/// Read a text file as LF. Windows checkouts with `core.autocrlf` turn the
/// vendor files into CRLF, while `render` always emits LF — byte equality then
/// fails even when the content is the same.
fn read_lf(path: &Path) -> Result<String> {
    let raw = std::fs::read_to_string(path)?;
    Ok(normalize_lf(&raw))
}

fn normalize_lf(text: &str) -> String {
    text.replace("\r\n", "\n").replace('\r', "\n")
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

/// Regenerate `THIRD_PARTY_NOTICES.md` from the actual dependency graph.
///
/// The list comes from `cargo tree`, not from the manifests, so it describes
/// what really links into the binaries. A notices file that drifts from the
/// build is worse than none: it makes a claim about redistribution that nobody
/// has checked.
fn third_party_notices() -> Result<()> {
    use std::collections::BTreeMap;

    let root = repo_root()?;
    let output = std::process::Command::new("cargo")
        .args([
            "tree",
            "--workspace",
            "--edges",
            "normal",
            "--prefix",
            "none",
            "--format",
            "{p}|{l}",
        ])
        .current_dir(&root)
        .output()
        .context("run cargo tree")?;

    if !output.status.success() {
        bail!(
            "cargo tree failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }

    // (crate, version) -> license. A BTreeMap both deduplicates the repeated
    // entries cargo prints for shared dependencies and sorts the result, so the
    // file only changes when the dependency graph does.
    let mut crates: BTreeMap<(String, String), String> = BTreeMap::new();
    let mut by_license: BTreeMap<String, usize> = BTreeMap::new();

    for line in String::from_utf8_lossy(&output.stdout).lines() {
        let Some((name, license)) = line.split_once('|') else {
            continue;
        };
        let name = name.replace(" (proc-macro)", "").replace(" (*)", "");
        let name = name.trim();
        let license = license.replace(" (*)", "");
        let license = license.trim();

        if name.starts_with("devorch") || license.is_empty() {
            continue;
        }
        let (krate, version) = match name.rsplit_once(" v") {
            Some((k, v)) => (k.trim().to_string(), v.trim().to_string()),
            None => (name.to_string(), String::new()),
        };
        if crates
            .insert((krate, version), license.to_string())
            .is_none()
        {
            *by_license.entry(license.to_string()).or_default() += 1;
        }
    }

    if crates.is_empty() {
        bail!("cargo tree produced no dependencies; refusing to write an empty notices file");
    }

    let mut out = String::new();
    out.push_str("# Third-party notices\n\n");
    out.push_str("Devorch is licensed under the Apache License 2.0 (see `LICENSE`).\n\n");
    out.push_str(
        "It links the Rust crates listed below. Every one of them is under a permissive\n\
         license compatible with Apache-2.0; there is no copyleft dependency in the\n\
         build. This file is generated from `cargo tree`, so it describes what actually\n\
         links rather than what a manifest intends.\n\n",
    );
    out.push_str(
        "Devorch contains no ported upstream source. The vendor adapters were written\n\
         against recorded CLI output, not against vendor code — see\n\
         `docs/upstream-sources.toml`.\n\n",
    );

    out.push_str("## Summary\n\n| License | Crates |\n|---|---:|\n");
    let mut summary: Vec<_> = by_license.into_iter().collect();
    summary.sort_by(|a, b| b.1.cmp(&a.1).then_with(|| a.0.cmp(&b.0)));
    for (license, count) in summary {
        out.push_str(&format!("| {license} | {count} |\n"));
    }
    out.push_str(&format!("\n**{} distinct crates.**\n\n", crates.len()));

    out.push_str("## Crates\n\n| Crate | Version | License |\n|---|---|---|\n");
    for ((krate, version), license) in &crates {
        out.push_str(&format!("| {krate} | {version} | {license} |\n"));
    }

    out.push_str(
        "\n## Fonts\n\n\
         `epaint_default_fonts` bundles the fonts egui renders with. They carry their\n\
         own licenses — OFL-1.1 and UFL-1.0 — which permit redistribution as part of a\n\
         larger work and are listed here for that reason.\n\n\
         ## Regenerating\n\n```bash\ncargo xtask third-party-notices\n```\n",
    );

    let path = root.join("THIRD_PARTY_NOTICES.md");
    std::fs::write(&path, out).with_context(|| format!("write {}", path.display()))?;
    println!("wrote {} ({} crates)", path.display(), crates.len());
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn crlf_checkouts_compare_equal_to_lf_renders() {
        let source = "rule one\nrule two\n";
        let wanted = render(source, "AGENTS.md", "Codex and Grok");
        let crlf = wanted.replace('\n', "\r\n");
        assert_ne!(crlf, wanted, "CRLF and LF are not byte-equal");
        assert_eq!(normalize_lf(&crlf), wanted);
    }
}
