use semver::Version;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::io::Write;
use std::path::{Path, PathBuf};

const LOCK_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionsLock {
    pub schema_version: u32,
    #[serde(default)]
    pub extensions: BTreeMap<String, ExtensionLockEntry>,
}

impl Default for ExtensionsLock {
    fn default() -> Self {
        Self {
            schema_version: LOCK_SCHEMA_VERSION,
            extensions: BTreeMap::new(),
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExtensionInstallType {
    Managed,
    Linked,
}

#[derive(Debug, Clone, Copy, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExtensionProviderKind {
    #[default]
    Executable,
    BundledStatic,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExtensionStateKind {
    Enabled,
    Disabled,
    Broken,
}

impl ExtensionStateKind {
    pub fn may_transition_to(self, next: Self) -> bool {
        use ExtensionStateKind::*;
        matches!(
            (self, next),
            (Enabled, Disabled | Broken)
                | (Disabled, Enabled | Broken)
                | (Broken, Enabled | Disabled)
        )
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionLockEntry {
    pub id: String,
    pub name: String,
    pub publisher_id: String,
    pub publisher_name: String,
    pub install_type: ExtensionInstallType,
    #[serde(default)]
    pub provider_kind: ExtensionProviderKind,
    pub state: ExtensionStateKind,
    pub enabled: bool,
    pub package_name: Option<String>,
    pub package_version: String,
    pub tool_version: Option<String>,
    pub integrity: Option<String>,
    #[serde(default)]
    pub signature_verified: bool,
    #[serde(default)]
    pub previous_signature_verified: Option<bool>,
    pub current_version: String,
    pub previous_version: Option<String>,
    pub manifest_path: String,
    pub executable_path: String,
    pub runtime_root: Option<String>,
    pub installed_at: u64,
    pub updated_at: u64,
    #[serde(default)]
    pub pinned: bool,
    #[serde(default = "default_channel")]
    pub channel: String,
}

fn default_channel() -> String {
    "latest".to_string()
}

impl ExtensionsLock {
    pub fn load(path: &Path) -> Result<Self, String> {
        if !path.exists() {
            return Ok(Self::default());
        }
        let bytes = std::fs::read(path)
            .map_err(|error| format!("Cannot read extension lock {}: {error}", path.display()))?;
        let lock: Self = serde_json::from_slice(&bytes)
            .map_err(|error| format!("Invalid extension lock {}: {error}", path.display()))?;
        if lock.schema_version != LOCK_SCHEMA_VERSION {
            return Err(format!(
                "Unsupported extensions.lock.json schema version {}",
                lock.schema_version
            ));
        }
        for (id, entry) in &lock.extensions {
            validate_id(id)?;
            if entry.id != *id {
                return Err(format!(
                    "Lock key {id} does not match entry id {}",
                    entry.id
                ));
            }
            if entry.install_type == ExtensionInstallType::Managed {
                validate_managed_version(&entry.package_version)?;
                validate_managed_version(&entry.current_version)?;
                if let Some(previous) = &entry.previous_version {
                    validate_managed_version(previous)?;
                }
            }
        }
        Ok(lock)
    }

    pub fn save(&self, path: &Path) -> Result<(), String> {
        let parent = path.parent().ok_or("Invalid extension lock path")?;
        std::fs::create_dir_all(parent)
            .map_err(|error| format!("Cannot create lock directory: {error}"))?;
        let bytes = serde_json::to_vec_pretty(self)
            .map_err(|error| format!("Cannot serialize extension lock: {error}"))?;
        let mut temporary = tempfile::NamedTempFile::new_in(parent)
            .map_err(|error| format!("Cannot create extension lock temporary file: {error}"))?;
        temporary
            .write_all(&bytes)
            .and_then(|_| temporary.flush())
            .and_then(|_| temporary.as_file().sync_all())
            .map_err(|error| format!("Cannot write extension lock: {error}"))?;
        temporary
            .persist(path)
            .map(|_| ())
            .map_err(|error| format!("Cannot persist extension lock: {error}"))
    }

    pub fn list(&self) -> Vec<ExtensionLockEntry> {
        self.extensions.values().cloned().collect()
    }

    pub fn get(&self, id: &str) -> Result<&ExtensionLockEntry, String> {
        self.extensions
            .get(id)
            .ok_or_else(|| format!("Extension is not installed: {id}"))
    }

    pub fn set_enabled(&mut self, id: &str, enabled: bool) -> Result<(), String> {
        let entry = self
            .extensions
            .get_mut(id)
            .ok_or_else(|| format!("Extension is not installed: {id}"))?;
        let next = if enabled {
            ExtensionStateKind::Enabled
        } else {
            ExtensionStateKind::Disabled
        };
        if entry.state != next && !entry.state.may_transition_to(next) {
            return Err(format!(
                "Cannot change extension {id} from {:?} to {:?}",
                entry.state, next
            ));
        }
        entry.enabled = enabled;
        entry.state = next;
        entry.updated_at = unix_now();
        Ok(())
    }
}

pub fn current_pointer_path(extensions_dir: &Path, id: &str) -> Result<PathBuf, String> {
    validate_id(id)?;
    Ok(extensions_dir.join(id).join("current.json"))
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct CurrentPointer<'a> {
    version: &'a str,
    previous_version: Option<&'a str>,
}

pub fn write_current_pointer(
    extensions_dir: &Path,
    entry: &ExtensionLockEntry,
) -> Result<(), String> {
    if entry.install_type == ExtensionInstallType::Linked {
        return Ok(());
    }
    let path = current_pointer_path(extensions_dir, &entry.id)?;
    let parent = path.parent().ok_or("Invalid current pointer path")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create extension directory: {error}"))?;
    let bytes = serde_json::to_vec_pretty(&CurrentPointer {
        version: &entry.current_version,
        previous_version: entry.previous_version.as_deref(),
    })
    .map_err(|error| format!("Cannot serialize current pointer: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create current pointer: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write current pointer: {error}"))?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| format!("Cannot persist current pointer: {error}"))
}

pub fn validate_id(id: &str) -> Result<(), String> {
    if id.is_empty()
        || !id.bytes().all(|byte| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || b"._-".contains(&byte)
        })
        || !id.bytes().any(|byte| b"._-".contains(&byte))
    {
        return Err(format!("Invalid extension id: {id}"));
    }
    Ok(())
}

fn validate_managed_version(version: &str) -> Result<(), String> {
    Version::parse(version)
        .map(|_| ())
        .map_err(|error| format!("Invalid managed extension version {version}: {error}"))
}

pub fn unix_now() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn persisted_state_machine_handles_availability_changes() {
        assert!(ExtensionStateKind::Enabled.may_transition_to(ExtensionStateKind::Disabled));
        assert!(ExtensionStateKind::Enabled.may_transition_to(ExtensionStateKind::Broken));
        assert!(ExtensionStateKind::Broken.may_transition_to(ExtensionStateKind::Enabled));
        assert!(!ExtensionStateKind::Enabled.may_transition_to(ExtensionStateKind::Enabled));
    }

    #[test]
    fn rejects_ids_that_could_escape_the_extension_root() {
        assert!(validate_id("io.example.tool").is_ok());
        assert!(validate_id("../tool").is_err());
        assert!(validate_id("UPPER.tool").is_err());
    }

    #[test]
    fn rejects_managed_versions_that_could_escape_the_extension_root() {
        assert!(validate_managed_version("1.2.3").is_ok());
        assert!(validate_managed_version("../../outside").is_err());
        assert!(validate_managed_version("latest").is_err());
    }

    #[test]
    fn old_lock_entries_default_to_executable_providers() {
        let value = serde_json::json!({
            "id": "example.tool",
            "name": "Example",
            "publisherId": "example",
            "publisherName": "Example",
            "installType": "linked",
            "state": "enabled",
            "enabled": true,
            "packageName": null,
            "packageVersion": "linked",
            "toolVersion": null,
            "integrity": null,
            "signatureVerified": false,
            "previousSignatureVerified": null,
            "currentVersion": "linked",
            "previousVersion": null,
            "manifestPath": "/tmp/floter.extension.json",
            "executablePath": "/tmp/example",
            "runtimeRoot": null,
            "installedAt": 1,
            "updatedAt": 1,
            "pinned": false,
            "channel": "external"
        });
        let entry: ExtensionLockEntry = serde_json::from_value(value).unwrap();
        assert_eq!(entry.provider_kind, ExtensionProviderKind::Executable);
    }
}
