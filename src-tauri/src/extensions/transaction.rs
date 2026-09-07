//! Recovery for legacy installation transactions and current removal/edit journals.
//!
//! Journals retain their entry schema while state writes go to the repository.
//! The journal records which transaction stage was
//! reached, so a crash at any point can be resolved on the next startup
//! ([`recover`]) without guessing: uncommitted transactions restore the
//! previously active version, committed transactions finish their cleanup, and
//! the `current.json` pointer is rebuilt from the repository.
//!
//! The stage machine is `resolved -> downloading -> downloaded -> verified ->
//! staged -> activated -> cleaned` (FEP/plan "确定性安装"). Download itself is
//! resumable via `.part` files with an independent journal (see `download.rs`);
//! the transaction journal only records which stage the install pipeline
//! reached before it stopped.

use crate::extensions::lock::{
    sync_directory, write_current_pointer, ExtensionLockEntry, ExtensionsLock,
};
use crate::extensions::manifest::ExtensionManifest;
use crate::extensions::ExtensionState;
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::{Path, PathBuf};

pub const TRANSACTION_JOURNAL_SCHEMA_VERSION: u32 = 3;

/// Stage of an installation transaction. Older journals (schema v1) did not
/// carry a stage; they are treated as [`TransactionState::Resolved`] and the
/// pre-existing `lock_committed` flag decides their recovery branch.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum TransactionState {
    /// Version selection finished; nothing has been downloaded yet.
    #[default]
    Resolved,
    /// Tarball download in progress (resumable via `download.rs` `.part`).
    Downloading,
    /// Downloads finished and integrity verified.
    Downloaded,
    /// Archive unpacked, manifest validated, provider described, probes run.
    Verified,
    /// Staging is complete and ready to be atomically activated.
    Staged,
    /// Version directory swapped and repository committed; cleanup remains.
    Activated,
    /// Backup/retained-version cleanup finished; journal may be removed.
    Cleaned,
}

/// Uninstall operation type tracked in the removal journal.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RemovalKind {
    /// Extension tree staged for removal but repository not yet updated.
    Staged,
    /// Repository entry removed; physical cleanup remains.
    Committed,
}

/// Intent behind a removal journal: uninstall vs edit.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RemovalIntent {
    /// Permanent removal (uninstall).
    #[default]
    Remove,
    /// Temporary backup for edit operation.
    Edit,
}

// NOTE: the staged-pipeline writers (`begin`, `progress`, `commit_version`,
// `commit_lock`) were removed together with the NPM distribution pipeline.
// `recover`, `write_journal`, and this enum stay because journals written by
// older builds must still load, recover, and be cleaned up at startup.

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

/// Uninstall-specific journal (schema v3+). Records pending removal so a crash
/// mid-uninstall can be completed on next startup without losing the fact that
/// removal was requested, even if repository commit succeeded but physical deletion
/// failed.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RemovalJournal {
    pub schema_version: u32,
    pub transaction_id: String,
    pub extension_id: String,
    /// Repository entry snapshot before removal, for rollback if needed.
    pub removed_entry: ExtensionLockEntry,
    /// Staged removal directory path (renamed from original location).
    pub staged_path: Option<PathBuf>,
    /// Additional paths to delete (generated integration, data).
    #[serde(default)]
    pub cleanup_paths: Vec<PathBuf>,
    /// Whether the repository entry has been removed.
    #[serde(default)]
    pub removal_kind: Option<RemovalKind>,
    /// Whether user data should be deleted.
    #[serde(default)]
    pub remove_data: bool,
    /// Intent: Remove (uninstall) or Edit (temporary backup).
    #[serde(default)]
    pub intent: RemovalIntent,
}

fn journal_dir(state: &ExtensionState) -> PathBuf {
    state.paths.extensions.join(".transactions")
}

fn removal_journal_path(state: &ExtensionState, transaction_id: &str) -> PathBuf {
    journal_dir(state).join(format!("removal-{}.json", transaction_id))
}

/// Persist a journal atomically. Production code no longer creates new
/// journals (the staged NPM pipeline was removed), but recovery tests use this
/// to fabricate legacy journals and `recover` still consumes their format.
#[allow(dead_code)]
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
    crate::extensions::commit_point("install-journal-persist");
    temporary
        .persist(&path)
        .map_err(|error| format!("Cannot persist extension transaction journal: {error}"))?;
    crate::extensions::commit_point("install-journal-directory-sync");
    sync_directory(&directory)
        .map_err(|error| format!("Cannot sync extension transaction journal: {error}"))?;
    Ok(path)
}

/// Write a removal journal atomically. Used by uninstall to record pending
/// removal before committing the repository, so crash/I/O failure during cleanup
/// can be recovered on next startup.
pub(crate) fn write_removal_journal(
    state: &ExtensionState,
    journal: &RemovalJournal,
) -> Result<PathBuf, String> {
    let directory = journal_dir(state);
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("Cannot create removal transaction journal: {error}"))?;
    let path = removal_journal_path(state, &journal.transaction_id);
    let bytes = serde_json::to_vec_pretty(journal)
        .map_err(|error| format!("Cannot serialize removal transaction journal: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(&directory)
        .map_err(|error| format!("Cannot create removal transaction journal: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write removal transaction journal: {error}"))?;
    crate::extensions::commit_point("removal-journal-persist");
    temporary
        .persist(&path)
        .map_err(|error| format!("Cannot persist removal transaction journal: {error}"))?;
    crate::extensions::commit_point("removal-journal-directory-sync");
    sync_directory(&directory)
        .map_err(|error| format!("Cannot sync removal transaction journal: {error}"))?;
    Ok(path)
}

fn remove_journal(path: &Path) -> Result<(), String> {
    if path.exists() {
        crate::extensions::commit_point("journal-remove");
        std::fs::remove_file(path)
            .map_err(|error| format!("Cannot remove extension transaction journal: {error}"))?;
        if let Some(parent) = path.parent() {
            sync_directory(parent)
                .map_err(|error| format!("Cannot sync extension transaction journal: {error}"))?;
        }
    }
    Ok(())
}

/// Recover interrupted removals and edits using the repository's commit state.
/// An uninstall with an entry restores its staged files; one without an entry
/// finishes cleanup. An edit keeps a committed replacement and otherwise
/// restores the old generation from its backup and journal.
///
/// The journal is removed ONLY when all planned operations succeed (or the paths
/// no longer exist). If any deletion/restore fails, the journal is kept on disk
/// so the next startup retries the operation.
fn recover_removal_journals(
    state: &ExtensionState,
    lock: &mut ExtensionsLock,
) -> Result<(), String> {
    let directory = journal_dir(state);
    for item in std::fs::read_dir(&directory)
        .map_err(|error| format!("Cannot scan removal transaction journals: {error}"))?
    {
        let item = item
            .map_err(|error| format!("Cannot read removal transaction journal: {error}"))?;
        let path = item.path();
        if !path
            .file_name()
            .and_then(|name| name.to_str())
            .is_some_and(|name| name.starts_with("removal-") && name.ends_with(".json"))
        {
            continue;
        }
        let bytes = std::fs::read(&path)
            .map_err(|error| format!("Cannot read removal transaction journal: {error}"))?;
        let journal: RemovalJournal = match serde_json::from_slice(&bytes) {
            Ok(journal) => journal,
            Err(_) => {
                crate::extensions::commit_point("removal-journal-quarantine");
                std::fs::rename(&path, path.with_extension("json.corrupt"))
                    .map_err(|error| format!("Cannot quarantine removal transaction journal: {error}"))?;
                continue;
            }
        };
        if journal.schema_version > TRANSACTION_JOURNAL_SCHEMA_VERSION {
            return Err(format!(
                "Unsupported removal transaction journal schema {}",
                journal.schema_version
            ));
        }
        let lock_entry_exists = lock.extensions.contains_key(&journal.extension_id);
        if lock_entry_exists && journal.intent != RemovalIntent::Edit {
            // Removal never committed (Staged branch): restore staged path if it exists.
            if let Some(staged) = &journal.staged_path {
                // Defect 1 fix: Determine restore target from WHERE staged_path lives,
                // not from cleanup_paths (which includes data dir for normal uninstall
                // with remove_data=true, causing wrong-location restore).
                let staged_canonical = staged.canonicalize().unwrap_or_else(|_| staged.clone());
                let extensions_canonical = state
                    .paths
                    .extensions
                    .canonicalize()
                    .unwrap_or_else(|_| state.paths.extensions.clone());

                let original = if staged_canonical.starts_with(&extensions_canonical) {
                    // Normal extension uninstall: staged is .removing-{id}- in extensions dir
                    state.paths.extensions.join(&journal.extension_id)
                } else if !journal.cleanup_paths.is_empty() {
                    // Custom integration edit: staged is backup in data dir
                    journal.cleanup_paths[0].clone()
                } else {
                    // Fallback: use extensions dir (shouldn't happen)
                    state.paths.extensions.join(&journal.extension_id)
                };

                if staged.exists() && !original.exists() {
                    // Defect 3 fix: Keep journal if restore fails
                    crate::extensions::commit_point("removal-recovery-restore");
                    if let Err(error) = std::fs::rename(staged, &original) {
                        tracing::warn!(
                            "Removal recovery: cannot restore {} to {}: {}; will retry on next startup",
                            staged.display(),
                            original.display(),
                            error
                        );
                        continue; // Keep journal, skip to next item
                    }
                }
            }
            remove_journal(&path)?;
        } else {
            // Edits require comparing generations even when an entry exists.
            match journal.intent {
                RemovalIntent::Edit => {
                    if let Some(staged) = &journal.staged_path {
                        if !journal.cleanup_paths.is_empty() {
                            let new_root = &journal.cleanup_paths[0];
                            let repository_is_new = lock
                                .extensions
                                .get(&journal.extension_id)
                                .is_some_and(|entry| {
                                    serde_json::to_value(entry).ok()
                                        != serde_json::to_value(&journal.removed_entry).ok()
                                });

                            if lock_entry_exists && new_root.exists() {
                                // Editing removes the entry before creating replacement files.
                                // An entry plus files therefore means the edit committed (or
                                // rollback restored it), even when the metadata is unchanged.
                                if staged.exists() {
                                    std::fs::remove_dir_all(staged).map_err(|error| {
                                        format!("Cannot remove completed edit backup: {error}")
                                    })?;
                                }
                                remove_journal(&path)?;
                            } else if staged.exists() {
                                // The repository still contains the old entry (or is missing it),
                                // so the edit did not commit. Remove any partially written new
                                // tree, restore the backup, and restore the old repository entry.
                                if new_root.exists() {
                                    std::fs::remove_dir_all(new_root).map_err(|error| {
                                        format!("Cannot remove incomplete edited integration: {error}")
                                    })?;
                                }
                                crate::extensions::commit_point("edit-recovery-restore");
                                if let Err(error) = std::fs::rename(staged, new_root) {
                                    tracing::warn!(
                                        "Edit recovery: cannot restore {} to {}: {}; will retry on next startup",
                                        staged.display(),
                                        new_root.display(),
                                        error
                                    );
                                    continue; // Keep journal
                                }
                                if !lock.extensions.contains_key(&journal.extension_id)
                                    || repository_is_new
                                {
                                    lock.extensions.insert(
                                        journal.extension_id.clone(),
                                        journal.removed_entry.clone(),
                                    );
                                    lock.save(&state.paths.repository_file)?;
                                }
                                remove_journal(&path)?;
                            } else if new_root.exists() {
                                // New files without a matching repository entry are an orphaned
                                // projection. Drop them and leave the extension absent.
                                std::fs::remove_dir_all(new_root).map_err(|error| {
                                    format!("Cannot remove orphaned edited integration: {error}")
                                })?;
                                if lock.extensions.remove(&journal.extension_id).is_some() {
                                    lock.save(&state.paths.repository_file)?;
                                }
                                remove_journal(&path)?;
                            } else {
                                // Both trees are gone. There is no projection to resurrect.
                                remove_journal(&path)?;
                            }
                        } else {
                            // No cleanup_paths: shouldn't happen for edit, treat as no-op
                            remove_journal(&path)?;
                        }
                    } else {
                        // No staged_path: nothing to restore
                        remove_journal(&path)?;
                    }
                }
                RemovalIntent::Remove => {
                    // Uninstall committed: finish physical cleanup. Keep the journal if
                    // any deletion fails so recovery can retry on next startup.
                    let mut cleanup_failed = false;
                    if let Some(staged) = &journal.staged_path {
                        if staged.exists() {
                            if let Err(error) = std::fs::remove_dir_all(staged) {
                                tracing::warn!(
                                    "Removal recovery: cannot delete {}: {}; will retry on next startup",
                                    staged.display(),
                                    error
                                );
                                cleanup_failed = true;
                            }
                        }
                    }
                    for cleanup_path in &journal.cleanup_paths {
                        if cleanup_path.exists() {
                            if let Err(error) = std::fs::remove_dir_all(cleanup_path) {
                                tracing::warn!(
                                    "Removal recovery: cannot delete {}: {}; will retry on next startup",
                                    cleanup_path.display(),
                                    error
                                );
                                cleanup_failed = true;
                            }
                        }
                    }
                    if !cleanup_failed {
                        remove_journal(&path)?;
                    }
                }
            }
        }
    }
    Ok(())
}

/// Recover interrupted transactions at startup. Four recovery branches are
/// distinguished (crash-consistent journaling plan):
///
/// 1. `activated = false` (or a v1 journal without `lock_committed`): the
///    transaction never became visible; remove the staged target, restore the
///    backup directory and reinstall the old repository entry.
/// 2. `activated = true` and the repository already points at the new entry: the
///    version swap committed; finish cleanup (backup, staging, retention).
/// 3. Repository and `current.json` disagree: rebuild every pointer from the repository.
/// 4. Unreadable journal: quarantine as `.corrupt`; never guess-delete version
///    directories.
///
/// A journal that never reached `Staged` (no `staged_version`) has no filesystem
/// side effects; it is dropped and repository entries are left untouched.
///
/// Removal journals (schema v3+) are processed separately: if the repository entry
/// still exists, the removal never committed, so drop the journal and restore
/// staged paths; if the entry is gone, finish cleanup or restore an interrupted edit.
pub(crate) fn recover(state: &ExtensionState) -> Result<(), String> {
    // Staging cleanup is independent of whether any journal exists: a crash
    // before the first journal write still leaves an unpacked staging tree.
    remove_orphaned_staging(state)?;
    let directory = journal_dir(state);
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;

    if directory.is_dir() {
        // Recover removal journals first: they must complete before install journals.
        recover_removal_journals(state, &mut lock)?;
        recover_install_journals(state, &mut lock)?;
    }
    recover_sync_import_staging(state, &lock)?;
    remove_orphaned_generated_data(state, &lock)?;
    rebuild_current_pointers(state, &lock)?;
    Ok(())
}

fn recover_install_journals(state: &ExtensionState, lock: &mut ExtensionsLock) -> Result<(), String> {
    let directory = journal_dir(state);
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
        // Removal journals have already been processed.
        if path.file_name()
            .and_then(|name| name.to_str())
            .is_some_and(|name| name.starts_with("removal-"))
        {
            continue;
        }
        let bytes = std::fs::read(&path)
            .map_err(|error| format!("Cannot read extension transaction journal: {error}"))?;
        let journal: InstallationJournal = match serde_json::from_slice(&bytes) {
            Ok(journal) => journal,
            Err(_) => {
                crate::extensions::commit_point("install-journal-quarantine");
                std::fs::rename(&path, path.with_extension("json.corrupt"))
                    .map_err(|error| format!("Cannot quarantine extension transaction journal: {error}"))?;
                continue;
            }
        };
        if journal.schema_version > TRANSACTION_JOURNAL_SCHEMA_VERSION {
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
        // The repository write and the journal's committed flag are intentionally
        // separate durable writes. If the process dies between them, the repository
        // is still the source of truth. This also covers a first install,
        // where there is no old entry to compare against.
        let lock_matches_new = lock
            .extensions
            .get(&journal.extension_id)
            .is_some_and(|entry| {
                serde_json::to_value(entry).ok() == serde_json::to_value(&journal.new_entry).ok()
            });
        let committed = journal.lock_committed || lock_matches_new;
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
                    crate::extensions::commit_point("install-recovery-restore");
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
    lock.save(&state.paths.repository_file)
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
        let item =
            item.map_err(|error| format!("Cannot scan extension staging directory: {error}"))?;
        if item
            .file_type()
            .map_err(|error| format!("Cannot inspect extension staging entry: {error}"))?
            .is_dir()
        {
            std::fs::remove_dir_all(item.path()).map_err(|error| {
                format!(
                    "Cannot remove orphaned staging {}: {error}",
                    item.path().display()
                )
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

fn remove_orphaned_generated_data(
    state: &ExtensionState,
    lock: &ExtensionsLock,
) -> Result<(), String> {
    if !state.paths.data.is_dir() {
        return Ok(());
    }
    for item in std::fs::read_dir(&state.paths.data)
        .map_err(|error| format!("Cannot scan extension data projections: {error}"))?
    {
        let item = item
            .map_err(|error| format!("Cannot read extension data projection: {error}"))?;
        let Some(id) = item.file_name().to_str().map(str::to_string) else {
            continue;
        };
        if id.starts_with('.') || lock.extensions.contains_key(&id) {
            continue;
        }
        let root = item.path();
        for generated in [root.join("integration"), root.join("sync")] {
            if generated.is_dir() {
                crate::extensions::commit_point("projection-remove-orphan-data");
                std::fs::remove_dir_all(&generated).map_err(|error| {
                    format!("Cannot remove orphaned extension data: {error}")
                })?;
            }
        }
        if root.is_dir()
            && std::fs::read_dir(&root)
                .map_err(|error| format!("Cannot inspect extension data projection: {error}"))?
                .next()
                .is_none()
        {
            crate::extensions::commit_point("projection-remove-orphan-data");
            std::fs::remove_dir(&root)
                .map_err(|error| format!("Cannot remove empty extension data: {error}"))?;
        }
    }
    Ok(())
}

fn recover_sync_import_staging(
    state: &ExtensionState,
    lock: &ExtensionsLock,
) -> Result<(), String> {
    if !state.paths.data.is_dir() {
        return Ok(());
    }
    for item in std::fs::read_dir(&state.paths.data)
        .map_err(|error| format!("Cannot scan sync import staging: {error}"))?
    {
        let item = item.map_err(|error| format!("Cannot read sync import staging: {error}"))?;
        let Some(id) = item.file_name().to_str().map(str::to_string) else {
            continue;
        };
        if id.starts_with('.') || !item.path().is_dir() {
            continue;
        }
        let root = item.path();
        let target = root.join("sync");
        let mut backups = Vec::new();
        for child in std::fs::read_dir(&root)
            .map_err(|error| format!("Cannot scan sync import staging: {error}"))?
        {
            let child = child.map_err(|error| format!("Cannot read sync import staging: {error}"))?;
            let name = child.file_name();
            let Some(name) = name.to_str() else { continue };
            if name.starts_with(".sync-import-backup-") {
                backups.push(child.path());
            } else if name.starts_with(".sync-import-") {
                crate::extensions::commit_point("sync-import-staging-remove");
                std::fs::remove_dir_all(child.path()).map_err(|error| {
                    format!("Cannot remove interrupted sync import staging: {error}")
                })?;
            }
        }
        for backup in backups {
            let manifest_path = target.join("floter.extension.json");
            let repository_committed = lock.extensions.get(&id).is_some_and(|entry| {
                entry.manifest_path == manifest_path.to_string_lossy()
                    && entry.approved_manifest_digest.as_deref().is_some_and(|digest| {
                        ExtensionManifest::load_with_digest(&manifest_path)
                            .ok()
                            .is_some_and(|(_, current)| current == digest)
                    })
            });
            if repository_committed {
                crate::extensions::commit_point("sync-import-backup-remove");
                std::fs::remove_dir_all(&backup).map_err(|error| {
                    format!("Cannot remove committed sync import backup: {error}")
                })?;
            } else {
                if target.exists() {
                    crate::extensions::commit_point("sync-import-orphan-remove");
                    std::fs::remove_dir_all(&target).map_err(|error| {
                        format!("Cannot remove incomplete sync import: {error}")
                    })?;
                }
                crate::extensions::commit_point("sync-import-backup-restore");
                std::fs::rename(&backup, &target).map_err(|error| {
                    format!("Cannot restore interrupted sync import: {error}")
                })?;
            }
        }
    }
    Ok(())
}

fn rebuild_current_pointers(state: &ExtensionState, lock: &ExtensionsLock) -> Result<(), String> {
    // The repository is authoritative. Extension directories and runtime
    // projections that have no repository entry are stale leftovers from an
    // interrupted commit and must not remain discoverable on disk.
    if state.paths.extensions.is_dir() {
        for item in std::fs::read_dir(&state.paths.extensions)
            .map_err(|error| format!("Cannot scan extension projections: {error}"))?
        {
            let item = item
                .map_err(|error| format!("Cannot read extension projection: {error}"))?;
            let name = item.file_name();
            let Some(id) = name.to_str() else { continue };
            if id.starts_with('.') || lock.extensions.contains_key(id) {
                continue;
            }
            if item
                .file_type()
                .map_err(|error| format!("Cannot inspect extension projection: {error}"))?
                .is_dir()
            {
                crate::extensions::commit_point("projection-remove-orphan");
                std::fs::remove_dir_all(item.path()).map_err(|error| {
                    format!("Cannot remove orphaned extension projection: {error}")
                })?;
            }
        }
    }
    for entry in lock.extensions.values() {
        let extension_root = state.paths.extensions.join(&entry.id);
        if entry.distribution_source != crate::extensions::lock::ExtensionDistributionSource::Npm {
            // Local integrations do not use NPM projections. Remove stale
            // pointer/shim directories left by an older installation.
            for projection in [extension_root.join("current.json"), extension_root.join("shims")] {
                if projection.is_dir() {
                    crate::extensions::commit_point("projection-remove-stale");
                    std::fs::remove_dir_all(&projection).map_err(|error| {
                        format!("Cannot remove stale extension projection: {error}")
                    })?;
                } else if projection.exists() {
                    crate::extensions::commit_point("projection-remove-stale");
                    std::fs::remove_file(&projection).map_err(|error| {
                        format!("Cannot remove stale extension projection: {error}")
                    })?;
                }
            }
            continue;
        }
        crate::extensions::artifacts::activate_entry_shims(&state.paths.extensions, entry)?;
        // A pointer is the runtime-facing projection of the repository. If it
        // cannot be rewritten, startup must fail loudly instead of leaving a
        // valid repository paired with a stale executable shim.
        write_current_pointer(&state.paths.extensions, entry)?;
    }
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
        let root = root
            .join("extensions")
            .join(id)
            .join("versions")
            .join(version);
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
            runtime_integrity: None,
            content_integrity: None,
            previous_integrity: None,
            previous_runtime_integrity: None,
            previous_content_integrity: None,
            asset_selection: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: version.into(),
            previous_version: None,
            manifest_path: root
                .join("floter.extension.json")
                .to_string_lossy()
                .into_owned(),
            executable_path: root.join("runtime/tool").to_string_lossy().into_owned(),
            runtime_root: Some(root.join("runtime").to_string_lossy().into_owned()),
            installed_at: 1,
            updated_at: 1,
            pinned: false,
            channel: "latest".into(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
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
                state
                    .paths
                    .extensions
                    .join(id)
                    .join("versions")
                    .join(new_version),
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
    fn recovery_writeback_through_repo_restores_install_entry() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        ExtensionsLock::default().save_legacy(&state.paths.lock_file).unwrap();
        crate::extensions::repository::migrate_to_repository(&state.paths).unwrap();
        let archive = state.paths.root.join("extensions.lock.json.migrated");
        let archive_bytes = std::fs::read(&archive).unwrap();
        let before = std::fs::read(&state.paths.repository_file).unwrap();
        let journal = staged_journal(
            &state, "example.repo-recovery", Some("1.0.0"), "2.0.0", false, TransactionState::Staged,
        );
        std::fs::create_dir_all(journal.target_version.as_ref().unwrap()).unwrap();
        let journal_path = write_journal(&state, &journal).unwrap();

        recover(&state).unwrap();

        let after = std::fs::read(&state.paths.repository_file).unwrap();
        assert_ne!(after, before);
        let json: serde_json::Value = serde_json::from_slice(&after).unwrap();
        assert_eq!(json["schemaVersion"], crate::extensions::repository::REPOSITORY_SCHEMA_VERSION);
        assert_eq!(json["extensions"][&journal.extension_id], serde_json::to_value(journal.old_entry.as_ref().unwrap()).unwrap());
        assert!(!journal_path.exists());
        assert!(!journal.target_version.as_ref().unwrap().exists());
        assert!(!state.paths.lock_file.exists());
        assert_eq!(std::fs::read(&archive).unwrap(), archive_bytes);
        let pointer: serde_json::Value = serde_json::from_slice(
            &std::fs::read(state.paths.extensions.join(&journal.extension_id).join("current.json")).unwrap(),
        ).unwrap();
        assert_eq!(pointer["version"], "1.0.0");
        recover(&state).unwrap();
        assert_eq!(std::fs::read(&state.paths.repository_file).unwrap(), after);
        assert!(!state.paths.lock_file.exists());
    }

    #[test]
    fn recovery_writeback_through_repo_restores_edit_entry() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "example.edit-recovery";
        let root = state.paths.data.join(id).join("integration");
        let backup = state.paths.data.join(id).join(".editing-backup");
        std::fs::create_dir_all(&root).unwrap();
        std::fs::write(root.join("provider.sh"), b"original").unwrap();
        let mut original = journal_entry(directory.path(), "1.0.0", id);
        original.distribution_source = ExtensionDistributionSource::Local;
        original.runtime_ownership = ExtensionRuntimeOwnership::System;
        original.manifest_path = root.join("floter.extension.json").to_string_lossy().into_owned();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(id.into(), original.clone());
        lock.save_legacy(&state.paths.lock_file).unwrap();
        crate::extensions::repository::migrate_to_repository(&state.paths).unwrap();
        let archive = state.paths.root.join("extensions.lock.json.migrated");
        let archive_bytes = std::fs::read(&archive).unwrap();
        std::fs::rename(&root, &backup).unwrap();
        let journal_path = write_removal_journal(&state, &RemovalJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: "edit-repo-recovery".into(),
            extension_id: id.into(),
            removed_entry: original.clone(),
            staged_path: Some(backup.clone()),
            cleanup_paths: vec![root.clone()],
            removal_kind: Some(RemovalKind::Staged),
            remove_data: false,
            intent: RemovalIntent::Edit,
        }).unwrap();
        // Crash after entry removal, before the replacement integration is written.
        lock.extensions.remove(id);
        lock.save(&state.paths.repository_file).unwrap();
        let before = std::fs::read(&state.paths.repository_file).unwrap();

        recover(&state).unwrap();

        let after = std::fs::read(&state.paths.repository_file).unwrap();
        assert_ne!(after, before);
        let json: serde_json::Value = serde_json::from_slice(&after).unwrap();
        assert_eq!(json["schemaVersion"], crate::extensions::repository::REPOSITORY_SCHEMA_VERSION);
        assert_eq!(json["extensions"][id], serde_json::to_value(original).unwrap());
        assert_eq!(std::fs::read(root.join("provider.sh")).unwrap(), b"original");
        assert!(!backup.exists());
        assert!(!journal_path.exists());
        assert!(!state.paths.lock_file.exists());
        assert_eq!(std::fs::read(&archive).unwrap(), archive_bytes);
        recover(&state).unwrap();
        assert_eq!(std::fs::read(&state.paths.repository_file).unwrap(), after);
        assert!(!state.paths.lock_file.exists());
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
        lock.save(&state.paths.repository_file).unwrap();
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
        lock.save(&state.paths.repository_file).unwrap();
        write_journal(&state, &journal).unwrap();

        recover(&state).unwrap();

        assert!(target.exists());
        assert!(!backup.exists());
        assert!(!journal.staged_version.as_ref().unwrap().exists());
    }

    #[test]
    fn recovery_after_migration_processes_install_journal() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.migrated",
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
        lock.extensions
            .insert(journal.new_entry.id.clone(), journal.new_entry.clone());
        lock.save_legacy(&state.paths.lock_file).unwrap();
        let journal_path = write_journal(&state, &journal).unwrap();
        crate::extensions::repository::migrate_to_repository(&state.paths).unwrap();

        recover(&state).unwrap();

        assert!(target.exists());
        assert!(!backup.exists());
        assert!(!journal_path.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get("example.migrated")
                .unwrap()
                .current_version,
            "2.0.0"
        );
        let pointer: serde_json::Value = serde_json::from_slice(
            &std::fs::read(state.paths.extensions.join("example.migrated/current.json")).unwrap(),
        )
        .unwrap();
        assert_eq!(pointer["version"], "2.0.0");
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
        lock.extensions
            .insert(journal.new_entry.id.clone(), journal.new_entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
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
    fn recovery_keeps_fresh_install_when_lock_was_written_before_journal_flag() {
        // A first install has no old entry, so recovery must use the matching
        // lock entry to recognize a crash after lock.save and before the
        // journal's committed flag was persisted.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.fresh",
            None,
            "1.0.0",
            false,
            TransactionState::Activated,
        );
        let target = journal.target_version.clone().unwrap();
        std::fs::create_dir_all(&target).unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions
            .insert(journal.new_entry.id.clone(), journal.new_entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
        write_journal(&state, &journal).unwrap();

        recover(&state).unwrap();

        assert!(target.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get("example.fresh")
                .unwrap()
                .current_version,
            "1.0.0"
        );
        assert!(!journal_dir(&state)
            .join(format!("{}.json", journal.transaction_id))
            .exists());
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
    fn removal_journal_persists_when_cleanup_fails_on_first_recovery() {
        // Committed removal journal with cleanup failure: journal must survive
        // so next recovery retries. Simulate a locked/undeletable directory by
        // replacing it with a file (portable failure mechanism).
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let entry = journal_entry(state.paths.root.as_path(), "1.0.0", "example.cleanup-fail");
        let staged_path = state
            .paths
            .extensions
            .join(".removing-example.cleanup-fail-staged");
        std::fs::create_dir_all(&staged_path).unwrap();
        std::fs::write(staged_path.join("data"), b"residue").unwrap();

        let journal = RemovalJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: "removal-cleanup-fail".into(),
            extension_id: "example.cleanup-fail".into(),
            removed_entry: entry.clone(),
            staged_path: Some(staged_path.clone()),
            cleanup_paths: Vec::new(),
            removal_kind: Some(RemovalKind::Committed),
            remove_data: false,
            intent: RemovalIntent::Remove,
        };
        let journal_path = write_removal_journal(&state, &journal).unwrap();

        // Replace staged directory with a file to simulate deletion failure.
        std::fs::remove_dir_all(&staged_path).unwrap();
        std::fs::write(&staged_path, b"locked").unwrap();

        let mut lock = ExtensionsLock::default();
        recover_removal_journals(&state, &mut lock).unwrap();

        // Journal must still exist because cleanup failed.
        assert!(journal_path.exists());
        // Staged path obstacle remains (simulated locked file).
        assert!(staged_path.exists());

        // Second recovery after removing the obstacle completes cleanup.
        std::fs::remove_file(&staged_path).unwrap();
        recover_removal_journals(&state, &mut lock).unwrap();
        assert!(!journal_path.exists());
        assert!(!staged_path.exists());
    }

    #[test]
    fn removal_journal_persists_when_cleanup_path_fails() {
        // Cleanup path deletion failure: journal survives for retry.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let entry = journal_entry(state.paths.root.as_path(), "1.0.0", "example.cleanup-path-fail");
        let cleanup_path = state.paths.data.join("example.cleanup-path-fail");
        std::fs::create_dir_all(&cleanup_path).unwrap();
        std::fs::write(cleanup_path.join("data"), b"user data").unwrap();

        let journal = RemovalJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: "removal-cleanup-path".into(),
            extension_id: "example.cleanup-path-fail".into(),
            removed_entry: entry,
            staged_path: None,
            cleanup_paths: vec![cleanup_path.clone()],
            removal_kind: Some(RemovalKind::Committed),
            remove_data: true,
            intent: RemovalIntent::Remove,
        };
        let journal_path = write_removal_journal(&state, &journal).unwrap();

        // Replace cleanup directory with a file to simulate deletion failure.
        std::fs::remove_dir_all(&cleanup_path).unwrap();
        std::fs::write(&cleanup_path, b"locked").unwrap();

        let mut lock = ExtensionsLock::default();
        recover_removal_journals(&state, &mut lock).unwrap();

        // Journal persists because cleanup failed.
        assert!(journal_path.exists());
        assert!(cleanup_path.exists());

        // Remove obstacle and retry.
        std::fs::remove_file(&cleanup_path).unwrap();
        recover_removal_journals(&state, &mut lock).unwrap();
        assert!(!journal_path.exists());
        assert!(!cleanup_path.exists());
    }

    #[test]
    fn fault_at_repository_persist_recovers_to_a_parseable_source_of_truth() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let mut lock = ExtensionsLock::default();
        let old = journal_entry(state.paths.root.as_path(), "1.0.0", "example.fault-repository");
        lock.extensions.insert(old.id.clone(), old.clone());
        lock.save(&state.paths.repository_file).unwrap();
        let before = std::fs::read(&state.paths.repository_file).unwrap();
        lock.extensions.insert(
            "example.fault-repository".into(),
            journal_entry(state.paths.root.as_path(), "2.0.0", "example.fault-repository"),
        );
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("repository-persist", || {
                lock.save(&state.paths.repository_file).unwrap();
            });
        }));
        assert!(result.is_err());
        recover(&state).unwrap();
        assert_eq!(std::fs::read(&state.paths.repository_file).unwrap(), before);
        let repository = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(repository.get(&old.id).unwrap()).unwrap(),
            serde_json::to_value(old).unwrap()
        );
    }

    #[test]
    fn fault_after_repository_replace_recovers_the_committed_entry() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let mut lock = ExtensionsLock::default();
        let entry = journal_entry(
            state.paths.root.as_path(),
            "1.0.0",
            "example.fault-directory-sync",
        );
        lock.extensions.insert(entry.id.clone(), entry.clone());
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("repository-directory-sync", || {
                lock.save(&state.paths.repository_file).unwrap();
            });
        }));
        assert!(result.is_err());
        recover(&state).unwrap();
        assert_eq!(
            ExtensionsLock::load(&state.paths.repository_file)
                .unwrap()
                .get(&entry.id)
                .unwrap()
                .current_version,
            "1.0.0"
        );
    }

    #[test]
    fn fault_at_current_pointer_persist_rebuilds_the_projection() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let entry = journal_entry(
            state.paths.root.as_path(),
            "2.0.0",
            "example.fault-pointer",
        );
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("current-pointer-persist", || {
                write_current_pointer(&state.paths.extensions, &entry).unwrap();
            });
        }));
        assert!(result.is_err());
        recover(&state).unwrap();
        let pointer: serde_json::Value = serde_json::from_slice(
            &std::fs::read(
                state
                    .paths
                    .extensions
                    .join(&entry.id)
                    .join("current.json"),
            )
            .unwrap(),
        )
        .unwrap();
        assert_eq!(pointer["version"], "2.0.0");
    }

    #[test]
    fn fault_at_install_journal_persist_leaves_no_half_registered_entry() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.fault-install-journal",
            None,
            "1.0.0",
            false,
            TransactionState::Staged,
        );
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("install-journal-persist", || {
                write_journal(&state, &journal).unwrap();
            });
        }));
        assert!(result.is_err());
        recover(&state).unwrap();
        assert!(ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .extensions
            .is_empty());
    }

    #[test]
    fn fault_at_removal_journal_persist_keeps_repository_coherent() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let entry = journal_entry(
            state.paths.root.as_path(),
            "1.0.0",
            "example.fault-removal-journal",
        );
        let journal = RemovalJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: "fault-removal-journal".into(),
            extension_id: entry.id.clone(),
            removed_entry: entry,
            staged_path: None,
            cleanup_paths: Vec::new(),
            removal_kind: Some(RemovalKind::Staged),
            remove_data: false,
            intent: RemovalIntent::Remove,
        };
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("removal-journal-persist", || {
                write_removal_journal(&state, &journal).unwrap();
            });
        }));
        assert!(result.is_err());
        recover(&state).unwrap();
        serde_json::from_slice::<serde_json::Value>(
            &std::fs::read(&state.paths.repository_file).unwrap(),
        )
        .unwrap();
    }

    #[test]
    fn fault_at_install_recovery_restore_retries_on_next_launch() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let journal = staged_journal(
            &state,
            "example.fault-install-restore",
            Some("1.0.0"),
            "2.0.0",
            false,
            TransactionState::Staged,
        );
        let target = journal.target_version.as_ref().unwrap();
        let backup = journal.backup_version.as_ref().unwrap();
        std::fs::create_dir_all(target).unwrap();
        std::fs::create_dir_all(backup).unwrap();
        write_journal(&state, &journal).unwrap();
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("install-recovery-restore", || {
                recover(&state).unwrap();
            });
        }));
        assert!(result.is_err());
        recover(&state).unwrap();
        assert!(target.exists());
        assert!(!backup.exists());
        assert!(ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get("example.fault-install-restore")
            .is_ok());
    }

    #[test]
    fn edit_recovery_drops_orphaned_new_files_when_repository_never_committed() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "example.fault-edit-orphan";
        let old = journal_entry(state.paths.root.as_path(), "1.0.0", id);
        let root = state.paths.data.join(id).join("integration");
        let backup = state.paths.data.join(id).join(".edit-backup");
        std::fs::create_dir_all(&backup).unwrap();
        std::fs::write(backup.join("old"), b"old").unwrap();
        std::fs::create_dir_all(&root).unwrap();
        std::fs::write(root.join("new"), b"new").unwrap();
        ExtensionsLock::default()
            .save(&state.paths.repository_file)
            .unwrap();
        let journal = RemovalJournal {
            schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: "fault-edit-orphan".into(),
            extension_id: id.into(),
            removed_entry: old.clone(),
            staged_path: Some(backup.clone()),
            cleanup_paths: vec![root.clone()],
            removal_kind: Some(RemovalKind::Staged),
            remove_data: false,
            intent: RemovalIntent::Edit,
        };
        write_removal_journal(&state, &journal).unwrap();
        recover(&state).unwrap();
        assert!(root.join("old").exists());
        assert!(!root.join("new").exists());
        assert!(ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get(id)
            .is_ok());
    }

    #[cfg(unix)]
    mod unix_faults {
        use super::*;
        use crate::extensions::fault_test_support::{
            crash_child_root, kill_at_commit_point, run_crash_child, skip_readonly_as_root,
            Corruption, ReadonlyDirectory,
        };
        use crate::extensions::with_commit_point_action;

        fn installed_fixture(state: &ExtensionState, id: &str) -> ExtensionsLock {
            let entry = journal_entry(&state.paths.root, "1.0.0", id);
            let mut lock = ExtensionsLock::default();
            std::fs::create_dir_all(Path::new(&entry.executable_path).parent().unwrap()).unwrap();
            std::fs::write(&entry.executable_path, b"installed executable").unwrap();
            lock.extensions.insert(id.into(), entry);
            lock.save(&state.paths.repository_file).unwrap();
            lock
        }

        fn assert_repository_unchanged(state: &ExtensionState, expected: &ExtensionsLock) {
            let actual = ExtensionsLock::load(&state.paths.repository_file).unwrap();
            assert_eq!(
                serde_json::to_value(&actual).unwrap(),
                serde_json::to_value(expected).unwrap()
            );
            for entry in expected.extensions.values() {
                assert_eq!(
                    std::fs::read(&entry.executable_path).unwrap(),
                    b"installed executable"
                );
            }
        }

        fn removal_fixture(state: &ExtensionState, entry: &ExtensionLockEntry) -> RemovalJournal {
            let staged = state.paths.extensions.join(".removing-fault");
            let cleanup = state.paths.data.join(&entry.id).join("user-data");
            for path in [&staged, &cleanup] {
                std::fs::create_dir_all(path).unwrap();
                std::fs::write(path.join("keep"), b"do not guess-delete").unwrap();
            }
            RemovalJournal {
                schema_version: TRANSACTION_JOURNAL_SCHEMA_VERSION,
                transaction_id: "fault-removal".into(),
                extension_id: entry.id.clone(),
                removed_entry: entry.clone(),
                staged_path: Some(staged),
                cleanup_paths: vec![cleanup],
                removal_kind: Some(RemovalKind::Staged),
                remove_data: true,
                intent: RemovalIntent::Remove,
            }
        }

        #[test]
        fn corrupt_current_pointer_is_rebuilt_from_the_repository() {
            for corruption in Corruption::ALL {
                let directory = tempfile::tempdir().unwrap();
                let state = test_state(directory.path());
                let lock = installed_fixture(&state, "example.corrupt-pointer");
                let entry = lock.extensions.values().next().unwrap();
                write_current_pointer(&state.paths.extensions, entry).unwrap();
                let path = crate::extensions::lock::current_pointer_path(
                    &state.paths.extensions,
                    &entry.id,
                )
                .unwrap();
                let expected = std::fs::read(&path).unwrap();
                corruption.apply(&path);

                let restarted = test_state(directory.path());
                for _ in 0..2 {
                    recover(&restarted).unwrap();
                    assert_eq!(std::fs::read(&path).unwrap(), expected);
                    assert_repository_unchanged(&restarted, &lock);
                }
            }
        }

        #[test]
        fn corrupt_install_journal_is_archived_without_guessing_cleanup() {
            for corruption in Corruption::ALL {
                let directory = tempfile::tempdir().unwrap();
                let state = test_state(directory.path());
                let id = "example.corrupt-install-journal";
                let lock = installed_fixture(&state, id);
                let journal = staged_journal(
                    &state,
                    id,
                    Some("1.0.0"),
                    "2.0.0",
                    false,
                    TransactionState::Staged,
                );
                let retained = [
                    journal.staged_version.as_ref().unwrap(),
                    journal.target_version.as_ref().unwrap(),
                    journal.backup_version.as_ref().unwrap(),
                ];
                for path in retained {
                    std::fs::create_dir_all(path).unwrap();
                    std::fs::write(path.join("keep"), b"do not guess-delete").unwrap();
                }
                let path = write_journal(&state, &journal).unwrap();
                let corrupted = corruption.apply(&path);

                let restarted = test_state(directory.path());
                for _ in 0..2 {
                    recover(&restarted).unwrap();
                    assert!(!path.exists());
                    assert_eq!(
                        std::fs::read(path.with_extension("json.corrupt")).unwrap(),
                        corrupted
                    );
                    assert_repository_unchanged(&restarted, &lock);
                    for retained in retained {
                        assert_eq!(
                            std::fs::read(retained.join("keep")).unwrap(),
                            b"do not guess-delete"
                        );
                    }
                }
            }
        }

        #[test]
        fn corrupt_removal_journal_is_archived_without_guessing_cleanup() {
            for corruption in Corruption::ALL {
                let directory = tempfile::tempdir().unwrap();
                let state = test_state(directory.path());
                let lock = installed_fixture(&state, "example.corrupt-removal-journal");
                let journal = removal_fixture(&state, lock.extensions.values().next().unwrap());
                let path = write_removal_journal(&state, &journal).unwrap();
                let corrupted = corruption.apply(&path);

                let restarted = test_state(directory.path());
                for _ in 0..2 {
                    recover(&restarted).unwrap();
                    assert!(!path.exists());
                    assert_eq!(
                        std::fs::read(path.with_extension("json.corrupt")).unwrap(),
                        corrupted
                    );
                    assert_repository_unchanged(&restarted, &lock);
                    for retained in journal
                        .staged_path
                        .iter()
                        .chain(journal.cleanup_paths.iter())
                    {
                        assert_eq!(
                            std::fs::read(retained.join("keep")).unwrap(),
                            b"do not guess-delete"
                        );
                    }
                }
            }
        }

        #[test]
        fn unsupported_journal_schemas_abort_without_modifying_state() {
            for removal in [false, true] {
                let directory = tempfile::tempdir().unwrap();
                let state = test_state(directory.path());
                let id = "example.future-journal";
                let lock = installed_fixture(&state, id);
                let path = if removal {
                    let mut journal = removal_fixture(&state, lock.get(id).unwrap());
                    journal.schema_version += 1;
                    write_removal_journal(&state, &journal).unwrap()
                } else {
                    let mut journal = staged_journal(
                        &state,
                        id,
                        Some("1.0.0"),
                        "2.0.0",
                        false,
                        TransactionState::Staged,
                    );
                    journal.schema_version += 1;
                    write_journal(&state, &journal).unwrap()
                };
                let bytes = std::fs::read(&path).unwrap();
                for _ in 0..2 {
                    let error = recover(&state).unwrap_err();
                    assert!(
                        error.contains("Unsupported") && error.contains("journal schema"),
                        "{error}"
                    );
                    assert_eq!(std::fs::read(&path).unwrap(), bytes);
                    assert!(!path.with_extension("json.corrupt").exists());
                    assert_repository_unchanged(&state, &lock);
                }
            }
        }

        #[test]
        fn readonly_journal_quarantine_aborts_and_retries_on_next_startup() {
            if skip_readonly_as_root() {
                return;
            }
            for removal in [false, true] {
                let directory = tempfile::tempdir().unwrap();
                let state = test_state(directory.path());
                let id = "example.readonly-quarantine";
                let lock = installed_fixture(&state, id);
                let (path, label) = if removal {
                    (
                        write_removal_journal(
                            &state,
                            &removal_fixture(&state, lock.get(id).unwrap()),
                        )
                        .unwrap(),
                        "removal-journal-quarantine",
                    )
                } else {
                    (
                        write_journal(
                            &state,
                            &staged_journal(
                                &state,
                                id,
                                Some("1.0.0"),
                                "2.0.0",
                                false,
                                TransactionState::Staged,
                            ),
                        )
                        .unwrap(),
                        "install-journal-quarantine",
                    )
                };
                let corrupted = Corruption::Flipped.apply(&path);
                let readonly = ReadonlyDirectory::new(&journal_dir(&state));
                let error = with_commit_point_action(label, readonly.arm(), || recover(&state))
                    .unwrap_err();
                assert!(error.contains("Cannot quarantine"), "{error}");
                assert_eq!(std::fs::read(&path).unwrap(), corrupted);
                assert!(!path.with_extension("json.corrupt").exists());
                assert_repository_unchanged(&state, &lock);
                drop(readonly);

                let restarted = test_state(directory.path());
                recover(&restarted).unwrap();
                assert!(!path.exists());
                assert_eq!(
                    std::fs::read(path.with_extension("json.corrupt")).unwrap(),
                    corrupted
                );
                assert_repository_unchanged(&restarted, &lock);
            }
        }

        #[test]
        fn readonly_install_journal_update_preserves_the_durable_rollback_record() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            let id = "example.readonly-install-journal";
            let lock = installed_fixture(&state, id);
            let mut journal = staged_journal(
                &state,
                id,
                Some("1.0.0"),
                "2.0.0",
                false,
                TransactionState::Staged,
            );
            let target = journal.target_version.as_ref().unwrap().clone();
            std::fs::create_dir_all(&target).unwrap();
            std::fs::write(target.join("uncommitted"), b"new content").unwrap();
            let path = write_journal(&state, &journal).unwrap();
            let before = std::fs::read(&path).unwrap();
            journal.lock_committed = true;
            journal.state = TransactionState::Activated;
            let readonly = ReadonlyDirectory::new(&journal_dir(&state));
            let error = with_commit_point_action("install-journal-persist", readonly.arm(), || {
                write_journal(&state, &journal)
            })
            .unwrap_err();
            assert!(
                error.contains("Cannot persist extension transaction journal"),
                "{error}"
            );
            assert_eq!(std::fs::read(&path).unwrap(), before);
            assert_repository_unchanged(&state, &lock);
            drop(readonly);

            let restarted = test_state(directory.path());
            for _ in 0..2 {
                recover(&restarted).unwrap();
                assert!(!path.exists());
                assert!(!target.exists());
                assert_repository_unchanged(&restarted, &lock);
            }
        }

        #[test]
        fn readonly_recovery_journal_removal_preserves_pending_work_for_retry() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            let id = "example.readonly-recovery";
            let lock = installed_fixture(&state, id);
            let journal = removal_fixture(&state, lock.get(id).unwrap());
            let path = write_removal_journal(&state, &journal).unwrap();
            let before = std::fs::read(&path).unwrap();
            let readonly = ReadonlyDirectory::new(&journal_dir(&state));
            let error = with_commit_point_action("journal-remove", readonly.arm(), || {
                ExtensionState::from_paths(state.paths.clone()).map(|_| ())
            })
            .unwrap_err();
            assert!(
                error.contains("Cannot remove extension transaction journal"),
                "{error}"
            );
            assert_eq!(std::fs::read(&path).unwrap(), before);
            assert_repository_unchanged(&state, &lock);
            drop(readonly);

            let restarted = test_state(directory.path());
            recover(&restarted).unwrap();
            assert!(!path.exists());
            assert_repository_unchanged(&restarted, &lock);
        }

        #[test]
        fn readonly_recovery_pointer_persist_fails_cleanly_and_rebuilds_on_retry() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            let id = "example.readonly-pointer";
            let lock = installed_fixture(&state, id);
            let path =
                crate::extensions::lock::current_pointer_path(&state.paths.extensions, id).unwrap();
            std::fs::write(&path, b"stale pointer").unwrap();
            let readonly = ReadonlyDirectory::new(path.parent().unwrap());
            let error = with_commit_point_action("current-pointer-persist", readonly.arm(), || {
                ExtensionState::from_paths(state.paths.clone()).map(|_| ())
            })
            .unwrap_err();
            assert!(error.contains("Cannot persist current pointer"), "{error}");
            assert_eq!(std::fs::read(&path).unwrap(), b"stale pointer");
            assert_repository_unchanged(&state, &lock);
            drop(readonly);

            let restarted = test_state(directory.path());
            recover(&restarted).unwrap();
            let pointer: serde_json::Value =
                serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
            assert_eq!(pointer["version"], "1.0.0");
            assert_repository_unchanged(&restarted, &lock);
        }

        #[test]
        #[ignore = "manual Unix process-crash simulation"]
        fn crash_simulation_during_projection_rebuild_replays_committed_journal() {
            const NAME: &str = concat!(
                module_path!(),
                "::crash_simulation_during_projection_rebuild_replays_committed_journal"
            );
            const LABEL: &str = "current-pointer-persist";
            let id = "example.crash-projection";
            if let Some(root) = crash_child_root(NAME) {
                let state = test_state(&root);
                let mut lock = installed_fixture(&state, id);
                write_current_pointer(&state.paths.extensions, lock.get(id).unwrap()).unwrap();
                let journal = staged_journal(
                    &state,
                    id,
                    Some("1.0.0"),
                    "2.0.0",
                    false,
                    TransactionState::Staged,
                );
                let target = journal.target_version.as_ref().unwrap();
                let backup = journal.backup_version.as_ref().unwrap();
                std::fs::create_dir_all(target).unwrap();
                std::fs::write(target.join("keep"), b"committed content").unwrap();
                std::fs::create_dir_all(backup).unwrap();
                std::fs::write(backup.join("old"), b"obsolete backup").unwrap();
                write_journal(&state, &journal).unwrap();
                lock.extensions.insert(id.into(), journal.new_entry.clone());
                lock.save(&state.paths.repository_file).unwrap();
                with_commit_point_action(LABEL, kill_at_commit_point(LABEL), || recover(&state))
                    .unwrap();
                panic!("crash boundary was not reached");
            }
            let directory = tempfile::tempdir().unwrap();
            run_crash_child(NAME, directory.path(), LABEL);
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            let pointer_path =
                crate::extensions::lock::current_pointer_path(&paths.extensions, id).unwrap();
            let stale: serde_json::Value =
                serde_json::from_slice(&std::fs::read(&pointer_path).unwrap()).unwrap();
            assert_eq!(stale["version"], "1.0.0");
            assert!(std::fs::read_dir(paths.extensions.join(".transactions"))
                .unwrap()
                .next()
                .is_none());
            let state = test_state(directory.path());
            for _ in 0..2 {
                recover(&state).unwrap();
                let lock = ExtensionsLock::load(&paths.repository_file).unwrap();
                assert_eq!(lock.extensions.len(), 1);
                assert_eq!(lock.get(id).unwrap().current_version, "2.0.0");
                let pointer: serde_json::Value =
                    serde_json::from_slice(&std::fs::read(&pointer_path).unwrap()).unwrap();
                assert_eq!(pointer["version"], "2.0.0");
                assert_eq!(pointer["previousVersion"], "1.0.0");
                assert_eq!(
                    std::fs::read(paths.extensions.join(id).join("versions/2.0.0/keep")).unwrap(),
                    b"committed content"
                );
                assert!(!paths
                    .extensions
                    .join(id)
                    .join("versions/2.0.0.txn-backup-0000")
                    .exists());
            }
        }
    }

    #[test]
    fn recovery_rebuilds_projections_without_a_journal_directory() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let entry = journal_entry(
            state.paths.root.as_path(),
            "3.0.0",
            "example.projection-without-journal",
        );
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
        let pointer = state
            .paths
            .extensions
            .join(&entry.id)
            .join("current.json");
        if let Some(parent) = pointer.parent() {
            std::fs::create_dir_all(parent).unwrap();
        }
        std::fs::write(&pointer, br#"{"version":"stale"}"#).unwrap();
        std::fs::remove_dir_all(state.paths.extensions.join(".transactions")).ok();
        recover(&state).unwrap();
        let pointer: serde_json::Value = serde_json::from_slice(&std::fs::read(pointer).unwrap()).unwrap();
        assert_eq!(pointer["version"], "3.0.0");
    }
}
