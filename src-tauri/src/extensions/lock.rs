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

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExtensionStateKind {
    Resolving,
    Downloading,
    Verifying,
    Installing,
    Enabled,
    Disabled,
    Updating,
    Rollback,
    Broken,
    Removing,
}

impl ExtensionStateKind {
    pub fn may_transition_to(self, next: Self) -> bool {
        use ExtensionStateKind::*;
        matches!(
            (self, next),
            (Resolving, Downloading)
                | (Downloading, Verifying)
                | (Verifying, Installing)
                | (Installing, Enabled | Disabled)
                | (Enabled, Disabled | Updating | Removing | Broken)
                | (Disabled, Enabled | Updating | Removing | Broken)
                | (Updating, Enabled | Disabled | Rollback | Broken)
                | (Rollback, Enabled | Disabled | Broken)
                | (Broken, Updating | Removing | Rollback)
                | (Removing, Broken)
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
    pub state: ExtensionStateKind,
    pub enabled: bool,
    pub package_name: Option<String>,
    pub package_version: String,
    pub tool_version: Option<String>,
    pub integrity: Option<String>,
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
    fn state_machine_allows_update_rollback() {
        assert!(ExtensionStateKind::Enabled.may_transition_to(ExtensionStateKind::Updating));
        assert!(ExtensionStateKind::Updating.may_transition_to(ExtensionStateKind::Rollback));
        assert!(ExtensionStateKind::Rollback.may_transition_to(ExtensionStateKind::Enabled));
        assert!(!ExtensionStateKind::Enabled.may_transition_to(ExtensionStateKind::Installing));
    }

    #[test]
    fn rejects_ids_that_could_escape_the_extension_root() {
        assert!(validate_id("io.example.tool").is_ok());
        assert!(validate_id("../tool").is_err());
        assert!(validate_id("UPPER.tool").is_err());
    }
}
