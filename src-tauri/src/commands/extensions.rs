use crate::commands::apps::{list_applications, ApplicationState};
use crate::extensions::catalog::{
    self, CatalogCompletionResponse, CatalogEntry, CatalogSearchRequest, CompletionRequest,
};
use crate::extensions::config::{self, ExtensionConfiguration};
use crate::extensions::install::{
    self, CustomIntegrationDefinition, CustomIntegrationRequest, ExtensionInstallRequest,
    ExtensionPermissionReview, ExtensionSearchResult, PathExecutable,
};
use crate::extensions::lock::{
    ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
    ExtensionRuntimeOwnership, ExtensionStateKind, ExtensionsLock,
};
use crate::extensions::health::HealthReport;
use crate::extensions::manifest::{ExtensionManifest, Permission, PlatformTarget};
use crate::extensions::provider::{DiagnoseCheck, DiagnoseResponse, ProviderResponse};
use crate::extensions::source_bundle::{self, SourceBundleExportRequest, SourceBundleExportResult};
use crate::extensions::source_inference::{self, SourceInferenceReport};
use crate::extensions::source_resolver::{self, SourceResolution, SourceResolveRequest};
use crate::extensions::sync::{self, ExtensionsExportResult, ExtensionsImportReport};
use crate::extensions::ExtensionState;
use chrono::{Local, Utc};
use serde::Serialize;
use serde_json::Value;
use std::collections::BTreeMap;
use std::path::Path;
use tauri::{AppHandle, State};
use tauri_plugin_dialog::{DialogExt, MessageDialogButtons};

#[tauri::command]
pub async fn extensions_infer_source(path: String) -> Result<SourceInferenceReport, String> {
    tauri::async_runtime::spawn_blocking(move || source_inference::infer(Path::new(&path)))
        .await
        .map_err(|error| format!("Source inference task failed: {error}"))?
}

#[tauri::command]
pub async fn extensions_export_source_bundle(
    request: SourceBundleExportRequest,
) -> Result<SourceBundleExportResult, String> {
    tauri::async_runtime::spawn_blocking(move || source_bundle::export(&request))
        .await
        .map_err(|error| format!("Source bundle export task failed: {error}"))?
}

#[tauri::command]
pub async fn extensions_resolve_source(
    state: State<'_, ExtensionState>,
    request: SourceResolveRequest,
) -> Result<SourceResolution, String> {
    source_resolver::resolve(&state, request).await
}

#[tauri::command]
pub async fn extensions_list(
    state: State<'_, ExtensionState>,
) -> Result<Vec<ExtensionListItem>, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let official_index = crate::extensions::official_index::fetch(&state).await.ok();
    let detected_adapters = crate::extensions::static_adapter::load_bundled()?;
    let mut items = lock
        .list()
        .into_iter()
        .map(|mut entry| {
            entry.official_verified = entry.signature_verified
                && official_index.as_ref().is_some_and(|index| {
                    entry.package_name.as_deref().is_some_and(|package| {
                        ExtensionManifest::load(Path::new(&entry.manifest_path))
                            .ok()
                            .is_some_and(|manifest| {
                                index.authorizes(
                                    &entry.id,
                                    package,
                                    &entry.publisher_id,
                                    manifest.signatures.as_ref(),
                                )
                            })
                    })
                });
            ExtensionListItem::installed(entry)
        })
        .collect::<Vec<_>>();
    for adapter in &detected_adapters {
        if lock.extensions.contains_key(&adapter.manifest.id) {
            continue;
        }
        items.push(ExtensionListItem::detected(adapter));
    }
    Ok(items)
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionListItem {
    #[serde(flatten)]
    pub entry: ExtensionLockEntry,
    pub connected: bool,
    pub runtime_source: String,
    pub runtime_available: bool,
    pub reconnect_available: bool,
    pub homepage: Option<String>,
    pub generated_custom: bool,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalManifestReview {
    pub manifest_path: String,
    pub extension_id: String,
    pub extension_name: String,
    pub runtime: String,
    pub source: String,
    pub platforms: Vec<String>,
    pub permissions: ExtensionPermissionReview,
}

impl ExtensionListItem {
    fn installed(entry: ExtensionLockEntry) -> Self {
        let generated_custom = install::is_generated_custom_integration(&entry);
        let stored_runtime_available = crate::extensions::registry::runtime_available(&entry);
        let reconnect_available = entry.runtime_ownership == ExtensionRuntimeOwnership::System
            && !stored_runtime_available
            && crate::extensions::ExtensionManifest::load(Path::new(&entry.manifest_path))
                .and_then(|manifest| install::find_system_executable(&manifest))
                .is_ok();
        let runtime_source = match entry.provider_kind {
            ExtensionProviderKind::BundledStatic => "bundled".to_string(),
            ExtensionProviderKind::Executable | ExtensionProviderKind::StaticDescriptor => {
                match entry.runtime_ownership {
                    ExtensionRuntimeOwnership::Bundled => "managed".to_string(),
                    ExtensionRuntimeOwnership::System => "system".to_string(),
                }
            }
        };
        let homepage = crate::extensions::ExtensionManifest::load(Path::new(&entry.manifest_path))
            .ok()
            .and_then(|manifest| manifest.homepage);
        Self {
            entry,
            connected: true,
            runtime_source,
            runtime_available: stored_runtime_available,
            reconnect_available,
            homepage,
            generated_custom,
        }
    }

    fn detected(adapter: &crate::extensions::static_adapter::StaticAdapter) -> Self {
        let version = env!("CARGO_PKG_VERSION").to_string();
        let entry = ExtensionLockEntry {
            id: adapter.manifest.id.clone(),
            name: adapter.manifest.name.clone(),
            publisher_id: adapter.manifest.publisher.id.clone(),
            publisher_name: adapter.manifest.publisher.name.clone(),
            distribution_source: ExtensionDistributionSource::BuiltIn,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::BundledStatic,
            state: ExtensionStateKind::Disabled,
            enabled: false,
            package_name: None,
            package_version: version.clone(),
            tool_version: None,
            integrity: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: version,
            previous_version: None,
            manifest_path: String::new(),
            executable_path: adapter.invocation.executable.to_string_lossy().into_owned(),
            runtime_root: None,
            installed_at: 0,
            updated_at: 0,
            pinned: false,
            channel: "bundled".to_string(),
        };
        Self {
            runtime_available: adapter.runtime_available,
            runtime_source: "bundled".to_string(),
            connected: false,
            reconnect_available: false,
            homepage: adapter.manifest.homepage.clone(),
            generated_custom: false,
            entry,
        }
    }
}

#[tauri::command]
pub async fn extensions_export(
    app: AppHandle,
    state: State<'_, ExtensionState>,
) -> Result<Option<ExtensionsExportResult>, String> {
    let now = Utc::now();
    let (sender, receiver) = tokio::sync::oneshot::channel();
    app.dialog()
        .file()
        .add_filter("JSON", &["json"])
        .set_file_name(sync::default_export_file_name(Local::now().date_naive()))
        .save_file(move |selection| {
            let _ = sender.send(selection);
        });
    let selection = receiver
        .await
        .map_err(|_| "Extension export picker closed unexpectedly".to_string())?;
    let Some(path) = selection else {
        return Ok(None);
    };
    let path = path
        .into_path()
        .map_err(|_| "Extension exports must be saved to a local file".to_string())?;
    let _guard = state.mutation_lock.lock().await;
    let document = sync::build_export(&state, now)?;
    sync::write_export(&path, &document)?;
    Ok(Some(ExtensionsExportResult {
        path: path.to_string_lossy().into_owned(),
        extension_count: document.extensions.len(),
    }))
}

#[tauri::command]
pub async fn extensions_import(
    app: AppHandle,
    state: State<'_, ExtensionState>,
    locale: Option<String>,
) -> Result<Option<ExtensionsImportReport>, String> {
    let (sender, receiver) = tokio::sync::oneshot::channel();
    app.dialog()
        .file()
        .add_filter("JSON", &["json"])
        .pick_file(move |selection| {
            let _ = sender.send(selection);
        });
    let selection = receiver
        .await
        .map_err(|_| "Extension import picker closed unexpectedly".to_string())?;
    let Some(path) = selection else {
        return Ok(None);
    };
    let path = path
        .into_path()
        .map_err(|_| "Extension imports must use a local file".to_string())?;
    let document = sync::read_import(&path)?;
    let locale = locale.as_deref().unwrap_or("en");
    let is_zh = locale.to_ascii_lowercase().starts_with("zh");
    let installed = ExtensionsLock::load(&state.paths.lock_file)?;
    let mut approved_permissions = BTreeMap::new();
    let mut permission_lines = Vec::new();
    for entry in &document.extensions {
        if let Some(current) = installed.extensions.get(&entry.id) {
            let same_source = current.distribution_source == entry.distribution_source
                && current.runtime_ownership == entry.runtime_ownership;
            if !same_source
                || current.distribution_source != ExtensionDistributionSource::Npm
                || current.current_version == entry.version
                || current.previous_version.as_deref() == Some(entry.version.as_str())
            {
                continue;
            }
        }
        let review = match entry.distribution_source {
            ExtensionDistributionSource::Npm => {
                let package = entry.package.clone().ok_or_else(|| {
                    format!("NPM integration {} has no package in the export", entry.id)
                })?;
                install::permissions_summary(
                    &state,
                    &ExtensionInstallRequest {
                        source: install::InstallSource::Npm,
                        package: Some(package),
                        version: Some(entry.version.clone()),
                        manifest_path: None,
                        executable_path: None,
                        approved_permissions: None,
                    },
                    locale,
                )
                .await?
            }
            ExtensionDistributionSource::Local | ExtensionDistributionSource::BuiltIn => {
                let manifest = entry
                    .manifest
                    .clone()
                    .or_else(|| {
                        state
                            .static_adapters
                            .iter()
                            .find(|adapter| adapter.manifest.id == entry.id)
                            .map(|adapter| adapter.manifest.clone())
                    })
                    .ok_or_else(|| format!("Local integration {} has no manifest", entry.id))?;
                install::permission_review(&manifest, locale)
            }
        };
        if !review.permissions.is_empty() {
            permission_lines.push(format!(
                "{}: {}",
                review.extension_name,
                review
                    .permissions
                    .iter()
                    .map(|permission| permission.title.as_str())
                    .collect::<Vec<_>>()
                    .join(", ")
            ));
            approved_permissions.insert(
                entry.id.clone(),
                review
                    .permissions
                    .into_iter()
                    .map(|permission| permission.permission)
                    .collect(),
            );
        }
    }
    if !permission_lines.is_empty() {
        let omitted = permission_lines.len().saturating_sub(50);
        let mut displayed = permission_lines
            .iter()
            .take(50)
            .cloned()
            .collect::<Vec<_>>();
        if omitted > 0 {
            displayed.push(if is_zh {
                format!("另有 {omitted} 个插件")
            } else {
                format!("...and {omitted} more extensions")
            });
        }
        let (title, message, approve, cancel) = if is_zh {
            (
                "确认插件权限",
                "导入将安装或更新以下插件，请确认它们请求的权限：",
                "确认并导入",
                "取消",
            )
        } else {
            (
                "Review extension permissions",
                "Importing will install or update these extensions. Review their requested permissions:",
                "Approve and import",
                "Cancel",
            )
        };
        let (sender, receiver) = tokio::sync::oneshot::channel();
        app.dialog()
            .message(format!("{message}\n\n{}", displayed.join("\n")))
            .title(title)
            .buttons(MessageDialogButtons::OkCancelCustom(
                approve.to_string(),
                cancel.to_string(),
            ))
            .show(move |approved| {
                let _ = sender.send(approved);
            });
        let approved = receiver
            .await
            .map_err(|_| "Permission review dialog closed unexpectedly".to_string())?;
        if !approved {
            return Ok(None);
        }
    }
    let report = sync::import_document(&state, &path, document, &approved_permissions).await;
    state.invalidate_provider_commands().await;
    Ok(Some(report))
}

#[tauri::command]
pub async fn extensions_install(
    state: State<'_, ExtensionState>,
    request: ExtensionInstallRequest,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::install(&state, request).await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_create_custom(
    state: State<'_, ExtensionState>,
    request: CustomIntegrationRequest,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::create_custom_integration(&state, request).await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub fn extensions_custom_get(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<CustomIntegrationDefinition, String> {
    install::custom_integration_definition(&state, &id)
}

#[tauri::command]
pub async fn extensions_custom_update(
    state: State<'_, ExtensionState>,
    id: String,
    request: CustomIntegrationRequest,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::update_custom_integration(&state, &id, request).await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub fn extensions_search_path(query: String, limit: Option<usize>) -> Vec<PathExecutable> {
    install::search_path_executables(&query, limit.unwrap_or(12))
}

#[tauri::command]
pub async fn extensions_permissions_summary(
    state: State<'_, ExtensionState>,
    request: ExtensionInstallRequest,
    locale: Option<String>,
) -> Result<ExtensionPermissionReview, String> {
    install::permissions_summary(&state, &request, locale.as_deref().unwrap_or("en")).await
}

#[tauri::command]
pub async fn extensions_pick_local_manifest(app: AppHandle) -> Result<Option<String>, String> {
    let (sender, receiver) = tokio::sync::oneshot::channel();
    app.dialog()
        .file()
        .add_filter("Floter extension manifest", &["json"])
        .pick_file(move |selection| {
            let _ = sender.send(selection);
        });
    let selection = receiver
        .await
        .map_err(|_| "Local manifest picker closed unexpectedly".to_string())?;
    selection
        .map(|path| {
            path.into_path()
                .map(|path| path.to_string_lossy().into_owned())
                .map_err(|_| "Local integrations must use a local manifest file".to_string())
        })
        .transpose()
}

/// Pick either a package directory or its manifest without blocking the webview.
/// The selected path is validated before it is returned so the UI can present a
/// confirmation dialog with trustworthy details.
#[tauri::command]
pub async fn extensions_pick_local_package(
    app: AppHandle,
    state: State<'_, ExtensionState>,
) -> Result<Option<String>, String> {
    let (sender, receiver) = tokio::sync::oneshot::channel();
    app.dialog()
        .message("Choose a package folder (OK) or a floter.extension.json file (Cancel).")
        .title("Connect extension package")
        .buttons(MessageDialogButtons::OkCancelCustom(
            "Choose folder".to_string(),
            "Choose file".to_string(),
        ))
        .show(move |folder| {
            let _ = sender.send(folder);
        });
    let choose_folder = receiver
        .await
        .map_err(|_| "Local package picker closed unexpectedly".to_string())?;
    let (sender, receiver) = tokio::sync::oneshot::channel();
    if choose_folder {
        app.dialog().file().pick_folder(move |selection| {
            let _ = sender.send(selection);
        });
    } else {
        app.dialog()
            .file()
            .add_filter("Floter extension manifest", &["json"])
            .pick_file(move |selection| {
                let _ = sender.send(selection);
            });
    }
    let selection = receiver
        .await
        .map_err(|_| "Local package picker closed unexpectedly".to_string())?;
    let Some(path) = selection else {
        return Ok(None);
    };
    let path = path
        .into_path()
        .map_err(|_| "Local extension packages must use a local path".to_string())?;
    let manifest_path = if path.is_dir() {
        find_local_manifest(&path)?
    } else {
        path
    };
    let manifest = ExtensionManifest::load(&manifest_path)
        .map_err(|error| format!("manifest_invalid: {error}"))?;
    manifest
        .validate_compatibility(env!("CARGO_PKG_VERSION"))
        .map_err(|error| format!("manifest_incompatible: {error}"))?;
    manifest
        .clone()
        .resolve(PlatformTarget::current()?)
        .map_err(|error| format!("platform_incompatible: {error}"))?;
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if lock.extensions.contains_key(&manifest.id) {
        return Err(format!("duplicate_id: {}", manifest.id));
    }
    Ok(Some(manifest_path.to_string_lossy().into_owned()))
}

#[tauri::command]
pub fn extensions_local_manifest_review(
    state: State<'_, ExtensionState>,
    manifest_path: String,
    locale: Option<String>,
) -> Result<LocalManifestReview, String> {
    let path = Path::new(&manifest_path);
    let manifest = ExtensionManifest::load(path)?;
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    manifest.clone().resolve(PlatformTarget::current()?)?;
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if lock.extensions.contains_key(&manifest.id) {
        return Err(format!("Extension is already installed: {}", manifest.id));
    }
    let runtime = match manifest.runtime {
        crate::extensions::manifest::Runtime::System { .. } => "system",
        crate::extensions::manifest::Runtime::Bundled { .. } => "bundled",
        crate::extensions::manifest::Runtime::Script { .. } => "script",
    };
    let platforms = manifest
        .platforms
        .iter()
        .map(|platform| format!("{platform:?}").to_ascii_lowercase())
        .collect();
    Ok(LocalManifestReview {
        manifest_path,
        extension_id: manifest.id.clone(),
        extension_name: manifest.name.clone(),
        runtime: runtime.to_string(),
        source: "local".to_string(),
        platforms,
        permissions: install::permission_review(&manifest, locale.as_deref().unwrap_or("en")),
    })
}

fn find_local_manifest(root: &Path) -> Result<std::path::PathBuf, String> {
    let candidates = [
        root.join("floter.extension.json"),
        root.join("package").join("floter.extension.json"),
        root.join("extension").join("floter.extension.json"),
    ];
    candidates
        .into_iter()
        .find(|path| path.is_file())
        .ok_or_else(|| "manifest_missing: No floter.extension.json found in the selected folder. Add the manifest at the package root or package/ directory.".to_string())
}

#[tauri::command]
pub async fn extensions_custom_export_script(
    app: AppHandle,
    id: String,
    content: String,
    extension: String,
) -> Result<Option<String>, String> {
    let (sender, receiver) = tokio::sync::oneshot::channel();
    app.dialog()
        .file()
        .add_filter("Script", &[extension.trim_start_matches('.')])
        .set_file_name(format!("{id}-script.{}", extension.trim_start_matches('.')))
        .save_file(move |selection| {
            let _ = sender.send(selection);
        });
    let selection = receiver
        .await
        .map_err(|_| "Script export picker closed unexpectedly".to_string())?;
    let Some(path) = selection else {
        return Ok(None);
    };
    let path = path
        .into_path()
        .map_err(|_| "Scripts must be saved to a local file".to_string())?;
    std::fs::write(&path, content)
        .map_err(|error| format!("Cannot write script export: {error}"))?;
    Ok(Some(path.to_string_lossy().into_owned()))
}

#[tauri::command]
pub fn extensions_bundled_permissions(
    state: State<'_, ExtensionState>,
    id: String,
    locale: Option<String>,
) -> Result<ExtensionPermissionReview, String> {
    let adapter = state
        .static_adapters
        .iter()
        .find(|adapter| adapter.manifest.id == id)
        .ok_or_else(|| format!("Bundled integration is not available: {id}"))?;
    Ok(install::permission_review(
        &adapter.manifest,
        locale.as_deref().unwrap_or("en"),
    ))
}

#[tauri::command]
pub async fn extensions_connect_bundled(
    state: State<'_, ExtensionState>,
    id: String,
    executable_path: Option<String>,
    approved_permissions: Option<Vec<Permission>>,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::connect_bundled(
        &state,
        &id,
        executable_path.as_deref(),
        approved_permissions.as_deref(),
    )
    .await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_reconnect_system(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let current = lock
        .extensions
        .get(&id)
        .cloned()
        .ok_or_else(|| format!("Integration is not connected: {id}"))?;
    if current.runtime_ownership != ExtensionRuntimeOwnership::System {
        return Err(format!("Integration does not use a system runtime: {id}"));
    }
    let manifest = crate::extensions::ExtensionManifest::load(Path::new(&current.manifest_path))?;
    let executable = install::find_system_executable(&manifest)?;
    let resolved = manifest
        .clone()
        .resolve(crate::extensions::PlatformTarget::current()?)?;
    let mut tool_version =
        install::linked_tool_version(&manifest, &resolved.provider, &executable).await;

    if current.provider_kind == ExtensionProviderKind::Executable {
        let mut invocation = crate::extensions::provider::ProviderInvocation {
            extension_id: current.id.clone(),
            executable: executable.clone(),
            executable_prefix: Vec::new(),
            runtime_root: None,
            package_version: current.package_version.clone(),
            tool_version_hint: tool_version.clone(),
            version_args: match &manifest.runtime {
                crate::extensions::manifest::Runtime::System { version_args, .. } => {
                    version_args.clone()
                }
                crate::extensions::manifest::Runtime::Script { version_args, .. } => {
                    version_args.clone()
                }
                crate::extensions::manifest::Runtime::Bundled { .. } => Vec::new(),
            },
            config: resolved.provider,
            permissions: manifest.permissions,
        };
        let _ = config::apply_persisted_configuration(&state.paths.data, &mut invocation)?;
        let response = state.provider.describe(&invocation, true).await?;
        if tool_version.is_none() {
            tool_version = Some(response.description.provider.version);
        }
    }

    let entry = lock
        .extensions
        .get_mut(&id)
        .ok_or_else(|| format!("Integration is not connected: {id}"))?;
    entry.executable_path = executable.to_string_lossy().into_owned();
    entry.tool_version = tool_version;
    entry.updated_at = crate::extensions::lock::unix_now();
    let entry = entry.clone();
    lock.save(&state.paths.lock_file)?;
    drop(_guard);
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_uninstall(
    state: State<'_, ExtensionState>,
    id: String,
    remove_data: Option<bool>,
) -> Result<(), String> {
    install::uninstall(&state, &id, remove_data.unwrap_or(false)).await?;
    state.invalidate_provider_commands().await;
    Ok(())
}

#[tauri::command]
pub async fn extensions_enable(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionLockEntry, String> {
    set_enabled(&state, &id, true).await
}

#[tauri::command]
pub async fn extensions_disable(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionLockEntry, String> {
    set_enabled(&state, &id, false).await
}

async fn set_enabled(
    state: &ExtensionState,
    id: &str,
    enabled: bool,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    if !enabled {
        state.provider.cancel_completions();
    }
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    lock.set_enabled(id, enabled)?;
    let entry = lock.get(id)?.clone();
    lock.save(&state.paths.lock_file)?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_update(
    state: State<'_, ExtensionState>,
    id: String,
    version: Option<String>,
    approved_permissions: Option<Vec<Permission>>,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::update(
        &state,
        &id,
        version.as_deref(),
        approved_permissions.as_deref(),
    )
    .await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_reinstall(
    state: State<'_, ExtensionState>,
    id: String,
    approved_permissions: Option<Vec<Permission>>,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::reinstall(&state, &id, approved_permissions.as_deref()).await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_rollback(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionLockEntry, String> {
    let entry = install::rollback(&state, &id).await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_describe(
    state: State<'_, ExtensionState>,
    id: String,
    force: Option<bool>,
) -> Result<ProviderResponse, String> {
    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();
    if entry.provider_kind == ExtensionProviderKind::BundledStatic {
        let adapter = state
            .static_adapters
            .iter()
            .find(|adapter| adapter.manifest.id == id)
            .ok_or_else(|| format!("Bundled integration is not available: {id}"))?;
        return Ok(ProviderResponse {
            description: adapter.description.clone(),
            runtime_available: crate::extensions::static_adapter::is_linked_executable(Path::new(
                &entry.executable_path,
            )),
            cached: true,
            stderr: None,
        });
    }
    if entry.provider_kind == ExtensionProviderKind::StaticDescriptor {
        let (description, _) = crate::extensions::registry::static_description(&entry)?;
        return Ok(ProviderResponse {
            description,
            runtime_available: crate::extensions::registry::runtime_available(&entry),
            cached: true,
            stderr: None,
        });
    }
    let mut invocation = crate::extensions::registry::provider_invocation(&entry)?;
    let _ = config::apply_persisted_configuration(&state.paths.data, &mut invocation)?;
    state
        .provider
        .describe(&invocation, force.unwrap_or(false))
        .await
}

#[tauri::command]
pub async fn extensions_diagnose(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<DiagnoseResponse, String> {
    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();
    if entry.provider_kind == ExtensionProviderKind::BundledStatic {
        let runtime_available = crate::extensions::static_adapter::is_linked_executable(Path::new(
            &entry.executable_path,
        ));
        return Ok(DiagnoseResponse {
            status: if runtime_available {
                "healthy"
            } else {
                "error"
            }
            .to_string(),
            checks: vec![DiagnoseCheck {
                id: "runtime".to_string(),
                status: if runtime_available {
                    "healthy"
                } else {
                    "error"
                }
                .to_string(),
                message: if runtime_available {
                    format!("System tool is available at {}", entry.executable_path)
                } else {
                    format!(
                        "System tool is missing at {}. Install it or reconnect the integration.",
                        entry.executable_path
                    )
                },
            }],
        });
    }
    if entry.provider_kind == ExtensionProviderKind::StaticDescriptor {
        let (_, invocation) = crate::extensions::registry::static_description(&entry)?;
        let available = crate::extensions::registry::runtime_available(&entry);
        return Ok(DiagnoseResponse {
            status: if available { "healthy" } else { "error" }.to_string(),
            checks: vec![DiagnoseCheck {
                id: "runtime".to_string(),
                status: if available { "healthy" } else { "error" }.to_string(),
                message: if available {
                    format!(
                        "Runtime is available at {}",
                        invocation.executable.display()
                    )
                } else {
                    format!(
                        "Runtime is unavailable at {}",
                        invocation.executable.display()
                    )
                },
            }],
        });
    }
    let mut invocation = crate::extensions::registry::provider_invocation(&entry)?;
    let _ = config::apply_persisted_configuration(&state.paths.data, &mut invocation)?;
    state.provider.diagnose(&invocation).await
}

#[tauri::command]
pub async fn extensions_health(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<HealthReport, String> {
    let _entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();
    let health_dir = state.paths.data.join(&id);
    crate::extensions::health::read_health_report(&health_dir)?
        .ok_or_else(|| format!("No health report for {id}. Run 'extensions_describe {id}' first."))
}

#[tauri::command]
pub async fn extensions_reprobe(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<HealthReport, String> {
    use crate::extensions::capability_probe::{CapabilityProbe, ProbeResult};
    use crate::extensions::health::{HealthReport, HealthStatus, ProbeRecord, ProbeFailure};

    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();

    // Get the tool's executable path
    let executable = std::path::PathBuf::from(&entry.executable_path);
    if !executable.exists() {
        return Err(format!("Tool executable not found: {}", executable.display()));
    }

    // Build default probes: --version and --help
    let version_probe = CapabilityProbe::version();
    let help_probe = CapabilityProbe::help();

    let probes = vec![version_probe, help_probe];
    let required = vec![true, false]; // version required, help optional

    let health_dir = state.paths.data.join(&id);
    let mut report = HealthReport::new(Default::default());

    for (i, probe) in probes.iter().enumerate() {
        let start = std::time::Instant::now();
        let timeout = std::time::Duration::from_secs(5);

        match crate::extensions::probe_runner::run_single_probe(&executable, &probe.args, timeout).await {
            Ok(result) => {
                let duration = start.elapsed();
                if result.passed {
                    report.record_pass(&probe.id, duration, result.exit_code);
                } else {
                    report.record_failure(&probe.id, duration, result.exit_code, result.stderr, !required[i]);
                }
            }
            Err(error) => {
                let duration = start.elapsed();
                report.record_failure(&probe.id, duration, None, error, !required[i]);
            }
        }
    }

    let required_ids: Vec<String> = required.iter().enumerate()
        .filter(|(_, r)| **r)
        .map(|(i, _)| probes[i].id.clone())
        .collect();
    report.finalize(&required_ids);

    // Save the health report
    crate::extensions::health::write_health_report(&health_dir, &report)?;

    Ok(report)
}

#[tauri::command]
pub async fn extensions_launch(
    state: State<'_, ExtensionState>,
    id: String,
    argv: Vec<String>,
    cwd: Option<String>,
) -> Result<serde_json::Value, String> {
    use crate::extensions::cwd_policy::CwdContext;
    use crate::extensions::session_restore::{SessionDescription, SessionResolver, RestorePolicy, ResolvedSession};

    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();

    // Get tool data directory
    let tool_data_dir = state.paths.data.join(&id);
    std::fs::create_dir_all(&tool_data_dir)
        .map_err(|e| format!("Cannot create tool data dir: {e}"))?;

    // Resolve cwd
    let active_session_cwd = cwd.as_deref().map(std::path::Path::new);
    let cwd_context = CwdContext::new(active_session_cwd, &tool_data_dir, false);

    // Default policy: InheritActiveSession
    let policy = crate::extensions::cwd_policy::CwdPolicy::InheritActiveSession;
    let resolved_cwd = policy.resolve(&cwd_context)?;

    // Resolve session
    let sessions_dir = tool_data_dir.join("sessions");
    let session_resolver = SessionResolver::new(sessions_dir);
    let restore_policy = RestorePolicy::Reattach;

    let tool_version = entry.package_version.clone();
    let session = session_resolver.resolve(
        &id,
        &tool_version,
        argv.clone(),
        resolved_cwd.clone(),
        vec!["profile:default".to_string()],
        None,
        restore_policy,
    )?;

    let (session_id, is_restart) = match &session {
        ResolvedSession::Reattach(s) => (s.session_id.clone(), false),
        ResolvedSession::Restart(s) => (s.session_id.clone(), true),
        ResolvedSession::New(s) => (s.session_id.clone(), false),
    };

    // Write session file
    let session_desc = match session {
        ResolvedSession::Reattach(s) => s,
        ResolvedSession::Restart(s) => s,
        ResolvedSession::New(s) => s,
    };
    session_resolver.write_session(&session_desc)?;

    // Build launch plan
    let launch_plan = serde_json::json!({
        "sessionId": session_id,
        "toolId": id,
        "version": tool_version,
        "argv": argv,
        "cwd": resolved_cwd.to_string_lossy(),
        "isRestart": is_restart,
        "terminal": {
            "required": true,
            "color": "truecolor",
            "unicode": true,
            "bracketedPaste": true,
            "synchronizedOutput": "preferred",
            "keyboardProtocol": "kitty-preferred"
        },
        "environment": {
            "TERM": "floter-256color",
            "COLORTERM": "truecolor",
            "TERM_PROGRAM": "floter",
        }
    });

    Ok(launch_plan)
}

#[tauri::command]
pub async fn extensions_search(
    state: State<'_, ExtensionState>,
    query: String,
    limit: Option<usize>,
) -> Result<Vec<ExtensionSearchResult>, String> {
    install::search(&state, &query, limit.unwrap_or(20)).await
}

#[tauri::command]
pub async fn extensions_config_get(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionConfiguration, String> {
    reject_bundled_static_configuration(&state, &id)?;
    config::get(&state, &id).await
}

#[tauri::command]
pub async fn extensions_config_set(
    state: State<'_, ExtensionState>,
    id: String,
    values: BTreeMap<String, Value>,
) -> Result<ExtensionConfiguration, String> {
    reject_bundled_static_configuration(&state, &id)?;
    let configuration = config::set(&state, &id, values).await?;
    state.invalidate_provider_commands().await;
    Ok(configuration)
}

#[tauri::command]
pub async fn extensions_config_copy(
    state: State<'_, ExtensionState>,
    id: String,
    values: BTreeMap<String, Value>,
) -> Result<String, String> {
    reject_bundled_static_configuration(&state, &id)?;
    config::export_json(&state, &id, values).await
}

#[tauri::command]
pub async fn extensions_config_export(
    app: AppHandle,
    state: State<'_, ExtensionState>,
    id: String,
    values: BTreeMap<String, Value>,
) -> Result<Option<String>, String> {
    reject_bundled_static_configuration(&state, &id)?;
    let json = config::export_json(&state, &id, values).await?;
    let (sender, receiver) = tokio::sync::oneshot::channel();
    app.dialog()
        .file()
        .add_filter("JSON", &["json"])
        .set_file_name(format!("floter-{id}-config.json"))
        .save_file(move |selection| {
            let _ = sender.send(selection);
        });
    let selection = receiver
        .await
        .map_err(|_| "Configuration export picker closed unexpectedly".to_string())?;
    let Some(path) = selection else {
        return Ok(None);
    };
    let path = path
        .into_path()
        .map_err(|_| "Configuration exports must be saved to a local file".to_string())?;
    config::write_export(&path, &json)?;
    Ok(Some(path.to_string_lossy().into_owned()))
}

fn reject_bundled_static_configuration(state: &ExtensionState, id: &str) -> Result<(), String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if matches!(
        lock.get(id)?.provider_kind,
        ExtensionProviderKind::BundledStatic | ExtensionProviderKind::StaticDescriptor
    ) {
        Err(format!(
            "Static integration {id} does not provide configurable settings"
        ))
    } else {
        Ok(())
    }
}

#[tauri::command]
pub async fn catalog_search(
    app: AppHandle,
    application_state: State<'_, ApplicationState>,
    extension_state: State<'_, ExtensionState>,
    request: CatalogSearchRequest,
) -> Result<Vec<CatalogEntry>, String> {
    let applications = list_applications(app, application_state, Some(false)).await?;
    catalog::search(&extension_state, &request, &applications).await
}

#[tauri::command]
pub async fn catalog_complete(
    state: State<'_, ExtensionState>,
    request: CompletionRequest,
) -> Result<CatalogCompletionResponse, String> {
    catalog::complete(&state, &request).await
}
