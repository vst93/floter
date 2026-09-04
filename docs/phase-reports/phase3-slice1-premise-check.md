# Premise Check — Phase 3 Slice 1

Verification date: 2026-09-05  
Baseline: commit 40fda3d (main)

## Audit Claim 1: Uninstall ordering bug (P1)
**Audit reference**: Line 135 in `docs/plugin-system-audit.md`  
**Claim**: `install.rs` uninstall (~1244-1317 pre-drift) renames extension dir to staged "removing" temp dir, commits lock FIRST, then deletes staged dir. If deletion fails, it returns error but lock entry is already gone → "extension listed as uninstalled but disk residue remains".

**Current code location**: `src-tauri/src/extensions/install.rs:1306-1382`

**Verification**:
```rust
// Lines 1323-1342: Stage the directory for removal
let source = state.paths.extensions.join(extension_id);
if source.exists() {
    let target = tempfile::Builder::new()
        .prefix(&format!(".removing-{extension_id}-"))
        .tempdir_in(&state.paths.extensions)?;
    std::fs::rename(&source, &target)?;
    moved = Some((source, target));
}

// Line 1343: Remove from lock
lock.extensions.remove(extension_id);

// Lines 1344-1349: COMMIT LOCK FIRST
if let Err(error) = lock.save(&state.paths.lock_file) {
    if let Some((source, target)) = &moved {
        let _ = std::fs::rename(target, source);
    }
    return Err(error);
}

// Lines 1350-1353: Then delete staged directory
if let Some((_, target)) = moved {
    std::fs::remove_dir_all(&target)
        .map_err(|error| format!("Cannot remove {}: {error}", target.display()))?;
}
```

**STATUS**: ✅ **CONFIRMED** — The bug exists exactly as described. Lock is committed at line 1344 before deletion at lines 1350-1353. If `remove_dir_all` fails, the function returns an error but the lock already lacks the entry, leaving disk residue with no recovery path except manual cleanup.

---

## Audit Claim 2: Reinstall lack of contract tests (P1)
**Audit reference**: Lines 69, 123 in `docs/plugin-system-audit.md`  
**Claim**: `install.rs` reinstall (~1062-1091 pre-drift) re-downloads locked version; there is NO contract test proving "reinstall failure keeps the old version working".

**Current code location**: N/A (function removed)

**Verification**: 
- Searched `install.rs`, `commands/extensions.rs` — no `reinstall` function found
- Git log shows commit 350e2d6 (2026-08-24): "refactor: remove dead NPM distribution path from extension backend"
- That commit removed NPM install/update/rollback/reinstall and the entire download pipeline (-5.7k lines)
- Current system only supports local manifest tool connection, no NPM distribution

**STATUS**: ❌ **STALE** — The `reinstall` function and NPM distribution path were completely removed after the audit was written. The audit claim is obsolete. Current codebase has no reinstall operation to test.

---

## Impact on Task Plan

**Task 1 (Uninstall ordering)**: PROCEED as specified. Bug confirmed at `install.rs:1306-1382`.

**Task 2 (Reinstall contract tests)**: SKIP ENTIRELY. No reinstall function exists in current codebase. The NPM distribution system was removed in 350e2d6. Testing a non-existent feature is meaningless.

**Task 3 (Adjacent bug instances)**: PROCEED. Will scan transaction.rs/install.rs for other commit-before-operation patterns.

---

## Adapted Task List

1. ✅ Fix uninstall ordering with transaction journal integration (Task 1)
2. ❌ ~~Reinstall failure tests~~ (obsolete — function removed)
3. ✅ Scan for adjacent instances of same bug class (Task 3)
4. ✅ Run full verification pipeline
5. ✅ Write IMPLEMENTATION_REPORT.md
