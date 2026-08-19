# The Go baseline's SQLite schema

These are the 22 migrations the Go runtime shipped, preserved verbatim after
the Go tree was removed.

They are kept for two reasons, neither of them sentimental:

- **The data still exists.** The Rust core writes `~/.devorch/devorch3.db`; a
  user who ran the Go build still has `~/.devorch/devorch.db` with their OkAON
  history in it. These files are what a migration of that data has to read.
- **`../sql-schema-map.md` refers to them by name.** A disposition table that
  points at deleted files is not a plan.

Nothing here runs. The Rust migrations live in
`crates/devorch-store/migrations/`.
