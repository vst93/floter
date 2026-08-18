//! User selections. A lock is a binding, not a hint: a missing executable
//! enters reconnect state and is never silently replaced by another candidate.

use super::inventory::{ToolCandidate, ToolLocator};
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::io::Write;
use std::path::Path;
use std::time::{SystemTime, UNIX_EPOCH};

const SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolLock {
    pub schema_version: u32,
    #[serde(default)]
    pub tools: BTreeMap<String, ToolLockEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolLockEntry {
    pub tool: String,
    pub locator: ToolLocator,
    pub fingerprint: Option<String>,
    pub locked_at: u64,
    pub state: LockState,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum LockState {
    Connected,
    ReconnectRequired,
    ReverifyRequired,
}

impl Default for ToolLock {
    fn default() -> Self {
        Self {
            schema_version: SCHEMA_VERSION,
            tools: BTreeMap::new(),
        }
    }
}

impl ToolLock {
    pub fn load(path: &Path) -> Result<Self, String> {
        if !path.exists() {
            return Ok(Self::default());
        }
        let bytes = std::fs::read(path).map_err(|e| format!("Cannot read tool lock: {e}"))?;
        let lock: Self =
            serde_json::from_slice(&bytes).map_err(|e| format!("Invalid tool lock: {e}"))?;
        if lock.schema_version != SCHEMA_VERSION {
            return Err(format!(
                "Unsupported tool lock schema version {}",
                lock.schema_version
            ));
        }
        Ok(lock)
    }

    pub fn save(&self, path: &Path) -> Result<(), String> {
        let parent = path.parent().ok_or("Invalid tool lock path")?;
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("Cannot create tool lock directory: {e}"))?;
        let bytes = serde_json::to_vec_pretty(self).map_err(|e| e.to_string())?;
        let mut temp = tempfile::NamedTempFile::new_in(parent).map_err(|e| e.to_string())?;
        temp.write_all(&bytes)
            .and_then(|_| temp.flush())
            .and_then(|_| temp.as_file().sync_all())
            .map_err(|e| e.to_string())?;
        temp.persist(path).map_err(|e| e.to_string())?;
        #[cfg(unix)]
        std::fs::File::open(parent)
            .and_then(|directory| directory.sync_all())
            .map_err(|e| e.to_string())?;
        Ok(())
    }

    pub fn bind(&mut self, tool: impl Into<String>, candidate: &ToolCandidate) {
        self.bind_locator(
            tool,
            candidate.locator.clone(),
            candidate.fingerprint.clone(),
        );
    }

    pub fn bind_locator(
        &mut self,
        tool: impl Into<String>,
        locator: ToolLocator,
        fingerprint: Option<String>,
    ) {
        let tool = tool.into();
        self.tools.insert(
            tool.clone(),
            ToolLockEntry {
                tool,
                locator,
                fingerprint,
                locked_at: unix_now(),
                state: LockState::Connected,
            },
        );
    }

    pub fn check(
        &mut self,
        tool: &str,
        candidate: Option<&ToolCandidate>,
    ) -> Result<&ToolLockEntry, String> {
        let entry = self
            .tools
            .get_mut(tool)
            .ok_or_else(|| format!("Tool is not locked: {tool}"))?;
        if let ToolLocator::Executable { path } = &entry.locator {
            if !Path::new(path).is_file() {
                entry.state = LockState::ReconnectRequired;
                return Ok(entry);
            }
        }
        if let Some(candidate) = candidate {
            entry.state = if candidate.locator.normalized() == entry.locator.normalized()
                && candidate.fingerprint == entry.fingerprint
            {
                LockState::Connected
            } else {
                LockState::ReverifyRequired
            };
        }
        Ok(entry)
    }

    pub fn reconnect(&mut self, tool: &str, candidate: &ToolCandidate) -> Result<(), String> {
        let entry = self
            .tools
            .get_mut(tool)
            .ok_or_else(|| format!("Tool is not locked: {tool}"))?;
        entry.locator = candidate.locator.clone();
        entry.fingerprint = candidate.fingerprint.clone();
        entry.locked_at = unix_now();
        entry.state = LockState::Connected;
        Ok(())
    }

    pub fn remove(&mut self, tool: &str) -> Option<ToolLockEntry> {
        self.tools.remove(tool)
    }
}

fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::inventory::{DiscoveryQuality, DiscoverySource};

    fn candidate(path: &str) -> ToolCandidate {
        ToolCandidate {
            id: path.into(),
            name: "tool".into(),
            locator: ToolLocator::Executable { path: path.into() },
            version: None,
            sources: vec![DiscoverySource::Path],
            quality: DiscoveryQuality::AutoDetected,
            available: true,
            fingerprint: None,
        }
    }

    #[test]
    fn default_is_current_schema() {
        assert_eq!(ToolLock::default().schema_version, 1);
    }

    #[test]
    fn a_different_locator_requires_reverification_even_without_fingerprints() {
        let temporary = tempfile::tempdir().unwrap();
        let original = temporary.path().join("original");
        let replacement = temporary.path().join("replacement");
        std::fs::write(&original, "original").unwrap();
        std::fs::write(&replacement, "replacement").unwrap();
        let original = candidate(&original.to_string_lossy());
        let replacement = candidate(&replacement.to_string_lossy());
        let mut lock = ToolLock::default();
        lock.bind("tool", &original);

        let entry = lock.check("tool", Some(&replacement)).unwrap();

        assert_eq!(entry.state, LockState::ReverifyRequired);
    }

    #[test]
    fn save_and_load_preserve_bindings() {
        let temporary = tempfile::tempdir().unwrap();
        let executable = temporary.path().join("tool");
        std::fs::write(&executable, "tool").unwrap();
        let candidate = candidate(&executable.to_string_lossy());
        let mut lock = ToolLock::default();
        lock.bind("tool", &candidate);
        let path = temporary.path().join("tool-lock.json");

        lock.save(&path).unwrap();
        let loaded = ToolLock::load(&path).unwrap();

        assert_eq!(loaded.tools["tool"].locator, candidate.locator);
        assert_eq!(loaded.tools["tool"].state, LockState::Connected);
    }

    #[test]
    fn reconnect_replaces_the_explicit_binding_and_remove_unbinds_it() {
        let temporary = tempfile::tempdir().unwrap();
        let original = temporary.path().join("original");
        let replacement = temporary.path().join("replacement");
        std::fs::write(&original, "original").unwrap();
        std::fs::write(&replacement, "replacement").unwrap();
        let original = candidate(&original.to_string_lossy());
        let replacement = candidate(&replacement.to_string_lossy());
        let mut lock = ToolLock::default();
        lock.bind("tool", &original);

        lock.reconnect("tool", &replacement).unwrap();
        assert_eq!(lock.tools["tool"].locator, replacement.locator);
        assert!(lock.remove("tool").is_some());
        assert!(!lock.tools.contains_key("tool"));
    }
}
