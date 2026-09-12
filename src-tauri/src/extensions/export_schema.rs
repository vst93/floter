//! Export schema for extension configuration bundles.
//!
//! Phase 5 validation: export distinguishes secret fields, device-specific
//! paths, and version constraints. Secrets are never exported in plaintext.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Portable export bundle for an extension's configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExtensionExportBundle {
    /// Schema version for forward compatibility.
    pub schema_version: u32,
    /// Extension identifier.
    pub extension_id: String,
    /// Extension manifest version this config was created for.
    pub manifest_version: String,
    /// Configuration fields, excluding secrets and device-specific paths.
    pub config: HashMap<String, serde_json::Value>,
    /// Field metadata for import validation.
    pub field_metadata: Vec<FieldMetadata>,
    /// Timestamp when exported (unix seconds).
    pub exported_at: u64,
}

/// Metadata for a configuration field in the export.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FieldMetadata {
    /// Field key in the config map.
    pub key: String,
    /// Field classification.
    pub category: FieldCategory,
    /// True if this field was excluded from export due to its category.
    pub excluded: bool,
}

/// Classification of configuration fields for export handling.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FieldCategory {
    /// Regular configuration value, safe to export.
    Normal,
    /// Secret credential (API key, password, token). Never exported in plaintext.
    Secret,
    /// Device-specific path. Excluded from export by default.
    DevicePath,
    /// Version constraint that may not apply on the target system.
    VersionConstraint,
}

pub const EXPORT_SCHEMA_VERSION: u32 = 1;

/// Classifies a config field based on its key and value patterns.
pub fn classify_field(key: &str, value: &serde_json::Value) -> FieldCategory {
    let key_lower = key.to_ascii_lowercase();

    // Secret detection: key contains common secret terms
    if key_lower.contains("secret")
        || key_lower.contains("password")
        || key_lower.contains("token")
        || key_lower.contains("api_key")
        || key_lower.contains("apikey")
        || key_lower.contains("private_key")
        || key_lower.contains("privatekey")
        || key_lower.contains("credentials")
    {
        return FieldCategory::Secret;
    }

    // Device path detection: absolute paths
    if let Some(s) = value.as_str() {
        if s.starts_with('/') || s.starts_with('\\') || s.contains(":\\") {
            return FieldCategory::DevicePath;
        }
    }

    // Version constraint detection
    if key_lower.contains("version") || key_lower == "v" {
        return FieldCategory::VersionConstraint;
    }

    FieldCategory::Normal
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_secrets_from_key_patterns() {
        assert_eq!(
            classify_field("api_key", &serde_json::json!("xyz")),
            FieldCategory::Secret
        );
        assert_eq!(
            classify_field("database_password", &serde_json::json!("pass123")),
            FieldCategory::Secret
        );
        assert_eq!(
            classify_field("github_token", &serde_json::json!("ghp_abc")),
            FieldCategory::Secret
        );
    }

    #[test]
    fn detects_device_paths() {
        assert_eq!(
            classify_field("config_path", &serde_json::json!("/home/user/.config")),
            FieldCategory::DevicePath
        );
        assert_eq!(
            classify_field("data_dir", &serde_json::json!("C:\\Users\\Alice")),
            FieldCategory::DevicePath
        );
    }

    #[test]
    fn normal_fields_pass_through() {
        assert_eq!(
            classify_field("enabled", &serde_json::json!(true)),
            FieldCategory::Normal
        );
        assert_eq!(
            classify_field("timeout_ms", &serde_json::json!(5000)),
            FieldCategory::Normal
        );
    }
}
