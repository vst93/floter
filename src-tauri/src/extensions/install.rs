use crate::extensions::lock::{
    unix_now, validate_id, ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
    ExtensionRuntimeOwnership, ExtensionStateKind, ExtensionsLock,
};
use crate::extensions::manifest::{
    validate_relative_path, Compatibility, Distribution, ExtensionManifest, Permission, PlatformOs,
    PlatformTarget, ProviderConfig, ProviderKind, Publisher, Runtime, ScriptLanguage,
    SignatureAlgorithm, SignatureConfig,
};
use crate::extensions::official_index;
use crate::extensions::provider::ProviderInvocation;
use crate::extensions::transaction::{commit_lock, commit_version};
use crate::extensions::ExtensionState;
use base64::Engine;
use ed25519_dalek::{Signature, VerifyingKey};
use flate2::read::GzDecoder;
use semver::{Version, VersionReq};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256, Sha384, Sha512};
use std::collections::BTreeMap;
use std::io::{Cursor, Write};
use std::path::{Component, Path, PathBuf};
use std::process::Stdio;
use std::time::Duration;

const REGISTRY_URL: &str = "https://registry.npmjs.org/";
const MAX_TARBALL_BYTES: usize = 128 * 1024 * 1024;
const MAX_SIGNATURE_BYTES: usize = 4 * 1024;
const MAX_REGISTRY_METADATA_BYTES: usize = 16 * 1024 * 1024;
const MAX_SEARCH_RESPONSE_BYTES: usize = 4 * 1024 * 1024;
const MAX_ARCHIVE_ENTRIES: usize = 100_000;

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionInstallRequest {
    pub source: InstallSource,
    pub package: Option<String>,
    pub version: Option<String>,
    pub manifest_path: Option<String>,
    pub executable_path: Option<String>,
    #[serde(default)]
    pub approved_permissions: Option<Vec<Permission>>,
}

#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum InstallSource {
    Npm,
    Linked,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionSearchResult {
    pub package: String,
    pub version: String,
    pub description: String,
    pub publisher: Option<String>,
    pub homepage: Option<String>,
    pub verified: bool,
    pub downloads: u64,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionPermissionReview {
    pub extension_id: String,
    pub extension_name: String,
    pub permissions: Vec<PermissionSummary>,
    pub publisher_signed: bool,
    pub official_verified: bool,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PermissionSummary {
    pub permission: Permission,
    pub title: String,
    pub description: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CustomIntegrationRequest {
    pub id: String,
    pub name: String,
    pub command: String,
    pub version: String,
    #[serde(default)]
    pub executable_path: String,
    #[serde(default = "default_custom_mode")]
    pub mode: String,
    #[serde(default)]
    pub script_language: Option<ScriptLanguage>,
    #[serde(default)]
    pub script_content: Option<String>,
    #[serde(default)]
    pub args_prefix: Vec<String>,
    #[serde(default)]
    pub version_args: Vec<String>,
    #[serde(default)]
    pub permissions: Vec<Permission>,
    #[serde(default)]
    pub platforms: Vec<PlatformOs>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CustomIntegrationDefinition {
    pub id: String,
    pub name: String,
    pub command: String,
    pub version: String,
    pub executable_path: String,
    pub mode: String,
    pub script_language: Option<ScriptLanguage>,
    pub script_content: Option<String>,
    pub args_prefix: Vec<String>,
    pub version_args: Vec<String>,
    pub permissions: Vec<Permission>,
    pub platforms: Vec<PlatformOs>,
}

fn default_custom_mode() -> String {
    "executable".to_string()
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PathExecutable {
    pub name: String,
    pub path: String,
}

#[derive(Debug, Clone, Deserialize)]
struct RegistryMetadata {
    #[serde(rename = "dist-tags", default)]
    dist_tags: BTreeMap<String, String>,
    #[serde(default)]
    versions: BTreeMap<String, RegistryVersion>,
}

#[derive(Debug, Clone, Deserialize)]
struct RegistryVersion {
    name: String,
    version: String,
    dist: RegistryDist,
}

#[derive(Debug, Clone, Deserialize)]
struct RegistryDist {
    tarball: String,
    integrity: Option<String>,
}

#[derive(Debug, Deserialize)]
struct PackageJson {
    name: String,
    version: String,
    #[serde(default)]
    keywords: Vec<String>,
    floter: Option<PackageFloter>,
}

#[derive(Debug, Deserialize)]
struct PackageFloter {
    manifest: String,
}

#[derive(Debug, Deserialize)]
struct RegistrySearchResponse {
    #[serde(default)]
    objects: Vec<RegistrySearchObject>,
}

#[derive(Debug, Deserialize)]
struct RegistrySearchObject {
    #[serde(default)]
    downloads: RegistryDownloads,
    package: RegistrySearchPackage,
}

#[derive(Debug, Default, Deserialize)]
struct RegistryDownloads {
    #[serde(default)]
    weekly: u64,
}

#[derive(Debug, Deserialize)]
struct RegistrySearchPackage {
    name: String,
    version: String,
    #[serde(default)]
    description: String,
    publisher: Option<RegistryPublisher>,
    #[serde(default)]
    links: BTreeMap<String, String>,
}

#[derive(Debug, Deserialize)]
struct RegistryPublisher {
    username: Option<String>,
}

#[derive(Debug, Clone, Copy)]
enum InstallationPhase {
    Resolving,
    Downloading,
    Verifying,
    Installing,
    Complete,
}

impl InstallationPhase {
    fn may_transition_to(self, next: Self) -> bool {
        use InstallationPhase::*;
        matches!(
            (self, next),
            (Resolving, Downloading)
                | (Downloading, Verifying)
                | (Verifying, Installing)
                | (Installing, Complete)
        )
    }
}

struct InstallationTransaction {
    phase: InstallationPhase,
}

impl InstallationTransaction {
    fn new() -> Self {
        Self {
            phase: InstallationPhase::Resolving,
        }
    }

    fn advance(&mut self, next: InstallationPhase) -> Result<(), String> {
        if !self.phase.may_transition_to(next) {
            return Err(format!(
                "Invalid installation transition: {:?} -> {:?}",
                self.phase, next
            ));
        }
        self.phase = next;
        Ok(())
    }
}

pub async fn install(
    state: &ExtensionState,
    request: ExtensionInstallRequest,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    match request.source {
        InstallSource::Npm => {
            let package = request
                .package
                .as_deref()
                .ok_or("NPM installation requires a package name")?;
            install_managed(
                state,
                package,
                request.version.as_deref(),
                None,
                request.approved_permissions.as_deref(),
            )
            .await
        }
        InstallSource::Linked => install_linked(state, request).await,
    }
}

pub(crate) async fn install_imported_managed_locked(
    state: &ExtensionState,
    extension_id: &str,
    package: &str,
    version: &str,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    install_managed(
        state,
        package,
        Some(version),
        Some(extension_id),
        approved_permissions,
    )
    .await
}

pub(crate) fn commit_preflight_managed(
    state: &ExtensionState,
    staged_state: &ExtensionState,
    mut entry: ExtensionLockEntry,
) -> Result<ExtensionLockEntry, String> {
    let staged_version = staged_state
        .paths
        .extensions
        .join(&entry.id)
        .join("versions")
        .join(&entry.current_version);
    let target = state
        .paths
        .extensions
        .join(&entry.id)
        .join("versions")
        .join(&entry.current_version);
    let old = ExtensionsLock::load(&state.paths.lock_file)?
        .extensions
        .get(&entry.id)
        .cloned();
    let rewrite = |value: &str| -> Result<String, String> {
        let relative = Path::new(value)
            .strip_prefix(&staged_version)
            .map_err(|_| "Preflight artifact escaped its verified version directory")?;
        Ok(target.join(relative).to_string_lossy().into_owned())
    };
    entry.manifest_path = rewrite(&entry.manifest_path)?;
    if Path::new(&entry.executable_path).starts_with(&staged_version) {
        entry.executable_path = rewrite(&entry.executable_path)?;
    } else if entry.runtime_ownership != ExtensionRuntimeOwnership::System {
        return Err(
            "Bundled preflight executable escaped its verified version directory".to_string(),
        );
    }
    entry.runtime_root = entry.runtime_root.as_deref().map(rewrite).transpose()?;
    if let Some(old) = &old {
        entry.installed_at = old.installed_at;
        entry.previous_version = Some(old.current_version.clone());
        entry.previous_signature_verified = Some(old.signature_verified);
        entry.previous_official_verified = Some(old.official_verified);
        entry.enabled = old.enabled;
        entry.state = old.state;
        entry.pinned = old.pinned;
    }
    std::fs::create_dir_all(target.parent().ok_or("Invalid extension target")?)
        .map_err(|error| format!("Cannot create extension versions directory: {error}"))?;
    commit_version(state, old.as_ref(), &entry, &staged_version, &target)?;
    Ok(entry)
}

pub async fn create_custom_integration(
    state: &ExtensionState,
    request: CustomIntegrationRequest,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    create_custom_integration_locked(state, request).await
}

async fn create_custom_integration_locked(
    state: &ExtensionState,
    request: CustomIntegrationRequest,
) -> Result<ExtensionLockEntry, String> {
    let id = request.id.trim().to_ascii_lowercase();
    validate_id(&id)?;
    let name = request.name.trim();
    if name.is_empty() || name.chars().count() > 80 {
        return Err("Custom integration name must contain 1 to 80 characters".to_string());
    }
    let command = request.command.trim().to_ascii_lowercase();
    if command.is_empty()
        || command.len() > 64
        || !command.bytes().all(|byte| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || matches!(byte, b'-' | b'_')
        })
        || !command
            .bytes()
            .next()
            .is_some_and(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit())
    {
        return Err("Command must start with a letter or number and contain only lowercase letters, numbers, hyphens, or underscores".to_string());
    }
    Version::parse(request.version.trim())
        .map_err(|error| format!("Invalid custom integration version: {error}"))?;
    if request.platforms.is_empty() {
        return Err("Select at least one supported platform".to_string());
    }
    let script_mode = request.mode == "script";
    if request.mode != "script" && request.mode != "executable" {
        return Err("Custom integration mode must be executable or script".to_string());
    }
    let script_language = request.script_language.unwrap_or(ScriptLanguage::Shell);
    let executable = if script_mode {
        PathBuf::new()
    } else {
        let path = PathBuf::from(request.executable_path.trim());
        if !is_linked_executable(&path) {
            return Err(format!(
                "System executable is not usable: {}",
                path.display()
            ));
        }
        path
    };
    if script_mode
        && request
            .script_content
            .as_deref()
            .unwrap_or("")
            .trim()
            .is_empty()
    {
        return Err("Custom provider script cannot be empty".to_string());
    }
    if !script_mode && request.script_language.is_some() {
        return Err("Script language is only valid for script integrations".to_string());
    }
    let executable_name = executable
        .file_name()
        .and_then(|value| value.to_str())
        .unwrap_or("custom-tool")
        .to_string();
    let manifest = ExtensionManifest {
        schema_version: "2.0".into(),
        id: id.clone(),
        name: name.to_string(),
        description: if script_mode {
            "Local script integration".to_string()
        } else {
            format!("Local integration for {executable_name}")
        },
        homepage: None,
        icon: None,
        publisher: Publisher {
            id: "local-user".into(),
            name: "Local user".into(),
        },
        compatibility: Compatibility {
            floter: format!(">={}", env!("CARGO_PKG_VERSION")),
            provider_protocol: "^1.0".into(),
        },
        distribution: Distribution::Local,
        runtime: if script_mode {
            Runtime::Script {
                language: script_language,
                path: format!("provider.{}", script_extension(script_language)),
                version_args: request.version_args.clone(),
            }
        } else {
            Runtime::System {
                executable_names: vec![executable_name],
                version_args: request.version_args.clone(),
            }
        },
        provider: ProviderConfig {
            kind: ProviderKind::StaticDescriptor,
            descriptor: Some("provider-description.json".to_string()),
            args_prefix: Vec::new(),
            describe_timeout_ms: 5_000,
            complete_timeout_ms: 800,
            environment: BTreeMap::new(),
        },
        signatures: None,
        platform_overrides: BTreeMap::new(),
        permissions: request.permissions.clone(),
        lifecycle: crate::extensions::lifecycle::ToolLifecycle::default(),
        platforms: request.platforms.clone(),
    };
    let manifest_bytes = serde_json::to_vec(&manifest)
        .map_err(|error| format!("Cannot serialize custom integration manifest: {error}"))?;
    ExtensionManifest::parse(&manifest_bytes)?;

    let package_root = state.paths.data.join(&id).join("integration");
    if package_root.exists() {
        return Err(format!("Custom integration files already exist for {id}"));
    }
    let data_root = package_root
        .parent()
        .ok_or("Custom integration directory has no parent")?;
    std::fs::create_dir_all(data_root)
        .map_err(|error| format!("Cannot create custom integration data directory: {error}"))?;
    std::fs::create_dir(&package_root).map_err(|error| {
        format!("Cannot reserve custom integration directory for {id}: {error}")
    })?;
    let manifest_path = package_root.join("floter.extension.json");
    let package_path = package_root.join("package.json");
    let descriptor_path = package_root.join("provider-description.json");
    let script_path = package_root.join(format!("provider.{}", script_extension(script_language)));
    let write_result = (|| -> Result<(), String> {
        let mut manifest_bytes = serde_json::to_vec_pretty(&manifest)
            .map_err(|error| format!("Cannot serialize custom integration manifest: {error}"))?;
        manifest_bytes.push(b'\n');
        std::fs::write(&manifest_path, manifest_bytes)
            .map_err(|error| format!("Cannot write custom integration manifest: {error}"))?;
        let mut package_bytes = serde_json::to_vec_pretty(&serde_json::json!({
            "name": format!("floter-local-{}", id.replace(['.', '_'], "-")),
            "version": request.version.trim(),
            "private": true,
            "keywords": ["floter-extension"],
            "floter": { "manifest": "floter.extension.json" }
        }))
        .map_err(|error| format!("Cannot serialize custom integration package: {error}"))?;
        package_bytes.push(b'\n');
        std::fs::write(&package_path, package_bytes)
            .map_err(|error| format!("Cannot write custom integration package: {error}"))?;
        let mut descriptor_bytes = serde_json::to_vec_pretty(&serde_json::json!({
            "protocolVersion": "1.0",
            "provider": {
                "id": id,
                "name": name,
                "version": request.version.trim(),
                "description": manifest.description
            },
            "commands": [{
                "id": command,
                "name": name,
                "description": manifest.description,
                "aliases": [],
                "keywords": [],
                "execution": {
                    "program": "self",
                    "argsPrefix": request.args_prefix,
                    "mode": "pty",
                    "workingDirectory": "current"
                },
                "arguments": []
            }]
        }))
        .map_err(|error| format!("Cannot serialize custom provider description: {error}"))?;
        descriptor_bytes.push(b'\n');
        std::fs::write(&descriptor_path, descriptor_bytes)
            .map_err(|error| format!("Cannot write custom provider description: {error}"))?;
        if script_mode {
            std::fs::write(
                &script_path,
                request.script_content.as_deref().unwrap_or(""),
            )
            .map_err(|error| format!("Cannot write custom provider script: {error}"))?;
            make_executable(&script_path)?;
        }
        Ok(())
    })();
    if let Err(error) = write_result {
        let _ = std::fs::remove_dir_all(&package_root);
        return Err(error);
    }

    let result = install_linked(
        state,
        ExtensionInstallRequest {
            source: InstallSource::Linked,
            package: None,
            version: None,
            manifest_path: Some(package_root.to_string_lossy().into_owned()),
            executable_path: (!script_mode).then(|| executable.to_string_lossy().into_owned()),
            approved_permissions: Some(request.permissions),
        },
    )
    .await;
    if result.is_err() {
        let _ = std::fs::remove_dir_all(&package_root);
    }
    result
}

pub fn is_generated_custom_integration(entry: &ExtensionLockEntry) -> bool {
    if entry.distribution_source != ExtensionDistributionSource::Local
        || entry.publisher_id != "local-user"
        || entry.provider_kind != ExtensionProviderKind::StaticDescriptor
    {
        return false;
    }
    let Some(root) = Path::new(&entry.manifest_path).parent() else {
        return false;
    };
    root.file_name().and_then(|name| name.to_str()) == Some("integration")
        && root
            .parent()
            .and_then(Path::file_name)
            .and_then(|name| name.to_str())
            == Some(entry.id.as_str())
}

pub fn custom_integration_definition(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<CustomIntegrationDefinition, String> {
    let entry = ExtensionsLock::load(&state.paths.lock_file)?
        .get(extension_id)?
        .clone();
    if !is_generated_custom_integration(&entry) {
        return Err(format!(
            "Integration {extension_id} is not editable in Floter"
        ));
    }
    let manifest_path = Path::new(&entry.manifest_path);
    let root = manifest_path
        .parent()
        .ok_or("Custom integration manifest has no parent directory")?;
    let manifest = ExtensionManifest::load(manifest_path)?;
    let descriptor_path = root.join(
        manifest
            .provider
            .descriptor
            .as_deref()
            .ok_or("Custom integration has no static descriptor")?,
    );
    let description = crate::extensions::provider::ProviderDescription::parse(
        &std::fs::read(&descriptor_path)
            .map_err(|error| format!("Cannot read custom integration descriptor: {error}"))?,
    )?;
    let command = description
        .commands
        .first()
        .ok_or("Custom integration descriptor has no command")?;
    if description.commands.len() != 1 {
        return Err("Only single-command custom integrations can be edited visually".to_string());
    }
    let (mode, executable_path, script_language, script_content, version_args) = match &manifest
        .runtime
    {
        Runtime::System { version_args, .. } => (
            "executable".to_string(),
            entry.executable_path.clone(),
            None,
            None,
            version_args.clone(),
        ),
        Runtime::Script {
            language,
            path,
            version_args,
        } => (
            "script".to_string(),
            String::new(),
            Some(*language),
            Some(
                std::fs::read_to_string(root.join(path))
                    .map_err(|error| format!("Cannot read custom integration script: {error}"))?,
            ),
            version_args.clone(),
        ),
        Runtime::Bundled { .. } => {
            return Err("Custom integrations cannot use a bundled runtime".to_string())
        }
    };
    Ok(CustomIntegrationDefinition {
        id: manifest.id,
        name: manifest.name,
        command: command.id.clone(),
        version: description.provider.version,
        executable_path,
        mode,
        script_language,
        script_content,
        args_prefix: command.execution.args_prefix.clone(),
        version_args,
        permissions: manifest.permissions,
        platforms: manifest.platforms,
    })
}

pub async fn update_custom_integration(
    state: &ExtensionState,
    extension_id: &str,
    request: CustomIntegrationRequest,
) -> Result<ExtensionLockEntry, String> {
    validate_id(extension_id)?;
    if request.id.trim().to_ascii_lowercase() != extension_id {
        return Err("Custom integration ID cannot be changed after creation".to_string());
    }
    let _guard = state.mutation_lock.lock().await;
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let current = lock.get(extension_id)?.clone();
    if !is_generated_custom_integration(&current) {
        return Err(format!(
            "Integration {extension_id} is not editable in Floter"
        ));
    }
    drop(lock);

    let root = state.paths.data.join(extension_id).join("integration");
    let backup = tempfile::Builder::new()
        .prefix(&format!(".{extension_id}-editing-"))
        .tempdir_in(state.paths.data.join(extension_id))
        .map_err(|error| format!("Cannot prepare custom integration update: {error}"))?;
    let backup_path = backup.path().to_path_buf();
    backup
        .close()
        .map_err(|error| format!("Cannot prepare custom integration backup: {error}"))?;
    std::fs::rename(&root, &backup_path)
        .map_err(|error| format!("Cannot stage custom integration update: {error}"))?;
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    lock.extensions.remove(extension_id);
    if let Err(error) = lock.save(&state.paths.lock_file) {
        let _ = std::fs::rename(&backup_path, &root);
        return Err(format!(
            "Cannot stage custom integration lock update: {error}"
        ));
    }
    drop(lock);

    let result = create_custom_integration_locked(state, request).await;
    if result.is_ok() {
        let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
        if let Some(updated) = lock.extensions.get_mut(extension_id) {
            updated.enabled = current.enabled;
            updated.state = if current.enabled {
                ExtensionStateKind::Enabled
            } else {
                ExtensionStateKind::Disabled
            };
            updated.installed_at = current.installed_at;
            if let Err(error) = lock.save(&state.paths.lock_file) {
                let _ = std::fs::remove_dir_all(&root);
                let files = std::fs::rename(&backup_path, &root);
                let mut restore = ExtensionsLock::load(&state.paths.lock_file)?;
                restore.extensions.insert(extension_id.to_string(), current);
                let restored_lock = restore.save(&state.paths.lock_file);
                return Err(format!(
                    "Cannot finalize custom integration update: {error}; rollback files={:?}, lock={:?}",
                    files.err(),
                    restored_lock.err()
                ));
            }
        }
        let _ = std::fs::remove_dir_all(&backup_path);
        return Ok(lock.get(extension_id)?.clone());
    }

    let _ = std::fs::remove_dir_all(&root);
    let restore_files = std::fs::rename(&backup_path, &root)
        .map_err(|error| format!("Cannot restore custom integration files: {error}"));
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    lock.extensions.insert(extension_id.to_string(), current);
    let restore_lock = lock.save(&state.paths.lock_file);
    match (result, restore_files, restore_lock) {
        (Err(error), Ok(()), Ok(())) => Err(error),
        (Err(error), files, lock) => Err(format!(
            "{error}; rollback failed: files={:?}, lock={:?}",
            files.err(),
            lock.err()
        )),
        _ => unreachable!(),
    }
}

fn script_extension(language: ScriptLanguage) -> &'static str {
    match language {
        ScriptLanguage::Js => "js",
        ScriptLanguage::Shell => "sh",
        ScriptLanguage::Powershell => "ps1",
    }
}

pub(crate) fn find_script_interpreter(language: ScriptLanguage) -> Result<PathBuf, String> {
    let names: &[&str] = match language {
        ScriptLanguage::Js => &["node"],
        ScriptLanguage::Shell => &["sh"],
        ScriptLanguage::Powershell => &["pwsh", "powershell"],
    };
    let path = std::env::var_os("PATH").ok_or("PATH is not set")?;
    for directory in std::env::split_paths(&path) {
        for name in names {
            for candidate in linked_candidate_names(name) {
                let path = directory.join(candidate);
                if is_linked_executable(&path) {
                    return Ok(path);
                }
            }
        }
    }
    Err(format!(
        "Script interpreter is not available: {}",
        names.join(" or ")
    ))
}

pub fn search_path_executables(query: &str, limit: usize) -> Vec<PathExecutable> {
    let needle = query.trim().to_ascii_lowercase();
    let Some(path) = std::env::var_os("PATH") else {
        return Vec::new();
    };
    let mut seen = std::collections::HashSet::new();
    let mut matches = Vec::new();
    for directory in std::env::split_paths(&path) {
        let Ok(entries) = std::fs::read_dir(directory) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if !is_linked_executable(&path) {
                continue;
            }
            let Some(name) = path.file_name().and_then(|value| value.to_str()) else {
                continue;
            };
            let key = name.to_ascii_lowercase();
            if !seen.insert(key.clone()) {
                continue;
            }
            let Some(score) = fuzzy_executable_score(&key, &needle) else {
                continue;
            };
            matches.push((score, name.to_string(), path));
        }
    }
    matches.sort_by(|left, right| {
        left.0
            .cmp(&right.0)
            .then_with(|| left.1.len().cmp(&right.1.len()))
            .then_with(|| left.1.cmp(&right.1))
    });
    matches
        .into_iter()
        .take(limit.clamp(1, 50))
        .map(|(_, name, path)| PathExecutable {
            name,
            path: path.to_string_lossy().into_owned(),
        })
        .collect()
}

fn fuzzy_executable_score(candidate: &str, query: &str) -> Option<usize> {
    if query.is_empty() {
        return Some(3);
    }
    if candidate == query {
        return Some(0);
    }
    if candidate.starts_with(query) {
        return Some(1);
    }
    if candidate.contains(query) {
        return Some(2);
    }
    let mut query_chars = query.chars();
    let mut current = query_chars.next()?;
    for character in candidate.chars() {
        if character == current {
            let Some(next) = query_chars.next() else {
                return Some(3);
            };
            current = next;
        }
    }
    None
}

pub async fn connect_bundled(
    state: &ExtensionState,
    extension_id: &str,
    executable_path: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let adapters = crate::extensions::static_adapter::load_bundled()?;
    let adapter = adapters
        .iter()
        .find(|adapter| adapter.manifest.id == extension_id)
        .cloned()
        .ok_or_else(|| format!("Bundled integration is not available: {extension_id}"))?;
    validate_permission_approval(&adapter.manifest.permissions, approved_permissions)?;
    let executable = executable_path
        .map(PathBuf::from)
        .unwrap_or_else(|| adapter.invocation.executable.clone());
    if !crate::extensions::static_adapter::is_linked_executable(&executable) {
        return Err(format!(
            "System tool is not available at {}",
            executable.display()
        ));
    }

    let _guard = state.mutation_lock.lock().await;
    connect_bundled_locked(state, extension_id, executable_path, approved_permissions).await
}

pub(crate) async fn connect_bundled_locked(
    state: &ExtensionState,
    extension_id: &str,
    executable_path: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let adapters = crate::extensions::static_adapter::load_bundled()?;
    let adapter = adapters
        .iter()
        .find(|adapter| adapter.manifest.id == extension_id)
        .cloned()
        .ok_or_else(|| format!("Bundled integration is not available: {extension_id}"))?;
    validate_permission_approval(&adapter.manifest.permissions, approved_permissions)?;
    let executable = executable_path
        .map(PathBuf::from)
        .unwrap_or_else(|| adapter.invocation.executable.clone());
    if !crate::extensions::static_adapter::is_linked_executable(&executable) {
        return Err(format!(
            "System tool is not available at {}",
            executable.display()
        ));
    }
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if lock.extensions.contains_key(extension_id) {
        return Err(format!("Integration is already connected: {extension_id}"));
    }
    let resolved = adapter
        .manifest
        .clone()
        .resolve(PlatformTarget::current()?)?;
    let tool_version =
        linked_tool_version(&adapter.manifest, &resolved.provider, &executable).await;
    let manifest_dir = state.paths.data.join(extension_id).join("integration");
    std::fs::create_dir_all(&manifest_dir)
        .map_err(|error| format!("Cannot create integration directory: {error}"))?;
    let manifest_path = manifest_dir.join("floter.extension.json");
    let mut manifest_bytes = serde_json::to_vec_pretty(&adapter.manifest)
        .map_err(|error| format!("Cannot serialize bundled integration manifest: {error}"))?;
    manifest_bytes.push(b'\n');
    std::fs::write(&manifest_path, manifest_bytes)
        .map_err(|error| format!("Cannot write bundled integration manifest: {error}"))?;

    let now = unix_now();
    let integration_version = env!("CARGO_PKG_VERSION").to_string();
    let entry = ExtensionLockEntry {
        id: adapter.manifest.id,
        name: adapter.manifest.name,
        publisher_id: adapter.manifest.publisher.id,
        publisher_name: adapter.manifest.publisher.name,
        distribution_source: ExtensionDistributionSource::BuiltIn,
        runtime_ownership: ExtensionRuntimeOwnership::System,
        provider_kind: ExtensionProviderKind::BundledStatic,
        state: ExtensionStateKind::Enabled,
        enabled: true,
        package_name: None,
        package_version: integration_version.clone(),
        tool_version,
        integrity: None,
        signature_verified: false,
        previous_signature_verified: None,
        official_verified: false,
        previous_official_verified: None,
        current_version: integration_version,
        previous_version: None,
        manifest_path: manifest_path.to_string_lossy().into_owned(),
        executable_path: executable.to_string_lossy().into_owned(),
        runtime_root: None,
        installed_at: now,
        updated_at: now,
        pinned: false,
        channel: "bundled".to_string(),
    };
    lock.extensions.insert(entry.id.clone(), entry.clone());
    lock.save(&state.paths.lock_file)?;
    Ok(entry)
}

pub async fn permissions_summary(
    state: &ExtensionState,
    request: &ExtensionInstallRequest,
    locale: &str,
) -> Result<ExtensionPermissionReview, String> {
    let mut trust = (false, false);
    let manifest = match request.source {
        InstallSource::Npm => {
            let package = request
                .package
                .as_deref()
                .ok_or("NPM installation requires a package name")?;
            validate_package_name(package)?;
            let official_index = official_index::fetch(state).await.ok();
            let version =
                resolve_registry_version(state, package, request.version.as_deref()).await?;
            let bytes = download_tarball(state, &version.dist).await?;
            let staging = tempfile::tempdir()
                .map_err(|error| format!("Cannot create permission review directory: {error}"))?;
            safe_unpack(&bytes, staging.path())?;
            let (package_json, manifest_path) = load_package_entry(staging.path())?;
            validate_package_entry(&package_json, &version, true)?;
            let manifest = ExtensionManifest::load(&manifest_path)?;
            if let Some(signatures) = manifest.signatures.as_ref() {
                trust.1 = official_index.as_ref().is_some_and(|index| {
                    index.authorizes(
                        &manifest.id,
                        package,
                        &manifest.publisher.id,
                        Some(signatures),
                    )
                });
                let signature = download_signature(state, signatures).await?;
                trust.1 = verify_official_tarball(trust.1, &bytes, &signature, signatures)?;
                trust.0 = true;
            }
            manifest
        }
        InstallSource::Linked => {
            let source = request
                .manifest_path
                .as_deref()
                .ok_or("Linked installation requires manifestPath")?;
            let path = PathBuf::from(source);
            if path.is_dir() {
                let (_, manifest_path) = load_package_entry(&path)?;
                ExtensionManifest::load(&manifest_path)?
            } else {
                ExtensionManifest::load(&path)?
            }
        }
    };
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let mut review = permission_review(&manifest, locale);
    review.publisher_signed = trust.0;
    review.official_verified = trust.1 && trust.0;
    Ok(review)
}

pub(crate) fn validate_permission_approval(
    requested: &[Permission],
    approved: Option<&[Permission]>,
) -> Result<(), String> {
    if requested.is_empty() {
        return Ok(());
    }
    let mut requested = requested.to_vec();
    requested.sort_unstable();
    let mut approved = approved.unwrap_or_default().to_vec();
    approved.sort_unstable();
    if requested == approved {
        Ok(())
    } else {
        Err("Extension permissions must be reviewed and approved before installation".to_string())
    }
}

pub(crate) fn permission_review(
    manifest: &ExtensionManifest,
    locale: &str,
) -> ExtensionPermissionReview {
    let is_zh = locale.to_ascii_lowercase().starts_with("zh");
    let permissions = manifest
        .permissions
        .iter()
        .copied()
        .map(|permission| {
            let (title, description) = match (permission, is_zh) {
                (Permission::FilesystemRead, false) => {
                    ("Read files", "Read files and folders available to Floter")
                }
                (Permission::FilesystemWrite, false) => (
                    "Modify files",
                    "Create, change, and remove files and folders",
                ),
                (Permission::NetworkFetch, false) => {
                    ("Access the network", "Make outbound network requests")
                }
                (Permission::ProcessSpawn, false) => (
                    "Start processes",
                    "Allow a command descriptor to ask Floter to run another program",
                ),
                (Permission::ClipboardRead, false) => {
                    ("Read clipboard", "Read content from the system clipboard")
                }
                (Permission::ClipboardWrite, false) => {
                    ("Write clipboard", "Write content to the system clipboard")
                }
                (Permission::Environment, false) => {
                    ("Read environment", "Inherit Floter's environment variables")
                }
                (Permission::FilesystemRead, true) => {
                    ("读取文件", "读取 Floter 可访问的文件和文件夹")
                }
                (Permission::FilesystemWrite, true) => ("修改文件", "创建、更改和删除文件及文件夹"),
                (Permission::NetworkFetch, true) => ("访问网络", "发起出站网络请求"),
                (Permission::ProcessSpawn, true) => {
                    ("启动进程", "允许命令描述要求 Floter 运行其他程序")
                }
                (Permission::ClipboardRead, true) => ("读取剪贴板", "读取系统剪贴板中的内容"),
                (Permission::ClipboardWrite, true) => ("写入剪贴板", "向系统剪贴板写入内容"),
                (Permission::Environment, true) => ("读取环境变量", "继承 Floter 的环境变量"),
            };
            PermissionSummary {
                permission,
                title: title.to_string(),
                description: description.to_string(),
            }
        })
        .collect();
    ExtensionPermissionReview {
        extension_id: manifest.id.clone(),
        extension_name: manifest.name.clone(),
        permissions,
        publisher_signed: false,
        official_verified: false,
    }
}

pub async fn update(
    state: &ExtensionState,
    extension_id: &str,
    version: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let current = lock.get(extension_id)?.clone();
    if current.distribution_source != ExtensionDistributionSource::Npm {
        return Err("Only NPM integrations are updated by Floter".to_string());
    }
    if current.pinned && version.is_none() {
        return Err(format!("Extension {extension_id} is pinned"));
    }
    let package = current
        .package_name
        .as_deref()
        .ok_or("NPM integration has no package name in the lock file")?;
    install_managed(
        state,
        package,
        version.or(Some(current.channel.as_str())),
        Some(extension_id),
        approved_permissions,
    )
    .await
}

pub async fn reinstall(
    state: &ExtensionState,
    extension_id: &str,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let current = lock.get(extension_id)?.clone();
    if current.distribution_source != ExtensionDistributionSource::Npm {
        return Err("Only NPM integrations can be reinstalled by Floter".to_string());
    }
    let package = current
        .package_name
        .as_deref()
        .ok_or("NPM integration has no package name in the lock file")?;
    install_managed(
        state,
        package,
        Some(current.current_version.as_str()),
        Some(extension_id),
        approved_permissions,
    )
    .await
}

pub async fn uninstall(
    state: &ExtensionState,
    extension_id: &str,
    remove_data: bool,
) -> Result<(), String> {
    let _guard = state.mutation_lock.lock().await;
    validate_id(extension_id)?;
    state.provider.cancel_completions();
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entry = lock.get(extension_id)?.clone();
    let generated_local_root = state.paths.data.join(extension_id).join("integration");
    let generated_local = is_generated_custom_integration(&entry)
        && Path::new(&entry.manifest_path).starts_with(&generated_local_root);
    let mut moved = None;
    if entry.distribution_source == ExtensionDistributionSource::Npm {
        let source = state.paths.extensions.join(extension_id);
        if source.exists() {
            let placeholder = tempfile::Builder::new()
                .prefix(&format!(".removing-{extension_id}-"))
                .tempdir_in(&state.paths.extensions)
                .map_err(|error| format!("Cannot create removal transaction: {error}"))?;
            let target = placeholder.path().to_path_buf();
            placeholder
                .close()
                .map_err(|error| format!("Cannot prepare removal transaction: {error}"))?;
            std::fs::rename(&source, &target).map_err(|error| {
                format!(
                    "Cannot stage extension {} for removal: {error}",
                    source.display()
                )
            })?;
            moved = Some((source, target));
        }
    }
    lock.extensions.remove(extension_id);
    if let Err(error) = lock.save(&state.paths.lock_file) {
        if let Some((source, target)) = &moved {
            let _ = std::fs::rename(target, source);
        }
        return Err(error);
    }
    if let Some((_, target)) = moved {
        std::fs::remove_dir_all(&target)
            .map_err(|error| format!("Cannot remove {}: {error}", target.display()))?;
    }
    if generated_local && generated_local_root.exists() {
        std::fs::remove_dir_all(&generated_local_root).map_err(|error| {
            format!(
                "Cannot remove generated integration {}: {error}",
                generated_local_root.display()
            )
        })?;
        let data_root = state.paths.data.join(extension_id);
        if data_root
            .read_dir()
            .is_ok_and(|mut entries| entries.next().is_none())
        {
            std::fs::remove_dir(&data_root).map_err(|error| {
                format!(
                    "Cannot remove empty integration data directory {}: {error}",
                    data_root.display()
                )
            })?;
        }
    }
    if remove_data {
        let data = state.paths.data.join(extension_id);
        if data.exists() {
            std::fs::remove_dir_all(&data)
                .map_err(|error| format!("Cannot remove {}: {error}", data.display()))?;
        }
    }
    Ok(())
}

pub async fn rollback(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    rollback_locked(state, extension_id).await
}

pub(crate) async fn rollback_locked(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<ExtensionLockEntry, String> {
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let original = lock.get(extension_id)?.clone();
    let entry = lock
        .extensions
        .get_mut(extension_id)
        .ok_or_else(|| format!("Extension is not installed: {extension_id}"))?;
    if entry.distribution_source != ExtensionDistributionSource::Npm {
        return Err("Only NPM integrations can be rolled back by Floter".to_string());
    }
    let previous = entry
        .previous_version
        .clone()
        .ok_or_else(|| format!("Extension {extension_id} has no previous version"))?;
    let previous_root = state
        .paths
        .extensions
        .join(extension_id)
        .join("versions")
        .join(&previous);
    if !previous_root.is_dir() {
        return Err(format!(
            "Previous version directory is missing: {}",
            previous_root.display()
        ));
    }
    let current = std::mem::replace(&mut entry.current_version, previous.clone());
    entry.previous_version = Some(current);
    entry.package_version = previous.clone();
    entry.state = if entry.enabled {
        ExtensionStateKind::Enabled
    } else {
        ExtensionStateKind::Disabled
    };
    entry.manifest_path = find_installed_manifest(&previous_root)?
        .to_string_lossy()
        .into_owned();
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    if manifest.id != entry.id || manifest.publisher.id != entry.publisher_id {
        return Err(format!(
            "Previous manifest identity does not match lock entry {}",
            entry.id
        ));
    }
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let resolved = manifest.clone().resolve(PlatformTarget::current()?)?;
    resolved.validate_minimum_os_version()?;
    let (executable, runtime_root, version_args, tool_version) = match &manifest.runtime {
        Runtime::Bundled { .. } => {
            let executable = managed_executable(&manifest, &previous_root)?;
            let runtime_root = previous_root.join("runtime");
            (executable, Some(runtime_root), Vec::new(), None)
        }
        Runtime::System { version_args, .. } => {
            let executable = find_system_executable(&manifest)?;
            let tool_version =
                linked_tool_version(&manifest, &resolved.provider, &executable).await;
            (executable, None, version_args.clone(), tool_version)
        }
        Runtime::Script { .. } => {
            return Err("NPM rollback does not support script runtimes".to_string())
        }
    };
    entry.runtime_ownership = match manifest.runtime {
        Runtime::Bundled { .. } => ExtensionRuntimeOwnership::Bundled,
        Runtime::System { .. } => ExtensionRuntimeOwnership::System,
        Runtime::Script { .. } => ExtensionRuntimeOwnership::System,
    };
    entry.executable_path = executable.to_string_lossy().into_owned();
    entry.runtime_root = runtime_root
        .as_ref()
        .map(|path| path.to_string_lossy().into_owned());
    let response = state
        .provider
        .describe(
            &ProviderInvocation {
                extension_id: entry.id.clone(),
                executable,
                executable_prefix: Vec::new(),
                runtime_root,
                package_version: previous,
                tool_version_hint: tool_version.clone(),
                version_args,
                config: resolved.provider,
                permissions: manifest.permissions,
            },
            true,
        )
        .await?;
    entry.tool_version = tool_version.or(Some(response.description.provider.version));
    let previous_signature_verified = entry.previous_signature_verified.unwrap_or(false);
    entry.previous_signature_verified = Some(entry.signature_verified);
    entry.signature_verified = previous_signature_verified;
    let previous_official_verified = entry.previous_official_verified.unwrap_or(false);
    entry.previous_official_verified = Some(entry.official_verified);
    entry.official_verified = previous_official_verified;
    entry.updated_at = unix_now();
    let result = entry.clone();
    commit_lock(state, &original, &result)?;
    Ok(result)
}

pub async fn search(
    state: &ExtensionState,
    query: &str,
    limit: usize,
) -> Result<Vec<ExtensionSearchResult>, String> {
    let official_index = official_index::fetch(state).await.ok();
    let url = format!("{REGISTRY_URL}-/v1/search");
    let response = state
        .client
        .get(url)
        .query(&[
            ("text", format!("keywords:floter-extension {query}")),
            ("size", limit.clamp(1, 50).to_string()),
        ])
        .send()
        .await
        .map_err(|error| format!("NPM search failed: {error}"))?
        .error_for_status()
        .map_err(|error| format!("NPM search failed: {error}"))?;
    ensure_https_response(&response, "NPM search")?;
    let bytes =
        read_response_limited(response, MAX_SEARCH_RESPONSE_BYTES, "NPM search response").await?;
    let response: RegistrySearchResponse = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid NPM search response: {error}"))?;
    Ok(response
        .objects
        .into_iter()
        .map(|object| {
            let publisher = object
                .package
                .publisher
                .and_then(|publisher| publisher.username);
            let verified = official_index.as_ref().is_some_and(|index| {
                index.search_verified(&object.package.name, publisher.as_deref())
            });
            ExtensionSearchResult {
                downloads: object.downloads.weekly,
                package: object.package.name,
                version: object.package.version,
                description: object.package.description,
                publisher,
                homepage: object
                    .package
                    .links
                    .get("homepage")
                    .or_else(|| object.package.links.get("repository"))
                    .cloned(),
                verified,
            }
        })
        .collect())
}

async fn install_managed(
    state: &ExtensionState,
    package: &str,
    selector: Option<&str>,
    expected_id: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    validate_package_name(package)?;
    let official_index = official_index::fetch(state).await.ok();
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let expected_entry = expected_id.and_then(|id| lock.extensions.get(id)).cloned();
    if let Some(entry) = expected_entry.as_ref() {
        if entry.distribution_source != ExtensionDistributionSource::Npm
            || entry.package_name.as_deref() != Some(package)
        {
            return Err("Update package does not match the installed extension".to_string());
        }
    }
    let mut transaction = InstallationTransaction::new();
    let base_version = resolve_registry_version(state, package, selector).await?;
    transaction.advance(InstallationPhase::Downloading)?;
    let base_bytes = download_tarball(state, &base_version.dist).await?;

    let staging_parent = state.paths.extensions.join(".staging");
    std::fs::create_dir_all(&staging_parent)
        .map_err(|error| format!("Cannot create installation staging directory: {error}"))?;
    let staging = tempfile::Builder::new()
        .prefix("install-")
        .tempdir_in(&staging_parent)
        .map_err(|error| format!("Cannot create installation transaction: {error}"))?;
    let version_root = staging.path().join("version");
    std::fs::create_dir_all(&version_root)
        .map_err(|error| format!("Cannot create staged version directory: {error}"))?;
    transaction.advance(InstallationPhase::Verifying)?;
    safe_unpack(&base_bytes, &version_root)?;
    let (package_json, manifest_path) = load_package_entry(&version_root)?;
    validate_package_entry(&package_json, &base_version, true)?;
    let manifest = ExtensionManifest::load(&manifest_path)?;
    if manifest.distribution != crate::extensions::manifest::Distribution::Npm {
        return Err("NPM packages must declare distribution.type = npm".to_string());
    }
    let official_verified = if let Some(signatures) = manifest.signatures.as_ref() {
        let index_authorized = official_index.as_ref().is_some_and(|index| {
            index.authorizes(
                &manifest.id,
                package,
                &manifest.publisher.id,
                Some(signatures),
            )
        });
        let signature = download_signature(state, signatures).await?;
        verify_official_tarball(index_authorized, &base_bytes, &signature, signatures)?
    } else {
        false
    };
    if expected_id.is_some_and(|expected| expected != manifest.id) {
        return Err(format!(
            "Package changed extension id from {} to {}",
            expected_id.unwrap_or_default(),
            manifest.id
        ));
    }
    if expected_id.is_none() && lock.extensions.contains_key(&manifest.id) {
        return Err(format!(
            "Extension {} is already installed; use update instead",
            manifest.id
        ));
    }
    let permission_approval_required = if let Some(entry) = expected_entry.as_ref() {
        let installed_manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
        if installed_manifest.id != entry.id
            || installed_manifest.publisher.id != entry.publisher_id
        {
            return Err(format!(
                "Installed manifest identity does not match lock entry {}",
                entry.id
            ));
        }
        has_added_permissions(&installed_manifest.permissions, &manifest.permissions)
    } else {
        true
    };
    if permission_approval_required {
        validate_permission_approval(&manifest.permissions, approved_permissions)?;
    }
    if expected_entry
        .as_ref()
        .is_some_and(|entry| entry.publisher_id != manifest.publisher.id)
    {
        return Err(format!("Publisher changed for extension {}", manifest.id));
    }
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let resolved = manifest.clone().resolve(PlatformTarget::current()?)?;
    resolved.validate_minimum_os_version()?;
    let (executable, runtime_root, version_args, tool_version) = match &manifest.runtime {
        Runtime::Bundled { .. } => {
            let platform_package = resolved
                .platform_package
                .as_deref()
                .ok_or("Bundled runtime has no platform package")?;
            let platform_version =
                resolve_registry_version(state, platform_package, Some(&base_version.version))
                    .await?;
            if platform_version.version != base_version.version {
                return Err("Base and platform package versions must match".to_string());
            }
            let runtime_bytes = download_tarball(state, &platform_version.dist).await?;
            let runtime_root = version_root.join("runtime");
            std::fs::create_dir_all(&runtime_root)
                .map_err(|error| format!("Cannot create runtime directory: {error}"))?;
            safe_unpack(&runtime_bytes, &runtime_root)?;
            let runtime_package: PackageJson = serde_json::from_slice(
                &std::fs::read(runtime_root.join("package.json"))
                    .map_err(|error| format!("Platform package has no package.json: {error}"))?,
            )
            .map_err(|error| format!("Invalid platform package.json: {error}"))?;
            validate_package_entry(&runtime_package, &platform_version, false)?;
            let executable = managed_executable(&manifest, &version_root)?;
            make_executable(&executable)?;
            (executable, Some(runtime_root), Vec::new(), None)
        }
        Runtime::System { version_args, .. } => {
            let executable = find_system_executable(&manifest)?;
            let tool_version =
                linked_tool_version(&manifest, &resolved.provider, &executable).await;
            (executable, None, version_args.clone(), tool_version)
        }
        Runtime::Script { .. } => {
            return Err("NPM packages cannot use local script runtimes".to_string())
        }
    };
    let invocation = ProviderInvocation {
        extension_id: manifest.id.clone(),
        executable: executable.clone(),
        runtime_root: runtime_root.clone(),
        package_version: base_version.version.clone(),
        tool_version_hint: tool_version.clone(),
        version_args,
        config: resolved.provider.clone(),
        permissions: manifest.permissions.clone(),
        executable_prefix: Vec::new(),
    };
    let description = state.provider.describe(&invocation, true).await?;

    // Run post-install capability probes if declared in lifecycle
    if !manifest.lifecycle.probes.is_empty() {
        let tool_data_dir = state.paths.data.join(&manifest.id);
        let probe_args: Vec<Vec<String>> = manifest.lifecycle.probes.iter()
            .map(|p| p.args.clone())
            .collect();
        let required: Vec<bool> = manifest.lifecycle.probes.iter()
            .map(|p| p.required)
            .collect();
        match crate::extensions::probe_runner::run_probes(state, &manifest.id, &executable, &probe_args, &required).await {
            Ok(ref report) => {
                let _ = crate::extensions::health::write_health_report(&tool_data_dir, &report);
                // Auto-rollback: required probes failed on new version
                if report.status == crate::extensions::health::HealthStatus::Unhealthy {
                    return Err(format!(
                        "Installation aborted: {} failed required probes. {}",
                        manifest.id,
                        if !report.failures.is_empty() {
                            format!("Failures: {}", report.failures.iter()
                                .map(|f| format!("{} (exit {:?})", f.probe, f.exit_code))
                                .collect::<Vec<_>>().join(", "))
                        } else { String::new() }
                    ));
                }
            }
            Err(e) => {
                eprintln!("Probe run failed for {}: {}", manifest.id, e);
            }
        }
    }

    let old = lock.extensions.get(&manifest.id).cloned();
    if old
        .as_ref()
        .is_some_and(|entry| entry.distribution_source != ExtensionDistributionSource::Npm)
    {
        return Err(format!(
            "Integration {} is already connected from a non-NPM source",
            manifest.id
        ));
    }
    let extension_root = state.paths.extensions.join(&manifest.id);
    let versions_root = extension_root.join("versions");
    std::fs::create_dir_all(&versions_root)
        .map_err(|error| format!("Cannot create extension versions directory: {error}"))?;
    let target = versions_root.join(&base_version.version);
    transaction.advance(InstallationPhase::Installing)?;

    let final_manifest = target.join(
        manifest_path
            .strip_prefix(&version_root)
            .map_err(|_| "Manifest escaped staged version root")?,
    );
    let (runtime_ownership, final_executable, final_runtime) = match &manifest.runtime {
        Runtime::Bundled { executable, .. } => {
            let final_runtime = target.join("runtime");
            (
                ExtensionRuntimeOwnership::Bundled,
                final_runtime.join(validate_relative_path(executable, "executable")?),
                Some(final_runtime),
            )
        }
        Runtime::System { .. } => (ExtensionRuntimeOwnership::System, executable, None),
        Runtime::Script { .. } => {
            return Err("NPM packages cannot use local script runtimes".to_string())
        }
    };
    let now = unix_now();
    let enabled = old.as_ref().is_none_or(|entry| entry.enabled);
    let entry = ExtensionLockEntry {
        id: manifest.id.clone(),
        name: manifest.name.clone(),
        publisher_id: manifest.publisher.id.clone(),
        publisher_name: manifest.publisher.name.clone(),
        distribution_source: ExtensionDistributionSource::Npm,
        runtime_ownership,
        provider_kind: ExtensionProviderKind::Executable,
        state: if enabled {
            ExtensionStateKind::Enabled
        } else {
            ExtensionStateKind::Disabled
        },
        enabled,
        package_name: Some(package.to_string()),
        package_version: base_version.version.clone(),
        tool_version: tool_version.or(Some(description.description.provider.version)),
        integrity: base_version.dist.integrity.clone(),
        signature_verified: manifest.signatures.is_some(),
        previous_signature_verified: old.as_ref().map(|entry| entry.signature_verified),
        official_verified,
        previous_official_verified: old.as_ref().map(|entry| entry.official_verified),
        current_version: base_version.version.clone(),
        previous_version: old.as_ref().map(|entry| entry.current_version.clone()),
        manifest_path: final_manifest.to_string_lossy().into_owned(),
        executable_path: final_executable.to_string_lossy().into_owned(),
        runtime_root: final_runtime.map(|path| path.to_string_lossy().into_owned()),
        installed_at: old.as_ref().map_or(now, |entry| entry.installed_at),
        updated_at: now,
        pinned: old.as_ref().is_some_and(|entry| entry.pinned),
        channel: selector
            .filter(|selector| Version::parse(selector).is_err())
            .unwrap_or("latest")
            .to_string(),
    };
    transaction.advance(InstallationPhase::Complete)?;
    commit_version(state, old.as_ref(), &entry, &version_root, &target)?;
    Ok(entry)
}

fn has_added_permissions(installed: &[Permission], requested: &[Permission]) -> bool {
    requested
        .iter()
        .any(|permission| !installed.contains(permission))
}

pub(crate) async fn install_linked(
    state: &ExtensionState,
    request: ExtensionInstallRequest,
) -> Result<ExtensionLockEntry, String> {
    let manifest_source = request
        .manifest_path
        .as_deref()
        .ok_or("Linked installation requires manifestPath")?;
    let manifest_path = PathBuf::from(manifest_source);
    let is_package_directory = manifest_path.is_dir();
    let (manifest, package_version, manifest_path) = if is_package_directory {
        let (package_json, actual_manifest_path) = load_package_entry(&manifest_path)?;
        if !package_json
            .keywords
            .iter()
            .any(|keyword| keyword == "floter-extension")
        {
            return Err("package.json is missing the floter-extension keyword".to_string());
        }
        (
            ExtensionManifest::load(&actual_manifest_path)?,
            package_json.version,
            actual_manifest_path,
        )
    } else {
        (
            ExtensionManifest::load(&manifest_path)?,
            "linked".to_string(),
            manifest_path,
        )
    };
    validate_permission_approval(
        &manifest.permissions,
        request.approved_permissions.as_deref(),
    )?;
    if manifest.distribution != crate::extensions::manifest::Distribution::Local {
        return Err("Local connections must declare distribution.type = local".to_string());
    }
    if !matches!(
        manifest.runtime,
        Runtime::System { .. } | Runtime::Script { .. }
    ) {
        return Err("Local connection requires a system or script runtime manifest".to_string());
    }
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let resolved = manifest.clone().resolve(PlatformTarget::current()?)?;
    resolved.validate_minimum_os_version()?;
    let executable = if let Some(path) = request.executable_path {
        let path = PathBuf::from(path);
        if !is_linked_executable(&path) {
            return Err(format!(
                "Linked executable is not usable: {}",
                path.display()
            ));
        }
        path
    } else if matches!(manifest.runtime, Runtime::Script { .. }) {
        PathBuf::from("script-provider")
    } else {
        find_system_executable(&manifest)?
    };
    let tool_version = linked_tool_version(&manifest, &resolved.provider, &executable).await;
    let executable_prefix = match &manifest.runtime {
        Runtime::Script { language, path, .. } => {
            let script = manifest_path.parent().unwrap_or(Path::new(".")).join(path);
            if !script.is_file() {
                return Err(format!("Script file is missing: {}", script.display()));
            }
            match language {
                ScriptLanguage::Js => vec![script.to_string_lossy().into_owned()],
                ScriptLanguage::Shell => vec![script.to_string_lossy().into_owned()],
                ScriptLanguage::Powershell => {
                    vec!["-File".into(), script.to_string_lossy().into_owned()]
                }
            }
        }
        _ => Vec::new(),
    };
    let executable = match &manifest.runtime {
        Runtime::Script { language, .. } => find_script_interpreter(*language)?,
        _ => executable,
    };
    let invocation = ProviderInvocation {
        extension_id: manifest.id.clone(),
        executable: executable.clone(),
        executable_prefix,
        runtime_root: None,
        package_version: package_version.clone(),
        tool_version_hint: tool_version.clone(),
        version_args: match &manifest.runtime {
            Runtime::System { version_args, .. } => version_args.clone(),
            Runtime::Script { version_args, .. } => version_args.clone(),
            Runtime::Bundled { .. } => Vec::new(),
        },
        config: resolved.provider,
        permissions: manifest.permissions.clone(),
    };
    let response = if manifest.provider.kind == ProviderKind::StaticDescriptor {
        let descriptor_path = manifest_path
            .parent()
            .ok_or("Local manifest has no parent directory")?
            .join(
                manifest
                    .provider
                    .descriptor
                    .as_deref()
                    .ok_or("Static descriptor path is missing")?,
            );
        let description = crate::extensions::provider::ProviderDescription::parse(
            &std::fs::read(&descriptor_path)
                .map_err(|error| format!("Cannot read static provider descriptor: {error}"))?,
        )?;
        crate::extensions::provider::validate_execution_descriptors(&description, &invocation)?;
        crate::extensions::provider::ProviderResponse {
            description,
            runtime_available: true,
            cached: true,
            stderr: None,
        }
    } else {
        state.provider.describe(&invocation, true).await?
    };

    // Run post-install capability probes for linked tools too
    if !manifest.lifecycle.probes.is_empty() {
        let tool_data_dir = state.paths.data.join(&manifest.id);
        let probe_args: Vec<Vec<String>> = manifest.lifecycle.probes.iter()
            .map(|p| p.args.clone())
            .collect();
        let required: Vec<bool> = manifest.lifecycle.probes.iter()
            .map(|p| p.required)
            .collect();
        match crate::extensions::probe_runner::run_probes(state, &manifest.id, &executable, &probe_args, &required).await {
            Ok(report) => {
                let _ = crate::extensions::health::write_health_report(&tool_data_dir, &report);
            }
            Err(e) => {
                eprintln!("Probe run failed for {}: {}", manifest.id, e);
            }
        }
    }
    let integration_version = if is_package_directory {
        package_version
    } else {
        response.description.provider.version.clone()
    };
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    if lock.extensions.contains_key(&manifest.id) {
        return Err(format!("Extension is already installed: {}", manifest.id));
    }
    let now = unix_now();
    let entry = ExtensionLockEntry {
        id: manifest.id.clone(),
        name: manifest.name,
        publisher_id: manifest.publisher.id,
        publisher_name: manifest.publisher.name,
        distribution_source: ExtensionDistributionSource::Local,
        runtime_ownership: ExtensionRuntimeOwnership::System,
        provider_kind: match manifest.provider.kind {
            ProviderKind::Executable => ExtensionProviderKind::Executable,
            ProviderKind::StaticDescriptor => ExtensionProviderKind::StaticDescriptor,
        },
        state: ExtensionStateKind::Enabled,
        enabled: true,
        package_name: request.package,
        package_version: integration_version.clone(),
        tool_version: tool_version.or(Some(response.description.provider.version)),
        integrity: None,
        signature_verified: false,
        previous_signature_verified: None,
        official_verified: false,
        previous_official_verified: None,
        current_version: integration_version,
        previous_version: None,
        manifest_path: manifest_path.to_string_lossy().into_owned(),
        executable_path: executable.to_string_lossy().into_owned(),
        runtime_root: None,
        installed_at: now,
        updated_at: now,
        pinned: false,
        channel: "external".into(),
    };
    lock.extensions.insert(entry.id.clone(), entry.clone());
    lock.save(&state.paths.lock_file)?;
    Ok(entry)
}

async fn resolve_registry_version(
    state: &ExtensionState,
    package: &str,
    selector: Option<&str>,
) -> Result<RegistryVersion, String> {
    let mut url = reqwest::Url::parse(REGISTRY_URL)
        .map_err(|error| format!("Invalid NPM registry URL: {error}"))?;
    url.path_segments_mut()
        .map_err(|_| "NPM registry URL cannot contain package paths")?
        .push(package);
    let response = state
        .client
        .get(url)
        .send()
        .await
        .map_err(|error| format!("Cannot resolve NPM package {package}: {error}"))?
        .error_for_status()
        .map_err(|error| format!("Cannot resolve NPM package {package}: {error}"))?;
    ensure_https_response(&response, "NPM registry metadata")?;
    let bytes = read_response_limited(
        response,
        MAX_REGISTRY_METADATA_BYTES,
        "NPM registry metadata",
    )
    .await?;
    let metadata: RegistryMetadata = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid NPM metadata for {package}: {error}"))?;
    let selector = selector.unwrap_or("latest");
    let exact = metadata
        .versions
        .get(selector)
        .or_else(|| {
            metadata
                .dist_tags
                .get(selector)
                .and_then(|version| metadata.versions.get(version))
        })
        .cloned();
    if let Some(version) = exact {
        validate_registry_version(package, &version)?;
        return Ok(version);
    }
    let requirement = VersionReq::parse(selector)
        .map_err(|_| format!("Unknown NPM version or dist-tag: {selector}"))?;
    let version = metadata
        .versions
        .into_iter()
        .filter_map(|(version, metadata)| {
            Version::parse(&version)
                .ok()
                .map(|version| (version, metadata))
        })
        .filter(|(version, _)| requirement.matches(version))
        .max_by(|(left, _), (right, _)| left.cmp(right))
        .map(|(_, metadata)| metadata)
        .ok_or_else(|| format!("No version of {package} matches {selector}"))?;
    validate_registry_version(package, &version)?;
    Ok(version)
}

async fn download_tarball(state: &ExtensionState, dist: &RegistryDist) -> Result<Vec<u8>, String> {
    let integrity = dist
        .integrity
        .as_deref()
        .ok_or("NPM package does not provide dist.integrity")?;
    let tarball_url = reqwest::Url::parse(&dist.tarball)
        .map_err(|error| format!("Invalid NPM tarball URL: {error}"))?;
    if tarball_url.scheme() != "https" {
        return Err("NPM tarball URL must use HTTPS".to_string());
    }
    crate::extensions::download::download_with_resume(
        &state.client,
        &state.paths.cache,
        tarball_url,
        integrity,
        MAX_TARBALL_BYTES,
        "NPM tarball",
        verify_integrity,
    )
    .await
}

fn verify_integrity(bytes: &[u8], integrity: &str) -> Result<(), String> {
    let mut supported = false;
    for token in integrity.split_whitespace() {
        let Some((algorithm, encoded)) = token.split_once('-') else {
            continue;
        };
        let expected = match base64::engine::general_purpose::STANDARD.decode(encoded) {
            Ok(expected) => expected,
            Err(_) => continue,
        };
        let actual = match algorithm {
            "sha512" => Sha512::digest(bytes).to_vec(),
            "sha384" => Sha384::digest(bytes).to_vec(),
            "sha256" => Sha256::digest(bytes).to_vec(),
            _ => continue,
        };
        supported = true;
        if constant_time_equal(&actual, &expected) {
            return Ok(());
        }
    }
    if supported {
        Err("NPM tarball integrity verification failed".to_string())
    } else {
        Err("NPM package has no supported SRI digest".to_string())
    }
}

async fn download_signature(
    state: &ExtensionState,
    config: &SignatureConfig,
) -> Result<Vec<u8>, String> {
    let signature_url = reqwest::Url::parse(&config.url)
        .map_err(|error| format!("Invalid extension signature URL: {error}"))?;
    if signature_url.scheme() != "https" {
        return Err("Extension signature URL must use HTTPS".to_string());
    }
    let response = state
        .client
        .get(signature_url)
        .send()
        .await
        .map_err(|error| format!("Cannot download extension signature: {error}"))?
        .error_for_status()
        .map_err(|error| format!("Cannot download extension signature: {error}"))?;
    ensure_https_response(&response, "extension signature")?;
    let signature =
        read_response_limited(response, MAX_SIGNATURE_BYTES, "Extension signature").await?;
    Ok(signature)
}

fn verify_official_tarball(
    index_authorized: bool,
    tarball: &[u8],
    signature_file: &[u8],
    signature_config: &SignatureConfig,
) -> Result<bool, String> {
    verify_signature(tarball, signature_file, signature_config)?;
    Ok(index_authorized)
}

fn ensure_https_response(response: &reqwest::Response, label: &str) -> Result<(), String> {
    if response.url().scheme() == "https" {
        Ok(())
    } else {
        Err(format!("{label} redirected to a non-HTTPS URL"))
    }
}

async fn read_response_limited(
    mut response: reqwest::Response,
    limit: usize,
    label: &str,
) -> Result<Vec<u8>, String> {
    if response
        .content_length()
        .is_some_and(|length| length > limit as u64)
    {
        return Err(format!("{label} exceeds {limit} bytes"));
    }
    let mut bytes = Vec::new();
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|error| format!("Cannot read {label}: {error}"))?
    {
        let new_length = bytes
            .len()
            .checked_add(chunk.len())
            .ok_or_else(|| format!("{label} size overflow"))?;
        if new_length > limit {
            return Err(format!("{label} exceeds {limit} bytes"));
        }
        bytes.extend_from_slice(&chunk);
    }
    Ok(bytes)
}

fn validate_registry_version(package: &str, version: &RegistryVersion) -> Result<(), String> {
    if version.name != package {
        return Err(format!(
            "NPM registry returned package {} while resolving {package}",
            version.name
        ));
    }
    Version::parse(&version.version)
        .map_err(|error| format!("Invalid NPM version {}: {error}", version.version))?;
    Ok(())
}

fn verify_signature(
    tarball: &[u8],
    signature_file: &[u8],
    config: &SignatureConfig,
) -> Result<(), String> {
    match config.algorithm {
        SignatureAlgorithm::Ed25519 => {}
    }

    let encoded_key = config
        .public_key
        .strip_prefix("ed25519:")
        .ok_or("Ed25519 public key must use the ed25519: prefix")?;
    let public_key: [u8; 32] = decode_base64(encoded_key, "Ed25519 public key")?
        .try_into()
        .map_err(|_| "Ed25519 public key must contain exactly 32 bytes".to_string())?;
    let verifying_key = VerifyingKey::from_bytes(&public_key)
        .map_err(|error| format!("Invalid Ed25519 public key: {error}"))?;

    let signature_text = std::str::from_utf8(signature_file)
        .map_err(|_| "Extension signature file must be UTF-8 Base64 text")?
        .trim();
    let encoded_signature = signature_text
        .strip_prefix("ed25519:")
        .unwrap_or(signature_text);
    let signature = Signature::from_slice(&decode_base64(encoded_signature, "Ed25519 signature")?)
        .map_err(|error| format!("Invalid Ed25519 signature: {error}"))?;

    verifying_key
        .verify_strict(tarball, &signature)
        .map_err(|_| "Extension signature verification failed".to_string())
}

fn decode_base64(value: &str, label: &str) -> Result<Vec<u8>, String> {
    base64::engine::general_purpose::STANDARD
        .decode(value)
        .or_else(|_| base64::engine::general_purpose::STANDARD_NO_PAD.decode(value))
        .map_err(|error| format!("Invalid Base64 {label}: {error}"))
}

fn constant_time_equal(left: &[u8], right: &[u8]) -> bool {
    let mut difference = left.len() ^ right.len();
    for index in 0..left.len().max(right.len()) {
        difference |= usize::from(
            left.get(index).copied().unwrap_or(0) ^ right.get(index).copied().unwrap_or(0),
        );
    }
    difference == 0
}

fn safe_unpack(bytes: &[u8], destination: &Path) -> Result<(), String> {
    let decoder = GzDecoder::new(Cursor::new(bytes));
    let mut archive = tar::Archive::new(decoder);
    let mut extracted_bytes = 0_u64;
    for (index, entry) in archive
        .entries()
        .map_err(|error| format!("Cannot read NPM archive: {error}"))?
        .enumerate()
    {
        if index >= MAX_ARCHIVE_ENTRIES {
            return Err(format!(
                "NPM archive contains more than {MAX_ARCHIVE_ENTRIES} entries"
            ));
        }
        let mut entry = entry.map_err(|error| format!("Invalid NPM archive entry: {error}"))?;
        let path = entry
            .path()
            .map_err(|error| format!("Invalid NPM archive path: {error}"))?;
        let relative = safe_archive_path(&path)?;
        if relative.as_os_str().is_empty() {
            continue;
        }
        let target = destination.join(&relative);
        let kind = entry.header().entry_type();
        if kind.is_symlink() || kind.is_hard_link() {
            return Err(format!(
                "NPM archive links are not allowed: {}",
                path.display()
            ));
        }
        if kind.is_dir() {
            std::fs::create_dir_all(&target)
                .map_err(|error| format!("Cannot create archive directory: {error}"))?;
            continue;
        }
        if !kind.is_file() {
            return Err(format!("Unsupported NPM archive entry: {}", path.display()));
        }
        let size = entry.size();
        extracted_bytes = extracted_bytes
            .checked_add(size)
            .ok_or("NPM archive extracted size overflow")?;
        if extracted_bytes > (MAX_TARBALL_BYTES as u64 * 4) {
            return Err("NPM archive expands beyond the installation size limit".to_string());
        }
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent)
                .map_err(|error| format!("Cannot create archive directory: {error}"))?;
        }
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&target)
            .map_err(|error| format!("Cannot create archive file {}: {error}", target.display()))?;
        std::io::copy(&mut entry, &mut file)
            .map_err(|error| format!("Cannot extract {}: {error}", target.display()))?;
        file.flush()
            .map_err(|error| format!("Cannot flush {}: {error}", target.display()))?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = entry.header().mode().unwrap_or(0o644) & 0o777;
            std::fs::set_permissions(&target, std::fs::Permissions::from_mode(mode))
                .map_err(|error| format!("Cannot set archive file mode: {error}"))?;
        }
    }
    Ok(())
}

fn safe_archive_path(path: &Path) -> Result<PathBuf, String> {
    let mut components = path.components();
    let Some(Component::Normal(root)) = components.next() else {
        return Err(format!("Archive path is not relative: {}", path.display()));
    };
    if root != "package" {
        return Err(format!(
            "NPM archive entry is outside package/: {}",
            path.display()
        ));
    }
    let mut relative = PathBuf::new();
    for component in components {
        match component {
            Component::Normal(value) => relative.push(value),
            _ => return Err(format!("Unsafe archive path: {}", path.display())),
        }
    }
    Ok(relative)
}

fn load_package_entry(root: &Path) -> Result<(PackageJson, PathBuf), String> {
    let package_path = root.join("package.json");
    let package: PackageJson = serde_json::from_slice(
        &std::fs::read(&package_path)
            .map_err(|error| format!("Package has no package.json: {error}"))?,
    )
    .map_err(|error| format!("Invalid package.json: {error}"))?;
    let manifest_name = package
        .floter
        .as_ref()
        .ok_or("package.json has no floter.manifest")?
        .manifest
        .as_str();
    let manifest_relative = validate_relative_path(manifest_name, "floter.manifest")?;
    Ok((package, root.join(manifest_relative)))
}

fn validate_package_entry(
    package: &PackageJson,
    registry: &RegistryVersion,
    require_extension_keyword: bool,
) -> Result<(), String> {
    if package.name != registry.name || package.version != registry.version {
        return Err(format!(
            "package.json identity {}@{} does not match registry {}@{}",
            package.name, package.version, registry.name, registry.version
        ));
    }
    if require_extension_keyword
        && !package
            .keywords
            .iter()
            .any(|keyword| keyword == "floter-extension")
    {
        return Err("package.json is missing the floter-extension keyword".to_string());
    }
    Ok(())
}

fn managed_executable(
    manifest: &ExtensionManifest,
    version_root: &Path,
) -> Result<PathBuf, String> {
    let Runtime::Bundled { executable, .. } = &manifest.runtime else {
        return Err("Bundled installation requires a bundled runtime".to_string());
    };
    let executable = version_root
        .join("runtime")
        .join(validate_relative_path(executable, "executable")?);
    if !executable.is_file() {
        return Err(format!(
            "Managed executable is missing: {}",
            executable.display()
        ));
    }
    Ok(executable)
}

pub(crate) fn make_executable(path: &Path) -> Result<(), String> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut permissions = path
            .metadata()
            .map_err(|error| format!("Cannot inspect executable {}: {error}", path.display()))?
            .permissions();
        permissions.set_mode(permissions.mode() | 0o700);
        std::fs::set_permissions(path, permissions)
            .map_err(|error| format!("Cannot set executable permission: {error}"))?;
    }
    Ok(())
}

fn find_installed_manifest(version_root: &Path) -> Result<PathBuf, String> {
    load_package_entry(version_root).map(|(_, path)| path)
}

pub(crate) fn find_system_executable(manifest: &ExtensionManifest) -> Result<PathBuf, String> {
    let Runtime::System {
        executable_names, ..
    } = &manifest.runtime
    else {
        return Err("Expected a system runtime".to_string());
    };
    let path = std::env::var_os("PATH").ok_or("PATH is not set")?;
    for directory in std::env::split_paths(&path) {
        for name in executable_names {
            for candidate_name in linked_candidate_names(name) {
                let candidate = directory.join(candidate_name);
                if is_linked_executable(&candidate) {
                    return Ok(candidate);
                }
            }
        }
    }
    Err(format!(
        "Cannot find system executable: {}",
        executable_names.join(", ")
    ))
}

pub(crate) fn linked_candidate_names(name: &str) -> Vec<String> {
    #[cfg(target_os = "windows")]
    {
        if Path::new(name).extension().is_some() {
            vec![name.to_string()]
        } else {
            [".exe", ".cmd", ".bat"]
                .into_iter()
                .map(|extension| format!("{name}{extension}"))
                .collect()
        }
    }
    #[cfg(not(target_os = "windows"))]
    {
        vec![name.to_string()]
    }
}

fn is_linked_executable(path: &Path) -> bool {
    if !path.is_file() {
        return false;
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        path.metadata()
            .is_ok_and(|metadata| metadata.permissions().mode() & 0o111 != 0)
    }
    #[cfg(windows)]
    {
        path.extension()
            .and_then(|extension| extension.to_str())
            .is_some_and(|extension| {
                matches!(
                    extension.to_ascii_lowercase().as_str(),
                    "exe" | "cmd" | "bat"
                )
            })
    }
    #[cfg(not(any(unix, windows)))]
    {
        true
    }
}

pub(crate) async fn linked_tool_version(
    manifest: &ExtensionManifest,
    provider: &crate::extensions::manifest::ProviderConfig,
    executable: &Path,
) -> Option<String> {
    let Runtime::System { version_args, .. } = &manifest.runtime else {
        return None;
    };
    if version_args.is_empty() {
        return None;
    }
    let mut command = tokio::process::Command::new(executable);
    if !manifest.permissions.contains(&Permission::Environment) {
        command.env_clear();
    }
    let environment =
        crate::extensions::proxy::command_environment(&manifest.permissions, &provider.environment);
    command
        .args(version_args)
        .envs(environment)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .kill_on_drop(true);
    let output = tokio::time::timeout(Duration::from_secs(2), command.output())
        .await
        .ok()?
        .ok()?;
    output
        .status
        .success()
        .then(|| {
            String::from_utf8_lossy(&output.stdout)
                .trim()
                .chars()
                .take(200)
                .collect()
        })
        .filter(|version: &String| !version.is_empty())
}

fn validate_package_name(package: &str) -> Result<(), String> {
    fn valid_segment(segment: &str) -> bool {
        segment
            .bytes()
            .next()
            .is_some_and(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit())
            && segment.bytes().all(|byte| {
                byte.is_ascii_lowercase()
                    || byte.is_ascii_digit()
                    || matches!(byte, b'.' | b'_' | b'-')
            })
            && segment != "."
            && segment != ".."
    }

    let valid = package.len() <= 214
        && if let Some(scoped) = package.strip_prefix('@') {
            scoped
                .split_once('/')
                .is_some_and(|(scope, name)| valid_segment(scope) && valid_segment(name))
        } else {
            !package.contains('/') && valid_segment(package)
        };
    if valid {
        Ok(())
    } else {
        Err(format!("Invalid NPM package name: {package}"))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::catalog::{self, CatalogSearchRequest};
    use crate::extensions::sync;
    use crate::extensions::ExtensionPaths;
    use chrono::Utc;
    use ed25519_dalek::{Signer, SigningKey};
    use std::collections::BTreeMap;

    fn test_state(root: &Path) -> ExtensionState {
        ExtensionState::from_paths(ExtensionPaths::from_root(root.to_path_buf())).unwrap()
    }

    fn journal_entry(state: &ExtensionState, version: &str) -> ExtensionLockEntry {
        let root = state
            .paths
            .extensions
            .join("example.journal")
            .join("versions")
            .join(version);
        ExtensionLockEntry {
            id: "example.journal".into(),
            name: "Journal test".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source: ExtensionDistributionSource::Npm,
            runtime_ownership: ExtensionRuntimeOwnership::Bundled,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: Some("example-journal".into()),
            package_version: version.into(),
            tool_version: Some("1.0.0".into()),
            integrity: None,
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: version.into(),
            previous_version: None,
            manifest_path: root
                .join("floter.extension.json")
                .to_string_lossy()
                .into_owned(),
            executable_path: root.join("runtime/tool").to_string_lossy().into_owned(),
            runtime_root: Some(root.join("runtime").to_string_lossy().into_owned()),
            installed_at: 1,
            updated_at: 1,
            pinned: false,
            channel: "latest".into(),
        }
    }

    #[test]
    fn recovery_rolls_back_directory_when_lock_was_not_committed() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let old = journal_entry(&state, "1.0.0");
        let new = journal_entry(&state, "1.0.0");
        let target = state
            .paths
            .extensions
            .join("example.journal/versions/1.0.0");
        let backup = state
            .paths
            .extensions
            .join("example.journal/versions/1.0.0.txn-backup");
        std::fs::create_dir_all(&target).unwrap();
        std::fs::create_dir_all(&backup).unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(old.id.clone(), old.clone());
        lock.save(&state.paths.lock_file).unwrap();
        crate::extensions::transaction::write_journal(
            &state,
            &crate::extensions::transaction::InstallationJournal {
                schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
                transaction_id: "rollback-journal".into(),
                extension_id: old.id.clone(),
                old_entry: Some(old.clone()),
                new_entry: new,
                staged_version: None,
                target_version: Some(target.clone()),
                backup_version: Some(backup.clone()),
                lock_committed: false,
                cleanup_paths: Vec::new(),
            },
        )
        .unwrap();
        crate::extensions::transaction::recover(&state).unwrap();
        assert!(target.exists());
        assert!(!backup.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get(&old.id)
                .unwrap()
                .current_version,
            "1.0.0"
        );
    }

    #[test]
    fn recovery_finishes_cleanup_when_lock_was_committed() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let old = journal_entry(&state, "1.0.0");
        let new = journal_entry(&state, "2.0.0");
        let target = state
            .paths
            .extensions
            .join("example.journal/versions/2.0.0");
        let backup = state
            .paths
            .extensions
            .join("example.journal/versions/1.0.0.txn-backup");
        std::fs::create_dir_all(&target).unwrap();
        std::fs::create_dir_all(&backup).unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(new.id.clone(), new.clone());
        lock.save(&state.paths.lock_file).unwrap();
        crate::extensions::transaction::write_journal(
            &state,
            &crate::extensions::transaction::InstallationJournal {
                schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
                transaction_id: "commit-journal".into(),
                extension_id: old.id,
                old_entry: Some(journal_entry(&state, "1.0.0")),
                new_entry: new,
                staged_version: None,
                target_version: Some(target),
                backup_version: Some(backup.clone()),
                lock_committed: true,
                cleanup_paths: Vec::new(),
            },
        )
        .unwrap();
        crate::extensions::transaction::recover(&state).unwrap();
        assert!(!backup.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get("example.journal")
                .unwrap()
                .current_version,
            "2.0.0"
        );
    }

    fn current_platforms() -> Vec<PlatformOs> {
        vec![PlatformTarget::current().unwrap().os]
    }

    fn script_request(id: &str, name: &str, command: &str) -> CustomIntegrationRequest {
        CustomIntegrationRequest {
            id: id.into(),
            name: name.into(),
            command: command.into(),
            version: "1.0.0".into(),
            executable_path: String::new(),
            mode: "script".into(),
            script_language: Some(ScriptLanguage::Shell),
            script_content: Some("printf original".into()),
            args_prefix: vec!["original".into()],
            version_args: Vec::new(),
            permissions: vec![Permission::Environment, Permission::FilesystemRead],
            platforms: current_platforms(),
        }
    }

    fn catalog_request(command: &str) -> CatalogSearchRequest {
        CatalogSearchRequest {
            query: command.into(),
            tokens: vec![command.into()],
            cwd: None,
            limit: 10,
            include_system_commands: false,
        }
    }

    async fn set_enabled_for_test(state: &ExtensionState, id: &str, enabled: bool) {
        let mut lock = ExtensionsLock::load(&state.paths.lock_file).unwrap();
        lock.set_enabled(id, enabled).unwrap();
        lock.save(&state.paths.lock_file).unwrap();
        state.invalidate_provider_commands().await;
    }

    async fn catalog_contains(state: &ExtensionState, command: &str) -> bool {
        catalog::search(state, &catalog_request(command), &[])
            .await
            .unwrap()
            .iter()
            .any(|entry| entry.command == command)
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn creates_an_executable_integration_with_an_exact_execution_plan() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let executable = Path::new("/usr/bin/printf");
        if !executable.is_file() {
            return;
        }
        let entry = create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.printf-test".into(),
                name: "Printf test".into(),
                command: "printf-test".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: vec!["prefix with spaces".into()],
                version_args: Vec::new(),
                permissions: vec![Permission::Environment],
                platforms: current_platforms(),
            },
        )
        .await
        .unwrap();

        assert_eq!(entry.provider_kind, ExtensionProviderKind::StaticDescriptor);
        let results = catalog::search(
            &state,
            &CatalogSearchRequest {
                query: "printf-test".into(),
                tokens: vec!["printf-test".into(), "user value".into()],
                cwd: None,
                limit: 10,
                include_system_commands: false,
            },
            &[],
        )
        .await
        .unwrap();
        let protected = results[0].execution.as_ref().unwrap();
        let plan = state
            .take_execution_plan(protected.plan_token.as_deref().unwrap())
            .unwrap();
        assert_eq!(Path::new(&plan.program), executable);
        assert_eq!(plan.args, ["prefix with spaces", "user value"]);
        assert!(plan.inherit_environment);
    }

    #[tokio::test]
    async fn creates_a_shell_script_integration_with_interpreter_prefix() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let entry = create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.shell-test".into(),
                name: "Shell test".into(),
                command: "shell-test".into(),
                version: "1.0.0".into(),
                executable_path: String::new(),
                mode: "script".into(),
                script_language: Some(ScriptLanguage::Shell),
                script_content: Some("printf '%s\\n' \"$@\"".into()),
                args_prefix: vec!["default value".into()],
                version_args: Vec::new(),
                permissions: vec![Permission::Environment],
                platforms: current_platforms(),
            },
        )
        .await
        .unwrap();

        let (_, invocation) = crate::extensions::registry::static_description(&entry).unwrap();
        assert!(invocation.executable.is_file());
        assert!(Path::new(&invocation.executable_prefix[0]).is_file());
        let results = catalog::search(
            &state,
            &CatalogSearchRequest {
                query: "shell-test".into(),
                tokens: vec!["shell-test".into(), "user value".into()],
                cwd: None,
                limit: 10,
                include_system_commands: false,
            },
            &[],
        )
        .await
        .unwrap();
        let protected = results[0].execution.as_ref().unwrap();
        let plan = state
            .take_execution_plan(protected.plan_token.as_deref().unwrap())
            .unwrap();
        assert_eq!(plan.args[0], invocation.executable_prefix[0]);
        assert_eq!(&plan.args[1..], ["default value", "user value"]);
    }

    #[tokio::test]
    async fn edits_and_reloads_a_generated_custom_integration() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let request = CustomIntegrationRequest {
            id: "local.edit-test".into(),
            name: "Edit test".into(),
            command: "edit-test".into(),
            version: "1.0.0".into(),
            executable_path: String::new(),
            mode: "script".into(),
            script_language: Some(ScriptLanguage::Shell),
            script_content: Some("printf old".into()),
            args_prefix: vec!["old".into()],
            version_args: Vec::new(),
            permissions: vec![Permission::Environment],
            platforms: current_platforms(),
        };
        let created = create_custom_integration(&state, request.clone())
            .await
            .unwrap();
        assert!(is_generated_custom_integration(&created));

        let mut changed = request;
        changed.name = "Edited test".into();
        changed.command = "edited-test".into();
        changed.version = "1.1.0".into();
        changed.script_content = Some("printf new".into());
        changed.args_prefix = vec!["new value".into()];
        let updated = update_custom_integration(&state, "local.edit-test", changed)
            .await
            .unwrap();
        let definition = custom_integration_definition(&state, "local.edit-test").unwrap();

        assert_eq!(updated.name, "Edited test");
        assert_eq!(definition.command, "edited-test");
        assert_eq!(definition.version, "1.1.0");
        assert_eq!(definition.script_content.as_deref(), Some("printf new"));
        assert_eq!(definition.args_prefix, ["new value"]);
    }

    #[tokio::test]
    async fn failed_custom_edit_restores_the_previous_integration() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let mut request = script_request("local.restore-test", "Restore test", "restore-test");
        request.script_content = Some("printf safe".into());
        request.args_prefix = Vec::new();
        create_custom_integration(&state, request.clone())
            .await
            .unwrap();
        let root = state
            .paths
            .data
            .join("local.restore-test")
            .join("integration");
        let original_lock = serde_json::to_value(
            ExtensionsLock::load(&state.paths.lock_file)
                .unwrap()
                .get("local.restore-test")
                .unwrap(),
        )
        .unwrap();
        let original_manifest = std::fs::read(root.join("floter.extension.json")).unwrap();
        let original_descriptor = std::fs::read(root.join("provider-description.json")).unwrap();
        let original_script = std::fs::read(root.join("provider.sh")).unwrap();
        let mut invalid = request;
        invalid.script_content = Some(String::new());
        assert!(
            update_custom_integration(&state, "local.restore-test", invalid)
                .await
                .is_err()
        );

        let restored = custom_integration_definition(&state, "local.restore-test").unwrap();
        assert_eq!(restored.script_content.as_deref(), Some("printf safe"));
        let restored_lock = ExtensionsLock::load(&state.paths.lock_file).unwrap();
        assert_eq!(
            serde_json::to_value(restored_lock.get("local.restore-test").unwrap()).unwrap(),
            original_lock
        );
        assert_eq!(
            std::fs::read(root.join("floter.extension.json")).unwrap(),
            original_manifest
        );
        assert_eq!(
            std::fs::read(root.join("provider-description.json")).unwrap(),
            original_descriptor
        );
        assert_eq!(
            std::fs::read(root.join("provider.sh")).unwrap(),
            original_script
        );
    }

    #[tokio::test]
    async fn custom_integration_completes_the_full_lifecycle() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let mut request =
            script_request("local.lifecycle-test", "Lifecycle test", "lifecycle-test");
        create_custom_integration(&state, request.clone())
            .await
            .unwrap();
        assert!(catalog_contains(&state, "lifecycle-test").await);

        request.name = "Lifecycle edited".into();
        request.command = "lifecycle-edited".into();
        request.version = "1.1.0".into();
        request.script_content = Some("printf edited".into());
        request.args_prefix = vec!["edited".into()];
        let edited = update_custom_integration(&state, "local.lifecycle-test", request.clone())
            .await
            .unwrap();
        state.invalidate_provider_commands().await;
        assert_eq!(edited.current_version, "1.1.0");
        assert!(catalog_contains(&state, "lifecycle-edited").await);

        set_enabled_for_test(&state, "local.lifecycle-test", false).await;
        let disabled = ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .get("local.lifecycle-test")
            .unwrap()
            .clone();
        assert!(!disabled.enabled);
        assert_eq!(disabled.state, ExtensionStateKind::Disabled);
        assert!(!catalog_contains(&state, "lifecycle-edited").await);

        set_enabled_for_test(&state, "local.lifecycle-test", true).await;
        let enabled = ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .get("local.lifecycle-test")
            .unwrap()
            .clone();
        assert!(enabled.enabled);
        assert_eq!(enabled.state, ExtensionStateKind::Enabled);
        assert!(catalog_contains(&state, "lifecycle-edited").await);

        let export = sync::build_export(&state, Utc::now()).unwrap();
        let exported = export
            .extensions
            .iter()
            .find(|entry| entry.id == "local.lifecycle-test")
            .unwrap();
        assert_eq!(exported.version, "1.1.0");
        assert_eq!(
            exported.manifest.as_ref().unwrap().permissions,
            request.permissions
        );

        let generated_root = state
            .paths
            .data
            .join("local.lifecycle-test")
            .join("integration");
        uninstall(&state, "local.lifecycle-test", false)
            .await
            .unwrap();
        state.invalidate_provider_commands().await;
        assert!(!generated_root.exists());

        let report = sync::import_document(
            &state,
            &directory.path().join("lifecycle-export.json"),
            export,
            &BTreeMap::from([(
                "local.lifecycle-test".to_string(),
                request.permissions.clone(),
            )]),
        )
        .await;
        assert!(report.failed.is_empty(), "{:?}", report.failed);
        assert_eq!(report.succeeded.len(), 1);
        state.invalidate_provider_commands().await;
        let imported = ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .get("local.lifecycle-test")
            .unwrap()
            .clone();
        assert_eq!(imported.id, "local.lifecycle-test");
        assert_eq!(imported.current_version, "1.1.0");
        assert_eq!(
            ExtensionManifest::load(Path::new(&imported.manifest_path))
                .unwrap()
                .permissions,
            request.permissions
        );
        assert!(catalog_contains(&state, "lifecycle-edited").await);

        uninstall(&state, "local.lifecycle-test", true)
            .await
            .unwrap();
        state.invalidate_provider_commands().await;
        assert!(!ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .contains_key("local.lifecycle-test"));
        assert!(!state.paths.data.join("local.lifecycle-test").exists());
        assert!(!catalog_contains(&state, "lifecycle-edited").await);

        let recreated = create_custom_integration(&state, request).await.unwrap();
        assert_eq!(recreated.id, "local.lifecycle-test");
        assert_eq!(recreated.name, "Lifecycle edited");
    }

    #[tokio::test]
    async fn deleting_generated_integration_removes_its_empty_data_root() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        create_custom_integration(
            &state,
            script_request("local.delete-test", "Delete test", "delete-test"),
        )
        .await
        .unwrap();
        let data_root = state.paths.data.join("local.delete-test");
        assert!(data_root.join("integration").is_dir());

        uninstall(&state, "local.delete-test", false).await.unwrap();

        assert!(!data_root.join("integration").exists());
        assert!(!data_root.exists());
        assert!(!ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .contains_key("local.delete-test"));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn uninstalling_a_system_integration_keeps_external_files() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let executable = directory.path().join("external-tool");
        std::fs::write(&executable, "#!/bin/sh\nprintf external\n").unwrap();
        make_executable(&executable).unwrap();

        let generated = create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.external-test".into(),
                name: "External test".into(),
                command: "external-test".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: Vec::new(),
                version_args: Vec::new(),
                permissions: vec![Permission::Environment],
                platforms: current_platforms(),
            },
        )
        .await
        .unwrap();
        let external_package = directory.path().join("external-package");
        let generated_package = Path::new(&generated.manifest_path).parent().unwrap();
        std::fs::create_dir(&external_package).unwrap();
        for file in [
            "floter.extension.json",
            "package.json",
            "provider-description.json",
        ] {
            std::fs::copy(generated_package.join(file), external_package.join(file)).unwrap();
        }
        uninstall(&state, "local.external-test", false)
            .await
            .unwrap();
        let connected = install(
            &state,
            ExtensionInstallRequest {
                source: InstallSource::Linked,
                package: None,
                version: None,
                manifest_path: Some(external_package.to_string_lossy().into_owned()),
                executable_path: Some(executable.to_string_lossy().into_owned()),
                approved_permissions: Some(vec![Permission::Environment]),
            },
        )
        .await
        .unwrap();
        assert_eq!(
            connected.runtime_ownership,
            ExtensionRuntimeOwnership::System
        );
        assert!(!is_generated_custom_integration(&connected));

        uninstall(&state, "local.external-test", false)
            .await
            .unwrap();

        assert!(executable.is_file());
        assert!(external_package.is_dir());
        assert!(!ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .contains_key("local.external-test"));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn failed_lock_save_restores_staged_npm_directory() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let extension_id = "example.rollback-test";
        let installed_root = state.paths.extensions.join(extension_id);
        std::fs::create_dir_all(&installed_root).unwrap();
        std::fs::write(installed_root.join("payload"), "installed").unwrap();
        let now = unix_now();
        let entry = ExtensionLockEntry {
            id: extension_id.into(),
            name: "Rollback test".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source: ExtensionDistributionSource::Npm,
            runtime_ownership: ExtensionRuntimeOwnership::Bundled,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: Some("floter-rollback-test".into()),
            package_version: "1.0.0".into(),
            tool_version: None,
            integrity: Some("sha512-test".into()),
            signature_verified: false,
            previous_signature_verified: None,
            official_verified: false,
            previous_official_verified: None,
            current_version: "1.0.0".into(),
            previous_version: None,
            manifest_path: installed_root
                .join("1.0.0/floter.extension.json")
                .to_string_lossy()
                .into_owned(),
            executable_path: installed_root
                .join("1.0.0/runtime/tool")
                .to_string_lossy()
                .into_owned(),
            runtime_root: Some(
                installed_root
                    .join("1.0.0/runtime")
                    .to_string_lossy()
                    .into_owned(),
            ),
            installed_at: now,
            updated_at: now,
            pinned: false,
            channel: "latest".into(),
        };
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(extension_id.into(), entry);
        lock.save(&state.paths.lock_file).unwrap();

        let original_mode = directory.path().metadata().unwrap().permissions().mode();
        std::fs::set_permissions(
            directory.path(),
            std::fs::Permissions::from_mode(original_mode & !0o222),
        )
        .unwrap();
        let result = uninstall(&state, extension_id, false).await;
        std::fs::set_permissions(
            directory.path(),
            std::fs::Permissions::from_mode(original_mode),
        )
        .unwrap();

        assert!(result.is_err());
        assert!(installed_root.join("payload").is_file());
        assert!(ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .extensions
            .contains_key(extension_id));
        assert!(!state
            .paths
            .extensions
            .read_dir()
            .unwrap()
            .flatten()
            .any(|entry| entry
                .file_name()
                .to_string_lossy()
                .starts_with(".removing-")));
    }

    fn archive_with_file(path: &str, contents: &[u8]) -> Vec<u8> {
        let encoder = flate2::write::GzEncoder::new(Vec::new(), flate2::Compression::default());
        let mut archive = tar::Builder::new(encoder);
        let mut header = tar::Header::new_gnu();
        header.set_size(contents.len() as u64);
        header.set_mode(0o755);
        header.set_cksum();
        archive
            .append_data(&mut header, path, contents)
            .expect("append archive file");
        archive
            .into_inner()
            .expect("finish tar")
            .finish()
            .expect("finish gzip")
    }

    #[test]
    fn verifies_sha512_integrity() {
        let bytes = b"extension tarball";
        let digest = base64::engine::general_purpose::STANDARD.encode(Sha512::digest(bytes));
        assert!(verify_integrity(bytes, &format!("sha512-{digest}")).is_ok());
        assert!(verify_integrity(b"changed", &format!("sha512-{digest}")).is_err());
    }

    fn signature_config(signing_key: &SigningKey) -> SignatureConfig {
        SignatureConfig {
            url: "https://example.com/floter-tool-1.0.0.sig".into(),
            public_key: format!(
                "ed25519:{}",
                base64::engine::general_purpose::STANDARD
                    .encode(signing_key.verifying_key().as_bytes())
            ),
            algorithm: SignatureAlgorithm::Ed25519,
        }
    }

    #[test]
    fn verifies_ed25519_tarball_signature() {
        let signing_key = SigningKey::from_bytes(&[7; 32]);
        let tarball = b"extension tarball";
        let signature =
            base64::engine::general_purpose::STANDARD.encode(signing_key.sign(tarball).to_bytes());

        assert!(verify_signature(
            tarball,
            signature.as_bytes(),
            &signature_config(&signing_key)
        )
        .is_ok());
    }

    #[test]
    fn verifies_a_normal_official_tarball_install() {
        let publisher = SigningKey::from_bytes(&[9; 32]);
        let publisher_key = format!(
            "ed25519:{}",
            base64::engine::general_purpose::STANDARD.encode(publisher.verifying_key().to_bytes())
        );
        let index = official_index::OfficialIndex {
            schema_version: 1,
            index_version: 1,
            expires_at: "2030-01-01T00:00:00Z".into(),
            entries: vec![official_index::OfficialIndexEntry {
                extension_id: "io.example.official".into(),
                npm_package: "floter-official".into(),
                publisher: "example".into(),
                signing_keys: vec![publisher_key],
            }],
        };
        let tarball = b"verified tarball";
        let signature =
            base64::engine::general_purpose::STANDARD.encode(publisher.sign(tarball).to_bytes());
        assert!(verify_official_tarball(
            index.authorizes(
                "io.example.official",
                "floter-official",
                "example",
                Some(&signature_config(&publisher)),
            ),
            tarball,
            signature.as_bytes(),
            &signature_config(&publisher),
        )
        .unwrap());
    }

    #[test]
    fn rejects_ed25519_signature_for_changed_tarball() {
        let signing_key = SigningKey::from_bytes(&[9; 32]);
        let signature = format!(
            "ed25519:{}\n",
            base64::engine::general_purpose::STANDARD
                .encode(signing_key.sign(b"extension tarball").to_bytes())
        );

        assert_eq!(
            verify_signature(
                b"changed tarball",
                signature.as_bytes(),
                &signature_config(&signing_key)
            )
            .unwrap_err(),
            "Extension signature verification failed"
        );
    }

    #[test]
    fn rejects_malformed_ed25519_signature_material() {
        let signing_key = SigningKey::from_bytes(&[11; 32]);
        let mut config = signature_config(&signing_key);
        config.public_key = "ed25519:not-base64!".into();
        assert!(verify_signature(b"tarball", b"not-base64!", &config).is_err());

        let config = signature_config(&signing_key);
        assert!(verify_signature(b"tarball", b"not-base64!", &config).is_err());
    }

    #[test]
    fn rejects_archive_escape_paths() {
        assert!(safe_archive_path(Path::new("package/bin/tool")).is_ok());
        assert!(safe_archive_path(Path::new("package/../tool")).is_err());
        assert!(safe_archive_path(Path::new("other/tool")).is_err());
        assert!(safe_archive_path(Path::new("/package/tool")).is_err());
    }

    #[test]
    fn safely_extracts_regular_npm_package_files() {
        let bytes = archive_with_file("package/bin/tool", b"provider");
        let directory = tempfile::tempdir().unwrap();
        safe_unpack(&bytes, directory.path()).unwrap();
        assert_eq!(
            std::fs::read(directory.path().join("bin/tool")).unwrap(),
            b"provider"
        );
    }

    #[test]
    fn validates_scoped_package_names() {
        assert!(validate_package_name("@scope/floter-tool").is_ok());
        assert!(validate_package_name("floter-tool").is_ok());
        assert!(validate_package_name("scope/tool/extra").is_err());
        assert!(validate_package_name("../tool").is_err());
        assert!(validate_package_name("@/tool").is_err());
        assert!(validate_package_name("@scope/").is_err());
        assert!(validate_package_name("UPPER").is_err());
    }

    #[test]
    fn fuzzy_path_search_prefers_exact_prefix_and_subsequence_matches() {
        assert_eq!(fuzzy_executable_score("cargo", "cargo"), Some(0));
        assert_eq!(fuzzy_executable_score("cargo-clippy", "cargo"), Some(1));
        assert_eq!(fuzzy_executable_score("my-cargo-tool", "cargo"), Some(2));
        assert_eq!(fuzzy_executable_score("cargo", "cgo"), Some(3));
        assert_eq!(fuzzy_executable_score("cargo", "xyz"), None);
    }

    #[test]
    fn permission_escalation_requires_a_new_approval() {
        let installed = [Permission::FilesystemRead];
        assert!(!has_added_permissions(&installed, &installed));
        assert!(!has_added_permissions(&installed, &[]));
        assert!(has_added_permissions(
            &installed,
            &[Permission::FilesystemRead, Permission::NetworkFetch],
        ));
    }

    #[test]
    fn installation_transaction_follows_installation_phases() {
        let mut transaction = InstallationTransaction::new();
        transaction.advance(InstallationPhase::Downloading).unwrap();
        transaction.advance(InstallationPhase::Verifying).unwrap();
        transaction.advance(InstallationPhase::Installing).unwrap();
        transaction.advance(InstallationPhase::Complete).unwrap();
    }

    #[test]
    fn installation_requires_exact_permission_confirmation() {
        let requested = [Permission::FilesystemRead, Permission::NetworkFetch];
        assert!(validate_permission_approval(&requested, None).is_err());
        assert!(validate_permission_approval(
            &requested,
            Some(&[Permission::NetworkFetch, Permission::FilesystemRead]),
        )
        .is_ok());
        assert!(
            validate_permission_approval(&requested, Some(&[Permission::FilesystemRead]),).is_err()
        );
    }

    #[test]
    fn permission_summary_is_localized() {
        let manifest = ExtensionManifest::parse(include_bytes!(
            "../../../docs/extensions/examples/v/floter.extension.json"
        ))
        .unwrap();
        let review = permission_review(&manifest, "zh-CN");
        assert_eq!(review.extension_name, "V Tools");
        assert_eq!(review.permissions[0].permission, Permission::FilesystemRead);
        assert_eq!(review.permissions[0].title, "读取文件");
    }
}
