# Phase 3 Slice 7: Advanced Fault Injection

## Step 0: Premise Check (Before Test Changes)

Inspected the clean `main` worktree on 2026-09-07. The references in this
section are the actual pre-change source locations, not the slice-3 design
document's proposed locations. No commit or push is authorized or performed.

| Area | Actual Source and Contract |
| --- | --- |
| Fault harness | `src-tauri/src/extensions/mod.rs:39`: `commit_point`; `mod.rs:62`: Tokio task-local `Cell<Option<&'static str>>`; `mod.rs:66`: `with_commit_point`; `mod.rs:71`: `with_async_commit_point`. A matching hook consumes the arm and panics. Production is a no-op. No callback or process-termination action exists yet. |
| Repository persistence | `src-tauri/src/extensions/repository.rs:109`: `write_repository`; temporary file write/flush/fsync precede the `repository-persist` hook at line 135; rename precedes `repository-directory-sync` at line 139. `lock.rs:217` delegates all saves to this writer. |
| Archives/migration | `repository.rs:144`: `archive_file`; rename hook at line 156, directory-sync hook at line 160. `repository.rs:171`: `migrate_to_repository` writes/syncs before archiving the legacy lock. Migration is not in `transaction.rs`. |
| Recovery/startup | `src-tauri/src/extensions/transaction.rs:421`: `recover`; staging cleanup, repository load, removal journals, installation journals, sync staging, generated-data cleanup, projections. `mod.rs:225` propagates recovery errors from startup. |
| Current pointer | `src-tauri/src/extensions/lock.rs:432`: `current_pointer_path`, `extensions/{id}/current.json`; `lock.rs:444`: atomic `write_current_pointer`; hooks at lines 470/474. `transaction.rs:690` rebuilds NPM pointers from repository entries and removes obsolete local pointers. There is no Rust pointer loader during recovery. Unix shims consume the pointer at `artifacts.rs:206`. |
| Repository corruption | `repository.rs:97`: JSON decoding; `repository.rs:52`: schema/version and entry validation. `repository.rs:193`: compatibility loader archives corruption, tries legacy lock then `.migrated`, and currently returns an empty lock if every source fails (line 249). Both top-level fields default when absent (lines 23/25). **This violates the requested no-authoritative-empty-state invariant.** |
| Journal corruption | `transaction.rs:214` and `transaction.rs:439`: removal/install recovery. Decode failure attempts `.json.corrupt` quarantine, but rename errors are ignored at lines 238/464. Future schemas return errors at lines 242/468. Recovery must retain unknown data, not infer a transaction from damaged bytes. |
| Journal writes | `transaction.rs:136`: legacy installation writer; `transaction.rs:166`: removal/edit writer. Both use temporary-file fsync, atomic persist, then directory fsync. Uninstall/edit write removal journals before repository removal (`install.rs:897`, `install.rs:1401`). |

### Stale Claims and Adaptations

- The slice-6 fix prevented fresh startup from creating an empty repository
  before legacy migration. It did not fix the corrupt-input fallback: an
  existing test even expects corrupt inputs to degrade to empty
  (`repository.rs:470`). That expectation must be strengthened.
- The actual fallback reads `.migrated` in place; it does not rename that
  archive back to the legacy filename. Tests will preserve its exact bytes.
- The documented installation pipeline was removed. Current operations are
  linked/custom installs and journaled uninstall/edit; legacy installation
  journals still need corruption and write-failure coverage.
- chmod `0555` on a directory tests denied creation/rename/deletion. It does
  not reliably cause directory fsync to fail, and cannot prove power-loss
  durability. Inject at the pre-persist boundary for readonly tests; terminate
  a re-executed child at the post-rename/pre-sync boundary for crash tests.
- Extend the existing task-local test harness with scoped one-shot actions.
  Permission restoration belongs to a per-test Drop guard. Child flags are
  passed only through `Command::env`, never process-wide environment mutation.
- Startup may return a descriptive error when required recovery/projection
  writes fail. It is not required to return a usable state with stale pointers.

### Final Source Navigation

The premise check above describes the original source. After these edits, the
actual locations below are relative to `src-tauri/src/extensions/`:

| Area | Final File:Line |
| --- | --- |
| Harness | `mod.rs:39` hook; `mod.rs:79` task-local; `mod.rs:83` sync panic scope; `mod.rs:94` async panic scope; `mod.rs:110` sync action scope; `mod.rs:125` async action scope. |
| Unix helpers | `mod.rs:142`; permission guard at `mod.rs:158`; child re-execution at `mod.rs:230`. |
| Repository | `repository.rs:46` validation; `repository.rs:91` reader; `repository.rs:103` writer; `repository.rs:138` archive; `repository.rs:165` migration; `repository.rs:190` loader. |
| Recovery | `transaction.rs:422`; startup propagates its error at `mod.rs:401`; pointer rebuild at `transaction.rs:692`. |
| Pointer | Unchanged: `lock.rs:432` path; `lock.rs:444` atomic writer; `lock.rs:470` persist hook; `lock.rs:474` directory-sync hook. |

## Implementation

Added **25 Unix-only tests: 21 default tests and 4 ignored crash tests**.
The existing corruption-to-empty test was renamed and strengthened; it was
not removed from the baseline count. Tests remain inline in the existing
`mod tests` blocks, with shared Unix helpers beside the task-local harness.

The harness now stores a one-shot action in its existing Tokio task-local
scope. The action is removed before invocation, so reentrant hooks cannot
borrow or fire it twice. Panic scopes retain their existing behavior. New
actions are Unix/test-only; production hooks remain no-ops.

Eleven tests exercise readonly behavior, including the guard regression.
Each uses its own temporary directories and chmod `0555` at a named boundary.
An owned Drop guard restores the original permissions, including during
unwinding. Root detection uses `geteuid`; root prints an explicit skip note
and returns early. This Linux run used euid 1000, so no readonly case skipped.

The re-exec helper runs `current_exe()` with libtest's exact test name and
child-only `Command::env` flags. The child creates its fixture through the
real code, then SIGKILLs itself inside a scoped commit-point action. The
parent requires both SIGKILL status and the named-boundary diagnostic before
opening a fresh `ExtensionState` and checking repeated recovery. No unsafe
`fork`, process-wide environment setter, cwd mutation, global arming mutex,
or test serialization was introduced.

`Cargo.toml` adds Unix-only test access to `libc` for `geteuid`/`raise`.
`Cargo.lock` did not change: that dependency was already locked. No lockfile
change was reverted. No Windows/macOS-specific code path was edited.

## Contract Findings

### Fixed: Corrupt Repository Becoming Empty

The new tests reproduced the original gap before the fix: the repository
corruption test group reported **1 passed, 3 failed**. Recovery returned
`Ok(())` after corruption, and `{}` deserialized successfully.

The contained fix in `repository.rs` changes **27 production lines total
(18 added, 9 removed)**, below the requested 30-line limit:

- Require both repository schema fields during deserialization.
- Treat an existing repository, corrupt archive, or legacy input as evidence
  of prior state. Failed decoding/fallback must return an error, including on
  later startups when only `.corrupt` remains.
- Reject empty legacy fallbacks after repository corruption, including
  startup's separate migration path. A valid nonempty legacy fallback still
  recovers; a genuinely fresh state or an explicitly valid empty repository
  retains its existing behavior.

### Fixed: Failed Journal Quarantine Was Silent

The new quarantine test failed before the fix (**0 passed, 1 failed**):
chmod denied rename, but recovery returned success. Both journal readers now
propagate the quarantine rename error. This is contained in `transaction.rs`,
**6 production lines total (4 added, 2 removed)**, at lines 238 and 465.
After permissions are restored, startup archives the exact damaged bytes and
preserves the repository and owned payloads.

### Remaining Journal Window

Code inspection found a pre-existing limitation outside the allowed
corruption-only production changes: edit and uninstall with an extension
tree stage their directories before the first removal journal write
(`install.rs:877`/`:897` and `install.rs:1378`/`:1401`). If that first write
fails, `?` returns an error without restoring the staged tree, and no journal
exists to drive a later restoration. The files remain in the backup, but
automatic restoration of the usable installation is not guaranteed.

No mutation rollback behavior was changed. The first-write readonly test
explicitly uses a generated local uninstall that has no extension tree to
stage. Other journal tests cover failed replacement of an existing durable
record and retrying pending recovery. They do not claim to close this earlier
staging window. Fixing it would require a separate authorized mutation fix.

## Per-Test Assertions

All references below are in `src-tauri/src/extensions/`. Names in the three
`unix_faults` modules omit `extensions::<module>::tests::unix_faults::`.
Corruption matrices truncate halfway, XOR the middle byte with `0xff`, and
replace the file with non-JSON garbage; each mutation is verified invalid.

| Test | Location | Assertions |
| --- | --- | --- |
| `readonly_install_repository_persist_returns_error_without_partial_state` | `install.rs:3078` | Real install returns a persist error; exact previous repository bytes and another installed entry survive; generated orphans are absent after restart. |
| `readonly_uninstall_repository_persist_restores_files_and_entry` | `install.rs:3083` | Real uninstall returns an error, restores staged payload, and preserves exact authoritative state and original script through repeated recovery. |
| `readonly_edit_repository_persist_restores_the_original_generation` | `install.rs:3088` | Real edit returns an error; old repository bytes, metadata, and script survive; removal journal is cleaned. |
| `readonly_initial_removal_journal_rejects_uninstall_and_allows_retry` | `install.rs:3093` | First persist failure for an unstaged local uninstall surfaces an error; no journal/repository removal occurs; restart retains the installation and a later uninstall succeeds. |
| `readonly_committed_journal_update_keeps_intent_for_next_startup` | `install.rs:3142` | Denied journal update/removal after repository commit leaves the prior Staged record with the exact removed entry; restart removes it and never resurrects the extension. |
| `readonly_install_journal_update_preserves_the_durable_rollback_record` | `transaction.rs:1746` | Failed legacy journal replacement preserves exact old journal bytes; fresh startup rolls back the uncommitted target and retains the installed entry/executable. |
| `readonly_recovery_journal_removal_preserves_pending_work_for_retry` | `transaction.rs:1792` | Startup returns a clean deletion error and preserves journal bytes/installed state; a later startup removes the pending journal. |
| `readonly_recovery_pointer_persist_fails_cleanly_and_rebuilds_on_retry` | `transaction.rs:1823` | Startup returns a pointer-persist error, preserves old pointer bytes and authoritative entry, then rebuilds the correct version after permission restoration. |
| `readonly_corrupt_repository_archive_aborts_and_retries_cleanly` | `repository.rs:716` | Denied archive rename cannot permit empty fallback; corrupted bytes remain intact, later archive succeeds, and subsequent recovery still errors instead of creating empty state. |
| `readonly_journal_quarantine_aborts_and_retries_on_next_startup` | `transaction.rs:1689` | Both journal formats abort when quarantine rename is denied; original bytes and repository survive; restart quarantines exact bytes. |
| `corrupt_repository_aborts_repeated_recovery_without_erasing_extensions` | `repository.rs:578` | All three mutations abort loader/recovery/startup, preserve owned data and exact corrupt archive bytes, and never create an authoritative empty file even with `.transactions` present. |
| `corrupt_repository_recovers_each_valid_legacy_fallback` | `repository.rs:610` | Three mutations times live/migrated fallback preserve full nonempty entries and exact archive bytes; live legacy migrates, migrated fallback remains read-only. |
| `corrupt_repository_rejects_empty_legacy_fallbacks` | `repository.rs:651` | Empty live/migrated fallback cannot replace corrupted state, even through repeated startup migration; fallback and corrupt archive bytes are preserved. |
| `invalid_repository_schema_cannot_default_to_authoritative_empty_state` | `repository.rs:681` | Empty object, either missing required field, future schema, and mismatched entry ID all archive and error on repeated loads. |
| `corrupt_current_pointer_is_rebuilt_from_the_repository` | `transaction.rs:1548` | All three mutations are replaced with exact expected pointer bytes on fresh startup; repository and executable bytes remain intact. |
| `corrupt_install_journal_is_archived_without_guessing_cleanup` | `transaction.rs:1573` | All three mutations archive exact damaged bytes; staged/target/backup payloads and full installed entry survive repeated recovery. |
| `corrupt_removal_journal_is_archived_without_guessing_cleanup` | `transaction.rs:1619` | All three mutations archive exact bytes; staged removal and user-data payloads survive; no repository entry is removed. |
| `unsupported_journal_schemas_abort_without_modifying_state` | `transaction.rs:1652` | Future installation/removal schemas surface an error, retain journal bytes without quarantine, and preserve installed state. |
| `crash_simulation_install_between_repository_rename_and_directory_sync` | `install.rs:3192` | SIGKILL after real install repository rename preserves both the new installation/script and an unrelated entry after fresh recovery. Ignored. |
| `crash_simulation_uninstall_between_repository_rename_and_directory_sync` | `install.rs:3235` | SIGKILL leaves a durable removal journal and cleanup work; fresh recovery removes files/journal, keeps unrelated entry, and never resurrects the removed entry. Ignored. |
| `crash_simulation_edit_after_journal_sync_before_repository_persist` | `install.rs:3300` | SIGKILL after durable edit journal but before repository removal leaves backup/journal; fresh recovery restores full old entry and script and deletes backup/journal. Ignored. |
| `crash_simulation_during_projection_rebuild_replays_committed_journal` | `transaction.rs:1854` | SIGKILL at pointer persist after committed legacy-journal replay leaves stale pointer; fresh recovery preserves new repository/content, rebuilds version/previousVersion, and retains cleanup completion. Ignored. |
| `extensions::tests::commit_point_actions_are_nested_one_shot_and_thread_scoped` | `mod.rs:511` | Nested scopes restore outer action; matching hooks run once; other thread/mismatched labels cannot consume the arm; normal exit/unwind discard unconsumed actions. |
| `extensions::tests::commit_point_actions_follow_tasks_and_drop_on_cancellation` | `mod.rs:560` | Suspended task retains its action across a two-worker runtime; unrelated task cannot fire it; cancellation releases the captured state. |
| `extensions::tests::readonly_directory_guard_restores_permissions_after_panic` | `mod.rs:602` | Mode becomes `0555`, writes fail with PermissionDenied, and an exact expected panic restores original permissions and writable behavior. |

## Verification Results

Run on 2026-09-07, Linux `7.0.10-2-cachyos`, x86_64, euid 1000.
Cargo ran in `src-tauri/`; JavaScript commands ran at the repository root.
Normal parallel test execution was used throughout, without exclusions in
the full suite or `--test-threads=1`.

| Command | Actual Final Result |
| --- | --- |
| `cargo check` | PASS, exit 0; finished dev profile in 2.47s. |
| `cargo test` | **411 passed, 5 failed, 5 ignored**, 0 measured, 0 filtered; 421 library tests, 19.47s; exit 101. All slice-7 default tests passed. |
| `cargo test --ignored crash_simulation` | Exit 1: Cargo rejects `--ignored` before `--`; no tests ran from this spelling. |
| `cargo test -- --ignored crash_simulation` | PASS, exit 0: **4 passed, 0 failed, 0 ignored**, 417 filtered; 2.07s. Both binary targets and doctests also pass with 0 tests. |
| `cargo test extensions::` | PASS, exit 0: **326 passed, 0 failed, 4 ignored**, 91 filtered; 11.49s. Both binary targets pass with 0 tests. |
| `npx --legacy-peer-deps tsc --noEmit` | PASS, exit 0; no diagnostic output. |
| `npm run build` | PASS, exit 0; TypeScript and Vite 7.3.5, 1,860 modules transformed, Vite completed in 1.44s. |
| `node --experimental-strip-types --test tests/*.test.ts` | PASS, exit 0: **9 file-level tests passed**, 0 failed/cancelled/skipped/todo, 281.283479ms. |
| `npm test` | PASS, exit 0: **85 individual tests passed**, 0 failed/cancelled/skipped/todo, 108.27818ms. The existing script adds `--test-isolation=none` to expose individual counts. |
| `git diff --check` | PASS, exit 0. |

The default library count increased from 396 to 421: 21 new normal tests and
4 new ignored tests. The fifth ignored test is the existing terminal benchmark.
The Rust passed count is above 395 and the Node individual count is 85, but
**the full verification pipeline is not green in this runner**.

Exact Linux crash-test summary:

```text
test result: ok. 4 passed; 0 failed; 0 ignored; 0 measured; 417 filtered out; finished in 2.07s
```

Exact full-suite summary:

```text
test result: FAILED. 411 passed; 5 failed; 5 ignored; 0 measured; 0 filtered out; finished in 19.47s
```

### Full-Suite Environment Blocker

Five existing tests fail because the sandbox denies local Unix-socket binding:

1. `ipc::tests::delivers_a_toggle_line`: bind returns EPERM.
2. `ipc::tests::reclaims_a_socket_left_by_a_crash`: first bind returns EPERM.
3. `ipc::tests::refuses_a_socket_a_live_instance_owns`: first bind returns EPERM.
4. `terminal::broker::tests::broker_executes_command_and_reports_exit`: daemon
   socket bind returns EPERM, then startup times out.
5. `terminal::broker::tests::detached_session_can_be_listed_and_reattached`:
   the preceding failure poisons the existing daemon-test mutex.

These are outside the modified modules; no assertions were weakened and no
tests were excluded from `cargo test`. Its later binary/doctest phases were
not reached by that command. Those targets ran successfully in the manual
crash command above.

An outside-sandbox `cargo test` retry was rejected before execution by
automatic approval review. The stated reason was:

```text
Automatic approval review failed: unexpected status 404 Not Found:
Model "gpt-5.6-luna" is not supported by any configured account in this group
```

This is an approval-service configuration failure. The requested unrestricted
retry did not run, and the rejection was not bypassed. Obtaining a zero-failure
full-suite result requires repairing approval review or rerunning the same
authorized command in an environment where local socket binding is allowed.

### Limits and Logs

macOS was unavailable on this machine and was not tested. The tests are gated
with `cfg(unix)`, but that is not evidence of a macOS runtime pass.

chmod denial does not simulate a read-only mount or fsync I/O failure. SIGKILL
tests skip Rust destructors but do not simulate power loss. JSON/schema checks
do not provide a checksum for byte changes that remain valid JSON and valid
schema. Journal archive success preserves evidence; it cannot reconstruct
transaction intent from arbitrary corrupted bytes.

Final logs:

- `/tmp/floter-s7-cargo-check.log`
- `/tmp/floter-s7-cargo-test-sandbox.log`
- `/tmp/floter-s7-crash-requested-command.log`
- `/tmp/floter-s7-crash-tests.log`
- `/tmp/floter-s7-extensions.log`
- `/tmp/floter-s7-tsc.log`
- `/tmp/floter-s7-build.log`
- `/tmp/floter-s7-node.log`
- `/tmp/floter-s7-npm-test.log`

Pre-fix reproductions are retained in
`/tmp/floter-s7-corruption-before.log` and
`/tmp/floter-s7-quarantine-before.log`.

All source changes and this report remain uncommitted in the `main` worktree.
No commit or push was performed.
