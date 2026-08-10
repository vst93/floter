use crate::commands::apps::{list_applications, ApplicationState};
use crate::extensions::catalog::{
    self, CatalogCompletionResponse, CatalogEntry, CatalogSearchRequest, CompletionRequest,
};
use crate::extensions::config::{self, ExtensionConfiguration};
use crate::extensions::install::{
    self, ExtensionInstallRequest, ExtensionPermissionReview, ExtensionSearchResult,
};
use crate::extensions::lock::{ExtensionInstallType, ExtensionLockEntry, ExtensionsLock};
use crate::extensions::manifest::Permission;
use crate::extensions::provider::{DiagnoseResponse, ProviderResponse};
use crate::extensions::sync::{self, ExtensionsExportResult, ExtensionsImportReport};
use crate::extensions::ExtensionState;
use chrono::{Local, Utc};
use serde_json::Value;
use std::collections::BTreeMap;
use tauri::{AppHandle, State};
use tauri_plugin_dialog::{DialogExt, MessageDialogButtons};

#[tauri::command]
pub fn extensions_list(
    state: State<'_, ExtensionState>,
) -> Result<Vec<ExtensionLockEntry>, String> {
    Ok(ExtensionsLock::load(&state.paths.lock_file)?.list())
}

#[tauri::command]
pub async fn extensions_export(
    app: AppHandle,
    state: State<'_, ExtensionState>,
) -> Result<Option<ExtensionsExportResult>, String> {
    let now = Utc::now();
    let selection = app
        .dialog()
        .file()
        .add_filter("JSON", &["json"])
        .set_file_name(sync::default_export_file_name(Local::now().date_naive()))
        .blocking_save_file();
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
    let selection = app
        .dialog()
        .file()
        .add_filter("JSON", &["json"])
        .blocking_pick_file();
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
            let same_source = matches!(
                (current.install_type, entry.source),
                (ExtensionInstallType::Managed, sync::SyncSource::Managed)
                    | (ExtensionInstallType::Linked, sync::SyncSource::Linked)
            );
            let current_version = if current.install_type == ExtensionInstallType::Linked {
                current
                    .tool_version
                    .as_deref()
                    .unwrap_or(&current.current_version)
            } else {
                &current.current_version
            };
            if !same_source
                || current.install_type == ExtensionInstallType::Linked
                || current_version == entry.version
                || current.previous_version.as_deref() == Some(entry.version.as_str())
            {
                continue;
            }
        }
        let review = match entry.source {
            sync::SyncSource::Managed => {
                let package = entry.package.clone().ok_or_else(|| {
                    format!(
                        "Managed extension {} has no package in the export",
                        entry.id
                    )
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
            sync::SyncSource::Linked => {
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
                    .ok_or_else(|| format!("Linked extension {} has no manifest", entry.id))?;
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
        let approved = app
            .dialog()
            .message(format!("{message}\n\n{}", displayed.join("\n")))
            .title(title)
            .buttons(MessageDialogButtons::OkCancelCustom(
                approve.to_string(),
                cancel.to_string(),
            ))
            .blocking_show();
        if !approved {
            return Ok(None);
        }
    }
    Ok(Some(
        sync::import_document(&state, &path, document, &approved_permissions).await,
    ))
}

#[tauri::command]
pub async fn extensions_install(
    state: State<'_, ExtensionState>,
    request: ExtensionInstallRequest,
) -> Result<ExtensionLockEntry, String> {
    install::install(&state, request).await
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
pub async fn extensions_uninstall(
    state: State<'_, ExtensionState>,
    id: String,
    remove_data: Option<bool>,
) -> Result<(), String> {
    install::uninstall(&state, &id, remove_data.unwrap_or(false)).await
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
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_update(
    state: State<'_, ExtensionState>,
    id: String,
    version: Option<String>,
    approved_permissions: Option<Vec<Permission>>,
) -> Result<ExtensionLockEntry, String> {
    install::update(
        &state,
        &id,
        version.as_deref(),
        approved_permissions.as_deref(),
    )
    .await
}

#[tauri::command]
pub async fn extensions_rollback(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionLockEntry, String> {
    install::rollback(&state, &id).await
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
    let mut invocation = catalog::invocation_from_entry(&entry)?;
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
    let mut invocation = catalog::invocation_from_entry(&entry)?;
    let _ = config::apply_persisted_configuration(&state.paths.data, &mut invocation)?;
    state.provider.diagnose(&invocation).await
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
    config::get(&state, &id).await
}

#[tauri::command]
pub async fn extensions_config_set(
    state: State<'_, ExtensionState>,
    id: String,
    values: BTreeMap<String, Value>,
) -> Result<ExtensionConfiguration, String> {
    config::set(&state, &id, values).await
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
