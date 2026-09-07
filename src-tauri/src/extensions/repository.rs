//! The versioned extension repository and the legacy lock migration adapter.
//!
//! The repository deliberately carries the same entry shape as
//! [`ExtensionsLock`].  This keeps the schema change narrow while existing
//! install, uninstall, sync, and recovery code continues to use the lock type
//! in memory. `ExtensionsLock::save` always writes the repository schema;
//! legacy lock files and their migrated archives are read-only fallback inputs.

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
    pub schema_version: u32,
    pub extensions: BTreeMap<String, ExtensionLockEntry>,
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
    let bytes = std::fs::read(path).map_err(|error| {
        format!(
            "Cannot read extension repository {}: {error}",
            path.display()
        )
    })?;
    let repository: ExtensionRepository = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid extension repository {}: {error}", path.display()))?;
    repository.into_lock(path)
}

pub(crate) fn write_repository(path: &Path, lock: &ExtensionsLock) -> Result<(), String> {
    if is_legacy_lock_path(path)
        || path
            .file_name()
            .is_some_and(|name| name == "extensions.lock.json.migrated")
    {
        let repository = repository_path(path);
        tracing::warn!(
            legacy_path = %path.display(),
            repository_path = %repository.display(),
            "Legacy extension state save target; redirecting to repository"
        );
        return write_repository(&repository, lock);
    }
    let parent = path.parent().ok_or("Invalid extension repository path")?;
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
    crate::extensions::commit_point("repository-persist");
    temporary
        .persist(path)
        .map_err(|error| format!("Cannot persist extension repository: {error}"))?;
    crate::extensions::commit_point("repository-directory-sync");
    sync_directory(parent)
        .map_err(|error| format!("Cannot sync extension repository directory: {error}"))
}

fn archive_file(path: &Path, archive: &Path, label: &str) -> Result<(), String> {
    if !path.exists() {
        return Ok(());
    }
    if archive.exists() {
        std::fs::remove_file(archive).map_err(|error| {
            format!(
                "Cannot replace {label} archive {}: {error}",
                archive.display()
            )
        })?;
    }
    crate::extensions::commit_point("repository-archive-rename");
    std::fs::rename(path, archive)
        .map_err(|error| format!("Cannot archive {label} {}: {error}", path.display()))?;
    if let Some(parent) = archive.parent() {
        crate::extensions::commit_point("repository-archive-directory-sync");
        sync_directory(parent)
            .map_err(|error| format!("Cannot sync {label} archive directory: {error}"))?;
    }
    Ok(())
}

/// Migrate the legacy lock to the repository. The repository is written
/// and synced before the lock is renamed, so a crash leaves either the old
/// lock or a complete repository. A valid repository is authoritative even
/// when a legacy file remains beside it (for example, after an archive failure).
pub(crate) fn migrate_to_repository(paths: &ExtensionPaths) -> Result<MigrationOutcome, String> {
    let lock_path = &paths.lock_file;
    let repository = &paths.repository_file;
    if repository.exists() {
        read_repository(repository)?;
        return Ok(MigrationOutcome::Noop);
    }
    if !lock_path.exists() {
        return Ok(MigrationOutcome::Noop);
    }

    let lock = ExtensionsLock::load_legacy(lock_path)?;
    if lock.extensions.is_empty() && corrupt_repository_path(repository).exists() {
        return Err("Cannot migrate empty legacy state over a corrupt repository".into());
    }

    write_repository(repository, &lock)?;
    archive_file(
        lock_path,
        &migrated_lock_path(lock_path),
        "legacy extension lock",
    )?;
    Ok(MigrationOutcome::Migrated)
}

pub(crate) fn load_for_legacy_path(lock_path: &Path) -> Result<ExtensionsLock, String> {
    let repository = repository_path(lock_path);
    let had_repository = repository.exists() || corrupt_repository_path(&repository).exists();
    let had_state = had_repository
        || lock_path.exists()
        || migrated_lock_path(lock_path).exists();
    if repository.exists() {
        match read_repository(&repository) {
            Ok(repository_lock) => return Ok(repository_lock),
            Err(error) => {
                tracing::warn!("Extension repository is invalid; archiving it: {error}");
                if let Err(error) = archive_file(
                    &repository,
                    &corrupt_repository_path(&repository),
                    "corrupt extension repository",
                ) {
                    tracing::warn!(
                        "Cannot archive corrupt extension repository; trying legacy data: {error}"
                    );
                }
            }
        }
    }

    if lock_path.exists() {
        match ExtensionsLock::load_legacy(lock_path) {
            Ok(lock) if !had_repository || !lock.extensions.is_empty() => {
                let paths = ExtensionPaths::from_root(
                    lock_path
                        .parent()
                        .unwrap_or_else(|| Path::new("."))
                        .to_path_buf(),
                );
                if let Err(error) = migrate_to_repository(&paths) {
                    tracing::warn!(
                        "Extension repository migration failed; using legacy lock: {error}"
                    );
                }
                return Ok(lock);
            }
            Ok(_) => {}
            Err(error) => {
                tracing::warn!("Invalid extension lock; trying migrated archive: {error}")
            }
        }
    }

    let migrated = migrated_lock_path(lock_path);
    if migrated.exists() {
        match ExtensionsLock::load_legacy(&migrated) {
            Ok(lock) if !had_repository || !lock.extensions.is_empty() => return Ok(lock),
            Ok(_) => {}
            Err(error) => tracing::warn!(
                "Invalid migrated extension lock: {error}"
            ),
        }
    }

    if had_state {
        return Err(format!(
            "No valid extension repository or legacy lock at {}; refusing to use an empty state",
            lock_path.display()
        ));
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
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
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
    fn migration_and_loader_keep_repository_authoritative_beside_legacy() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let repository_lock = fixture_lock("example.repository");
        write_repository(&repository_path(&paths.lock_file), &repository_lock).unwrap();
        let legacy_lock = fixture_lock("example.legacy");
        legacy_lock.save_legacy(&paths.lock_file).unwrap();
        let legacy_bytes = std::fs::read(&paths.lock_file).unwrap();
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
        for path in [&paths.lock_file, &paths.repository_file] {
            let loaded = ExtensionsLock::load(path).unwrap();
            assert!(loaded.extensions.contains_key("example.repository"));
            assert!(!loaded.extensions.contains_key("example.legacy"));
        }

        ExtensionsLock::default()
            .save(&paths.repository_file)
            .unwrap();
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
        assert!(ExtensionsLock::load(&paths.lock_file)
            .unwrap()
            .extensions
            .is_empty());
        assert_eq!(std::fs::read(&paths.lock_file).unwrap(), legacy_bytes);
    }

    #[test]
    fn legacy_write_tripwire_warns_and_redirects_without_changing_legacy_files() {
        for migrated in [false, true] {
            let directory = tempfile::tempdir().unwrap();
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            let original = fixture_lock("example.legacy");
            original.save_legacy(&paths.lock_file).unwrap();
            let legacy_bytes = std::fs::read(&paths.lock_file).unwrap();
            if migrated {
                migrate_to_repository(&paths).unwrap();
            }
            let log_path = directory.path().join("tripwire.log");
            let subscriber = tracing_subscriber::fmt()
                .without_time()
                .with_ansi(false)
                .with_max_level(tracing::Level::WARN)
                .with_writer(std::fs::File::create(&log_path).unwrap())
                .finish();
            let changed = fixture_lock("example.updated");
            tracing::subscriber::with_default(subscriber, || {
                changed.save(&paths.lock_file).unwrap();
                if migrated {
                    changed.save(&migrated_lock_path(&paths.lock_file)).unwrap();
                }
            });

            let logs = std::fs::read_to_string(log_path).unwrap();
            assert!(logs.contains("WARN"), "{logs}");
            assert!(
                logs.contains("Legacy extension state save target; redirecting to repository"),
                "{logs}"
            );
            assert!(
                logs.contains(&paths.lock_file.display().to_string()),
                "{logs}"
            );
            let repository = read_repository(&paths.repository_file).unwrap();
            assert!(repository.extensions.contains_key("example.updated"));
            assert!(!repository.extensions.contains_key("example.legacy"));
            if migrated {
                assert!(!paths.lock_file.exists());
                assert_eq!(
                    std::fs::read(migrated_lock_path(&paths.lock_file)).unwrap(),
                    legacy_bytes
                );
            } else {
                assert_eq!(std::fs::read(&paths.lock_file).unwrap(), legacy_bytes);
            }
        }
    }

    #[test]
    fn repository_save_load_round_trip_without_legacy_files() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().join("new-root"));
        let lock = fixture_lock("example.fresh");
        lock.save(&paths.repository_file).unwrap();
        let json: serde_json::Value =
            serde_json::from_slice(&std::fs::read(&paths.repository_file).unwrap()).unwrap();
        assert_eq!(json["schemaVersion"], REPOSITORY_SCHEMA_VERSION);
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(loaded).unwrap(),
            serde_json::to_value(lock).unwrap()
        );
        assert!(!paths.lock_file.exists());
        assert!(!migrated_lock_path(&paths.lock_file).exists());
        assert_eq!(std::fs::read_dir(&paths.root).unwrap().count(), 1);
    }

    #[test]
    fn repository_load_falls_back_when_migration_archive_fails() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.legacy");
        lock.save_legacy(&paths.lock_file).unwrap();
        let legacy_bytes = std::fs::read(&paths.lock_file).unwrap();
        // An archive directory makes the rename fail after the repository commit.
        std::fs::create_dir(migrated_lock_path(&paths.lock_file)).unwrap();
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert!(loaded.extensions.contains_key("example.legacy"));
        assert!(paths.repository_file.exists());
        ExtensionsLock::default()
            .save(&paths.repository_file)
            .unwrap();
        assert!(ExtensionsLock::load(&paths.repository_file)
            .unwrap()
            .extensions
            .is_empty());
        assert_eq!(std::fs::read(&paths.lock_file).unwrap(), legacy_bytes);
    }

    #[test]
    fn invalid_repository_and_legacy_lock_abort_loading() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        std::fs::write(repository_path(&paths.lock_file), b"not json").unwrap();
        std::fs::write(&paths.lock_file, b"not json").unwrap();
        assert!(ExtensionsLock::load(&paths.lock_file).is_err());
        assert!(ExtensionsLock::load(&paths.lock_file).is_err());
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
        lock.save_legacy(&migrated_lock_path(&paths.lock_file))
            .unwrap();
        std::fs::write(&repository_path(&paths.lock_file), b"not json").unwrap();
        let loaded = ExtensionsLock::load(&paths.lock_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(corrupt_repository_path(&repository_path(&paths.lock_file)).exists());
        let archive_bytes = std::fs::read(migrated_lock_path(&paths.lock_file)).unwrap();
        loaded.save(&paths.repository_file).unwrap();
        assert!(ExtensionsLock::load(&paths.repository_file)
            .unwrap()
            .extensions
            .contains_key("example.repository"));
        assert!(!paths.lock_file.exists());
        assert_eq!(
            std::fs::read(migrated_lock_path(&paths.lock_file)).unwrap(),
            archive_bytes
        );
    }

    #[test]
    fn fresh_state_recovery_does_not_preempt_legacy_migration() {
        let directory = tempfile::tempdir().unwrap();
        let state = crate::extensions::ExtensionState::from_paths(ExtensionPaths::from_root(
            directory.path().to_path_buf(),
        ))
        .unwrap();
        assert!(!state.paths.repository_file.exists());
        assert!(!state.paths.extensions.join(".transactions").exists());
        let lock = fixture_lock("example.late-migration");
        lock.save_legacy(&state.paths.lock_file).unwrap();
        let legacy_bytes = std::fs::read(&state.paths.lock_file).unwrap();

        assert_eq!(migrate_to_repository(&state.paths).unwrap(), MigrationOutcome::Migrated);
        assert_eq!(
            std::fs::read(migrated_lock_path(&state.paths.lock_file)).unwrap(),
            legacy_bytes
        );
        assert!(!state.paths.lock_file.exists());
        assert!(read_repository(&state.paths.repository_file)
            .unwrap()
            .extensions
            .contains_key("example.late-migration"));
    }

    #[test]
    fn fault_at_legacy_archive_rename_keeps_repository_authoritative() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.archive-fault");
        lock.save_legacy(&paths.lock_file).unwrap();
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            crate::extensions::with_commit_point("repository-archive-rename", || {
                migrate_to_repository(&paths).unwrap();
            });
        }));
        assert!(result.is_err());
        assert!(read_repository(&paths.repository_file)
            .unwrap()
            .extensions
            .contains_key("example.archive-fault"));
        assert!(paths.lock_file.exists());
        assert_eq!(migrate_to_repository(&paths).unwrap(), MigrationOutcome::Noop);
    }

    #[cfg(unix)]
    mod unix_faults {
        use super::*;
        use crate::extensions::fault_test_support::{
            skip_readonly_as_root, Corruption, ReadonlyDirectory,
        };
        use crate::extensions::{transaction, with_commit_point_action, ExtensionState};

        #[test]
        fn corrupt_repository_aborts_repeated_recovery_without_erasing_extensions() {
            for corruption in Corruption::ALL {
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let state = ExtensionState::from_paths(paths.clone()).unwrap();
                let lock = fixture_lock("example.corrupt-repository");
                lock.save(&paths.repository_file).unwrap();
                let integration = paths.data.join("example.corrupt-repository/integration");
                std::fs::create_dir_all(&integration).unwrap();
                std::fs::write(integration.join("keep"), b"installed content").unwrap();
                std::fs::create_dir_all(paths.extensions.join(".transactions")).unwrap();
                let corrupted = corruption.apply(&paths.repository_file);

                for _ in 0..2 {
                    let error = transaction::recover(&state).unwrap_err();
                    assert!(error.contains("refusing to use an empty state"), "{error}");
                    assert!(ExtensionsLock::load(&paths.repository_file).is_err());
                    assert!(ExtensionState::from_paths(paths.clone()).is_err());
                    assert!(!paths.repository_file.exists());
                    assert_eq!(
                        std::fs::read(corrupt_repository_path(&paths.repository_file)).unwrap(),
                        corrupted
                    );
                    assert_eq!(
                        std::fs::read(integration.join("keep")).unwrap(),
                        b"installed content"
                    );
                }
            }
        }

        #[test]
        fn corrupt_repository_recovers_each_valid_legacy_fallback() {
            for corruption in Corruption::ALL {
                for migrated in [false, true] {
                    let directory = tempfile::tempdir().unwrap();
                    let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                    let state = ExtensionState::from_paths(paths.clone()).unwrap();
                    let lock = fixture_lock("example.corrupt-fallback");
                    let fallback = if migrated {
                        migrated_lock_path(&paths.lock_file)
                    } else {
                        paths.lock_file.clone()
                    };
                    lock.save_legacy(&fallback).unwrap();
                    let legacy_bytes = std::fs::read(&fallback).unwrap();
                    lock.save(&paths.repository_file).unwrap();
                    let corrupted = corruption.apply(&paths.repository_file);

                    for _ in 0..2 {
                        transaction::recover(&state).unwrap();
                        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
                        assert_eq!(
                            serde_json::to_value(&loaded).unwrap(),
                            serde_json::to_value(&lock).unwrap()
                        );
                        assert_eq!(
                            std::fs::read(corrupt_repository_path(&paths.repository_file)).unwrap(),
                            corrupted
                        );
                        assert_eq!(
                            std::fs::read(migrated_lock_path(&paths.lock_file)).unwrap(),
                            legacy_bytes
                        );
                        assert!(!paths.lock_file.exists());
                        // An archived fallback is read in place; a live lock is migrated.
                        assert_eq!(paths.repository_file.exists(), !migrated);
                    }
                }
            }
        }

        #[test]
        fn corrupt_repository_rejects_empty_legacy_fallbacks() {
            for migrated in [false, true] {
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let state = ExtensionState::from_paths(paths.clone()).unwrap();
                fixture_lock("example.nonempty")
                    .save(&paths.repository_file)
                    .unwrap();
                let fallback = if migrated {
                    migrated_lock_path(&paths.lock_file)
                } else {
                    paths.lock_file.clone()
                };
                ExtensionsLock::default().save_legacy(&fallback).unwrap();
                let before = std::fs::read(&fallback).unwrap();
                let corrupted = Corruption::Truncated.apply(&paths.repository_file);
                for _ in 0..2 {
                    assert!(transaction::recover(&state).is_err());
                    assert!(ExtensionState::from_paths(paths.clone()).is_err());
                    assert!(!paths.repository_file.exists());
                    assert_eq!(std::fs::read(&fallback).unwrap(), before);
                    assert_eq!(
                        std::fs::read(corrupt_repository_path(&paths.repository_file)).unwrap(),
                        corrupted
                    );
                }
            }
        }

        #[test]
        fn invalid_repository_schema_cannot_default_to_authoritative_empty_state() {
            let invalid = [
                serde_json::json!({}),
                serde_json::json!({"schemaVersion": REPOSITORY_SCHEMA_VERSION}),
                serde_json::json!({"extensions": {}}),
                serde_json::json!({"schemaVersion": REPOSITORY_SCHEMA_VERSION + 1, "extensions": {}}),
                {
                    let mut value = serde_json::to_value(ExtensionRepository::from(&fixture_lock(
                        "example.schema",
                    )))
                    .unwrap();
                    value["extensions"]["example.schema"]["id"] = "example.mismatched".into();
                    value
                },
            ];
            for value in invalid {
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let bytes = serde_json::to_vec(&value).unwrap();
                std::fs::write(&paths.repository_file, &bytes).unwrap();
                for _ in 0..2 {
                    assert!(
                        ExtensionsLock::load(&paths.repository_file).is_err(),
                        "{value}"
                    );
                    assert!(!paths.repository_file.exists());
                    assert_eq!(
                        std::fs::read(corrupt_repository_path(&paths.repository_file)).unwrap(),
                        bytes
                    );
                }
            }
        }

        #[test]
        fn readonly_corrupt_repository_archive_aborts_and_retries_cleanly() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            let state = ExtensionState::from_paths(paths.clone()).unwrap();
            fixture_lock("example.readonly-corrupt")
                .save(&paths.repository_file)
                .unwrap();
            let corrupted = Corruption::Flipped.apply(&paths.repository_file);
            let readonly = ReadonlyDirectory::new(&paths.root);
            let error =
                with_commit_point_action("repository-archive-rename", readonly.arm(), || {
                    transaction::recover(&state)
                })
                .unwrap_err();
            assert!(error.contains("refusing to use an empty state"), "{error}");
            assert_eq!(std::fs::read(&paths.repository_file).unwrap(), corrupted);
            assert!(!corrupt_repository_path(&paths.repository_file).exists());
            drop(readonly);

            assert!(transaction::recover(&state).is_err());
            assert!(!paths.repository_file.exists());
            assert_eq!(
                std::fs::read(corrupt_repository_path(&paths.repository_file)).unwrap(),
                corrupted
            );
            assert!(transaction::recover(&state).is_err());
        }
    }
}
