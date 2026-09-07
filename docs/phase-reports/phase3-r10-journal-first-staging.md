# Phase 3 R10: Journal Before Destructive Staging

> Slice 8 update: repository-only reads now match the repository-only writes
> documented here. `write_journal` is compiled only for test fixtures; production
> installation-journal replay remains for older-build crashes. R10 removal/edit
> journal ordering and retry duties remain intact.

## Premise Check (Unmodified Source)

Read `docs/phase-reports/phase3-slice7-advanced-fault-injection.md` first.
Inspected clean `main` on 2026-09-07. The prompt's operation labels were
reversed: the earlier site is edit, the later site is uninstall.
All locations below are relative to `src-tauri/src/extensions/` and refer
to the source before this R10 change.

| Operation / Sweep Result | Actual File:Line and Ordering |
| --- | --- |
| Edit, affected | `install.rs:849`: `update_custom_integration`. Empty backup reservation at `:869` is closed at `:874`. `edit-stage-rename` at `:877`, destructive `integration -> .{id}-editing-*` rename at `:878`, first removal journal at `:897`, `edit-repository-remove` at `:902`, repository save at `:903`. Replacement creation at `:912` reaches `install-repository-add` at `:1670` / save at `:1671`; metadata finalization uses `edit-repository-finalize` at `:923` / save at `:924`; backup/journal cleanup at `:936`. |
| Uninstall, affected | `install.rs:1338`: `uninstall`. Empty reservation at `:1370` is closed at `:1375`. `uninstall-stage-rename` at `:1378`, destructive `extensions/{id} -> .removing-{id}-*` rename at `:1379`, first removal journal at `:1401`, `uninstall-repository-remove` at `:1405`, repository save at `:1406`, committed journal replacement at `:1419`, physical cleanup at `:1423`, journal removal at `:1448`. |
| Sync import, separate existing recovery | `sync.rs:657` copies prepared files non-destructively; `sync-import-backup-rename` at `:659` / existing `sync -> .sync-import-backup-*` rename at `:660`; `sync-import-rename` at `:663` / activation at `:664`; `sync-import-directory-sync` at `:666`; linked install at `:669` reaches `install-repository-add`; `sync-import-backup-remove` at `:690`. There is no installation/removal journal writer on this path. `transaction.rs:625` already discovers these backups and uses the repository manifest path/digest (`:662`) to choose cleanup (`:671`) or restoration (`:682`). Reconciliation accepts matching installed entries as Ready (`sync.rs:542`) and rejects version changes (`:545`), so the backup branch is not an installed-version update path. |
| Shell completion regeneration, derived files | `lifecycle.rs:478` builds replacement completion files without touching the old tree, `:485` moves old completions to `.completions-old-*`, `:488` activates the replacement, `:490` restores on rename error, `:495` syncs. No journal/repository commit or commit-point hooks. These are regenerable lifecycle outputs, not the registered extension manifest/runtime tree; that separate projection swap is outside R10. |
| Descriptor refresh | `install.rs:557` writes a temporary descriptor; `descriptor-rename` at `:559`, atomic replacement at `:560`. The original remains available until replacement; no live-tree-to-backup window or removal journal. |
| Other install/update paths | Custom creation reserves a new root and rejects an existing root (`install.rs:266`); linked installation writes the repository (`:1671`) without moving an installed tree. The staged NPM update writers have been removed (`transaction.rs:71`); `write_journal` at `:136` remains for legacy recovery tests. Repository archives and atomic config/pointer/artifact writes move metadata or replace files, not installed extension trees. Recovery-only renames consume existing journals/backups. |

Both affected journal calls use `transaction.rs:166`: temporary bytes are
flushed/fsynced at `:178`, `removal-journal-persist` at `:183` precedes atomic
persist at `:185`, `removal-journal-directory-sync` at `:187` precedes journal
directory fsync at `:188`. Repository saves use `repository-persist`
(`repository.rs:129`), atomic persist (`:131`), then
`repository-directory-sync` (`:133`) and directory fsync.

Neither mutation directly runs a projection rebuild. Fresh state construction
calls recovery (`mod.rs:401`). `transaction.rs:422` removes orphan installation
staging, loads the repository, replays removal journals (`:431`) then legacy
installation journals (`:432`), recovers sync backups (`:434`), removes generated
orphans (`:435`), and rebuilds projections (`:436`). Projection hooks are
`projection-remove-orphan-data`, `projection-remove-orphan`,
`projection-remove-stale`, artifact/shim persist and directory-sync hooks,
then `current-pointer-persist` / `current-pointer-directory-sync`.

Removal replay already tolerates a planned backup that does not exist:
uninstall restores only when the backup exists and the original is absent
(`transaction.rs:274`), then drops the journal. Edit preserves an existing
repository entry plus its original tree (`:304`) even when the backup is
missing. `staged_path: None` is also accepted (`:252`, `:361`). Repository
presence/generation decides rollback versus cleanup; `RemovalKind::Staged`
alone does not establish whether the repository committed.

## Design

Choose journal-first. Reserve and remove an empty destination directory, write
and sync the removal journal, then move the live tree. A failed first journal
write now occurs while the original installation is intact. An interruption
after journal persistence but before rename leaves an absent planned backup,
which existing recovery handles idempotently. No schema change or new state
file is needed.

Retry must finish pending removal replay before planning another backup, or a
same-ID journal replacement could discard a prior backup's only record. Use
the same removal replay as startup under the mutation lock, and reject a new
mutation when replay still has pending work. Error handling must retain the
journal until rollback/cleanup succeeds, using the persisted repository to
resolve a repository save error that may occur before or after atomic rename.

The requested first-write failure "after tree staging" describes the old
order. Tests will use real movable installation trees and assert that the
first-write boundary is now before the destructive rename. Separate cases
will deny repository writes after actual staging and prove journal-driven
restoration. SIGKILL will cover both sides of journal persistence and staging.

## Per-Test Assertions

Implementation and verification are in progress. Exact cases, final source
navigation, counts, and environment limitations will be recorded here after
the required pipeline completes.

## Verification (completed by coordinator after agent interruption)

The agent's session hit a relay 429 after implementation and tests were
complete; the pipeline below was re-run independently on the uncommitted
tree and passed in full. Counts moved 416→428 passed (+12 default tests,
+2 ignored crash tests).

| Command | Result |
| --- | --- |
| `cargo test` (src-tauri/) | PASS: 428 passed / 0 failed / 7 ignored |
| `cargo test -- --ignored crash_simulation` | PASS: 6 passed / 0 failed (429 filtered) |
| `npx --legacy-peer-deps tsc --noEmit` | PASS |
| `npm run build` | PASS (Vite 7.3.5) |
| `node --experimental-strip-types --test tests/*.test.ts` | PASS: 85/85 |
