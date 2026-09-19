//! User selections. A lock is a binding, not a hint: a missing executable
//! enters reconnect state and is never silently replaced by another candidate.
//!
//! A *fingerprint* change at the same path (the upstream binary was rebuilt or
//! reinstalled) is a recoverable condition, not a failure: once the caller has
//! validated the refreshed provider the binding is silently re-pointed at the
//! new fingerprint via [`resolve_executable_binding`]. Switching to a
//! different executable still requires an explicit reconnect.

use super::inventory::{ToolCandidate, ToolLocator};
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::io::Write;
use std::path::Path;
use std::time::{SystemTime, UNIX_EPOCH};

const SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolLock {
    pub schema_version: u32,
    #[serde(default)]
    pub tools: BTreeMap<String, ToolLockEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolLockEntry {
    pub tool: String,
    pub locator: ToolLocator,
    pub fingerprint: Option<String>,
    pub locked_at: u64,
    pub state: LockState,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum LockState {
    Connected,
    ReconnectRequired,
    ReverifyRequired,
}

impl Default for ToolLock {
    fn default() -> Self {
        Self {
            schema_version: SCHEMA_VERSION,
            tools: BTreeMap::new(),
        }
    }
}

impl ToolLock {
    pub fn load(path: &Path) -> Result<Self, String> {
        if !path.exists() {
            return Ok(Self::default());
        }
        let bytes = std::fs::read(path).map_err(|e| format!("Cannot read tool lock: {e}"))?;
        let lock: Self =
            serde_json::from_slice(&bytes).map_err(|e| format!("Invalid tool lock: {e}"))?;
        if lock.schema_version != SCHEMA_VERSION {
            return Err(format!(
                "Unsupported tool lock schema version {}",
                lock.schema_version
            ));
        }
        Ok(lock)
    }

    pub fn save(&self, path: &Path) -> Result<(), String> {
        let parent = path.parent().ok_or("Invalid tool lock path")?;
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Cannot create tool lock directory: {e}"))?;
        let bytes = serde_json::to_vec_pretty(self).map_err(|e| e.to_string())?;
        let mut temp = tempfile::NamedTempFile::new_in(parent).map_err(|e| e.to_string())?;
        temp.write_all(&bytes)
            .and_then(|_| temp.flush())
            .and_then(|_| temp.as_file().sync_all())
            .map_err(|e| e.to_string())?;
        temp.persist(path).map_err(|e| e.to_string())?;
        #[cfg(unix)]
        std::fs::File::open(parent)
            .and_then(|directory| directory.sync_all())
            .map_err(|e| e.to_string())?;
        Ok(())
    }

    pub fn bind(&mut self, tool: impl Into<String>, candidate: &ToolCandidate) {
        self.bind_locator(
            tool,
            candidate.locator.clone(),
            candidate.fingerprint.clone(),
        );
    }

    pub fn bind_locator(
        &mut self,
        tool: impl Into<String>,
        locator: ToolLocator,
        fingerprint: Option<String>,
    ) {
        let tool = tool.into();
        self.tools.insert(
            tool.clone(),
            ToolLockEntry {
                tool,
                locator,
                fingerprint,
                locked_at: unix_now(),
                state: LockState::Connected,
            },
        );
    }

    pub fn check(
        &mut self,
        tool: &str,
        candidate: Option<&ToolCandidate>,
    ) -> Result<&ToolLockEntry, String> {
        let entry = self
            .tools
            .get_mut(tool)
            .ok_or_else(|| format!("Tool is not locked: {tool}"))?;
        if let ToolLocator::Executable { path } = &entry.locator {
            if !Path::new(path).is_file() {
                entry.state = LockState::ReconnectRequired;
                return Ok(entry);
            }
        }
        if let Some(candidate) = candidate {
            entry.state = if candidate.locator.normalized() == entry.locator.normalized()
                && candidate.fingerprint == entry.fingerprint
            {
                LockState::Connected
            } else {
                LockState::ReverifyRequired
            };
        }
        Ok(entry)
    }

    pub fn reconnect(&mut self, tool: &str, candidate: &ToolCandidate) -> Result<(), String> {
        let entry = self
            .tools
            .get_mut(tool)
            .ok_or_else(|| format!("Tool is not locked: {tool}"))?;
        entry.locator = candidate.locator.clone();
        entry.fingerprint = candidate.fingerprint.clone();
        entry.locked_at = unix_now();
        entry.state = LockState::Connected;
        Ok(())
    }

    /// Refresh the stored fingerprint after the executable at the *same path*
    /// was rebuilt or reinstalled (a version upgrade). The locator is never
    /// changed, so this cannot silently switch to a different tool.
    ///
    /// NOTE: "same path" is a purely *lexical* comparison of
    /// [`ToolLocator::normalized`] values (absolute-ize relative paths and
    /// lowercase on Windows) -- it never calls `canonicalize`. Consequences:
    ///
    /// * A symlink whose link path is unchanged but whose *target* is swapped
    ///   still counts as the same path, so the binding is silently re-pointed
    ///   at whatever the new target provides. This is an accepted residual,
    ///   consistent with the pre-existing `check()` decision model.
    /// * On non-Windows a pure case difference is treated as a *different*
    ///   path (conservative: demands a manual reconnect).
    ///
    /// Returns `Ok(true)` when the binding was refreshed, `Ok(false)` when
    /// nothing changed (a different executable, or an already-connected
    /// candidate), and `Err` when the tool has no binding at all.
    pub fn reconnect_changed_candidate(
        &mut self,
        tool: &str,
        candidate: &ToolCandidate,
    ) -> Result<bool, String> {
        let entry = self
            .tools
            .get_mut(tool)
            .ok_or_else(|| format!("Tool is not locked: {tool}"))?;
        if candidate.locator.normalized() != entry.locator.normalized() {
            return Ok(false);
        }
        if candidate.fingerprint == entry.fingerprint && entry.state == LockState::Connected {
            return Ok(false);
        }
        entry.fingerprint = candidate.fingerprint.clone();
        entry.locked_at = unix_now();
        entry.state = LockState::Connected;
        Ok(true)
    }

    pub fn remove(&mut self, tool: &str) -> Option<ToolLockEntry> {
        self.tools.remove(tool)
    }
}

/// Resolve `binding` against the executable currently at `executable_path`.
///
/// [`LockState::ReverifyRequired`] (same path, changed fingerprint) is the
/// recoverable outcome of an upstream rebuild: when `validate` accepts the
/// refreshed provider the binding is silently re-pointed at the new
/// fingerprint and reported as [`LockState::Connected`]. Every other outcome
/// leaves the persisted binding untouched so callers can fall back to their
/// broken handling.
///
/// This variant may create a *first* binding when the tool has none (a
/// `bind_locator` write). The catalog load path uses it because a connected
/// integration has no other durable place to record its binding. Read paths
/// must use [`resolve_existing_binding`] instead.
///
/// Returns the effective state plus whether the persisted lock changed and
/// should therefore be saved by the caller.
pub(crate) fn resolve_executable_binding(
    lock: &mut ToolLock,
    binding: &str,
    executable_path: &str,
    validate: impl FnOnce() -> Result<(), String>,
) -> Result<(LockState, bool), String> {
    resolve_executable_binding_impl(lock, binding, executable_path, true, validate)
}

/// Read-only variant of [`resolve_executable_binding`]: a tool with no
/// persisted binding is reported as [`LockState::ReconnectRequired`] and the
/// lock is left completely untouched. Used by the list view (and the legacy
/// `check_executable_binding` helper) so a read operation can neither create a
/// first binding nor otherwise mutate `tool-lock.json`.
///
/// The missing entry is *not* stamped into memory either: the catalog load
/// path still owns the lazy first binding, and stamping a bindingless entry
/// here would make a later catalog load see a bogus binding and mark the tool
/// broken. The returned "changed" flag is therefore always `false` for a
/// missing binding.
pub(crate) fn resolve_existing_binding(
    lock: &mut ToolLock,
    binding: &str,
    executable_path: &str,
    validate: impl FnOnce() -> Result<(), String>,
) -> Result<(LockState, bool), String> {
    resolve_executable_binding_impl(lock, binding, executable_path, false, validate)
}

/// Strictly read-only sibling of [`resolve_existing_binding`]: compute the
/// effective [`LockState`] for `binding` from the *live* executable without
/// touching the lock at all — not on disk and not in memory.
///
/// `changed` mirrors what the mutating variants would report: `true` when the
/// persisted binding disagrees with what is on disk (a same-path fingerprint
/// change `validate` accepts, or a state transition such as a removed
/// executable). A read-path caller uses that flag to drop derived caches; it
/// must **not** use it to write. The durable reconcile belongs to the startup
/// pass, the catalog load path and the explicit connect/reconnect/reprobe
/// commands.
///
/// The same-path rebind is still *reported* as [`LockState::Connected`] so the
/// user-visible state never regresses to "reverify" merely because the read
/// path is inert; only the persistence moves.
pub(crate) fn inspect_executable_binding(
    lock: &ToolLock,
    binding: &str,
    executable_path: &str,
    validate: impl FnOnce() -> Result<(), String>,
) -> Result<(LockState, bool), String> {
    let Some(entry) = lock.tools.get(binding) else {
        // No persisted binding: report "unbound" and never a durable change.
        // First bindings belong to the explicit commands / catalog lazy bind.
        return Ok((LockState::ReconnectRequired, false));
    };
    let previous = entry.state;
    let candidate = crate::extensions::inventory::executable_candidate(
        Path::new(executable_path),
        Path::new(executable_path)
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or(executable_path),
    );
    let live = if matches!(&entry.locator, ToolLocator::Executable { path } if !Path::new(path).is_file())
    {
        LockState::ReconnectRequired
    } else if candidate.locator.normalized() == entry.locator.normalized()
        && candidate.fingerprint == entry.fingerprint
    {
        LockState::Connected
    } else {
        LockState::ReverifyRequired
    };
    // A same-path fingerprint change the refreshed provider validates is the
    // silent rebind: report Connected and flag the durable divergence so the
    // caller can drop derived caches. The write itself is deferred.
    if live == LockState::ReverifyRequired
        && candidate.locator.normalized() == entry.locator.normalized()
        && validate().is_ok()
    {
        return Ok((LockState::Connected, true));
    }
    Ok((live, previous != live))
}

fn resolve_executable_binding_impl(
    lock: &mut ToolLock,
    binding: &str,
    executable_path: &str,
    allow_first_binding: bool,
    validate: impl FnOnce() -> Result<(), String>,
) -> Result<(LockState, bool), String> {
    if !lock.tools.contains_key(binding) && !allow_first_binding {
        return Ok((LockState::ReconnectRequired, false));
    }
    let candidate = crate::extensions::inventory::executable_candidate(
        Path::new(executable_path),
        Path::new(executable_path)
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or(executable_path),
    );
    let inserted = !lock.tools.contains_key(binding);
    if inserted {
        lock.bind_locator(
            binding,
            ToolLocator::Executable {
                path: executable_path.to_string(),
            },
            candidate.fingerprint.clone(),
        );
    }
    let previous = lock.tools[binding].state;
    let state = lock.check(binding, Some(&candidate))?.state;
    if state == LockState::ReverifyRequired
        && validate().is_ok()
        && lock.reconnect_changed_candidate(binding, &candidate)?
    {
        return Ok((LockState::Connected, true));
    }
    let changed = inserted || previous != state;
    Ok((state, changed))
}

fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::inventory::{DiscoveryQuality, DiscoverySource};

    fn candidate(path: &str) -> ToolCandidate {
        ToolCandidate {
            id: path.into(),
            name: "tool".into(),
            locator: ToolLocator::Executable { path: path.into() },
            version: None,
            description: None,
            sources: vec![DiscoverySource::Path],
            quality: DiscoveryQuality::AutoDetected,
            available: true,
            fingerprint: None,
        }
    }

    #[test]
    fn default_is_current_schema() {
        assert_eq!(ToolLock::default().schema_version, 1);
    }

    #[test]
    fn a_different_locator_requires_reverification_even_without_fingerprints() {
        let temporary = tempfile::tempdir().unwrap();
        let original = temporary.path().join("original");
        let replacement = temporary.path().join("replacement");
        std::fs::write(&original, "original").unwrap();
        std::fs::write(&replacement, "replacement").unwrap();
        let original = candidate(&original.to_string_lossy());
        let replacement = candidate(&replacement.to_string_lossy());
        let mut lock = ToolLock::default();
        lock.bind("tool", &original);

        let entry = lock.check("tool", Some(&replacement)).unwrap();

        assert_eq!(entry.state, LockState::ReverifyRequired);
    }

    #[test]
    fn save_and_load_preserve_bindings() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        std::fs::write(&executable, "tool").unwrap();
        let candidate = candidate(&executable.to_string_lossy());
        let mut lock = ToolLock::default();
        lock.bind("tool", &candidate);
        let path = temporary.path().join("tool-lock.json");

        lock.save(&path).unwrap();
        let loaded = ToolLock::load(&path).unwrap();

        assert_eq!(loaded.tools["tool"].locator, candidate.locator);
        assert_eq!(loaded.tools["tool"].state, LockState::Connected);
    }

    #[test]
    fn reconnect_replaces_the_explicit_binding_and_remove_unbinds_it() {
        let temporary = tempfile::tempdir().unwrap();
        let original = temporary.path().join("original");
        let replacement = temporary.path().join("replacement");
        std::fs::write(&original, "original").unwrap();
        std::fs::write(&replacement, "replacement").unwrap();
        let original = candidate(&original.to_string_lossy());
        let replacement = candidate(&replacement.to_string_lossy());
        let mut lock = ToolLock::default();
        lock.bind("tool", &original);

        lock.reconnect("tool", &replacement).unwrap();
        assert_eq!(lock.tools["tool"].locator, replacement.locator);
        assert!(lock.remove("tool").is_some());
        assert!(!lock.tools.contains_key("tool"));
    }

    fn executable_candidate(path: &std::path::Path) -> ToolCandidate {
        crate::extensions::inventory::executable_candidate(path, "tool")
    }

    #[cfg(unix)]
    fn write_executable(path: &std::path::Path, contents: &str) {
        use std::os::unix::fs::PermissionsExt;
        std::fs::write(path, contents).unwrap();
        std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o755)).unwrap();
    }

    #[cfg(unix)]
    #[test]
    fn changed_fingerprint_at_the_same_path_is_rebound_to_connected() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");
        let mut lock = ToolLock::default();
        lock.bind("tool", &executable_candidate(&executable));

        // Upstream rebuild: same path, new bytes/mtime.
        write_executable(&executable, "#!/bin/sh\nprintf rebuilt\n");
        let (state, changed) =
            resolve_executable_binding(&mut lock, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();

        assert_eq!(state, LockState::Connected);
        assert!(changed);
        assert_eq!(lock.tools["tool"].state, LockState::Connected);
        assert_eq!(
            lock.tools["tool"].fingerprint,
            executable_candidate(&executable).fingerprint
        );
        assert_eq!(
            lock.tools["tool"].locator,
            executable_candidate(&executable).locator
        );
    }

    #[cfg(unix)]
    #[test]
    fn a_different_locator_is_never_silently_rebound() {
        let temporary = tempfile::tempdir().unwrap();
        let original = temporary.path().join("original");
        let replacement = temporary.path().join("replacement");
        write_executable(&original, "origin");
        write_executable(&replacement, "replacement");
        let mut lock = ToolLock::default();
        lock.bind("tool", &executable_candidate(&original));

        let (state, _) = resolve_executable_binding(
            &mut lock,
            "tool",
            &replacement.to_string_lossy(),
            || Ok(()),
        )
        .unwrap();

        assert_eq!(state, LockState::ReverifyRequired);
        assert_eq!(
            lock.tools["tool"].locator,
            ToolLocator::Executable {
                path: original.to_string_lossy().into_owned()
            }
        );
    }

    #[cfg(unix)]
    #[test]
    fn a_failed_validation_keeps_the_changed_binding_in_reverify() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");
        let mut lock = ToolLock::default();
        lock.bind("tool", &executable_candidate(&executable));
        let stored = lock.tools["tool"].fingerprint.clone();

        write_executable(&executable, "#!/bin/sh\nprintf rebuilt\n");
        let (state, changed) =
            resolve_executable_binding(&mut lock, "tool", &executable.to_string_lossy(), || {
                Err("invalid descriptor".into())
            })
            .unwrap();

        assert_eq!(state, LockState::ReverifyRequired);
        // The binding fingerprint is not refreshed: the stored value still
        // points at the tool the user approved, so the caller falls back to
        // its broken handling. The state transition itself is persisted.
        assert!(changed);
        assert_eq!(lock.tools["tool"].fingerprint, stored);
        assert_eq!(lock.tools["tool"].state, LockState::ReverifyRequired);
    }

    #[test]
    fn a_missing_executable_still_requires_reconnect() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        let mut lock = ToolLock::default();
        lock.bind("tool", &executable_candidate(&executable));

        std::fs::remove_file(&executable).unwrap();
        let (state, _) =
            resolve_executable_binding(&mut lock, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();

        assert_eq!(state, LockState::ReconnectRequired);
    }

    #[test]
    fn an_unbound_tool_is_marked_reconnect_required_without_a_durable_binding() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");
        let mut lock = ToolLock::default();

        let (state, changed) =
            resolve_existing_binding(&mut lock, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();

        assert_eq!(state, LockState::ReconnectRequired);
        // No durable change and no in-memory stamp: the read path is inert.
        assert!(!changed);
        assert!(!lock.tools.contains_key("tool"));

        // A second resolution is stable and still reports no durable change.
        let (state, changed) =
            resolve_existing_binding(&mut lock, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();
        assert_eq!(state, LockState::ReconnectRequired);
        assert!(!changed);
        assert!(!lock.tools.contains_key("tool"));
    }

    #[test]
    fn a_first_binding_is_created_only_when_allow_first_binding_is_true() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");
        let mut lock = ToolLock::default();

        let (state, changed) =
            resolve_executable_binding(&mut lock, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();

        assert_eq!(state, LockState::Connected);
        assert!(changed);
        assert_eq!(lock.tools["tool"].state, LockState::Connected);
        assert!(lock.tools["tool"].fingerprint.is_some());
    }

    /// The strict read path ([`inspect_executable_binding`]) must report the
    /// same effective state as the mutating variants while leaving the lock
    /// byte-identical in memory — no keys added, no fingerprints refreshed, no
    /// state transitions stamped.
    #[cfg(unix)]
    #[test]
    fn inspect_reports_but_never_mutates_the_lock() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");

        // Unbound: reported ReconnectRequired, no durable change, no stamp.
        let lock = ToolLock::default();
        let (state, diverged) =
            inspect_executable_binding(&lock, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();
        assert_eq!(state, LockState::ReconnectRequired);
        assert!(!diverged);
        assert!(!lock.tools.contains_key("tool"));

        // Bound, rebuilt in place: reported Connected (the silent rebind is
        // still user-visible) with `diverged == true`, but the stored
        // fingerprint must remain the one the user approved.
        let mut bound = ToolLock::default();
        bound.bind("tool", &executable_candidate(&executable));
        let approved = bound.tools["tool"].fingerprint.clone();
        write_executable(&executable, "#!/bin/sh\nprintf rebuilt\n");
        let (state, diverged) =
            inspect_executable_binding(&bound, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();
        assert_eq!(state, LockState::Connected);
        assert!(
            diverged,
            "the divergence is reported so the caller can drop caches"
        );
        assert_eq!(bound.tools["tool"].fingerprint, approved);
        assert_eq!(bound.tools["tool"].state, LockState::Connected);

        // A removed executable is reported ReconnectRequired and flagged, but
        // the persisted entry is untouched.
        std::fs::remove_file(&executable).unwrap();
        let (state, diverged) =
            inspect_executable_binding(&bound, "tool", &executable.to_string_lossy(), || Ok(()))
                .unwrap();
        assert_eq!(state, LockState::ReconnectRequired);
        assert!(diverged);
        assert_eq!(bound.tools["tool"].fingerprint, approved);
        assert_eq!(bound.tools["tool"].state, LockState::Connected);
    }

    /// A failed validation must not be reported as a silent rebind: the entry
    /// stays `ReverifyRequired` (the caller's broken handling), exactly as the
    /// mutating sibling decides.
    #[cfg(unix)]
    #[test]
    fn inspect_keeps_a_rejected_refresh_in_reverify() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");
        let mut lock = ToolLock::default();
        lock.bind("tool", &executable_candidate(&executable));
        write_executable(&executable, "#!/bin/sh\nprintf rebuilt\n");

        let (state, diverged) =
            inspect_executable_binding(&lock, "tool", &executable.to_string_lossy(), || {
                Err("invalid descriptor".into())
            })
            .unwrap();
        assert_eq!(state, LockState::ReverifyRequired);
        assert!(diverged);
        assert_eq!(lock.tools["tool"].state, LockState::Connected);
    }
}
