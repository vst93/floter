//! The versioned extension repository and the legacy lock migration adapter.
//!
//! The repository deliberately carries the same entry shape as
//! [`ExtensionsLock`].  This keeps the schema change narrow while existing
//! install, uninstall, sync, and recovery code continues to use the lock type
//! in memory.  Writers are redirected by `ExtensionsLock::save` once a
//! repository has been established; call sites therefore remain unchanged for
//! this slice.

use crate::extensions::lock::{
    sync_directory, ExtensionLockEntry, ExtensionsLock, LOCK_SCHEMA_VERSION,
};
use crate::extensions::ExtensionPaths;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::io::Write;
use std::path::{Path, PathBuf};

pub const REPOSITORY_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionRepository {
    #[serde(default = "default_repository_schema_version")]
    pub schema_version: u32,
    #[serde(default)]
    pub extensions: BTreeMap<String, ExtensionLockEntry>,
}

fn default_repository_schema_version() -> u32 {
    REPOSITORY_SCHEMA_VERSION
}

impl Default for ExtensionRepository {
    fn default() -> Self {
        Self {
            schema_version: REPOSITORY_SCHEMA_VERSION,
            extensions: BTreeMap::new(),
        }
    }
}

impl From<&ExtensionsLock> for ExtensionRepository {
    fn from(lock: &ExtensionsLock) -> Self {
        Self {
            schema_version: REPOSITORY_SCHEMA_VERSION,
            extensions: lock.extensions.clone(),
        }
    }
}

impl ExtensionRepository {
    fn into_lock(self, path: &Path) -> Result<ExtensionsLock, String> {
        if self.schema_version != REPOSITORY_SCHEMA_VERSION {
            return Err(format!(
                "Unsupported extension repository schema version {} in {}",
                self.schema_version,
                path.display()
            ));
        }
        let lock = ExtensionsLock {
            schema_version: LOCK_SCHEMA_VERSION,
            extensions: self.extensions,
        };
        lock.validate_entries()
            .map_err(|error| format!("Invalid extension repository {}: {error}", path.display()))?;
        Ok(lock)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MigrationOutcome {
    Noop,
    Migrated,
}

pub fn repository_path(lock_path: &Path) -> PathBuf {
    lock_path
        .parent()
        .unwrap_or_else(|| Path::new("."))
        .join("extension-repository.json")
}

fn migrated_lock_path(lock_path: &Path) -> PathBuf {
    lock_path.with_file_name("extensions.lock.json.migrated")
}

fn corrupt_repository_path(repository_path: &Path) -> PathBuf {
    repository_path.with_file_name("extension-repository.json.corrupt")
}

pub(crate) fn is_legacy_lock_path(path: &Path) -> bool {
    path.file_name()
        .and_then(|name| name.to_str())
        .is_some_and(|name| name == "extensions.lock.json")
}

fn read_repository(path: &Path) -> Result<ExtensionsLock, String> {
    let bytes = std::fs::read(path)
        .map_err(|error| format!("Cannot read extension repository {}: {error}", path.display()))?;
    let repository: ExtensionRepository = serde_json::from_slice(&bytes).map_err(|error| {
        format!("Invalid extension repository {}: {error}", path.display())
    })?;
    repository.into_lock(path)
}

fn write_repository(path: &Path, lock: &ExtensionsLock) -> Result<(), String> {
    let parent = path
        .parent()
        .ok_or("Invalid extension repository path")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create repository directory: {error}"))?;
    let bytes = serde_json::to_vec_pretty(&ExtensionRepository::from(lock))
        .map_err(|error| format!("Cannot serialize extension repository: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create extension repository temporary file: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write extension repository: {error}"))?;
    temporary
        .persist(path)
        .map_err(|error| format!("Cannot persist extension repository: {error}"))?;
    sync_directory(parent)
        .map_err(|error| format!("Cannot sync extension repository directory: {error}"))
}

fn archive_file(path: &Path, archive: &Path, label: &str) -> Result<(), String> {
    if !path.exists() {
        return Ok(());
    }
    if archive.exists() {
        std::fs::remove_file(archive)
            .map_err(|error| format!("Cannot replace {label} archive {}: {error}", archive.display()))?;
    }
    std::fs::rename(path, archive)
        .map_err(|error| format!("Cannot archive {label} {}: {error}", path.display()))?;
    if let Some(parent) = archive.parent() {
        sync_directory(parent)
            .map_err(|error| format!("Cannot sync {label} archive directory: {error}"))?;
    }
    Ok(())
}

/// Migrate the live legacy lock to the repository. The repository is written
/// and synced before the lock is renamed, so a crash leaves either the old
/// lock or a complete repository. When both files exist, the lock was written
/// by the pre-slice-5 writers; if it contains IDs absent from the repository,
/// its contents win and are migrated again.
pub(crate) fn migrate_to_repository(
    paths: &ExtensionPaths,
) -> Result<MigrationOutcome, String> {
    let lock_path = &paths.lock_file;
    let repository = repository_path(lock_path);
    if !lock_path.exists() {
        return Ok(MigrationOutcome::Noop);
    }

    let lock = ExtensionsLock::load_legacy(lock_path)?;
    if repository.exists() {
        let current = read_repository(&repository)?;
        let lock_differs = serde_json::to_value(&lock.extensions).ok()
            != serde_json::to_value(&current.extensions).ok();
        if !lock_differs {
            return Ok(MigrationOutcome::Noop);
        }
        // The legacy lock was the live writer until Slice 5 swaps writers. Its
        // complete snapshot wins whenever it differs, including removals and
        // edits, while the repository remains the first load candidate.
    }

    write_repository(&repository, &lock)?;
    archive_file(
        lock_path,
        &migrated_lock_path(lock_path),
        "legacy extension lock",
    )?;
    Ok(MigrationOutcome::Migrated)
}

pub(crate) fn load_for_legacy_path(lock_path: &Path) -> Result<ExtensionsLock, String> {
    let repository = repository_path(lock_path);
    if repository.exists() {
        match read_repository(&repository) {
            Ok(repository_lock) => {
                // Normally the old lock has already been archived. If a
                // pre-slice-5 writer left it beside the repository, its live
                // snapshot wins whenever it differs from the repository.
                if lock_path.exists() {
                    match ExtensionsLock::load_legacy(lock_path) {
                        Ok(lock)
                            if serde_json::to_value(&lock.extensions).ok()
                                != serde_json::to_value(&repository_lock.extensions).ok() =>
                        {
                            let paths = ExtensionPaths::from_root(
                                lock_path
                                    .parent()
                                    .unwrap_or_else(|| Path::new("."))
                                    .to_path_buf(),
                            );
                            if let Err(error) = migrate_to_repository(&paths) {
                                tracing::warn!(
                                    "Extension repository re-migration failed; using repository: {error}"
                                );
                            } else if let Ok(lock) = read_repository(&repository) {
                                return Ok(lock);
                            }
                        }
                        Ok(_) => {}
                        Err(error) => tracing::warn!(
                            "Ignoring invalid legacy extension lock beside repository: {error}"
                        ),
                    }
                }
                return Ok(repository_lock);
            }
            Err(error) => {
                tracing::warn!("Extension repository is invalid; archiving it: {error}");
                if let Err(error) = archive_file(
                    &repository,
                    &corrupt_repository_path(&repository),
                    "corrupt extension repository",
                ) {
                    tracing::warn!("Cannot archive corrupt extension repository; trying legacy data: {error}");
                }
            }
        }
    }

    if lock_path.exists() {
        match ExtensionsLock::load_legacy(lock_path) {
            Ok(lock) => {
                let paths = ExtensionPaths::from_root(
                    lock_path
                        .parent()
                        .unwrap_or_else(|| Path::new("."))
                        .to_path_buf(),
                );
                if let Err(error) = migrate_to_repository(&paths) {
                    tracing::warn!("Extension repository migration failed; using legacy lock: {error}");
                }
                return Ok(lock);
            }
            Err(error) => tracing::warn!("Invalid extension lock; trying migrated archive: {error}"),
        }
    }

    let migrated = migrated_lock_path(lock_path);
    if migrated.exists() {
        match ExtensionsLock::load_legacy(&migrated) {
            Ok(lock) => return Ok(lock),
            Err(error) => tracing::warn!(
                "Invalid migrated extension lock; starting with an empty repository: {error}"
            ),
        }
    }

    tracing::warn!(
        "No valid extension repository or legacy lock at {}; starting with an empty lock",
        lock_path.display()
    );
    Ok(ExtensionsLock::default())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn fixture_lock(id: &str) -> ExtensionsLock {
        let entry: ExtensionLockEntry = serde_json::from_value(serde_json::json!({
            "id": id,
            "name": "Example",
            "publisherId": "example",
            "publisherName": "Example",
            "distributionSource": "local",
            "runtimeOwnership": "system",
            "providerKind": "executable",
            "state": "enabled",
            "enabled": true,
            "packageName": null,
            "packageVersion": "local",
            "toolVersion": null,
            "integrity": null,
            "runtimeIntegrity": null,
            "contentIntegrity": null,
            "previousIntegrity": null,
            "previousRuntimeIntegrity": null,
            "previousContentIntegrity": null,
            "assetSelection": null,
            "signatureVerified": false,
            "previousSignatureVerified": null,
            "officialVerified": false,
            "previousOfficialVerified": null,
            "currentVersion": "local",
            "previousVersion": null,
            "manifestPath": "/tmp/example.json",
            "executablePath": "/tmp/example",
            "runtimeRoot": null,
            "installedAt": 1,
            "updatedAt": 1,
            "pinned": false,
            "channel": "external",
            "approvedPermissions": [],
            "approvedAt": 0,
            "approvedManifestDigest": null,
            "lastErrorCode": null,
            "lastErrorDetail": null,
            "lastErrorAt": null,
            "brokenReason": null,
            "enabledBeforeBroken": null
        }))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(id.to_string(), entry);
        lock
    }

    #[test]
    fn migrate_from_legacy_lock_is_atomic_and_idempotent() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.repository");
        lock.save_legacy(&paths.lock_file).unwrap();

        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Migrated
        );
        let repository = read_repository(&repository_path(&paths.lock_file)).unwrap();
        assert_eq!(
            serde_json::to_value(&repository.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(!paths.lock_file.exists());
        assert!(migrated_lock_path(&paths.lock_file).exists());
        assert_eq!(migrate_to_repository(&paths).unwrap(), MigrationOutcome::Noop);
    }

    #[test]
    fn loader_migrates_an_existing_legacy_lock() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.legacy");
        lock.save_legacy(&paths.lock_file).unwrap();
        let loaded = ExtensionsLock::load(&paths.lock_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(repository_path(&paths.lock_file).exists());
        assert!(migrated_lock_path(&paths.lock_file).exists());
    }

    #[test]
    fn migration_prefers_legacy_entries_missing_from_repository() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let repository_lock = fixture_lock("example.repository");
        write_repository(&repository_path(&paths.lock_file), &repository_lock).unwrap();
        let legacy_lock = fixture_lock("example.legacy");
        legacy_lock.save_legacy(&paths.lock_file).unwrap();
        assert_eq!(migrate_to_repository(&paths).unwrap(), MigrationOutcome::Migrated);
        let loaded = read_repository(&repository_path(&paths.lock_file)).unwrap();
        assert!(loaded.extensions.contains_key("example.legacy"));
        assert!(!loaded.extensions.contains_key("example.repository"));
    }

    #[test]
    fn invalid_repository_and_legacy_lock_degrade_to_empty() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        std::fs::write(repository_path(&paths.lock_file), b"not json").unwrap();
        std::fs::write(&paths.lock_file, b"not json").unwrap();
        let loaded = ExtensionsLock::load(&paths.lock_file).unwrap();
        assert!(loaded.extensions.is_empty());
        assert!(corrupt_repository_path(&repository_path(&paths.lock_file)).exists());
    }

    #[test]
    fn loader_prefers_repository_and_recovers_corrupt_repository_from_archive() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.repository");
        write_repository(&repository_path(&paths.lock_file), &lock).unwrap();
        let loaded = ExtensionsLock::load(&paths.lock_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );

        std::fs::remove_file(repository_path(&paths.lock_file)).unwrap();
        lock.save_legacy(&migrated_lock_path(&paths.lock_file)).unwrap();
        std::fs::write(&repository_path(&paths.lock_file), b"not json").unwrap();
        let loaded = ExtensionsLock::load(&paths.lock_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(corrupt_repository_path(&repository_path(&paths.lock_file)).exists());
    }
}
