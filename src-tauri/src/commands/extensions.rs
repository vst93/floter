use crate::commands::apps::{list_applications, ApplicationState};
use crate::extensions::catalog::{
    self, CatalogCompletionResponse, CatalogEntry, CatalogSearchRequest, CompletionRequest,
};
use crate::extensions::config::{self, ExtensionConfiguration};
use crate::extensions::install::{
    self, ExtensionInstallRequest, ExtensionPermissionReview, ExtensionSearchResult,
};
use crate::extensions::lock::{ExtensionLockEntry, ExtensionsLock};
use crate::extensions::provider::{DiagnoseResponse, ProviderResponse};
use crate::extensions::ExtensionState;
use serde_json::Value;
use std::collections::BTreeMap;
use tauri::{AppHandle, State};

#[tauri::command]
pub fn extensions_list(
    state: State<'_, ExtensionState>,
) -> Result<Vec<ExtensionLockEntry>, String> {
    Ok(ExtensionsLock::load(&state.paths.lock_file)?.list())
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
) -> Result<ExtensionLockEntry, String> {
    install::update(&state, &id, version.as_deref()).await
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
