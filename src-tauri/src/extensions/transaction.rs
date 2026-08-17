use crate::extensions::lock::{
    sync_directory, write_current_pointer, ExtensionLockEntry, ExtensionsLock,
};
use crate::extensions::ExtensionState;
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::{Path, PathBuf};

pub const TRANSACTION_JOURNAL_SCHEMA_VERSION: u32 = 1;

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

pub(crate) fn recover(state: &ExtensionState) -> Result<(), String> {
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
            remove_journal(path)?;
        } else {
            if let Some(target) = &journal.target_version {
                if target.exists() {
                    let _ = std::fs::remove_dir_all(target);
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

fn rebuild_current_pointers(state: &ExtensionState, lock: &ExtensionsLock) -> Result<(), String> {
    for entry in lock.extensions.values() {
        let _ = write_current_pointer(&state.paths.extensions, entry);
    }
    Ok(())
}

pub(crate) fn commit_version(
    state: &ExtensionState,
    old: Option<&ExtensionLockEntry>,
    entry: &ExtensionLockEntry,
    staged_version: &Path,
    target: &Path,
) -> Result<(), String> {
    let backup = if target.exists() {
        Some(target.with_extension(format!("txn-backup-{}", uuid::Uuid::new_v4())))
    } else {
        None
    };
    let journal = InstallationJournal {
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
    };
    let journal_path = write_journal(state, &journal)?;
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
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    lock.extensions.insert(entry.id.clone(), entry.clone());
    lock.save(&state.paths.lock_file)?;
    let mut committed_journal = journal.clone();
    committed_journal.lock_committed = true;
    write_journal(state, &committed_journal)?;
    let _ = write_current_pointer(&state.paths.extensions, entry);
    if let Some(backup) = backup {
        if backup.exists() {
            std::fs::remove_dir_all(backup)
                .map_err(|error| format!("Cannot remove previous extension version: {error}"))?;
        }
    }
    remove_journal(&journal_path)?;
    Ok(())
}

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
    };
    let journal_path = write_journal(state, &journal)?;
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
