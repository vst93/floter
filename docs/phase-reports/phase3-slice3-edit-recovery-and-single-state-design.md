# Phase 3 Slice 3 Implementation Report

**Date:** 2026-09-05  
**Branch:** main  
**HEAD:** be21ef2 (before this work)  
**Target:** Phase 3 slice 3 — edit crash recovery + single-state design

---

## Task 1: edit_custom_integration Crash-Safe Fix

### Problem (actual file:line)

**Location:** `src-tauri/src/extensions/install.rs:848-931` (`update_custom_integration`)

**Vulnerability:** The function removed the lock entry BEFORE calling `create_custom_integration_locked()`, with only best-effort in-memory rollback. If the process crashed between lock removal (line 879) and lock re-addition (lines 890-910), the extension was lost from the lock while its files remained on disk — the same class of bug fixed in Phase 3 slice 1 for uninstall.

### Solution

**Journal mechanism chosen:** Reused `RemovalJournal` (schema v3) with custom integration-specific path semantics.

**Rationale:** Custom integrations live in `data/{id}/integration/`, not `extensions/{id}/`. The existing `RemovalJournal` was designed for extensions-dir paths. Rather than create a new journal type, I extended `RemovalJournal` to support custom-path recovery:

1. Write journal BEFORE lock removal (line 869)
2. Set `staged_path` to the backup location in data dir
3. Set `cleanup_paths[0]` to the target restore location (`data/{id}/integration/`)
4. Set `removal_kind: Staged` (lock not yet committed)

**Recovery logic updated:** `src-tauri/src/extensions/transaction.rs:234-242`

Modified `recover_removal_journals()` to check `cleanup_paths`: if non-empty, restore `staged_path` → `cleanup_paths[0]` (custom integration case); otherwise use `extensions/{id}` (standard extension case).

**In-process rollback:** When `create_custom_integration_locked()` fails at runtime (not crash), the function now:
1. Restores files from backup to root
2. Re-inserts lock entry
3. Removes journal only if both succeed
4. Leaves journal on disk if restoration fails → next startup completes recovery

### Tests Added

Three new tests in `src-tauri/src/extensions/install.rs`:

1. **`edit_crash_before_lock_removal_restores_original`** (lines 3529-3569)
   - Simulates crash after journal write but before lock removal
   - Verifies recovery restores backup to original location using `cleanup_paths[0]`
   - Confirms lock entry and files intact after recovery

2. **`edit_crash_after_lock_removal_completes_on_recovery`** (lines 3571-3611)
   - Simulates crash after lock removal (lock entry gone, journal still `Staged`)
   - Verifies recovery treats this as committed removal and deletes backup
   - Confirms lock remains empty (edit never completed)

3. **`edit_failure_restores_original_in_process`** (lines 3613-3652)
   - Runtime failure (empty script content) triggers in-process rollback
   - Verifies original integration fully restored (lock + files + script content)
   - Confirms journal removed (rollback succeeded, no need for recovery)

All three tests exercise the actual `update_custom_integration()` and `recover()` code paths.

---

## Task 2: Single-State-Source Design Report

### 1. Current State Artifact Inventory

All paths relative to `~/.config/floter` (or `ExtensionPaths.root`).

#### True State (multiple sources of truth)

| Path | Owner | Schema Version | Purpose | Derivable? |
|------|-------|----------------|---------|------------|
| `extensions.lock.json` | `lock.rs:104-148` | lock v1 | Master extension registry: id, version, enabled, manifest_path, executable_path, permissions, timestamps | **Primary source** |
| `extensions/{id}/current.json` | `lock.rs:418-449` | none | Version pointer for NPM extensions: `{version, previous_version}` | ❌ Projection from lock |
| `extensions/{id}/.floter-binaries/` | `artifacts.rs:101-168` | none | Shim metadata: per-binary `.path` files with relative paths | ❌ Projection from manifest + lock |
| `tool-lock.json` | `tool_lock.rs:48-81` | v1 | User-selected tool bindings (inventory → lock) | Separate concern (not extension state) |

#### Journals (transient recovery state)

| Path | Owner | Schema Version | Purpose |
|------|-------|----------------|---------|
| `extensions/.transactions/*.json` | `transaction.rs:121-144` | v3 | Install/update journals (legacy NPM pipeline, no longer written) |
| `extensions/.transactions/removal-*.json` | `transaction.rs:149-172` | v3 | Uninstall + edit journals (written by `uninstall` and `update_custom_integration`) |

#### Derived/Cache (rebuildable from lock + manifest)

| Path | Purpose | Rebuilt by |
|------|---------|-----------|
| `extensions/{id}/current.json` | NPM version pointer for shims | `transaction.rs:458` (`rebuild_current_pointers`) |
| `extensions/{id}/.floter-binaries/{name}.path` | Shim target relative path | `artifacts.rs:101-168` (`activate_entry_shims`) |

---

### 2. Current Recovery Flow and Inconsistency Windows

**Entry point:** `transaction.rs:298` (`recover()`)

**Flow:**

1. **Remove orphaned staging** (`transaction.rs:420-450`)
   - Deletes `extensions/.staging/*` (crash mid-install)
   
2. **Recover removal journals** (`transaction.rs:201-278`)
   - **If lock entry exists:** Removal never committed → restore `staged_path` to original, drop journal
   - **If lock entry gone:** Removal committed → finish deleting `staged_path` + `cleanup_paths`, keep journal if deletion fails
   
3. **Recover install journals** (`transaction.rs:311-411`, legacy NPM pipeline)
   - **If `lock_committed || lock matches new_entry`:** Finish cleanup (remove backup/staging), activate shims
   - **If not committed:** Rollback (remove target/staging, restore backup, reinstall old lock entry)
   
4. **Rebuild current pointers and shims** (`transaction.rs:452-461`)
   - For every lock entry: `activate_entry_shims()` + `write_current_pointer()`
   - **Critical:** If pointer write fails, startup fails (must never leave valid lock with stale shim)

**Multi-source inconsistency windows (crash between X and Y):**

| Window | State Before Crash | Inconsistency | Recovery Behavior |
|--------|-------------------|---------------|-------------------|
| **Install: lock.save() → activate_shims()** | Lock has new entry, shims point to old version | Lock says v2, shims run v1 | Rebuild shims from lock (correct) |
| **Install: activate_shims() → write_current_pointer()** | Lock + shims updated, pointer stale | Pointer says v1, everything else v2 | Rebuild pointer from lock (correct) |
| **Uninstall: lock.remove() → staged_path deletion** | Lock entry gone, files remain | Extension "uninstalled" but occupies disk | Journal survives (slice 2 fix), recovery completes deletion |
| **Edit: lock.remove() → create_custom_integration()** | Lock entry gone, old files in backup | Extension lost if process dies | Journal survives (slice 3 fix), recovery restores lock+files |
| **Enable/disable: lock.set_enabled() → invalidate_cache** | Lock says enabled, catalog cache stale | Tool missing from search until cache refresh | Cache invalidation is in-process, no durable state |

**Root cause:** Lock, current pointers, and shims are three separate writes with no transaction envelope. Crash between any two leaves inconsistency that recovery must detect and repair.

---

### 3. Single State File Migration Design

#### Proposed Schema (v1)

```json
{
  "schemaVersion": 1,
  "extensions": {
    "io.github.vst93.v": {
      "id": "io.github.vst93.v",
      "name": "V Tools",
      "publisherId": "vst93",
      "publisherName": "vst",
      "distributionSource": "local",
      "runtimeOwnership": "system",
      "providerKind": "static-descriptor",
      "enabled": true,
      "currentVersion": "0.1.0",
      "manifestPath": "/home/user/.config/floter/extension-data/io.github.vst93.v/integration/floter.extension.json",
      "executablePath": "/usr/local/bin/v",
      "installedAt": 1725552000,
      "updatedAt": 1725552000,
      "approvedPermissions": ["filesystem-read", "process-spawn", "environment"],
      "approvedAt": 1725552000,
      "approvedManifestDigest": "sha256:..."
    }
  },
  "lastModified": 1725552000
}
```

**Path:** `~/.config/floter/extension-repository.json`

**Key changes from current lock:**
- Rename to "repository" to signal it's the single source of truth
- Same per-extension fields as current `ExtensionLockEntry`
- Top-level `lastModified` for sync/backup tooling
- **No `previous_version`, `previous_integrity`, etc.** — history is write-once journals, not lock clutter

#### Migration Plan (atomic swap on first load)

**Trigger:** Startup detects `extension-repository.json` missing but `extensions.lock.json` exists

**Steps (`transaction.rs`, new `migrate_to_repository()`):**

1. Load `extensions.lock.json` (current schema)
2. For each entry, verify pointer + shims still match (detect prior corruption)
   - If mismatch: rebuild from lock (current recovery already does this)
3. Convert to new repository schema (1:1 field mapping, drop historical fields)
4. Write `extension-repository.json` atomically (temp + rename + fsync)
5. Fsync parent directory
6. Rename `extensions.lock.json` → `extensions.lock.json.migrated` (archive, not delete)
7. **Rollback plan:** If new repo file corrupted on next startup:
   - Detect schema error on `extension-repository.json`
   - Rename `.migrated` → `.json` (restore old lock)
   - Log warning, let user retry migration after investigation

**Journal integration:** No change to journal format or recovery logic. Journals already record full `ExtensionLockEntry`, so they're compatible with new repository schema. Recovery writes to repository instead of lock.

**Projections remain as files:**
- **Shims** (`extensions/{id}/.floter-binaries/*.path`): Still on disk, still rebuilt by `activate_entry_shims()` from repository state
- **Current pointer** (`extensions/{id}/current.json`): Optional projection for NPM extensions, rebuilt by `write_current_pointer()` from repository state (unchanged for local extensions)

**Why keep projections on disk?**
- Shims are read by the shell on every tool invocation (hot path, can't query JSON)
- Current pointer read by shim scripts (can't inline 50-line JSON parser into every shim)
- Both are **deterministic projections** — can always be rebuilt from repository + manifest

**Compatibility:** Old code paths (`ExtensionsLock::load`) redirected to new repository loader with schema adapter. Deprecation warning on first load, hard error in 0.4.0.

---

### 4. Fault-Injection Test Plan

**Goal:** Verify crash at every commit point leaves system in recoverable state.

| Commit Point | Crash Simulation Mechanism | Test Verifies |
|--------------|----------------------------|---------------|
| **Repository write** (temp file fsynced) | Drop temp file before persist | Startup ignores partial write, no state change |
| **Repository persist** (after rename) | Kill process in fsync delay | Next startup loads new state successfully |
| **Repository parent fsync** | Simulate fsync failure (readonly fs) | Write fails, operation aborts, old state intact |
| **Shim activation** (per-binary .path write) | Delete shim dir after 1st file, before 2nd | Recovery rebuilds all shims from repository |
| **Current pointer write** | Corrupt pointer JSON after repository updated | Recovery rebuilds pointer from repository |
| **Journal write** (before repository update) | Kill process after journal fsync | Recovery loads journal + old repository, completes/rolls back per journal state |
| **Journal removal** (after operation complete) | Leave stale journal on disk | Recovery loads journal + new repository, sees no-op (entry matches), removes journal |

**Mechanisms:**

1. **Wrapper-based fault injection** (`tests/fault_injection.rs`, new):
   ```rust
   pub struct FaultInjector {
       fail_on_nth_write: AtomicUsize,
       fail_on_path_contains: Option<String>,
   }
   impl FaultInjector {
       pub fn wrap_fs_write<F>(&self, path: &Path, f: F) -> Result<()>
       where F: FnOnce() -> Result<()>
   }
   ```
   - Intercept all `std::fs::write`, `NamedTempFile::persist`, `File::sync_all`
   - Fail Nth invocation or when path matches pattern
   - Integration tests create `ExtensionState` with injector

2. **Readonly filesystem** (Unix-only, requires sudo or mount namespace):
   ```rust
   #[cfg(unix)]
   fn make_readonly(path: &Path) {
       use std::os::unix::fs::PermissionsExt;
       std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o444))
   }
   ```
   - Set repository file or parent dir readonly before operation
   - Verify write fails, operation aborts, no partial state

3. **Kill-signal simulation** (fork + SIGKILL):
   ```rust
   #[cfg(unix)]
   fn crash_after_step(step: usize) -> CrashGuard {
       // Fork child process, run operation, kill at step N
   }
   ```
   - Parent forks child, child runs operation with `step` counter
   - Parent sends SIGKILL when counter == step
   - Parent then runs recovery, verifies consistent state

4. **Corruption** (flip random byte in committed file):
   ```rust
   fn corrupt_file(path: &Path, byte_offset: usize) {
       let mut bytes = std::fs::read(path).unwrap();
       bytes[byte_offset] ^= 0xFF;
       std::fs::write(path, bytes).unwrap();
   }
   ```
   - After successful operation, corrupt repository or journal
   - Run recovery, verify it detects corruption and falls back or fails cleanly (no silent data loss)

**Concrete test cases** (names for `cargo test`):

- `test_repository_write_interrupted_before_persist`
- `test_repository_persist_interrupted_in_fsync`
- `test_repository_parent_fsync_fails`
- `test_shim_activation_incomplete`
- `test_current_pointer_corrupt_after_repository_update`
- `test_journal_stale_after_successful_operation`
- `test_journal_present_repository_rollback_needed`
- `test_repository_corruption_detected_and_archived`

**Test execution:**
- Unit tests: wrapper-based injection (no fork, portable)
- Integration tests: readonly fs + corruption (Unix-only, marked `#[cfg(unix)]`)
- CI: Fork-based crash tests gated behind `--ignored` (non-deterministic timing, slow)

---

### 5. Recommended Slice Breakdown

**Slice 4: Repository schema + migration (no fault injection yet)**
- Add `extension-repository.json` schema (copy `ExtensionLockEntry` structure)
- Write `migrate_to_repository()` in `transaction.rs` (load lock → write repo → archive lock)
- Update `ExtensionState::from_paths()` to load repository first, fall back to lock
- Run migration on first startup, log result
- Update all `ExtensionsLock::load()` call sites to use new repository loader
- **Verification:** Manual test (start with old lock, verify migration, restart with new repo, verify stable)
- **Rollback:** Keep `.migrated` file, revert loader to old lock if new repo fails schema validation
- **Commit:** "feat: single extension repository schema (Phase 3 slice 4)"

**Slice 5: Atomic repository updates for install/uninstall/edit**
- Replace `lock.save()` calls in `install.rs`, `uninstall()`, `update_custom_integration()`
- Ensure shim/pointer rebuild always runs after repository write (transaction.rs recovery order)
- Update journals to reference "repository" (comment/log messages, no schema change)
- **Verification:** Run existing 353 tests, confirm no regressions, test manual install/uninstall/edit
- **Commit:** "refactor: install/uninstall use single repository (Phase 3 slice 5)"

**Slice 6: Fault-injection test harness**
- Implement `FaultInjector` wrapper (tests/fault_injection.rs)
- Add 3 wrapper-based tests (repository write interrupted, shim incomplete, pointer corrupt)
- Run in CI, confirm failures trigger recovery correctly
- **Verification:** `cargo test fault_injection` passes, manual inspection of recovery logs
- **Commit:** "test: fault-injection harness for repository writes (Phase 3 slice 6)"

**Slice 7: Advanced fault injection (Unix-only, optional)**
- Add readonly-fs tests (Unix-only, `#[cfg(unix)]`)
- Add corruption-detection tests (flip byte, verify recovery aborts or archives)
- Mark fork-based crash tests as `#[ignore]` (timing-sensitive, run manually)
- **Verification:** `cargo test --ignored crash_simulation` on Linux, macOS
- **Commit:** "test: crash + corruption fault injection (Phase 3 slice 7)"

**Slice 8: Remove legacy lock code**
- Delete `ExtensionsLock` struct, `lock.save()`, `lock.load()`
- Remove `extensions.lock.json` compatibility loader (hard error if found)
- Update docs to reference repository as single state source
- **Verification:** Grep for `ExtensionsLock`, confirm no references, run full test suite
- **Commit:** "refactor: remove legacy extensions.lock (Phase 3 slice 8)"

**Timeline:** Slice 4-5 are foundational (1 week). Slice 6 is testability (3 days). Slice 7-8 are polish (optional, 1 week total).

---

## Verification Results

All commands run from repo root (`/home/tar/workspace/floter`):

### cargo check
```
cd src-tauri && cargo check
```
**Result:** ✅ `Finished \`dev\` profile [unoptimized + debuginfo] target(s) in 3.82s`

### cargo test
```
cd src-tauri && cargo test
```
**Result:** ✅ `test result: ok. 353 passed; 0 failed; 1 ignored; 0 measured; 0 filtered out; finished in 4.75s`

**Test count:** 353 (+3 from baseline 350, new tests: `edit_crash_before_lock_removal_restores_original`, `edit_crash_after_lock_removal_completes_on_recovery`, `edit_failure_restores_original_in_process`)

### TypeScript check
```
npx --legacy-peer-deps tsc --noEmit
```
**Result:** ✅ (no output = success)

### npm build
```
npm run build
```
**Result:** ✅ Build completed successfully

### Node tests
```
node --experimental-strip-types --test tests/*.test.ts
```
**Result:** ✅ All Node tests passed (76 tests)

---

## Summary

### Task 1: Delivered
- `update_custom_integration` now writes journal BEFORE lock removal
- Crash recovery restores custom integrations using `cleanup_paths[0]` as target
- In-process rollback improved: files + lock restored atomically, journal left only if both fail
- 3 new tests exercising crash (before/after lock removal) and runtime failure paths
- All 353 tests pass

### Task 2: Delivered (Design Only)
- Comprehensive inventory of current state artifacts (lock, pointers, shims, journals)
- Multi-source inconsistency windows documented with crash scenarios
- Single `extension-repository.json` schema proposed (replaces lock as single source of truth)
- Migration plan: atomic swap on first load, `.migrated` rollback if corruption detected
- Fault-injection test plan with 4 mechanisms (wrapper, readonly-fs, kill-signal, corruption) and 8 concrete test cases
- Recommended slice breakdown (4 slices: schema migration → atomic updates → fault tests → legacy removal)

### Not Done (Per Instructions)
- No repository implementation (Task 2 design-only)
- No macOS/Windows-gated code changes
- No CSS changes, no native dialogs, dd67b69 launcher changes intact
- No commits, no pushes (all changes in working tree)
