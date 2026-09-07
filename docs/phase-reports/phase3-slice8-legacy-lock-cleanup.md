# Phase 3 Slice 8: Legacy Lock Cleanup

Baseline: `main`, `0db2ed9`, initially clean worktree. No commit or push.

## Step 0: Premise Check (Baseline File:Line Evidence)

| Evidence at baseline HEAD | Actual role and decision |
| --- | --- |
| `src-tauri/src/extensions/lock.rs:14`, `:75`, `:196`, `:217`; `repository.rs:21`, `:36`, `:45`, `:117` | `ExtensionsLock` is the live in-memory model, with repository persistence since slice 5. Strictly, the disk envelope is `ExtensionRepository` (schema v1); the adapter retains lock schema v2 and the same `ExtensionLockEntry` serde fields. Deleting the model or its live load/save API would break repository users. Keep these schemas and entry data. |
| `src-tauri/src/extensions/repository.rs:190`, `:195`, `:215`, `:239` | Every load tries repository, live legacy file, then migrated archive. Even callers passing the repository path enter this chain via `lock.rs:163`. Remove this implicit read-time migration/fallback. |
| `src-tauri/src/extensions/repository.rs:165`; `mod.rs:371`, `:376`, `:406` | Startup calls migration, logs and suppresses errors, then calls recovery, which loads/migrates again. A valid current repository takes the no-op branch. A missing repository plus live legacy file is an upgrader or interrupted migration. Keep one explicit recovery entry point; propagate failures before filesystem cleanup. |
| `src-tauri/src/extensions/repository.rs:70`, `:85`, `:103`; `lock.rs:163`, `:178` | Legacy filename detection, sibling repository-path adapter, generic legacy loader, and warning-and-redirect writer are compatibility plumbing. Remove redirects and the generic legacy loader API; isolate legacy decoding within migration. Reject legacy read/write targets explicitly. |
| `src-tauri/src/extensions/repository.rs:137`, `:192`, `:200`, `:239`, `:251` | Invalid repository is archived as `extension-repository.json.corrupt`. `.migrated` is read in place, not renamed back. `.corrupt` is durable evidence of prior state, so repeat startup must fail instead of inventing an empty state if recovery inputs are invalid/empty. Keep corruption quarantine and migration-archive recovery. |
| `src-tauri/src/extensions/transaction.rs:450`, `:455`, `:457`, `:574`, `:464` | `recover()` loads through the fallback chain; a `.migrated` fallback need not be persisted unless journals cause a save. A pre-slice-7 startup can therefore leave only `.migrated` and `.corrupt`. Explicit recovery must publish the recovered repository durably before journals or projection cleanup run. |
| `src-tauri/src/extensions/transaction.rs:71`, `:136`; `install.rs:1929`, `:1978`; `transaction.rs:904`, `:1005`, `:1721`, `:2004` | Staged NPM writers were removed in `350e2d6`. All `write_journal` callers are inside test modules, but the writer is still compiled in production with `allow(dead_code)`. Make the fixture writer test-only. |
| `src-tauri/src/extensions/transaction.rs:460`, `:468`, `:523`, `:542`, `:556`, `:565` | Installation-journal replay is live production recovery. It identifies committed installs, discards pre-staging attempts, restores an old version backup and entry for interrupted swaps, and rebuilds shims. An upgrade after an old-build crash can still need it. Keep replay, schema, state enum, and wire field `lock_committed`. Removal journals do not record old/new install versions or installation backup paths. |
| `src-tauri/src/extensions/transaction.rs:99`, `:165`, `:222`; `install.rs:893`, `:1372` | Removal/edit journals protect current operations, including R10 journal-before-staging. Keep them and their durability/retry behavior. |
| `src-tauri/src/extensions/lock.rs:243`, `:253`, `:275`, `:303`, `:333`, `:364`, `:386`, `:404`, `:432`, `:444`, `:479`, `:491` | Beyond entry/pointer types, this module owns listing/lookup, state and release-policy mutation, broken-state recovery, permission approvals, ID/version/channel validation, and directory fsync. These are live repository behavior. Keep them. `save_legacy` at `:223` is already test-only. |
| `src-tauri/src/extensions/mod.rs:301`, `:320`; `commands/extensions.rs:34`; `extensions/install.rs:479`; `sync.rs:126`; `catalog.rs:429`; `config.rs:130`; `transaction.rs:211` | Production readers still use `paths.lock_file`. Move all callers to `paths.repository_file` and label the old path explicitly as a migration input. |
| `src-tauri/src/extensions/asset_matcher.rs:1`; `lock.rs:101` | Asset-selection metadata survives in migrated repository entries and journals, not just old lock files. Keep the types and correct their module comment. |
| `src-tauri/src/extensions/config.rs:701`, `:745` | `load_legacy_secrets` migrates configuration secrets, not extension registry state. Outside this cleanup. |

### Full Search Inventory

Searched `src/`, `src-tauri/src/`, and `docs/` for
`extensions.lock.json`, `.migrated`, `load_legacy`, and `load_for_legacy_path`,
and searched Markdown for lock-file/source-of-truth claims. No frontend `src/`
matches. Source matches are confined to the modules above plus legacy fixtures
in `repository.rs`, `lock.rs`, `transaction.rs`, `install.rs`, `sync.rs`, and
`commands/extensions.rs`. The literal `example.migrated` in transaction tests
is an extension ID, not a file or fallback API.

Documentation inventory (baseline lines):

- `docs/plugin-system-audit.md:22`, `:80`, `:87`: extension state path and pointer source.
- `docs/DEVELOPMENT_PLAN.md:388`, `:416`, `:432`, `:439`, `:829`, `:900`: persistent state and module descriptions.
- `docs/extensions/FEP-3-lifecycle.md:3`, `:13`, `:34`, `:40`: outdated state/transaction contract.
- `docs/extensions/FEP-1-package.md:76`, `FEP-4-npm-registry.md:85`, `:202`, `FEP-5-permissions.md:63`, `:117`: registration, version/rollback metadata, internal state, and approval record terminology.
- `docs/phase-reports/phase3-slice3-edit-recovery-and-single-state-design.md:72`, `:169`, `:179`, `:182`, `:196`, `:286`, `:291`, `:315`: historical dual-source inventory, migration proposal, and stale slice-8 deletion instructions.
- `docs/phase-reports/phase3-slice4-repository-migration.md:12`, `:25`, `:30`, `:40`: historical live-lock and fallback claims.
- `docs/phase-reports/phase3-slice7-advanced-fault-injection.md:16`, `:26`, `:97`, `:149`: historical fallback/corruption behavior and assertions.
- `docs/phase-reports/phase3-r10-journal-first-staging.md:18`: installation writer retained for tests.
- Earlier slice-1/2 and Phase-2 reports use "lock" for what is now repository state; annotate their historical context without rewriting their original findings.

## Migration Decision

Keep one explicit migration/recovery entry point. Migration landed in
`c451b24` on 2026-09-05; repository-only writes landed in `05403c9` on
2026-09-06; corruption hardening landed in `69d612a` on 2026-09-07. The user
builds this personal-use app from pushed commits. A commit being available
does not establish that the installed app has run it; builds may skip slices.
There is no evidence that every installed tree is post-migration. Hard-erroring
all legacy-only trees would unnecessarily strand valid installed extensions.

Normal load/save will address only `extension-repository.json`. Recovery may
import an old lock or `.migrated` archive, but must commit the repository before
returning state to consumers. A valid repository always wins, including an
empty repository beside stale legacy files. Migration write errors must abort
startup, naming the input file. Corrupt-state evidence must prevent empty
recovery on every retry. The legacy installation replay and archive recovery
are retained for crash safety, not treated as dead code.

## Reachability And Safety Decisions

At the baseline, a current startup with a valid repository invokes migration
at `mod.rs:371` (validation/no-op), then `recover()` at `:406` and the fallback
loader at `transaction.rs:455`. It returns at `repository.rs:197`; no legacy
file is read. A stale live lock beside that valid repository is ignored,
including when the repository intentionally contains zero extensions.

The legacy input branches were not exclusively upgrader code: corruption of a
current repository can reach them too. The raw-file import covers a user who
skipped migration builds and a migration interrupted before archiving the old
file. The `.migrated` branch covers interrupted or older-build corruption
recovery. With no journal directory, baseline recovery could use the archive
without publishing a repository at all. Therefore deleting archive recovery,
or installation-journal replay, would remove a live recovery duty. Those parts
of the proposed removal were stopped and retained.

Recovery still accepts valid nonempty live legacy input after corruption. An
old migration could commit the repository and crash before archiving its input;
that raw file may be its only surviving valid migration copy. This is handled
by the same explicit importer as archived recovery. No normal loader returns
either input, and no failed repository write permits recovery to continue on
in-memory legacy state. Invalid/empty inputs after corruption cannot replace
the repository; the original inputs and `.corrupt` remain for retry.

## Implementation

- Removed `load_for_legacy_path`, `repository_path`, `is_legacy_lock_path`, the
  generic `ExtensionsLock::load_legacy` API, and load/save filename redirects.
  Both normal load and save reject any target other than the repository file,
  naming the supplied path. No code writes a legacy state file in production.
- All command, catalog, configuration, install, sync, and transaction readers
  pass `paths.repository_file`. Renamed `ExtensionPaths.lock_file` to
  `legacy_lock_file` and documented its migration-only role.
- `repository.rs:159` implements repository-only loading. Invalid repository
  bytes are quarantined; missing state with any legacy/corrupt evidence errors
  instead of resetting the registry. Filesystem inspection errors propagate.
- `repository.rs:196` is the private migration decoder. It validates schema,
  IDs, and entries and includes the input path in decoding/validation errors.
- `repository.rs:230` retains the one migration/recovery entry point.
  `transaction.rs:449` calls it before staging cleanup, journal replay, or
  projection cleanup. The duplicated, error-suppressing startup migration call
  in `mod.rs` is gone. A source imported successfully must be atomically
  persisted and directory-synced before consumers can load it.
- Failure to archive the live input after a successful repository fsync is
  logged; startup can use the committed repository. This is safe coexistence,
  not fallback. Recovery reads an existing `.migrated` input in place and keeps
  its bytes. Existing archive replacement when migrating a live file retains
  the prior archive policy.
- `transaction.rs:133` now gates `write_journal` with `#[cfg(test)]`, replacing
  `allow(dead_code)`. Replay, the installation wire schema/stages, and
  `lock_committed` remain production code. `save_legacy` was already test-only.
- Kept `ExtensionsLock`, `ExtensionRepository`, `ExtensionLockEntry`, approvals,
  state transitions, removal/edit journals, current pointers, and shims. No
  schema or state-file additions; no installation replay ordering changes.
- Corrected module docs, the plugin audit, development plan, FEPs, and phase
  reports. Historical reports explicitly distinguish their baseline claims
  from the current repository-only contract. All documentation-sweep edits
  are `.md` files under `docs/`; schemas, SDK templates, examples, and assets
  there are untouched. This root report is the separately requested deliverable.

## Per-Test Assertions

Six new default Rust tests; no tests removed. The library inventory grows
from 435 (428 default + 7 ignored) to 441 (434 default + 7 ignored).
All legacy fixture consumers outside repository tests now load the repository;
their existing entry, archive-byte, pointer, and recovery assertions remain.

Repository tests live in `src-tauri/src/extensions/repository.rs`; full test
names below identify each case without depending on shifting test line numbers.

| Test | Assertions |
| --- | --- |
| `legacy_only_tree_requires_startup_migration` (updated) | Ordinary load errors naming the legacy file without creating a repository or changing the input. Real startup imports all entry fields, archives exact bytes, preserves installed content, and a second startup leaves repository bytes stable. |
| `migration_and_loader_keep_repository_authoritative_beside_legacy` (updated) | Legacy-path load is an error. Both nonempty and intentionally empty repositories win over stale legacy state; stale bytes remain unchanged. |
| `legacy_read_and_write_targets_are_errors_without_changing_state` (updated) | Raw, migrated, and arbitrary JSON targets error with their names for both load/save. Repository bytes/existence and legacy bytes stay unchanged; no alternate file is created. |
| `startup_uses_committed_repository_when_migration_archive_fails` (updated) | An archive-directory collision cannot enable ordinary fallback; startup commits usable repository state. A subsequent empty repository still wins; live legacy bytes remain unchanged. |
| `loader_requires_explicit_recovery_of_corrupt_repository_from_archive` (updated) | Normal reads fail repeatedly and preserve the archive. Startup imports its complete entries into a real repository; quarantine survives, source bytes stay identical, and a repeat startup is byte-stable. |
| `startup_ignores_stale_and_invalid_legacy_files_beside_repository` (new) | Nonempty/empty repository crossed with valid/invalid stale lock, plus invalid archive: repeated startup preserves exact repository and input bytes and never resurrects stale entries. |
| `startup_errors_name_invalid_migration_inputs_and_preserve_installed_files` (new) | Invalid JSON, unsupported schema, and mismatched IDs in raw/archive inputs all produce named startup errors on repeated attempts; no repository is created and both installed integration and staging bytes survive. |
| `startup_commits_archive_only_state_left_by_pre_slice7_recovery` (new) | Archive-only and archive-plus-corrupt-marker trees both recover without journals. Full entries and installed files survive; a durable repository exists, repeated startup is byte-stable, archive/corrupt bytes remain unchanged, and no raw lock reappears. |
| `startup_migrates_valid_empty_pre_repository_state` (new) | Legitimately empty pre-repository state migrates, remains empty on repeated startup, and archives its exact input bytes. |
| `migration_commit_faults_preserve_inputs_and_recover_on_restart` (new) | Scoped panic at repository persist, repository directory sync, live-input archive rename, and archive directory sync hits the exact named hook. Raw/archive inputs survive and fresh repeated startup restores every entry. Archived inputs exercise both repository commit boundaries. |
| `migration_write_failure_aborts_startup_before_cleanup_and_retries` (new, Unix-only) | chmod denial at repository persist errors naming input/output; raw/archive bytes, installed content, staging, and corrupt evidence survive. No repository is published on failure; permission restoration allows complete, repeatable recovery. |
| `corrupt_repository_recovery_commits_each_valid_migration_input` (updated) | Truncated/flipped/garbage repository crossed with live/archive input: explicit recovery retains all fields and exact archived/corrupt bytes. Strengthened to require an actual repository for both input kinds. |
| `migrate_from_legacy_lock_is_atomic_and_idempotent` (retained) | Full entry equality, archived input, and migration no-op on repeat. |
| `repository_save_load_round_trip_without_legacy_files` (retained) | Repository schema version/full model round-trip; no legacy files and exactly one state file. |
| `invalid_repository_and_legacy_lock_abort_loading` (retained) | Repeated loading errors and durable corruption quarantine. |
| `fresh_state_recovery_does_not_preempt_legacy_migration` (retained) | Empty first startup creates neither repository nor journal directory; later explicit legacy migration preserves entries and exact archive bytes. |
| `fault_at_legacy_archive_rename_keeps_repository_authoritative` (retained) | Archive-rename crash leaves both authoritative repository and legacy input; repeat migration is a no-op. |
| `corrupt_repository_aborts_repeated_recovery_without_erasing_extensions` (retained) | All three corruption modes reject repeated recovery/startup, retain exact quarantine and installed bytes, and never create empty state. |
| `corrupt_repository_rejects_empty_legacy_fallbacks` (retained) | Empty live/archive input cannot replace corrupt repository on repeated recovery/startup; all evidence is preserved. The name describes the rejected historical behavior. |
| `invalid_repository_schema_cannot_default_to_authoritative_empty_state` (retained) | Missing schema/extensions, future version, and mismatched IDs error on repeat; exact invalid bytes are quarantined. |
| `readonly_corrupt_repository_archive_aborts_and_retries_cleanly` (retained) | Denied quarantine preserves original corrupt bytes; later quarantine succeeds but recovery still errors without valid input. |
| `lock.rs::legacy_lock_file_with_populated_npm_entry_migrates` (updated) | Ordinary legacy load errors; explicit migration preserves NPM source/name/version, all integrity fields, and signature/official flags. |

Existing installation and removal replay tests, R10 retry tests, all six
SIGKILL crash simulations, and task-local isolation tests remain intact.
No test-infrastructure global state, environment mutation, or serialization
was added. The new permission-denial test is inside `#[cfg(unix)]`; no
Windows/macOS-specific production or test code was changed.

## Verification Results

| Command | Result |
| --- | --- |
| `cd src-tauri && cargo check` | PASS, exit 0. |
| `cd src-tauri && cargo test` | **429 passed, 5 failed, 7 ignored**, 0 measured/filtered; 441 total. All five failures are the known Unix-socket sandbox limitation, detailed below. No extension test failed. |
| `cd src-tauri && cargo test -- --ignored crash_simulation` | **6 passed, 0 failed, 0 ignored, 435 filtered out**. Binary/doc-test targets each ran 0 tests successfully. |
| `npx --legacy-peer-deps tsc --noEmit` | PASS, exit 0. |
| `npm run build` | PASS, exit 0; Vite transformed 1,860 modules and produced both application and clipboard-page bundles. |
| `node --experimental-strip-types --test tests/*.test.ts` | PASS, exit 0, but this Node v24.13.0 environment reports **9 file-level tests**, 0 failed/skipped, rather than individual assertions. Not counted as 85. |
| `node --experimental-strip-types --test --test-isolation=none tests/*.test.ts` | **85 passed, 0 failed, 0 skipped/cancelled/todo**, exit 0. Added verification after the file-level reporting discrepancy; no source or test-runner configuration changes. |
| `cargo test extensions::repository::` (focused, before full pipeline) | **21 passed, 0 failed, 0 ignored, 420 filtered out**. |
| `cargo test legacy_read_and_write_targets_are_errors_without_changing_state` (final assertion check) | **1 passed, 0 failed, 0 ignored, 440 filtered out**. Confirms an added explicit repository-existence assertion after the full run; no production behavior changed after that run. |
| `git diff --check` | PASS. Documentation paths checked: only `.md` under `docs/`; no dependency/lockfile changes. |

The expected default Rust inventory is 434 (six more than the 428 baseline).
The sandbox actually executed 429 successfully, above the requested floor;
the other five are reported as failures, not claimed as passes. The Node
85-test floor is verified by the explicit no-isolation run.

### Sandbox And Approval Caveats

The full Rust run failed only at:

- `ipc::tests::delivers_a_toggle_line`: `src/ipc.rs:248`, Unix bind EPERM.
- `ipc::tests::reclaims_a_socket_left_by_a_crash`: `src/ipc.rs:267`, bind EPERM.
- `ipc::tests::refuses_a_socket_a_live_instance_owns`: `src/ipc.rs:279`, bind EPERM.
- `terminal::broker::tests::broker_executes_command_and_reports_exit`:
  `src/terminal/broker.rs:975`, bind EPERM; daemon startup then fails at `:984`.
- `terminal::broker::tests::detached_session_can_be_listed_and_reattached`:
  `src/terminal/broker.rs:1056`, poisoned mutex after the denied broker bind.

Attempted the same authorized `cargo test` outside the sandbox to verify these
five. **Automatic approval review rejected that action** because its review
service returned HTTP 404: model `gpt-5.6-luna` was unsupported by the configured
account group. The test command never started outside the sandbox. No workaround
or bypass was attempted; the known environment limitation remains unverified
outside the sandbox. This is an approval-service failure, not a source/test error.

The separate Node discrepancy is observed, not attributed to a proven root
cause: the requested process-isolated command reports nine file wrappers, while
direct execution of the two-test frozen-NPM file reports two tests and the
no-isolation suite reports all 85 named tests. No test coverage was removed.

Logs: `/tmp/floter-slice8-cargo-test.log`,
`/tmp/floter-slice8-crash-tests.log`, `/tmp/floter-slice8-node-tests.log`, and
`/tmp/floter-slice8-node-tests-no-isolation.log`.

All changes remain uncommitted on `main`. No push, dependency changes, new
extension state files, or changes to the seven ignored-test declarations.
