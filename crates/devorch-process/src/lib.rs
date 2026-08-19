//! Cross-platform process execution.
//!
//! The Go baseline reached directly for `/bin/zsh`, `SIGHUP` and a Unix-only PTY
//! crate. That is a correct Unix implementation but not a process layer for
//! macOS, Windows and Linux at once, so this crate defines the platform-neutral
//! contract first and lets platform detail live behind it.

use std::collections::BTreeMap;
use std::ffi::OsStr;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::Duration;

use tokio::process::Command;

/// Failure modes of running a child process.
#[derive(Debug, thiserror::Error)]
pub enum ProcessError {
    #[error("failed to spawn `{program}`: {source}")]
    Spawn {
        program: String,
        #[source]
        source: std::io::Error,
    },

    #[error("`{program}` was not found on PATH")]
    NotFound { program: String },

    #[error("`{program}` did not finish within {timeout:?}")]
    Timeout { program: String, timeout: Duration },

    #[error("i/o error while running `{program}`: {source}")]
    Io {
        program: String,
        #[source]
        source: std::io::Error,
    },
}

/// A command to run, described without reference to any shell.
///
/// Devorch never builds a shell string: arguments are passed as a vector so a
/// path containing a space or a quote cannot change what gets executed.
#[derive(Debug, Clone)]
pub struct ProcessSpec {
    pub program: String,
    pub args: Vec<String>,
    pub cwd: Option<PathBuf>,
    /// Environment given to the child. `None` inherits the parent environment;
    /// agent processes should always pass `Some`, so the secret broker controls
    /// exactly which credentials are visible.
    pub env: Option<BTreeMap<String, String>>,
    pub timeout: Option<Duration>,
}

impl ProcessSpec {
    /// Start describing a run of `program`.
    pub fn new(program: impl Into<String>) -> Self {
        Self {
            program: program.into(),
            args: Vec::new(),
            cwd: None,
            env: None,
            timeout: None,
        }
    }

    /// Append one argument.
    pub fn arg(mut self, arg: impl Into<String>) -> Self {
        self.args.push(arg.into());
        self
    }

    /// Append several arguments.
    pub fn args<I, S>(mut self, args: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: AsRef<OsStr>,
    {
        self.args.extend(
            args.into_iter()
                .map(|a| a.as_ref().to_string_lossy().into_owned()),
        );
        self
    }

    /// Run the child in `dir`.
    pub fn cwd(mut self, dir: impl AsRef<Path>) -> Self {
        self.cwd = Some(dir.as_ref().to_path_buf());
        self
    }

    /// Replace the child's entire environment.
    pub fn env(mut self, env: BTreeMap<String, String>) -> Self {
        self.env = Some(env);
        self
    }

    /// Kill the child if it has not finished within `timeout`.
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = Some(timeout);
        self
    }

    /// Render the command for logs and audit records. Not for execution.
    pub fn display(&self) -> String {
        let mut out = self.program.clone();
        for arg in &self.args {
            out.push(' ');
            out.push_str(arg);
        }
        out
    }
}

/// What a finished process produced.
#[derive(Debug, Clone)]
pub struct ProcessOutput {
    pub exit_code: Option<i32>,
    pub stdout: String,
    pub stderr: String,
    pub duration: Duration,
}

impl ProcessOutput {
    /// True when the process exited with status 0.
    pub fn success(&self) -> bool {
        self.exit_code == Some(0)
    }

    /// stdout with trailing whitespace removed.
    pub fn stdout_trimmed(&self) -> &str {
        self.stdout.trim_end()
    }

    /// Non-empty stdout lines.
    pub fn stdout_lines(&self) -> impl Iterator<Item = &str> {
        self.stdout.lines().filter(|l| !l.trim().is_empty())
    }
}

/// Run `spec` to completion, capturing stdout and stderr.
pub async fn run(spec: &ProcessSpec) -> Result<ProcessOutput, ProcessError> {
    let started = std::time::Instant::now();

    let mut cmd = Command::new(&spec.program);
    cmd.args(&spec.args)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true);

    if let Some(dir) = &spec.cwd {
        cmd.current_dir(dir);
    }
    if let Some(env) = &spec.env {
        cmd.env_clear();
        for (k, v) in env {
            cmd.env(k, v);
        }
    }

    let child = cmd.spawn().map_err(|source| {
        if source.kind() == std::io::ErrorKind::NotFound {
            ProcessError::NotFound {
                program: spec.program.clone(),
            }
        } else {
            ProcessError::Spawn {
                program: spec.program.clone(),
                source,
            }
        }
    })?;

    let output = match spec.timeout {
        Some(timeout) => match tokio::time::timeout(timeout, child.wait_with_output()).await {
            Ok(result) => result,
            Err(_) => {
                // kill_on_drop reaps the child when the future is dropped.
                return Err(ProcessError::Timeout {
                    program: spec.program.clone(),
                    timeout,
                });
            }
        },
        None => child.wait_with_output().await,
    }
    .map_err(|source| ProcessError::Io {
        program: spec.program.clone(),
        source,
    })?;

    Ok(ProcessOutput {
        exit_code: output.status.code(),
        stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
        stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
        duration: started.elapsed(),
    })
}

/// Whether `program` resolves on PATH, without running it.
pub async fn which(program: &str) -> Option<PathBuf> {
    let finder = if cfg!(windows) { "where" } else { "which" };
    let spec = ProcessSpec::new(finder).arg(program);
    let out = run(&spec).await.ok()?;
    if !out.success() {
        return None;
    }
    let first = out.stdout_lines().next().map(PathBuf::from);
    first
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn runs_a_command_and_captures_stdout() {
        let out = run(&ProcessSpec::new("echo").arg("hello"))
            .await
            .expect("run");
        assert!(out.success());
        assert_eq!(out.stdout_trimmed(), "hello");
    }

    #[tokio::test]
    async fn reports_a_nonzero_exit_code() {
        let out = run(&ProcessSpec::new("sh").arg("-c").arg("exit 3"))
            .await
            .expect("run");
        assert!(!out.success());
        assert_eq!(out.exit_code, Some(3));
    }

    #[tokio::test]
    async fn missing_program_is_not_found_rather_than_a_generic_spawn_error() {
        let err = run(&ProcessSpec::new("devorch-no-such-binary-xyz"))
            .await
            .expect_err("should fail");
        assert!(matches!(err, ProcessError::NotFound { .. }));
    }

    #[tokio::test]
    async fn a_hanging_process_times_out() {
        let spec = ProcessSpec::new("sh")
            .arg("-c")
            .arg("sleep 30")
            .timeout(Duration::from_millis(200));
        let err = run(&spec).await.expect_err("should time out");
        assert!(matches!(err, ProcessError::Timeout { .. }));
    }

    #[tokio::test]
    async fn env_is_replaced_not_merged() {
        let mut env = BTreeMap::new();
        env.insert("DEVORCH_TEST_VAR".to_string(), "scoped".to_string());
        // PATH is required for `sh` to find `printenv` on some systems.
        env.insert(
            "PATH".to_string(),
            std::env::var("PATH").unwrap_or_default(),
        );

        std::env::set_var("DEVORCH_LEAK_CHECK", "leaked");
        let spec = ProcessSpec::new("sh")
            .arg("-c")
            .arg("echo \"$DEVORCH_TEST_VAR:${DEVORCH_LEAK_CHECK:-absent}\"")
            .env(env);
        let out = run(&spec).await.expect("run");
        assert_eq!(out.stdout_trimmed(), "scoped:absent");
    }

    #[tokio::test]
    async fn cwd_is_honoured() {
        let dir = std::env::temp_dir();
        let out = run(&ProcessSpec::new("pwd").cwd(&dir)).await.expect("run");
        assert!(out.success());
        assert!(!out.stdout_trimmed().is_empty());
    }

    #[tokio::test]
    async fn which_finds_a_known_binary_and_misses_an_unknown_one() {
        assert!(which("sh").await.is_some());
        assert!(which("devorch-no-such-binary-xyz").await.is_none());
    }

    #[test]
    fn display_renders_program_and_args() {
        let spec = ProcessSpec::new("git").arg("status").arg("--porcelain=v1");
        assert_eq!(spec.display(), "git status --porcelain=v1");
    }
}
