//! Data ownership classification and path resolution.
//!
//! Defines four data categories per Phase 5 audit requirements:
//! - **Host settings**: Floter's own configuration (extensions.lock.json)
//! - **Tool settings**: Extension-specific configuration (per-extension config files)
//! - **Runtime data**: Transient state (process IDs, sockets, temp files)
//! - **Generated artifacts**: Persistent outputs (logs, caches, user data)

use std::path::{Path, PathBuf};

/// Data ownership categories for extension lifecycle operations.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DataCategory {
    /// Floter's own configuration (extensions.lock.json, global settings).
    /// Managed by floter core, never deleted during uninstall.
    HostSettings,

    /// Extension-specific configuration (manifest-derived settings, user prefs).
    /// Stored per-extension, deletable during uninstall with user consent.
    ToolSettings,

    /// Runtime state (PIDs, sockets, temp files created during execution).
    /// Short-lived, cleaned up on process exit or periodic sweeps.
    RuntimeData,

    /// Persistent outputs (logs, caches, generated files, user data).
    /// Long-lived, deletable during uninstall with user consent.
    GeneratedArtifacts,
}

/// Path resolution for each data category within an extension's directory tree.
pub struct DataPaths {
    /// Root directory for all extensions (e.g., `~/.config/floter/extensions`)
    pub extensions_root: PathBuf,
}

impl DataPaths {
    pub fn new(extensions_root: PathBuf) -> Self {
        Self { extensions_root }
    }

    /// Returns the extension's installation directory (program files).
    pub fn extension_program(&self, extension_id: &str) -> PathBuf {
        self.extensions_root.join(extension_id)
    }

    /// Returns the extension's tool settings directory.
    /// Currently stored alongside program files; future versions may use XDG_CONFIG_HOME.
    pub fn extension_tool_settings(&self, extension_id: &str) -> PathBuf {
        self.extensions_root.join(extension_id).join("config")
    }

    /// Returns the extension's runtime data directory.
    pub fn extension_runtime_data(&self, extension_id: &str) -> PathBuf {
        self.extensions_root.join(extension_id).join("runtime")
    }

    /// Returns the extension's generated artifacts directory.
    pub fn extension_generated_artifacts(&self, extension_id: &str) -> PathBuf {
        self.extensions_root.join(extension_id).join("artifacts")
    }

    /// Classifies a path into a data category relative to an extension root.
    pub fn classify_path(&self, extension_id: &str, path: &Path) -> Option<DataCategory> {
        let ext_root = self.extension_program(extension_id);
        let relative = path.strip_prefix(&ext_root).ok()?;

        let first_component = relative.components().next()?.as_os_str().to_str()?;
        match first_component {
            "config" => Some(DataCategory::ToolSettings),
            "runtime" => Some(DataCategory::RuntimeData),
            "artifacts" => Some(DataCategory::GeneratedArtifacts),
            _ => None, // Program files (uncategorized under current scheme)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_extension_paths_correctly() {
        let paths = DataPaths::new("/home/user/.config/floter/extensions".into());
        let ext_id = "example.journal";

        assert_eq!(
            paths.classify_path(ext_id, &paths.extension_tool_settings(ext_id)),
            Some(DataCategory::ToolSettings)
        );
        assert_eq!(
            paths.classify_path(ext_id, &paths.extension_runtime_data(ext_id)),
            Some(DataCategory::RuntimeData)
        );
        assert_eq!(
            paths.classify_path(ext_id, &paths.extension_generated_artifacts(ext_id)),
            Some(DataCategory::GeneratedArtifacts)
        );
    }
}
