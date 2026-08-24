use crate::commands::apps::{list_applications, ApplicationState};
use crate::extensions::catalog::{
    self, CatalogCompletionResponse, CatalogEntry, CatalogSearchRequest, CompletionRequest,
};
use crate::extensions::config::{self, ExtensionConfiguration};
use crate::extensions::health::HealthReport;
use crate::extensions::install::{
    self, CustomIntegrationDefinition, CustomIntegrationRequest, ExtensionInstallRequest,
    ExtensionPermissionReview,
};
use crate::extensions::inventory::{self, ToolCandidate, ToolLocator};
use crate::extensions::lock::{
    ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
    ExtensionRuntimeOwnership, ExtensionStateKind, ExtensionsLock,
};
use crate::extensions::manifest::{ExtensionManifest, Permission, PlatformTarget, Runtime};
use crate::extensions::provider::{DiagnoseCheck, DiagnoseResponse, ProviderResponse};
use crate::extensions::sync::{self, ExtensionsExportResult, ExtensionsImportReport};
use crate::extensions::tool_manifests;
use crate::extensions::{
    resolver, ExtensionState, LockState, ResolveRequest, ResolveResult, ToolLockEntry,
};
use chrono::{Local, Utc};
use serde::Serialize;
use serde_json::Value;
use std::collections::BTreeMap;
use std::path::Path;
use tauri::{AppHandle, Manager, State};
use tauri_plugin_dialog::{DialogExt, MessageDialogButtons};

#[tauri::command]
pub fn extensions_list(state: State<'_, ExtensionState>) -> Result<Vec<ExtensionListItem>, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let candidates = state
        .tool_inventory
        .lock()
        .map_err(|_| "Tool inventory is unavailable".to_string())?
        .candidates();
    let mut tool_lock = state
        .tool_lock
        .lock()
        .map_err(|_| "Tool lock is unavailable".to_string())?;
    let tool_lock_snapshot = tool_lock.clone();
    let mut tool_lock_changed = false;
    let mut items = Vec::new();
    for mut entry in lock.list() {
        // Installation persists this result only after both package and
        // official-index signatures pass. Never present an official badge
        // if the package signature is no longer trusted.
        entry.official_verified &= entry.signature_verified;
        let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path)).ok();
        let current_candidate = (entry.runtime_ownership == ExtensionRuntimeOwnership::System)
            .then(|| {
                inventory::executable_candidate(
                    Path::new(&entry.executable_path),
                    executable_display_name(&entry.executable_path),
                )
            });
        let lock_state = if entry.runtime_ownership == ExtensionRuntimeOwnership::System {
            if !tool_lock.tools.contains_key(&entry.id) {
                tool_lock.bind_locator(
                    &entry.id,
                    ToolLocator::Executable {
                        path: entry.executable_path.clone(),
                    },
                    current_candidate
                        .as_ref()
                        .and_then(|candidate| candidate.fingerprint.clone()),
                );
                tool_lock_changed = true;
            }
            let previous = tool_lock.tools.get(&entry.id).map(|binding| binding.state);
            let current = tool_lock
                .check(&entry.id, current_candidate.as_ref())?
                .state;
            tool_lock_changed |= previous != Some(current);
            Some(current)
        } else {
            None
        };
        let tool_candidates = if lock_state.is_some_and(|state| state != LockState::Connected) {
            let mut resolution_pool = candidates.clone();
            if let Some(candidate) = current_candidate.filter(|candidate| candidate.available) {
                if !resolution_pool
                    .iter()
                    .any(|existing| existing.locator.normalized() == candidate.locator.normalized())
                {
                    resolution_pool.push(candidate);
                }
            }
            manifest
                .as_ref()
                .map(|manifest| {
                    resolution_candidates(resolve_manifest_candidate(
                        manifest,
                        &resolution_pool,
                        tool_lock
                            .tools
                            .get(&entry.id)
                            .map(|binding| binding.locator.normalized()),
                    ))
                })
                .unwrap_or_default()
        } else {
            Vec::new()
        };
        let reconnect_available = !tool_candidates.is_empty();
        items.push(ExtensionListItem::installed(
            entry,
            lock_state,
            reconnect_available,
            tool_candidates,
        ));
    }
    if tool_lock_changed {
        if let Err(error) = tool_lock.save(&state.paths.tool_lock_file) {
            *tool_lock = tool_lock_snapshot;
            return Err(error);
        }
    }
    // Recommended tools ship as ordinary manifest/descriptor data. They
    // surface in the same suggestion area as PATH discoveries and connect
    // through the same generic linked-install pipeline.
    for recommendation in &state.recommendations {
        if lock.extensions.contains_key(&recommendation.manifest.id) {
            continue;
        }
        let tool_candidates = resolution_candidates(resolve_manifest_candidate(
            &recommendation.manifest,
            &candidates,
            None,
        ));
        if tool_candidates.is_empty() {
            continue;
        }
        if let Some(path) = tool_candidates[0].locator.executable_path() {
            if lock.extensions.values().any(|entry| {
                entry
                    .executable_path
                    .eq_ignore_ascii_case(&path.to_string_lossy())
            }) {
                continue;
            }
        }
        items.push(ExtensionListItem::suggested_recommended(
            recommendation,
            tool_candidates,
        ));
    }
    // Convention-location manifests (<config>/floter/tools/*.json): every
    // valid local manifest becomes a suggestion row that connects through
    // the same pipeline as shipped recommendations. The scanner skips
    // unreadable or invalid files silently.
    let manifest_rows = manifest_suggestions(
        &lock,
        &state.recommendations,
        &candidates,
        &tool_manifests::directory_for_root(&state.paths.root),
    );
    let reserved_executables: Vec<String> = manifest_rows
        .iter()
        .filter_map(|(_, candidates)| {
            candidates
                .first()
                .and_then(|candidate| candidate.locator.executable_path())
                .map(|path| path.to_string_lossy().into_owned())
        })
        .collect();
    for (tool, tool_candidates) in &manifest_rows {
        items.push(ExtensionListItem::suggested_manifest(
            tool,
            tool_candidates.clone(),
        ));
    }
    // Discovery suggestions: PATH executables that are not connected yet and
    // do not collide with a known integration name. Sorted by discovery
    // quality then name so the best candidates surface first.
    let mut suggestions: Vec<ToolCandidate> = Vec::new();
    for candidate in &candidates {
        if !candidate.available {
            continue;
        }
        let Some(path) = candidate.locator.executable_path() else {
            continue;
        };
        if !install::is_linked_executable_public(path) {
            continue;
        }
        if reserved_executables
            .iter()
            .any(|reserved| reserved.eq_ignore_ascii_case(&path.to_string_lossy()))
        {
            continue;
        }
        if lock.extensions.values().any(|entry| {
            entry.name.eq_ignore_ascii_case(&candidate.name)
                || entry
                    .executable_path
                    .eq_ignore_ascii_case(&path.to_string_lossy())
        }) {
            continue;
        }
        if suggestions
            .iter()
            .any(|existing| existing.name.eq_ignore_ascii_case(&candidate.name))
        {
            continue;
        }
        suggestions.push(candidate.clone());
    }
    suggestions.sort_by(|left, right| {
        left.quality
            .cmp(&right.quality)
            .then(left.name.cmp(&right.name))
    });
    if suggestions.len() > 12 {
        suggestions.truncate(12);
    }
    for candidate in &suggestions {
        items.push(ExtensionListItem::suggested_discovered(candidate));
    }
    Ok(items)
}

#[tauri::command]
pub async fn extensions_refresh_official_status(
    state: State<'_, ExtensionState>,
) -> Result<BTreeMap<String, bool>, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let official_index = crate::extensions::official_index::fetch(&state).await.ok();
    Ok(lock
        .list()
        .into_iter()
        .map(|entry| {
            let verified = entry.signature_verified
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
            (entry.id, verified)
        })
        .collect())
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
    pub recommended: bool,
    /// Suggested from a convention-location manifest
    /// (`<config>/floter/tools/*.json`) rather than a PATH scan.
    pub manifest_suggestion: bool,
    pub tool_lock_state: Option<LockState>,
    pub tool_candidates: Vec<ToolCandidate>,
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
    fn installed(
        entry: ExtensionLockEntry,
        tool_lock_state: Option<LockState>,
        reconnect_available: bool,
        tool_candidates: Vec<ToolCandidate>,
    ) -> Self {
        let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path)).ok();
        let generated_custom = install::is_generated_custom_integration(&entry);
        let stored_runtime_available = crate::extensions::registry::runtime_available(&entry);
        let runtime_available = stored_runtime_available
            && tool_lock_state.is_none_or(|state| state == LockState::Connected);
        let runtime_source = match entry.provider_kind {
            ExtensionProviderKind::BundledStatic => "bundled".to_string(),
            ExtensionProviderKind::Executable | ExtensionProviderKind::StaticDescriptor => {
                match entry.runtime_ownership {
                    ExtensionRuntimeOwnership::Bundled => "managed".to_string(),
                    ExtensionRuntimeOwnership::System => "system".to_string(),
                }
            }
        };
        let homepage = manifest.and_then(|manifest| manifest.homepage);
        Self {
            entry,
            connected: true,
            runtime_source,
            runtime_available,
            reconnect_available,
            homepage,
            generated_custom,
            recommended: false,
            manifest_suggestion: false,
            tool_lock_state,
            tool_candidates,
        }
    }

    /// Suggestion row for a shipped recommended tool. The entry is synthetic
    /// (not in the lock yet); it mirrors a local static integration so the
    /// frontend routes it through the same connect flow as a PATH discovery.
    fn suggested_recommended(
        recommendation: &crate::extensions::recommendations::RecommendedTool,
        tool_candidates: Vec<ToolCandidate>,
    ) -> Self {
        let candidate = (tool_candidates.len() == 1).then(|| &tool_candidates[0]);
        let version = recommendation.description.provider.version.clone();
        let entry = ExtensionLockEntry {
            id: recommendation.manifest.id.clone(),
            name: recommendation.manifest.name.clone(),
            publisher_id: recommendation.manifest.publisher.id.clone(),
            publisher_name: recommendation.manifest.publisher.name.clone(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::StaticDescriptor,
            state: ExtensionStateKind::Disabled,
            enabled: false,
            package_name: None,
            package_version: version.clone(),
            tool_version: candidate.and_then(|candidate| candidate.version.clone()),
            integrity: None,
            runtime_integrity: None,
            content_integrity: None,
            previous_integrity: None,
            previous_runtime_integrity: None,
            previous_content_integrity: None,
            asset_selection: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: version,
            previous_version: None,
            manifest_path: String::new(),
            executable_path: candidate
                .and_then(|candidate| candidate.locator.executable_path())
                .map(|path| path.to_string_lossy().into_owned())
                .unwrap_or_default(),
            runtime_root: None,
            installed_at: 0,
            updated_at: 0,
            pinned: false,
            channel: "external".to_string(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
        };
        Self {
            runtime_available: !tool_candidates.is_empty(),
            runtime_source: "system".to_string(),
            connected: false,
            recommended: true,
            manifest_suggestion: false,
            reconnect_available: false,
            homepage: recommendation.manifest.homepage.clone(),
            generated_custom: false,
            tool_lock_state: None,
            tool_candidates,
            entry,
        }
    }

    /// Discovery suggestion for an arbitrary PATH executable. Rendered by the
    /// same detected-row path as bundled suggestions; connecting runs the
    /// regular custom-integration pipeline (see install::connect_tool).
    fn suggested_discovered(candidate: &ToolCandidate) -> Self {
        let executable = candidate
            .locator
            .executable_path()
            .map(|path| path.to_string_lossy().into_owned())
            .unwrap_or_default();
        let entry = ExtensionLockEntry {
            id: format!("local.discovered:{}", candidate.name),
            name: candidate.name.clone(),
            publisher_id: "local-user".to_string(),
            publisher_name: "Local user".to_string(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::StaticDescriptor,
            state: ExtensionStateKind::Disabled,
            enabled: false,
            package_name: None,
            package_version: "0.0.0".to_string(),
            tool_version: candidate.version.clone(),
            integrity: None,
            runtime_integrity: None,
            content_integrity: None,
            previous_integrity: None,
            previous_runtime_integrity: None,
            previous_content_integrity: None,
            asset_selection: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: candidate.version.clone().unwrap_or_else(|| "0.0.0".into()),
            previous_version: None,
            manifest_path: String::new(),
            executable_path: executable.clone(),
            runtime_root: None,
            installed_at: 0,
            updated_at: 0,
            pinned: false,
            channel: "external".to_string(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
        };
        Self {
            runtime_available: candidate.available,
            runtime_source: "system".to_string(),
            connected: false,
            recommended: false,
            manifest_suggestion: false,
            reconnect_available: false,
            homepage: None,
            generated_custom: false,
            tool_lock_state: None,
            tool_candidates: Vec::new(),
            entry,
        }
    }

    /// Suggestion row for a convention-location manifest discovered in
    /// `<config>/floter/tools/`. Rendered like the other discovery
    /// suggestions, but carrying the authored manifest id so connecting runs
    /// the recommended-tool pipeline with the original manifest bytes
    /// instead of a generated single-command integration.
    fn suggested_manifest(
        tool: &tool_manifests::DiscoveredManifest,
        tool_candidates: Vec<ToolCandidate>,
    ) -> Self {
        let candidate = tool_candidates.first();
        let version = tool
            .descriptor_bytes
            .as_deref()
            .and_then(|bytes| crate::extensions::provider::ProviderDescription::parse(bytes).ok())
            .map(|description| description.provider.version)
            .unwrap_or_else(|| "0.0.0".to_string());
        let entry = ExtensionLockEntry {
            id: tool.manifest.id.clone(),
            name: tool.manifest.name.clone(),
            publisher_id: tool.manifest.publisher.id.clone(),
            publisher_name: tool.manifest.publisher.name.clone(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: match tool.manifest.provider.kind {
                crate::extensions::manifest::ProviderKind::Executable => {
                    ExtensionProviderKind::Executable
                }
                crate::extensions::manifest::ProviderKind::StaticDescriptor => {
                    ExtensionProviderKind::StaticDescriptor
                }
            },
            state: ExtensionStateKind::Disabled,
            enabled: false,
            package_name: None,
            package_version: version.clone(),
            tool_version: candidate.and_then(|candidate| candidate.version.clone()),
            integrity: None,
            runtime_integrity: None,
            content_integrity: None,
            previous_integrity: None,
            previous_runtime_integrity: None,
            previous_content_integrity: None,
            asset_selection: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: version,
            previous_version: None,
            manifest_path: tool.source_path.to_string_lossy().into_owned(),
            executable_path: candidate
                .and_then(|candidate| candidate.locator.executable_path())
                .map(|path| path.to_string_lossy().into_owned())
                .unwrap_or_default(),
            runtime_root: None,
            installed_at: 0,
            updated_at: 0,
            pinned: false,
            channel: "external".to_string(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
        };
        Self {
            runtime_available: !tool_candidates.is_empty(),
            runtime_source: "system".to_string(),
            connected: false,
            recommended: false,
            manifest_suggestion: true,
            reconnect_available: false,
            homepage: tool.manifest.homepage.clone(),
            generated_custom: false,
            tool_lock_state: None,
            tool_candidates,
            entry,
        }
    }
}

fn executable_display_name(path: &str) -> String {
    Path::new(path)
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or(path)
        .to_string()
}

fn resolve_manifest_candidate(
    manifest: &ExtensionManifest,
    candidates: &[ToolCandidate],
    preferred_locator: Option<String>,
) -> ResolveResult {
    let Runtime::System {
        executable_names, ..
    } = &manifest.runtime
    else {
        return ResolveResult::NotFound {
            tool: manifest.id.clone(),
        };
    };
    let tool = executable_names
        .first()
        .cloned()
        .unwrap_or_else(|| manifest.id.clone());
    resolver::resolve_executable_names(
        &ResolveRequest {
            tool,
            profile: None,
            required_version: None,
            preferred_locator,
        },
        executable_names,
        candidates,
    )
}

fn resolution_candidates(result: ResolveResult) -> Vec<ToolCandidate> {
    match result {
        ResolveResult::Selected { candidate, .. } => vec![candidate],
        ResolveResult::Ambiguous { candidates } => candidates
            .into_iter()
            .map(|candidate| candidate.candidate)
            .collect(),
        ResolveResult::NotFound { .. } => Vec::new(),
    }
}

/// Suggestion rows from convention-location manifests, already filtered:
/// connected integrations are never re-suggested (by extension id or by
/// executable path), shipped recommendations keep precedence for their own
/// ids, and manifests without a resolvable runtime yield no row.
pub(crate) fn manifest_suggestions(
    lock: &ExtensionsLock,
    recommendations: &[crate::extensions::recommendations::RecommendedTool],
    candidates: &[ToolCandidate],
    directory: &Path,
) -> Vec<(tool_manifests::DiscoveredManifest, Vec<ToolCandidate>)> {
    let mut rows = Vec::new();
    let mut reserved_paths: Vec<String> = Vec::new();
    for tool in tool_manifests::scan(directory) {
        if lock.extensions.contains_key(&tool.manifest.id)
            || recommendations
                .iter()
                .any(|recommendation| recommendation.manifest.id == tool.manifest.id)
        {
            continue;
        }
        let tool_candidates =
            resolution_candidates(resolve_manifest_candidate(&tool.manifest, candidates, None));
        let Some(first) = tool_candidates.first() else {
            continue;
        };
        if let Some(path) = first.locator.executable_path() {
            let path_text = path.to_string_lossy().into_owned();
            if lock
                .extensions
                .values()
                .any(|entry| entry.executable_path.eq_ignore_ascii_case(&path_text))
                || reserved_paths
                    .iter()
                    .any(|reserved| reserved.eq_ignore_ascii_case(&path_text))
            {
                continue;
            }
            reserved_paths.push(path_text);
        }
        rows.push((tool, tool_candidates));
    }
    rows
}

fn discovered_system_candidate(
    state: &ExtensionState,
    binding: &str,
    manifest: &ExtensionManifest,
    force_refresh: bool,
) -> Result<ToolCandidate, String> {
    let preferred = state
        .tool_lock
        .lock()
        .map_err(|_| "Tool lock is unavailable".to_string())?
        .tools
        .get(binding)
        .map(|entry| entry.locator.clone());
    let mut candidates = {
        let mut inventory = state
            .tool_inventory
            .lock()
            .map_err(|_| "Tool inventory is unavailable".to_string())?;
        if force_refresh {
            inventory.refresh();
        }
        inventory.candidates()
    };
    if let Some(path) = preferred.as_ref().and_then(ToolLocator::executable_path) {
        if let Some(candidate) =
            inventory::inspect_executable(path, executable_display_name(&path.to_string_lossy()))
        {
            if !candidates
                .iter()
                .any(|existing| existing.locator.normalized() == candidate.locator.normalized())
            {
                candidates.push(candidate);
            }
        }
    }
    let preferred_locator = preferred.map(|locator| locator.normalized());
    match resolve_manifest_candidate(manifest, &candidates, preferred_locator) {
        ResolveResult::Selected { candidate, .. } => Ok(candidate),
        ResolveResult::Ambiguous { candidates } => Err(format!(
            "Multiple system tools match {}: {}",
            manifest.name,
            candidates
                .iter()
                .map(|candidate| candidate.candidate.locator.normalized())
                .collect::<Vec<_>>()
                .join(", ")
        )),
        ResolveResult::NotFound { .. } => {
            Err(format!("Cannot find a system tool for {}", manifest.name))
        }
    }
}

fn inspect_manifest_executable(
    manifest: &ExtensionManifest,
    path: &str,
) -> Result<ToolCandidate, String> {
    let candidate = inventory::inspect_executable(Path::new(path), executable_display_name(path))
        .ok_or_else(|| format!("System tool is not available at {path}"))?;
    match resolve_manifest_candidate(manifest, std::slice::from_ref(&candidate), None) {
        ResolveResult::Selected { .. } => Ok(candidate),
        ResolveResult::Ambiguous { .. } | ResolveResult::NotFound { .. } => Err(format!(
            "Executable {} does not match the runtime declared by {}",
            path, manifest.name
        )),
    }
}

fn persist_tool_binding(
    state: &ExtensionState,
    binding: &str,
    candidate: &ToolCandidate,
) -> Result<ToolLockEntry, String> {
    let mut lock = state
        .tool_lock
        .lock()
        .map_err(|_| "Tool lock is unavailable".to_string())?;
    let previous = lock.clone();
    if lock.tools.contains_key(binding) {
        lock.reconnect(binding, candidate)?;
    } else {
        lock.bind(binding, candidate);
    }
    if let Err(error) = lock.save(&state.paths.tool_lock_file) {
        *lock = previous;
        return Err(error);
    }
    Ok(lock.tools[binding].clone())
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
                // NPM distribution is retired: there is nothing to review
                // offline, and the import itself will report the entry as
                // failed with guidance to reconnect it as a local tool.
                continue;
            }
            ExtensionDistributionSource::Local | ExtensionDistributionSource::BuiltIn => {
                let manifest = entry
                    .manifest
                    .clone()
                    .or_else(|| {
                        state
                            .recommendations
                            .iter()
                            .find(|recommendation| recommendation.manifest.id == entry.id)
                            .map(|recommendation| recommendation.manifest.clone())
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
pub async fn extensions_search_tools(
    app: AppHandle,
    query: String,
    limit: Option<usize>,
    force_refresh: Option<bool>,
    executable_only: Option<bool>,
) -> Result<Vec<ToolCandidate>, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let state = app.state::<ExtensionState>();
        let mut inventory = state
            .tool_inventory
            .lock()
            .map_err(|_| "Tool inventory is unavailable".to_string())?;
        if force_refresh.unwrap_or(false) {
            inventory.refresh();
        }
        let mut candidates = inventory.search(&query);
        if executable_only.unwrap_or(false) {
            candidates.retain(|candidate| candidate.locator.executable_path().is_some());
        }
        candidates.truncate(limit.unwrap_or(12).clamp(1, 50));
        Ok(candidates)
    })
    .await
    .map_err(|error| format!("Tool discovery task failed: {error}"))?
}

/// One-click connection of an auto-discovered PATH tool. The candidate comes
/// from the discovery suggestions in `extensions_list`; the connection itself
/// runs the regular custom-integration pipeline.
#[tauri::command]
pub async fn extensions_connect_tool(
    state: State<'_, ExtensionState>,
    candidate: ToolCandidate,
    approved_permissions: Vec<crate::extensions::manifest::Permission>,
) -> Result<ExtensionLockEntry, String> {
    if !candidate.available {
        return Err(format!("Tool {} is not available", candidate.name));
    }
    let expected = install::tool_binding_permissions();
    let mut approved = approved_permissions;
    approved.sort();
    approved.dedup();
    if approved != expected {
        return Err("Permission approval does not match the disclosure for this tool".to_string());
    }
    let entry = install::connect_tool(&state, candidate).await?;
    state.invalidate_provider_commands().await;
    Ok(entry)
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

/// Resolve a suggested tool id to its manifest, preferring shipped
/// recommendations and falling back to a convention-location manifest in
/// `<config>/floter/tools/`. Both connect through the same pipeline, so the
/// panel treats them identically.
fn resolve_suggested_tool<'a>(
    state: &'a ExtensionState,
    id: &str,
) -> Result<SuggestedTool<'a>, String> {
    if let Some(recommendation) = state
        .recommendations
        .iter()
        .find(|recommendation| recommendation.manifest.id == id)
    {
        return Ok(SuggestedTool::Recommended(recommendation));
    }
    tool_manifests::find(&tool_manifests::directory_for_root(&state.paths.root), id)
        .map(SuggestedTool::Manifest)
        .ok_or_else(|| format!("Recommended tool is not available: {id}"))
}

enum SuggestedTool<'a> {
    Recommended(&'a crate::extensions::recommendations::RecommendedTool),
    Manifest(tool_manifests::DiscoveredManifest),
}

impl SuggestedTool<'_> {
    fn manifest(&self) -> &ExtensionManifest {
        match self {
            Self::Recommended(recommendation) => &recommendation.manifest,
            Self::Manifest(tool) => &tool.manifest,
        }
    }
}

#[tauri::command]
pub fn extensions_recommended_permissions(
    state: State<'_, ExtensionState>,
    id: String,
    locale: Option<String>,
) -> Result<ExtensionPermissionReview, String> {
    let tool = resolve_suggested_tool(&state, &id)?;
    Ok(install::permission_review(
        tool.manifest(),
        locale.as_deref().unwrap_or("en"),
    ))
}

#[tauri::command]
pub async fn extensions_connect_recommended(
    state: State<'_, ExtensionState>,
    id: String,
    executable_path: Option<String>,
    approved_permissions: Option<Vec<Permission>>,
) -> Result<ExtensionLockEntry, String> {
    // Shipped recommendations and convention-location manifests share one
    // connection flow: both carry a full authored manifest, so connecting
    // materializes those bytes instead of regenerating a generic one.
    let tool = resolve_suggested_tool(&state, &id)?;
    let candidate = match executable_path.as_deref() {
        Some(path) => inspect_manifest_executable(tool.manifest(), path)?,
        None => discovered_system_candidate(&state, &id, tool.manifest(), true)?,
    };
    let candidate_path = candidate
        .locator
        .executable_path()
        .ok_or("Resolved system tool is not executable")?
        .to_string_lossy()
        .into_owned();
    let entry = match &tool {
        SuggestedTool::Recommended(recommendation) => {
            install::connect_recommended_tool(
                &state,
                recommendation,
                Some(&candidate_path),
                approved_permissions.as_deref(),
            )
            .await?
        }
        SuggestedTool::Manifest(manifest_tool) => {
            install::connect_manifest_tool(
                &state,
                &manifest_tool,
                Some(&candidate_path),
                approved_permissions.as_deref(),
            )
            .await?
        }
    };
    if let Err(error) = persist_tool_binding(&state, &id, &candidate) {
        let rollback = install::uninstall(&state, &id, false).await;
        return Err(format!(
            "Cannot persist system tool binding: {error}; connection rollback={:?}",
            rollback.err()
        ));
    }
    state.invalidate_provider_commands().await;
    Ok(entry)
}

#[tauri::command]
pub async fn extensions_reconnect_system(
    state: State<'_, ExtensionState>,
    id: String,
    executable_path: Option<String>,
) -> Result<ExtensionLockEntry, String> {
    reconnect_system(&state, &id, executable_path.as_deref()).await
}

async fn reconnect_system(
    state: &ExtensionState,
    id: &str,
    executable_path: Option<&str>,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let current = lock
        .extensions
        .get(id)
        .cloned()
        .ok_or_else(|| format!("Integration is not connected: {id}"))?;
    if current.runtime_ownership != ExtensionRuntimeOwnership::System {
        return Err(format!("Integration does not use a system runtime: {id}"));
    }
    let manifest = crate::extensions::ExtensionManifest::load(Path::new(&current.manifest_path))?;
    let candidate = match executable_path {
        Some(path) => inspect_manifest_executable(&manifest, path)?,
        None => discovered_system_candidate(state, id, &manifest, true)?,
    };
    let executable = candidate
        .locator
        .executable_path()
        .ok_or("Resolved system tool is not executable")?
        .to_path_buf();
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

    {
        let entry = lock
            .extensions
            .get_mut(id)
            .ok_or_else(|| format!("Integration is not connected: {id}"))?;
        entry.executable_path = executable.to_string_lossy().into_owned();
        entry.tool_version = tool_version;
        entry.updated_at = crate::extensions::lock::unix_now();
    }
    // A successful reconnect proves the runtime is bound again, so any
    // persisted broken state and operation error are cleared here as well.
    lock.clear_broken(id)?;
    let entry = lock.get(id)?.clone();
    let previous_tool_lock = {
        let mut tool_lock = state
            .tool_lock
            .lock()
            .map_err(|_| "Tool lock is unavailable".to_string())?;
        let previous = tool_lock.clone();
        if tool_lock.tools.contains_key(id) {
            tool_lock.reconnect(id, &candidate)?;
        } else {
            tool_lock.bind(id, &candidate);
        }
        if let Err(error) = tool_lock.save(&state.paths.tool_lock_file) {
            *tool_lock = previous;
            return Err(error);
        }
        previous
    };
    if let Err(error) = lock.save(&state.paths.lock_file) {
        let rollback = state
            .tool_lock
            .lock()
            .map_err(|_| "Tool lock is unavailable during rollback".to_string())
            .and_then(|mut tool_lock| {
                *tool_lock = previous_tool_lock;
                tool_lock.save(&state.paths.tool_lock_file)
            });
        return Err(format!(
            "Cannot persist reconnected integration: {error}; tool binding rollback={:?}",
            rollback.err()
        ));
    }
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
    // Commit the binding removal before touching the extension lock. If the
    // uninstall itself fails, restore the binding so the two state files do
    // not describe different installations.
    let previous_tool_lock = {
        let mut tool_lock = state
            .tool_lock
            .lock()
            .map_err(|_| "Tool lock is unavailable".to_string())?;
        let previous = tool_lock.clone();
        if tool_lock.remove(&id).is_some() {
            if let Err(error) = tool_lock.save(&state.paths.tool_lock_file) {
                *tool_lock = previous;
                return Err(error);
            }
        }
        previous
    };
    if let Err(error) = install::uninstall(&state, &id, remove_data.unwrap_or(false)).await {
        let rollback = state
            .tool_lock
            .lock()
            .map_err(|_| "Tool lock is unavailable during rollback".to_string())
            .and_then(|mut tool_lock| {
                *tool_lock = previous_tool_lock;
                tool_lock.save(&state.paths.tool_lock_file)
            });
        return Err(format!(
            "Cannot uninstall integration: {error}; tool binding rollback={:?}",
            rollback.err()
        ));
    }
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

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionRepairReport {
    pub id: String,
    pub repaired: bool,
    pub action: String,
    pub detail: String,
    pub entry: ExtensionLockEntry,
}

#[tauri::command]
pub async fn extensions_repair(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<ExtensionRepairReport, String> {
    match install::verify_installed(&state, &id).await {
        Ok(entry) => {
            // Verification passing clears any stale operation-error record so
            // the health section reflects the current, verified state.
            let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
            if lock.clear_broken(&id)? {
                let cleared = lock.get(&id)?.clone();
                lock.save(&state.paths.lock_file)?;
                return Ok(ExtensionRepairReport {
                    id,
                    repaired: false,
                    action: "verified".to_string(),
                    detail: "Manifest, runtime, and provider verification passed".to_string(),
                    entry: cleared,
                });
            }
            Ok(ExtensionRepairReport {
                id,
                repaired: false,
                action: "verified".to_string(),
                detail: "Manifest, runtime, and provider verification passed".to_string(),
                entry,
            })
        }
        Err(problem) => {
            let current = ExtensionsLock::load(&state.paths.lock_file)?
                .get(&id)?
                .clone();
            // Persist the failure as the structured broken state before any
            // repair attempt, so a crash mid-repair still leaves the reason
            // visible after restart.
            let code = install::classify_verify_error(&problem);
            let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
            lock.mark_broken(&id, &code, &problem)?;
            lock.save(&state.paths.lock_file)?;
            let action = if current.runtime_ownership == ExtensionRuntimeOwnership::System {
                match reconnect_system(&state, &id, None).await {
                    Ok(_) => "reconnected-system-runtime",
                    Err(repair_error) => {
                        let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
                        lock.mark_broken(&id, &code, &repair_error)?;
                        lock.save(&state.paths.lock_file)?;
                        return Err(format!("Cannot repair {id}: {repair_error}"));
                    }
                }
            } else {
                return Err(format!("Cannot repair {id}: {problem}"));
            };
            // Repair succeeded: restore the pre-broken enabled/disabled state
            // and drop the recorded error.
            let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
            lock.clear_broken(&id)?;
            lock.save(&state.paths.lock_file)?;
            let entry = lock.get(&id)?.clone();
            state.invalidate_provider_commands().await;
            Ok(ExtensionRepairReport {
                id,
                repaired: true,
                action: action.to_string(),
                detail: problem,
                entry,
            })
        }
    }
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
    let response = if matches!(
        entry.provider_kind,
        ExtensionProviderKind::StaticDescriptor | ExtensionProviderKind::BundledStatic
    ) {
        let (description, _) = crate::extensions::registry::static_description(&entry)?;
        ProviderResponse {
            description,
            runtime_available: crate::extensions::registry::runtime_available(&entry),
            cached: true,
            stderr: None,
        }
    } else {
        let mut invocation = crate::extensions::registry::provider_invocation(&entry)?;
        let _ = config::apply_persisted_configuration(&state.paths.data, &mut invocation)?;
        state
            .provider
            .describe(&invocation, force.unwrap_or(false))
            .await?
    };
    if response.runtime_available {
        clear_broken_after_success(&state, &id).await?;
    }
    Ok(response)
}

async fn clear_broken_after_success(state: &ExtensionState, id: &str) -> Result<(), String> {
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if lock.get(id)?.state != ExtensionStateKind::Broken {
        return Ok(());
    }
    if lock.clear_broken(id)? {
        lock.save(&state.paths.lock_file)?;
    }
    Ok(())
}

#[tauri::command]
pub async fn extensions_diagnose(
    state: State<'_, ExtensionState>,
    id: String,
) -> Result<DiagnoseResponse, String> {
    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();
    if matches!(
        entry.provider_kind,
        ExtensionProviderKind::StaticDescriptor | ExtensionProviderKind::BundledStatic
    ) {
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
    use crate::extensions::capability_probe::CapabilityProbe;
    use crate::extensions::health::HealthReport;

    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(&id)?
        .clone();

    // Get the tool's executable path
    let executable = std::path::PathBuf::from(&entry.executable_path);
    if !executable.exists() {
        return Err(format!(
            "Tool executable not found: {}",
            executable.display()
        ));
    }

    // Build default probes: --version and --help
    let version_probe = CapabilityProbe::version();
    let help_probe = CapabilityProbe::help();

    let probes = [version_probe, help_probe];
    let required = [true, false]; // version required, help optional

    let health_dir = state.paths.data.join(&id);
    let mut report = HealthReport::new(Default::default());

    for (i, probe) in probes.iter().enumerate() {
        let start = std::time::Instant::now();
        let timeout = std::time::Duration::from_secs(5);

        match crate::extensions::probe_runner::run_single_probe(&executable, &probe.args, timeout)
            .await
        {
            Ok(result) => {
                let duration = start.elapsed();
                if result.passed {
                    report.record_pass(&probe.id, duration, result.exit_code);
                } else {
                    report.record_failure(
                        &probe.id,
                        duration,
                        result.exit_code,
                        result.stderr,
                        !required[i],
                    );
                }
            }
            Err(error) => {
                let duration = start.elapsed();
                report.record_failure(&probe.id, duration, None, error, !required[i]);
            }
        }
    }

    let required_ids: Vec<String> = required
        .iter()
        .enumerate()
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
    use crate::extensions::session_restore::{
        ResolvedSession, RestorePolicy, SessionResolveRequest, SessionResolver,
    };

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
    let session = session_resolver.resolve(SessionResolveRequest {
        tool_id: id.clone(),
        tool_version: tool_version.clone(),
        argv: argv.clone(),
        cwd: resolved_cwd.clone(),
        environment_refs: vec!["profile:default".to_string()],
        terminal_profile: None,
        restore_policy,
    })?;

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

#[cfg(test)]
mod tests {
    use super::*;

    const MANIFEST_JSON: &str = r#"{
        "schemaVersion": "2.0",
        "id": "local.demo.tool",
        "name": "Demo Tool",
        "publisher": { "id": "demo", "name": "Demo" },
        "compatibility": { "floter": ">=0.1.0", "providerProtocol": "^1.0" },
        "distribution": { "type": "local" },
        "runtime": {
            "type": "system",
            "executableNames": ["demo-tool"],
            "versionArgs": ["--version"]
        },
        "provider": {
            "type": "static-descriptor",
            "descriptor": "provider-description.json",
            "argsPrefix": []
        },
        "permissions": ["environment"]
    }"#;

    #[cfg(unix)]
    fn write_demo_tools_dir(root: &Path) -> std::path::PathBuf {
        use std::os::unix::fs::PermissionsExt;

        let tools = tool_manifests::directory_for_root(root);
        std::fs::create_dir_all(&tools).unwrap();
        std::fs::write(tools.join("demo.json"), MANIFEST_JSON).unwrap();
        let executable = root.join("demo-tool");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        tools
    }

    #[cfg(unix)]
    fn demo_candidate(root: &Path) -> ToolCandidate {
        inventory::executable_candidate(&root.join("demo-tool"), "demo-tool")
    }

    #[cfg(unix)]
    #[test]
    fn manifest_suggestions_skip_connected_integrations() {
        let directory = tempfile::tempdir().unwrap();
        let tools = write_demo_tools_dir(directory.path());
        let candidate = demo_candidate(directory.path());
        let candidates = vec![candidate.clone()];

        // Not connected yet: the manifest becomes a suggestion row.
        let rows = manifest_suggestions(&ExtensionsLock::default(), &[], &candidates, &tools);
        assert_eq!(rows.len(), 1);
        assert_eq!(rows[0].0.manifest.id, "local.demo.tool");
        assert!(!rows[0].1.is_empty());

        // Connected by id: never suggested twice.
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(
            "local.demo.tool".into(),
            serde_json::from_value(serde_json::json!({
                "id": "local.demo.tool",
                "name": "Demo Tool",
                "publisherId": "demo",
                "publisherName": "Demo",
                "distributionSource": "local",
                "runtimeOwnership": "system",
                "state": "enabled",
                "enabled": true,
                "packageName": null,
                "packageVersion": "local",
                "toolVersion": null,
                "integrity": null,
                "currentVersion": "local",
                "previousVersion": null,
                "manifestPath": "/tmp/floter.extension.json",
                "executablePath": directory.path().join("demo-tool").to_string_lossy(),
                "installedAt": 1,
                "updatedAt": 1
            }))
            .unwrap(),
        );
        assert!(manifest_suggestions(&lock, &[], &candidates, &tools).is_empty());

        // A different id bound to the same executable path is also skipped.
        lock.extensions.remove("local.demo.tool").unwrap();
        let mut entry: ExtensionLockEntry = serde_json::from_value(serde_json::json!({
            "id": "other.tool",
            "name": "Other",
            "publisherId": "demo",
            "publisherName": "Demo",
            "distributionSource": "local",
            "runtimeOwnership": "system",
            "state": "enabled",
            "enabled": true,
            "packageName": null,
            "packageVersion": "local",
            "toolVersion": null,
            "integrity": null,
            "currentVersion": "local",
            "previousVersion": null,
            "manifestPath": "/tmp/floter.extension.json",
            "executablePath": directory.path().join("demo-tool").to_string_lossy(),
            "installedAt": 1,
            "updatedAt": 1
        }))
        .unwrap();
        entry.executable_path = candidate
            .locator
            .executable_path()
            .unwrap()
            .to_string_lossy()
            .into_owned();
        lock.extensions.insert("other.tool".into(), entry);
        assert!(manifest_suggestions(&lock, &[], &candidates, &tools).is_empty());
    }
}
