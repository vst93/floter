//! Phase 5 validation tests for export/import secret filtering.

#[cfg(test)]
mod tests {
    use crate::extensions::export_schema::{classify_field, FieldCategory};
    use crate::extensions::lock::{
        ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
        ExtensionRuntimeOwnership, ExtensionStateKind, ExtensionsLock,
    };
    use crate::extensions::sync::{build_export, write_export};
    use crate::extensions::{ExtensionPaths, ExtensionState};
    use chrono::Utc;
    use serde_json::json;
    use tempfile::TempDir;

    fn test_state(temp: &TempDir) -> ExtensionState {
        let paths = ExtensionPaths::from_root(temp.path().to_path_buf());
        ExtensionState::from_paths(paths).unwrap()
    }

    fn mock_extension_entry(id: &str, version: &str) -> ExtensionLockEntry {
        ExtensionLockEntry {
            id: id.into(),
            name: id.into(),
            publisher_id: "test".into(),
            publisher_name: "Test".into(),
            distribution_source: ExtensionDistributionSource::Npm,
            runtime_ownership: ExtensionRuntimeOwnership::Bundled,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: Some(format!("@test/{id}")),
            package_version: version.into(),
            tool_version: None,
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

    #[test]
    fn export_filters_secrets_from_config() {
        let temp = TempDir::new().unwrap();
        let state = test_state(&temp);

        // Create mock extension with config containing secrets
        let entry = mock_extension_entry("example.api", "1.0.0");
        let mut lock = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap_or_else(|_| ExtensionsLock {
                schema_version: 1,
                extensions: Default::default(),
            });
        lock.extensions.insert(entry.id.clone(), entry);
        lock.save(&state.paths.repository_file).unwrap();

        // Write config with secrets
        let config_dir = state.paths.data.join("example.api");
        std::fs::create_dir_all(&config_dir).unwrap();
        let config_file = config_dir.join("config.json");
        let config = json!({
            "api_key": "secret_value_123",
            "database_password": "admin123",
            "endpoint": "https://api.example.com",
            "timeout_ms": 5000
        });
        std::fs::write(config_file, config.to_string()).unwrap();

        let export = build_export(&state, Utc::now()).unwrap();
        let exported_entry = export
            .extensions
            .iter()
            .find(|e| e.id == "example.api")
            .unwrap();

        // Secrets should be excluded
        assert!(!exported_entry.config.contains_key("api_key"));
        assert!(!exported_entry.config.contains_key("database_password"));

        // Normal fields should be present
        assert_eq!(
            exported_entry.config.get("endpoint").unwrap(),
            &json!("https://api.example.com")
        );
        assert_eq!(exported_entry.config.get("timeout_ms").unwrap(), &json!(5000));

        // Field metadata should document what was excluded
        let metadata = exported_entry.field_metadata.as_ref().unwrap();
        assert_eq!(metadata.len(), 4);

        let api_key_meta = metadata.iter().find(|m| m.key == "api_key").unwrap();
        assert_eq!(api_key_meta.category, FieldCategory::Secret);
        assert!(api_key_meta.excluded);

        let endpoint_meta = metadata.iter().find(|m| m.key == "endpoint").unwrap();
        assert_eq!(endpoint_meta.category, FieldCategory::Normal);
        assert!(!endpoint_meta.excluded);
    }

    #[test]
    fn export_filters_device_paths_from_config() {
        let temp = TempDir::new().unwrap();
        let state = test_state(&temp);

        let entry = mock_extension_entry("example.paths", "1.0.0");
        let mut lock = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap_or_else(|_| ExtensionsLock {
                schema_version: 1,
                extensions: Default::default(),
            });
        lock.extensions.insert(entry.id.clone(), entry);
        lock.save(&state.paths.repository_file).unwrap();

        let config_dir = state.paths.data.join("example.paths");
        std::fs::create_dir_all(&config_dir).unwrap();
        let config = json!({
            "config_path": "/home/user/.config/app",
            "data_dir": "C:\\Users\\Alice\\AppData",
            "relative_path": "./data",
            "enabled": true
        });
        std::fs::write(
            config_dir.join("config.json"),
            config.to_string(),
        )
        .unwrap();

        let export = build_export(&state, Utc::now()).unwrap();
        let exported_entry = export
            .extensions
            .iter()
            .find(|e| e.id == "example.paths")
            .unwrap();

        // Absolute paths should be excluded
        assert!(!exported_entry.config.contains_key("config_path"));
        assert!(!exported_entry.config.contains_key("data_dir"));

        // Relative paths and normal fields should remain
        assert_eq!(
            exported_entry.config.get("relative_path").unwrap(),
            &json!("./data")
        );
        assert_eq!(exported_entry.config.get("enabled").unwrap(), &json!(true));
    }

    #[test]
    fn field_classification_detects_all_secret_patterns() {
        let patterns = [
            ("api_key", json!("xyz")),
            ("apiKey", json!("xyz")),
            ("secret_token", json!("xyz")),
            ("database_password", json!("pass")),
            ("github_token", json!("ghp_abc")),
            ("private_key", json!("-----BEGIN")),
            ("privateKey", json!("-----BEGIN")),
            ("credentials", json!("user:pass")),
        ];

        for (key, value) in patterns {
            assert_eq!(
                classify_field(key, &value),
                FieldCategory::Secret,
                "Failed to detect secret: {key}"
            );
        }
    }

    #[test]
    fn exported_document_roundtrips_with_metadata() {
        let temp = TempDir::new().unwrap();
        let state = test_state(&temp);

        let entry = mock_extension_entry("example.test", "1.0.0");
        let mut lock = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap_or_else(|_| ExtensionsLock {
                schema_version: 1,
                extensions: Default::default(),
            });
        lock.extensions.insert(entry.id.clone(), entry);
        lock.save(&state.paths.repository_file).unwrap();

        let config_dir = state.paths.data.join("example.test");
        std::fs::create_dir_all(&config_dir).unwrap();
        let config = json!({
            "token": "secret",
            "url": "https://example.com"
        });
        std::fs::write(
            config_dir.join("config.json"),
            config.to_string(),
        )
        .unwrap();

        let export = build_export(&state, Utc::now()).unwrap();
        let export_path = temp.path().join("export.json");
        write_export(&export_path, &export).unwrap();

        // Read back and verify structure
        let bytes = std::fs::read(&export_path).unwrap();
        let parsed: serde_json::Value = serde_json::from_slice(&bytes).unwrap();

        let entry = &parsed["extensions"][0];
        assert!(entry["fieldMetadata"].is_array());
        assert!(!entry["config"].as_object().unwrap().contains_key("token"));
        assert_eq!(entry["config"]["url"], json!("https://example.com"));
    }
}
