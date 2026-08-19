# Devorch 3 — Development Simulation & Architecture Validation Report

**Date:** 2026-08-19  
**Scope:** four-agent worktree workflow, deterministic candidate evaluation, event normalization, persistence/restart, policy, resource scheduling, Rust migration feasibility  
**Result:** **GO with conditions**  
**Primary architecture decision:** staged replacement to Rust + native Rust GUI

---

# 1. What was actually validated

This was not only a prose review.

A local synthetic Git repository was created and the proposed Devorch orchestration was executed using deterministic mock agents representing:

```text
Codex
Claude
Grok
Gemini
```

The simulation performed:

```text
seed failing repository
create four Git worktrees
apply four independent candidate patches
run the same test suite
collect tracked and untracked changes
reject failing candidate
rank passing candidates
merge winner into integration branch
rerun tests
persist mission/candidate state in SQLite
close/reopen SQLite to simulate daemon restart
normalize four vendor event-stream shapes
apply policy decisions
simulate adaptive agent concurrency
```

---

# 2. Initial simulation failure — important discovery

The first simulation failed during the winner merge.

Reason:

```text
Python tests generated __pycache__/ files.
Those untracked/generated files interfered with merge.
```

This demonstrates that a plan which only says:

```text
run tests
then merge
```

is insufficient.

The implementation plan was upgraded with an explicit generated/untracked artifact gate.

---

# 3. Second issue discovered

The next simulation revealed:

```text
git diff --numstat
```

does **not** include untracked files.

A candidate intentionally created an unnecessary `NOTES.md`.

If the evaluator only looked at normal diff stats, the extra file would be missed.

The upgraded plan now requires:

```bash
git status --porcelain=v1
git diff --numstat
git diff --cached --numstat
git ls-files --others --exclude-standard
```

before evaluation/merge.

---

# 4. Final four-worktree simulation

The final corrected simulation produced:

| Agent | Tests | Changed files | Untracked | Churn | Result |
|---|---:|---:|---|---:|---|
| Codex | PASS | 1 | none | 2 | winner |
| Claude | PASS | 1 | none | 3 | valid |
| Grok | FAIL | 1 | none | 2 | rejected |
| Gemini | PASS | 2 | `NOTES.md` | 4 | valid but larger scope |

The deterministic evaluator selected the smallest passing candidate.

Final integration tests:

```text
PASS
```

This validates the high-level:

```text
worktree
→ candidate
→ deterministic test
→ complete inventory
→ compare
→ winner
→ merge
→ full verification
```

flow.

---

# 5. Persistence/restart simulation

Mission result and four candidate results were written into SQLite.

The connection was closed and reopened.

After reload the simulation recovered:

```text
mission state = SUCCEEDED
winner = codex mock candidate
all four candidate records present
base/final commit identity preserved
```

Result:

```text
PASS
```

This validates the persistence shape, not the final production schema.

---

# 6. Adapter normalization simulation

Public/current event shapes were used where available.

## Gemini

Current public Gemini CLI stream schema contains:

```text
init
message
tool_use
tool_result
error
result
```

Simulation normalized to:

```text
SESSION_STARTED
TEXT
TOOL_STARTED
TOOL_COMPLETED
COMPLETED
```

## Codex

Current Codex JSONL event family contains:

```text
thread.started
turn.started
item.started
item.updated
item.completed
turn.completed
turn.failed
error
```

Simulation normalized these into Devorch events.

## Grok

Current Grok `streaming-json` contains:

```text
tool_call
tool_call_update
text
usage
end
error
...
```

Normalization succeeded.

## Claude

A fixture contract equivalent to Claude structured headless operation was normalized.

**Condition:** the production adapter must capture the exact installed Claude CLI stream during R0/R2 and version the fixture. The simulation does not claim a fixed forever-valid Claude wire schema.

Overall adapter abstraction:

```text
PASS at architecture/fixture level
```

---

# 7. Policy simulation

Actions:

```text
read_file
edit_worktree
git_push
delete_outside_worktree
```

Decisions:

```text
read_file               ALLOW
edit_worktree           ALLOW_SCOPE
git_push                ASK
delete_outside_worktree DENY
```

Result:

```text
PASS
```

This validates that an executor can be separated from Devorch's policy decision.

---

# 8. Resource scheduling simulation

The system must not keep all four CLIs permanently active.

Simulated bootstrap policy:

| Risk | Candidates | Max parallel | Peak agent processes |
|---|---:|---:|---:|
| Low | 1 | 1 | 1 |
| Normal | 2 | 2 | 2 |
| High | 4 | 2 | 2 |

High risk can still evaluate all four; it executes them in two waves by default.

This means:

```text
four models available
!=
four models permanently resident
```

This is critical to the “extreme efficiency” goal.

---

# 9. Rust migration validation

## Evidence supporting Rust

### Current Devorch

The Go PTY code currently contains Unix-specific assumptions such as:

```text
/bin/zsh
SIGHUP
creack/pty
```

A true common Windows/macOS/Linux Operating Layer requires new abstraction work regardless of language.

### Codex

Current open-source Codex contains a substantial:

```text
codex-rs
```

implementation including core/exec/app-server/exec-policy style components.

### Grok Build

Current Grok Build publishes its agent runtime and workspace/tool stack as Rust crates.

Therefore Rust is not a speculative language choice for this product category.

---

# 10. Why a big-bang rewrite was rejected

A big-bang rewrite creates:

```text
no parity checkpoint
large merge surface
hard regression localization
long period with no runnable product
```

The selected migration is:

```text
Go baseline frozen as behavioral reference
        ↓
Rust vertical slice
        ↓
parity gates
        ↓
Rust four-agent ADE
        ↓
native Rust GUI
        ↓
Go runtime retired
```

Final state remains one programming language.

---

# 11. Why keeping Go permanently was rejected

Keeping Go itself is technically valid.

However final Devorch also needs:

```text
native desktop GUI
Windows UI Automation
macOS Accessibility/CGEvent/ScreenCaptureKit
Linux display/input variants
low-level process control
cross-platform PTY
browser/computer runtime
```

A permanent Go core plus Rust native/Tauri bridge or TS frontend would move away from the user's design goal:

```text
as close to one language as practical
```

The current code is early/incomplete enough that the migration cost is still defensible.

---

# 12. GUI decision

Default final GUI:

```text
Rust native GUI — egui/eframe class
```

Why:

```text
one programming language
no Node frontend runtime/toolchain
no WebView requirement
direct use of Rust domain types
dynamic mission/agent dashboard is a good immediate-mode fit
Devorch does not need to become a full Monaco-based IDE
```

Fallback:

```text
Rust + Tauri/Web frontend
```

only if native Rust GUI creates unacceptable product/UI development friction.

Do not choose Tauri merely because it was in the previous document.

---

# 13. What was NOT validated

The following were not executable in this validation environment:

```text
actual Rust compilation
actual current Devorch `go test ./...`
real Codex account smoke
real Claude account smoke
real Grok account smoke
real Gemini account smoke
macOS native input
Windows native input
Linux input
real Chromium CDP integration
```

Reasons:

```text
Rust toolchain is not installed in this execution container.
The environment cannot network-clone GitHub.
The local Go toolchain is 1.23 while Devorch declares Go 1.24.
External user CLI authentication is not available here.
```

Therefore it would be incorrect to claim:

```text
"The finished product is proven to work."
```

What is proven is:

> the revised development architecture and integration flow survive a realistic deterministic worktree/evaluation/persistence simulation, and two concrete flaws in the earlier plan were found and corrected.

---

# 14. Required real-machine validation gates

Before declaring the architecture confirmed on the development machine:

## V0

```text
Go 1.24 baseline build/test
```

## V1

```text
Rust workspace compile
```

## V2

```text
each of 4 installed CLIs inspected
```

Commands include installed-version help/introspection, not hardcoded assumptions.

## V3

```text
live harmless smoke in temporary worktree
```

for all four.

## V4

```text
native Rust GUI opens and controls fixture mission
```

## V5

```text
browser fixture mission
```

## V6

```text
bounded native OS test
```

---

# 15. Final risk table

| Area | Validation confidence | Notes |
|---|---:|---|
| Git worktree isolation | High | actually simulated |
| Four-candidate evaluation | High | actually simulated |
| Untracked file handling | High after fix | failure discovered and corrected |
| Winner merge | High | actually simulated |
| SQLite restart persistence | High | actually simulated |
| Common event abstraction | High for Gemini/Codex/Grok, medium for Claude until live fixture | public schemas + fixture |
| Adaptive concurrency | High conceptually | deterministic scheduler policy |
| Rust architecture choice | High | strong fit, but compile benchmark still required |
| Native Rust GUI | Medium/High | implementation not executed here |
| Browser runtime | Medium | not executed |
| macOS/Windows/Linux computer use | Medium | requires controlled native testing |
| Fully autonomous Level 5 | Medium | intentionally late gate |

---

# 16. Final verdict

**Proceed.**

But proceed with this exact sequence:

```text
R0 baseline + capability capture
↓
Rust vertical slice
↓
four adapters
↓
headless multi-agent mission
↓
deterministic evaluator
↓
native Rust GUI
↓
retire Go
↓
policy/verification/browser
↓
bounded native control
↓
full Operating Layer
```

Do not implement Go GUI work that will immediately be thrown away.

Do not launch four agents by default.

Do not implement native mouse control before policy/verification.

Do not judge candidates using `git diff` alone.

The two upgraded implementation documents incorporate all of these findings.
