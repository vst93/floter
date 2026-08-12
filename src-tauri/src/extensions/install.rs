use crate::extensions::lock::{
    unix_now, validate_id, write_current_pointer, ExtensionInstallType, ExtensionLockEntry,
    ExtensionProviderKind, ExtensionStateKind, ExtensionsLock,
};
use crate::extensions::manifest::{
    validate_relative_path, ExtensionManifest, Permission, PlatformTarget, Runtime,
    SignatureAlgorithm, SignatureConfig,
};
use crate::extensions::provider::ProviderInvocation;
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
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PermissionSummary {
    pub permission: Permission,
    pub title: String,
    pub description: String,
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

pub async fn install_imported_managed(
    state: &ExtensionState,
    extension_id: &str,
    package: &str,
    version: &str,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    install_managed(
        state,
        package,
        Some(version),
        Some(extension_id),
        approved_permissions,
    )
    .await
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
        install_type: ExtensionInstallType::Linked,
        provider_kind: ExtensionProviderKind::BundledStatic,
        state: ExtensionStateKind::Enabled,
        enabled: true,
        package_name: None,
        package_version: integration_version.clone(),
        tool_version,
        integrity: None,
        signature_verified: false,
        previous_signature_verified: None,
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
    let manifest = match request.source {
        InstallSource::Npm => {
            let package = request
                .package
                .as_deref()
                .ok_or("NPM installation requires a package name")?;
            validate_package_name(package)?;
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
                download_and_verify_signature(state, &bytes, signatures).await?;
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
    Ok(permission_review(&manifest, locale))
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
                    "Run additional programs from extension commands",
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
                (Permission::ProcessSpawn, true) => ("启动进程", "通过插件命令运行其他程序"),
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
    if current.install_type != ExtensionInstallType::Managed {
        return Err("Linked extensions are updated by their external package manager".to_string());
    }
    if current.pinned && version.is_none() {
        return Err(format!("Extension {extension_id} is pinned"));
    }
    let package = current
        .package_name
        .as_deref()
        .ok_or("Managed extension has no package name in the lock file")?;
    install_managed(
        state,
        package,
        version.or(Some(current.channel.as_str())),
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
    let mut moved = None;
    if entry.install_type == ExtensionInstallType::Managed {
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
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let original = lock.get(extension_id)?.clone();
    let entry = lock
        .extensions
        .get_mut(extension_id)
        .ok_or_else(|| format!("Extension is not installed: {extension_id}"))?;
    if entry.install_type != ExtensionInstallType::Managed {
        return Err("Linked extensions cannot be rolled back by Floter".to_string());
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
    let executable = managed_executable(&manifest, &previous_root)?;
    entry.executable_path = executable.to_string_lossy().into_owned();
    let runtime_root = previous_root.join("runtime");
    entry.runtime_root = Some(runtime_root.to_string_lossy().into_owned());
    let response = state
        .provider
        .describe(
            &ProviderInvocation {
                extension_id: entry.id.clone(),
                executable,
                runtime_root: Some(runtime_root),
                package_version: previous,
                tool_version_hint: None,
                version_args: Vec::new(),
                config: resolved.provider,
                permissions: manifest.permissions,
            },
            true,
        )
        .await?;
    entry.tool_version = Some(response.description.provider.version);
    let previous_signature_verified = entry.previous_signature_verified.unwrap_or(false);
    entry.previous_signature_verified = Some(entry.signature_verified);
    entry.signature_verified = previous_signature_verified;
    entry.updated_at = unix_now();
    let result = entry.clone();
    write_current_pointer(&state.paths.extensions, entry)?;
    if let Err(error) = lock.save(&state.paths.lock_file) {
        let _ = write_current_pointer(&state.paths.extensions, &original);
        return Err(error);
    }
    Ok(result)
}

pub async fn search(
    state: &ExtensionState,
    query: &str,
    limit: usize,
) -> Result<Vec<ExtensionSearchResult>, String> {
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
        .map(|object| ExtensionSearchResult {
            downloads: object.downloads.weekly,
            package: object.package.name,
            version: object.package.version,
            description: object.package.description,
            publisher: object
                .package
                .publisher
                .and_then(|publisher| publisher.username),
            homepage: object
                .package
                .links
                .get("homepage")
                .or_else(|| object.package.links.get("repository"))
                .cloned(),
            verified: false,
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
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let expected_entry = expected_id.and_then(|id| lock.extensions.get(id)).cloned();
    if let Some(entry) = expected_entry.as_ref() {
        if entry.install_type != ExtensionInstallType::Managed
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
    if let Some(signatures) = manifest.signatures.as_ref() {
        download_and_verify_signature(state, &base_bytes, signatures).await?;
    }
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
    let platform_package = resolved
        .platform_package
        .as_deref()
        .ok_or("Managed extension has no platform package")?;
    let platform_version =
        resolve_registry_version(state, platform_package, Some(&base_version.version)).await?;
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
    let invocation = ProviderInvocation {
        extension_id: manifest.id.clone(),
        executable: executable.clone(),
        runtime_root: Some(runtime_root.clone()),
        package_version: base_version.version.clone(),
        tool_version_hint: None,
        version_args: Vec::new(),
        config: resolved.provider.clone(),
        permissions: manifest.permissions.clone(),
    };
    let description = state.provider.describe(&invocation, true).await?;

    let old = lock.extensions.get(&manifest.id).cloned();
    if old
        .as_ref()
        .is_some_and(|entry| entry.install_type != ExtensionInstallType::Managed)
    {
        return Err(format!("Extension {} is already linked", manifest.id));
    }
    let extension_root = state.paths.extensions.join(&manifest.id);
    let versions_root = extension_root.join("versions");
    std::fs::create_dir_all(&versions_root)
        .map_err(|error| format!("Cannot create extension versions directory: {error}"))?;
    let target = versions_root.join(&base_version.version);
    if target.exists() {
        return Err(format!(
            "Extension version is already installed: {}",
            base_version.version
        ));
    }
    transaction.advance(InstallationPhase::Installing)?;
    std::fs::rename(&version_root, &target)
        .map_err(|error| format!("Cannot atomically install extension version: {error}"))?;

    let final_manifest = target.join(
        manifest_path
            .strip_prefix(&version_root)
            .map_err(|_| "Manifest escaped staged version root")?,
    );
    let final_runtime = target.join("runtime");
    let final_executable = final_runtime.join(match &manifest.runtime {
        Runtime::Managed { executable, .. } => validate_relative_path(executable, "executable")?,
        Runtime::Linked { .. } => return Err("NPM installation requires a managed runtime".into()),
    });
    let now = unix_now();
    let enabled = old.as_ref().is_none_or(|entry| entry.enabled);
    let entry = ExtensionLockEntry {
        id: manifest.id.clone(),
        name: manifest.name.clone(),
        publisher_id: manifest.publisher.id.clone(),
        publisher_name: manifest.publisher.name.clone(),
        install_type: ExtensionInstallType::Managed,
        provider_kind: ExtensionProviderKind::Executable,
        state: if enabled {
            ExtensionStateKind::Enabled
        } else {
            ExtensionStateKind::Disabled
        },
        enabled,
        package_name: Some(package.to_string()),
        package_version: base_version.version.clone(),
        tool_version: Some(description.description.provider.version),
        integrity: base_version.dist.integrity.clone(),
        signature_verified: manifest.signatures.is_some(),
        previous_signature_verified: old.as_ref().map(|entry| entry.signature_verified),
        current_version: base_version.version.clone(),
        previous_version: old.as_ref().map(|entry| entry.current_version.clone()),
        manifest_path: final_manifest.to_string_lossy().into_owned(),
        executable_path: final_executable.to_string_lossy().into_owned(),
        runtime_root: Some(final_runtime.to_string_lossy().into_owned()),
        installed_at: old.as_ref().map_or(now, |entry| entry.installed_at),
        updated_at: now,
        pinned: old.as_ref().is_some_and(|entry| entry.pinned),
        channel: selector
            .filter(|selector| Version::parse(selector).is_err())
            .unwrap_or("latest")
            .to_string(),
    };
    transaction.advance(InstallationPhase::Complete)?;
    if let Err(error) = write_current_pointer(&state.paths.extensions, &entry) {
        let _ = std::fs::remove_dir_all(&target);
        return Err(error);
    }
    lock.extensions.insert(entry.id.clone(), entry.clone());
    if let Err(error) = lock.save(&state.paths.lock_file) {
        let _ = std::fs::remove_dir_all(&target);
        if let Some(old) = old.as_ref() {
            let _ = write_current_pointer(&state.paths.extensions, old);
        } else if let Ok(pointer) =
            crate::extensions::lock::current_pointer_path(&state.paths.extensions, &manifest.id)
        {
            let _ = std::fs::remove_file(pointer);
        }
        return Err(error);
    }
    Ok(entry)
}

fn has_added_permissions(installed: &[Permission], requested: &[Permission]) -> bool {
    requested
        .iter()
        .any(|permission| !installed.contains(permission))
}

async fn install_linked(
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
    if !matches!(manifest.runtime, Runtime::Linked { .. }) {
        return Err("Linked installation requires a linked runtime manifest".to_string());
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
    } else {
        find_linked_executable(&manifest)?
    };
    let tool_version = linked_tool_version(&manifest, &resolved.provider, &executable).await;
    let invocation = ProviderInvocation {
        extension_id: manifest.id.clone(),
        executable: executable.clone(),
        runtime_root: None,
        package_version: package_version.clone(),
        tool_version_hint: tool_version.clone(),
        version_args: match &manifest.runtime {
            Runtime::Linked { version_args, .. } => version_args.clone(),
            Runtime::Managed { .. } => Vec::new(),
        },
        config: resolved.provider,
        permissions: manifest.permissions.clone(),
    };
    let response = state.provider.describe(&invocation, true).await?;
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
        install_type: ExtensionInstallType::Linked,
        provider_kind: ExtensionProviderKind::Executable,
        state: ExtensionStateKind::Enabled,
        enabled: true,
        package_name: request.package,
        package_version: integration_version.clone(),
        tool_version: tool_version.or(Some(response.description.provider.version)),
        integrity: None,
        signature_verified: false,
        previous_signature_verified: None,
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
    let response = state
        .client
        .get(tarball_url)
        .send()
        .await
        .map_err(|error| format!("Cannot download NPM tarball: {error}"))?
        .error_for_status()
        .map_err(|error| format!("Cannot download NPM tarball: {error}"))?;
    ensure_https_response(&response, "NPM tarball")?;
    let bytes = read_response_limited(response, MAX_TARBALL_BYTES, "NPM tarball").await?;
    verify_integrity(&bytes, integrity)?;
    Ok(bytes)
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

async fn download_and_verify_signature(
    state: &ExtensionState,
    tarball: &[u8],
    config: &SignatureConfig,
) -> Result<(), String> {
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
    verify_signature(tarball, &signature, config)
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
    let Runtime::Managed { executable, .. } = &manifest.runtime else {
        return Err("Managed installation requires a managed runtime".to_string());
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

fn make_executable(path: &Path) -> Result<(), String> {
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

fn find_linked_executable(manifest: &ExtensionManifest) -> Result<PathBuf, String> {
    let Runtime::Linked {
        executable_names, ..
    } = &manifest.runtime
    else {
        return Err("Expected linked runtime".to_string());
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
        "Cannot find linked executable: {}",
        executable_names.join(", ")
    ))
}

fn linked_candidate_names(name: &str) -> Vec<String> {
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
    let Runtime::Linked { version_args, .. } = &manifest.runtime else {
        return None;
    };
    if version_args.is_empty() {
        return None;
    }
    let mut command = tokio::process::Command::new(executable);
    if !manifest.permissions.contains(&Permission::Environment) {
        command.env_clear();
    }
    command
        .args(version_args)
        .envs(&provider.environment)
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
    use ed25519_dalek::{Signer, SigningKey};

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
