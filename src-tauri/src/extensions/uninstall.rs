//! Componentized uninstall with data ownership categories.
//!
//! Phase 5 validation 4: users can selectively remove program files, host
//! config, tool data, and generated artifacts. Progress events track each
//! component phase.

use crate::extensions::data_ownership::{DataCategory, DataPaths};
use crate::extensions::lock::{validate_id, ExtensionsLock};
use crate::extensions::operation::OperationProgress;
use crate::extensions::transaction::{RemovalIntent, RemovalJournal, RemovalKind};
use crate::extensions::ExtensionState;
use serde::{Deserialize, Serialize};

/// Uninstall request with component-level control.
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UninstallRequest {
    /// Extension to uninstall.
    pub extension_id: String,
    /// Remove the program files (always required for uninstall).
    #[serde(default = "default_true")]
    pub remove_program: bool,
    /// Remove host-owned configuration.
    #[serde(default = "default_true")]
    pub remove_host_config: bool,
    /// Remove tool-owned data (config, sessions, state).
    #[serde(default = "default_true")]
    pub remove_tool_data: bool,
    /// Remove generated artifacts (logs, caches, user-generated files).
    #[serde(default = "default_true")]
    pub remove_artifacts: bool,
}

fn default_true() -> bool {
    true
}

/// Uninstall result with component-level status.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UninstallResult {
    pub removed_program: bool,
    pub removed_host_config: bool,
    pub removed_tool_data: bool,
    pub removed_artifacts: bool,
}

/// Uninstall an extension with component-level control.
pub async fn uninstall_componentized(
    state: &ExtensionState,
    request: UninstallRequest,
) -> Result<UninstallResult, String> {
    validate_id(&request.extension_id)?;
    state.check_cancelled()?;
    state.provider.cancel_completions();

    state.emit_progress(OperationProgress {
        extension_id: request.extension_id.clone(),
        kind: "uninstall".to_string(),
        phase: "Preparing".to_string(),
        percent: Some(10),
    });

    crate::extensions::transaction::recover_pending_removals(state)?;
    let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
    let entry = lock.get(&request.extension_id)?.clone();

    let data_paths = DataPaths::new(state.paths.data.clone());
    let extension_root = state.paths.extensions.join(&request.extension_id);
    let data_root = state.paths.data.join(&request.extension_id);

    // Build cleanup paths based on user selection
    let mut cleanup_paths = Vec::new();

    // Generated local integration always goes with program removal
    let generated_local_root = data_root.join("integration");
    let is_generated_local = crate::extensions::install::is_generated_custom_integration(&entry)
        && generated_local_root.exists();

    if request.remove_program && is_generated_local {
        cleanup_paths.push(generated_local_root.clone());
    }

    // Host config
    if request.remove_host_config {
        let config_file = data_root.join("config.json");
        let secrets_dir = data_root.join("config-secrets");
        if config_file.exists() {
            cleanup_paths.push(config_file);
        }
        if secrets_dir.exists() {
            cleanup_paths.push(secrets_dir);
        }
    }

    // Tool data (config, sessions, health)
    if request.remove_tool_data {
        for subdir in ["sessions", "completions"] {
            let path = data_root.join(subdir);
            if path.exists() {
                cleanup_paths.push(path);
            }
        }
        let health = data_root.join("health.json");
        if health.exists() {
            cleanup_paths.push(health);
        }
    }

    // Generated artifacts
    if request.remove_artifacts {
        let artifacts_dir = data_paths.extension_generated_artifacts(&request.extension_id);
        if artifacts_dir.exists() {
            cleanup_paths.push(artifacts_dir);
        }
    }

    // If all data categories are selected, just remove the entire data root
    let remove_entire_data = request.remove_host_config
        && request.remove_tool_data
        && request.remove_artifacts
        && data_root.exists();

    if remove_entire_data {
        cleanup_paths.clear();
        cleanup_paths.push(data_root.clone());
    }

    state.check_cancelled()?;
    state.emit_progress(OperationProgress {
        extension_id: request.extension_id.clone(),
        kind: "uninstall".to_string(),
        phase: "Creating backup".to_string(),
        percent: Some(30),
    });

    let transaction_id = format!("uninstall-{}-{}", request.extension_id, entry.updated_at);
    let mut staged_path = None;

    // Reserve a backup path for the extension directory
    if extension_root.exists() && request.remove_program {
        let placeholder = tempfile::Builder::new()
            .prefix(&format!(".removing-{}-", request.extension_id))
            .tempdir_in(&state.paths.extensions)
            .map_err(|error| format!("Cannot create removal transaction: {error}"))?;
        let target = placeholder.path().to_path_buf();
        placeholder
            .close()
            .map_err(|error| format!("Cannot prepare removal transaction: {error}"))?;
        staged_path = Some(target);
    }

    state.check_cancelled()?;

    // Journal the planned removal durably before staging or committing
    let journal = RemovalJournal {
        schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
        transaction_id: transaction_id.clone(),
        extension_id: request.extension_id.clone(),
        removed_entry: entry.clone(),
        staged_path: staged_path.clone(),
        cleanup_paths: cleanup_paths.clone(),
        removal_kind: Some(RemovalKind::Staged),
        remove_data: remove_entire_data,
        intent: RemovalIntent::Remove,
    };
    let journal_path = crate::extensions::transaction::write_removal_journal(state, &journal)?;

    state.emit_progress(OperationProgress {
        extension_id: request.extension_id.clone(),
        kind: "uninstall".to_string(),
        phase: "Staging removal".to_string(),
        percent: Some(50),
    });

    if let Some(target) = &staged_path {
        crate::extensions::commit_point("uninstall-stage-rename");
        std::fs::rename(&extension_root, target).map_err(|error| {
            format!(
                "Cannot stage extension {} for removal: {error}",
                extension_root.display()
            )
        })?;
    }

    state.check_cancelled()?;
    state.emit_progress(OperationProgress {
        extension_id: request.extension_id.clone(),
        kind: "uninstall".to_string(),
        phase: "Updating registry".to_string(),
        percent: Some(70),
    });

    // Remove from repository
    lock.extensions.remove(&request.extension_id);
    crate::extensions::commit_point("uninstall-repository-remove");
    if let Err(error) = lock.save(&state.paths.repository_file) {
        crate::extensions::transaction::recover_pending_removals(state)
            .map_err(|recovery| format!("{error}; {recovery}"))?;
        return Err(error);
    }

    state.check_cancelled()?;

    // Update journal to mark repository state as committed
    let journal = RemovalJournal {
        removal_kind: Some(RemovalKind::Committed),
        ..journal
    };
    let _ = crate::extensions::transaction::write_removal_journal(state, &journal);

    // Physical cleanup with per-component progress
    let mut result = UninstallResult {
        removed_program: false,
        removed_host_config: false,
        removed_tool_data: false,
        removed_artifacts: false,
    };

    if request.remove_program {
        state.emit_progress(OperationProgress {
            extension_id: request.extension_id.clone(),
            kind: "uninstall".to_string(),
            phase: "Removing program files".to_string(),
            percent: Some(75),
        });

        if let Some(target) = &staged_path {
            if target.exists() {
                std::fs::remove_dir_all(target)
                    .map_err(|error| format!("Cannot remove {}: {error}", target.display()))?;
                result.removed_program = true;
            }
        }
    }

    state.check_cancelled()?;

    // Clean up data components
    if remove_entire_data {
        state.emit_progress(OperationProgress {
            extension_id: request.extension_id.clone(),
            kind: "uninstall".to_string(),
            phase: "Removing all data".to_string(),
            percent: Some(85),
        });

        if data_root.exists() {
            std::fs::remove_dir_all(&data_root)
                .map_err(|error| format!("Cannot remove {}: {error}", data_root.display()))?;
            result.removed_host_config = request.remove_host_config;
            result.removed_tool_data = request.remove_tool_data;
            result.removed_artifacts = request.remove_artifacts;
        }
    } else {
        // Selective component removal
        for cleanup_path in &cleanup_paths {
            if cleanup_path.exists() {
                let category = if cleanup_path.ends_with("config.json")
                    || cleanup_path.ends_with("config-secrets")
                {
                    "host config"
                } else if cleanup_path.ends_with("sessions")
                    || cleanup_path.ends_with("completions")
                    || cleanup_path.ends_with("health.json")
                {
                    "tool data"
                } else if cleanup_path.ends_with("artifacts") {
                    "artifacts"
                } else {
                    "data"
                };

                state.emit_progress(OperationProgress {
                    extension_id: request.extension_id.clone(),
                    kind: "uninstall".to_string(),
                    phase: format!("Removing {}", category),
                    percent: Some(85),
                });

                if cleanup_path.is_dir() {
                    std::fs::remove_dir_all(cleanup_path).map_err(|error| {
                        format!("Cannot remove {}: {error}", cleanup_path.display())
                    })?;
                } else {
                    std::fs::remove_file(cleanup_path).map_err(|error| {
                        format!("Cannot remove {}: {error}", cleanup_path.display())
                    })?;
                }

                // Update result flags based on what was removed
                if category == "host config" {
                    result.removed_host_config = true;
                } else if category == "tool data" {
                    result.removed_tool_data = true;
                } else if category == "artifacts" {
                    result.removed_artifacts = true;
                }
            }
        }
    }

    state.emit_progress(OperationProgress {
        extension_id: request.extension_id.clone(),
        kind: "uninstall".to_string(),
        phase: "Complete".to_string(),
        percent: Some(100),
    });

    // Remove journal on success
    let _ = std::fs::remove_file(&journal_path);
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::lock::{
        ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
        ExtensionRuntimeOwnership, ExtensionStateKind,
    };
    use crate::extensions::ExtensionPaths;

    fn test_state(root: &std::path::Path) -> ExtensionState {
        ExtensionState::from_paths(ExtensionPaths::from_root(root.to_path_buf())).unwrap()
    }

    fn test_entry(id: &str, version: &str) -> ExtensionLockEntry {
        ExtensionLockEntry {
            id: id.into(),
            name: "Test Extension".into(),
            publisher_id: "test".into(),
            publisher_name: "Test".into(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::StaticDescriptor,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: version.into(),
            tool_version: Some(version.into()),
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
            manifest_path: String::new(),
            executable_path: String::new(),
            runtime_root: None,
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
            probe_report: None,
            config_generation: 0,
        }
    }

    #[tokio::test]
    async fn selective_uninstall_preserves_unchecked_components() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "example.selective";
        let extension_root = state.paths.extensions.join(id);
        let data_root = state.paths.data.join(id);

        // Set up extension program files
        std::fs::create_dir_all(&extension_root).unwrap();
        std::fs::write(extension_root.join("manifest.json"), b"{}").unwrap();

        // Set up extension with all data categories
        std::fs::create_dir_all(&data_root).unwrap();
        std::fs::write(data_root.join("config.json"), b"{}").unwrap();
        std::fs::create_dir_all(data_root.join("sessions")).unwrap();
        std::fs::write(data_root.join("sessions/s1.json"), b"{}").unwrap();
        std::fs::create_dir_all(data_root.join("artifacts")).unwrap();
        std::fs::write(data_root.join("artifacts/log.txt"), b"log").unwrap();

        let entry = test_entry(id, "1.0.0");
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(id.into(), entry);
        lock.save(&state.paths.repository_file).unwrap();

        // Uninstall program and host config only, preserve tool data and artifacts
        let request = UninstallRequest {
            extension_id: id.into(),
            remove_program: true,
            remove_host_config: true,
            remove_tool_data: false,
            remove_artifacts: false,
        };

        let result = uninstall_componentized(&state, request).await.unwrap();

        assert!(result.removed_program);
        assert!(result.removed_host_config);
        assert!(!result.removed_tool_data);
        assert!(!result.removed_artifacts);

        // Config should be removed
        assert!(!data_root.join("config.json").exists());
        // Tool data and artifacts should remain
        assert!(data_root.join("sessions/s1.json").exists());
        assert!(data_root.join("artifacts/log.txt").exists());
    }
}
