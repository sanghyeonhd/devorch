# Devorch 3 — Rust Agent Operating Layer / Autonomous Computer Runtime Plan

**Version:** 3.0 — Rust-first, simulation-informed  
**Depends on:** `DEVORCH_3_RUST_MULTI_AGENT_ADE_IMPLEMENTATION.md`  
**Primary executors available:** Codex / Claude / Grok / Gemini  
**Final runtime language:** Rust  
**Safety principle:** structured API → semantic target → policy → action → verification  
**Native computer control is late-stage, never the first milestone**

---

# 0. Final product

Devorch becomes:

> A Rust-native local Agent Operating Layer that dynamically selects and coordinates AI coding agents and, under explicit policy, can operate the visible computer.

It is not a kernel.

It is installed on:

```text
macOS
Windows
Linux
```

Architecture:

```text
                         User Goal
                            │
                            ▼
                       Rust Mission
                            │
                            ▼
                        Task DAG
                            │
             ┌──────────────┴──────────────┐
             │                             │
         Agent Tasks                  Computer Tasks
             │                             │
   Codex/Claude/Grok/Gemini       Browser / App / OS
             │                             │
             └──────────────┬──────────────┘
                            ▼
                       Verification
                            │
                     Outcome / Failure
                            │
                     OkAON Learning
```

---

# 1. Prerequisite

Do not start native control because “the mouse API is easy.”

Required first:

```text
Rust core
Mission persistence
Task scheduler
Agent adapters
Workspace isolation
Policy engine
Verification engine
Event/audit journal
Emergency stop
```

Browser autonomy comes before native OS autonomy.

---

# 2. Crate expansion

After Document 1:

```text
crates/
  devorch-mission/
  devorch-scheduler/
  devorch-router/
  devorch-okaon/
  devorch-policy/
  devorch-verify/
  devorch-browser/
  devorch-computer/
  devorch-computer-macos/
  devorch-computer-windows/
  devorch-computer-linux/
  devorch-secrets/
  devorch-skills/
```

Platform crates depend on platform-neutral contracts.

The core must never import platform-specific native types.

---

# 3. Mission state model

```rust
pub struct Mission {
    pub id: MissionId,
    pub goal: String,
    pub status: MissionStatus,
    pub policy_id: PolicyId,
    pub budget: MissionBudget,
    pub repo: Option<RepositoryContext>,
    pub created_at: DateTime<Utc>,
    pub started_at: Option<DateTime<Utc>>,
    pub completed_at: Option<DateTime<Utc>>,
}
```

States:

```text
CREATED
PLANNING
RUNNING
WAITING_PERMISSION
PAUSED
PAUSED_EMERGENCY
VERIFYING
SUCCEEDED
FAILED
CANCELLED
```

---

# 4. Persistent task DAG

```rust
pub struct TaskNode {
    pub id: TaskId,
    pub mission_id: MissionId,
    pub kind: TaskKind,
    pub goal: String,
    pub state: TaskState,

    pub dependencies: Vec<TaskId>,
    pub executor: Option<ExecutorSelection>,
    pub workspace_id: Option<WorkspaceId>,

    pub retry_policy: RetryPolicy,
    pub verification: VerificationPolicy,
}
```

Task states:

```text
PENDING
READY
LEASED
RUNNING
WAITING_PERMISSION
WAITING_EXTERNAL
VERIFYING
SUCCEEDED
FAILED_RETRYABLE
FAILED_FINAL
CANCELLED
SKIPPED
```

---

# 5. Lease-based execution

A scheduler must survive daemon restarts.

Persist:

```text
lease_owner
lease_expires_at
heartbeat_at
attempt
```

Restart:

```text
load active missions
→ find stale leases
→ inspect external process/session
→ reconnect if provable
→ otherwise classify unknown
→ re-verify
→ retry/recover
```

Never convert “process disappeared” directly to success.

---

# 6. Mission event journal

Persist low-volume domain events:

```text
MISSION_CREATED
TASK_READY
EXECUTOR_SELECTED
TASK_STARTED
POLICY_DECISION
TASK_VERIFYING
TASK_SUCCEEDED
TASK_FAILED
RECOVERY_STARTED
MISSION_SUCCEEDED
...
```

Do not persist every terminal byte as a durable row.

High-volume:

```text
terminal chunks
screenshots
raw model stream
```

use bounded file/blob storage with retention policy.

---

# 7. Executor abstraction

```rust
#[async_trait]
pub trait Executor {
    fn kind(&self) -> ExecutorKind;
    fn can_execute(&self, task: &TaskNode) -> bool;

    async fn start(&self, req: ExecuteRequest)
        -> Result<ExecutionHandle, ExecutorError>;

    async fn poll(&self, handle: &ExecutionHandle)
        -> Result<ExecutionStatus, ExecutorError>;

    async fn cancel(&self, handle: &ExecutionHandle)
        -> Result<(), ExecutorError>;
}
```

Implement:

```text
AgentExecutor
ShellExecutor
BrowserExecutor
ComputerExecutor
HumanExecutor
```

---

# 8. Four-agent router

Candidate executors:

```text
Codex
Claude
Grok
Gemini
```

Initial bootstrap should not assume a fixed rank.

OkAON learns:

```text
task type
repo fingerprint
language
framework
OS
agent
model
runtime mode
tool configuration
success
test result
latency
cost when known
interventions
```

---

# 9. Hierarchical selection

Avoid combinatorial arm explosion.

Select:

```text
1. executor kind
2. agent
3. model/profile
4. runtime mode
5. optional strategy
```

Example:

```text
coding
→ Codex
→ installed/default model
→ OneShotStructured
→ local worktree
```

---

# 10. Adaptive multi-agent execution

The router decides candidate count.

```text
confidence high
→ one agent

confidence medium
→ top two

confidence low / high-impact
→ up to four
```

Max parallelism defaults to 2.

This is a core efficiency feature.

---

# 11. Raw outcome first, reward later

Store raw evidence:

```text
task_success
build
tests
quality
latency
tokens
reported cost
retry count
intervention count
security risk
regression result
```

Then calculate versioned reward.

```rust
pub struct RewardDefinition {
    pub version: String,
    pub weights: BTreeMap<MetricName, f64>,
}
```

Historical data always records reward version.

---

# 12. Verification is mandatory

Agent text is not proof.

Priority:

```text
1. deterministic program state
2. process exit status
3. test/build output
4. filesystem/database state
5. browser DOM/network state
6. accessibility state
7. visual evidence
8. LLM judge
```

LLM judge is supplementary.

---

# 13. Change verification

For coding tasks inspect:

```text
tracked diff
staged diff
untracked files
deleted files
binary files
generated files
test outputs
build outputs
```

The simulation proved that `git diff` alone is insufficient.

---

# 14. Fake backends first

Required:

```text
FakeAgent
FakeBrowser
FakeComputer
FakePolicyApprover
FakeClock
```

This allows deterministic autonomous-loop tests without real subscriptions or mouse control.

---

# 15. Browser before Computer

Order:

```text
FakeComputer
    ↓
Visible Chromium
    ↓
Browser autonomous mission
    ↓
macOS bounded native
    ↓
Windows bounded native
    ↓
Linux
```

Browser is easier to verify because DOM/network evidence exists.

---

# 16. Browser runtime

```rust
pub trait BrowserRuntime {
    async fn launch(&self, profile: BrowserProfile)
        -> Result<BrowserSession, BrowserError>;

    async fn snapshot(&self, session: &BrowserSession)
        -> Result<BrowserSnapshot, BrowserError>;

    async fn execute(&self, action: BrowserAction)
        -> Result<ActionEvidence, BrowserError>;

    async fn close(&self, session: BrowserSession)
        -> Result<(), BrowserError>;
}
```

Capabilities:

```text
visible Chromium
dedicated profile
tabs
navigation
DOM snapshot
click
fill
keyboard
download
screenshot
console
network events
```

---

# 17. No personal browser profile

Autonomous browser tests must use:

```text
~/.devorch/browser-profiles/<mission>
```

or temporary profiles.

Do not attach to the user's personal Chrome profile by default.

---

# 18. Semantic computer action

Planner output:

```rust
pub struct PlannedAction {
    pub kind: ActionKind,
    pub target: SemanticTarget,
    pub input: Option<ActionInput>,
}
```

Example:

```json
{
  "kind": "click",
  "target": {
    "role": "button",
    "name": "Run Tests"
  }
}
```

Never make screen coordinates the primary planning language.

---

# 19. Target resolution hierarchy

```text
1. application-native API
2. DOM/CDP
3. accessibility tree
4. window/process metadata
5. screen vision
6. OCR
7. coordinate fallback
```

Resolver output:

```rust
pub struct ResolvedTarget {
    pub method: ResolutionMethod,
    pub native_id: Option<String>,
    pub bounds: Option<Rect>,
    pub point: Option<Point>,
    pub confidence: f32,
    pub evidence: Vec<EvidenceRef>,
}
```

---

# 20. Computer contract

```rust
#[async_trait]
pub trait Computer {
    async fn snapshot(&self) -> Result<ComputerSnapshot, ComputerError>;

    async fn execute(
        &self,
        action: ResolvedComputerAction,
    ) -> Result<ActionEvidence, ComputerError>;
}
```

Separate:

```text
Planner
Resolver
Policy
Executor
Verifier
```

Never combine them into a single `click()` tool exposed directly to the LLM.

---

# 21. Policy bridge

Do not create a completely separate permission world.

Port the useful semantics from existing Devorch permission/authz behavior into:

```text
devorch-policy
```

Canonical decisions:

```text
ALLOW
ALLOW_SCOPE
ALLOW_ONCE
ASK
ASK_STRONG
DENY
```

Risk:

```text
L0 OBSERVE
L1 REVERSIBLE_LOCAL
L2 LOCAL_MODIFICATION
L3 EXTERNAL_SIDE_EFFECT
L4 SENSITIVE
L5 DESTRUCTIVE
```

---

# 22. Default policy

Example default:

```text
L0 allow
L1 allow inside mission scope
L2 allow inside owned worktree
L3 ask
L4 ask_strong
L5 deny
```

Examples:

```text
read code                  L0/L1
edit worktree              L2
run tests                  L2
git commit worktree        L2
git push                   L3
send message               L3
credential changes         L4
delete personal directory  L5
financial transaction      L5
```

---

# 23. Capability-based security

Bad:

```text
allow shell
```

Good:

```text
allow process.run("cargo test", workspace)
allow git.status(workspace)
ask git.push(repo)
deny fs.delete(outside_workspace)
```

Map vendor permissions to Devorch capability policy.

The provider never owns final security policy.

---

# 24. Emergency stop

Must exist before real native input.

Paths:

```text
GUI button
global hotkey
CLI
local daemon endpoint
```

CLI:

```bash
devorch emergency-stop
```

Behavior:

```text
pause scheduler
cancel queued side effects
stop Devorch input injection
deny new external effects
mark missions PAUSED_EMERGENCY
emit audit event
```

Do not kill unrelated user applications.

---

# 25. Audit

Every side effect records:

```text
mission
task
agent/executor
planned action
resolved target
resolution evidence
policy
approval
before evidence
after evidence
verification
duration
failure class
```

Screenshots have retention/redaction rules.

---

# 26. Secret broker

Do not pass full parent environment to an agent.

```text
Agent process env
= minimal environment
+ explicitly scoped credentials
```

Native secret sources:

```text
macOS Keychain
Windows Credential Manager
Linux Secret Service
```

The broker releases only the credential required by the current capability.

---

# 27. Failure taxonomy

```text
AGENT_NOT_INSTALLED
AGENT_AUTH
AGENT_PROTOCOL
MODEL_RATE_LIMIT
AGENT_CRASH
PROCESS_HANG
TOOL_FAILURE

BUILD_FAILURE
TEST_FAILURE

WORKTREE_CONFLICT
UNTRACKED_FILE_POLICY
ENVIRONMENT_FAILURE

NETWORK_FAILURE
PERMISSION_DENIED
POLICY_BLOCKED

TARGET_NOT_FOUND
LOW_TARGET_CONFIDENCE
COMPUTER_ACTION_MISMATCH
VERIFICATION_FAILED

USER_ABORTED
BUDGET_EXCEEDED
```

Every failure has bounded recovery.

---

# 28. Recovery

```text
classify
→ safe same retry?
→ alternate tool?
→ alternate runtime mode?
→ alternate agent?
→ recreate workspace?
→ replan?
→ human handoff
```

Never infinite retry.

---

# 29. Mission budget

```rust
pub struct MissionBudget {
    pub max_duration: Duration,
    pub max_attempts_per_task: u32,
    pub max_agent_processes: usize,
    pub max_parallel_agents: usize,
    pub max_external_side_effects: u32,
    pub max_cost: Option<Money>,
}
```

Provider cost can be unknown.

Unknown cost is not zero.

---

# 30. Resource scheduler

Observe:

```text
CPU
RAM
battery
load
agent process count
Chromium process count
local model memory
```

Priority:

```text
verified mission success
without exhausting host
```

Not:

```text
run all four because they exist
```

---

# 31. Computer platform crates

## macOS

```text
devorch-computer-macos
```

Use:

```text
Accessibility APIs
ScreenCaptureKit
CGEvent
window/process APIs
clipboard
app launch
```

Runtime reports missing permissions as typed errors.

## Windows

```text
devorch-computer-windows
```

Use:

```text
UI Automation
Windows capture APIs
SendInput
window/process APIs
clipboard
```

No administrator requirement for ordinary operation.

## Linux

```text
devorch-computer-linux
```

Do not claim universal support.

Detect:

```text
X11
Wayland compositor
screen capture capability
input capability
accessibility capability
```

Return a capability matrix.

---

# 32. Platform capability API

```rust
pub struct ComputerCapabilities {
    pub observe_windows: bool,
    pub accessibility_tree: bool,
    pub screen_capture: bool,
    pub pointer_input: bool,
    pub keyboard_input: bool,
    pub clipboard: bool,
}
```

UI displays actual capability.

---

# 33. Native test environment

Never start on daily personal desktop.

Use:

```text
dedicated macOS test account
Windows VM
controlled Linux session
temporary browser profile
fixture applications
```

No real email/social/finance/production actions in default native tests.

---

# 34. Computer simulation tests

FakeComputer can simulate:

```text
target moves
dialog appears
button disappears
wrong window focused
resolution confidence drops
action fails
verification fails
permission denied
emergency stop
```

All control loop logic must pass FakeComputer before native APIs.

---

# 35. Browser fixture app

Create:

```text
testdata/browser-app/
```

Scenarios:

```text
login success
login failure
button moved
modal overlay
network failure
slow result
download
```

CI launches local HTTP fixture.

No internet required.

---

# 36. Native bounded acceptance

First native test:

```text
1. Open dedicated test editor.
2. Detect its window.
3. Resolve text area semantically/accessibility.
4. Type unique token.
5. Save only under temporary test directory.
6. Verify file contents directly.
7. Close test window.
8. Clean up.
9. Confirm audit records.
```

No arbitrary desktop mission yet.

---

# 37. Autonomous control loop

```text
OBSERVE
  ↓
PLAN
  ↓
SELECT EXECUTOR / TARGET
  ↓
POLICY
  ↓
ACT
  ↓
VERIFY
  ↓
SUCCESS? ─ yes → LEARN
  │
  no
  ↓
CLASSIFY
  ↓
RECOVER / REPLAN / HANDOFF
```

Each arrow is observable and testable.

---

# 38. Planner contract

Do not depend on one provider.

Planning can be performed by:

```text
Codex
Claude
Grok
Gemini
or future provider
```

The planner produces schema-constrained output.

Validate before execution.

Invalid plans are rejected, not “best effort parsed.”

---

# 39. Dual-agent planning for dangerous missions

For high-risk tasks:

```text
Planner A
→ plan

Planner B
→ critique

Devorch deterministic policy
→ final admissible graph
```

This is optional for coding but recommended for external side effects.

---

# 40. Human handoff

Handoff contains actionable state:

```text
what goal
what succeeded
what failed
current workspace
current app/window
last verification
recommended next action
```

Not just:

```text
"I need help."
```

---

# 41. Memory architecture

## Working

Current task facts.

## Episodic

Verified past mission events.

## Skill

Versioned reusable procedures.

Only verified outcomes should become trusted skill evidence.

---

# 42. Skill example

```yaml
id: rust-full-gate
version: 1

steps:
  - cargo fmt --all -- --check
  - cargo check --workspace
  - cargo test --workspace
  - cargo clippy --workspace --all-targets -- -D warnings

success:
  all_exit_codes: 0
```

Agent can propose a skill.

Human/review gate publishes it.

---

# 43. Self-improvement boundary

Automatic:

```text
routing statistics
candidate count
runtime mode selection
retry strategy ranking
tool ordering
skill recommendation
```

Human-reviewed only:

```text
policy source
emergency stop
secret broker
audit deletion
native privilege escalation
self-update trust
permission defaults
```

---

# 44. UI Operating Layer panels

Native Rust GUI adds:

## Computer View

```text
live snapshot
target overlay
resolution method
confidence
current policy
verification
```

## Action Timeline

```text
Observe
Resolve
Policy
Execute
Verify
Recover
```

## Autonomy

```text
Observe
Suggest
Assisted
Workspace
Browser
Computer
```

---

# 45. Autonomy levels

```text
0 OBSERVE
1 SUGGEST
2 ASSISTED
3 WORKSPACE
4 BROWSER
5 COMPUTER
```

Default stable release:

```text
2 or 3
```

Level 5 is explicit opt-in.

---

# 46. Rust GUI benefit for Operating Layer

A native Rust GUI allows:

```text
same Rust types
same event model
same policy types
direct native platform integration
no local HTTP requirement for every internal action
```

However GUI and daemon remain logically separated by commands/events.

This preserves testability.

---

# 47. Process topology

Recommended:

```text
devorchd    persistent control plane
devorch-ui  optional desktop UI
agent CLIs  on-demand children
Chromium    on-demand dedicated browser
```

All application binaries are Rust.

The separate UI process provides crash isolation.

---

# 48. Local transport

Prefer:

```text
Unix socket  macOS/Linux
Named Pipe   Windows
```

HTTP/WebSocket can remain for development/remote features, bound to loopback by default.

Computer-control API is never anonymously exposed on LAN.

---

# 49. Development agent allocation

## C0 — domain + fakes

```text
Claude -> Mission/Task DAG
Codex  -> Scheduler/lease
Grok   -> FakeComputer/action model
Gemini -> persistence/recovery test matrix
```

## C1 — policy + verify + browser

```text
Claude -> policy
Codex  -> verifier
Grok   -> browser runtime
Gemini -> browser fixture/e2e
```

## C2 — OkAON

```text
Claude -> reward domain
Codex  -> deterministic evaluator
Grok   -> executor selection
Gemini -> data/benchmark analysis
```

## C3 — native macOS

One shared interface owner only.

Platform-specific files split.

## C4 — native Windows

Rotate roles.

---

# 50. Security code review

The following requires two independent non-author reviews:

```text
policy
secret broker
input injection
emergency stop
external message action
filesystem delete
git push
audit redaction
self-update
```

Four models are available; use that redundancy.

---

# 51. Native merge gate

No native computer code merges if:

```text
FakeComputer equivalent test absent
policy path bypassed
verification absent
action uses raw coordinates from planner
secret logged
unbounded retry
requires permanent admin/root
```

---

# 52. Definition of Done — Agent Operating Layer

A final acceptance mission:

```text
"Open this seeded repository, fix the login issue,
verify it in a visible browser and prepare a PR."
```

Pass only if:

```text
1. Mission persists.
2. Task DAG is valid.
3. Router selects 1-4 agent candidates.
4. Worktrees are isolated.
5. Candidate changes include untracked inventory.
6. Deterministic tests reject bad candidates.
7. Winner merges.
8. Visible dedicated Chromium launches.
9. DOM/browser verification passes.
10. Native action, if needed, goes through semantic resolver.
11. Policy approves/asks/denies correctly.
12. Action evidence is captured.
13. Verification proves expected result.
14. PR action follows L3 policy.
15. CI is observed.
16. Outcome enters OkAON data.
17. Losing workspaces are safely removed.
18. Daemon restart preserves mission/audit state.
19. No unnecessary AI processes remain alive.
20. Emergency stop remains functional.
```

---

# 53. Implementation order

```text
Document 1
Rust migration + four-agent ADE
             ↓
C0 Mission DAG/Fakes
             ↓
C1 Policy/Verify/Browser
             ↓
C2 OkAON 2
             ↓
macOS bounded native
             ↓
Windows bounded native
             ↓
Linux capability expansion
             ↓
Autonomy Level 5
```

Do not reorder this into “mouse first.”

---

# 54. First implementation prompt for this document

```text
Implement only the assigned C0 ticket.

Read the Rust ADE document, agent rules, current Rust protocol/store crates,
and relevant legacy Go behavior.

No native mouse/keyboard code.
No browser automation.
No provider permission bypass.
No external side effects.

Use fakes and deterministic tests.
Only modify assigned paths.
Return changed/untracked files, test output, risks and commit SHA.
```

If an agent jumps ahead to native control, reject the patch.
