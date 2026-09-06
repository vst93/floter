# Phase 3 Slice 6 / R8 Fixup

## Root Causes

### P1: Missing Migration Archive

The previous round removed recovery's early return when `.transactions` is
absent, but left its unconditional repository save. `ExtensionState::from_paths`
therefore created an empty `extension-repository.json` in a fresh, isolated
temporary directory. Tests subsequently wrote their legacy fixtures, but
`migrate_to_repository` correctly returned `Noop` because a valid repository
already existed. It never reached `archive_file`.

Migration passes the real repository path. The legacy-path redirect predates
the seven-file diff and does not skip or remove an archive. `ExtensionPaths`
derives every path from each test's unique temporary root; shared filesystem
paths are not the cause. The empty repository also caused projection cleanup
to remove the target in `recovery_after_migration_processes_install_journal`.

Recovery must rebuild projections without a journal, but a fresh startup with
no transaction must not manufacture an authoritative empty repository.

### P2: Fault Injection Escaping Its Test

The previous hook stored one armed label in a process-global mutex. Any test
could consume a matching label, overwrite another test's arm, or clear it.
Serializing only fault tests would not exclude ordinary tests hitting hooks.

The replacement uses Tokio task-local scopes: `sync_scope` around synchronous
operations and `scope` around asynchronous operations. Each scope owns one
label, consumes only a matching hook, and restores the prior context on exit,
panic, or cancellation. Async state follows the scoped future across polls and
worker threads. Child tasks do not inherit the arm; a fault in a child must be
scoped explicitly around that child's future. Production hooks remain no-ops.

### Edit-Recovery Expectation Change

`edit_crash_after_new_content_written_keeps_new_content` removes the repository
entry, creates only a new script file, and expects recovery to delete the old
backup while preserving those unregistered new files. Under the single-source
design, writing files is not a commit: the repository must contain the new
entry. Preserving the files while discarding the old generation loses a working
integration and leaves an orphan. This fixture must instead assert restoration
of the complete old entry and original script, removal of the backup/journal,
and stable repeated recovery. A separate committed-edit test must prove that a
new repository entry and its files survive recovery.

The previous edit recovery checked for a new repository entry only inside the
branch where that entry was absent, making committed-edit cleanup unreachable.
Generated-data cleanup also ran before journal recovery could restore entries.

The fix handles edits independently of the uninstall presence/absence branch
and cleans generated data after journal recovery. It also accounts for edits
whose repository metadata is unchanged: the edit pipeline removes the entry
before writing replacement files, so an entry and files together mean a
committed edit or completed rollback. Comparing entry inequality would discard
a committed script-only change made within the same timestamp second. The
committed-edit regression covers both identical and changed metadata, injects
another interruption before journal deletion, and verifies two later recoveries.

## Commit-Point Inventory

All locations below are in `src-tauri/src/extensions`. Hooks run immediately
before the named operation. In particular, directory-sync hooks run after the
preceding rename/persist is visible, before its parent directory is synced.
There are **38 distinct production labels at 41 call sites**. Repeated labels
share the same fault boundary kind; an armed scope fails at its first match.

`Fxx` refers to the direct injection tests in the next table. `None` means the
hook is instrumented but has no dedicated injected-crash assertion in this
slice; ordinary workflow coverage must not be mistaken for fault coverage.

| Label | File / Operation | Direct Injection |
| --- | --- | --- |
| `repository-persist` | `repository.rs`: replace repository with synced temporary file | F04 |
| `repository-directory-sync` | `repository.rs`: sync repository parent after replacement | F05, F12 |
| `repository-archive-rename` | `repository.rs`: rename legacy/corrupt input to archive | F03 |
| `repository-archive-directory-sync` | `repository.rs`: sync archive parent | None |
| `current-pointer-persist` | `lock.rs`: replace `current.json` | F06 |
| `current-pointer-directory-sync` | `lock.rs`: sync pointer parent | None |
| `artifact-persist` | `artifacts.rs`: persist shim or artifact metadata | F01 |
| `artifact-directory-sync` | `artifacts.rs`: sync prepared artifact metadata directory | None |
| `shim-directory-sync` | `artifacts.rs`: sync executable stable shim directory | F02 |
| `descriptor-rename` | `install.rs`: replace reprobed provider description | None |
| `edit-stage-rename` | `install.rs`: move integration to edit backup | None |
| `edit-repository-remove` | `install.rs`: save removal of old edit entry | F10 |
| `edit-repository-finalize` | `install.rs`: save final edit metadata | None |
| `edit-rollback-rename` | `install.rs`: restore backup after failed edit | None |
| `uninstall-stage-rename` | `install.rs`: move extension to removal staging | None |
| `uninstall-repository-remove` | `install.rs`: save uninstall removal | None |
| `install-repository-add` | `install.rs`: save linked/custom installation entry | F11 |
| `sync-import-backup-rename` | `sync.rs`: move existing imported tree to backup | None |
| `sync-import-rename` | `sync.rs`: activate staged imported tree | F13 |
| `sync-import-directory-sync` | `sync.rs`: sync activated import parent | None |
| `sync-import-backup-remove` | `sync.rs`, `transaction.rs`: delete committed import backup (2 sites) | None |
| `sync-atomic-persist` | `sync.rs`: persist export or rewritten import file | None |
| `install-journal-persist` | `transaction.rs`: persist installation journal | F07 |
| `install-journal-directory-sync` | `transaction.rs`: sync installation journal parent | None |
| `removal-journal-persist` | `transaction.rs`: persist removal/edit journal | F08 |
| `removal-journal-directory-sync` | `transaction.rs`: sync removal journal parent | None |
| `journal-remove` | `transaction.rs`: delete completed journal | F14 |
| `removal-journal-quarantine` | `transaction.rs`: quarantine invalid removal journal | None |
| `install-journal-quarantine` | `transaction.rs`: quarantine invalid installation journal | None |
| `removal-recovery-restore` | `transaction.rs`: restore uncommitted uninstall staging | None |
| `edit-recovery-restore` | `transaction.rs`: restore interrupted edit backup | None |
| `install-recovery-restore` | `transaction.rs`: restore interrupted version swap | F09 |
| `projection-remove-orphan-data` | `transaction.rs`: remove unregistered generated tree or empty data directory (2 sites) | None |
| `sync-import-staging-remove` | `transaction.rs`: remove interrupted import staging | None |
| `sync-import-orphan-remove` | `transaction.rs`: delete uncommitted import target | None |
| `sync-import-backup-restore` | `transaction.rs`: restore interrupted import backup | None |
| `projection-remove-orphan` | `transaction.rs`: remove extension directory absent from repository | None |
| `projection-remove-stale` | `transaction.rs`: remove obsolete local pointer/shim file or directory (2 sites) | None |

## Per-Test Coverage

Names below omit the common `extensions::` prefix and the `tests::` module.
Fourteen tests directly inject failures across thirteen distinct boundary
labels. Three further tests verify injection isolation and scope lifetime.

| ID | Test | Assertions |
| --- | --- | --- |
| F01 | `artifacts::fault_at_shim_commit_is_repaired_by_projection_rebuild` | Interrupted shim persist can be retried and yields a shim file. |
| F02 | `artifacts::fault_at_shim_directory_sync_is_repaired_by_projection_rebuild` | Shim present before failed directory sync remains repairable. |
| F03 | `repository::fault_at_legacy_archive_rename_keeps_repository_authoritative` | Repository contains migrated entry; legacy remains; repeated migration is `Noop`. |
| F04 | `transaction::fault_at_repository_persist_recovers_to_a_parseable_source_of_truth` | Existing repository bytes and complete old entry survive failed replacement. |
| F05 | `transaction::fault_after_repository_replace_recovers_the_committed_entry` | Replacement visible before directory sync remains the registered version. |
| F06 | `transaction::fault_at_current_pointer_persist_rebuilds_the_projection` | Recovery reconstructs pointer version from the repository. |
| F07 | `transaction::fault_at_install_journal_persist_leaves_no_half_registered_entry` | Failed first installation journal does not register an entry. |
| F08 | `transaction::fault_at_removal_journal_persist_keeps_repository_coherent` | Failed journal persist leaves recoverable, parseable repository state. |
| F09 | `transaction::fault_at_install_recovery_restore_retries_on_next_launch` | Interrupted backup restoration retries; target and old entry exist, backup is gone. |
| F10 | `install::fault_at_edit_repository_commit_recovers_the_previous_generation` | Failure before old-entry removal restores old name/files and removes edit journal. |
| F11 | `install::fault_at_install_repository_commit_removes_generated_orphans` | Failure before registration leaves no entry or generated integration. |
| F12 | `install::fault_after_uninstall_repository_commit_does_not_resurrect_entry` | Failure after repository removal preserves removal and cleans files/journal. |
| F13 | `sync::fault_at_sync_directory_commit_cleans_staging_on_recovery` | Failure before imported-tree activation leaves empty repository and no target/staging. |
| F14 | `install::edit_crash_after_repository_commit_keeps_new_generation` | Changed and identical metadata retain new script/entry after journal-delete interruption and repeated recovery. |
| S01 | `commit_point_scopes_isolate_threads_and_match_once` | Other thread cannot fire arm; mismatched labels do not consume it; matching label fires once; normal exit and unrelated panic clear context. |
| S02 | `commit_point_scopes_isolate_tasks_on_one_thread` | Another task on the same worker cannot consume an arm while its owner is suspended. |
| S03 | `commit_point_scopes_isolate_tasks_across_worker_threads` | Scoped async fault survives suspension on a multi-worker runtime and delivers the exact injected panic to its owning task. |

The remaining recovery regressions cover these state transitions without a
direct injected panic:

| Test | Coverage |
| --- | --- |
| `repository::fresh_state_recovery_does_not_preempt_legacy_migration` | Fresh startup leaves repository absent; later migration returns `Migrated`, preserves exact archive bytes, and loads the entry. |
| `transaction::edit_recovery_drops_orphaned_new_files_when_repository_never_committed` | Old backup and journal recover the old entry; uncommitted new tree is removed. |
| `transaction::recovery_rebuilds_projections_without_a_journal_directory` | Stale pointer is rebuilt even when `.transactions` is absent. |
| `install::edit_crash_after_uncommitted_content_written_restores_original` | Full old-entry equality, original script bytes, backup/journal deletion, and idempotent recovery. Replaces the incompatible expectation documented above. |

### Original Failure List

| Original Test | Resolution / Preserved Coverage |
| --- | --- |
| `commands::extensions::writer_swap_enable_disable_after_migration` | P1 fix; archive preservation and repository-only enable/disable assertions retained. |
| `install::writer_swap_install_uninstall_edit_and_import_after_migration` | P1 fix; install/uninstall/edit/import writer and archive assertions retained. |
| `install::edit_crash_after_new_content_written_keeps_new_content` | Renamed to `edit_crash_after_uncommitted_content_written_restores_original`; documented semantic correction with stronger rollback checks. |
| `install::fault_at_edit_repository_commit_recovers_the_previous_generation` | Scoped injection, edit dispatch fix; F10. |
| `install::fault_at_install_repository_commit_removes_generated_orphans` | Scoped injection; F11. |
| `sync::mid_import_failure_rolls_back_prior_extensions` | P1 fix; prior entries, archive, and rollback assertions retained. |
| `sync::rollback_snapshot_restores_existing_lock_version_and_configuration` | P1 fix; version/configuration restoration and archive assertions retained. |
| `transaction::recovery_after_migration_processes_install_journal` | P1 fix; migrated entry is loaded before orphan cleanup, so committed target survives. |
| `transaction::recovery_writeback_through_repo_restores_install_entry` | P1 fix; repository writeback, archive preservation, pointer, and idempotence assertions retained. |
| `transaction::recovery_writeback_through_repo_restores_edit_entry` | P1 fix; old entry/files restored before generated-data cleanup, with archive and idempotence assertions retained. |

## Verification Limits

The injector models a panic immediately before selected filesystem boundaries.
It does not simulate an OS power loss, skipped Rust destructors, a torn write,
or `fsync` returning an error. Fault coverage is explicitly limited to the
mapped labels; the inventory does not claim exhaustive crash-window coverage.
There is no global test serialization and no `--test-threads=1` requirement.

## Final Pipeline Results

Verified on 2026-09-06, on `main`. Cargo commands ran in `src-tauri`; JavaScript
commands ran at the repository root. The full pipeline is **not fully green in
this runner** because local Unix-socket tests are blocked by its sandbox.

| Command | Final Result |
| --- | --- |
| `cargo check` | PASS, exit 0. |
| `cargo check --tests` | PASS, exit 0, including the final test changes. |
| `cargo test` | **390 passed, 5 failed, 1 ignored**, 396 library tests total; exit 101. All extension tests pass. Default parallel execution, no exclusions. |
| `cargo test extensions:: -- --test-threads=16` | **305 passed, 0 failed, 0 ignored**, 91 filtered out; exit 0. Both binary test targets also pass with 0 tests. |
| `npx --legacy-peer-deps tsc --noEmit` | PASS, exit 0. |
| `node --experimental-strip-types --test tests/*.test.ts` | PASS: **9 file-level tests**, 0 failed, 0 skipped; exit 0. |
| `npm test` | PASS: **85 individual tests**, 0 failed, 0 skipped; exit 0. Uses the repository's `--test-isolation=none` command to expose individual test counts. |
| `npm run build` | PASS, exit 0; TypeScript and Vite complete, 1,860 modules transformed. |
| `git diff --check` | PASS. |

The five full-suite failures are:

1. `ipc::tests::delivers_a_toggle_line`: Unix socket bind returns `EPERM`.
2. `ipc::tests::reclaims_a_socket_left_by_a_crash`: first bind returns `EPERM`.
3. `ipc::tests::refuses_a_socket_a_live_instance_owns`: first bind returns `EPERM`.
4. `terminal::broker::tests::broker_executes_command_and_reports_exit`: daemon
   cannot bind its temporary Unix socket, then startup times out.
5. `terminal::broker::tests::detached_session_can_be_listed_and_reattached`: the
   preceding broker failure poisons their existing shared daemon-test mutex.

These failures are outside the extension modules. Their assertions and test
selection were not changed. The existing ignored test remains the manual
terminal throughput benchmark. Because `cargo test` stops at the library
failure, its later binary and documentation test phases are not counted as
completed by that full-suite command.

Two attempts to run the requested `cargo test` outside the sandbox were rejected
before execution by automatic approval review. Its reviewer failed with:

```text
404 Not Found: Model "gpt-5.6-luna" is not supported by any configured account in this group
```

The rejection is an approval-service configuration failure, not a Rust test
result. Completing the full-suite verification requires enabling the approved
outside-sandbox test execution or repairing that reviewer configuration so
the existing local IPC tests can bind sockets. No denied action was bypassed.

Final logs are available at `/tmp/floter-r8-cargo-test-sandbox.log`,
`/tmp/floter-r8-extensions-parallel.log`, `/tmp/floter-r8-cargo-check.log`,
`/tmp/floter-r8-cargo-check-tests.log`, `/tmp/floter-r8-tsc.log`,
`/tmp/floter-r8-node-tests.log`, `/tmp/floter-r8-npm-tests.log`, and
`/tmp/floter-r8-build.log`.

All seven original modified Rust files remain in the working tree together
with this report. No commit or push was made.
