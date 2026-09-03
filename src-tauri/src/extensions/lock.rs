use semver::Version;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::io::Write;
use std::path::{Path, PathBuf};

use crate::extensions::asset_matcher::AssetSelection;
use crate::extensions::manifest::Permission;

const LOCK_SCHEMA_VERSION: u32 = 2;

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
pub enum ExtensionDistributionSource {
    Npm,
    Local,
    BuiltIn,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExtensionRuntimeOwnership {
    Bundled,
    System,
}

#[derive(Debug, Clone, Copy, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ExtensionProviderKind {
    #[default]
    Executable,
    StaticDescriptor,
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
    pub distribution_source: ExtensionDistributionSource,
    pub runtime_ownership: ExtensionRuntimeOwnership,
    #[serde(default)]
    pub provider_kind: ExtensionProviderKind,
    pub state: ExtensionStateKind,
    pub enabled: bool,
    pub package_name: Option<String>,
    pub package_version: String,
    pub tool_version: Option<String>,
    pub integrity: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub runtime_integrity: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub content_integrity: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub previous_integrity: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub previous_runtime_integrity: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub previous_content_integrity: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub asset_selection: Option<AssetSelection>,
    #[serde(default)]
    pub signature_verified: bool,
    #[serde(default)]
    pub previous_signature_verified: Option<bool>,
    #[serde(default)]
    pub official_verified: bool,
    #[serde(default)]
    pub previous_official_verified: Option<bool>,
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
    /// Permission set the user actually approved, recorded at approval time.
    /// Empty when the manifest declares no permissions. This is the audit
    /// record: the live manifest permissions may drift after an update, and
    /// comparing against this set is how added permissions are detected.
    #[serde(default)]
    pub approved_permissions: Vec<Permission>,
    /// Unix timestamp of the last permission approval (install, update with
    /// new permissions, or reconnect). Zero when never approved.
    #[serde(default)]
    pub approved_at: u64,
    /// SHA-256 of the exact manifest bytes the approval applies to. A later
    /// manifest with different bytes cannot silently inherit this approval.
    #[serde(default)]
    pub approved_manifest_digest: Option<String>,
    /// Structured error code of the last failed verify/describe/probe
    /// operation (for example `integrity-mismatch`). Cleared on success.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_error_code: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_error_detail: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_error_at: Option<u64>,
    /// Why the extension entered the `broken` state, if it is broken. The
    /// state alone says that something failed; this keeps the reason
    /// visible across restarts until repair succeeds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub broken_reason: Option<String>,
    /// Enabled/disabled intent captured when the entry entered `broken`.
    /// `mark_broken` forces `enabled=false` while the runtime is unusable;
    /// this remembers what the user actually wanted so `clear_broken` can
    /// restore it. `None` on entries broken before the field existed (serde
    /// default keeps old lock files loading), in which case clearing falls
    /// back to the persisted `enabled` flag.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled_before_broken: Option<bool>,
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
            if entry.distribution_source == ExtensionDistributionSource::Npm {
                validate_npm_version(&entry.package_version)?;
                validate_npm_version(&entry.current_version)?;
                if let Some(previous) = &entry.previous_version {
                    validate_npm_version(previous)?;
                }
                normalize_release_channel(&entry.channel)?;
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
            .map_err(|error| format!("Cannot persist extension lock: {error}"))?;
        sync_directory(parent)
            .map_err(|error| format!("Cannot sync extension lock directory: {error}"))
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

    pub fn set_release_policy(
        &mut self,
        id: &str,
        pinned: Option<bool>,
        channel: Option<&str>,
    ) -> Result<(), String> {
        let entry = self
            .extensions
            .get_mut(id)
            .ok_or_else(|| format!("Extension is not installed: {id}"))?;
        if entry.distribution_source != ExtensionDistributionSource::Npm {
            return Err("Release policy is only available for NPM integrations".to_string());
        }
        if let Some(channel) = channel {
            entry.channel = normalize_release_channel(channel)?.to_string();
        }
        if let Some(pinned) = pinned {
            entry.pinned = pinned;
        }
        entry.updated_at = unix_now();
        Ok(())
    }

    /// Record a failed verify/describe/probe as the structured `broken` state.
    /// Idempotent for an already-broken extension: the first failure wins so
    /// the stored reason stays the one that broke it, and repeated failures
    /// refresh only the error code and timestamp. Returns false when the state
    /// machine rejects the transition (for example a not-yet-installed entry).
    pub fn mark_broken(&mut self, id: &str, code: &str, detail: &str) -> Result<bool, String> {
        let entry = self
            .extensions
            .get_mut(id)
            .ok_or_else(|| format!("Extension is not installed: {id}"))?;
        if entry.state != ExtensionStateKind::Broken {
            if !entry.state.may_transition_to(ExtensionStateKind::Broken) {
                return Ok(false);
            }
            entry.state = ExtensionStateKind::Broken;
            // Remember the user's enabled/disabled intent before forcing the
            // entry out of the catalog, so recovery can restore it later.
            entry.enabled_before_broken = Some(entry.enabled);
            // A broken extension must not stay runnable in the catalog; the
            // enabled flag follows the state the way set_enabled keeps them.
            entry.enabled = false;
            entry.broken_reason = Some(detail.to_string());
        }
        entry.last_error_code = Some(code.to_string());
        entry.last_error_detail = Some(detail.to_string());
        entry.last_error_at = Some(unix_now());
        entry.updated_at = unix_now();
        Ok(true)
    }

    /// Clear `broken` and any recorded operation error after a successful
    /// verify/repair/install. The target state is the intent recorded when
    /// the entry broke (`enabledBeforeBroken`); entries broken before that
    /// field existed fall back to the persisted `enabled` flag's value,
    /// mirroring how enable/disable persists both fields.
    pub fn clear_broken(&mut self, id: &str) -> Result<bool, String> {
        let entry = self
            .extensions
            .get_mut(id)
            .ok_or_else(|| format!("Extension is not installed: {id}"))?;
        let was_broken = entry.state == ExtensionStateKind::Broken;
        if was_broken {
            if !ExtensionStateKind::Broken.may_transition_to(ExtensionStateKind::Enabled) {
                return Ok(false);
            }
            let restored_enabled = entry.enabled_before_broken.unwrap_or(entry.enabled);
            entry.enabled = restored_enabled;
            entry.state = if restored_enabled {
                ExtensionStateKind::Enabled
            } else {
                ExtensionStateKind::Disabled
            };
            entry.broken_reason = None;
            entry.enabled_before_broken = None;
        }
        if entry.last_error_code.is_some() {
            entry.last_error_code = None;
            entry.last_error_detail = None;
            entry.last_error_at = None;
            entry.updated_at = unix_now();
        }
        Ok(was_broken)
    }

    /// Persist the permission approval audit record: the exact approved set,
    /// when it was approved, and the manifest bytes it applies to.
    pub fn record_permission_approval(
        &mut self,
        id: &str,
        approved: &[Permission],
        manifest_digest: &str,
    ) -> Result<(), String> {
        let entry = self
            .extensions
            .get_mut(id)
            .ok_or_else(|| format!("Extension is not installed: {id}"))?;
        let mut approved = approved.to_vec();
        approved.sort_unstable();
        entry.approved_permissions = approved;
        entry.approved_at = unix_now();
        entry.approved_manifest_digest = Some(manifest_digest.to_string());
        entry.updated_at = unix_now();
        Ok(())
    }

    /// True when the recorded approval covers exactly this permission set for
    /// these manifest bytes. A digest mismatch forces re-approval even when
    /// the permission lists happen to be equal.
    pub fn has_valid_approval(
        &self,
        id: &str,
        requested: &[Permission],
        manifest_digest: &str,
    ) -> bool {
        let Some(entry) = self.extensions.get(id) else {
            return false;
        };
        if entry.approved_manifest_digest.as_deref() != Some(manifest_digest) {
            return false;
        }
        let mut requested = requested.to_vec();
        requested.sort_unstable();
        requested == entry.approved_permissions
    }
}

pub fn normalize_release_channel(channel: &str) -> Result<&str, String> {
    let channel = channel.trim();
    if channel == "latest" {
        return Ok("stable");
    }
    if channel.is_empty()
        || channel.len() > 64
        || !channel.bytes().all(|byte| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || b"._-".contains(&byte)
        })
        || !channel
            .as_bytes()
            .first()
            .is_some_and(u8::is_ascii_alphanumeric)
        || Version::parse(channel).is_ok()
    {
        return Err(format!("Invalid NPM release channel: {channel}"));
    }
    Ok(channel)
}

pub fn release_channel_selector(channel: &str) -> Result<&str, String> {
    match normalize_release_channel(channel)? {
        "stable" => Ok("latest"),
        other => Ok(other),
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
    if entry.distribution_source != ExtensionDistributionSource::Npm {
        return Ok(());
    }
    let path = current_pointer_path(extensions_dir, &entry.id)?;
    let parent = path
        .parent()
        .ok_or("Invalid current pointer path")?
        .to_path_buf();
    std::fs::create_dir_all(&parent)
        .map_err(|error| format!("Cannot create extension directory: {error}"))?;
    let bytes = serde_json::to_vec_pretty(&CurrentPointer {
        version: &entry.current_version,
        previous_version: entry.previous_version.as_deref(),
    })
    .map_err(|error| format!("Cannot serialize current pointer: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(&parent)
        .map_err(|error| format!("Cannot create current pointer: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write current pointer: {error}"))?;
    temporary
        .persist(path)
        .map_err(|error| format!("Cannot persist current pointer: {error}"))?;
    sync_directory(&parent)
        .map_err(|error| format!("Cannot sync current pointer directory: {error}"))
}

pub(crate) fn sync_directory(path: &Path) -> std::io::Result<()> {
    #[cfg(unix)]
    {
        std::fs::File::open(path)?.sync_all()
    }
    #[cfg(not(unix))]
    {
        let _ = path;
        Ok(())
    }
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

fn validate_npm_version(version: &str) -> Result<(), String> {
    Version::parse(version)
        .map(|_| ())
        .map_err(|error| format!("Invalid NPM integration version {version}: {error}"))
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
    fn rejects_npm_versions_that_could_escape_the_extension_root() {
        assert!(validate_npm_version("1.2.3").is_ok());
        assert!(validate_npm_version("../../outside").is_err());
        assert!(validate_npm_version("latest").is_err());
    }

    #[test]
    fn old_lock_entries_default_to_unverified_official_source() {
        let entry: ExtensionLockEntry = serde_json::from_value(serde_json::json!({
            "id": "example.tool",
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
            "signatureVerified": true,
            "currentVersion": "local",
            "previousVersion": null,
            "manifestPath": "/tmp/floter.extension.json",
            "executablePath": "/tmp/example",
            "runtimeRoot": null,
            "installedAt": 1,
            "updatedAt": 1,
            "pinned": false,
            "channel": "external"
        }))
        .unwrap();
        assert!(!entry.official_verified);
        assert_eq!(entry.previous_official_verified, None);
        assert_eq!(entry.runtime_integrity, None);
        assert_eq!(entry.content_integrity, None);
        assert_eq!(entry.previous_integrity, None);
    }

    #[test]
    fn legacy_lock_file_with_populated_npm_entry_still_loads() {
        // Regression for the NPM distribution removal: lock files written by
        // older builds carry managed entries with integrity/signature fields
        // populated. They must keep parsing without error.
        let directory = tempfile::tempdir().unwrap();
        let path = directory.path().join("extensions.lock.json");
        std::fs::write(
            &path,
            serde_json::to_vec_pretty(&serde_json::json!({
                "schemaVersion": 2,
                "extensions": {
                    "legacy.npm.tool": {
                        "id": "legacy.npm.tool",
                        "name": "Legacy NPM Tool",
                        "publisherId": "example",
                        "publisherName": "Example",
                        "distributionSource": "npm",
                        "runtimeOwnership": "bundled",
                        "providerKind": "executable",
                        "state": "enabled",
                        "enabled": true,
                        "packageName": "@example/legacy-npm-tool",
                        "packageVersion": "1.2.3",
                        "toolVersion": "1.0.0",
                        "integrity": "sha512-AAAA",
                        "runtimeIntegrity": "sha512-BBBB",
                        "contentIntegrity": "sha512-CCCC",
                        "previousIntegrity": null,
                        "previousRuntimeIntegrity": null,
                        "previousContentIntegrity": null,
                        "assetSelection": null,
                        "signatureVerified": true,
                        "previousSignatureVerified": null,
                        "officialVerified": true,
                        "previousOfficialVerified": null,
                        "currentVersion": "1.2.3",
                        "previousVersion": "1.2.2",
                        "manifestPath": "/tmp/versions/1.2.3/floter.extension.json",
                        "executablePath": "/tmp/versions/1.2.3/runtime/tool",
                        "runtimeRoot": "/tmp/versions/1.2.3/runtime",
                        "installedAt": 100,
                        "updatedAt": 200,
                        "pinned": false,
                        "channel": "stable"
                    }
                }
            }))
            .unwrap(),
        )
        .unwrap();

        let lock = ExtensionsLock::load(&path).unwrap();
        let entry = lock.extensions.get("legacy.npm.tool").unwrap();
        assert_eq!(entry.distribution_source, ExtensionDistributionSource::Npm);
        assert_eq!(
            entry.package_name.as_deref(),
            Some("@example/legacy-npm-tool")
        );
        assert_eq!(entry.current_version, "1.2.3");
        assert_eq!(entry.integrity.as_deref(), Some("sha512-AAAA"));
        assert_eq!(entry.runtime_integrity.as_deref(), Some("sha512-BBBB"));
        assert_eq!(entry.content_integrity.as_deref(), Some("sha512-CCCC"));
        assert!(entry.signature_verified);
        assert!(entry.official_verified);
    }

    #[test]
    fn release_channels_normalize_stable_and_reject_versions() {
        assert_eq!(normalize_release_channel("latest").unwrap(), "stable");
        assert_eq!(release_channel_selector("stable").unwrap(), "latest");
        assert_eq!(release_channel_selector("beta").unwrap(), "beta");
        assert!(normalize_release_channel("1.2.3").is_err());
        assert!(normalize_release_channel("../beta").is_err());
    }

    fn test_entry(id: &str, state: ExtensionStateKind) -> ExtensionLockEntry {
        let bytes = serde_json::to_vec(&serde_json::json!({
            "id": id,
            "name": "Example",
            "publisherId": "example",
            "publisherName": "Example",
            "distributionSource": "local",
            "runtimeOwnership": "system",
            "providerKind": "executable",
            "state": state,
            "enabled": state == ExtensionStateKind::Enabled,
            "packageName": null,
            "packageVersion": "local",
            "toolVersion": null,
            "integrity": null,
            "signatureVerified": false,
            "currentVersion": "local",
            "previousVersion": null,
            "manifestPath": "/tmp/floter.extension.json",
            "executablePath": "/tmp/example",
            "runtimeRoot": null,
            "installedAt": 1,
            "updatedAt": 1,
            "pinned": false,
            "channel": "external"
        }))
        .unwrap();
        serde_json::from_slice(&bytes).unwrap()
    }

    #[test]
    fn mark_broken_persists_state_reason_and_error_code() {
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            "example.tool".into(),
            test_entry("example.tool", ExtensionStateKind::Enabled),
        );

        assert!(lock
            .mark_broken("example.tool", "integrity-mismatch", "tree hash changed")
            .unwrap());
        let entry = lock.get("example.tool").unwrap();
        assert_eq!(entry.state, ExtensionStateKind::Broken);
        assert!(!entry.enabled);
        assert_eq!(entry.broken_reason.as_deref(), Some("tree hash changed"));
        assert_eq!(entry.last_error_code.as_deref(), Some("integrity-mismatch"));
        assert!(entry.last_error_at.is_some());

        // Already-broken: first reason wins, only the error timestamp refreshes.
        let recorded_at = entry.last_error_at;
        assert!(lock
            .mark_broken("example.tool", "describe-failed", "later problem")
            .unwrap());
        let entry = lock.get("example.tool").unwrap();
        assert_eq!(entry.broken_reason.as_deref(), Some("tree hash changed"));
        assert_eq!(entry.last_error_code.as_deref(), Some("describe-failed"));
        assert_eq!(entry.last_error_at, recorded_at);
    }

    #[test]
    fn clear_broken_restores_enabled_flag_and_drops_the_error_record() {
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            "example.tool".into(),
            test_entry("example.tool", ExtensionStateKind::Broken),
        );
        lock.extensions
            .get_mut("example.tool")
            .unwrap()
            .last_error_code = Some("x".into());
        lock.extensions.get_mut("example.tool").unwrap().enabled = true;

        assert!(lock.clear_broken("example.tool").unwrap());
        let entry = lock.get("example.tool").unwrap();
        assert_eq!(entry.state, ExtensionStateKind::Enabled);
        assert_eq!(entry.broken_reason, None);
        assert_eq!(entry.last_error_code, None);

        // A disabled-before-broken extension returns to disabled, not enabled.
        lock.extensions.insert(
            "other.tool".into(),
            test_entry("other.tool", ExtensionStateKind::Broken),
        );
        lock.extensions.get_mut("other.tool").unwrap().enabled = false;
        assert!(lock.clear_broken("other.tool").unwrap());
        assert_eq!(
            lock.get("other.tool").unwrap().state,
            ExtensionStateKind::Disabled
        );

        // Clearing a healthy extension is a no-op that reports nothing changed.
        lock.extensions.insert(
            "third.tool".into(),
            test_entry("third.tool", ExtensionStateKind::Enabled),
        );
        assert!(!lock.clear_broken("third.tool").unwrap());
    }

    #[test]
    fn broken_recovery_restores_the_pre_broken_enabled_intent() {
        let mut lock = ExtensionsLock::default();

        // Enabled before breaking -> recovery restores the enabled state.
        lock.extensions.insert(
            "enabled.tool".into(),
            test_entry("enabled.tool", ExtensionStateKind::Enabled),
        );
        lock.mark_broken("enabled.tool", "binding-missing", "gone")
            .unwrap();
        assert_eq!(
            lock.get("enabled.tool").unwrap().enabled_before_broken,
            Some(true)
        );
        assert!(lock.clear_broken("enabled.tool").unwrap());
        let restored = lock.get("enabled.tool").unwrap();
        assert_eq!(restored.state, ExtensionStateKind::Enabled);
        assert!(restored.enabled);
        assert_eq!(restored.enabled_before_broken, None);

        // Disabled before breaking -> recovery keeps it disabled.
        lock.extensions.insert(
            "disabled.tool".into(),
            test_entry("disabled.tool", ExtensionStateKind::Disabled),
        );
        lock.mark_broken("disabled.tool", "binding-missing", "gone")
            .unwrap();
        assert_eq!(
            lock.get("disabled.tool").unwrap().enabled_before_broken,
            Some(false)
        );
        assert!(lock.clear_broken("disabled.tool").unwrap());
        let restored = lock.get("disabled.tool").unwrap();
        assert_eq!(restored.state, ExtensionStateKind::Disabled);
        assert!(!restored.enabled);

        // Legacy entry broken before the field existed (serde default None)
        // still recovers through the persisted enabled flag.
        lock.extensions.insert(
            "legacy.tool".into(),
            test_entry("legacy.tool", ExtensionStateKind::Broken),
        );
        assert_eq!(lock.get("legacy.tool").unwrap().enabled_before_broken, None);
        assert!(lock.clear_broken("legacy.tool").unwrap());
        let restored = lock.get("legacy.tool").unwrap();
        assert_ne!(restored.state, ExtensionStateKind::Broken);
        assert_eq!(restored.enabled_before_broken, None);
    }

    #[test]
    fn permission_approval_is_bound_to_the_manifest_digest() {
        use crate::extensions::manifest::Permission;

        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            "example.tool".into(),
            test_entry("example.tool", ExtensionStateKind::Enabled),
        );

        lock.record_permission_approval(
            "example.tool",
            &[Permission::ClipboardWrite, Permission::NetworkFetch],
            "digest-1",
        )
        .unwrap();
        // Order-independent set comparison against the same manifest.
        assert!(lock.has_valid_approval(
            "example.tool",
            &[Permission::NetworkFetch, Permission::ClipboardWrite],
            "digest-1"
        ));
        // Different manifest bytes invalidate the approval even for the same set.
        assert!(!lock.has_valid_approval(
            "example.tool",
            &[Permission::NetworkFetch, Permission::ClipboardWrite],
            "digest-2"
        ));
        // A changed permission set needs re-approval.
        assert!(!lock.has_valid_approval("example.tool", &[Permission::NetworkFetch], "digest-1"));

        let entry = lock.get("example.tool").unwrap();
        assert_eq!(
            entry.approved_permissions,
            vec![Permission::NetworkFetch, Permission::ClipboardWrite]
        );
        assert!(entry.approved_at > 0);
    }

    #[test]
    fn old_lock_entries_load_with_default_phase_2_fields() {
        // Regression: Phase 2 added approved_at, approved_manifest_digest,
        // last_error_code, last_error_detail, broken_reason, and
        // enabled_before_broken. Lock files written before Phase 2 must still
        // load, with new fields taking serde defaults and pinned/channel
        // preserved.
        let old_entry_json = serde_json::json!({
            "id": "example.tool",
            "name": "Example Tool",
            "publisherId": "example.publisher",
            "publisherName": "Example Publisher",
            "distributionSource": "local",
            "runtimeOwnership": "system",
            "providerKind": "executable",
            "state": "enabled",
            "enabled": true,
            "packageVersion": "1.0.0",
            "currentVersion": "1.0.0",
            "manifestPath": "/path/to/manifest",
            "executablePath": "/usr/bin/example",
            "installedAt": 1693000000,
            "updatedAt": 1693000000,
            "pinned": true,
            "channel": "beta",
            "approvedPermissions": ["filesystem-read"]
        });
        let entry: ExtensionLockEntry = serde_json::from_value(old_entry_json).unwrap();
        assert_eq!(entry.id, "example.tool");
        assert_eq!(entry.pinned, true);
        assert_eq!(entry.channel, "beta");
        assert_eq!(entry.approved_permissions.len(), 1);
        // New Phase 2 fields default correctly:
        assert_eq!(entry.approved_at, 0);
        assert_eq!(entry.approved_manifest_digest, None);
        assert_eq!(entry.last_error_code, None);
        assert_eq!(entry.last_error_detail, None);
        assert_eq!(entry.broken_reason, None);
        assert_eq!(entry.enabled_before_broken, None);
    }
}
