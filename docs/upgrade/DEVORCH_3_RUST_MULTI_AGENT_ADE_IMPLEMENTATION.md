# Devorch 3 — Rust-First Multi-Agent ADE / GUI Implementation Plan

**Version:** 3.0 — Rust-first, four-agent, execution-ready  
**Target repository:** `github.com/sanghyeonhd/devorch`  
**Validated repository baseline:** `main@9ecdc71d93ff54f811564bcbd0332131f859c796`  
**Primary development agents:** Codex CLI / Claude Code / Grok Build / Gemini CLI  
**Final language target:** Rust for application/core/GUI; SQL/config files excluded from “programming language count”  
**GUI default:** native Rust `egui/eframe` family; Tauri/Web UI is fallback, not default  
**Migration method:** staged replacement, **not** permanent Go+Rust hybrid and **not** one-shot big-bang rewrite

---

# 0. Final architectural decision

The final Devorch should converge to:

```text
                    DEVORCH
                       │
          ┌────────────┴────────────┐
          │                         │
       Rust Core                Rust GUI
          │                     egui/eframe
          │                         │
          └────────────┬────────────┘
                       │
                Agent Runtime
       ┌────────┬────────┬────────┬────────┐
       │        │        │        │        │
     Codex    Claude    Grok    Gemini   Generic
       │        │        │        │
   JSON/App   JSON      ACP/     JSON/
    Server    Stream    JSON     A2A
       └────────┴────────┴────────┴────────┘
                       │
                  Workspaces
                       │
                Git Worktrees
```

This is intentionally different from the previous plan:

```text
Go daemon + React + Tauri
```

The new target is:

```text
Rust control plane + Rust native GUI
```

The reason is not “Rust is always faster than Go.”

The reason is that Devorch’s final product requires all of the following in one product:

- subprocess supervision;
- PTY;
- high-volume structured event streaming;
- SQLite;
- native macOS/Windows/Linux APIs;
- screen capture;
- input control;
- local policy engine;
- browser/runtime control;
- GUI;
- low idle resource use;
- deterministic cleanup;
- long-running daemon behavior.

Rust gives one ownership/lifetime model across all of these layers and avoids a future Go↔Rust native bridge.

---

# 1. Current repository reality

The current repository is valuable and is **not discarded as design knowledge**.

Verified current characteristics include:

```text
Go module: devorch
Go version declared: 1.24.0
SQLite: modernc.org/sqlite
PTY: github.com/creack/pty
WebSocket: github.com/gorilla/websocket
TUI: Bubble Tea
Existing daemon: cmd/devorchd
Existing CLI: cmd/devorch
Existing static Web UI: cmd/devorchd/static/index.html
Existing OkAON / router / learning / quality / provider / agent packages
```

The existing README describes approximately:

```text
248 Go source files
22 SQL migrations
multi-provider routing
Thompson Sampling
UCB
reward calculation
fallback
quality
experiments
```

These are **behavioral assets** that must be preserved by parity tests.

They are not necessarily code that must remain Go forever.

---

# 2. Why the Rust decision is now justified

## 2.1 The current PTY layer exposes a future portability problem

The current Go PTY implementation directly uses concepts such as:

```text
/bin/zsh
syscall.SIGHUP
github.com/creack/pty
```

This is a valid Unix implementation but is not a single cross-platform process layer for Windows/macOS/Linux.

Devorch 3 needs a process/PTY abstraction designed for all target platforms from day one.

## 2.2 Two directly relevant modern coding-agent runtimes are already Rust

Current open-source Codex contains:

```text
codex-rs/
  core/
  cli/
  exec/
  app-server/
  exec-server/
  execpolicy/
  ...
```

Grok Build publishes a Rust crate architecture including:

```text
xai-grok-pager
xai-grok-shell
xai-grok-tools
xai-grok-workspace
sandbox/config/MCP crates
```

This is strong evidence that Rust is appropriate for a terminal-agent runtime with process control, sandboxing, event protocols and low-level platform responsibilities.

## 2.3 The current Go repository has incomplete areas

The current source itself contains unfinished abstractions such as a quality evaluator `ErrNotImplemented`, and the large unified CLI exposes signs of accumulated integration debt.

That changes the rewrite calculus.

If the Go product were fully integrated and battle-tested, a rewrite would be hard to justify.

Here, Devorch is still early enough that **porting verified behavior into a cleaner Rust core can be cheaper than adding a GUI, four agent adapters and native computer control on top of the existing integration debt.**

---

# 3. Decision matrix

The architecture decision is:

| Option | Final one-language | Runtime/native fit | Time to first result | Migration safety | Long-term fit | Decision |
|---|---:|---:|---:|---:|---:|---|
| Keep Go + Tauri/React | Low | Medium | Highest | Highest | Medium | Reject as final architecture |
| Rust core + Tauri/React | Medium | High | High | Medium | High | Valid fallback |
| Big-bang Rust + native GUI | Highest | Highest | Low | Low | Highest | Reject |
| **Staged Rust replacement + native Rust GUI** | **Highest** | **Highest** | **Medium/High** | **High** | **Highest** | **SELECTED** |

Important:

> “Staged” is a migration method, not a permanent hybrid architecture.

The final product must not retain Go simply because migration was staged.

---

# 4. Critical performance truth

Moving the daemon from Go to Rust will **not by itself** make the entire product dramatically lighter if four AI CLIs and Chromium are permanently running.

The largest process costs will often be:

```text
Gemini CLI / Node runtime
Claude process
Grok process
Codex process
Chromium
local LLMs if enabled
```

Therefore the biggest resource optimization is architectural:

```text
Do not keep every agent alive.
```

Default runtime:

```text
low-risk task     -> 1 agent
normal task       -> 2 candidate agents
high-risk task    -> 2 at a time, up to 4 candidates
explicit compare  -> all 4 if requested
```

Default:

```text
max_parallel_agents = 2
```

The router may increase it after checking:

```text
RAM
CPU
battery
provider quota
task importance
budget
historical confidence
```

This saves more memory than micro-optimizing the orchestrator.

---

# 5. Agent execution modes

Every adapter supports a common concept:

```rust
pub enum AgentRuntimeMode {
    OneShotStructured,
    PersistentProtocol,
    InteractivePty,
}
```

Priority:

```text
1. OneShotStructured  ← default, low idle memory
2. PersistentProtocol ← when repeated turns justify keeping a process alive
3. InteractivePty     ← compatibility/fallback
```

Do not default to PTY scraping when a structured protocol exists.

---

# 6. Four first-class agents

All four are equal first-class executors.

```text
codex
claude
grok
gemini
```

No provider is hardcoded as “best.”

OkAON learns that later.

---

# 7. Gemini CLI adapter

Verified capabilities from the current public repository:

```text
gemini -p "..." --output-format json
gemini -p "..." --output-format stream-json
```

Current stream event family includes:

```text
init
message
tool_use
tool_result
error
result
```

The repository also contains an **experimental A2A server**.

## Default

```text
OneShotStructured
→ gemini -p ... --output-format stream-json
```

## Optional persistent mode

```text
A2A server
```

Because A2A is marked experimental, it cannot be the only Gemini integration.

Adapter must always have stream-json fallback.

---

# 8. Codex adapter

Current Codex is itself heavily Rust-based.

Verified `codex exec` support includes:

```text
--json
resume
fork
review
```

JSONL events currently include families such as:

```text
thread.started
turn.started
turn.completed
turn.failed
item.started
item.updated
item.completed
error
```

Item payloads include:

```text
agent_message
reasoning
command_execution
file_change
mcp_tool_call
collab_tool_call
web_search
todo_list
error
```

## Default

```text
codex exec --json ...
```

## Persistent mode

Investigate `app-server` protocol and use it only behind a versioned adapter.

Do not couple Devorch domain objects directly to Codex app-server schema.

---

# 9. Grok Build adapter

Current Grok Build is a Rust agent runtime.

Verified surfaces:

```text
grok -p "..."
--output-format json
--output-format streaming-json
--output-format streaming-messages-json
grok agent stdio
```

Current streaming-json includes events such as:

```text
text
thought
tool_call
tool_call_update
usage
plan
available_commands
end
error
```

Grok exposes explicit permission allow/deny controls and an ACP integration surface.

## Default

```text
grok -p ... --output-format streaming-json
```

## Persistent mode

```text
grok agent stdio
```

Do not use always-approve/yolo as the product permission model.

---

# 10. Claude Code adapter

Claude Code remains first-class.

Because CLI flags and output details may evolve independently of this repository, Claude adapter must use **runtime capability discovery**.

At install/runtime:

```text
claude --version
claude --help
```

Record supported:

```text
headless mode
output formats
session resume
permission modes
allowed/disallowed tools
MCP/SDK availability
```

Default integration should use structured headless output when supported.

PTY is fallback.

Never depend on undocumented parser assumptions.

---

# 11. Runtime capability discovery

Create:

```text
devorch agent inspect
```

Output:

```json
{
  "id": "gemini",
  "installed": true,
  "version": "...",
  "runtime_modes": [
    "one_shot_structured",
    "persistent_protocol",
    "interactive_pty"
  ],
  "structured_formats": ["stream-json"],
  "resume": true,
  "tool_permissions": true,
  "protocols": ["a2a"],
  "authenticated": true
}
```

The adapter selects from detected capability, not version-number guesses.

---

# 12. Canonical normalized agent events

Rust protocol crate:

```rust
pub enum AgentEvent {
    SessionStarted(SessionStarted),
    TurnStarted(TurnStarted),
    TextDelta(TextDelta),
    PlanUpdated(PlanUpdated),

    ToolStarted(ToolStarted),
    ToolUpdated(ToolUpdated),
    ToolCompleted(ToolCompleted),

    FileChanged(FileChanged),
    CommandStarted(CommandStarted),
    CommandCompleted(CommandCompleted),

    PermissionRequired(PermissionRequired),
    Usage(Usage),

    TurnCompleted(TurnCompleted),
    Completed(Completed),
    Failed(Failed),
}
```

Each event also keeps optional original vendor payload:

```rust
pub struct EventEnvelope {
    pub id: EventId,
    pub seq: u64,
    pub timestamp: DateTime<Utc>,

    pub mission_id: Option<MissionId>,
    pub task_id: Option<TaskId>,
    pub workspace_id: Option<WorkspaceId>,
    pub session_id: SessionId,

    pub event: AgentEvent,
    pub vendor_payload: Option<serde_json::Value>,
}
```

---

# 13. Rust workspace target

Final repository:

```text
devorch/
├── Cargo.toml
├── Cargo.lock
├── rust-toolchain.toml
│
├── crates/
│   ├── devorch-protocol/
│   ├── devorch-core/
│   ├── devorch-config/
│   ├── devorch-store/
│   ├── devorch-events/
│   ├── devorch-process/
│   ├── devorch-pty/
│   ├── devorch-git/
│   ├── devorch-workspace/
│   │
│   ├── devorch-agent/
│   ├── devorch-agent-codex/
│   ├── devorch-agent-claude/
│   ├── devorch-agent-grok/
│   ├── devorch-agent-gemini/
│   │
│   ├── devorch-mission/
│   ├── devorch-router/
│   ├── devorch-okaon/
│   ├── devorch-quality/
│   ├── devorch-policy/
│   ├── devorch-verify/
│   │
│   ├── devorch-http/
│   └── devorch-ui/
│
├── bins/
│   ├── devorch/
│   └── devorchd/
│
├── migrations/
├── testdata/
├── docs/
└── xtask/
```

Computer/browser crates are introduced by Document 2.

---

# 14. Dependency philosophy

Do not chase minimum dependency count at the expense of correctness.

Target classes:

```text
tokio          async process/network scheduler
serde          protocol serialization
serde_json
thiserror
tracing
rusqlite       SQLite, simple and controlled
axum           local HTTP/API if retained
portable-pty   cross-platform PTY candidate
notify         filesystem observation where needed
```

Before adding any crate:

```text
license
maintenance
platform support
dependency tree
unsafe surface
binary impact
```

must be reviewed.

Versions are pinned in the real implementation after Wave R0.

This document intentionally does not invent future crate version numbers.

---

# 15. SQLite migration policy

Do **not** throw away existing OkAON/benchmark data by default.

Current SQL migrations become migration input.

Create a compatibility audit:

```text
docs/rust-migration/sql-schema-map.md
```

For every existing table:

```text
KEEP
RENAME
MIGRATE
DEPRECATE
DROP_AFTER_BACKUP
```

Rust uses the same SQLite database during parity testing where safe.

Backup before first Rust migration.

---

# 16. No line-by-line Go translation

Bad migration:

```text
Go file -> equivalent Rust file
```

Correct migration:

```text
existing behavior
→ characterization test
→ Rust domain contract
→ Rust implementation
→ parity
→ switch
→ delete old Go subsystem
```

This removes historical integration debt instead of reproducing it.

---

# 17. Migration sequence

## R0 — Measure and freeze

No product feature work.

Tasks:

```text
capture Go baseline
capture DB schema
capture CLI outputs
capture daemon endpoints
capture core routing behavior
capture existing tests
capture known stubs/failures
```

On a machine with Go 1.24:

```bash
go test ./...
go vet ./...
go build ./cmd/devorch
go build ./cmd/devorchd
```

Measure, do not guess:

```text
binary size
daemon idle RSS
startup time
basic request p50/p95
```

Save:

```text
docs/rust-migration/baseline.json
```

---

# 18. R1 — Rust workspace skeleton

One agent only modifies root:

```text
Cargo.toml
Cargo.lock
rust-toolchain.toml
```

This avoids four-agent root workspace conflicts.

Initial crates:

```text
protocol
store
events
git
workspace
process
agent
```

Gate:

```bash
cargo fmt --all -- --check
cargo check --workspace
cargo test --workspace
cargo clippy --workspace --all-targets -- -D warnings
```

---

# 19. R2 — Vertical slice parity

Do not port all 248 Go files.

Implement one vertical slice:

```text
config
→ SQLite
→ workspace
→ worktree
→ event
→ CLI command
```

Acceptance:

```text
devorch workspace create
devorch workspace list
devorch workspace remove
```

with real Git worktrees.

Only after this works should four agent adapters begin.

---

# 20. R3 — Four agent adapters

Create four independent crates.

```text
devorch-agent-codex
devorch-agent-claude
devorch-agent-grok
devorch-agent-gemini
```

Cross-implementation assignment is preferred:

```text
Claude develops Codex adapter
Codex develops Grok adapter
Grok develops Gemini adapter
Gemini develops Claude adapter
```

Reason:

> an agent must not be the sole author and sole validator of its own integration.

Each adapter includes recorded fixtures.

No paid/online call required in normal CI.

---

# 21. Agent fixture tree

```text
testdata/agents/
  codex/
    init.jsonl
    edit.jsonl
    tool.jsonl
    failure.jsonl
    completed.jsonl
  claude/
  grok/
  gemini/
```

Every live smoke session can optionally update a candidate fixture, but fixture changes require review.

---

# 22. R4 — Headless four-agent mission

Before GUI:

```text
Mission
  ↓
2 or 4 worktrees
  ↓
Agent sessions
  ↓
Normalized events
  ↓
Test/verify
  ↓
Compare
  ↓
Winner
  ↓
Merge
```

CLI acceptance command example:

```bash
devorch mission run \
  --repo . \
  --goal "Fix seeded authentication regression" \
  --agents codex,claude,grok,gemini \
  --max-parallel 2
```

---

# 23. Generated/untracked file gate

Development simulation exposed this as a real issue.

**Never evaluate candidates with only:**

```bash
git diff
```

because untracked files are omitted.

Candidate inventory must include:

```bash
git status --porcelain=v1
git diff --numstat
git diff --cached --numstat
git ls-files --others --exclude-standard
```

Before merge reject unexpected:

```text
binary files
build outputs
credentials
cache files
temporary logs
vendor directories
```

unless ticket explicitly owns them.

---

# 24. Candidate change inventory

Canonical model:

```rust
pub struct ChangeInventory {
    pub tracked_modified: Vec<PathBuf>,
    pub staged: Vec<PathBuf>,
    pub untracked: Vec<PathBuf>,
    pub deleted: Vec<PathBuf>,
    pub binary: Vec<PathBuf>,
    pub generated: Vec<PathBuf>,
}
```

Candidate evaluation uses all categories.

---

# 25. R5 — Native Rust GUI

## Default choice: egui/eframe

Why:

```text
Rust-only application code
no Node frontend build
no React dependency
no WebView required
good dynamic dashboard fit
cross-platform
direct process/native integration
```

Devorch is not attempting to become a full text-editor replacement.

Therefore losing Monaco/React ecosystem convenience is acceptable.

The core product is:

```text
Mission Control
Agent Control
Diff/Review
Terminal/Event view
Computer view
Policy/Approvals
```

These map well to a native panel UI.

---

# 26. GUI dependency rule

UI is a client of core state.

Bad:

```text
egui click
→ directly mutate scheduler
```

Correct:

```text
egui
→ Command
→ Core
→ Event
→ UI state
```

This guarantees:

```text
CLI
GUI
future mobile/remote
```

share the same domain behavior.

---

# 27. GUI layout

```text
┌────────────────┬────────────────────────────────────┬────────────────┐
│ Missions       │ Main workspace                     │ Inspector      │
│ Workspaces     │                                    │                │
│ Agents         │ Mission DAG / Agent panes          │ Router         │
│ History        │ Terminal / Diff / Event            │ Cost           │
│ Automations    │                                    │ Policy         │
│ Settings       │                                    │ Verification   │
└────────────────┴────────────────────────────────────┴────────────────┘
```

---

# 28. Main screens

## Mission Control

Display:

```text
goal
status
task DAG
active executors
budget
current gate
blockers
verification
```

## Agent Board

Four potential cards:

```text
Codex
Claude
Grok
Gemini
```

A card can be:

```text
not selected
queued
starting
running
waiting permission
verifying
completed
failed
cancelled
```

## Workspace

```text
Agent
Events
Terminal
Files
Diff
Tests
Metrics
```

## Compare

Show deterministic evidence first.

```text
tests
build
lint
changed/untracked files
churn
risk
cost if available
latency
interventions
```

## Router

Show why an agent was selected without exposing private model chain-of-thought.

```text
historical success
task similarity
estimated cost
latency
confidence
fallback
```

---

# 29. Terminal strategy

Do not build a full terminal emulator before the structured event UI.

Priority:

```text
1. normalized agent stream
2. command output panel
3. interactive PTY
4. full ANSI fidelity
```

PTY is provided by the Rust process layer.

Terminal UI can use a VT parser and egui rendering.

It is not the source of truth for agent state.

---

# 30. Remote/browser preview

Do not embed a browser engine into the GUI.

For initial Devorch:

```text
open/attach dedicated visible Chromium
show browser state/events in Devorch
```

This saves implementation and memory complexity.

Browser control belongs to Document 2.

---

# 31. Four-agent developer instruction system

Four products use different project instruction conventions.

Create one canonical source:

```text
docs/agent-rules.md
```

Generate:

```text
AGENTS.md    -> Codex + Grok
CLAUDE.md    -> Claude
GEMINI.md    -> Gemini
```

Use Rust `xtask`:

```bash
cargo xtask sync-agent-rules
cargo xtask verify-agent-rules
```

Do not maintain four independent copies by hand.

---

# 32. Canonical developer rules

Every coding agent receives:

```text
1. Never edit main.
2. Only work in assigned worktree.
3. Only modify owned paths.
4. Never delete tests to pass a gate.
5. Never silence a failing command.
6. Do not introduce a second implementation of an existing crate.
7. Do not change root Cargo files unless assigned.
8. No broad dependency upgrade.
9. No license-incompatible copying.
10. Report changed + untracked files.
11. Run ticket gate.
12. Return commit SHA.
```

---

# 33. Four-agent development topology

Do not assume “4 agents available” means “4 agents always coding.”

Use:

```text
3 Builders + 1 Reviewer/Integrator
```

or:

```text
2 Builders + 2 Reviewers
```

for shared-core/security work.

Use four builders only when file ownership is completely disjoint.

---

# 34. Root-file lock

Only the Integrator may modify during a wave:

```text
Cargo.toml
Cargo.lock
rust-toolchain.toml
migrations registry
shared protocol breaking changes
```

If a builder needs a dependency:

```text
builder records dependency request
→ integrator adds it
→ builder rebases
```

This prevents merge churn.

---

# 35. Ticket packet

```markdown
# Ticket
DEV3-ADE-...

## Base SHA
...

## Goal
...

## Owned paths
...

## Read-only dependencies
...

## Forbidden paths
...

## Required interfaces
...

## Tests
...

## Commands
...

## Non-goals
...

## Completion report
- changed files
- untracked files
- test outputs
- risk
- commit SHA
```

---

# 36. Development Waves

## Wave R0 — four-way audit

### Codex
Rust workspace/CI spike.

### Claude
Domain and existing Go behavior map.

### Grok
Cross-platform process/PTY/runtime spike.

### Gemini
Repository-wide package/schema/test inventory.

No overlapping implementation.

**Gate R0**

Produce a single migration decision record.

---

# 37. Wave R1 — core crates

### Codex
`devorch-protocol`

### Claude
`devorch-store`

### Grok
`devorch-process` / `devorch-pty`

### Gemini
`devorch-git` / `devorch-workspace`

Integrator alone updates workspace root.

**Gate R1**

Real worktree lifecycle + persistence tests pass.

---

# 38. Wave R2 — agent adapters

Cross assignment:

```text
Claude -> Codex adapter
Codex  -> Grok adapter
Grok   -> Gemini adapter
Gemini -> Claude adapter
```

Two non-author adapters review each protocol mapping.

**Gate R2**

Recorded streams normalize to the same event contract.

---

# 39. Wave R3 — mission execution

### Claude
Mission/task graph.

### Codex
Scheduler/process execution.

### Grok
Verification command runner.

### Gemini
Candidate inventory/evaluator.

**Gate R3**

Four fake/fixture agents complete one persisted mission.

---

# 40. Wave R4 — real agent smoke

Run a harmless seeded repository.

Required:

```text
Codex live smoke
Claude live smoke
Grok live smoke
Gemini live smoke
```

Each in a separate worktree.

Never use production repository for the first smoke.

**Gate R4**

All four can independently:

```text
start
edit only worktree
emit events
finish
be verified
cleanup
```

---

# 41. Wave R5 — compare/winner/merge

Add:

```text
deterministic candidate gate
complete change inventory
winner selection
integration branch merge
re-run gate after merge
```

The evaluator must include untracked files.

**Gate R5**

One deliberately wrong agent result is rejected even if it says “completed.”

---

# 42. Wave R6 — native GUI

One agent owns GUI shell.

Other agents build backend features/review while GUI shell is active.

Do not have four agents concurrently modify:

```text
devorch-ui/src/app.rs
```

Split UI modules first.

---

# 43. Wave R7 — retire Go

Before deletion:

```text
all selected Go behavior has parity test
SQLite migration verified
Rust CLI/daemon/GUI acceptance pass
backup/restore verified
```

Then remove Go runtime from the production branch.

Keep git history/tag:

```text
devorch-go-final
```

Final repository is Rust-first.

---

# 44. CI

Rust required gates:

```bash
cargo fmt --all -- --check
cargo check --workspace
cargo test --workspace
cargo clippy --workspace --all-targets -- -D warnings
```

Additional:

```text
license
dependency audit
adapter fixture tests
worktree integration tests
SQLite migration tests
UI compile
```

OS matrix later:

```text
Linux
macOS
Windows
```

---

# 45. Live AI tests

Default CI does not require subscriptions.

Live tests opt-in:

```text
DEVORCH_LIVE_CODEX=1
DEVORCH_LIVE_CLAUDE=1
DEVORCH_LIVE_GROK=1
DEVORCH_LIVE_GEMINI=1
```

Never fail ordinary contributor CI because a provider quota is unavailable.

---

# 46. Runtime process policy

Default state:

```text
0 idle AI agent processes
```

A mission creates processes as required.

After completion:

```text
one-shot process exits
persistent session is kept only if policy says keep-alive
```

Config:

```toml
[runtime]
max_parallel_agents = 2
idle_session_ttl_seconds = 300
persistent_protocols = "auto"
```

---

# 47. Adaptive candidate policy

Initial policy before OkAON has enough data:

```text
LOW risk
  1 candidate

NORMAL
  2 candidates

HIGH
  up to 4 candidates
  max 2 running concurrently by default

EXPLICIT_COMPARE_ALL
  4 candidates
```

After learning:

```text
confidence >= threshold
→ fewer candidates

low confidence / regression-sensitive
→ more candidates
```

This turns model diversity into an efficiency tool instead of a permanent resource cost.

---

# 48. Candidate scoring

Never use agent self-reported confidence as the main score.

Example:

```text
hard gate:
  build
  tests
  lint/security required by repo

then:
  change scope
  untracked files
  churn
  regression risk
  latency
  cost
  human intervention

optional:
  LLM review
```

A failing deterministic gate cannot win because an LLM likes its answer.

---

# 49. Open-source absorption after Rust decision

Priority changes.

## Tier A — Rust implementation references

### OpenAI Codex
Use for:

```text
Rust workspace patterns
JSONL exec event modeling
app-server separation
exec-policy patterns
process/sandbox concepts
MCP boundaries
```

Apache-2.0 obligations apply to copied code.

### Grok Build
Use for:

```text
Rust agent runtime
headless event projection
ACP
workspace/tool boundaries
sandbox/config patterns
permission mapping
```

Apache-2.0 obligations apply.

## Tier B — protocol/behavior references

### Gemini CLI
Use:

```text
stream-json schema
A2A server concept
session behavior
sandbox integration ideas
```

### Claude Code
Reference public behavior/docs.

Do not copy code from the Claude Code repository into Devorch; current repository license is not an open-source redistribution license.

## Tier C — ADE UX

```text
Orca
Emdash
Agent Deck
OpenHands
Superset(reference only depending on license)
Opcode(reference only depending on license)
```

After selecting native Rust GUI, borrow UX patterns rather than frontend stacks.

---

# 50. Third-party source manifest

Required:

```text
THIRD_PARTY_NOTICES.md
docs/upstream-sources.toml
```

Example:

```toml
[[source]]
name = "openai-codex"
repo = "https://github.com/openai/codex"
commit = "<exact>"
license = "Apache-2.0"
usage = "PORT"
source_path = "codex-rs/exec/..."
target_crate = "devorch-agent-codex"
```

No upstream code without exact source commit.

---

# 51. Definition of Done — Document 1

The ADE/Rust migration is complete only when:

```text
1. Production runtime is Rust.
2. GUI production code is Rust.
3. Go runtime is no longer required.
4. Four CLI agents are detected independently.
5. Four adapter fixture suites pass.
6. Live smoke for all four passes on developer machine.
7. Worktrees are isolated.
8. Untracked/generated files are inventoried.
9. Deterministic evaluation rejects bad candidates.
10. Winner merges and full repo gate reruns.
11. Mission survives daemon restart.
12. No AI agent process remains idle without policy.
13. macOS/Linux/Windows builds compile in CI.
14. Existing OkAON-compatible data is preserved or explicitly migrated.
```

---

# 52. First actual command to the development agents

Do not tell them to implement this entire document.

Begin with R0.

Canonical prompt:

```text
Read docs/agent-rules.md and your generated vendor-specific rules.

We are evaluating a staged migration of Devorch from the current Go baseline
to a final Rust-only control plane and native Rust GUI.

Do not implement product features.

Execute only your assigned Wave R0 audit/spike.
Do not modify files owned by another agent.
Do not modify root dependency files unless you are the assigned integrator.
Record every command and result.
Do not claim a successful build without command output.
Return changed files, untracked files, risks and commit SHA.
```

The migration starts only after R0 evidence is reviewed.
