use crate::extensions::config;
use crate::extensions::install::{self, ExtensionInstallRequest, InstallSource};
use crate::extensions::lock::{
    validate_id, ExtensionInstallType, ExtensionLockEntry, ExtensionsLock,
};
use crate::extensions::manifest::{ExtensionManifest, Permission, Runtime};
use crate::extensions::ExtensionState;
use chrono::{DateTime, NaiveDate, SecondsFormat, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashSet};
use std::io::Write;
use std::path::{Path, PathBuf};

const SYNC_FORMAT_VERSION: u32 = 1;
const MAX_IMPORT_BYTES: u64 = 4 * 1024 * 1024;
const MAX_IMPORT_EXTENSIONS: usize = 1_000;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ExtensionsSyncDocument {
    pub version: u32,
    pub exported_at: String,
    pub extensions: Vec<ExtensionsSyncEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ExtensionsSyncEntry {
    pub id: String,
    pub version: String,
    pub enabled: bool,
    #[serde(default)]
    pub config: BTreeMap<String, Value>,
    pub source: SyncSource,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub package: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub manifest: Option<ExtensionManifest>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SyncSource {
    Managed,
    Linked,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionsExportResult {
    pub path: String,
    pub extension_count: usize,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionsImportReport {
    pub path: String,
    pub succeeded: Vec<ExtensionsImportItem>,
    pub failed: Vec<ExtensionsImportItem>,
    pub skipped: Vec<ExtensionsImportItem>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionsImportItem {
    pub id: String,
    pub message: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ReconcileAction {
    Install,
    Update,
    Rollback,
    Ready,
}

pub fn default_export_file_name(date: NaiveDate) -> String {
    format!("floter-extensions-{date}.json")
}

pub fn build_export(
    state: &ExtensionState,
    now: DateTime<Utc>,
) -> Result<ExtensionsSyncDocument, String> {
    let entries = ExtensionsLock::load(&state.paths.lock_file)?.list();
    let extensions = entries
        .into_iter()
        .map(|entry| export_entry(state, entry))
        .collect::<Result<Vec<_>, _>>()?;
    Ok(ExtensionsSyncDocument {
        version: SYNC_FORMAT_VERSION,
        exported_at: now.to_rfc3339_opts(SecondsFormat::Secs, true),
        extensions,
    })
}

fn export_entry(
    state: &ExtensionState,
    entry: ExtensionLockEntry,
) -> Result<ExtensionsSyncEntry, String> {
    let source = source_for(&entry);
    let manifest = if source == SyncSource::Linked {
        Some(linked_manifest(state, &entry)?)
    } else {
        None
    };
    Ok(ExtensionsSyncEntry {
        id: entry.id.clone(),
        version: installed_version(&entry).to_string(),
        enabled: entry.enabled,
        config: config::export_values(&state.paths.data, &entry.id)?,
        source,
        package: entry.package_name,
        manifest,
    })
}

fn linked_manifest(
    state: &ExtensionState,
    entry: &ExtensionLockEntry,
) -> Result<ExtensionManifest, String> {
    ExtensionManifest::load(Path::new(&entry.manifest_path)).or_else(|file_error| {
        state
            .static_adapters
            .iter()
            .find(|adapter| adapter.manifest.id == entry.id)
            .map(|adapter| adapter.manifest.clone())
            .ok_or_else(|| {
                format!(
                    "Cannot export linked extension {} because its manifest is unavailable: {file_error}",
                    entry.id
                )
            })
    })
}

pub fn write_export(path: &Path, document: &ExtensionsSyncDocument) -> Result<(), String> {
    let mut bytes = serde_json::to_vec_pretty(document)
        .map_err(|error| format!("Cannot serialize extension export: {error}"))?;
    bytes.push(b'\n');
    atomic_write(path, &bytes, "extension export")
}

pub fn read_import(path: &Path) -> Result<ExtensionsSyncDocument, String> {
    let metadata = std::fs::metadata(path)
        .map_err(|error| format!("Cannot read extension import {}: {error}", path.display()))?;
    if metadata.len() > MAX_IMPORT_BYTES {
        return Err(format!(
            "Extension import exceeds the {MAX_IMPORT_BYTES} byte limit"
        ));
    }
    let bytes = std::fs::read(path)
        .map_err(|error| format!("Cannot read extension import {}: {error}", path.display()))?;
    if bytes.len() as u64 > MAX_IMPORT_BYTES {
        return Err(format!(
            "Extension import exceeds the {MAX_IMPORT_BYTES} byte limit"
        ));
    }
    parse_import(&bytes)
}

fn parse_import(bytes: &[u8]) -> Result<ExtensionsSyncDocument, String> {
    let document: ExtensionsSyncDocument = serde_json::from_slice(bytes)
        .map_err(|error| format!("Invalid extension import JSON: {error}"))?;
    validate_import(&document)?;
    Ok(document)
}

fn validate_import(document: &ExtensionsSyncDocument) -> Result<(), String> {
    if document.version != SYNC_FORMAT_VERSION {
        return Err(format!(
            "Unsupported extension export version {}; expected {SYNC_FORMAT_VERSION}",
            document.version
        ));
    }
    DateTime::parse_from_rfc3339(&document.exported_at)
        .map_err(|error| format!("Invalid exportedAt timestamp: {error}"))?;
    if document.extensions.len() > MAX_IMPORT_EXTENSIONS {
        return Err(format!(
            "Extension import contains more than {MAX_IMPORT_EXTENSIONS} extensions"
        ));
    }
    let mut ids = HashSet::new();
    for entry in &document.extensions {
        validate_id(&entry.id)?;
        if entry.version.trim().is_empty() {
            return Err(format!("Extension {} has an empty version", entry.id));
        }
        if !ids.insert(entry.id.as_str()) {
            return Err(format!("Duplicate extension id: {}", entry.id));
        }
        if let Some(manifest) = &entry.manifest {
            if entry.source != SyncSource::Linked || manifest.id != entry.id {
                return Err(format!(
                    "Extension {} has a mismatched linked manifest",
                    entry.id
                ));
            }
            if !matches!(manifest.runtime, Runtime::Linked { .. }) {
                return Err(format!("Extension {} has a non-linked manifest", entry.id));
            }
            let bytes = serde_json::to_vec(manifest)
                .map_err(|error| format!("Cannot validate linked manifest: {error}"))?;
            ExtensionManifest::parse(&bytes)
                .map_err(|error| format!("Invalid linked manifest for {}: {error}", entry.id))?;
        }
    }
    Ok(())
}

pub async fn import_document(
    state: &ExtensionState,
    path: &Path,
    document: ExtensionsSyncDocument,
    approved_permissions: &BTreeMap<String, Vec<Permission>>,
) -> ExtensionsImportReport {
    let mut report = ExtensionsImportReport {
        path: path.to_string_lossy().into_owned(),
        succeeded: Vec::new(),
        failed: Vec::new(),
        skipped: Vec::new(),
    };
    for entry in document.extensions {
        let id = entry.id.clone();
        match import_entry(
            state,
            &entry,
            approved_permissions.get(&entry.id).map(Vec::as_slice),
        )
        .await
        {
            Ok(true) => report.succeeded.push(ExtensionsImportItem {
                id,
                message: "Installed or restored".to_string(),
            }),
            Ok(false) => report.skipped.push(ExtensionsImportItem {
                id,
                message: "Already matches the import".to_string(),
            }),
            Err(message) => report.failed.push(ExtensionsImportItem { id, message }),
        }
    }
    report
}

async fn import_entry(
    state: &ExtensionState,
    desired: &ExtensionsSyncEntry,
    approved_permissions: Option<&[Permission]>,
) -> Result<bool, String> {
    let installed = ExtensionsLock::load(&state.paths.lock_file)?
        .extensions
        .get(&desired.id)
        .cloned();
    let action = reconcile_action(installed.as_ref(), desired)?;
    let mut changed = action != ReconcileAction::Ready;

    match action {
        ReconcileAction::Install | ReconcileAction::Update => match desired.source {
            SyncSource::Managed => {
                let package = desired.package.as_deref().ok_or_else(|| {
                    format!(
                        "Managed extension {} has no package in the export",
                        desired.id
                    )
                })?;
                install::install_imported_managed(
                    state,
                    &desired.id,
                    package,
                    &desired.version,
                    approved_permissions,
                )
                .await?;
            }
            SyncSource::Linked => {
                install_imported_linked(state, desired, approved_permissions).await?;
            }
        },
        ReconcileAction::Rollback => {
            install::rollback(state, &desired.id).await?;
        }
        ReconcileAction::Ready => {}
    }

    let restored = ExtensionsLock::load(&state.paths.lock_file)?
        .extensions
        .get(&desired.id)
        .cloned()
        .ok_or_else(|| format!("Extension {} was not installed", desired.id))?;
    if installed_version(&restored) != desired.version {
        if installed.is_none() {
            let _ = install::uninstall(state, &desired.id, false).await;
        }
        return Err(format!(
            "Extension {} resolved to version {}, but the export requires {}",
            desired.id,
            installed_version(&restored),
            desired.version
        ));
    }

    if !desired.config.is_empty() {
        changed |= config::import_values(state, &desired.id, &desired.config).await?;
    }
    changed |= restore_enabled(state, &desired.id, desired.enabled).await?;
    Ok(changed)
}

fn reconcile_action(
    installed: Option<&ExtensionLockEntry>,
    desired: &ExtensionsSyncEntry,
) -> Result<ReconcileAction, String> {
    let Some(installed) = installed else {
        return Ok(ReconcileAction::Install);
    };
    if source_for(installed) != desired.source {
        return Err(format!(
            "Extension {} is installed from a different source",
            desired.id
        ));
    }
    if installed_version(installed) == desired.version {
        return Ok(ReconcileAction::Ready);
    }
    match desired.source {
        SyncSource::Managed => {
            let package = desired.package.as_deref().ok_or_else(|| {
                format!(
                    "Managed extension {} has no package in the export",
                    desired.id
                )
            })?;
            if installed.package_name.as_deref() != Some(package) {
                return Err(format!(
                    "Extension {} is installed from a different package",
                    desired.id
                ));
            }
            if installed.previous_version.as_deref() == Some(desired.version.as_str()) {
                Ok(ReconcileAction::Rollback)
            } else {
                Ok(ReconcileAction::Update)
            }
        }
        SyncSource::Linked => Err(format!(
            "Linked executable for {} is version {}, but the export requires {}",
            desired.id,
            installed_version(installed),
            desired.version
        )),
    }
}

async fn install_imported_linked(
    state: &ExtensionState,
    desired: &ExtensionsSyncEntry,
    approved_permissions: Option<&[Permission]>,
) -> Result<(), String> {
    let manifest = desired
        .manifest
        .clone()
        .or_else(|| {
            state
                .static_adapters
                .iter()
                .find(|adapter| adapter.manifest.id == desired.id)
                .map(|adapter| adapter.manifest.clone())
        })
        .ok_or_else(|| format!("Linked extension {} has no manifest", desired.id))?;
    let path = imported_manifest_path(&state.paths.data, &desired.id)?;
    let mut bytes = serde_json::to_vec_pretty(&manifest)
        .map_err(|error| format!("Cannot serialize linked manifest: {error}"))?;
    bytes.push(b'\n');
    atomic_write(&path, &bytes, "linked extension manifest")?;
    install::install(
        state,
        ExtensionInstallRequest {
            source: InstallSource::Linked,
            package: None,
            version: None,
            manifest_path: Some(path.to_string_lossy().into_owned()),
            executable_path: None,
            approved_permissions: approved_permissions.map(<[Permission]>::to_vec),
        },
    )
    .await?;
    Ok(())
}

fn imported_manifest_path(data_root: &Path, extension_id: &str) -> Result<PathBuf, String> {
    validate_id(extension_id)?;
    Ok(data_root
        .join(extension_id)
        .join("sync")
        .join("floter.extension.json"))
}

async fn restore_enabled(
    state: &ExtensionState,
    extension_id: &str,
    enabled: bool,
) -> Result<bool, String> {
    let _guard = state.mutation_lock.lock().await;
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if lock.get(extension_id)?.enabled == enabled {
        return Ok(false);
    }
    if !enabled {
        state.provider.cancel_completions();
    }
    lock.set_enabled(extension_id, enabled)?;
    lock.save(&state.paths.lock_file)?;
    Ok(true)
}

fn installed_version(entry: &ExtensionLockEntry) -> &str {
    if entry.install_type == ExtensionInstallType::Linked {
        entry
            .tool_version
            .as_deref()
            .unwrap_or(&entry.current_version)
    } else {
        &entry.current_version
    }
}

fn source_for(entry: &ExtensionLockEntry) -> SyncSource {
    match entry.install_type {
        ExtensionInstallType::Managed => SyncSource::Managed,
        ExtensionInstallType::Linked => SyncSource::Linked,
    }
}

fn atomic_write(path: &Path, bytes: &[u8], label: &str) -> Result<(), String> {
    let parent = path
        .parent()
        .ok_or_else(|| format!("Invalid {label} path"))?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create {label} directory: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create temporary {label}: {error}"))?;
    temporary
        .write_all(bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write {label}: {error}"))?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| format!("Cannot persist {label}: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::lock::ExtensionStateKind;

    fn lock_entry(install_type: ExtensionInstallType, version: &str) -> ExtensionLockEntry {
        ExtensionLockEntry {
            id: "example.tools".into(),
            name: "Example Tools".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            install_type,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: (install_type == ExtensionInstallType::Managed)
                .then(|| "@example/floter-tools".into()),
            package_version: version.into(),
            tool_version: (install_type == ExtensionInstallType::Linked).then(|| version.into()),
            integrity: None,
            signature_verified: false,
            previous_signature_verified: None,
            current_version: version.into(),
            previous_version: None,
            manifest_path: String::new(),
            executable_path: String::new(),
            runtime_root: None,
            installed_at: 1,
            updated_at: 1,
            pinned: false,
            channel: "latest".into(),
        }
    }

    fn sync_entry(source: SyncSource, version: &str) -> ExtensionsSyncEntry {
        ExtensionsSyncEntry {
            id: "example.tools".into(),
            version: version.into(),
            enabled: true,
            config: BTreeMap::new(),
            source,
            package: (source == SyncSource::Managed).then(|| "@example/floter-tools".into()),
            manifest: None,
        }
    }

    #[test]
    fn serializes_the_v1_export_shape() {
        let document = ExtensionsSyncDocument {
            version: 1,
            exported_at: "2026-08-10T12:00:00Z".into(),
            extensions: vec![sync_entry(SyncSource::Managed, "1.2.3")],
        };

        let value = serde_json::to_value(document).unwrap();

        assert_eq!(value["version"], 1);
        assert_eq!(value["exportedAt"], "2026-08-10T12:00:00Z");
        assert_eq!(value["extensions"][0]["id"], "example.tools");
        assert_eq!(value["extensions"][0]["version"], "1.2.3");
        assert_eq!(value["extensions"][0]["enabled"], true);
        assert_eq!(value["extensions"][0]["config"], serde_json::json!({}));
        assert_eq!(value["extensions"][0]["source"], "managed");
    }

    #[test]
    fn rejects_invalid_and_unsupported_imports() {
        assert!(parse_import(b"not json").is_err());
        let unsupported = br#"{
          "version": 2,
          "exportedAt": "2026-08-10T12:00:00Z",
          "extensions": []
        }"#;
        assert!(parse_import(unsupported)
            .unwrap_err()
            .contains("Unsupported extension export version 2"));
    }

    #[test]
    fn rejects_imports_with_too_many_extensions() {
        let document = ExtensionsSyncDocument {
            version: SYNC_FORMAT_VERSION,
            exported_at: "2026-08-10T00:00:00Z".into(),
            extensions: (0..=MAX_IMPORT_EXTENSIONS)
                .map(|index| ExtensionsSyncEntry {
                    id: format!("io.example.tool-{index}"),
                    version: "1.0.0".into(),
                    enabled: true,
                    config: BTreeMap::new(),
                    source: SyncSource::Managed,
                    package: Some(format!("floter-tool-{index}")),
                    manifest: None,
                })
                .collect(),
        };

        assert!(validate_import(&document)
            .unwrap_err()
            .contains("more than 1000 extensions"));
    }

    #[test]
    fn repeated_import_skips_an_identical_installation() {
        let installed = lock_entry(ExtensionInstallType::Managed, "1.2.3");
        let desired = sync_entry(SyncSource::Managed, "1.2.3");

        assert_eq!(
            reconcile_action(Some(&installed), &desired).unwrap(),
            ReconcileAction::Ready
        );
    }

    #[test]
    fn managed_version_mismatch_updates_or_rolls_back() {
        let mut installed = lock_entry(ExtensionInstallType::Managed, "2.0.0");
        let desired = sync_entry(SyncSource::Managed, "1.5.0");
        assert_eq!(
            reconcile_action(Some(&installed), &desired).unwrap(),
            ReconcileAction::Update
        );

        installed.previous_version = Some("1.5.0".into());
        assert_eq!(
            reconcile_action(Some(&installed), &desired).unwrap(),
            ReconcileAction::Rollback
        );
    }

    #[test]
    fn linked_version_mismatch_is_reported() {
        let installed = lock_entry(ExtensionInstallType::Linked, "2.0.0");
        let desired = sync_entry(SyncSource::Linked, "1.5.0");

        assert!(reconcile_action(Some(&installed), &desired)
            .unwrap_err()
            .contains("Linked executable"));
    }

    #[test]
    fn uses_the_requested_default_export_name() {
        assert_eq!(
            default_export_file_name(NaiveDate::from_ymd_opt(2026, 8, 10).unwrap()),
            "floter-extensions-2026-08-10.json"
        );
    }

    #[test]
    fn serialized_linked_manifest_remains_schema_valid() {
        let manifest = ExtensionManifest::parse(include_bytes!(
            "../../../extensions/v-tools/floter.extension.json"
        ))
        .unwrap();

        let serialized = serde_json::to_vec(&manifest).unwrap();

        ExtensionManifest::parse(&serialized).unwrap();
    }
}
