# Devorch 3 migration — waves R0 → R2

**Date:** 2026-08-19
**Baseline:** `main@9ecdc71`
**Result:** R0, R1 and R2 gates all pass on this machine.

The three planning documents in `docs/upgrade/` were executed in the order they
specify. Nothing past R2 was implemented: no GUI, no agent adapters, no browser
or native control. Waves exist so a patch that reaches past its wave can be
rejected, and that applies to this work too.

---

## R0 — measure and freeze

### Toolchains verified on this machine

| Tool | Declared | Installed |
|---|---|---|
| Go | 1.24.0 | 1.26.5 |
| cargo | — | 1.97.1 |
| rustc | — | 1.97.1 |

The validation report listed "local Go toolchain is 1.23 while Devorch declares
1.24" and "Rust toolchain is not installed" as blockers. Both are resolved here;
V0 and V1 can be closed.

### Go baseline gates

| Gate | Result |
|---|---|
| `go build ./...` | PASS |
| `go build ./cmd/devorch` | PASS — 20,141,264 bytes, 16.5 s |
| `go build ./cmd/devorchd` | PASS — 15,985,008 bytes |
| `go vet ./...` | PASS *(after the three fixes below)* |
| `go test ./...` | PASS — 3 ok, 0 fail, 94 packages with no tests |

### Measured, not guessed

| Metric | Value |
|---|---|
| `devorchd` idle RSS | 14.1 MB at 4 s; 16.1 MB after probes |
| `devorch --help` startup | 0.48 s cold, 0.01 s warm |
| `/health` latency | 0.55–0.66 ms |
| `/api/providers` latency | 0.59–0.66 ms |
| Daemon HTTP endpoints | 27 |

Raw output is in `docs/rust-migration/r0/`, structured data in `baseline.json`.

### Source inventory

The README describes "approximately 248 Go source files". The actual count at
this commit is **359 files, 58,685 lines, 97 packages**.

The more consequential number is the second one: **3 test files across 97
packages**. The migration plan calls for parity tests against "verified
behavior", and that behavior is almost entirely unverified today. Parity
characterization tests in R2/R3 have to be *written*, not merely ported.

### Three defects found

**R0-D1 — `internal/orchestrator/brain.go` was corrupt.** The file had a
duplicate `package orchestrator` line, ~317 blank lines, and then the entire
remainder of the file collapsed onto a single line *with the original line order
reversed*. It also imported `github.com/devorch/...` while the module is
`devorch`. The package never compiled, and nothing in the repository imported it.

The content is not mechanically recoverable — the newlines are gone, so the
reversed lines cannot be split apart reliably. The whole `internal/orchestrator`
package (5 files; the other 4 depend on types declared only in the corrupt one)
is quarantined under `archive/broken/` with `.quarantine` suffixes so it stays
out of the build.

This is the "AI OS autonomous mode" tier system. Its replacement is the mission
and task DAG specified in Document 2 — it should be rebuilt there, not repaired.

**R0-D2 — `internal/cli/unified.go:4785`** computed `memUsedGB` as `int64` and
printed it with `%.1f`. Because `go test` runs vet, this single line was failing
the entire `internal/cli` test binary. Fixed.

**R0-D3 — `internal/session/manager.go:160`** copied a `Session` by value and
json-encoded the copy; `Session` embeds a `sync.RWMutex`, so this copied a lock
while holding it. Fixed by adding a lock-free `sessionDoc` snapshot type. The
on-disk JSON shape is unchanged.

Three defects, all in the "go vet is failing so nobody runs it" category. This
supports the migration rationale in Document 1 §2.3 more concretely than the
document does: the integration debt is real and is currently invisible.

---

## R1 — Rust workspace skeleton

Eight crates plus a binary and `xtask`, per Document 1 §13:

```
crates/devorch-protocol    ids, normalized events, change inventory
crates/devorch-config      runtime policy and paths
crates/devorch-process     cross-platform process execution
crates/devorch-git         worktree lifecycle, complete change inventory
crates/devorch-store       SQLite migrations, workspaces, event journal
crates/devorch-events      sequence assignment and journalling
crates/devorch-workspace   worktrees bound to persisted records
crates/devorch-agent       capability discovery
bins/devorch               CLI
xtask                      agent-rules generation
```

### Gate R1

```
cargo fmt --all -- --check                                PASS
cargo check --workspace                                   PASS
cargo test --workspace                                    PASS — 78 tests
cargo clippy --workspace --all-targets -- -D warnings     PASS
```

### Decisions worth recording

**The database is a separate file.** `~/.devorch/devorch3.db`, not the Go
`devorch.db`. Two migration systems against one file makes the first Rust bug a
data-loss event. See `sql-schema-map.md` for the per-table disposition of all 22
Go migrations, and the backup gate that must precede any shared-file testing.

**`ProcessSpec` has no shell.** Arguments are a vector, and `env` is `Option`:
`None` inherits the parent environment, `Some` replaces it entirely. Agent
processes must always pass `Some`, which is what makes the secret broker in
Document 2 §26 implementable rather than aspirational. There is a test asserting
a parent variable does not leak into a scoped child.

**Sequence numbers are assigned centrally.** `SessionRecorder` owns numbering,
not the adapters, so four independently written adapters cannot each invent
their own rule. `resume()` continues from the journal's high-water mark; there is
a test that writes events, drops the store, reopens the file and asserts the next
event lands at seq 2 rather than 0.

**Unknown cost is `Option<f64>`, never 0.0.** Document 2 §29.

---

## R2 — vertical slice parity

`config → SQLite → repository → worktree → persisted record → CLI`, on real git.

### Acceptance

```
devorch workspace create <name> --repo <path> [--base <rev>] [--agent <agent>]
devorch workspace list [--all]
devorch workspace remove <name> [--force]
devorch workspace inventory <name>
devorch agent inspect [--agent <agent>]
devorch doctor
```

All support `--json`.

### The simulation, reproduced through the real CLI

Four worktrees off one seeded repository, given the four candidate patches from
`devorch_development_simulation.py`:

| Workspace | tracked | untracked | churn |
|---|---|---|---:|
| codex | `calculator.py` | — | 2 |
| claude | `calculator.py` | — | 3 |
| grok | `calculator.py` | — | 2 |
| gemini | `calculator.py` | **`NOTES.md`** | 3 |

This is the defect the simulation found, now caught by production code: on a
`git diff --numstat` basis alone, gemini and claude are indistinguishable. The
inventory runs all four commands the plan requires — `status --porcelain=v1`,
`diff --numstat`, `diff --cached --numstat`, `ls-files --others
--exclude-standard` — and `ChangeInventory::touched_count()` counts untracked
files toward candidate scope.

`__pycache__/calculator.pyc` was also dropped into the codex worktree — the
artifact that broke the simulation's first merge. Being gitignored, it correctly
does not appear as a candidate change at all. When such output is *not* ignored,
it is classified as `generated` rather than silently counted as source; there is
a test for each case.

### Isolation, persistence and cleanup, verified

- Two worktrees editing different files see none of each other's changes.
- A fresh process against the same database lists all four workspaces — restart
  persistence, matching the simulation's SQLite reopen test.
- `remove --force` deletes the worktree and branch, and `git worktree list`
  confirms they are gone from the repository.
- The record survives as `removed` and stays visible under `list --all`. The
  evidence lives in the journal, not in the directory.

### Agent inspection — the V2 gate

All four CLIs are installed and were inspected by running them, not by assuming
their flags:

| Agent | Version | Mode | Structured formats | Protocols | Resume | Permissions |
|---|---|---|---|---|---|---|
| codex | codex-cli 0.144.5 | OneShotStructured | `json` | app-server, mcp | yes | yes |
| claude | 2.1.228 | OneShotStructured | `stream-json`, `json` | mcp | yes | yes |
| grok | 1.0.4 (d846eb93d94d) | OneShotStructured | `streaming-messages-json`, `streaming-json`, `json` | acp, mcp | yes | yes |
| gemini | 0.46.0 | OneShotStructured | `stream-json`, `json` | acp, mcp | yes | yes |

Every one supports structured headless output, so **PTY is the fallback for all
four, not the integration path for any of them** (Document 1 §5).

The Claude condition from the validation report — that the adapter must capture
the *installed* CLI's surface rather than assume a fixed schema — is satisfied
structurally: capabilities come from parsing what the binary advertises, so if a
flag disappears in a later version the capability disappears with it and the
router stops selecting that mode, instead of the adapter failing mid-mission.

### Resource policy

`max_parallel_agents` defaults to **2**. Candidate count is chosen by task risk
(low 1, normal 2, high 4), and concurrency is capped independently, so high-risk
work evaluates four candidates in two waves rather than running four processes.
Idle agent processes: **zero** — nothing starts until a mission needs it.

---

## Supporting deliverables

- `docs/agent-rules.md` — the single source for the four-agent rules, with
  `AGENTS.md` / `CLAUDE.md` / `GEMINI.md` generated by
  `cargo xtask sync-agent-rules`. CI fails if a generated file was hand-edited.
- `.github/workflows/rust.yml` — the four gates plus the rules check, on Linux,
  macOS and Windows.
- `docs/rust-migration/sql-schema-map.md` — disposition of all 22 Go migrations.
- `docs/rust-migration/baseline.json`, `docs/rust-migration/r0/` — R0 evidence.

---

## What is NOT done

Stated plainly, because the validation report was right to insist on it.

- **No agent adapters** (R3). Capability discovery exists; nothing drives an
  agent yet. No fixtures have been recorded.
- **No live smoke** (R4 / V3). No agent has been run against a worktree. This is
  the next wave and needs the recorded fixtures first.
- **No mission or task DAG**, no scheduler, no leases (R3 / C0).
- **No evaluator or merge** (R5). The inventory that feeds it exists and is
  tested; the scoring and merge logic does not.
- **No GUI** (R6), no policy engine, no verification engine, no browser, no
  native control. Document 2 is untouched.
- **Go is not retired** (R7) and must not be until parity tests exist. The near
  absence of Go tests makes this the most expensive remaining wave, and R0
  measured that cost rather than estimating it.
- **Windows and Linux are unbuilt.** The CI matrix will be their first real test;
  `devorch-process` was written platform-neutral but has only run on macOS.

## Recommended next step

Wave R3, cross-assigned as Document 1 §38 specifies — an agent must not be both
sole author and sole validator of its own integration:

```
Claude -> Codex adapter
Codex  -> Grok adapter
Grok   -> Gemini adapter
Gemini -> Claude adapter
```

Record fixtures under `testdata/agents/<agent>/` from real sessions first, then
write the adapters against the fixtures. Default CI must not require a
subscription.
