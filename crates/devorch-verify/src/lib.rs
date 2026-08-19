//! Verification.
//!
//! An agent saying a task is done is a claim, not evidence. This crate produces
//! the evidence, and it ranks evidence in exactly one order:
//!
//! ```text
//! 1. deterministic program state   (exit codes)
//! 2. test and build output
//! 3. filesystem state
//! 4. browser / accessibility state  (not in this release)
//! 5. visual evidence                (not in this release)
//! 6. LLM judgement                  (supplementary, never decisive)
//! ```
//!
//! There is no code path here that lets a language model overrule an exit code.

use std::path::{Path, PathBuf};
use std::time::Duration;

use devorch_process::{run, ProcessError, ProcessSpec};
use serde::{Deserialize, Serialize};

/// Failure modes of running a verification step.
#[derive(Debug, thiserror::Error)]
pub enum VerifyError {
    #[error("verification command `{command}` could not run: {source}")]
    Process {
        command: String,
        #[source]
        source: ProcessError,
    },
}

/// One command whose exit status is treated as evidence.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Check {
    /// Human-readable label, e.g. "tests".
    pub name: String,
    /// Program to run.
    pub program: String,
    /// Arguments, never a shell string.
    pub args: Vec<String>,
    /// A failing required check disqualifies a candidate outright. A failing
    /// optional check is recorded and scored, but does not disqualify.
    pub required: bool,
}

impl Check {
    /// A required check named `name` running `program`.
    pub fn required(name: &str, program: &str, args: &[&str]) -> Self {
        Self {
            name: name.to_string(),
            program: program.to_string(),
            args: args.iter().map(|a| a.to_string()).collect(),
            required: true,
        }
    }

    /// An advisory check that is scored but never disqualifying.
    pub fn optional(name: &str, program: &str, args: &[&str]) -> Self {
        Self {
            required: false,
            ..Self::required(name, program, args)
        }
    }

    /// The command as it would be shown in an audit record.
    pub fn display(&self) -> String {
        std::iter::once(self.program.clone())
            .chain(self.args.iter().cloned())
            .collect::<Vec<_>>()
            .join(" ")
    }
}

/// The outcome of one check.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CheckOutcome {
    pub name: String,
    pub command: String,
    pub required: bool,
    /// `None` when the command could not be run at all.
    pub exit_code: Option<i32>,
    pub passed: bool,
    /// Tail of the combined output, bounded so a runaway log cannot fill the
    /// database.
    pub output_tail: String,
    pub duration_ms: u64,
}

/// Everything known about one candidate's correctness.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct VerificationReport {
    pub checks: Vec<CheckOutcome>,
}

impl VerificationReport {
    /// True when every required check passed.
    ///
    /// An empty report is **not** a pass. A candidate nobody verified has not
    /// been shown to work.
    pub fn passed(&self) -> bool {
        !self.checks.is_empty() && self.checks.iter().all(|c| !c.required || c.passed)
    }

    /// Required checks that failed.
    pub fn failures(&self) -> Vec<&CheckOutcome> {
        self.checks
            .iter()
            .filter(|c| c.required && !c.passed)
            .collect()
    }

    /// How many optional checks failed, used as a tie-breaker in scoring.
    pub fn advisory_failures(&self) -> usize {
        self.checks
            .iter()
            .filter(|c| !c.required && !c.passed)
            .count()
    }

    /// Total wall-clock time spent verifying.
    pub fn duration_ms(&self) -> u64 {
        self.checks.iter().map(|c| c.duration_ms).sum()
    }
}

/// How much output to retain per check.
const OUTPUT_TAIL_BYTES: usize = 4096;

/// Run every check in `dir` and collect the evidence.
///
/// Checks run in order and all of them run: a failing build still lets the test
/// command report, because "which checks failed" is more useful than "the first
/// one did".
pub async fn verify(
    dir: &Path,
    checks: &[Check],
    timeout: Option<Duration>,
) -> Result<VerificationReport, VerifyError> {
    let mut report = VerificationReport::default();

    for check in checks {
        let mut spec = ProcessSpec::new(&check.program).args(&check.args).cwd(dir);
        if let Some(timeout) = timeout {
            spec = spec.timeout(timeout);
        }

        let outcome = match run(&spec).await {
            Ok(out) => CheckOutcome {
                name: check.name.clone(),
                command: check.display(),
                required: check.required,
                exit_code: out.exit_code,
                passed: out.success(),
                output_tail: tail(&format!("{}{}", out.stdout, out.stderr)),
                duration_ms: out.duration.as_millis() as u64,
            },
            // A check that cannot run has not passed. It is never skipped
            // silently.
            Err(source) => CheckOutcome {
                name: check.name.clone(),
                command: check.display(),
                required: check.required,
                exit_code: None,
                passed: false,
                output_tail: tail(&source.to_string()),
                duration_ms: 0,
            },
        };

        tracing::info!(
            check = %outcome.name,
            passed = outcome.passed,
            exit_code = ?outcome.exit_code,
            "verification check finished"
        );
        report.checks.push(outcome);
    }

    Ok(report)
}

/// Keep the last `OUTPUT_TAIL_BYTES` of `text`, on a character boundary.
fn tail(text: &str) -> String {
    let trimmed = text.trim_end();
    if trimmed.len() <= OUTPUT_TAIL_BYTES {
        return trimmed.to_string();
    }
    let mut start = trimmed.len() - OUTPUT_TAIL_BYTES;
    while start < trimmed.len() && !trimmed.is_char_boundary(start) {
        start += 1;
    }
    format!("…{}", &trimmed[start..])
}

/// Guess a reasonable check set for the project in `dir`.
///
/// This is a convenience for the CLI, not a policy: a mission may always be
/// given explicit checks instead. Detection is by manifest file, and an
/// unrecognised project gets an empty set rather than a fabricated one —
/// pretending to verify is worse than admitting there is nothing to run.
pub fn detect_checks(dir: &Path) -> Vec<Check> {
    let has = |name: &str| dir.join(name).exists();

    if has("Cargo.toml") {
        return vec![
            Check::required("build", "cargo", &["check", "--workspace"]),
            Check::required("tests", "cargo", &["test", "--workspace"]),
            Check::optional("format", "cargo", &["fmt", "--all", "--", "--check"]),
            Check::optional(
                "lint",
                "cargo",
                &[
                    "clippy",
                    "--workspace",
                    "--all-targets",
                    "--",
                    "-D",
                    "warnings",
                ],
            ),
        ];
    }
    if has("go.mod") {
        return vec![
            Check::required("build", "go", &["build", "./..."]),
            Check::required("tests", "go", &["test", "./..."]),
            Check::optional("vet", "go", &["vet", "./..."]),
        ];
    }
    if has("package.json") {
        return vec![Check::required("tests", "npm", &["test", "--silent"])];
    }
    if has("pyproject.toml") || has("setup.py") || has("requirements.txt") {
        return vec![Check::required("tests", "python3", &["-m", "pytest", "-q"])];
    }
    Vec::new()
}

/// Paths that, if a project has them, hint at how it is tested.
pub fn project_markers(dir: &Path) -> Vec<PathBuf> {
    ["Cargo.toml", "go.mod", "package.json", "pyproject.toml"]
        .iter()
        .map(|m| dir.join(m))
        .filter(|p| p.exists())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn a_passing_check_is_recorded_as_evidence() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![Check::required("ok", "sh", &["-c", "echo fine"])];
        let report = verify(dir.path(), &checks, None).await.unwrap();

        assert!(report.passed());
        assert_eq!(report.checks[0].exit_code, Some(0));
        assert!(report.checks[0].output_tail.contains("fine"));
    }

    #[tokio::test]
    async fn a_failing_required_check_disqualifies() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![Check::required(
            "tests",
            "sh",
            &["-c", "echo boom >&2; exit 1"],
        )];
        let report = verify(dir.path(), &checks, None).await.unwrap();

        assert!(!report.passed());
        assert_eq!(report.failures().len(), 1);
        assert!(report.checks[0].output_tail.contains("boom"));
    }

    #[tokio::test]
    async fn a_failing_optional_check_is_scored_but_not_disqualifying() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![
            Check::required("tests", "sh", &["-c", "exit 0"]),
            Check::optional("lint", "sh", &["-c", "exit 1"]),
        ];
        let report = verify(dir.path(), &checks, None).await.unwrap();

        assert!(report.passed());
        assert_eq!(report.advisory_failures(), 1);
    }

    #[tokio::test]
    async fn every_check_runs_even_after_one_fails() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![
            Check::required("build", "sh", &["-c", "exit 1"]),
            Check::required("tests", "sh", &["-c", "exit 2"]),
        ];
        let report = verify(dir.path(), &checks, None).await.unwrap();

        assert_eq!(report.checks.len(), 2, "the second check must still run");
        assert_eq!(report.checks[1].exit_code, Some(2));
    }

    #[tokio::test]
    async fn a_check_that_cannot_run_is_a_failure_not_a_skip() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![Check::required("tests", "devorch-no-such-binary-xyz", &[])];
        let report = verify(dir.path(), &checks, None).await.unwrap();

        assert!(!report.passed());
        assert_eq!(report.checks[0].exit_code, None);
    }

    #[tokio::test]
    async fn a_hanging_check_fails_rather_than_blocking_the_mission() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![Check::required("tests", "sh", &["-c", "sleep 30"])];
        let report = verify(dir.path(), &checks, Some(Duration::from_millis(200)))
            .await
            .unwrap();

        assert!(!report.passed());
    }

    #[test]
    fn an_empty_report_is_not_a_pass() {
        assert!(!VerificationReport::default().passed());
    }

    #[tokio::test]
    async fn runaway_output_is_truncated() {
        let dir = tempfile::tempdir().unwrap();
        let checks = vec![Check::required(
            "noisy",
            "sh",
            &["-c", "yes noisyoutput | head -c 100000"],
        )];
        let report = verify(dir.path(), &checks, None).await.unwrap();

        assert!(report.passed());
        assert!(
            report.checks[0].output_tail.len() <= OUTPUT_TAIL_BYTES + 8,
            "output was not truncated: {}",
            report.checks[0].output_tail.len()
        );
    }

    #[test]
    fn checks_are_detected_from_the_project_manifest() {
        let dir = tempfile::tempdir().unwrap();
        assert!(detect_checks(dir.path()).is_empty());

        std::fs::write(dir.path().join("Cargo.toml"), "[package]\n").unwrap();
        let checks = detect_checks(dir.path());
        assert!(checks.iter().any(|c| c.name == "tests" && c.required));
        assert!(checks.iter().any(|c| c.name == "lint" && !c.required));
    }
}
