These rules apply to every coding agent working on Devorch — Codex, Claude,
Grok and Gemini alike. They exist because four agents work this repository
concurrently in separate git worktrees, and a rule broken by one agent shows up
as a merge conflict or a lost test for the other three.

## Ground rules

1. **Never edit `main`.** Work only in the worktree assigned to your ticket.
2. **Only modify the paths your ticket owns.** Paths listed as read-only
   dependencies may be read, never written.
3. **Never delete or skip a test to make a gate pass.** If a test is wrong, say
   so in the completion report and leave it failing.
4. **Never silence a failing command.** No `|| true`, no swallowed exit codes,
   no commented-out assertions.
5. **Do not add a second implementation of something a crate already does.**
   Extend the existing crate or say why it cannot be extended.
6. **Do not modify root files unless you are the assigned integrator.**
   That means `Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml`, the migration
   registry, and any breaking change to `devorch-protocol`.
7. **No broad dependency upgrades.** If you need a new dependency, record the
   request in your report; the integrator adds it and you rebase.
8. **No license-incompatible copying.** Upstream code may only be ported with an
   exact source commit recorded in `docs/upstream-sources.toml`.

## Reporting

Every completed ticket returns:

- changed files **and untracked files** — `git diff` alone does not show the
  second, and an unrequested file is a finding, not a detail;
- the full output of the ticket's gate commands;
- risks you noticed but did not address;
- the commit SHA.

## The gate

Unless your ticket says otherwise, run all four before reporting:

```bash
cargo fmt --all -- --check
cargo check --workspace
cargo test --workspace
cargo clippy --workspace --all-targets -- -D warnings
```

## Scope discipline

Devorch is being migrated to Rust in waves, and each wave has a reason to exist.
Do not jump ahead of your ticket. In particular, no native mouse or keyboard
control, no browser automation, and no external side effects until the wave that
introduces the policy and verification engines has landed. A patch that reaches
past its wave is rejected on sight, however good the code is.

## Verification

An agent saying a task is done is a claim, not evidence. Deterministic program
state outranks it every time: exit codes, test output, filesystem state, then
DOM or accessibility state, then visual evidence, and only then an LLM judgement.
Never report a successful build you did not observe the output of.
