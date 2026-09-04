# Implementation Report — Manifest Load Fallback Hardening

## Baseline Health Check (before fixes)

All pipeline commands passed on commit dd67b69:

```
✓ cargo check         — passed
✓ cargo test          — 343 passed, 1 ignored
✓ tsc --noEmit        — passed
✓ npm run build       — passed
✓ node tests          — 76 passed
```

No failures found in the maintainer's manual commit. The codebase was healthy at baseline.

## Baseline Health Check (after fixes)

All pipeline commands pass after implementing the fallback hardening:

```
✓ cargo check         — passed
✓ cargo test          — 346 passed, 1 ignored (+3 new tests)
✓ tsc --noEmit        — passed
✓ npm run build       — passed
✓ node tests          — 76 passed
```

## Files Changed

### `src-tauri/src/commands/extensions.rs`

**1. `extensions_reprobe` (lines 1639-1691)**
- **Reason**: Wrapped `ExtensionManifest::load()` in a `match` to handle missing/corrupt manifests gracefully
- **Fallback behavior**: Synthesizes a minimal manifest with empty `lifecycle.probes` so `probe_executor` uses the `--version`/`--help` fallback (backward-compatible default)
- **Warning path**: Logs extension id, manifest path, and error via `tracing::warn!` before falling back
- **Previously-working case preserved**: Extensions that launched before manifests declared lifecycle probes continue to work

**2. `extensions_launch` (lines 1728-1739)**
- **Reason**: Wrapped `ExtensionManifest::load()` in a `match` to handle missing/corrupt manifests gracefully
- **Fallback behavior**: Sets `launch_config = None`, which triggers pre-a02dab0 defaults:
  - `cwd_policy` = `InheritActiveSession`
  - `restore_policy` = `Reattach`
  - Standard terminal config (no custom overrides)
- **Warning path**: Logs extension id, manifest path, and error via `tracing::warn!` before falling back
- **Previously-working case preserved**: Extensions that launched before a02dab0 introduced manifest-based lifecycle configuration continue to work with the same defaults they used before

**3. Added ownership fixes (lines 1765-1807)**
- **Reason**: Used `.as_ref()` when accessing `launch_config` multiple times to avoid moving the `Option<LaunchConfig>`
- **No functional change**: This is a compilation fix to support the fallback pattern

## Manifest-Load Failure Modes Covered

### Missing manifest file
- **Detection**: `std::fs::read()` returns IO error
- **Fallback**: `extensions_launch` → pre-a02dab0 defaults; `extensions_reprobe` → `--version`/`--help` probes
- **Test**: `extensions_launch_proceeds_with_missing_manifest`, `extensions_reprobe_synthesizes_fallback_manifest_when_load_fails`

### Corrupt/unparseable manifest file
- **Detection**: `serde_json::from_slice()` returns parse error
- **Fallback**: Same as missing manifest
- **Test**: `extensions_launch_proceeds_with_corrupt_manifest`

### Previously-working launch/reprobe preserved
Both functions used to work without requiring a valid manifest:
- `extensions_launch` (before a02dab0): no manifest load at all, hardcoded defaults
- `extensions_reprobe` (before ac3d09d): only needed executable path, not manifest lifecycle config

After the fix, both functions continue to work in those same cases.

## Other Manifest-Load Sites (NOT changed)

The following `ExtensionManifest::load()` sites were examined and deliberately NOT changed, as they are part of flows where a missing/corrupt manifest is correctly a hard error:

1. **`extensions_list` (line 52)**: Uses `.ok()` already (graceful)
2. **`extensions_refresh_official_status` (line 239)**: Uses `.ok()` already (graceful)
3. **`extensions_list` (line 294)**: Uses `.ok()` already (graceful)
4. **`extensions_pick_local_package` (line 1047)**: User is actively installing a new package; manifest validation is required before presenting the confirmation dialog
5. **`extensions_local_manifest_review` (line 1070)**: User is reviewing a manifest for installation; load failure is correctly an error
6. **`reconnect_system` (line 1262)**: Reconnecting a system tool requires the manifest to validate the executable matches the runtime declaration; this is a repair operation, not a previously-working launch

## Tests Added

Three new Rust unit tests in `src-tauri/src/commands/extensions.rs`:

1. **`extensions_launch_proceeds_with_missing_manifest`**  
   Verifies that when the manifest file is missing, `extensions_launch` produces `launch_config = None` and falls back to `InheritActiveSession` cwd policy and `Reattach` restore policy (the pre-a02dab0 defaults).

2. **`extensions_launch_proceeds_with_corrupt_manifest`**  
   Verifies that when the manifest file is unparseable, the same fallback behavior applies.

3. **`extensions_reprobe_synthesizes_fallback_manifest_when_load_fails`**  
   Verifies that when the manifest load fails during `extensions_reprobe`, a minimal manifest with empty `lifecycle.probes` is synthesized so `probe_executor` uses the `--version`/`--help` fallback.

**Test floor maintained**: 343 → 346 cargo tests (+3), 76 node tests (unchanged).

## What Was NOT Done

1. **No changes to platform-gated code**: `#[cfg(target_os = "macos")]`/windows code was not touched.
2. **No CSS changes**: Styling was out of scope.
3. **No changes to dd67b69 manual rework**: The launcher height animation and plugin-page sandbox rework were left intact.
4. **No changes to other optional-data loads**: Only the two manifest loads introduced/affected by a02dab0 (in `extensions_launch`) and ac3d09d (in `extensions_reprobe`) were hardened. Other optional-data loads in the codebase were either already graceful (using `.ok()`) or correctly require the data (installation/validation flows).

## Graceful-Degradation Gate Applied

Every `?` operator added in the diff was checked:
- No new bare `?` operators were introduced on optional-data loads
- The two manifest loads that had bare `?` (from a02dab0 and ac3d09d) were replaced with `match` arms that log warnings and fall back to pre-existing defaults

The graceful-degradation regression class introduced by a02dab0's bare `?` has been fixed.
