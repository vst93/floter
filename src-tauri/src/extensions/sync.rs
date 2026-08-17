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

#[cfg(test)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ImportFailurePoint {
    BeforeCommit,
    BeforeConfig,
}

#[cfg(test)]
static IMPORT_FAILURES: std::sync::Mutex<Vec<(String, ImportFailurePoint)>> =
    std::sync::Mutex::new(Vec::new());

#[cfg(test)]
fn inject_import_failure(extension_id: &str, point: ImportFailurePoint) {
    IMPORT_FAILURES
        .lock()
        .unwrap()
        .push((extension_id.to_string(), point));
}

fn maybe_fail_import(extension_id: &str, point: &str) -> Result<(), String> {
    #[cfg(test)]
    {
        let expected = match point {
            "commit" => ImportFailurePoint::BeforeCommit,
            "config" => ImportFailurePoint::BeforeConfig,
            _ => return Ok(()),
        };
        let mut failures = IMPORT_FAILURES.lock().unwrap();
        if let Some(index) = failures
            .iter()
            .position(|(id, candidate)| id == extension_id && *candidate == expected)
        {
            failures.remove(index);
            return Err(format!("Injected import {point} failure"));
        }
    }
    #[cfg(not(test))]
    let _ = (extension_id, point);
    Ok(())
}

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
    let _guard = state.mutation_lock.lock().await;
    let legacy_version_semantics = document.legacy_version_semantics;
    let installed = match ExtensionsLock::load(&state.paths.lock_file) {
        Ok(lock) => lock,
        Err(message) => {
            report.failed.push(ExtensionsImportItem {
                id: "import".to_string(),
                message,
            });
            return report;
        }
    };
    let mut plan = Vec::new();
    for mut entry in document.extensions {
        if entry.distribution_source == ExtensionDistributionSource::Local
            && state
                .static_adapters
                .iter()
                .any(|adapter| adapter.manifest.id == entry.id)
        {
            entry.distribution_source = ExtensionDistributionSource::BuiltIn;
        }
        match reconcile_action(
            installed.extensions.get(&entry.id),
            &entry,
            legacy_version_semantics,
        ) {
            Ok(action) => plan.push((entry, action)),
            Err(message) => report.failed.push(ExtensionsImportItem {
                id: entry.id,
                message,
            }),
        }
    }
    if !report.failed.is_empty() {
        return report;
    }

    let mut prepared = Vec::new();
    for (entry, action) in plan {
        match preflight_entry(
            state,
            &entry,
            action,
            approved_permissions.get(&entry.id).map(Vec::as_slice),
        )
        .await
        {
            Ok(artifact) => prepared.push(PreparedImport {
                desired: entry,
                action,
                artifact,
            }),
            Err(message) => {
                report.failed.push(ExtensionsImportItem {
                    id: entry.id,
                    message: format!("Preflight failed: {message}"),
                });
                return report;
            }
        }
    }

    let snapshot = match ImportSnapshot::capture(
        state,
        prepared.iter().map(|item| item.desired.id.as_str()),
    ) {
        Ok(snapshot) => snapshot,
        Err(message) => {
            report.failed.push(ExtensionsImportItem {
                id: "import".to_string(),
                message,
            });
            return report;
        }
    };
    for item in &prepared {
        let result = match maybe_fail_import(&item.desired.id, "commit") {
            Ok(()) => commit_prepared_entry(state, item).await,
            Err(error) => Err(error),
        };
        match result {
            Ok(true) => report.succeeded.push(ExtensionsImportItem {
                id: item.desired.id.clone(),
                message: "Installed or restored".to_string(),
            }),
            Ok(false) => report.skipped.push(ExtensionsImportItem {
                id: item.desired.id.clone(),
                message: "Already matches the import".to_string(),
            }),
            Err(message) => {
                let rollback = crate::extensions::transaction::recover(state)
                    .and_then(|()| snapshot.restore(state));
                report.succeeded.clear();
                report.skipped.clear();
                report.failed.push(ExtensionsImportItem {
                    id: item.desired.id.clone(),
                    message: match rollback {
                        Ok(()) => format!("Import failed and was rolled back: {message}"),
                        Err(rollback_error) => format!(
                            "Import failed: {message}; rollback also failed: {rollback_error}"
                        ),
                    },
                });
                return report;
            }
        }
    }
    report
}

struct PreparedImport {
    desired: ExtensionsSyncEntry,
    action: ReconcileAction,
    artifact: Option<PreparedArtifact>,
}

struct PreparedArtifact {
    _root: tempfile::TempDir,
    state: ExtensionState,
    entry: ExtensionLockEntry,
}

async fn preflight_entry(
    state: &ExtensionState,
    desired: &ExtensionsSyncEntry,
    action: ReconcileAction,
    approved_permissions: Option<&[Permission]>,
) -> Result<Option<PreparedArtifact>, String> {
    if action == ReconcileAction::Ready {
        config::preflight_import_values(state, &desired.id, &desired.config).await?;
        return Ok(None);
    }
    let root = tempfile::tempdir_in(&state.paths.cache)
        .map_err(|error| format!("Cannot create import preflight directory: {error}"))?;
    let prepared_state = ExtensionState::from_paths_with_official_index(
        crate::extensions::ExtensionPaths::from_root(root.path().join("state")),
        state.official_index.clone(),
    )?;
    let entry = match desired.distribution_source {
        ExtensionDistributionSource::Npm => {
            let package = desired.package.as_deref().ok_or_else(|| {
                format!(
                    "NPM integration {} has no package in the export",
                    desired.id
                )
            })?;
            install::install_imported_managed_locked(
                &prepared_state,
                &desired.id,
                package,
                &desired.version,
                approved_permissions,
            )
            .await?
        }
        ExtensionDistributionSource::Local | ExtensionDistributionSource::BuiltIn => {
            install_imported_linked(&prepared_state, desired, approved_permissions).await?
        }
    };
    if entry.distribution_source != desired.distribution_source
        || entry.runtime_ownership != desired.runtime_ownership
    {
        return Err(format!(
            "Integration {} resolved with different distribution or runtime ownership",
            desired.id
        ));
    }
    if installed_version(&entry) != desired.version {
        return Err(format!(
            "Extension {} resolved to version {}, but the export requires {}",
            desired.id,
            installed_version(&entry),
            desired.version
        ));
    }
    config::preflight_import_values(&prepared_state, &desired.id, &desired.config).await?;
    Ok(Some(PreparedArtifact {
        _root: root,
        state: prepared_state,
        entry,
    }))
}

async fn commit_prepared_entry(
    state: &ExtensionState,
    prepared: &PreparedImport,
) -> Result<bool, String> {
    let mut changed = prepared.action != ReconcileAction::Ready;
    if let Some(artifact) = &prepared.artifact {
        match prepared.desired.distribution_source {
            ExtensionDistributionSource::Npm => {
                install::commit_preflight_managed(state, &artifact.state, artifact.entry.clone())?;
            }
            ExtensionDistributionSource::Local => {
                commit_preflight_linked(state, &artifact.state, &prepared.desired).await?;
            }
            ExtensionDistributionSource::BuiltIn => {
                install::connect_bundled_locked(
                    state,
                    &prepared.desired.id,
                    None,
                    Some(&artifact.entry_permissions()?),
                )
                .await?;
            }
        }
    }
    if !prepared.desired.config.is_empty() {
        maybe_fail_import(&prepared.desired.id, "config")?;
        changed |=
            config::import_values_locked(state, &prepared.desired.id, &prepared.desired.config)
                .await?;
    }
    changed |= restore_enabled_locked(state, &prepared.desired.id, prepared.desired.enabled)?;
    Ok(changed)
}

impl PreparedArtifact {
    fn entry_permissions(&self) -> Result<Vec<Permission>, String> {
        Ok(ExtensionManifest::load(Path::new(&self.entry.manifest_path))?.permissions)
    }
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
) -> Result<ExtensionLockEntry, String> {
    if state
        .static_adapters
        .iter()
        .any(|adapter| adapter.manifest.id == desired.id)
    {
        return install::connect_bundled(state, &desired.id, None, approved_permissions).await;
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
    install::install_linked(
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
    .await
}

async fn commit_preflight_linked(
    state: &ExtensionState,
    staged_state: &ExtensionState,
    desired: &ExtensionsSyncEntry,
) -> Result<ExtensionLockEntry, String> {
    let staged_sync = staged_state.paths.data.join(&desired.id).join("sync");
    let data_root = state.paths.data.join(&desired.id);
    std::fs::create_dir_all(&data_root)
        .map_err(|error| format!("Cannot create imported integration data directory: {error}"))?;
    let target = data_root.join("sync");
    let temporary = data_root.join(format!(".sync-import-{}", uuid::Uuid::new_v4()));
    copy_directory(&staged_sync, &temporary)?;
    if target.exists() {
        std::fs::remove_dir_all(&target)
            .map_err(|error| format!("Cannot replace imported integration files: {error}"))?;
    }
    std::fs::rename(&temporary, &target)
        .map_err(|error| format!("Cannot commit imported integration files: {error}"))?;
    crate::extensions::lock::sync_directory(&data_root)
        .map_err(|error| format!("Cannot sync imported integration directory: {error}"))?;
    install::install_linked(
        state,
        ExtensionInstallRequest {
            source: InstallSource::Linked,
            package: None,
            version: None,
            manifest_path: Some(
                target
                    .join("floter.extension.json")
                    .to_string_lossy()
                    .into_owned(),
            ),
            executable_path: None,
            approved_permissions: desired
                .manifest
                .as_ref()
                .map(|manifest| manifest.permissions.clone()),
        },
    )
    .await
}

fn imported_manifest_path(data_root: &Path, extension_id: &str) -> Result<PathBuf, String> {
    validate_id(extension_id)?;
    Ok(data_root
        .join(extension_id)
        .join("sync")
        .join("floter.extension.json"))
}

fn restore_enabled_locked(
    state: &ExtensionState,
    extension_id: &str,
    enabled: bool,
) -> Result<bool, String> {
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

struct ImportSnapshot {
    _root: tempfile::TempDir,
    lock: ExtensionsLock,
    items: Vec<ImportSnapshotItem>,
}

struct ImportSnapshotItem {
    id: String,
    extension: Option<PathBuf>,
    data: Option<PathBuf>,
}

impl ImportSnapshot {
    fn capture<'a>(
        state: &ExtensionState,
        ids: impl Iterator<Item = &'a str>,
    ) -> Result<Self, String> {
        let root = tempfile::tempdir_in(&state.paths.cache)
            .map_err(|error| format!("Cannot create import rollback snapshot: {error}"))?;
        let lock = ExtensionsLock::load(&state.paths.lock_file)?;
        let mut items = Vec::new();
        for id in ids {
            let item_root = root.path().join(id);
            let extension_source = state.paths.extensions.join(id);
            let extension = extension_source
                .exists()
                .then(|| item_root.join("extension"));
            if let Some(target) = &extension {
                copy_directory(&extension_source, target)?;
            }
            let data_source = state.paths.data.join(id);
            let data = data_source.exists().then(|| item_root.join("data"));
            if let Some(target) = &data {
                copy_directory(&data_source, target)?;
            }
            items.push(ImportSnapshotItem {
                id: id.to_string(),
                extension,
                data,
            });
        }
        Ok(Self {
            _root: root,
            lock,
            items,
        })
    }

    fn restore(&self, state: &ExtensionState) -> Result<(), String> {
        for item in self.items.iter().rev() {
            restore_directory(
                &state.paths.extensions.join(&item.id),
                item.extension.as_deref(),
            )?;
            restore_directory(&state.paths.data.join(&item.id), item.data.as_deref())?;
        }
        self.lock.save(&state.paths.lock_file)?;
        for entry in self.lock.extensions.values() {
            crate::extensions::lock::write_current_pointer(&state.paths.extensions, entry)?;
        }
        Ok(())
    }
}

fn restore_directory(target: &Path, snapshot: Option<&Path>) -> Result<(), String> {
    if target.exists() {
        std::fs::remove_dir_all(target).map_err(|error| {
            format!(
                "Cannot clear {} during import rollback: {error}",
                target.display()
            )
        })?;
    }
    if let Some(snapshot) = snapshot {
        copy_directory(snapshot, target)?;
    }
    Ok(())
}

fn copy_directory(source: &Path, target: &Path) -> Result<(), String> {
    std::fs::create_dir_all(target)
        .map_err(|error| format!("Cannot create {}: {error}", target.display()))?;
    for entry in std::fs::read_dir(source)
        .map_err(|error| format!("Cannot read {}: {error}", source.display()))?
    {
        let entry = entry.map_err(|error| format!("Cannot read directory entry: {error}"))?;
        let destination = target.join(entry.file_name());
        if entry
            .file_type()
            .map_err(|error| format!("Cannot inspect {}: {error}", entry.path().display()))?
            .is_dir()
        {
            copy_directory(&entry.path(), &destination)?;
        } else {
            std::fs::copy(entry.path(), &destination).map_err(|error| {
                format!(
                    "Cannot copy {} to {}: {error}",
                    entry.path().display(),
                    destination.display()
                )
            })?;
        }
    }
    Ok(())
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
    use crate::extensions::manifest::{
        Compatibility, Distribution, PlatformTarget, ProviderConfig, ProviderKind, Publisher,
        ScriptLanguage,
    };
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

    fn portable_script_entry(id: &str, endpoint: &str) -> ExtensionsSyncEntry {
        let description = serde_json::json!({
            "protocolVersion": "1.0",
            "provider": {
                "id": id,
                "name": id,
                "version": "1.0.0",
                "description": "Import transaction fixture"
            },
            "commands": [{
                "id": "run",
                "name": "Run",
                "description": "Run the fixture",
                "execution": {
                    "program": "self",
                    "argsPrefix": [],
                    "mode": "pty",
                    "workingDirectory": "current"
                },
                "arguments": []
            }]
        });
        let config = serde_json::json!({
            "configuration": {
                "configVersion": 1,
                "owner": "host",
                "schema": [{
                    "key": "endpoint",
                    "type": "text",
                    "required": true
                }]
            }
        });
        let script = format!(
            "if [ \"$1\" = config ]; then printf '%s' '{}'; else printf '%s' '{}'; fi",
            config.to_string().replace('\'', "'\\''"),
            description.to_string().replace('\'', "'\\''"),
        );
        ExtensionsSyncEntry {
            id: id.into(),
            version: "1.0.0".into(),
            enabled: true,
            config: BTreeMap::from([("endpoint".into(), Value::String(endpoint.into()))]),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            package: None,
            manifest: Some(ExtensionManifest {
                schema_version: "2.0".into(),
                id: id.into(),
                name: id.into(),
                description: "Import transaction fixture".into(),
                homepage: None,
                icon: None,
                publisher: Publisher {
                    id: "test".into(),
                    name: "Test".into(),
                },
                compatibility: Compatibility {
                    floter: format!(">={}", env!("CARGO_PKG_VERSION")),
                    provider_protocol: "^1.0".into(),
                },
                distribution: Distribution::Local,
                runtime: Runtime::Script {
                    language: ScriptLanguage::Shell,
                    path: "provider.sh".into(),
                    version_args: Vec::new(),
                },
                provider: ProviderConfig {
                    kind: ProviderKind::StaticDescriptor,
                    descriptor: Some("provider-description.json".into()),
                    args_prefix: Vec::new(),
                    describe_timeout_ms: 5_000,
                    complete_timeout_ms: 800,
                    environment: BTreeMap::new(),
                },
                platforms: vec![PlatformTarget::current().unwrap().os],
                signatures: None,
                platform_overrides: BTreeMap::new(),
                permissions: vec![Permission::ProcessSpawn],
                lifecycle: crate::extensions::lifecycle::ToolLifecycle::default(),
            }),
            script_content: Some(script),
            provider_descriptor: Some(description),
        }
    }

    fn import_document_with(entries: Vec<ExtensionsSyncEntry>) -> ExtensionsSyncDocument {
        ExtensionsSyncDocument {
            version: SYNC_FORMAT_VERSION,
            exported_at: "2026-08-13T00:00:00Z".into(),
            extensions: entries,
            legacy_version_semantics: false,
        }
    }

    fn fixture_approvals(entries: &[ExtensionsSyncEntry]) -> BTreeMap<String, Vec<Permission>> {
        entries
            .iter()
            .map(|entry| (entry.id.clone(), vec![Permission::ProcessSpawn]))
            .collect()
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

    #[tokio::test]
    async fn preflight_failure_does_not_modify_local_state() {
        if install::find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().to_path_buf()))
                .unwrap();
        let mut entry = portable_script_entry("local.preflight-test", "valid");
        entry.config = BTreeMap::from([("unknown".into(), Value::String("invalid".into()))]);
        let approvals = fixture_approvals(std::slice::from_ref(&entry));

        let report = import_document(
            &state,
            Path::new("fixture.json"),
            import_document_with(vec![entry]),
            &approvals,
        )
        .await;

        assert_eq!(report.failed.len(), 1);
        assert!(ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .is_empty());
        assert!(!state.paths.extensions.join("local.preflight-test").exists());
        assert!(!state.paths.data.join("local.preflight-test").exists());
    }

    #[tokio::test]
    async fn mid_import_failure_rolls_back_prior_extensions() {
        if install::find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().to_path_buf()))
                .unwrap();
        let entries = vec![
            portable_script_entry("local.transaction-a", "a"),
            portable_script_entry("local.transaction-b", "b"),
        ];
        let approvals = fixture_approvals(&entries);
        inject_import_failure("local.transaction-b", ImportFailurePoint::BeforeCommit);

        let report = import_document(
            &state,
            Path::new("fixture.json"),
            import_document_with(entries),
            &approvals,
        )
        .await;

        assert_eq!(report.failed.len(), 1);
        assert!(report.succeeded.is_empty());
        assert!(ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .is_empty());
        assert!(!state.paths.data.join("local.transaction-a").exists());
        assert!(!state.paths.data.join("local.transaction-b").exists());
    }

    #[tokio::test]
    async fn configuration_commit_failure_rolls_back_the_extension() {
        if install::find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().to_path_buf()))
                .unwrap();
        let entry = portable_script_entry("local.config-rollback", "new");
        let approvals = fixture_approvals(std::slice::from_ref(&entry));
        inject_import_failure("local.config-rollback", ImportFailurePoint::BeforeConfig);

        let report = import_document(
            &state,
            Path::new("fixture.json"),
            import_document_with(vec![entry]),
            &approvals,
        )
        .await;

        assert_eq!(report.failed.len(), 1);
        assert!(ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .is_empty());
        assert!(!state.paths.data.join("local.config-rollback").exists());
    }

    #[test]
    fn rollback_snapshot_restores_existing_lock_version_and_configuration() {
        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().to_path_buf()))
                .unwrap();
        let id = "example.tools";
        let version_root = state.paths.extensions.join(id).join("versions/1.0.0");
        std::fs::create_dir_all(&version_root).unwrap();
        std::fs::write(version_root.join("old-file"), b"old").unwrap();
        let data_root = state.paths.data.join(id);
        std::fs::create_dir_all(&data_root).unwrap();
        std::fs::write(data_root.join("config.json"), b"old-config").unwrap();
        let mut original_entry = lock_entry(
            ExtensionDistributionSource::Npm,
            ExtensionRuntimeOwnership::Bundled,
            "1.0.0",
        );
        original_entry.manifest_path = version_root
            .join("floter.extension.json")
            .to_string_lossy()
            .into_owned();
        original_entry.executable_path = version_root.join("tool").to_string_lossy().into_owned();
        let mut original_lock = ExtensionsLock::default();
        original_lock.extensions.insert(id.into(), original_entry);
        original_lock.save(&state.paths.lock_file).unwrap();
        let snapshot = ImportSnapshot::capture(&state, std::iter::once(id)).unwrap();

        std::fs::remove_dir_all(state.paths.extensions.join(id)).unwrap();
        let new_version = state.paths.extensions.join(id).join("versions/2.0.0");
        std::fs::create_dir_all(&new_version).unwrap();
        std::fs::write(new_version.join("new-file"), b"new").unwrap();
        std::fs::write(data_root.join("config.json"), b"new-config").unwrap();
        ExtensionsLock::default()
            .save(&state.paths.lock_file)
            .unwrap();

        snapshot.restore(&state).unwrap();

        let restored = ExtensionsLock::load(&state.paths.lock_file).unwrap();
        assert_eq!(restored.get(id).unwrap().current_version, "1.0.0");
        assert!(version_root.join("old-file").exists());
        assert!(!new_version.exists());
        assert_eq!(
            std::fs::read(data_root.join("config.json")).unwrap(),
            b"old-config"
        );
    }

    #[tokio::test]
    async fn successful_import_is_idempotent() {
        if install::find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().to_path_buf()))
                .unwrap();
        let entry = portable_script_entry("local.idempotent", "stable");
        let approvals = fixture_approvals(std::slice::from_ref(&entry));

        let first = import_document(
            &state,
            Path::new("fixture.json"),
            import_document_with(vec![entry.clone()]),
            &approvals,
        )
        .await;
        let lock_after_first = std::fs::read(&state.paths.lock_file).unwrap();
        let second = import_document(
            &state,
            Path::new("fixture.json"),
            import_document_with(vec![entry]),
            &approvals,
        )
        .await;

        assert_eq!(first.succeeded.len(), 1);
        assert_eq!(second.skipped.len(), 1);
        assert!(second.failed.is_empty());
        assert_eq!(
            std::fs::read(&state.paths.lock_file).unwrap(),
            lock_after_first
        );
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
