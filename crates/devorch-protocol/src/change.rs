//! Candidate change inventory.
//!
//! The development simulation proved that judging a candidate on `git diff`
//! alone is wrong: `git diff --numstat` omits untracked files, so an agent that
//! drops an unrequested `NOTES.md` into the worktree looks identical to one that
//! did not. Every category below must therefore be collected before a candidate
//! is scored or merged.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

/// The complete set of changes a candidate made to its worktree.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ChangeInventory {
    pub tracked_modified: Vec<PathBuf>,
    pub staged: Vec<PathBuf>,
    pub untracked: Vec<PathBuf>,
    pub deleted: Vec<PathBuf>,
    pub binary: Vec<PathBuf>,
    pub generated: Vec<PathBuf>,
    /// Added + removed lines across tracked changes.
    pub churn: u64,
}

impl ChangeInventory {
    /// Every distinct path the candidate touched, across all categories.
    pub fn all_paths(&self) -> BTreeSet<&Path> {
        let mut paths = BTreeSet::new();
        for group in [
            &self.tracked_modified,
            &self.staged,
            &self.untracked,
            &self.deleted,
            &self.binary,
            &self.generated,
        ] {
            paths.extend(group.iter().map(PathBuf::as_path));
        }
        paths
    }

    /// Number of distinct touched paths. This, not the diff line count, is the
    /// scope measure used when comparing candidates.
    pub fn touched_count(&self) -> usize {
        self.all_paths().len()
    }

    /// True when the candidate changed nothing at all.
    pub fn is_empty(&self) -> bool {
        self.touched_count() == 0
    }

    /// Paths that fall outside `owned`, i.e. files the ticket did not authorize.
    pub fn paths_outside<'a>(&'a self, owned: &[PathBuf]) -> Vec<&'a Path> {
        self.all_paths()
            .into_iter()
            .filter(|p| !owned.iter().any(|root| p.starts_with(root)))
            .collect()
    }
}

/// Why a candidate was rejected before scoring.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum InventoryViolation {
    /// A binary blob appeared in a source change.
    UnexpectedBinary { path: PathBuf },
    /// A build output, cache or log the ticket does not own.
    UnexpectedGenerated { path: PathBuf },
    /// A file outside the paths the ticket assigned to this candidate.
    OutsideOwnedPaths { path: PathBuf },
    /// Something that looks like a credential.
    PossibleSecret { path: PathBuf },
}

/// Path patterns treated as generated/derived output rather than source.
const GENERATED_MARKERS: &[&str] = &[
    "__pycache__/",
    "/node_modules/",
    "/target/debug/",
    "/target/release/",
    "/.pytest_cache/",
    "/dist/",
    "/build/",
    "/vendor/",
];

const GENERATED_SUFFIXES: &[&str] = &[".pyc", ".pyo", ".class", ".o", ".obj", ".log", ".tmp"];

const SECRET_MARKERS: &[&str] = &[
    ".env",
    "id_rsa",
    "id_ed25519",
    ".pem",
    ".p12",
    "credentials.json",
    ".npmrc",
];

/// True when `path` looks like generated or derived output.
///
/// The check is deliberately conservative: it matches on well-known build,
/// cache and log conventions rather than trying to infer intent.
pub fn is_generated(path: &Path) -> bool {
    // Normalize to forward slashes with a leading slash so the markers match at
    // a segment boundary rather than mid-name.
    let text = format!("/{}", path.to_string_lossy().replace('\\', "/"));
    GENERATED_MARKERS.iter().any(|m| text.contains(m))
        || GENERATED_SUFFIXES.iter().any(|s| text.ends_with(s))
}

/// True when `path` looks like it may hold a credential.
pub fn looks_like_secret(path: &Path) -> bool {
    let text = path.to_string_lossy().replace('\\', "/");
    let name = text.rsplit('/').next().unwrap_or(&text);
    SECRET_MARKERS
        .iter()
        .any(|m| name == *m || name.starts_with(m) || name.ends_with(m))
}

/// Check an inventory against the paths a ticket authorized.
///
/// An empty `owned` list means the ticket did not restrict paths, so the
/// ownership check is skipped; the binary/generated/secret checks always run.
pub fn check_inventory(inv: &ChangeInventory, owned: &[PathBuf]) -> Vec<InventoryViolation> {
    let mut violations = Vec::new();

    for path in &inv.binary {
        violations.push(InventoryViolation::UnexpectedBinary { path: path.clone() });
    }

    for path in inv.all_paths() {
        if is_generated(path) && !inv.binary.iter().any(|b| b == path) {
            violations.push(InventoryViolation::UnexpectedGenerated {
                path: path.to_path_buf(),
            });
        }
        if looks_like_secret(path) {
            violations.push(InventoryViolation::PossibleSecret {
                path: path.to_path_buf(),
            });
        }
    }

    if !owned.is_empty() {
        for path in inv.paths_outside(owned) {
            violations.push(InventoryViolation::OutsideOwnedPaths {
                path: path.to_path_buf(),
            });
        }
    }

    violations
}

#[cfg(test)]
mod tests {
    use super::*;

    fn inv_with_untracked(paths: &[&str]) -> ChangeInventory {
        ChangeInventory {
            untracked: paths.iter().map(PathBuf::from).collect(),
            ..Default::default()
        }
    }

    #[test]
    fn untracked_files_count_toward_scope() {
        // The exact simulation case: same tracked edit, one extra untracked file.
        let lean = ChangeInventory {
            tracked_modified: vec![PathBuf::from("calculator.py")],
            churn: 2,
            ..Default::default()
        };
        let wide = ChangeInventory {
            tracked_modified: vec![PathBuf::from("calculator.py")],
            untracked: vec![PathBuf::from("NOTES.md")],
            churn: 2,
            ..Default::default()
        };
        assert_eq!(lean.touched_count(), 1);
        assert_eq!(wide.touched_count(), 2);
        assert!(wide.touched_count() > lean.touched_count());
    }

    #[test]
    fn all_paths_deduplicates_across_categories() {
        let inv = ChangeInventory {
            tracked_modified: vec![PathBuf::from("a.rs")],
            staged: vec![PathBuf::from("a.rs")],
            ..Default::default()
        };
        assert_eq!(inv.touched_count(), 1);
    }

    #[test]
    fn empty_inventory_is_empty() {
        assert!(ChangeInventory::default().is_empty());
        assert!(!inv_with_untracked(&["x"]).is_empty());
    }

    #[test]
    fn generated_paths_are_detected() {
        assert!(is_generated(Path::new(
            "pkg/__pycache__/mod.cpython-311.pyc"
        )));
        assert!(is_generated(Path::new("target/debug/devorch")));
        assert!(is_generated(Path::new(
            "app/node_modules/left-pad/index.js"
        )));
        assert!(is_generated(Path::new("run.log")));
        assert!(!is_generated(Path::new("src/main.rs")));
        // "build" must match a directory segment, not a filename prefix.
        assert!(!is_generated(Path::new("src/builder.rs")));
    }

    #[test]
    fn secret_like_paths_are_detected() {
        assert!(looks_like_secret(Path::new(".env")));
        assert!(looks_like_secret(Path::new("deploy/.env.production")));
        assert!(looks_like_secret(Path::new("keys/server.pem")));
        assert!(!looks_like_secret(Path::new("src/environment.rs")));
    }

    #[test]
    fn inventory_check_flags_generated_and_secret_files() {
        let inv = inv_with_untracked(&["__pycache__/x.pyc", ".env", "src/ok.rs"]);
        let violations = check_inventory(&inv, &[]);
        assert!(violations
            .iter()
            .any(|v| matches!(v, InventoryViolation::UnexpectedGenerated { .. })));
        assert!(violations
            .iter()
            .any(|v| matches!(v, InventoryViolation::PossibleSecret { .. })));
    }

    #[test]
    fn inventory_check_flags_paths_outside_ownership() {
        let inv = ChangeInventory {
            tracked_modified: vec![
                PathBuf::from("crates/a/src/lib.rs"),
                PathBuf::from("Cargo.toml"),
            ],
            ..Default::default()
        };
        let violations = check_inventory(&inv, &[PathBuf::from("crates/a")]);
        assert_eq!(violations.len(), 1);
        assert!(matches!(
            &violations[0],
            InventoryViolation::OutsideOwnedPaths { path } if path == Path::new("Cargo.toml")
        ));
    }

    #[test]
    fn clean_inventory_inside_ownership_has_no_violations() {
        let inv = ChangeInventory {
            tracked_modified: vec![PathBuf::from("crates/a/src/lib.rs")],
            churn: 4,
            ..Default::default()
        };
        assert!(check_inventory(&inv, &[PathBuf::from("crates/a")]).is_empty());
    }
}
