use crate::extensions::config;
use crate::extensions::install::{self, ExtensionInstallRequest, InstallSource};
use crate::extensions::lock::{
    validate_id, ExtensionDistributionSource, ExtensionLockEntry, ExtensionRuntimeOwnership,
    ExtensionsLock,
};
use crate::extensions::manifest::{ExtensionManifest, Permission, Runtime};
use crate::extensions::ExtensionState;
use chrono::{DateTime, NaiveDate, SecondsFormat, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashSet};
use std::io::Write;
use std::path::{Path, PathBuf};

const SYNC_FORMAT_VERSION: u32 = 2;
const MAX_IMPORT_BYTES: u64 = 4 * 1024 * 1024;
const MAX_IMPORT_EXTENSIONS: usize = 1_000;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ExtensionsSyncDocument {
    pub version: u32,
    pub exported_at: String,
    pub extensions: Vec<ExtensionsSyncEntry>,
    #[serde(skip)]
    legacy_version_semantics: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ExtensionsSyncEntry {
    pub id: String,
    pub version: String,
    pub enabled: bool,
    #[serde(default)]
    pub config: BTreeMap<String, Value>,
    pub distribution_source: ExtensionDistributionSource,
    pub runtime_ownership: ExtensionRuntimeOwnership,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub package: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub manifest: Option<ExtensionManifest>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub script_content: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider_descriptor: Option<Value>,
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
        legacy_version_semantics: false,
    })
}

fn export_entry(
    state: &ExtensionState,
    entry: ExtensionLockEntry,
) -> Result<ExtensionsSyncEntry, String> {
    let manifest = if entry.distribution_source != ExtensionDistributionSource::Npm {
        Some(linked_manifest(state, &entry)?)
    } else {
        None
    };
    let script_content = match manifest.as_ref().map(|manifest| &manifest.runtime) {
        Some(Runtime::Script { path, .. }) => {
            let root = Path::new(&entry.manifest_path)
                .parent()
                .ok_or_else(|| format!("Script integration {} has no package root", entry.id))?;
            Some(std::fs::read_to_string(root.join(path)).map_err(|error| {
                format!("Cannot export script for integration {}: {error}", entry.id)
            })?)
        }
        _ => None,
    };
    let provider_descriptor = match manifest
        .as_ref()
        .and_then(|manifest| manifest.provider.descriptor.as_deref())
    {
        Some(descriptor) => {
            let root = Path::new(&entry.manifest_path)
                .parent()
                .ok_or_else(|| format!("Static integration {} has no package root", entry.id))?;
            Some(
                serde_json::from_slice(&std::fs::read(root.join(descriptor)).map_err(|error| {
                    format!(
                        "Cannot export descriptor for integration {}: {error}",
                        entry.id
                    )
                })?)
                .map_err(|error| {
                    format!(
                        "Cannot parse descriptor for integration {}: {error}",
                        entry.id
                    )
                })?,
            )
        }
        None => None,
    };
    Ok(ExtensionsSyncEntry {
        id: entry.id.clone(),
        version: installed_version(&entry).to_string(),
        enabled: entry.enabled,
        config: config::export_values(&state.paths.data, &entry.id)?,
        distribution_source: entry.distribution_source,
        runtime_ownership: entry.runtime_ownership,
        package: entry.package_name,
        manifest,
        script_content,
        provider_descriptor,
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
                    "Cannot export local integration {} because its manifest is unavailable: {file_error}",
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
    let mut value: Value = serde_json::from_slice(bytes)
        .map_err(|error| format!("Invalid extension import JSON: {error}"))?;
    let legacy_version_semantics = value.get("version").and_then(Value::as_u64) == Some(1);
    migrate_v1_import(&mut value)?;
    let mut document: ExtensionsSyncDocument = serde_json::from_value(value)
        .map_err(|error| format!("Invalid extension import: {error}"))?;
    document.legacy_version_semantics = legacy_version_semantics;
    validate_import(&document)?;
    Ok(document)
}

fn migrate_v1_import(value: &mut Value) -> Result<(), String> {
    if value.get("version").and_then(Value::as_u64) != Some(1) {
        return Ok(());
    }
    let extensions = value
        .get_mut("extensions")
        .and_then(Value::as_array_mut)
        .ok_or_else(|| "Extension import extensions must be an array".to_string())?;
    for entry in extensions {
        let object = entry
            .as_object_mut()
            .ok_or_else(|| "Extension import entry must be an object".to_string())?;
        let source = object
            .remove("source")
            .and_then(|value| value.as_str().map(str::to_string))
            .ok_or_else(|| "Extension import entry has no source".to_string())?;
        let (distribution, runtime) = match source.as_str() {
            "managed" => ("npm", "bundled"),
            "linked" => ("local", "system"),
            other => return Err(format!("Unsupported legacy sync source: {other}")),
        };
        object.insert(
            "distributionSource".to_string(),
            Value::String(distribution.to_string()),
        );
        object.insert(
            "runtimeOwnership".to_string(),
            Value::String(runtime.to_string()),
        );
    }
    value["version"] = Value::from(SYNC_FORMAT_VERSION);
    Ok(())
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
            if entry.distribution_source == ExtensionDistributionSource::Npm
                || manifest.id != entry.id
            {
                return Err(format!(
                    "Extension {} has a mismatched local manifest",
                    entry.id
                ));
            }
            if !matches!(
                manifest.runtime,
                Runtime::System { .. } | Runtime::Script { .. }
            ) {
                return Err(format!(
                    "Extension {} has an unsupported local manifest",
                    entry.id
                ));
            }
            if matches!(manifest.runtime, Runtime::Script { .. }) && entry.script_content.is_none()
            {
                return Err(format!(
                    "Script integration {} has no script content",
                    entry.id
                ));
            }
            if manifest.provider.descriptor.is_some() && entry.provider_descriptor.is_none() {
                return Err(format!(
                    "Static integration {} has no provider descriptor",
                    entry.id
                ));
            }
            let bytes = serde_json::to_vec(manifest)
                .map_err(|error| format!("Cannot validate local manifest: {error}"))?;
            ExtensionManifest::parse(&bytes)
                .map_err(|error| format!("Invalid local manifest for {}: {error}", entry.id))?;
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
    let legacy_version_semantics = document.legacy_version_semantics;
    for mut entry in document.extensions {
        if entry.distribution_source == ExtensionDistributionSource::Local
            && state
                .static_adapters
                .iter()
                .any(|adapter| adapter.manifest.id == entry.id)
        {
            entry.distribution_source = ExtensionDistributionSource::BuiltIn;
        }
        let id = entry.id.clone();
        match import_entry(
            state,
            &entry,
            legacy_version_semantics,
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
    legacy_version_semantics: bool,
    approved_permissions: Option<&[Permission]>,
) -> Result<bool, String> {
    let installed = ExtensionsLock::load(&state.paths.lock_file)?
        .extensions
        .get(&desired.id)
        .cloned();
    let action = reconcile_action(installed.as_ref(), desired, legacy_version_semantics)?;
    let mut changed = action != ReconcileAction::Ready;

    match action {
        ReconcileAction::Install | ReconcileAction::Update => match desired.distribution_source {
            ExtensionDistributionSource::Npm => {
                let package = desired.package.as_deref().ok_or_else(|| {
                    format!(
                        "NPM integration {} has no package in the export",
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
            ExtensionDistributionSource::Local | ExtensionDistributionSource::BuiltIn => {
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
    if restored.distribution_source != desired.distribution_source
        || restored.runtime_ownership != desired.runtime_ownership
    {
        if installed.is_none() {
            let _ = install::uninstall(state, &desired.id, false).await;
        }
        return Err(format!(
            "Integration {} resolved with different distribution or runtime ownership",
            desired.id
        ));
    }
    let version_must_match = !legacy_version_semantics
        || desired.distribution_source == ExtensionDistributionSource::Npm;
    if version_must_match && installed_version(&restored) != desired.version {
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
    legacy_version_semantics: bool,
) -> Result<ReconcileAction, String> {
    let Some(installed) = installed else {
        return Ok(ReconcileAction::Install);
    };
    if installed.distribution_source != desired.distribution_source
        || installed.runtime_ownership != desired.runtime_ownership
    {
        return Err(format!(
            "Extension {} is installed from a different source",
            desired.id
        ));
    }
    if installed_version(installed) == desired.version
        || (legacy_version_semantics
            && desired.distribution_source != ExtensionDistributionSource::Npm)
    {
        return Ok(ReconcileAction::Ready);
    }
    match desired.distribution_source {
        ExtensionDistributionSource::Npm => {
            let package = desired.package.as_deref().ok_or_else(|| {
                format!(
                    "NPM integration {} has no package in the export",
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
        ExtensionDistributionSource::Local | ExtensionDistributionSource::BuiltIn => Err(format!(
            "Local integration {} is version {}, but the export requires {}",
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
    if state
        .static_adapters
        .iter()
        .any(|adapter| adapter.manifest.id == desired.id)
    {
        install::connect_bundled(state, &desired.id, None, approved_permissions).await?;
        return Ok(());
    }
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
        .ok_or_else(|| format!("Local integration {} has no manifest", desired.id))?;
    let path = imported_manifest_path(&state.paths.data, &desired.id)?;
    let mut bytes = serde_json::to_vec_pretty(&manifest)
        .map_err(|error| format!("Cannot serialize local manifest: {error}"))?;
    bytes.push(b'\n');
    atomic_write(&path, &bytes, "local integration manifest")?;
    if let Some(descriptor_path) = manifest.provider.descriptor.as_deref() {
        let target = path
            .parent()
            .ok_or("Imported manifest has no parent directory")?
            .join(descriptor_path);
        let mut bytes = serde_json::to_vec_pretty(
            desired
                .provider_descriptor
                .as_ref()
                .ok_or("Imported provider descriptor is missing")?,
        )
        .map_err(|error| format!("Cannot serialize imported provider descriptor: {error}"))?;
        bytes.push(b'\n');
        atomic_write(&target, &bytes, "local provider descriptor")?;
    }
    if let Runtime::Script {
        path: script_path, ..
    } = &manifest.runtime
    {
        let target = path
            .parent()
            .ok_or("Imported manifest has no parent directory")?
            .join(script_path);
        atomic_write(
            &target,
            desired
                .script_content
                .as_deref()
                .ok_or("Imported script content is missing")?
                .as_bytes(),
            "local provider script",
        )?;
        install::make_executable(&target)?;
    }
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
    &entry.current_version
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
    use crate::extensions::install::{create_custom_integration, CustomIntegrationRequest};
    use crate::extensions::lock::{ExtensionProviderKind, ExtensionStateKind};
    use crate::extensions::manifest::{PlatformTarget, ScriptLanguage};
    use crate::extensions::ExtensionPaths;

    fn lock_entry(
        distribution_source: ExtensionDistributionSource,
        runtime_ownership: ExtensionRuntimeOwnership,
        version: &str,
    ) -> ExtensionLockEntry {
        ExtensionLockEntry {
            id: "example.tools".into(),
            name: "Example Tools".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source,
            runtime_ownership,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: (distribution_source == ExtensionDistributionSource::Npm)
                .then(|| "@example/floter-tools".into()),
            package_version: version.into(),
            tool_version: (runtime_ownership == ExtensionRuntimeOwnership::System)
                .then(|| version.into()),
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

    fn sync_entry(
        distribution_source: ExtensionDistributionSource,
        runtime_ownership: ExtensionRuntimeOwnership,
        version: &str,
    ) -> ExtensionsSyncEntry {
        ExtensionsSyncEntry {
            id: "example.tools".into(),
            version: version.into(),
            enabled: true,
            config: BTreeMap::new(),
            distribution_source,
            runtime_ownership,
            package: (distribution_source == ExtensionDistributionSource::Npm)
                .then(|| "@example/floter-tools".into()),
            manifest: None,
            script_content: None,
            provider_descriptor: None,
        }
    }

    #[tokio::test]
    async fn export_carries_generated_script_and_static_descriptor() {
        if install::find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().to_path_buf()))
                .unwrap();
        create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.export-test".into(),
                name: "Export test".into(),
                command: "export-test".into(),
                version: "1.0.0".into(),
                executable_path: String::new(),
                mode: "script".into(),
                script_language: Some(ScriptLanguage::Shell),
                script_content: Some("printf '%s\\n' \"$@\"".into()),
                args_prefix: vec!["default".into()],
                version_args: Vec::new(),
                permissions: vec![Permission::Environment],
                platforms: vec![PlatformTarget::current().unwrap().os],
            },
        )
        .await
        .unwrap();

        let document = build_export(&state, Utc::now()).unwrap();
        let exported = &document.extensions[0];
        assert_eq!(
            exported.script_content.as_deref(),
            Some("printf '%s\\n' \"$@\"")
        );
        assert_eq!(
            exported.provider_descriptor.as_ref().unwrap()["commands"][0]["id"],
            "export-test"
        );
        assert!(matches!(
            exported.manifest.as_ref().unwrap().runtime,
            Runtime::Script { .. }
        ));
    }

    #[test]
    fn serializes_the_v2_export_shape() {
        let document = ExtensionsSyncDocument {
            version: 2,
            exported_at: "2026-08-10T12:00:00Z".into(),
            extensions: vec![sync_entry(
                ExtensionDistributionSource::Npm,
                ExtensionRuntimeOwnership::Bundled,
                "1.2.3",
            )],
            legacy_version_semantics: false,
        };

        let value = serde_json::to_value(document).unwrap();

        assert_eq!(value["version"], 2);
        assert_eq!(value["exportedAt"], "2026-08-10T12:00:00Z");
        assert_eq!(value["extensions"][0]["id"], "example.tools");
        assert_eq!(value["extensions"][0]["version"], "1.2.3");
        assert_eq!(value["extensions"][0]["enabled"], true);
        assert_eq!(value["extensions"][0]["config"], serde_json::json!({}));
        assert_eq!(value["extensions"][0]["distributionSource"], "npm");
        assert_eq!(value["extensions"][0]["runtimeOwnership"], "bundled");
    }

    #[test]
    fn rejects_invalid_and_unsupported_imports() {
        assert!(parse_import(b"not json").is_err());
        let unsupported = br#"{
          "version": 3,
          "exportedAt": "2026-08-10T12:00:00Z",
          "extensions": []
        }"#;
        assert!(parse_import(unsupported)
            .unwrap_err()
            .contains("Unsupported extension export version 3"));
    }

    #[test]
    fn migrates_v1_exports() {
        let legacy = br#"{
          "version": 1,
          "exportedAt": "2026-08-10T12:00:00Z",
          "extensions": [{
            "id": "example.tools",
            "version": "1.2.3",
            "enabled": true,
            "config": {},
            "source": "managed",
            "package": "@example/floter-tools"
          }]
        }"#;
        let document = parse_import(legacy).unwrap();
        let entry = &document.extensions[0];

        assert_eq!(document.version, 2);
        assert_eq!(entry.distribution_source, ExtensionDistributionSource::Npm);
        assert_eq!(entry.runtime_ownership, ExtensionRuntimeOwnership::Bundled);
    }

    #[test]
    fn legacy_local_exports_keep_their_tool_version_semantics() {
        let legacy = br#"{
          "version": 1,
          "exportedAt": "2026-08-10T12:00:00Z",
          "extensions": [{
            "id": "example.tools",
            "version": "9.8.7",
            "enabled": true,
            "config": {},
            "source": "linked"
          }]
        }"#;
        let document = parse_import(legacy).unwrap();
        let installed = lock_entry(
            ExtensionDistributionSource::Local,
            ExtensionRuntimeOwnership::System,
            "1.2.3",
        );

        assert!(document.legacy_version_semantics);
        assert_eq!(
            reconcile_action(Some(&installed), &document.extensions[0], true).unwrap(),
            ReconcileAction::Ready
        );
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
                    distribution_source: ExtensionDistributionSource::Npm,
                    runtime_ownership: ExtensionRuntimeOwnership::Bundled,
                    package: Some(format!("floter-tool-{index}")),
                    manifest: None,
                    script_content: None,
                    provider_descriptor: None,
                })
                .collect(),
            legacy_version_semantics: false,
        };

        assert!(validate_import(&document)
            .unwrap_err()
            .contains("more than 1000 extensions"));
    }

    #[test]
    fn repeated_import_skips_an_identical_installation() {
        let installed = lock_entry(
            ExtensionDistributionSource::Npm,
            ExtensionRuntimeOwnership::Bundled,
            "1.2.3",
        );
        let desired = sync_entry(
            ExtensionDistributionSource::Npm,
            ExtensionRuntimeOwnership::Bundled,
            "1.2.3",
        );

        assert_eq!(
            reconcile_action(Some(&installed), &desired, false).unwrap(),
            ReconcileAction::Ready
        );
    }

    #[test]
    fn managed_version_mismatch_updates_or_rolls_back() {
        let mut installed = lock_entry(
            ExtensionDistributionSource::Npm,
            ExtensionRuntimeOwnership::Bundled,
            "2.0.0",
        );
        let desired = sync_entry(
            ExtensionDistributionSource::Npm,
            ExtensionRuntimeOwnership::Bundled,
            "1.5.0",
        );
        assert_eq!(
            reconcile_action(Some(&installed), &desired, false).unwrap(),
            ReconcileAction::Update
        );

        installed.previous_version = Some("1.5.0".into());
        assert_eq!(
            reconcile_action(Some(&installed), &desired, false).unwrap(),
            ReconcileAction::Rollback
        );
    }

    #[test]
    fn local_version_mismatch_is_reported() {
        let installed = lock_entry(
            ExtensionDistributionSource::Local,
            ExtensionRuntimeOwnership::System,
            "2.0.0",
        );
        let desired = sync_entry(
            ExtensionDistributionSource::Local,
            ExtensionRuntimeOwnership::System,
            "1.5.0",
        );

        assert!(reconcile_action(Some(&installed), &desired, false)
            .unwrap_err()
            .contains("Local integration"));
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
