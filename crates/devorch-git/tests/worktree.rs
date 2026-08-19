//! Integration tests against real git repositories.
//!
//! These reproduce the scenarios the development simulation exercised,
//! including the two flaws it uncovered: `git diff` missing untracked files, and
//! generated test output interfering with candidate comparison.

use std::path::Path;

use devorch_git::{change_inventory, Repository};
use devorch_process::{run, ProcessSpec};

async fn git_ok(dir: &Path, args: &[&str]) {
    let spec = ProcessSpec::new("git").args(args).cwd(dir);
    let out = run(&spec).await.expect("git ran");
    assert!(
        out.success(),
        "git {args:?} failed: {}{}",
        out.stdout,
        out.stderr
    );
}

/// Seed a repository with the simulation's buggy calculator.
async fn seed_repo(dir: &Path) {
    git_ok(dir, &["init", "-b", "main"]).await;
    git_ok(dir, &["config", "user.email", "test@devorch.local"]).await;
    git_ok(dir, &["config", "user.name", "Devorch Test"]).await;
    std::fs::write(dir.join(".gitignore"), "__pycache__/\n*.pyc\n").unwrap();
    std::fs::write(
        dir.join("calculator.py"),
        "def add(a, b):\n    return a - b\n",
    )
    .unwrap();
    git_ok(dir, &["add", "."]).await;
    git_ok(dir, &["commit", "-m", "seed bug"]).await;
}

#[tokio::test]
async fn opens_a_repository_and_resolves_head() {
    let tmp = tempfile::tempdir().unwrap();
    seed_repo(tmp.path()).await;

    let repo = Repository::open(tmp.path()).await.expect("open");
    let sha = repo.rev_parse("HEAD").await.expect("rev-parse");
    assert_eq!(sha.len(), 40, "expected a full SHA, got {sha:?}");
    assert_eq!(
        repo.current_branch().await.unwrap().as_deref(),
        Some("main")
    );
}

#[tokio::test]
async fn opening_a_non_repository_is_a_typed_error() {
    let tmp = tempfile::tempdir().unwrap();
    let err = Repository::open(tmp.path()).await.expect_err("not a repo");
    assert!(matches!(err, devorch_git::GitError::NotARepository { .. }));
}

#[tokio::test]
async fn worktrees_are_created_listed_and_removed() {
    let tmp = tempfile::tempdir().unwrap();
    let repo_dir = tmp.path().join("repo");
    std::fs::create_dir_all(&repo_dir).unwrap();
    seed_repo(&repo_dir).await;

    let repo = Repository::open(&repo_dir).await.expect("open");
    let base = repo.rev_parse("HEAD").await.expect("head");

    let wt = tmp.path().join("wt-codex");
    repo.worktree_add(&wt, "devorch/codex", &base)
        .await
        .expect("worktree add");
    assert!(wt.join("calculator.py").exists());

    let listed = repo.worktree_list().await.expect("list");
    assert_eq!(
        listed.len(),
        2,
        "main worktree plus the candidate: {listed:?}"
    );

    repo.worktree_remove(&wt, true).await.expect("remove");
    repo.branch_delete("devorch/codex", true)
        .await
        .expect("branch delete");
    assert!(!wt.exists());
    assert_eq!(repo.worktree_list().await.expect("list").len(), 1);
}

#[tokio::test]
async fn inventory_reports_a_clean_worktree_as_empty() {
    let tmp = tempfile::tempdir().unwrap();
    seed_repo(tmp.path()).await;

    let inv = change_inventory(tmp.path()).await.expect("inventory");
    assert!(inv.is_empty(), "unexpected changes: {inv:?}");
    assert_eq!(inv.churn, 0);
}

#[tokio::test]
async fn inventory_sees_untracked_files_that_git_diff_omits() {
    // This is the exact defect the development simulation found: the Gemini
    // candidate edited one tracked file and added NOTES.md. A `git diff`-only
    // evaluator scores it identically to a candidate that added nothing.
    let tmp = tempfile::tempdir().unwrap();
    seed_repo(tmp.path()).await;

    std::fs::write(
        tmp.path().join("calculator.py"),
        "def add(a, b):\n    return a + b\n",
    )
    .unwrap();
    std::fs::write(tmp.path().join("NOTES.md"), "extra file\n").unwrap();

    let inv = change_inventory(tmp.path()).await.expect("inventory");

    assert_eq!(inv.tracked_modified, vec![Path::new("calculator.py")]);
    assert_eq!(inv.untracked, vec![Path::new("NOTES.md")]);
    assert_eq!(
        inv.touched_count(),
        2,
        "untracked files must count toward candidate scope"
    );
    assert!(inv.churn > 0);
}

#[tokio::test]
async fn inventory_separates_staged_deleted_and_generated_paths() {
    let tmp = tempfile::tempdir().unwrap();
    seed_repo(tmp.path()).await;

    // A file to delete later; committed before anything else is staged, so the
    // commit cannot sweep up the staged edit below.
    std::fs::write(tmp.path().join("doomed.txt"), "bye\n").unwrap();
    git_ok(tmp.path(), &["add", "doomed.txt"]).await;
    git_ok(tmp.path(), &["commit", "-m", "add doomed"]).await;

    // A staged edit.
    std::fs::write(
        tmp.path().join("calculator.py"),
        "def add(a, b):\n    return a + b\n",
    )
    .unwrap();
    git_ok(tmp.path(), &["add", "calculator.py"]).await;

    // A deletion.
    std::fs::remove_file(tmp.path().join("doomed.txt")).unwrap();

    // Generated output that is NOT gitignored, so it reaches the inventory.
    std::fs::create_dir_all(tmp.path().join("build")).unwrap();
    std::fs::write(tmp.path().join("build/out.log"), "noise\n").unwrap();

    let inv = change_inventory(tmp.path()).await.expect("inventory");

    assert!(inv
        .staged
        .contains(&Path::new("calculator.py").to_path_buf()));
    assert!(inv.deleted.contains(&Path::new("doomed.txt").to_path_buf()));
    assert!(
        inv.generated
            .iter()
            .any(|p| p == Path::new("build/out.log")),
        "generated output must be classified: {:?}",
        inv.generated
    );
}

#[tokio::test]
async fn gitignored_build_output_stays_out_of_the_inventory() {
    // The simulation's first failure: Python test runs created __pycache__ and
    // broke the winner merge. With the directory ignored, it must not appear as
    // a candidate change at all.
    let tmp = tempfile::tempdir().unwrap();
    seed_repo(tmp.path()).await;

    std::fs::create_dir_all(tmp.path().join("__pycache__")).unwrap();
    std::fs::write(tmp.path().join("__pycache__/calculator.pyc"), "bytecode").unwrap();

    let inv = change_inventory(tmp.path()).await.expect("inventory");
    assert!(inv.is_empty(), "ignored build output leaked in: {inv:?}");
}

#[tokio::test]
async fn binary_files_are_classified_rather_than_counted_as_churn() {
    let tmp = tempfile::tempdir().unwrap();
    seed_repo(tmp.path()).await;

    let blob = tmp.path().join("asset.bin");
    std::fs::write(&blob, [0u8, 159, 146, 150, 0, 1, 2, 3]).unwrap();
    git_ok(tmp.path(), &["add", "asset.bin"]).await;
    git_ok(tmp.path(), &["commit", "-m", "add binary"]).await;
    std::fs::write(&blob, [3u8, 2, 1, 0, 150, 146, 159, 0]).unwrap();

    let inv = change_inventory(tmp.path()).await.expect("inventory");
    assert!(
        inv.binary.iter().any(|p| p == Path::new("asset.bin")),
        "binary change must be flagged: {inv:?}"
    );
}

#[tokio::test]
async fn candidates_in_separate_worktrees_do_not_see_each_others_changes() {
    let tmp = tempfile::tempdir().unwrap();
    let repo_dir = tmp.path().join("repo");
    std::fs::create_dir_all(&repo_dir).unwrap();
    seed_repo(&repo_dir).await;

    let repo = Repository::open(&repo_dir).await.expect("open");
    let base = repo.rev_parse("HEAD").await.expect("head");

    let a = tmp.path().join("wt-a");
    let b = tmp.path().join("wt-b");
    repo.worktree_add(&a, "devorch/a", &base).await.unwrap();
    repo.worktree_add(&b, "devorch/b", &base).await.unwrap();

    std::fs::write(
        a.join("calculator.py"),
        "def add(a, b):\n    return a + b\n",
    )
    .unwrap();
    std::fs::write(b.join("NOTES.md"), "only in b\n").unwrap();

    let inv_a = change_inventory(&a).await.expect("inventory a");
    let inv_b = change_inventory(&b).await.expect("inventory b");

    assert_eq!(inv_a.tracked_modified, vec![Path::new("calculator.py")]);
    assert!(inv_a.untracked.is_empty(), "a saw b's file: {inv_a:?}");
    assert_eq!(inv_b.untracked, vec![Path::new("NOTES.md")]);
    assert!(
        inv_b.tracked_modified.is_empty(),
        "b saw a's edit: {inv_b:?}"
    );
}
