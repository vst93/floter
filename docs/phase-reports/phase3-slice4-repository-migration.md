# Phase 3 Slice 4 — Single ExtensionRepository Schema + Migration

> Current contract after slice 8: only `extension-repository.json` plus journals
> holds extension state. The adapter/fallback behavior below describes slice 4
> historically. Normal load/save now reject legacy paths; startup recovery owns
> one migration entry and persists imported state before cleanup. `.migrated`
> remains an upgrade/crash recovery input, never a normal read source, and
> `.corrupt` prevents empty-state resets. `ExtensionsLock` remains a live API.

Round: R5 of 2026-09-05 session, dispatched to Codex (gpt-6-astra, reasoning
effort max) after the user switched the dev tool from Claude Code (relay 403:
key group no longer permitted). Codex died at the verification/report stage on
429 Too Many Requests after 572k tokens; the complete implementation was left
in the working tree and verified independently by the coordinator. Report
written by the coordinator (agent died before writing it).

## Premise-check-against-code (coordinator verified post-hoc)

- `ExtensionsLock` is defined in `src-tauri/src/extensions/lock.rs`; the
  former `extensions.lock.json` (schema v2) is now only a migration input.
- Startup init: `ExtensionPaths` now carries `repository_file`
  (`extension-repository.json`); migration is wired into the extensions
  subsystem init with `tracing` outcome logs (Migrated / Noop / skipped+warn).
- Journal recovery entry point `recover()` (transaction.rs) keeps working —
  it reads/writes the lock through the `ExtensionsLock` API, which now routes
  legacy paths through the repository loader.

## Implementation (as landed)

- New module `src-tauri/src/extensions/repository.rs` (402 lines, 5 tests):
  versioned `extension-repository.json` schema mirroring `ExtensionLockEntry`,
  `migrate_to_repository()` with atomic temp-file+rename write, legacy lock
  archived as `.migrated` (not deleted), idempotent re-run, both-exist rule
  (lock preferred while it was the live writer), `MigrationOutcome` enum.
- `lock.rs` converted to the adapter pattern the spec allowed as smallest
  churn: `ExtensionsLock::load` routes legacy lock paths through
  `repository::load_for_legacy_path` (repo first, lock fallback); `load_legacy`
  + `validate_entries` split out for reuse. `save` gains `save_legacy` for
  test fixtures that must exercise the old file format.
- `sync.rs`: import/export paths aligned with the new loader.
- No writer swaps (slice 5), no legacy deletion (slice 8), no frontend changes
  — per spec.

## Tests

5 new tests in repository.rs covering: legacy-lock migration + archive +
idempotency, loader fallback (repo present / lock present / repo corrupt +
.migrated lock), and migration+recovery interplay (journal processed
correctly after migration; pointer file contents asserted). Existing suite
adapted where it constructed locks directly.

## Verification (coordinator, independent)

- cargo check: pass
- cargo test: 360 passed, 0 failed, 1 ignored (floor 354) on clean runs;
  one INTERMITTENT failure observed under parallel load (~1/3 of runs):
  `failing_version_probe_rejects_the_binary_set` (artifacts.rs:308,
  `assertion failed: error.contains("version probe failed")`). This is the
  known load-sensitive probe test (e174e51 injected a 10s timeout when the
  suite was 343 tests; at 360 the injected budget is no longer sufficient
  under load). NOT caused by this slice (artifacts.rs untouched). Fix
  scheduled as Task 0 of the next round per the no-rerun-and-dismiss rule.
- tsc --noEmit: pass
- npm run build: pass
- node tests: 76/76 pass

## Next slices (design report §Slice 5-8)

Slices 5-8 are complete: repository writers, scoped fault injection, advanced
crash/corruption coverage, and removal of implicit legacy load/save adapters.
Explicit migration and installation-journal recovery remain for crash safety.
