# SQLite schema compatibility map

**Wave:** R0
**Go baseline:** `main@9ecdc71`, 22 migrations under `internal/storage/sqlite/migrations/`
**Rust core:** `crates/devorch-store/migrations/`, starting at `0001_init.sql`

## Coexistence rule

The Rust core writes a **separate database file** — `~/.devorch/devorch3.db` —
while the Go baseline keeps using `devorch.db`. Nothing in the Rust core opens,
migrates or mutates the Go database.

This is deliberate. The plan calls for parity testing against the same data, but
running two migration systems against one file makes the first Rust bug a data
loss event rather than a failed test. Shared-file parity testing is a decision
for wave R3, once the Rust store has a mission's worth of real use, and it must
be preceded by a verified backup and restore.

## Disposition of the 22 Go migrations

| Table | Migration | Disposition | Notes |
|---|---|---|---|
| `meta` | 0001 | **REPLACE** | Rust uses `schema_migrations`; the Go `meta` key/value row is not a general-purpose table. |
| `sessions` | 0001 | **MIGRATE** | Superseded by the mission/task model. Session identity moves to `SessionId` on the event journal. |
| `messages` | 0001 | **MIGRATE** | Becomes `AgentEvent::TextDelta` rows in `event_journal`, with the vendor payload retained. |
| `usage_ledger` | 0002 | **KEEP** | Maps onto `AgentEvent::Usage`. Note that unknown cost must stay `NULL`, never 0. |
| `okaon_runs` | 0003 | **DEPRECATE** | Superseded by `okaon_run` (0020). Retained read-only for history. |
| `local_runtime_inventory` | 0004 | **MIGRATE** | Replaced by runtime capability discovery (`devorch agent inspect`), which observes rather than caches. |
| `auth_tokens` | 0005 | **REPLACE** | Credentials move to the OS secret broker (Keychain / Credential Manager / Secret Service). Not a Devorch table. |
| `auth_sessions` | 0010 | **REPLACE** | Same as above. |
| `bench_runs` | 0006 | **KEEP** | OkAON benchmark input. |
| `benchmarks` | 0013 | **KEEP** | OkAON benchmark input. |
| `quality_runs` | 0007 | **KEEP** | Feeds the verification engine's historical quality signal. |
| `tool_runs` | 0008, 0011 | **MIGRATE** | Becomes `ToolStarted` / `ToolCompleted` in the event journal. |
| `test_runs` | 0009 | **MIGRATE** | Becomes verification evidence attached to a candidate, not a free-standing table. |
| `model_stats` | 0012 | **KEEP** | Router input. |
| `feedback` | 0014 | **KEEP** | Human intervention signal; an input to the reward function. |
| `router_policy` | 0015, 0016 | **KEEP** | Router policy, including scope. |
| `router_policy_snapshots`, `router_drift_events` | 0018 | **KEEP** | Drift and rollback history. |
| `env_signatures`, `attribution_events` | 0017 | **KEEP** | Repo/environment fingerprinting; needed for task-similarity routing. |
| `experiments`, `experiment_variants`, `experiment_assignments`, `experiment_outcomes`, `experiment_decisions` | 0019 | **KEEP** | Experiment framework. Port whole, not piecemeal. |
| `okaon_work`, `okaon_run`, `okaon_quality`, `okaon_reward`, `okaon_arm_stats` | 0020 | **KEEP** | The live OkAON learning schema. This is the data the migration must not lose. |
| `okaon_runs_v`, `okaon_arm_stats_v`, `okaon_rewards_v` | 0021 | **KEEP** | Views over the above. Re-derive in Rust rather than copying rows. |
| `okaon_run_context`, `okaon_eval_results` | 0022 | **KEEP** | Hardware/repo/task context and evaluation results. |

## Required change on port

`okaon_reward` must record the **reward definition version** alongside every
value. Raw outcome metrics are stored first and the reward is computed from
them; a reward row without its version cannot be recompared after the weights
change, which silently corrupts every historical comparison.

## Backup gate

Before any wave that touches the Go database:

```bash
cp ~/.devorch/devorch.db ~/.devorch/devorch.db.pre-rust-backup
sqlite3 ~/.devorch/devorch.db ".schema" > docs/rust-migration/go-schema-snapshot.sql
```

Restore must be verified, not assumed.
