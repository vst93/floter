# Phase 3 Slice 2 — Removal Journal Retry-on-Failure + Retention Reachability Verdict

Round: R3 of 2026-09-04/05 session. Dispatched 2026-09-05 04:05, Claude session
died silently before writing its report (rc 0, truncated output — the known
"empty-output rc 0" failure shape). The working tree held a complete,
self-consistent diff for Tasks 1–2; Hermes reviewed the full diff, ran the
independent verification pipeline, and committed it.

## Task 1 — Recovery debt fix (landed)

`recover_removal_journals()` (src-tauri/src/extensions/transaction.rs, committed
branch) previously ignored cleanup failures (`let _ = remove_dir_all`) and then
deleted the journal unconditionally — a second deletion failure (locked file,
permissions, I/O) would destroy the residue's auto-recovery path, violating
slice 1's own invariant ("NEVER lock gone + residue without auto-recovery").

Now: if any staged_path / cleanup_path deletion fails, the journal is KEPT on
disk (tracing::warn with path + error) and retried on the next startup; the
journal is only removed when every planned deletion succeeded or the paths no
longer exist.

Tests (both exercise the real `recover_removal_journals()` code path; failure
simulated portably by replacing the target directory with a file):
- `removal_journal_persists_when_cleanup_fails_on_first_recovery` — staged-path
  branch: first recovery keeps journal + obstacle; removing the obstacle lets a
  second recovery finish cleanup and drop the journal.
- `removal_journal_persists_when_cleanup_path_fails` — cleanup_paths branch,
  same two-phase contract.

## Task 2 — Version-retention reachability premise check: verdict B (dead code)

Evidence: after 350e2d6 removed the NPM pipeline (update/rollback/reinstall),
`retain_versions` had zero reachable callers. The only remaining call site was
inside `recover()`'s install-journal branch itself. No current install/update
path writes a second version dir, and no command/frontend path triggers
retention or rollback.

Verdict B executed: `retain_versions` and its helpers/tests removed entirely
(net: the `retention_keeps_current_and_previous_only` test deleted; dead-path
call in recover() removed). The Phase 3 acceptance criterion "at least 2
configurable previous versions retained" is OBSOLETE and is dropped from the
roadmap (pre-dates the NPM removal; versioned retention no longer has an
operation that could produce multiple versions).

## Task 3 — edit_custom_integration same-class sweep: NOT done

Session died before this task; carried into the next round (R4).

## Verification (independent, Hermes-run)

- cargo check: pass
- cargo test: 350 passed, 0 failed, 1 ignored (floor was 349)
- tsc --noEmit: pass
- npm run build: pass
- node tests: 76/76 pass

## Carried to R4

- edit_custom_integration commit-before-operation fix (intent journal before
  lock removal, recovery completes or restores).
- Phase 3 core premise check: single state source + fault injection at every
  commit point (design report before implementation).
