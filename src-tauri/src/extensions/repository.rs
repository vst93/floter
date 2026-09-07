//! The authoritative extension repository and startup migration recovery.
//!
//! The repository deliberately carries the same entry shape as
//! [`ExtensionsLock`].  This keeps the schema change narrow while existing
//! install, uninstall, sync, and recovery code continues to use the lock type
//! in memory. Normal loads and saves use only `extension-repository.json`.
//! Startup recovery imports legacy files into a durable repository before
//! journals or projections can be processed; it never returns legacy state
//! directly to consumers.

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

fn migrated_lock_path(lock_path: &Path) -> PathBuf {
    lock_path.with_file_name("extensions.lock.json.migrated")
}

fn corrupt_repository_path(repository_path: &Path) -> PathBuf {
    repository_path.with_file_name("extension-repository.json.corrupt")
}

fn require_repository_path(path: &Path) -> Result<(), String> {
    if path
        .file_name()
        .is_some_and(|name| name == "extension-repository.json")
    {
        Ok(())
    } else {
        Err(format!(
            "Extension state must use extension-repository.json; unsupported state path {}",
            path.display()
        ))
    }
}

fn state_file_exists(path: &Path) -> Result<bool, String> {
    path.try_exists()
        .map_err(|error| format!("Cannot inspect extension state {}: {error}", path.display()))
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
    require_repository_path(path)?;
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

/// Read only the repository. Evidence of prior state requires explicit recovery;
/// it must never turn a missing or corrupt repository into an empty registry.
pub(crate) fn load_repository(path: &Path) -> Result<ExtensionsLock, String> {
    require_repository_path(path)?;
    if state_file_exists(path)? {
        return match read_repository(path) {
            Ok(lock) => Ok(lock),
            Err(error) => {
                tracing::warn!("Extension repository is invalid; archiving it: {error}");
                let archive_error = archive_file(
                    path,
                    &corrupt_repository_path(path),
                    "corrupt extension repository",
                )
                .err()
                .map(|error| format!("; {error}"))
                .unwrap_or_default();
                Err(format!(
                    "{error}{archive_error}; refusing to use an empty state"
                ))
            }
        };
    }
    for prior in [
        corrupt_repository_path(path),
        path.with_file_name("extensions.lock.json"),
        migrated_lock_path(path),
    ] {
        if state_file_exists(&prior)? {
            return Err(format!(
                "Missing extension repository {}; found {}; startup recovery required; refusing to use an empty state",
                path.display(), prior.display()
            ));
        }
    }
    Ok(ExtensionsLock::default())
}

/// The only legacy decoder. Missing files are errors, never empty state.
fn read_migration_input(path: &Path) -> Result<ExtensionsLock, String> {
    let bytes = std::fs::read(path).map_err(|error| {
        format!(
            "Cannot read extension migration input {}: {error}",
            path.display()
        )
    })?;
    let lock: ExtensionsLock = serde_json::from_slice(&bytes).map_err(|error| {
        format!(
            "Invalid extension migration input {}: {error}",
            path.display()
        )
    })?;
    if lock.schema_version != LOCK_SCHEMA_VERSION {
        return Err(format!(
            "Unsupported extension migration schema version {} in {}",
            lock.schema_version,
            path.display()
        ));
    }
    lock.validate_entries().map_err(|error| {
        format!(
            "Invalid extension migration input {}: {error}",
            path.display()
        )
    })?;
    Ok(lock)
}

/// Prepare repository state before recovery can touch installed files. A valid
/// repository always wins. Otherwise import pre-repository state or recover a
/// migration interrupted in an older build, including an archived corrupt repo.
/// The new repository is synced before archiving a live legacy input; when
/// `.migrated` is the recovery input, its bytes stay intact.
pub(crate) fn migrate_to_repository(paths: &ExtensionPaths) -> Result<MigrationOutcome, String> {
    let repository = &paths.repository_file;
    let mut failure = match load_repository(repository) {
        Ok(_) => return Ok(MigrationOutcome::Noop),
        Err(error) => error,
    };
    // A failed quarantine must not overwrite the only copy of damaged state.
    if state_file_exists(repository)? {
        return Err(failure);
    }
    let had_repository = state_file_exists(&corrupt_repository_path(repository))?;
    let migrated = migrated_lock_path(&paths.legacy_lock_file);
    for input in [&paths.legacy_lock_file, &migrated] {
        if !state_file_exists(input)? {
            continue;
        }
        let lock = match read_migration_input(input) {
            Ok(lock) if !had_repository || !lock.extensions.is_empty() => lock,
            Ok(_) => {
                failure.push_str(&format!(
                    "; refusing empty migration input {} after repository corruption",
                    input.display()
                ));
                continue;
            }
            Err(error) => {
                failure.push_str(&format!("; {error}"));
                continue;
            }
        };
        write_repository(repository, &lock).map_err(|error| {
            format!(
                "Cannot migrate {} to {}: {error}",
                input.display(),
                repository.display()
            )
        })?;
        if input == &paths.legacy_lock_file {
            if let Err(error) = archive_file(input, &migrated, "legacy extension lock") {
                // Commit and directory sync succeeded; an unused input can safely
                // coexist with the authoritative repository on the next startup.
                tracing::warn!("Repository migration committed; legacy archive pending: {error}");
            }
        }
        tracing::info!(input = %input.display(), "Migrated extension state to repository");
        return Ok(MigrationOutcome::Migrated);
    }
    Err(failure)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::ExtensionState;

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
        lock.save_legacy(&paths.legacy_lock_file).unwrap();

        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Migrated
        );
        let repository = read_repository(&paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(&repository.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(!paths.legacy_lock_file.exists());
        assert!(migrated_lock_path(&paths.legacy_lock_file).exists());
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
    }

    #[test]
    fn legacy_only_tree_requires_startup_migration() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.legacy");
        lock.save_legacy(&paths.legacy_lock_file).unwrap();
        let legacy_bytes = std::fs::read(&paths.legacy_lock_file).unwrap();
        let integration = paths.data.join("example.legacy/integration");
        std::fs::create_dir_all(&integration).unwrap();
        std::fs::write(integration.join("keep"), b"installed content").unwrap();
        let error = ExtensionsLock::load(&paths.repository_file).unwrap_err();
        assert!(
            error.contains(&paths.legacy_lock_file.display().to_string()),
            "{error}"
        );
        assert!(!paths.repository_file.exists());
        assert_eq!(
            std::fs::read(&paths.legacy_lock_file).unwrap(),
            legacy_bytes
        );
        ExtensionState::from_paths(paths.clone()).unwrap();
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(paths.repository_file.exists());
        assert!(!paths.legacy_lock_file.exists());
        assert_eq!(
            std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
            legacy_bytes
        );
        let repository_bytes = std::fs::read(&paths.repository_file).unwrap();
        ExtensionState::from_paths(paths.clone()).unwrap();
        assert_eq!(
            std::fs::read(&paths.repository_file).unwrap(),
            repository_bytes
        );
        assert_eq!(
            std::fs::read(integration.join("keep")).unwrap(),
            b"installed content"
        );
    }

    #[test]
    fn migration_and_loader_keep_repository_authoritative_beside_legacy() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let repository_lock = fixture_lock("example.repository");
        write_repository(&paths.repository_file, &repository_lock).unwrap();
        let legacy_lock = fixture_lock("example.legacy");
        legacy_lock.save_legacy(&paths.legacy_lock_file).unwrap();
        let legacy_bytes = std::fs::read(&paths.legacy_lock_file).unwrap();
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
        let error = ExtensionsLock::load(&paths.legacy_lock_file).unwrap_err();
        assert!(
            error.contains(&paths.legacy_lock_file.display().to_string()),
            "{error}"
        );
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert!(loaded.extensions.contains_key("example.repository"));
        assert!(!loaded.extensions.contains_key("example.legacy"));

        ExtensionsLock::default()
            .save(&paths.repository_file)
            .unwrap();
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
        assert!(ExtensionsLock::load(&paths.repository_file)
            .unwrap()
            .extensions
            .is_empty());
        assert_eq!(
            std::fs::read(&paths.legacy_lock_file).unwrap(),
            legacy_bytes
        );
    }

    #[test]
    fn legacy_read_and_write_targets_are_errors_without_changing_state() {
        for migrated in [false, true] {
            let directory = tempfile::tempdir().unwrap();
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            let original = fixture_lock("example.legacy");
            original.save_legacy(&paths.legacy_lock_file).unwrap();
            let legacy_bytes = std::fs::read(&paths.legacy_lock_file).unwrap();
            if migrated {
                migrate_to_repository(&paths).unwrap();
            }
            let repository_bytes = std::fs::read(&paths.repository_file).ok();
            let changed = fixture_lock("example.updated");
            for target in [
                paths.legacy_lock_file.clone(),
                migrated_lock_path(&paths.legacy_lock_file),
                paths.root.join("other.json"),
            ] {
                for error in [
                    changed.save(&target).unwrap_err(),
                    ExtensionsLock::load(&target).unwrap_err(),
                ] {
                    assert!(error.contains("extension-repository.json"), "{error}");
                    assert!(error.contains(&target.display().to_string()), "{error}");
                }
            }
            assert_eq!(std::fs::read(&paths.repository_file).ok(), repository_bytes);
            assert_eq!(paths.repository_file.exists(), migrated);
            assert!(!paths.root.join("other.json").exists());
            if migrated {
                assert!(!paths.legacy_lock_file.exists());
                assert_eq!(
                    std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
                    legacy_bytes
                );
            } else {
                assert_eq!(
                    std::fs::read(&paths.legacy_lock_file).unwrap(),
                    legacy_bytes
                );
                assert!(!migrated_lock_path(&paths.legacy_lock_file).exists());
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
        assert!(!paths.legacy_lock_file.exists());
        assert!(!migrated_lock_path(&paths.legacy_lock_file).exists());
        assert_eq!(std::fs::read_dir(&paths.root).unwrap().count(), 1);
    }

    #[test]
    fn startup_uses_committed_repository_when_migration_archive_fails() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.legacy");
        lock.save_legacy(&paths.legacy_lock_file).unwrap();
        let legacy_bytes = std::fs::read(&paths.legacy_lock_file).unwrap();
        // An archive directory makes the rename fail after the repository commit.
        std::fs::create_dir(migrated_lock_path(&paths.legacy_lock_file)).unwrap();
        assert!(ExtensionsLock::load(&paths.repository_file).is_err());
        ExtensionState::from_paths(paths.clone()).unwrap();
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert!(loaded.extensions.contains_key("example.legacy"));
        assert!(paths.repository_file.exists());
        ExtensionsLock::default()
            .save(&paths.repository_file)
            .unwrap();
        ExtensionState::from_paths(paths.clone()).unwrap();
        assert!(ExtensionsLock::load(&paths.repository_file)
            .unwrap()
            .extensions
            .is_empty());
        assert_eq!(
            std::fs::read(&paths.legacy_lock_file).unwrap(),
            legacy_bytes
        );
    }

    #[test]
    fn invalid_repository_and_legacy_lock_abort_loading() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        std::fs::write(&paths.repository_file, b"not json").unwrap();
        std::fs::write(&paths.legacy_lock_file, b"not json").unwrap();
        assert!(ExtensionsLock::load(&paths.repository_file).is_err());
        assert!(ExtensionsLock::load(&paths.repository_file).is_err());
        assert!(corrupt_repository_path(&paths.repository_file).exists());
    }

    #[test]
    fn loader_requires_explicit_recovery_of_corrupt_repository_from_archive() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        let lock = fixture_lock("example.repository");
        write_repository(&paths.repository_file, &lock).unwrap();
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );

        std::fs::remove_file(&paths.repository_file).unwrap();
        lock.save_legacy(&migrated_lock_path(&paths.legacy_lock_file))
            .unwrap();
        std::fs::write(&paths.repository_file, b"not json").unwrap();
        let archive_bytes = std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap();
        for _ in 0..2 {
            assert!(ExtensionsLock::load(&paths.repository_file).is_err());
            assert!(!paths.repository_file.exists());
            assert_eq!(
                std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
                archive_bytes
            );
        }
        ExtensionState::from_paths(paths.clone()).unwrap();
        let loaded = ExtensionsLock::load(&paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(&loaded.extensions).unwrap(),
            serde_json::to_value(&lock.extensions).unwrap()
        );
        assert!(corrupt_repository_path(&paths.repository_file).exists());
        let repository_bytes = std::fs::read(&paths.repository_file).unwrap();
        ExtensionState::from_paths(paths.clone()).unwrap();
        assert_eq!(
            std::fs::read(&paths.repository_file).unwrap(),
            repository_bytes
        );
        assert!(ExtensionsLock::load(&paths.repository_file)
            .unwrap()
            .extensions
            .contains_key("example.repository"));
        assert!(!paths.legacy_lock_file.exists());
        assert_eq!(
            std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
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
        lock.save_legacy(&state.paths.legacy_lock_file).unwrap();
        let legacy_bytes = std::fs::read(&state.paths.legacy_lock_file).unwrap();

        assert_eq!(
            migrate_to_repository(&state.paths).unwrap(),
            MigrationOutcome::Migrated
        );
        assert_eq!(
            std::fs::read(migrated_lock_path(&state.paths.legacy_lock_file)).unwrap(),
            legacy_bytes
        );
        assert!(!state.paths.legacy_lock_file.exists());
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
        lock.save_legacy(&paths.legacy_lock_file).unwrap();
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
        assert!(paths.legacy_lock_file.exists());
        assert_eq!(
            migrate_to_repository(&paths).unwrap(),
            MigrationOutcome::Noop
        );
    }

    #[test]
    fn startup_ignores_stale_and_invalid_legacy_files_beside_repository() {
        for empty in [false, true] {
            for invalid in [false, true] {
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let lock = if empty {
                    ExtensionsLock::default()
                } else {
                    fixture_lock("example.current")
                };
                lock.save(&paths.repository_file).unwrap();
                let repository_bytes = std::fs::read(&paths.repository_file).unwrap();
                if invalid {
                    std::fs::write(&paths.legacy_lock_file, b"stale invalid lock").unwrap();
                } else {
                    fixture_lock("example.stale")
                        .save_legacy(&paths.legacy_lock_file)
                        .unwrap();
                }
                let legacy_bytes = std::fs::read(&paths.legacy_lock_file).unwrap();
                let archive = migrated_lock_path(&paths.legacy_lock_file);
                std::fs::write(&archive, b"stale invalid archive").unwrap();
                for _ in 0..2 {
                    ExtensionState::from_paths(paths.clone()).unwrap();
                    assert_eq!(
                        std::fs::read(&paths.repository_file).unwrap(),
                        repository_bytes
                    );
                    assert_eq!(
                        std::fs::read(&paths.legacy_lock_file).unwrap(),
                        legacy_bytes
                    );
                    assert_eq!(std::fs::read(&archive).unwrap(), b"stale invalid archive");
                    assert_eq!(
                        serde_json::to_value(ExtensionsLock::load(&paths.repository_file).unwrap())
                            .unwrap(),
                        serde_json::to_value(&lock).unwrap()
                    );
                }
            }
        }
    }

    #[test]
    fn startup_errors_name_invalid_migration_inputs_and_preserve_installed_files() {
        let mut invalid_entry = serde_json::to_value(fixture_lock("example.invalid")).unwrap();
        invalid_entry["extensions"]["example.invalid"]["id"] = "example.mismatch".into();
        let invalid_inputs = [
            b"not json".to_vec(),
            br#"{"schemaVersion":999,"extensions":{}}"#.to_vec(),
            serde_json::to_vec(&invalid_entry).unwrap(),
        ];
        for migrated in [false, true] {
            for bytes in &invalid_inputs {
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let input = if migrated {
                    migrated_lock_path(&paths.legacy_lock_file)
                } else {
                    paths.legacy_lock_file.clone()
                };
                std::fs::write(&input, bytes).unwrap();
                let integration = paths.data.join("example.invalid/integration");
                let staging = paths.extensions.join(".staging/unfinished");
                for tree in [&integration, &staging] {
                    std::fs::create_dir_all(tree).unwrap();
                    std::fs::write(tree.join("keep"), b"installed content").unwrap();
                }
                for _ in 0..2 {
                    let error = ExtensionState::from_paths(paths.clone())
                        .err()
                        .expect("invalid migration must abort startup");
                    assert!(error.contains(&input.display().to_string()), "{error}");
                    assert!(error.contains("refusing to use an empty state"), "{error}");
                    assert!(!paths.repository_file.exists());
                    assert_eq!(std::fs::read(&input).unwrap(), *bytes);
                    for tree in [&integration, &staging] {
                        assert_eq!(
                            std::fs::read(tree.join("keep")).unwrap(),
                            b"installed content"
                        );
                    }
                }
            }
        }
    }

    #[test]
    fn startup_commits_archive_only_state_left_by_pre_slice7_recovery() {
        for corrupt_marker in [false, true] {
            let directory = tempfile::tempdir().unwrap();
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            let lock = fixture_lock("example.archive-only");
            let archive = migrated_lock_path(&paths.legacy_lock_file);
            lock.save_legacy(&archive).unwrap();
            let archive_bytes = std::fs::read(&archive).unwrap();
            let corrupt = corrupt_repository_path(&paths.repository_file);
            if corrupt_marker {
                std::fs::write(&corrupt, b"previously quarantined repository").unwrap();
            }
            let integration = paths.data.join("example.archive-only/integration");
            std::fs::create_dir_all(&integration).unwrap();
            std::fs::write(integration.join("keep"), b"installed content").unwrap();
            assert!(ExtensionsLock::load(&paths.repository_file).is_err());
            ExtensionState::from_paths(paths.clone()).unwrap();
            let repository_bytes = std::fs::read(&paths.repository_file).unwrap();
            for _ in 0..2 {
                ExtensionState::from_paths(paths.clone()).unwrap();
                assert_eq!(
                    std::fs::read(&paths.repository_file).unwrap(),
                    repository_bytes
                );
                assert_eq!(
                    serde_json::to_value(read_repository(&paths.repository_file).unwrap()).unwrap(),
                    serde_json::to_value(&lock).unwrap()
                );
                assert_eq!(std::fs::read(&archive).unwrap(), archive_bytes);
                assert!(!paths.legacy_lock_file.exists());
                assert!(!paths.extensions.join(".transactions").exists());
                assert_eq!(
                    std::fs::read(integration.join("keep")).unwrap(),
                    b"installed content"
                );
                if corrupt_marker {
                    assert_eq!(
                        std::fs::read(&corrupt).unwrap(),
                        b"previously quarantined repository"
                    );
                } else {
                    assert!(!corrupt.exists());
                }
            }
        }
    }

    #[test]
    fn startup_migrates_valid_empty_pre_repository_state() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        ExtensionsLock::default()
            .save_legacy(&paths.legacy_lock_file)
            .unwrap();
        let legacy_bytes = std::fs::read(&paths.legacy_lock_file).unwrap();
        for _ in 0..2 {
            ExtensionState::from_paths(paths.clone()).unwrap();
            assert!(read_repository(&paths.repository_file)
                .unwrap()
                .extensions
                .is_empty());
            assert_eq!(
                std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
                legacy_bytes
            );
            assert!(!paths.legacy_lock_file.exists());
        }
    }

    #[test]
    fn migration_commit_faults_preserve_inputs_and_recover_on_restart() {
        for migrated in [false, true] {
            for label in [
                "repository-persist",
                "repository-directory-sync",
                "repository-archive-rename",
                "repository-archive-directory-sync",
            ] {
                if migrated && label.starts_with("repository-archive-") {
                    continue;
                }
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let lock = fixture_lock("example.migration-crash");
                let archive = migrated_lock_path(&paths.legacy_lock_file);
                let input = if migrated {
                    &archive
                } else {
                    &paths.legacy_lock_file
                };
                lock.save_legacy(input).unwrap();
                let legacy_bytes = std::fs::read(input).unwrap();
                let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                    crate::extensions::with_commit_point(label, || {
                        ExtensionState::from_paths(paths.clone()).unwrap();
                    });
                }));
                let panic = result.expect_err("migration must hit the injected commit point");
                assert_eq!(
                    panic.downcast_ref::<String>(),
                    Some(&format!("injected crash at commit point {label}"))
                );
                let surviving_input = if paths.legacy_lock_file.exists() {
                    &paths.legacy_lock_file
                } else {
                    &archive
                };
                assert_eq!(std::fs::read(surviving_input).unwrap(), legacy_bytes);
                for _ in 0..2 {
                    ExtensionState::from_paths(paths.clone()).unwrap();
                    assert_eq!(
                        serde_json::to_value(read_repository(&paths.repository_file).unwrap())
                            .unwrap(),
                        serde_json::to_value(&lock).unwrap()
                    );
                    let surviving_input = if paths.legacy_lock_file.exists() {
                        &paths.legacy_lock_file
                    } else {
                        &archive
                    };
                    assert_eq!(std::fs::read(surviving_input).unwrap(), legacy_bytes);
                }
            }
        }
    }

    #[cfg(unix)]
    mod unix_faults {
        use super::*;
        use crate::extensions::fault_test_support::{
            skip_readonly_as_root, Corruption, ReadonlyDirectory,
        };
        use crate::extensions::{transaction, with_commit_point_action, ExtensionState};

        #[test]
        fn migration_write_failure_aborts_startup_before_cleanup_and_retries() {
            if skip_readonly_as_root() {
                return;
            }
            for migrated in [false, true] {
                let directory = tempfile::tempdir().unwrap();
                let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                let lock = fixture_lock("example.migration-write-failure");
                let input = if migrated {
                    migrated_lock_path(&paths.legacy_lock_file)
                } else {
                    paths.legacy_lock_file.clone()
                };
                lock.save_legacy(&input).unwrap();
                let legacy_bytes = std::fs::read(&input).unwrap();
                if migrated {
                    std::fs::write(
                        corrupt_repository_path(&paths.repository_file),
                        b"damaged repository",
                    )
                    .unwrap();
                }
                let integration = paths
                    .data
                    .join("example.migration-write-failure/integration");
                let staging = paths.extensions.join(".staging/unfinished");
                for tree in [&integration, &staging] {
                    std::fs::create_dir_all(tree).unwrap();
                    std::fs::write(tree.join("keep"), b"installed content").unwrap();
                }
                let readonly = ReadonlyDirectory::new(&paths.root);
                let error = with_commit_point_action("repository-persist", readonly.arm(), || {
                    ExtensionState::from_paths(paths.clone())
                })
                .err()
                .expect("migration write failure must abort startup");
                assert!(error.contains("Cannot migrate"), "{error}");
                assert!(error.contains(&input.display().to_string()), "{error}");
                assert!(
                    error.contains(&paths.repository_file.display().to_string()),
                    "{error}"
                );
                assert!(!paths.repository_file.exists());
                assert_eq!(std::fs::read(&input).unwrap(), legacy_bytes);
                for tree in [&integration, &staging] {
                    assert_eq!(
                        std::fs::read(tree.join("keep")).unwrap(),
                        b"installed content"
                    );
                }
                drop(readonly);
                for _ in 0..2 {
                    ExtensionState::from_paths(paths.clone()).unwrap();
                    assert_eq!(
                        serde_json::to_value(read_repository(&paths.repository_file).unwrap())
                            .unwrap(),
                        serde_json::to_value(&lock).unwrap()
                    );
                    assert_eq!(
                        std::fs::read(integration.join("keep")).unwrap(),
                        b"installed content"
                    );
                    assert_eq!(
                        std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
                        legacy_bytes
                    );
                    if migrated {
                        assert_eq!(
                            std::fs::read(corrupt_repository_path(&paths.repository_file)).unwrap(),
                            b"damaged repository"
                        );
                    }
                }
            }
        }

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
        fn corrupt_repository_recovery_commits_each_valid_migration_input() {
            for corruption in Corruption::ALL {
                for migrated in [false, true] {
                    let directory = tempfile::tempdir().unwrap();
                    let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
                    let state = ExtensionState::from_paths(paths.clone()).unwrap();
                    let lock = fixture_lock("example.corrupt-fallback");
                    let fallback = if migrated {
                        migrated_lock_path(&paths.legacy_lock_file)
                    } else {
                        paths.legacy_lock_file.clone()
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
                            std::fs::read(migrated_lock_path(&paths.legacy_lock_file)).unwrap(),
                            legacy_bytes
                        );
                        assert!(!paths.legacy_lock_file.exists());
                        assert!(paths.repository_file.exists());
                        assert_eq!(
                            serde_json::to_value(read_repository(&paths.repository_file).unwrap())
                                .unwrap(),
                            serde_json::to_value(&lock).unwrap()
                        );
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
                    migrated_lock_path(&paths.legacy_lock_file)
                } else {
                    paths.legacy_lock_file.clone()
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
