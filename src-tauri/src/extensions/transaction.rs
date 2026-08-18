//! Crash-consistent installation transactions.
//!
//! Every install/update/rollback flows through a journal that is fsynced before
//! any visible state changes. The journal records which transaction stage was
//! reached, so a crash at any point can be resolved on the next startup
//! ([`recover`]) without guessing: uncommitted transactions restore the
//! previously active version, committed transactions finish their cleanup, and
//! the `current.json` pointer is rebuilt from the lock.
//!
//! The stage machine is `resolved -> downloading -> downloaded -> verified ->
//! staged -> activated -> cleaned` (FEP/plan "确定性安装"). Download itself is
//! resumable via `.part` files with an independent journal (see `download.rs`);
//! the transaction journal only records which stage the install pipeline
//! reached before it stopped.

use crate::extensions::lock::{
    sync_directory, write_current_pointer, ExtensionLockEntry, ExtensionsLock,
};
use crate::extensions::ExtensionState;
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::{Path, PathBuf};

pub const TRANSACTION_JOURNAL_SCHEMA_VERSION: u32 = 2;

/// Number of installed versions kept on disk besides the current one: the
/// current version plus one previous version survive an update, so rollback
/// only switches the pointer and never needs to re-download.

/// Stage of an installation transaction. Older journals (schema v1) did not
/// carry a stage; they are treated as [`TransactionState::Resolved`] and the
/// pre-existing `lock_committed` flag decides their recovery branch.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum TransactionState {
    /// Version selection finished; nothing has been downloaded yet.
    Resolved,
    /// Tarball download in progress (resumable via `download.rs` `.part`).
    Downloading,
    /// Downloads finished and integrity verified.
    Downloaded,
    /// Archive unpacked, manifest validated, provider described, probes run.
    Verified,
    /// Staging is complete and ready to be atomically activated.
    Staged,
    /// Version directory swapped and lock committed; cleanup remains.
    Activated,
    /// Backup/retained-version cleanup finished; journal may be removed.
    Cleaned,
}

impl TransactionState {
    fn may_transition_to(self, next: Self) -> bool {
        use TransactionState::*;
        matches!(
            (self, next),
            (Resolved, Downloading)
                | (Downloading, Downloaded)
                | (Downloaded, Verified)
                | (Verified, Staged)
                | (Staged, Activated)
                | (Activated, Cleaned)
        )
    }
}

impl Default for TransactionState {
    fn default() -> Self {
        Self::Resolved
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InstallationJournal {
    pub schema_version: u32,
    pub transaction_id: String,
    pub extension_id: String,
    pub old_entry: Option<ExtensionLockEntry>,
    pub new_entry: ExtensionLockEntry,
    pub staged_version: Option<PathBuf>,
    pub target_version: Option<PathBuf>,
    pub backup_version: Option<PathBuf>,
    #[serde(default)]
    pub lock_committed: bool,
    #[serde(default)]
    pub cleanup_paths: Vec<PathBuf>,
    /// Transaction pipeline stage (schema v2+; v1 journals default to Resolved).
    #[serde(default)]
    pub state: TransactionState,
}

fn journal_dir(state: &ExtensionState) -> PathBuf {
    state.paths.extensions.join(".transactions")
}

pub(crate) fn write_journal(
    state: &ExtensionState,
    journal: &InstallationJournal,
) -> Result<PathBuf, String> {
    let directory = journal_dir(state);
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("Cannot create extension transaction journal: {error}"))?;
    let path = directory.join(format!("{}.json", journal.transaction_id));
    let bytes = serde_json::to_vec_pretty(journal)
        .map_err(|error| format!("Cannot serialize extension transaction journal: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(&directory)
        .map_err(|error| format!("Cannot create extension transaction journal: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write extension transaction journal: {error}"))?;
    temporary
        .persist(&path)
        .map_err(|error| format!("Cannot persist extension transaction journal: {error}"))?;
    sync_directory(&directory)
        .map_err(|error| format!("Cannot sync extension transaction journal: {error}"))?;
    Ok(path)
}

fn remove_journal(path: &Path) -> Result<(), String> {
    if path.exists() {
        std::fs::remove_file(path)
            .map_err(|error| format!("Cannot remove extension transaction journal: {error}"))?;
        if let Some(parent) = path.parent() {
            sync_directory(parent)
                .map_err(|error| format!("Cannot sync extension transaction journal: {error}"))?;
        }
    }
    Ok(())
}

/// Start a journaled installation for `extension_id`. The returned journal is
/// already persisted in the `Resolved` stage.
pub(crate) fn begin(
    state: &ExtensionState,
    extension_id: &str,
    old: Option<&ExtensionLockEntry>,
) -> Result<InstallationJournal, String> {
    let journal = InstallationJournal {
        schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
        transaction_id: uuid::Uuid::new_v4().to_string(),
        extension_id: extension_id.to_string(),
        old_entry: old.cloned(),
        new_entry: placeholder_entry(extension_id),
        staged_version: None,
        target_version: None,
        backup_version: None,
        lock_committed: false,
        cleanup_paths: Vec::new(),
        state: TransactionState::Resolved,
    };
    write_journal(state, &journal)?;
    Ok(journal)
}

/// Minimal lock entry recorded in a journal before the real entry exists;
/// overwritten by [`commit_version`] with the final entry.
fn placeholder_entry(id: &str) -> ExtensionLockEntry {
    ExtensionLockEntry {
        id: id.to_string(),
        name: id.to_string(),
        publisher_id: String::new(),
        publisher_name: String::new(),
        distribution_source: crate::extensions::lock::ExtensionDistributionSource::Npm,
        runtime_ownership: crate::extensions::lock::ExtensionRuntimeOwnership::Bundled,
        provider_kind: crate::extensions::lock::ExtensionProviderKind::Executable,
        state: crate::extensions::lock::ExtensionStateKind::Enabled,
        enabled: true,
        package_name: None,
        package_version: String::new(),
        tool_version: None,
        integrity: None,
        asset_selection: None,
        signature_verified: false,
        previous_signature_verified: None,
        official_verified: false,
        previous_official_verified: None,
        current_version: String::new(),
        previous_version: None,
        manifest_path: String::new(),
        executable_path: String::new(),
        runtime_root: None,
        installed_at: 0,
        updated_at: 0,
        pinned: false,
        channel: "latest".into(),
    }
}

/// Advance a journal to the next pipeline stage and persist it. Illegal jumps
/// (e.g. `Downloading -> Staged`) are rejected before anything is written.
pub(crate) fn progress(
    state: &ExtensionState,
    journal: &mut InstallationJournal,
    next: TransactionState,
) -> Result<(), String> {
    if !journal.state.may_transition_to(next) {
        return Err(format!(
            "Invalid installation transition: {:?} -> {:?}",
            journal.state, next
        ));
    }
    journal.state = next;
    write_journal(state, journal)?;
    Ok(())
}

/// Remove on-disk version directories outside the retained set (current plus
/// the single previous version). Used after a successful activation so old
/// versions do not accumulate, and by recovery for committed journals.
fn retain_versions(state: &ExtensionState, entry: &ExtensionLockEntry) -> Result<(), String> {
    let versions = state.paths.extensions.join(&entry.id).join("versions");
    if !versions.is_dir() {
        return Ok(());
    }
    let mut retained = vec![entry.current_version.clone()];
    if let Some(previous) = &entry.previous_version {
        retained.push(previous.clone());
    }
    let mut removed = false;
    for item in std::fs::read_dir(&versions)
        .map_err(|error| format!("Cannot scan retained extension versions: {error}"))?
    {
        let item = item.map_err(|error| format!("Cannot scan retained extension versions: {error}"))?;
        if !item
            .file_type()
            .map_err(|error| format!("Cannot inspect retained extension version: {error}"))?
            .is_dir()
        {
            continue;
        }
        let name = item.file_name().to_string_lossy().into_owned();
        if retained.iter().any(|keep| keep == &name) {
            continue;
        }
        std::fs::remove_dir_all(item.path()).map_err(|error| {
            format!("Cannot remove retained extension version {}: {error}", item.path().display())
        })?;
        removed = true;
    }
    if removed {
        sync_directory(&versions)
            .map_err(|error| format!("Cannot sync extension versions directory: {error}"))?;
    }
    Ok(())
}

/// Recover interrupted transactions at startup. Four recovery branches are
/// distinguished (crash-consistent journaling plan):
///
/// 1. `activated = false` (or a v1 journal without `lock_committed`): the
///    transaction never became visible; remove the staged target, restore the
///    backup directory and reinstall the old lock entry.
/// 2. `activated = true` and the lock already points at the new entry: the
///    version swap committed; finish cleanup (backup, staging, retention).
/// 3. Lock and `current.json` disagree: rebuild every pointer from the lock.
/// 4. Unreadable journal: quarantine as `.corrupt`; never guess-delete version
///    directories.
///
/// A journal that never reached `Staged` (no `staged_version`) has no filesystem
/// side effects; it is dropped and the lock is left untouched.
pub(crate) fn recover(state: &ExtensionState) -> Result<(), String> {
    // Staging cleanup is independent of whether any journal exists: a crash
    // before the first journal write still leaves an unpacked staging tree.
    remove_orphaned_staging(state)?;
    let directory = journal_dir(state);
    if !directory.is_dir() {
        return Ok(());
    }
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let mut entries: Vec<(PathBuf, InstallationJournal)> = Vec::new();
    for item in std::fs::read_dir(&directory)
        .map_err(|error| format!("Cannot scan extension transaction journal: {error}"))?
    {
        let item =
            item.map_err(|error| format!("Cannot read extension transaction journal: {error}"))?;
        let path = item.path();
        if path.extension().and_then(|value| value.to_str()) != Some("json") {
            continue;
        }
        let bytes = std::fs::read(&path)
            .map_err(|error| format!("Cannot read extension transaction journal: {error}"))?;
        let journal: InstallationJournal = match serde_json::from_slice(&bytes) {
            Ok(journal) => journal,
            Err(_) => {
                let _ = std::fs::rename(&path, path.with_extension("json.corrupt"));
                continue;
            }
        };
        if journal.schema_version != TRANSACTION_JOURNAL_SCHEMA_VERSION {
            return Err(format!(
                "Unsupported extension transaction journal schema {}",
                journal.schema_version
            ));
        }
        entries.push((path, journal));
    }
    entries.sort_by(|a, b| {
        a.1.new_entry
            .updated_at
            .cmp(&b.1.new_entry.updated_at)
            .then(a.0.cmp(&b.0))
    });
    for (path, journal) in &entries {
        let old_differs = journal.old_entry.as_ref().is_some_and(|old| {
            serde_json::to_value(old).ok() != serde_json::to_value(&journal.new_entry).ok()
        });
        let committed = journal.lock_committed
            || (old_differs
                && lock
                    .extensions
                    .get(&journal.extension_id)
                    .is_some_and(|entry| {
                        serde_json::to_value(entry).ok()
                            == serde_json::to_value(&journal.new_entry).ok()
                    }));
        if committed {
            if let Some(backup) = &journal.backup_version {
                if backup.exists() {
                    std::fs::remove_dir_all(backup).map_err(|error| {
                        format!("Cannot clean committed extension transaction backup: {error}")
                    })?;
                }
            }
            if let Some(staged) = &journal.staged_version {
                if staged.exists() {
                    let _ = std::fs::remove_dir_all(staged);
                }
            }
            let _ = retain_versions(state, &journal.new_entry);
            crate::extensions::artifacts::activate_entry_shims(
                &state.paths.extensions,
                &journal.new_entry,
            )?;
            remove_journal(path)?;
        } else if journal.staged_version.is_none() && journal.target_version.is_none() {
            // Download/pre-staging journal: nothing became visible, drop it.
            remove_journal(path)?;
        } else {
            if let Some(target) = &journal.target_version {
                if target.exists() {
                    let _ = std::fs::remove_dir_all(target);
                }
            }
            if let Some(staged) = &journal.staged_version {
                if staged.exists() {
                    let _ = std::fs::remove_dir_all(staged);
                }
            }
            if let (Some(backup), Some(target)) = (&journal.backup_version, &journal.target_version)
            {
                if backup.exists() && !target.exists() {
                    std::fs::rename(backup, target).map_err(|error| {
                        format!("Cannot restore interrupted extension transaction: {error}")
                    })?;
                }
            }
            if let Some(old) = &journal.old_entry {
                lock.extensions
                    .insert(journal.extension_id.clone(), old.clone());
            } else {
                lock.extensions.remove(&journal.extension_id);
            }
            remove_journal(path)?;
        }
    }
    lock.save(&state.paths.lock_file)?;
    rebuild_current_pointers(state, &lock)?;
    Ok(())
}

/// Remove staging directories left behind by a crash mid-install. Staging
/// holds unpacked-but-unactivated versions only; on startup no install is in
/// flight, so every `.staging` entry is orphaned by definition.
fn remove_orphaned_staging(state: &ExtensionState) -> Result<(), String> {
    let staging_root = state.paths.extensions.join(".staging");
    if !staging_root.is_dir() {
        return Ok(());
    }
    let mut removed = false;
    for item in std::fs::read_dir(&staging_root)
        .map_err(|error| format!("Cannot scan extension staging directory: {error}"))?
    {
        let item = item.map_err(|error| format!("Cannot scan extension staging directory: {error}"))?;
        if item
            .file_type()
            .map_err(|error| format!("Cannot inspect extension staging entry: {error}"))?
            .is_dir()
        {
            std::fs::remove_dir_all(item.path()).map_err(|error| {
                format!("Cannot remove orphaned staging {}: {error}", item.path().display())
            })?;
            removed = true;
        }
    }
    if removed {
        sync_directory(&staging_root)
            .map_err(|error| format!("Cannot sync extension staging directory: {error}"))?;
    }
    Ok(())
}

fn rebuild_current_pointers(state: &ExtensionState, lock: &ExtensionsLock) -> Result<(), String> {
    for entry in lock.extensions.values() {
        crate::extensions::artifacts::activate_entry_shims(&state.paths.extensions, entry)?;
        let _ = write_current_pointer(&state.paths.extensions, entry);
    }
    Ok(())
}

/// Atomically activate a staged version:
///
/// 1. persist the journal in the `Activated` stage (before any visible change);
/// 2. swap the version directory (backing up an existing same-version target);
/// 3. commit the lock and rewrite `current.json`;
/// 4. persist `Cleaned`, then run retention and remove the journal.
///
/// Pass the journal created by [`begin`]/[`progress`] when available; pass
/// `None` for callers that did not run the staged pipeline (a fresh journal is
/// created for them). The previous version directory is retained (not deleted)
/// so rollback only switches the pointer.
pub(crate) fn commit_version(
    state: &ExtensionState,
    old: Option<&ExtensionLockEntry>,
    entry: &ExtensionLockEntry,
    staged_version: &Path,
    target: &Path,
    journal: Option<InstallationJournal>,
) -> Result<(), String> {
    let backup = if target.exists() {
        Some(target.with_extension(format!("txn-backup-{}", uuid::Uuid::new_v4())))
    } else {
        None
    };
    let mut journal = match journal {
        Some(mut journal) => {
            journal.staged_version = Some(staged_version.to_path_buf());
            journal.target_version = Some(target.to_path_buf());
            journal.backup_version = backup.clone();
            journal.old_entry = old.cloned();
            journal.new_entry = entry.clone();
            journal
        }
        None => InstallationJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: uuid::Uuid::new_v4().to_string(),
            extension_id: entry.id.clone(),
            old_entry: old.cloned(),
            new_entry: entry.clone(),
            staged_version: Some(staged_version.to_path_buf()),
            target_version: Some(target.to_path_buf()),
            backup_version: backup.clone(),
            lock_committed: false,
            cleanup_paths: Vec::new(),
            state: TransactionState::Staged,
        },
    };
    progress(state, &mut journal, TransactionState::Activated)?;
    if let Some(backup) = &backup {
        std::fs::rename(target, backup)
            .map_err(|error| format!("Cannot stage previous extension version: {error}"))?;
        sync_directory(target.parent().ok_or("Invalid extension target")?)
            .map_err(|error| format!("Cannot sync extension versions directory: {error}"))?;
    }
    std::fs::rename(staged_version, target)
        .map_err(|error| format!("Cannot atomically install extension version: {error}"))?;
    sync_directory(target.parent().ok_or("Invalid extension target")?)
        .map_err(|error| format!("Cannot sync extension versions directory: {error}"))?;
    crate::extensions::artifacts::activate_entry_shims(&state.paths.extensions, entry)?;
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    lock.extensions.insert(entry.id.clone(), entry.clone());
    lock.save(&state.paths.lock_file)?;
    let mut committed_journal = journal.clone();
    committed_journal.lock_committed = true;
    write_journal(state, &committed_journal)?;
    let _ = write_current_pointer(&state.paths.extensions, entry);
    progress(state, &mut journal, TransactionState::Cleaned)?;
    retain_versions(state, entry)?;
    if let Some(backup) = backup {
        if backup.exists() {
            std::fs::remove_dir_all(backup)
                .map_err(|error| format!("Cannot remove previous extension version: {error}"))?;
        }
    }
    let journal_path = journal_dir(state).join(format!("{}.json", journal.transaction_id));
    remove_journal(&journal_path)?;
    Ok(())
}

/// Commit a lock-only change (rollback, enable/disable, permission update).
/// Version directories are not touched; the previous entry is journaled so a
/// crash between the lock write and the pointer rewrite can be resolved.
pub(crate) fn commit_lock(
    state: &ExtensionState,
    old: &ExtensionLockEntry,
    entry: &ExtensionLockEntry,
) -> Result<(), String> {
    let journal = InstallationJournal {
        schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
        transaction_id: uuid::Uuid::new_v4().to_string(),
        extension_id: entry.id.clone(),
        old_entry: Some(old.clone()),
        new_entry: entry.clone(),
        staged_version: None,
        target_version: None,
        backup_version: None,
        lock_committed: false,
        cleanup_paths: Vec::new(),
        state: TransactionState::Resolved,
    };
    let journal_path = write_journal(state, &journal)?;
    crate::extensions::artifacts::activate_entry_shims(&state.paths.extensions, entry)?;
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    lock.extensions.insert(entry.id.clone(), entry.clone());
    lock.save(&state.paths.lock_file)?;
    let mut committed_journal = journal.clone();
    committed_journal.lock_committed = true;
    write_journal(state, &committed_journal)?;
    let _ = write_current_pointer(&state.paths.extensions, entry);
    remove_journal(&journal_path)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::lock::{
        ExtensionDistributionSource, ExtensionProviderKind, ExtensionRuntimeOwnership,
        ExtensionStateKind,
    };
    use crate::extensions::ExtensionPaths;

    fn test_state(root: &Path) -> ExtensionState {
        ExtensionState::from_paths(ExtensionPaths::from_root(root.to_path_buf())).unwrap()
    }

    fn journal_entry(root: &Path, version: &str, id: &str) -> ExtensionLockEntry {
        let root = root.join("extensions").join(id).join("versions").join(version);
        ExtensionLockEntry {
            id: id.into(),
            name: "Example Journal".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source: ExtensionDistributionSource::Npm,
            runtime_ownership: ExtensionRuntimeOwnership::Bundled,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: Some("example-journal".into()),
            package_version: version.into(),
            tool_version: Some("1.0.0".into()),
            integrity: None,
            asset_selection: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: version.into(),
            previous_version: None,
            manifest_path: root.join("floter.extension.json").to_string_lossy().into_owned(),
            executable_path: root.join("runtime/tool").to_string_lossy().into_owned(),
            runtime_root: Some(root.join("runtime").to_string_lossy().into_owned()),
            installed_at: 1,
            updated_at: 1,
            pinned: false,
            channel: "latest".into(),
        }
    }

    fn staged_journal(
        state: &ExtensionState,
        id: &str,
        old_version: Option<&str>,
        new_version: &str,
        lock_committed: bool,
        state_stage: TransactionState,
    ) -> InstallationJournal {
        let old = old_version.map(|v| journal_entry(state.paths.root.as_path(), v, id));
        let new = {
            let mut entry = journal_entry(state.paths.root.as_path(), new_version, id);
            entry.previous_version = old_version.map(str::to_string);
            entry
        };
        InstallationJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: format!("tx-{}-{new_version}", old_version.unwrap_or("none")),
            extension_id: id.into(),
            old_entry: old,
            new_entry: new,
            staged_version: Some(
                state
                    .paths
                    .extensions
                    .join(id)
                    .join(".staging/install-1/version"),
            ),
            target_version: Some(
                state.paths.extensions.join(id).join("versions").join(new_version),
            ),
            backup_version: Some(
                state
                    .paths
                    .extensions
                    .join(id)
                    .join("versions")
                    .join(format!("{new_version}.txn-backup-0000")),
            ),
            lock_committed,
            cleanup_paths: Vec::new(),
            state: state_stage,
        }
    }

    #[test]
    fn stage_transitions_are_strict() {
        use TransactionState::*;
        assert!(Resolved.may_transition_to(Downloading));
        assert!(Downloading.may_transition_to(Downloaded));
        assert!(Downloaded.may_transition_to(Verified));
        assert!(Verified.may_transition_to(Staged));
        assert!(Staged.may_transition_to(Activated));
        assert!(Activated.may_transition_to(Cleaned));
        assert!(!Resolved.may_transition_to(Verified));
        assert!(!Downloaded.may_transition_to(Activated));
        assert!(!Cleaned.may_transition_to(Resolved));
    }

    #[test]
    fn progress_persists_stage_and_rejects_jumps() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let mut journal = begin(&state, "example.journal", None).unwrap();
        progress(&state, &mut journal, TransactionState::Downloading).unwrap();
        let on_disk: InstallationJournal = serde_json::from_slice(
            &std::fs::read(
                journal_dir(&state)
                    .join(format!("{}.json", journal.transaction_id)),
            )
            .unwrap(),
        )
        .unwrap();
        assert_eq!(on_disk.state, TransactionState::Downloading);
        assert!(progress(&state, &mut journal, TransactionState::Staged).is_err());
        let on_disk: InstallationJournal = serde_json::from_slice(
            &std::fs::read(
                journal_dir(&state)
                    .join(format!("{}.json", journal.transaction_id)),
            )
            .unwrap(),
        )
        .unwrap();
        assert_eq!(on_disk.state, TransactionState::Downloading);
    }

    #[test]
    fn recovery_rolls_back_directory_when_lock_was_not_committed() {
        // Crash after the staged version moved into place but before the lock
        // committed: the new version must be removed and the old backup
        // restored, and the lock must point at the old entry again.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.journal",
            Some("1.0.0"),
            "2.0.0",
            false,
            TransactionState::Activated,
        );
        let target = journal.target_version.clone().unwrap();
        let backup = journal.backup_version.clone().unwrap();
        std::fs::create_dir_all(target.join("runtime")).unwrap();
        std::fs::write(target.join("runtime/tool"), b"new").unwrap();
        std::fs::create_dir_all(&backup).unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            journal.old_entry.clone().unwrap().id.clone(),
            journal.old_entry.clone().unwrap(),
        );
        lock.save(&state.paths.lock_file).unwrap();
        write_journal(&state, &journal).unwrap();

        recover(&state).unwrap();

        assert!(target.exists());
        assert!(!backup.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get("example.journal")
                .unwrap()
                .current_version,
            "1.0.0"
        );
    }

    #[test]
    fn recovery_restores_backup_when_target_was_not_swapped() {
        // Crash after the old target was renamed to backup but before the
        // staged version was moved into place.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.journal",
            Some("1.0.0"),
            "2.0.0",
            false,
            TransactionState::Activated,
        );
        let target = journal.target_version.clone().unwrap();
        let backup = journal.backup_version.clone().unwrap();
        std::fs::create_dir_all(&backup).unwrap();
        std::fs::create_dir_all(journal.staged_version.as_ref().unwrap()).unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            journal.old_entry.clone().unwrap().id.clone(),
            journal.old_entry.clone().unwrap(),
        );
        lock.save(&state.paths.lock_file).unwrap();
        write_journal(&state, &journal).unwrap();

        recover(&state).unwrap();

        assert!(target.exists());
        assert!(!backup.exists());
        assert!(!journal.staged_version.as_ref().unwrap().exists());
    }

    #[test]
    fn recovery_finishes_cleanup_when_lock_was_committed() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.journal",
            Some("1.0.0"),
            "2.0.0",
            true,
            TransactionState::Activated,
        );
        let target = journal.target_version.clone().unwrap();
        let backup = journal.backup_version.clone().unwrap();
        std::fs::create_dir_all(&target).unwrap();
        std::fs::create_dir_all(&backup).unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            journal.new_entry.id.clone(),
            journal.new_entry.clone(),
        );
        lock.save(&state.paths.lock_file).unwrap();
        write_journal(&state, &journal).unwrap();

        recover(&state).unwrap();

        assert!(target.exists());
        assert!(!backup.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get("example.journal")
                .unwrap()
                .current_version,
            "2.0.0"
        );
    }

    #[test]
    fn recovery_drops_pre_staging_journal_without_touching_lock() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.journal",
            None,
            "1.0.0",
            false,
            TransactionState::Downloading,
        );
        write_journal(&state, &journal).unwrap();
        recover(&state).unwrap();
        assert!(!journal_dir(&state)
            .join(format!("{}.json", journal.transaction_id))
            .exists());
        // Nothing was ever visible: no version directory, no lock entry.
        assert!(!state.paths.extensions.join("example.journal").exists());
    }

    #[test]
    fn orphaned_staging_is_removed_on_recovery() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let staging = state.paths.extensions.join(".staging").join("install-42");
        std::fs::create_dir_all(staging.join("version/runtime")).unwrap();
        std::fs::write(staging.join("version/runtime/tool"), b"stale").unwrap();

        recover(&state).unwrap();

        assert!(!staging.exists());
        assert!(state.paths.extensions.join(".staging").exists());
    }

    #[test]
    fn corrupt_journal_is_quarantined() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let directory = journal_dir(&state);
        std::fs::create_dir_all(&directory).unwrap();
        std::fs::write(directory.join("broken.json"), b"{not json").unwrap();
        recover(&state).unwrap();
        assert!(!directory.join("broken.json").exists());
        assert!(directory.join("broken.json.corrupt").exists());
    }

    #[test]
    fn retention_keeps_current_and_previous_only() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let mut entry = journal_entry(state.paths.root.as_path(), "2.0.0", "example.retain");
        entry.previous_version = Some("1.5.0".into());
        let versions = state.paths.extensions.join("example.retain").join("versions");
        for version in ["1.0.0", "1.5.0", "1.9.0", "2.0.0", "0.9.0"] {
            std::fs::create_dir_all(versions.join(version)).unwrap();
        }
        std::fs::create_dir_all(versions.join("2.0.0.txn-backup-0000")).unwrap();

        retain_versions(&state, &entry).unwrap();

        assert!(versions.join("2.0.0").exists());
        assert!(versions.join("1.5.0").exists());
        assert!(!versions.join("1.0.0").exists());
        assert!(!versions.join("1.9.0").exists());
        assert!(!versions.join("0.9.0").exists());
        assert!(!versions.join("2.0.0.txn-backup-0000").exists());
    }
}
