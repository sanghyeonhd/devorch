# Devorch

**A local-first multi-agent development orchestrator, written entirely in Rust.**

Devorch runs several AI coding agents against the same task in isolated git
worktrees, judges their work on deterministic evidence, and merges the one that
actually passes.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

---

## The idea

Four coding agents are installed on a typical machine today — Codex, Claude
Code, Grok Build, Gemini CLI. Most tools treat that as a menu. Devorch treats it
as a comparison.

Three decisions follow from that, and they are what distinguishes this tool:

**Evidence outranks assertion.** An agent saying it fixed the bug is a claim.
The exit code of the test suite is evidence. A candidate that fails the gate
cannot win because it wrote a smaller diff, took less time, or said it
succeeded. There is no code path where a language model overrules a test result.

**The whole change set counts.** `git diff --numstat` does not show untracked
files. An agent that fixes the bug *and* drops an unrequested `NOTES.md` looks
identical to one that only fixed the bug — unless you look at
`git status --porcelain=v1`, `git ls-files --others` and the staged diff too.
Devorch looks at all of them, and classifies what it finds: build output,
binaries, and anything that resembles a credential are grounds for rejection,
not footnotes.

**Four agents installed does not mean four agents running.** Four CLIs plus a
browser dwarf the orchestrator's own footprint, so candidate count is the
largest resource decision the system makes. Low-risk work gets one agent.
High-risk work compares four — two at a time. Idle agent processes: zero.

## What a mission does

```
goal
 └─ router picks how many candidates, and which
     └─ one git worktree per candidate, branched from the same base
         └─ each agent runs in its own worktree, streaming normalized events
             └─ deterministic gate: build, tests, lint
                 └─ complete change inventory, including untracked files
                     └─ compare → winner
                         └─ merge onto an integration branch
                             └─ run the gate again on the merged result
                                 └─ losing worktrees removed, records kept
```

The gate runs twice on purpose. A candidate that passes alone can still break
the integration branch; if the merged result fails, the branch is rolled back
to base and the mission fails rather than leaving a bad merge behind.

## Install

Requires Rust 1.85+ and git.

```bash
git clone https://github.com/sanghyeonhd/devorch
cd devorch
cargo build --release
```

Two binaries are produced: `target/release/devorch` (CLI) and
`target/release/devorch-ui` (desktop app). On macOS, `cargo xtask bundle-macos`
wraps the UI in a proper `.app`.

Devorch drives whichever agent CLIs you already have installed and signed in.
It installs nothing and stores no credentials.

## Use

```bash
# What is installed, and what can each one actually do?
devorch agent inspect

# Run a mission.
devorch mission run \
  --repo . \
  --goal "Fix the failing authentication test" \
  --risk high

# Compare without merging.
devorch mission run --repo . --goal "..." --dry-run

# What happened?
devorch mission list
devorch mission show <id>
```

Capabilities are read from the installed binaries at runtime, never assumed. If
a vendor renames a flag, the capability disappears and the router stops choosing
that mode — instead of a mission failing halfway through.

### The desktop app

```bash
cargo run --release -p devorch-ui-bin
```

Six screens: missions with a live log, the agent board, workspaces, a compare
view, the policy as it actually evaluates, and settings. The compare view has
two modes — an evidence table, and a 3D constellation that shows the shape of a
mission: which candidates ran, which were cut, and what reached the merge.

To see it with data before running anything real:

```bash
cargo run -p devorch-mission --example seed_demo -- /tmp/devorch-demo
DEVORCH_HOME=/tmp/devorch-demo cargo run --release -p devorch-ui-bin
```

## Policy

Devorch owns the final permission decision. Vendor approval and sandbox flags
are inputs, not authority — an agent started with a permissive vendor flag still
has to pass this engine.

Policy is written against capabilities, not categories. "Allow shell" is not a
policy; `process.run` inside the candidate's own worktree is. Scope is what
separates routine from damage: the same write is ordinary work inside a worktree
and refused outside one, with `..` resolved before the file exists rather than
after. A risk level with no rule denies — policy gaps fail closed.

## Configuration

`~/.devorch/config.toml`, all optional:

```toml
[runtime]
max_parallel_agents = 2      # peak agent processes, regardless of candidate count
idle_session_ttl_seconds = 300
persistent_protocols = "auto"

[agents]
enabled = ["codex", "claude", "grok", "gemini"]
```

The Rust core uses `~/.devorch/devorch3.db`. If you ran the Go build, its
`devorch.db` is untouched and still there.

## Development

```bash
cargo fmt --all -- --check
cargo check --workspace
cargo test --workspace
cargo clippy --workspace --all-targets -- -D warnings
cargo xtask verify-agent-rules
```

Tests do not require an API key or a subscription. Adapter conformance runs
against recorded streams in `testdata/agents/`, including live captures from the
real CLIs; missions run end to end against real git with scripted agents.

Live smoke against the actual CLIs is opt-in:

```bash
DEVORCH_LIVE_GROK=1 cargo test --workspace -- --ignored
```

Agent rules live in `docs/agent-rules.md`; `AGENTS.md`, `CLAUDE.md` and
`GEMINI.md` are generated from it by `cargo xtask sync-agent-rules`, and CI
fails if they drift.

## What this release does not do

Stated plainly rather than left to discovery:

- **No native OS control.** No mouse, no keyboard, no screen capture. The policy
  engine denies it and there is no runtime behind it.
- **No browser automation.**
- **No autonomous operation.** Every mission is started by a person.
- **OkAON learning data is not migrated** from the Go database. The schema map
  in `docs/rust-migration/sql-schema-map.md` records the disposition of all 22
  tables; the migration itself is future work.
- **Windows and Linux are built in CI but not yet exercised in anger.** The
  process layer was written platform-neutral; macOS is where it has actually run.

## History

Devorch was a Go project through v0.1.0. The full Go tree is preserved at the
tag `devorch-go-final`. The migration, its measurements and the two development
simulations that shaped it are documented in `docs/rust-migration/` and
`docs/upgrade/`. What actually shipped in 3.0.0-alpha.1 — adapters, the mission
loop, the GUI, Go retirement, and the live defects that changed the adapters —
is in `docs/upgrade/DEVORCH_3_RELEASE.md`.

## License

Apache-2.0. See [LICENSE](LICENSE), [NOTICE](NOTICE) and
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Devorch contains no ported upstream source; the vendor adapters were written
against recorded CLI output. See `docs/upstream-sources.toml`.
