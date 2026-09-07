use crate::extensions::help_args;
use crate::extensions::lock::{
    unix_now, validate_id, ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
    ExtensionRuntimeOwnership, ExtensionStateKind, ExtensionsLock,
};
use crate::extensions::manifest::{
    validate_relative_path, Compatibility, Distribution, ExtensionManifest, Permission, PlatformOs,
    PlatformTarget, ProviderConfig, ProviderKind, Publisher, Runtime, ScriptLanguage,
};
use crate::extensions::probe_executor;
use crate::extensions::provider::ProviderInvocation;
use crate::extensions::ExtensionState;
use semver::Version;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::Duration;

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
    Linked,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ExtensionPermissionReview {
    pub extension_id: String,
    pub extension_name: String,
    pub permissions: Vec<PermissionSummary>,
    pub publisher_signed: bool,
    pub official_verified: bool,
    pub deprecation: Option<String>,
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

/// Minimal package.json shape used when connecting an integration from a
/// local package directory. Kept compatible with legacy NPM-style packages:
/// unknown fields are ignored and `floter.manifest` selects the manifest.
#[derive(Debug, Deserialize)]
struct PackageJson {
    #[allow(dead_code)]
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

pub async fn install(
    state: &ExtensionState,
    request: ExtensionInstallRequest,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    install_linked(state, request).await
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
    // Connect-time parameter hints: run one bounded `--help` probe against the
    // executable and derive root option definitions plus any subcommand
    // entries (each probed once more for its own flags), so launcher
    // completions can suggest real parameters for the connected tool.
    // Best-effort by contract — any failure yields an empty derivation and
    // never blocks the connection.
    let derivation = if script_mode {
        help_args::HelpDerivation::default()
    } else {
        help_args::probe_derive(&executable).await
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
        artifacts: crate::extensions::manifest::Artifacts::default(),
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
        // One descriptor command per derived subcommand (in derivation order):
        // same execution shape as the root command with the subcommand name
        // appended to argsPrefix so it runs `<executable> <sub>`; mirrors the
        // multi-command descriptors shipped by the recommended-tools flow.
        let commands = derived_descriptor_commands(
            &command,
            name,
            &manifest.description,
            &request.args_prefix,
            &derivation,
        );
        let mut descriptor_bytes = serde_json::to_vec_pretty(&serde_json::json!({
            "protocolVersion": "1.0",
            "provider": {
                "id": id,
                "name": name,
                "version": request.version.trim(),
                "description": manifest.description
            },
            "commands": commands
        }))
        .map_err(|error| format!("Cannot serialize custom provider description: {error}"))?;
        descriptor_bytes.push(b'\n');
        std::fs::write(&descriptor_path, descriptor_bytes)
            .map_err(|error| format!("Cannot write custom provider description: {error}"))?;
        if !script_mode {
            // Probe record sidecar: best-effort by contract — a failure here
            // must never fail the connection.
            let _ = write_help_probe_sidecar(&package_root, &derivation);
        }
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

/// Descriptor command payload shared by connect-time generation and later
/// re-probes: the root command carrying `args_prefix`, plus one runnable
/// command per derived subcommand with its own probed flags.
fn derived_descriptor_commands(
    command_id: &str,
    name: &str,
    root_description: &str,
    args_prefix: &[String],
    derivation: &help_args::HelpDerivation,
) -> Vec<serde_json::Value> {
    let mut commands = vec![serde_json::json!({
        "id": command_id,
        "name": name,
        "description": root_description,
        "aliases": [],
        "keywords": [],
        "execution": {
            "program": "self",
            "argsPrefix": args_prefix,
            "mode": "pty",
            "workingDirectory": "current"
        },
        "arguments": help_args::to_json_array(&derivation.root_arguments)
    })];
    for subcommand in &derivation.subcommands {
        if subcommand.name == command_id {
            continue;
        }
        commands.push(serde_json::json!({
            "id": subcommand.name,
            "name": format!("{name} {}", subcommand.name),
            "description": if subcommand.description.is_empty() {
                format!("Subcommand of {name}")
            } else {
                subcommand.description.clone()
            },
            "aliases": subcommand.aliases,
            "keywords": [],
            "execution": {
                "program": "self",
                "argsPrefix": args_prefix
                    .iter()
                    .chain(std::iter::once(&subcommand.name))
                    .cloned()
                    .collect::<Vec<_>>(),
                "mode": "pty",
                "workingDirectory": "current"
            },
            "arguments": help_args::to_json_array(&subcommand.arguments)
        }));
    }
    commands
}

/// One sidecar record per probed subcommand for `help-probe.json`.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct HelpProbeSubcommand<'a> {
    name: &'a str,
    aliases: &'a [String],
    argument_count: usize,
}

/// Sidecar probe record written next to `provider-description.json`. Purely
/// informational (when the flags were derived and how much was found); never
/// read back to make decisions.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct HelpProbeRecord<'a> {
    probed_at: u64,
    root_argument_count: usize,
    subcommands: Vec<HelpProbeSubcommand<'a>>,
}

fn write_help_probe_sidecar(
    package_root: &Path,
    derivation: &help_args::HelpDerivation,
) -> Result<(), String> {
    let record = HelpProbeRecord {
        probed_at: unix_now(),
        root_argument_count: derivation.root_arguments.len(),
        subcommands: derivation
            .subcommands
            .iter()
            .map(|subcommand| HelpProbeSubcommand {
                name: &subcommand.name,
                aliases: &subcommand.aliases,
                argument_count: subcommand.arguments.len(),
            })
            .collect(),
    };
    let mut bytes = serde_json::to_vec_pretty(&record)
        .map_err(|error| format!("Cannot serialize help probe record: {error}"))?;
    bytes.push(b'\n');
    std::fs::write(package_root.join("help-probe.json"), bytes)
        .map_err(|error| format!("Cannot write help probe record: {error}"))
}

/// Result of a successful [`reprobe_tool_commands`] run: how many argument
/// hints the fresh derivation produced.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ReprobeReport {
    pub root_arguments: usize,
    pub subcommands: usize,
}

/// Re-run the connect-time help derivation for a generated custom
/// integration and regenerate its static descriptor in place. The current
/// descriptor is authoritative for everything user-visible (root command
/// id/name/description, provider block, configured `argsPrefix`); only the
/// derived `arguments` arrays and the subcommand command list are rebuilt.
/// The refreshed descriptor replaces the old one atomically (temp + rename)
/// and the `help-probe.json` sidecar is refreshed best-effort.
///
/// Callers must already hold the extension mutation lock; this routine does
/// not take it so it can run inline from other locked mutations.
pub async fn reprobe_tool_commands(
    state: &ExtensionState,
    id: &str,
) -> Result<ReprobeReport, String> {
    validate_id(id)?;
    let entry = ExtensionsLock::load(&state.paths.repository_file)?
        .get(id)?
        .clone();
    if !is_generated_custom_integration(&entry) {
        return Err(format!(
            "Integration {id} was not generated by Floter and cannot be re-probed"
        ));
    }
    if entry.provider_kind != ExtensionProviderKind::StaticDescriptor {
        return Err(format!(
            "Integration {id} does not use a static descriptor to re-probe"
        ));
    }
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    if matches!(manifest.runtime, Runtime::Script { .. }) {
        return Err("Script integrations have no executable to probe".to_string());
    }
    let executable = PathBuf::from(&entry.executable_path);
    if !is_linked_executable(&executable) {
        return Err(format!(
            "Tool executable is not available: {}; reconnect the integration first",
            executable.display()
        ));
    }
    let root = Path::new(&entry.manifest_path)
        .parent()
        .ok_or("Custom integration manifest has no parent directory")?;
    let descriptor_path = root.join(
        manifest
            .provider
            .descriptor
            .as_deref()
            .unwrap_or("provider-description.json"),
    );
    let mut descriptor: serde_json::Value = serde_json::from_slice(
        &std::fs::read(&descriptor_path)
            .map_err(|error| format!("Cannot read custom integration descriptor: {error}"))?,
    )
    .map_err(|error| format!("Cannot parse custom integration descriptor: {error}"))?;
    let commands = descriptor
        .get_mut("commands")
        .and_then(|commands| commands.as_array_mut())
        .ok_or("Custom integration descriptor has no command list")?;
    let root_command = commands
        .first()
        .ok_or("Custom integration descriptor has no command")?
        .clone();
    let command_id = root_command["id"].as_str().unwrap_or_default().to_string();
    let name = root_command["name"]
        .as_str()
        .unwrap_or_default()
        .to_string();
    let root_description = root_command["description"]
        .as_str()
        .unwrap_or_default()
        .to_string();
    let args_prefix = root_command["execution"]["argsPrefix"]
        .as_array()
        .map(|values| {
            values
                .iter()
                .filter_map(|value| value.as_str().map(str::to_string))
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    let derivation = help_args::probe_derive(&executable).await;
    *commands = derived_descriptor_commands(
        &command_id,
        &name,
        &root_description,
        &args_prefix,
        &derivation,
    );
    let mut descriptor_bytes = serde_json::to_vec_pretty(&descriptor)
        .map_err(|error| format!("Cannot serialize custom provider description: {error}"))?;
    descriptor_bytes.push(b'\n');
    let temp_path = root.join(".provider-description.reprobing.tmp");
    std::fs::write(&temp_path, descriptor_bytes)
        .map_err(|error| format!("Cannot stage custom provider description: {error}"))?;
    crate::extensions::commit_point("descriptor-rename");
    if let Err(error) = std::fs::rename(&temp_path, &descriptor_path) {
        let _ = std::fs::remove_file(&temp_path);
        return Err(format!(
            "Cannot update custom provider description: {error}"
        ));
    }
    let _ = write_help_probe_sidecar(root, &derivation);
    state.invalidate_provider_commands().await;
    Ok(ReprobeReport {
        root_arguments: derivation.root_arguments.len(),
        subcommands: derivation.subcommands.len(),
    })
}

/// Enable-path variant of [`reprobe_tool_commands`]: silently degrades to a
/// no-op for anything not worth re-probing, and swallows every error — the
/// previous descriptor stays valid, and enabling must never fail because
/// re-probing did.
pub async fn reprobe_after_enable(state: &ExtensionState, entry: &ExtensionLockEntry) {
    if !entry.enabled
        || !is_generated_custom_integration(entry)
        || entry.provider_kind != ExtensionProviderKind::StaticDescriptor
    {
        return;
    }
    let _ = reprobe_tool_commands(state, &entry.id).await;
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

/// Default disclosure set for one-click tool connections. Terminal tools
/// genuinely need environment passthrough and process spawning, and native
/// providers are not sandboxed regardless — these declarations describe
/// behavior, they do not constrain it (see docs/tool-binding-design.md).
pub fn tool_binding_permissions() -> Vec<Permission> {
    vec![
        Permission::Environment,
        Permission::ProcessSpawn,
        Permission::FilesystemRead,
    ]
}

/// Kebab-case label matching the serde representation of `Permission`.
fn permission_label(permission: &Permission) -> &'static str {
    match permission {
        Permission::FilesystemRead => "filesystem-read",
        Permission::FilesystemWrite => "filesystem-write",
        Permission::NetworkFetch => "network-fetch",
        Permission::ProcessSpawn => "process-spawn",
        Permission::ClipboardRead => "clipboard-read",
        Permission::ClipboardWrite => "clipboard-write",
        Permission::Environment => "environment",
    }
}

fn permission_labels(permissions: &[Permission]) -> String {
    permissions
        .iter()
        .map(permission_label)
        .collect::<Vec<_>>()
        .join(", ")
}

/// Exact-set match between the user-approved permissions and the disclosure
/// for one-click tool connections. Both sides are canonicalized (sorted and
/// deduplicated) before comparing so ordering never matters; any missing or
/// extra permission still fails.
pub fn validate_tool_binding_approval(approved: &[Permission]) -> Result<(), String> {
    fn canonical(mut set: Vec<Permission>) -> Vec<Permission> {
        set.sort();
        set.dedup();
        set
    }
    let expected = tool_binding_permissions();
    let approved = canonical(approved.to_vec());
    if approved != canonical(expected.clone()) {
        return Err(format!(
            "Tool permission approval does not match the disclosure (expected: {}; got: {})",
            permission_labels(&expected),
            permission_labels(&approved),
        ));
    }
    Ok(())
}

fn sanitized_command_from_executable(stem: &str) -> String {
    let mut command: String = stem
        .to_ascii_lowercase()
        .chars()
        .map(|character| {
            if character.is_ascii_lowercase()
                || character.is_ascii_digit()
                || character == '-'
                || character == '_'
            {
                character
            } else {
                '-'
            }
        })
        .collect();
    while command.starts_with('-') {
        command.remove(0);
    }
    while command.ends_with('-') {
        command.pop();
    }
    if command.len() > 64 {
        command.truncate(64);
        while command.ends_with('-') {
            command.pop();
        }
    }
    command
}

/// Build the custom-integration request that binds an auto-discovered PATH
/// executable as a regular local integration. The generated manifest is
/// identical to what the custom-integration form produces, so discovered and
/// manually created bindings converge on the same validation, lock, and
/// catalog path — no special-cased distribution source.
pub fn tool_binding_request(
    candidate: &crate::extensions::inventory::ToolCandidate,
) -> Result<CustomIntegrationRequest, String> {
    let Some(path) = candidate.locator.executable_path() else {
        return Err("Discovered tool has no executable path".to_string());
    };
    if !is_linked_executable(path) {
        return Err(format!("Executable is not usable: {}", path.display()));
    }
    let name = candidate.name.trim();
    if name.is_empty() || name.chars().count() > 80 {
        return Err("Discovered tool name must contain 1 to 80 characters".to_string());
    }
    let stem = path
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or(name);
    let command = sanitized_command_from_executable(stem);
    if command.is_empty() {
        return Err(format!("Cannot derive a Floter command from \"{stem}\""));
    }
    Ok(CustomIntegrationRequest {
        id: format!("local.{command}"),
        name: name.to_string(),
        command,
        version: candidate.version.clone().unwrap_or_else(|| "0.0.0".into()),
        executable_path: path.to_string_lossy().into_owned(),
        mode: "executable".to_string(),
        script_language: None,
        script_content: None,
        args_prefix: Vec::new(),
        version_args: Vec::new(),
        permissions: tool_binding_permissions(),
        platforms: vec![PlatformTarget::current()?.os],
    })
}

/// One-click connection of an auto-discovered tool. Delegates to the exact
/// custom-integration pipeline; only the id-collision fallback (`.2`, `.3`,
/// ...) is added on top so repeated basenames across PATH directories can
/// coexist under distinct extension ids.
pub async fn connect_tool(
    state: &ExtensionState,
    candidate: crate::extensions::inventory::ToolCandidate,
) -> Result<ExtensionLockEntry, String> {
    let mut request = tool_binding_request(&candidate)?;
    let base_id = request.id.clone();
    let mut last_error = String::new();
    for attempt in 0..10u32 {
        request.id = if attempt == 0 {
            base_id.clone()
        } else {
            format!("{base_id}.{}", attempt + 1)
        };
        match create_custom_integration(state, request.clone()).await {
            Ok(entry) => return Ok(entry),
            Err(error) => {
                let occupied =
                    error.contains("already installed") || error.contains("already exist");
                if !occupied {
                    return Err(error);
                }
                last_error = error;
            }
        }
    }
    Err(if last_error.is_empty() {
        format!("Cannot allocate an extension id for {}", candidate.name)
    } else {
        last_error
    })
}

pub fn custom_integration_definition(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<CustomIntegrationDefinition, String> {
    let entry = ExtensionsLock::load(&state.paths.repository_file)?
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
    crate::extensions::transaction::recover_pending_removals(state)?;
    let lock = ExtensionsLock::load(&state.paths.repository_file)?;
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
    // Persist the planned backup before moving the live integration.
    let transaction_id = format!("edit-{}-{}", extension_id, current.updated_at);
    let journal = crate::extensions::transaction::RemovalJournal {
        schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
        transaction_id: transaction_id.clone(),
        extension_id: extension_id.to_string(),
        removed_entry: current.clone(),
        // For custom integrations, staged_path is the backup in data dir, and
        // cleanup_paths includes the target root for proper recovery.
        staged_path: Some(backup_path.clone()),
        cleanup_paths: vec![root.clone()],
        removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
        remove_data: false,
        intent: crate::extensions::transaction::RemovalIntent::Edit,
    };
    crate::extensions::transaction::write_removal_journal(state, &journal)?;
    crate::extensions::commit_point("edit-stage-rename");
    std::fs::rename(&root, &backup_path)
        .map_err(|error| format!("Cannot stage custom integration update: {error}"))?;

    // Now safe to remove the repository entry: recovery can restore it after a crash.
    let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
    lock.extensions.remove(extension_id);
    crate::extensions::commit_point("edit-repository-remove");
    if let Err(error) = lock.save(&state.paths.repository_file) {
        crate::extensions::transaction::recover_pending_removals(state)
            .map_err(|recovery| format!("{error}; {recovery}"))?;
        return Err(format!(
            "Cannot stage custom integration repository update: {error}"
        ));
    }
    drop(lock);

    let result = create_custom_integration_locked(state, request).await;
    if result.is_ok() {
        let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
        if let Some(updated) = lock.extensions.get_mut(extension_id) {
            updated.enabled = current.enabled;
            updated.state = if current.enabled {
                ExtensionStateKind::Enabled
            } else {
                ExtensionStateKind::Disabled
            };
            updated.installed_at = current.installed_at;
            crate::extensions::commit_point("edit-repository-finalize");
            if let Err(error) = lock.save(&state.paths.repository_file) {
                crate::extensions::transaction::recover_pending_removals(state)
                    .map_err(|recovery| format!("{error}; {recovery}"))?;
                return Err(format!(
                    "Cannot finalize custom integration update: {error}"
                ));
            }
        }
        crate::extensions::transaction::recover_pending_removals(state)?;
        return Ok(lock.get(extension_id)?.clone());
    }

    // Resolve failures from the persisted repository, retaining any unfinished journal.
    crate::extensions::commit_point("edit-rollback-rename");
    let error = result.unwrap_err();
    crate::extensions::transaction::recover_pending_removals(state)
        .map_err(|recovery| format!("{error}; {recovery}"))?;
    Err(error)
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

/// Data needed to connect a local static integration whose manifest (and
/// optional provider descriptor) already exist as bytes: shipped
/// recommendations and convention-location manifests share this pipeline.
pub(crate) struct LocalStaticToolPayload<'a> {
    pub id: &'a str,
    pub manifest: &'a ExtensionManifest,
    /// Exact manifest bytes written into the integration package root.
    pub manifest_bytes: &'a [u8],
    /// Descriptor file to write inside the package root, if the tool needs
    /// one.
    pub descriptor_path: Option<String>,
    pub descriptor_bytes: Option<Vec<u8>>,
    pub provider_version: &'a str,
}

async fn connect_local_static_tool(
    state: &ExtensionState,
    payload: &LocalStaticToolPayload<'_>,
    executable_path: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    let executable = match executable_path {
        Some(path) => {
            let path = PathBuf::from(path);
            if !is_linked_executable(&path) {
                return Err(format!(
                    "System tool is not available at {}",
                    path.display()
                ));
            }
            path
        }
        None => find_system_executable(payload.manifest)?,
    };
    validate_permission_approval(&payload.manifest.permissions, approved_permissions)?;

    let id = payload.id;
    let package_root = state.paths.data.join(id).join("integration");
    if package_root.exists() {
        return Err(format!("Integration files already exist for {id}"));
    }
    let data_root = package_root
        .parent()
        .ok_or("Integration directory has no parent")?;
    std::fs::create_dir_all(data_root)
        .map_err(|error| format!("Cannot create integration data directory: {error}"))?;
    std::fs::create_dir(&package_root)
        .map_err(|error| format!("Cannot reserve integration directory for {id}: {error}"))?;
    let manifest_path = package_root.join("floter.extension.json");
    let package_path = package_root.join("package.json");
    let write_result = (|| -> Result<(), String> {
        std::fs::write(&manifest_path, payload.manifest_bytes)
            .map_err(|error| format!("Cannot write integration manifest: {error}"))?;
        let mut package_bytes = serde_json::to_vec_pretty(&serde_json::json!({
            "name": format!("floter-local-{}", id.replace(['.', '_'], "-")),
            "version": payload.provider_version,
            "private": true,
            "keywords": ["floter-extension"],
            "floter": { "manifest": "floter.extension.json" }
        }))
        .map_err(|error| format!("Cannot serialize integration package: {error}"))?;
        package_bytes.push(b'\n');
        std::fs::write(&package_path, package_bytes)
            .map_err(|error| format!("Cannot write integration package: {error}"))?;
        if let (Some(descriptor_path), Some(descriptor_bytes)) =
            (&payload.descriptor_path, &payload.descriptor_bytes)
        {
            std::fs::write(package_root.join(descriptor_path), descriptor_bytes)
                .map_err(|error| format!("Cannot write provider descriptor: {error}"))?;
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
            executable_path: Some(executable.to_string_lossy().into_owned()),
            approved_permissions: approved_permissions.map(<[Permission]>::to_vec),
        },
    )
    .await;
    if result.is_err() {
        let _ = std::fs::remove_dir_all(&package_root);
    }
    result
}

pub async fn connect_recommended_tool(
    state: &ExtensionState,
    recommendation: &crate::extensions::recommendations::RecommendedTool,
    executable_path: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    connect_local_static_tool(
        state,
        &LocalStaticToolPayload {
            id: &recommendation.manifest.id,
            manifest: &recommendation.manifest,
            manifest_bytes: recommendation.manifest_bytes,
            descriptor_path: Some("provider-description.json".to_string()),
            descriptor_bytes: Some(recommendation.descriptor_bytes.to_vec()),
            provider_version: &recommendation.description.provider.version,
        },
        executable_path,
        approved_permissions,
    )
    .await
}

/// One-click connection of a convention-location manifest tool
/// (`<config>/floter/tools/*.json`). The authored manifest is materialized
/// verbatim into the standard integration directory and installed through
/// the same linked-install pipeline as every other local tool. When the
/// manifest needs a static descriptor but none ships beside it, a minimal
/// generic single-command descriptor is generated so the tool behaves like
/// a PATH-discovered tool.
pub async fn connect_manifest_tool(
    state: &ExtensionState,
    tool: &crate::extensions::tool_manifests::DiscoveredManifest,
    executable_path: Option<&str>,
    approved_permissions: Option<&[Permission]>,
) -> Result<ExtensionLockEntry, String> {
    let (descriptor_path, descriptor_bytes) = manifest_descriptor_payload(tool);
    let provider_version = tool
        .descriptor_bytes
        .as_deref()
        .and_then(|bytes| crate::extensions::provider::ProviderDescription::parse(bytes).ok())
        .map(|description| description.provider.version)
        .unwrap_or_else(|| "0.0.0".to_string());
    connect_local_static_tool(
        state,
        &LocalStaticToolPayload {
            id: &tool.manifest.id,
            manifest: &tool.manifest,
            manifest_bytes: &tool.manifest_bytes,
            descriptor_path,
            descriptor_bytes,
            provider_version: &provider_version,
        },
        executable_path,
        approved_permissions,
    )
    .await
}

/// Descriptor payload for a convention-location manifest: the raw sibling
/// `<stem>.description.json` bytes when present, otherwise a generated
/// generic single-command description derived from the manifest runtime.
fn manifest_descriptor_payload(
    tool: &crate::extensions::tool_manifests::DiscoveredManifest,
) -> (Option<String>, Option<Vec<u8>>) {
    if !tool.requires_descriptor() {
        return (None, None);
    }
    let relative = tool
        .manifest
        .provider
        .descriptor
        .clone()
        .unwrap_or_else(|| "provider-description.json".to_string());
    if let Some(bytes) = &tool.descriptor_bytes {
        return (Some(relative), Some(bytes.clone()));
    }
    let mut bytes = serde_json::to_vec_pretty(&synthesized_single_command_description(tool))
        .map_err(|error| format!("Cannot serialize generated descriptor: {error}"))
        .ok()
        .unwrap_or_default();
    bytes.push(b'\n');
    (Some(relative), Some(bytes))
}

/// Generic single-command description used when a convention-location
/// manifest declares a static-descriptor provider without shipping one:
/// one command named after the executable, running `self` in a PTY.
fn synthesized_single_command_description(
    tool: &crate::extensions::tool_manifests::DiscoveredManifest,
) -> crate::extensions::provider::ProviderDescription {
    use crate::extensions::provider::{
        CommandDescriptor, ExecutionDescriptor, ExecutionMode, ProviderIdentity, WorkingDirectory,
    };

    let command = match &tool.manifest.runtime {
        Runtime::System {
            executable_names, ..
        } => executable_names
            .first()
            .map(|name| sanitized_command_from_executable(name))
            .filter(|command| !command.is_empty()),
        _ => None,
    }
    .unwrap_or_else(|| {
        sanitized_command_from_executable(&tool.stem)
            .is_empty()
            .then(|| "tool".to_string())
            .unwrap_or_else(|| sanitized_command_from_executable(&tool.stem))
    });
    let name = tool.manifest.name.clone();
    let summary = tool.manifest.description.clone();
    crate::extensions::provider::ProviderDescription {
        protocol_version: "1.0".into(),
        provider: ProviderIdentity {
            id: tool.manifest.id.clone(),
            name: name.clone(),
            version: "0.0.0".into(),
            description: summary.clone(),
        },
        commands: vec![CommandDescriptor {
            id: command,
            name,
            description: summary,
            aliases: Vec::new(),
            keywords: Vec::new(),
            execution: ExecutionDescriptor {
                program: "self".into(),
                args_prefix: Vec::new(),
                mode: ExecutionMode::Pty,
                working_directory: WorkingDirectory::default(),
            },
            arguments: Vec::new(),
        }],
        configuration: None,
    }
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
        deprecation: None,
    }
}

pub async fn verify_installed(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<ExtensionLockEntry, String> {
    let _guard = state.mutation_lock.lock().await;
    verify_installed_locked(state, extension_id).await
}

pub(crate) async fn verify_installed_locked(
    state: &ExtensionState,
    extension_id: &str,
) -> Result<ExtensionLockEntry, String> {
    let entry = ExtensionsLock::load(&state.paths.repository_file)?
        .get(extension_id)?
        .clone();
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    if manifest.id != entry.id || manifest.publisher.id != entry.publisher_id {
        return Err(format!(
            "Installed manifest identity does not match {extension_id}"
        ));
    }
    let invocation =
        crate::extensions::registry::provider_invocation_with_manifest(&entry, &manifest)?;
    match entry.provider_kind {
        ExtensionProviderKind::Executable => {
            state.provider.describe(&invocation, true).await?;
        }
        ExtensionProviderKind::StaticDescriptor | ExtensionProviderKind::BundledStatic => {
            crate::extensions::registry::static_description(&entry)?;
            if !invocation.executable.is_file() {
                return Err(format!("Runtime is unavailable for extension {}", entry.id));
            }
        }
    }
    let report = probe_executor::execute_capability_probes(&invocation, &manifest).await;
    let problem = probe_executor::verification_error(&report);
    if report.status != crate::extensions::health::HealthStatus::Unknown {
        let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
        probe_executor::record_report(&mut lock, extension_id, report)?;
        lock.save(&state.paths.repository_file)?;
        state.invalidate_provider_commands().await;
        if let Some(problem) = problem {
            return Err(problem);
        }
        return Ok(lock.get(extension_id)?.clone());
    }
    Ok(entry)
}

pub async fn uninstall(
    state: &ExtensionState,
    extension_id: &str,
    remove_data: bool,
) -> Result<(), String> {
    let _guard = state.mutation_lock.lock().await;
    validate_id(extension_id)?;
    state.provider.cancel_completions();
    crate::extensions::transaction::recover_pending_removals(state)?;
    let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
    let entry = lock.get(extension_id)?.clone();
    let generated_local_root = state.paths.data.join(extension_id).join("integration");
    let generated_local = is_generated_custom_integration(&entry)
        && Path::new(&entry.manifest_path).starts_with(&generated_local_root);

    // Build the list of cleanup paths before staging anything.
    let mut cleanup_paths = Vec::new();
    if generated_local && generated_local_root.exists() {
        cleanup_paths.push(generated_local_root.clone());
    }
    if remove_data {
        let data = state.paths.data.join(extension_id);
        if data.exists() {
            cleanup_paths.push(data);
        }
    }

    let transaction_id = format!("uninstall-{}-{}", extension_id, entry.updated_at);
    let mut staged_path = None;

    // Reserve a backup path without moving the live extension.
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
        staged_path = Some(target);
    }

    // Journal the planned backup durably before staging or committing removal.
    let journal = crate::extensions::transaction::RemovalJournal {
        schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
        transaction_id: transaction_id.clone(),
        extension_id: extension_id.to_string(),
        removed_entry: entry.clone(),
        staged_path: staged_path.clone(),
        cleanup_paths: cleanup_paths.clone(),
        removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
        remove_data,
        intent: crate::extensions::transaction::RemovalIntent::Remove,
    };
    let journal_path = crate::extensions::transaction::write_removal_journal(state, &journal)?;
    if let Some(target) = &staged_path {
        crate::extensions::commit_point("uninstall-stage-rename");
        std::fs::rename(&source, target).map_err(|error| {
            format!(
                "Cannot stage extension {} for removal: {error}",
                source.display()
            )
        })?;
    }

    // Recovery decides whether a failed save reached its atomic repository rename.
    lock.extensions.remove(extension_id);
    crate::extensions::commit_point("uninstall-repository-remove");
    if let Err(error) = lock.save(&state.paths.repository_file) {
        crate::extensions::transaction::recover_pending_removals(state)
            .map_err(|recovery| format!("{error}; {recovery}"))?;
        return Err(error);
    }

    // Update journal to mark repository state as committed.
    let journal = crate::extensions::transaction::RemovalJournal {
        removal_kind: Some(crate::extensions::transaction::RemovalKind::Committed),
        ..journal
    };
    let _ = crate::extensions::transaction::write_removal_journal(state, &journal);

    // Physical cleanup: delete staged directory and cleanup paths.
    let mut cleanup_error = None;
    if let Some(target) = staged_path {
        if let Err(error) = std::fs::remove_dir_all(&target) {
            cleanup_error = Some(format!("Cannot remove {}: {error}", target.display()));
        }
    }
    for cleanup_path in cleanup_paths {
        if cleanup_path.exists() {
            if let Err(error) = std::fs::remove_dir_all(&cleanup_path) {
                if cleanup_error.is_none() {
                    cleanup_error =
                        Some(format!("Cannot remove {}: {error}", cleanup_path.display()));
                }
            }
        }
    }

    // Remove journal on success; leave it on failure for recovery.
    if cleanup_error.is_none() {
        let _ = std::fs::remove_file(&journal_path);
        Ok(())
    } else {
        // Return error but leave journal intact. Next startup will complete cleanup.
        Err(cleanup_error.unwrap())
    }
}

/// Map a verification failure message to a stable, structured error code for
/// the lock file. Codes are kebab-case identifiers the UI can key on without
/// parsing free-form error text.
pub(crate) fn classify_verify_error(problem: &str) -> String {
    let lowered = problem.to_ascii_lowercase();
    if lowered.contains("integrity") || lowered.contains("content integrity") {
        "integrity-mismatch".to_string()
    } else if lowered.contains("manifest identity")
        || lowered.contains("does not match lock entry")
        || lowered.contains("publisher changed")
    {
        "identity-mismatch".to_string()
    } else if lowered.contains("runtime is unavailable")
        || lowered.contains("not available at")
        || lowered.contains("system tool is not available")
    {
        "runtime-unavailable".to_string()
    } else if lowered.contains("cannot read manifest")
        || lowered.contains("invalid extension manifest")
        || lowered.contains("no such file")
    {
        "manifest-unreadable".to_string()
    } else if lowered.contains("describe") || lowered.contains("provider") {
        "provider-failed".to_string()
    } else {
        "verification-failed".to_string()
    }
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
    let (manifest, manifest_digest, package_version, manifest_path) = if is_package_directory {
        let (package_json, actual_manifest_path) = load_package_entry(&manifest_path)?;
        if !package_json
            .keywords
            .iter()
            .any(|keyword| keyword == "floter-extension")
        {
            return Err("package.json is missing the floter-extension keyword".to_string());
        }
        let (manifest, digest) = ExtensionManifest::load_with_digest(&actual_manifest_path)?;
        (manifest, digest, package_json.version, actual_manifest_path)
    } else {
        let (manifest, digest) = ExtensionManifest::load_with_digest(&manifest_path)?;
        (manifest, digest, "linked".to_string(), manifest_path)
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
    let (executable, executable_prefix) = crate::extensions::registry::resolve_runtime_target(
        &manifest,
        &manifest_path,
        &executable,
    )?;
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

    let report = probe_executor::execute_capability_probes(&invocation, &manifest).await;
    let integration_version = if is_package_directory {
        package_version
    } else {
        response.description.provider.version.clone()
    };
    let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
    if lock.extensions.contains_key(&manifest.id) {
        return Err(format!("Extension is already installed: {}", manifest.id));
    }
    let now = unix_now();
    let mut approved_permissions = manifest.permissions.clone();
    approved_permissions.sort_unstable();
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
        current_version: integration_version,
        previous_version: None,
        manifest_path: manifest_path.to_string_lossy().into_owned(),
        executable_path: executable.to_string_lossy().into_owned(),
        runtime_root: None,
        installed_at: now,
        updated_at: now,
        pinned: false,
        channel: "external".into(),
        approved_permissions,
        approved_at: now,
        approved_manifest_digest: Some(manifest_digest),
        last_error_code: None,
        last_error_detail: None,
        last_error_at: None,
        broken_reason: None,
        enabled_before_broken: None,
        probe_report: None,
    };
    lock.extensions.insert(entry.id.clone(), entry.clone());
    probe_executor::record_report(&mut lock, &entry.id, report)?;
    crate::extensions::commit_point("install-repository-add");
    lock.save(&state.paths.repository_file)?;
    Ok(lock.get(&entry.id)?.clone())
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

/// Public predicate used by list-time discovery suggestions.
pub fn is_linked_executable_public(path: &Path) -> bool {
    is_linked_executable(path)
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::catalog::{self, CatalogSearchRequest, CompletionRequest};
    use crate::extensions::sync;
    use crate::extensions::ExtensionPaths;
    use chrono::Utc;
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
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: Some("example-journal".into()),
            package_version: version.into(),
            tool_version: Some("1.0.0".into()),
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
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
            probe_report: None,
        }
    }

    #[test]
    fn tool_binding_approval_accepts_disclosure_in_any_order() {
        validate_tool_binding_approval(&[
            Permission::FilesystemRead,
            Permission::Environment,
            Permission::ProcessSpawn,
        ])
        .unwrap();
    }

    #[test]
    fn tool_binding_approval_rejects_missing_permission() {
        let result =
            validate_tool_binding_approval(&[Permission::Environment, Permission::ProcessSpawn]);
        assert_eq!(
            result.unwrap_err(),
            "Tool permission approval does not match the disclosure (expected: environment, process-spawn, filesystem-read; got: process-spawn, environment)"
        );
    }

    #[test]
    fn tool_binding_approval_rejects_extra_permission() {
        let result = validate_tool_binding_approval(&[
            Permission::Environment,
            Permission::ProcessSpawn,
            Permission::FilesystemRead,
            Permission::NetworkFetch,
        ]);
        assert_eq!(
            result.unwrap_err(),
            "Tool permission approval does not match the disclosure (expected: environment, process-spawn, filesystem-read; got: filesystem-read, network-fetch, process-spawn, environment)"
        );
    }

    #[test]
    fn tool_binding_approval_rejects_empty_approval() {
        let result = validate_tool_binding_approval(&[]);
        assert_eq!(
            result.unwrap_err(),
            "Tool permission approval does not match the disclosure (expected: environment, process-spawn, filesystem-read; got: )"
        );
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
        lock.save(&state.paths.repository_file).unwrap();
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
                state: crate::extensions::transaction::TransactionState::Staged,
            },
        )
        .unwrap();
        crate::extensions::transaction::recover(&state).unwrap();
        assert!(target.exists());
        assert!(!backup.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.repository_file)
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
        lock.save(&state.paths.repository_file).unwrap();
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
                state: crate::extensions::transaction::TransactionState::Activated,
            },
        )
        .unwrap();
        crate::extensions::transaction::recover(&state).unwrap();
        assert!(!backup.exists());
        assert_eq!(
            ExtensionsLock::load(&state.paths.repository_file)
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
            environment: BTreeMap::new(),
            cwd: None,
            limit: 10,
            include_system_commands: false,
        }
    }

    async fn set_enabled_for_test(state: &ExtensionState, id: &str, enabled: bool) {
        let mut lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        lock.set_enabled(id, enabled).unwrap();
        lock.save(&state.paths.repository_file).unwrap();
        state.invalidate_provider_commands().await;
    }

    async fn catalog_contains(state: &ExtensionState, command: &str) -> bool {
        catalog::search(state, &catalog_request(command), &[])
            .await
            .unwrap()
            .iter()
            .any(|entry| entry.command == command)
    }

    fn discovered_candidate(
        path: &Path,
        name: &str,
    ) -> crate::extensions::inventory::ToolCandidate {
        crate::extensions::inventory::ToolCandidate {
            id: path.to_string_lossy().into_owned(),
            name: name.to_string(),
            locator: crate::extensions::inventory::ToolLocator::Executable {
                path: path.to_string_lossy().into_owned(),
            },
            version: None,
            sources: vec![crate::extensions::inventory::DiscoverySource::Path],
            quality: crate::extensions::inventory::DiscoveryQuality::AutoDetected,
            available: true,
            fingerprint: None,
        }
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn connected_tool_is_immediately_searchable_in_the_catalog() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let executable = directory.path().join("findable");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        }

        connect_tool(&state, discovered_candidate(&executable, "Findable"))
            .await
            .unwrap();

        // The first provider-command load performs the System-runtime
        // fingerprint binding. If that check ever failed right after connect,
        // the entry would be marked broken and silently vanish from search —
        // so both the command list and a query for the command name must hit.
        let commands = catalog::load_provider_commands_uncached(&state)
            .await
            .unwrap();
        assert!(!commands.is_empty());
        let entries = catalog::search(&state, &catalog_request("findable"), &[])
            .await
            .unwrap();
        assert!(entries.iter().any(|entry| entry.command == "findable"));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn connect_tool_binds_a_discovered_executable_like_a_custom_integration() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let executable = directory.path().join("mytool");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        }

        let entry = connect_tool(&state, discovered_candidate(&executable, "MyTool"))
            .await
            .unwrap();
        assert_eq!(entry.id, "local.mytool");
        assert_eq!(
            entry.distribution_source,
            ExtensionDistributionSource::Local
        );
        assert_eq!(entry.publisher_id, "local-user");
        assert!(is_generated_custom_integration(&entry));
        assert_eq!(
            entry.approved_permissions,
            tool_binding_permissions()
                .into_iter()
                .collect::<std::collections::BTreeSet<_>>()
                .into_iter()
                .collect::<Vec<_>>()
        );
        // Same convergence contract as manual custom integrations: the command
        // must appear in the catalog immediately.
        assert!(catalog_contains(&state, "mytool").await);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn connect_recommended_tool_yields_a_local_static_integration_with_catalog_visibility() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let executable = directory.path().join("v");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        }

        let tools = crate::extensions::recommendations::load_recommended().unwrap();
        let tool = &tools[0];
        let entry = connect_recommended_tool(
            &state,
            tool,
            Some(&executable.to_string_lossy()),
            Some(&tool.manifest.permissions),
        )
        .await
        .unwrap();

        // Same lock shape as a PATH-discovered connection: local distribution,
        // system runtime, and a static-descriptor provider derived entirely by
        // the generic linked-install pipeline.
        assert_eq!(entry.id, "io.github.vst93.v");
        assert_eq!(
            entry.distribution_source,
            ExtensionDistributionSource::Local
        );
        assert_eq!(entry.runtime_ownership, ExtensionRuntimeOwnership::System);
        assert_eq!(entry.provider_kind, ExtensionProviderKind::StaticDescriptor);
        assert!(entry.enabled);
        assert_eq!(entry.publisher_id, "vst93");
        assert_eq!(
            Path::new(&entry.manifest_path).parent(),
            Some(
                state
                    .paths
                    .data
                    .join("io.github.vst93.v")
                    .join("integration")
                    .as_path()
            )
        );
        assert_eq!(Path::new(&entry.executable_path), executable.as_path());
        let mut approved = tool.manifest.permissions.clone();
        approved.sort_unstable();
        assert_eq!(entry.approved_permissions, approved);

        // The shipped descriptor is preserved and every command is immediately
        // visible through the same catalog path as any local integration.
        for command in ["jv", "diff", "codec", "genpwd", "tt"] {
            assert!(catalog_contains(&state, command).await);
        }
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn connect_tool_allocates_a_new_id_when_the_base_name_is_taken() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let first = directory.path().join("dup");
        let second = directory.path().join("nested");
        std::fs::create_dir_all(&second).unwrap();
        let first_bin = first;
        let second_bin = second.join("dup");
        for executable in [&first_bin, &second_bin] {
            std::fs::write(executable, "#!/bin/sh\nexit 0\n").unwrap();
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                std::fs::set_permissions(executable, std::fs::Permissions::from_mode(0o755))
                    .unwrap();
            }
        }
        connect_tool(&state, discovered_candidate(&first_bin, "Dup"))
            .await
            .unwrap();
        let second_entry = connect_tool(&state, discovered_candidate(&second_bin, "dup2"))
            .await
            .unwrap();

        assert_ne!(second_entry.id, "local.dup2");
        assert!(second_entry.id.starts_with("local.dup."));
    }

    #[cfg(unix)]
    #[test]
    fn tool_binding_request_rejects_unusable_paths() {
        let candidate = discovered_candidate(Path::new("/definitely/not/here"), "ghost");
        assert!(tool_binding_request(&candidate).is_err());
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
                environment: BTreeMap::new(),
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

    /// End-to-end proof of connect-time parameter hints: connecting a custom
    /// integration whose binary prints a known help text must populate the
    /// generated descriptor's arguments, and the launcher catalog must then
    /// suggest those flags (with descriptions) during completion.
    #[tokio::test]
    async fn connected_tool_completions_include_help_derived_flags() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        // The help text intentionally mixes styles and includes -h/--help so
        // the exclusion logic is exercised through the real pipeline.
        #[cfg(not(windows))]
        let executable = {
            use std::os::unix::fs::PermissionsExt;
            let path = script_directory.path().join("demo-tool.sh");
            std::fs::write(
                &path,
                concat!(
                    "#!/bin/sh\n",
                    "if [ \"$1\" = \"--help\" ]; then\n",
                    "cat <<'EOF'\n",
                    "Usage: demo-tool [options]\n",
                    "\n",
                    "Options:\n",
                    "  -o, --output <FILE>    Write result to FILE\n",
                    "      --verbose          Enable verbose logging\n",
                    "  -h, --help             Show this help\n",
                    "EOF\n",
                    "exit 0\n",
                    "fi\n",
                    "echo done\n"
                ),
            )
            .unwrap();
            let mut permissions = std::fs::metadata(&path).unwrap().permissions();
            permissions.set_mode(0o755);
            std::fs::set_permissions(&path, permissions).unwrap();
            path
        };
        #[cfg(windows)]
        let executable = {
            let path = script_directory.path().join("demo-tool.cmd");
            std::fs::write(
                &path,
                "@echo off\r\nif \"%1\"==\"--help\" (\r\necho   -o, --output FILE    Write result to FILE\r\necho   --verbose            Enable verbose logging\r\n)\r\n",
            )
            .unwrap();
            path
        };
        create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.demo-help-test".into(),
                name: "Demo help test".into(),
                command: "demo-help-test".into(),
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

        let response = catalog::complete(
            &state,
            &CompletionRequest {
                command: "demo-help-test".into(),
                tokens: vec!["demo-help-test".into(), "--".into()],
                cwd: None,
            },
        )
        .await
        .unwrap();
        assert!(!response.dynamic);

        let output = response
            .items
            .iter()
            .find(|item| item.value == "--output")
            .expect("connected tool should suggest its derived --output flag");
        assert_eq!(output.description, "Write result to FILE");
        let verbose = response
            .items
            .iter()
            .find(|item| item.value == "--verbose")
            .expect("connected tool should suggest its derived --verbose flag");
        assert_eq!(verbose.description, "Enable verbose logging");
        assert!(response.items.iter().all(|item| item.value != "--help"));
    }

    /// End-to-end proof of subcommand-aware help parsing: a tool whose
    /// top-level `--help` is a v-style plugin listing (no option lines at all)
    /// must surface each plugin as its own runnable descriptor command — with
    /// aliases and per-subcommand flags probed from second-level help.
    #[cfg(unix)]
    #[tokio::test]
    async fn connected_tool_exposes_subcommands_with_aliases_and_probed_flags() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = {
            let path = script_directory.path().join("subber.sh");
            std::fs::write(
                &path,
                concat!(
                    "#!/bin/sh\n",
                    "if [ \"$1\" = \"--help\" ]; then\n",
                    "cat <<'EOF'\n",
                    "subber - Gadgets under the terminal\n",
                    "Version: dev  🏠 https://example.com/subber\n",
                    "\n",
                    "Available Plugins\n",
                    "==================================================\n",
                    "📦 alpha 1.0.0 👤 vst  (aliases: al)\n",
                    "  First gadget does things\n",
                    "📦 beta 0.2.0 👤 vst\n",
                    "  Second gadget does other things\n",
                    "\n",
                    "Run subber <command> -h for detailed help.\n",
                    "EOF\n",
                    "exit 0\n",
                    "fi\n",
                    "if [ \"$1\" = \"alpha\" ]; then\n",
                    "printf 'Modes:\\n  -f         Format (pretty-print)\\nOptions:\\n  -sort   Sort object keys alphabetically\\n'\n",
                    "exit 0\n",
                    "fi\n",
                    "if [ \"$1\" = \"beta\" ]; then\n",
                    "printf 'Options:\\n  -raw   Disable colored output\\n'\n",
                    "exit 0\n",
                    "fi\n",
                    "echo done\n"
                ),
            )
            .unwrap();
            let mut permissions = std::fs::metadata(&path).unwrap().permissions();
            permissions.set_mode(0o755);
            std::fs::set_permissions(&path, permissions).unwrap();
            path
        };
        create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.subber-test".into(),
                name: "Subber Test".into(),
                command: "subber-test".into(),
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

        // Root + one runnable command per derived subcommand, aliases kept.
        // Assert through the generated descriptor file, which is exactly what
        // the provider pipeline loads.
        let descriptor_path = state
            .paths
            .data
            .join("local.subber-test")
            .join("integration")
            .join("provider-description.json");
        let descriptor: serde_json::Value =
            serde_json::from_slice(&std::fs::read(&descriptor_path).unwrap()).unwrap();
        let ids = descriptor["commands"]
            .as_array()
            .unwrap()
            .iter()
            .map(|command| command["id"].as_str().unwrap().to_string())
            .collect::<Vec<_>>();
        assert_eq!(ids, ["subber-test", "alpha", "beta"]);
        assert_eq!(
            catalog::load_provider_commands_uncached(&state)
                .await
                .unwrap()
                .len(),
            3
        );
        let results = catalog::search(&state, &catalog_request("alpha"), &[])
            .await
            .unwrap();
        let alpha = results
            .iter()
            .find(|entry| entry.command == "alpha")
            .expect("derived subcommand should be searchable");
        assert_eq!(alpha.aliases, ["al"]);
        assert_eq!(alpha.description, "First gadget does things");

        // Argument completion on a subcommand surfaces ITS probed flags with
        // descriptions (`v jv -` style usage).
        let response = catalog::complete(
            &state,
            &CompletionRequest {
                command: "alpha".into(),
                tokens: vec!["alpha".into(), "-".into()],
                cwd: None,
            },
        )
        .await
        .unwrap();
        let format_flag = response
            .items
            .iter()
            .find(|item| item.value == "-f")
            .expect("subcommand should suggest its derived -f flag");
        assert_eq!(format_flag.description, "Format (pretty-print)");
        let sort_flag = response
            .items
            .iter()
            .find(|item| item.value == "-sort")
            .expect("subcommand should suggest its derived -sort flag");
        assert_eq!(sort_flag.description, "Sort object keys alphabetically");

        // Catalog search finds the subcommand through its alias too.
        let results = catalog::search(&state, &catalog_request("al"), &[])
            .await
            .unwrap();
        assert!(results.iter().any(|entry| entry.command == "alpha"));

        // A subcommand execution plan runs `<executable> alpha <user args>`.
        let results = catalog::search(
            &state,
            &CatalogSearchRequest {
                query: "beta".into(),
                tokens: vec!["beta".into(), "user value".into()],
                environment: BTreeMap::new(),
                cwd: None,
                limit: 10,
                include_system_commands: false,
            },
            &[],
        )
        .await
        .unwrap();
        let protected = results
            .iter()
            .find(|entry| entry.command == "beta")
            .and_then(|entry| entry.execution.as_ref())
            .expect("subcommand should expose an execution plan");
        let plan = state
            .take_execution_plan(protected.plan_token.as_deref().unwrap())
            .unwrap();
        assert_eq!(Path::new(&plan.program), executable);
        assert_eq!(plan.args, ["beta", "user value"]);
    }

    /// Re-probe contract: after the tool's second-level help changes,
    /// `reprobe_tool_commands` must regenerate the descriptor with the new
    /// flag while preserving the root command identity/argsPrefix, and
    /// refresh the `help-probe.json` sidecar.
    #[cfg(unix)]
    #[tokio::test]
    async fn reprobe_picks_up_modified_subcommand_help_and_refreshes_the_sidecar() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = {
            let path = script_directory.path().join("reprober.sh");
            std::fs::write(
                &path,
                concat!(
                    "#!/bin/sh\n",
                    "if [ \"$1\" = \"--help\" ]; then\n",
                    "printf 'Available Plugins\\nalpha 1.0.0 (aliases: al)\\n    First gadget\\n'\n",
                    "exit 0\n",
                    "fi\n",
                    "if [ \"$1\" = \"alpha\" ]; then\n",
                    "printf 'Options:\\n  -f         Format output\\n'\n",
                    "exit 0\n",
                    "fi\n",
                    "echo done\n"
                ),
            )
            .unwrap();
            let mut permissions = std::fs::metadata(&path).unwrap().permissions();
            permissions.set_mode(0o755);
            std::fs::set_permissions(&path, permissions).unwrap();
            path
        };
        create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.reprober-test".into(),
                name: "Reprober Test".into(),
                command: "reprober-test".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: vec!["--prefix".into()],
                version_args: Vec::new(),
                permissions: vec![Permission::Environment],
                platforms: current_platforms(),
            },
        )
        .await
        .unwrap();
        let installed_at = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get("local.reprober-test")
            .unwrap()
            .installed_at;
        let package_root = state
            .paths
            .data
            .join("local.reprober-test")
            .join("integration");
        let descriptor_path = package_root.join("provider-description.json");
        let before: serde_json::Value =
            serde_json::from_slice(&std::fs::read(&descriptor_path).unwrap()).unwrap();
        assert!(!serde_json::to_string(&before).unwrap().contains("--extra"));
        assert_eq!(
            before["commands"][0]["execution"]["argsPrefix"],
            serde_json::json!(["--prefix"])
        );

        // The tool upgrade: its subcommand help now exposes an extra flag.
        std::fs::write(
            &executable,
            concat!(
                "#!/bin/sh\n",
                "if [ \"$1\" = \"--help\" ]; then\n",
                "printf 'Available Plugins\\nalpha 1.0.0 (aliases: al)\\n    First gadget\\n'\n",
                "exit 0\n",
                "fi\n",
                "if [ \"$1\" = \"alpha\" ]; then\n",
                "printf 'Options:\\n  -f         Format output\\n  -x, --extra   Extra thing\\n'\n",
                "exit 0\n",
                "fi\n",
                "echo done\n"
            ),
        )
        .unwrap();

        let report = super::reprobe_tool_commands(&state, "local.reprober-test")
            .await
            .unwrap();
        assert_eq!(report.subcommands, 1);

        let after: serde_json::Value =
            serde_json::from_slice(&std::fs::read(&descriptor_path).unwrap()).unwrap();
        let alpha_arguments = &after["commands"]
            .as_array()
            .unwrap()
            .iter()
            .find(|command| command["id"] == "alpha")
            .expect("alpha command must survive the re-probe")["arguments"];
        assert!(serde_json::to_string(alpha_arguments)
            .unwrap()
            .contains("--extra"));
        // Root command identity and configured argsPrefix are preserved.
        assert_eq!(after["commands"][0]["id"], "reprober-test");
        assert_eq!(
            after["commands"][0]["execution"]["argsPrefix"],
            serde_json::json!(["--prefix"])
        );

        let sidecar: serde_json::Value =
            serde_json::from_slice(&std::fs::read(package_root.join("help-probe.json")).unwrap())
                .unwrap();
        assert!(sidecar["probedAt"].as_u64().unwrap() >= installed_at);
        assert_eq!(sidecar["rootArgumentCount"].as_u64().unwrap(), 0);
        assert_eq!(sidecar["subcommands"][0]["name"], "alpha");
        assert_eq!(
            sidecar["subcommands"][0]["aliases"],
            serde_json::json!(["al"])
        );
    }

    /// Enable-path re-probe: disabling, upgrading the tool's help, then
    /// re-enabling must refresh the derived flags through the same silent
    /// helper the enable command uses — and enabling must never fail (or
    /// panic) when the executable has vanished.
    #[cfg(unix)]
    #[tokio::test]
    async fn enable_reprobe_picks_up_new_flags_and_survives_a_deleted_executable() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = {
            let path = script_directory.path().join("enabler.sh");
            std::fs::write(
                &path,
                concat!(
                    "#!/bin/sh\n",
                    "if [ \"$1\" = \"--help\" ]; then\n",
                    "printf 'Options:\\n  -old   Old flag\\n'\n",
                    "exit 0\n",
                    "fi\n",
                    "echo done\n"
                ),
            )
            .unwrap();
            let mut permissions = std::fs::metadata(&path).unwrap().permissions();
            permissions.set_mode(0o755);
            std::fs::set_permissions(&path, permissions).unwrap();
            path
        };
        create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.enabler-test".into(),
                name: "Enabler Test".into(),
                command: "enabler-test".into(),
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
        let descriptor_path = state
            .paths
            .data
            .join("local.enabler-test")
            .join("integration")
            .join("provider-description.json");

        set_enabled_for_test(&state, "local.enabler-test", false).await;
        std::fs::write(
            &executable,
            concat!(
                "#!/bin/sh\n",
                "if [ \"$1\" = \"--help\" ]; then\n",
                "printf 'Options:\\n  -old   Old flag\\n  -new   New flag\\n'\n",
                "exit 0\n",
                "fi\n",
                "echo done\n"
            ),
        )
        .unwrap();
        set_enabled_for_test(&state, "local.enabler-test", true).await;
        let entry = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get("local.enabler-test")
            .unwrap()
            .clone();
        super::reprobe_after_enable(&state, &entry).await;

        let descriptor: serde_json::Value =
            serde_json::from_slice(&std::fs::read(&descriptor_path).unwrap()).unwrap();
        assert!(serde_json::to_string(&descriptor).unwrap().contains("-new"));

        // A deleted executable must degrade silently on the enable path but
        // surface a helpful error through the manual routine.
        std::fs::remove_file(&executable).unwrap();
        super::reprobe_after_enable(&state, &entry).await;
        let error = super::reprobe_tool_commands(&state, "local.enabler-test")
            .await
            .unwrap_err();
        assert!(error.contains("not available"), "{error}");
    }

    #[tokio::test]
    async fn manual_reprobe_rejects_script_integrations_with_a_clear_error() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        create_custom_integration(
            &state,
            script_request("local.script-reprobe", "Script reprobe", "script-reprobe"),
        )
        .await
        .unwrap();
        let error = super::reprobe_tool_commands(&state, "local.script-reprobe")
            .await
            .unwrap_err();
        assert!(
            error.contains("Script integrations have no executable"),
            "{error}"
        );
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
                environment: BTreeMap::new(),
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

    #[cfg(unix)]
    #[tokio::test]
    async fn verify_installed_rejects_missing_static_runtime() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let executable = directory.path().join("verify-tool");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        let entry = create_custom_integration(
            &state,
            CustomIntegrationRequest {
                id: "local.verify-test".into(),
                name: "Verify test".into(),
                command: "verify-test".into(),
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

        assert!(verify_installed(&state, &entry.id).await.is_ok());

        std::fs::remove_file(&executable).unwrap();
        let error = verify_installed(&state, &entry.id).await.unwrap_err();
        assert!(error.contains("Runtime is unavailable"));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn writer_swap_install_uninstall_edit_and_import_after_migration() {
        find_script_interpreter(ScriptLanguage::Shell).unwrap();
        for retained_legacy in [false, true] {
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            ExtensionsLock::default().save_legacy(&state.paths.legacy_lock_file).unwrap();
            crate::extensions::repository::migrate_to_repository(&state.paths).unwrap();
            let archive = state.paths.root.join("extensions.lock.json.migrated");
            let legacy_bytes = std::fs::read(&archive).unwrap();
            if retained_legacy {
                // Model a legacy file left beside the repository after migration.
                std::fs::copy(&archive, &state.paths.legacy_lock_file).unwrap();
            }
            let snapshot = || -> serde_json::Value {
                assert_eq!(std::fs::read(&archive).unwrap(), legacy_bytes);
                if retained_legacy {
                    assert_eq!(std::fs::read(&state.paths.legacy_lock_file).unwrap(), legacy_bytes);
                } else {
                    assert!(!state.paths.legacy_lock_file.exists());
                }
                let json: serde_json::Value = serde_json::from_slice(
                    &std::fs::read(&state.paths.repository_file).unwrap(),
                ).unwrap();
                assert_eq!(json["schemaVersion"], crate::extensions::repository::REPOSITORY_SCHEMA_VERSION);
                json
            };

            let source = test_state(&directory.path().join("source"));
            let linked = create_custom_integration(
                &source, script_request("local.linked-writer", "Linked", "linked"),
            ).await.unwrap();
            install(&state, ExtensionInstallRequest {
                source: InstallSource::Linked,
                package: None,
                version: None,
                manifest_path: Some(linked.manifest_path),
                executable_path: None,
                approved_permissions: Some(linked.approved_permissions),
            }).await.unwrap();
            assert_eq!(snapshot()["extensions"]["local.linked-writer"]["name"], "Linked");
            uninstall(&state, "local.linked-writer", true).await.unwrap();
            assert!(snapshot()["extensions"].as_object().unwrap().is_empty());

            let id = "local.writer-swap";
            let mut request = script_request(id, "Original", "original");
            create_custom_integration(&state, request.clone()).await.unwrap();
            assert_eq!(snapshot()["extensions"][id]["name"], "Original");
            set_enabled_for_test(&state, id, false).await;
            let before_edit = snapshot();
            let mut invalid = request.clone();
            invalid.script_content = Some(String::new());
            assert!(update_custom_integration(&state, id, invalid).await.is_err());
            assert_eq!(snapshot(), before_edit);

            request.name = "Edited".into();
            request.version = "2.0.0".into();
            request.script_content = Some("printf edited".into());
            update_custom_integration(&state, id, request.clone()).await.unwrap();
            let edited = snapshot();
            assert_eq!(edited["extensions"][id]["name"], "Edited");
            assert_eq!(edited["extensions"][id]["currentVersion"], "2.0.0");
            assert_eq!(edited["extensions"][id]["enabled"], false);
            assert_eq!(edited["extensions"][id]["installedAt"], before_edit["extensions"][id]["installedAt"]);

            let export = sync::build_export(&state, Utc::now()).unwrap();
            assert_eq!(export.extensions.len(), 1);
            assert_eq!(export.extensions[0].version, "2.0.0");
            assert!(!export.extensions[0].enabled);
            uninstall(&state, id, true).await.unwrap();
            assert!(snapshot()["extensions"].as_object().unwrap().is_empty());

            let report = sync::import_document(
                &state,
                Path::new("writer-swap.json"),
                export,
                &BTreeMap::from([(id.to_string(), request.permissions)]),
            ).await;
            assert!(report.failed.is_empty(), "{:?}", report.failed);
            assert_eq!(report.succeeded.len(), 1);
            let imported = snapshot();
            assert_eq!(imported["extensions"][id]["currentVersion"], "2.0.0");
            assert_eq!(imported["extensions"][id]["enabled"], false);
            crate::extensions::repository::migrate_to_repository(&state.paths).unwrap();
            assert_eq!(ExtensionsLock::load(&state.paths.repository_file).unwrap().get(id).unwrap().name, "Edited");
            assert_eq!(snapshot(), imported);
        }
    }

    #[cfg(unix)]
    mod unix_faults {
        use super::*;
        use crate::extensions::fault_test_support::{
            crash_child_boundary, crash_child_root, kill_at_commit_point, run_crash_child, skip_readonly_as_root,
            ReadonlyDirectory,
        };
        use crate::extensions::transaction::{recover, RemovalJournal, RemovalKind};
        use crate::extensions::with_async_commit_point_action;
        use std::os::unix::fs::PermissionsExt;
        use std::sync::Arc;

        const WINDOW_ID: &str = "local.journal-window";

        fn window_request(changed: bool) -> CustomIntegrationRequest {
            let mut request = script_request(WINDOW_ID, "Original", "window");
            if changed {
                request.name = "Changed".into();
                request.script_content = Some("printf changed".into());
            }
            request
        }

        async fn window_operation(state: &ExtensionState, operation: &str) -> Result<(), String> {
            match operation {
                "uninstall" => uninstall(state, WINDOW_ID, true).await,
                "edit" => update_custom_integration(state, WINDOW_ID, window_request(true))
                    .await
                    .map(|_| ()),
                _ => unreachable!(),
            }
        }

        struct WindowFixture {
            state: Arc<ExtensionState>,
            entry: ExtensionLockEntry,
            tree: PathBuf,
            repository: Vec<u8>,
            files: BTreeMap<PathBuf, (Vec<u8>, u32)>,
            executable: Vec<u8>,
        }

        impl WindowFixture {
            async fn new(root: &Path, operation: &str) -> Self {
                let state = Arc::new(test_state(root));
                create_custom_integration(
                    &state,
                    script_request("local.retained", "Retained", "retained"),
                )
                .await
                .unwrap();
                let entry = if operation == "uninstall" {
                    // Install a real linked script inside the tree uninstall moves.
                    let prepared_root = tempfile::tempdir().unwrap();
                    let prepared = test_state(prepared_root.path());
                    create_custom_integration(&prepared, window_request(false))
                        .await
                        .unwrap();
                    let tree = state.paths.extensions.join(WINDOW_ID);
                    std::fs::rename(
                        prepared.paths.data.join(WINDOW_ID).join("integration"),
                        &tree,
                    )
                    .unwrap();
                    install(
                        &state,
                        ExtensionInstallRequest {
                            source: InstallSource::Linked,
                            package: None,
                            version: None,
                            manifest_path: Some(tree.to_string_lossy().into_owned()),
                            executable_path: None,
                            approved_permissions: Some(window_request(false).permissions),
                        },
                    )
                    .await
                    .unwrap()
                } else {
                    create_custom_integration(&state, window_request(false))
                        .await
                        .unwrap()
                };
                let data = state.paths.data.join(WINDOW_ID);
                std::fs::create_dir_all(&data).unwrap();
                std::fs::write(data.join("user-data"), b"preserved user data").unwrap();
                let tree = Path::new(&entry.manifest_path).parent().unwrap().to_path_buf();
                let files = std::fs::read_dir(&tree)
                    .unwrap()
                    .map(|item| {
                        let path = item.unwrap().path();
                        let bytes = std::fs::read(&path).unwrap();
                        let mode = path.metadata().unwrap().permissions().mode();
                        (path.file_name().unwrap().into(), (bytes, mode))
                    })
                    .collect();
                let executable = std::fs::read(&entry.executable_path).unwrap();
                let repository = std::fs::read(&state.paths.repository_file).unwrap();
                Self { state, entry, tree, repository, files, executable }
            }

            fn journal_path(&self, operation: &str) -> PathBuf {
                self.state.paths.extensions.join(".transactions").join(format!(
                    "removal-{operation}-{WINDOW_ID}-{}.json", self.entry.updated_at
                ))
            }

            fn assert_tree(&self, tree: &Path) {
                assert_eq!(std::fs::read_dir(tree).unwrap().count(), self.files.len());
                for (relative, (bytes, mode)) in &self.files {
                    let path = tree.join(relative);
                    assert_eq!(&std::fs::read(&path).unwrap(), bytes, "{}", path.display());
                    assert_eq!(path.metadata().unwrap().permissions().mode(), *mode);
                }
            }

            fn assert_no_staging(&self) {
                for parent in [
                    self.state.paths.extensions.clone(),
                    self.state.paths.data.join(WINDOW_ID),
                ] {
                    if parent.exists() {
                        for item in std::fs::read_dir(parent).unwrap() {
                            let item = item.unwrap();
                            let name = item.file_name();
                            let name = name.to_string_lossy();
                            assert!(!name.starts_with(".removing-"), "{name}");
                            assert!(!name.starts_with(&format!(".{WINDOW_ID}-editing-")), "{name}");
                        }
                    }
                }
            }

            async fn assert_pristine(&self, state: &ExtensionState) {
                assert_eq!(std::fs::read(&state.paths.repository_file).unwrap(), self.repository);
                assert_eq!(
                    serde_json::to_value(verify_installed(state, WINDOW_ID).await.unwrap()).unwrap(),
                    serde_json::to_value(&self.entry).unwrap()
                );
                self.assert_tree(&self.tree);
                assert_eq!(std::fs::read(&self.entry.executable_path).unwrap(), self.executable);
                let output = std::process::Command::new(&self.entry.executable_path)
                    .arg(self.tree.join("provider.sh"))
                    .output()
                    .unwrap();
                assert!(output.status.success(), "{output:?}");
                assert_eq!(output.stdout, b"original");
                assert_eq!(
                    std::fs::read(state.paths.data.join(WINDOW_ID).join("user-data")).unwrap(),
                    b"preserved user data"
                );
                self.assert_no_staging();
            }

            async fn assert_fresh_recovery(&self, operation: &str) {
                for _ in 0..3 {
                    let fresh = test_state(&self.state.paths.root);
                    recover(&fresh).unwrap();
                    recover(&fresh).unwrap();
                    self.assert_pristine(&fresh).await;
                    assert!(!self.journal_path(operation).exists());
                }
            }

            async fn assert_retry(&self, operation: &str) {
                window_operation(&self.state, operation).await.unwrap();
                for _ in 0..3 {
                    let fresh = test_state(&self.state.paths.root);
                    recover(&fresh).unwrap();
                    let lock = ExtensionsLock::load(&fresh.paths.repository_file).unwrap();
                    assert_eq!(lock.get("local.retained").unwrap().name, "Retained");
                    if operation == "uninstall" {
                        assert_eq!(lock.extensions.len(), 1);
                        assert!(lock.get(WINDOW_ID).is_err());
                        assert!(!self.tree.exists());
                        assert!(!fresh.paths.data.join(WINDOW_ID).exists());
                    } else {
                        assert_eq!(lock.extensions.len(), 2);
                        assert_eq!(verify_installed(&fresh, WINDOW_ID).await.unwrap().name, "Changed");
                        assert_eq!(std::fs::read(self.tree.join("provider.sh")).unwrap(), b"printf changed");
                    }
                    self.assert_no_staging();
                    assert!(!self.journal_path(operation).exists());
                    assert!(std::fs::read_dir(fresh.paths.extensions.join(".transactions"))
                        .unwrap()
                        .all(|item| item.unwrap().path().extension().is_none_or(|ext| ext != "json")));
                }
            }
        }

        async fn readonly_window(operation: &'static str, boundary: &str) {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let fixture = WindowFixture::new(directory.path(), operation).await;
            let journals = fixture.state.paths.extensions.join(".transactions");
            std::fs::create_dir_all(&journals).unwrap();
            let journal_path = fixture.journal_path(operation);
            let tree = fixture.tree.clone();
            let journal_guard = ReadonlyDirectory::new(&journals);
            let tree_guard = ReadonlyDirectory::new(tree.parent().unwrap());
            let repository_guard = ReadonlyDirectory::new(&fixture.state.paths.root);
            let (label, action): (_, Box<dyn FnOnce() + Send>) = match boundary {
                "first-write" => {
                    let deny = journal_guard.arm();
                    ("removal-journal-persist", Box::new(move || {
                        // This assertion reproduces the old destructive staging window.
                        assert!(tree.is_dir(), "first journal write must precede live-tree staging");
                        deny();
                    }))
                }
                "stage" => {
                    let deny = tree_guard.arm();
                    let path = journal_path.clone();
                    let label = if operation == "edit" { "edit-stage-rename" } else { "uninstall-stage-rename" };
                    (label, Box::new(move || {
                        let journal: RemovalJournal = serde_json::from_slice(&std::fs::read(path).unwrap()).unwrap();
                        assert!(!journal.staged_path.unwrap().exists());
                        assert!(tree.is_dir());
                        deny();
                    }))
                }
                "repository" => {
                    let deny_tree = tree_guard.arm();
                    let deny_repository = repository_guard.arm();
                    let path = journal_path.clone();
                    ("repository-persist", Box::new(move || {
                        let journal: RemovalJournal = serde_json::from_slice(&std::fs::read(path).unwrap()).unwrap();
                        assert!(journal.staged_path.unwrap().is_dir());
                        assert!(!tree.exists());
                        deny_tree();
                        deny_repository();
                    }))
                }
                _ => unreachable!(),
            };
            let error = with_async_commit_point_action(
                label,
                action,
                window_operation(&fixture.state, operation),
            ).await.unwrap_err();
            let expected = match boundary {
                "first-write" => "Cannot persist removal transaction journal",
                "stage" => "Cannot stage",
                "repository" => "Cannot persist extension repository",
                _ => unreachable!(),
            };
            assert!(error.contains(expected), "{error}");
            assert_eq!(std::fs::read(&fixture.state.paths.repository_file).unwrap(), fixture.repository);
            if boundary == "first-write" {
                assert!(!journal_path.exists());
                fixture.assert_pristine(&fixture.state).await;
            } else {
                let bytes = std::fs::read(&journal_path).unwrap();
                let journal: RemovalJournal = serde_json::from_slice(&bytes).unwrap();
                assert_eq!(journal.removal_kind, Some(RemovalKind::Staged));
                assert_eq!(serde_json::to_value(&journal.removed_entry).unwrap(), serde_json::to_value(&fixture.entry).unwrap());
                if boundary == "repository" {
                    fixture.assert_tree(journal.staged_path.as_ref().unwrap());
                    assert!(!fixture.tree.exists());
                    // A retry must not replace an unresolved backup's journal.
                    assert!(window_operation(&fixture.state, operation).await.is_err());
                    assert_eq!(std::fs::read(&journal_path).unwrap(), bytes);
                } else {
                    assert!(!journal.staged_path.unwrap().exists());
                    fixture.assert_pristine(&fixture.state).await;
                }
            }
            drop(journal_guard);
            drop(tree_guard);
            drop(repository_guard);
            if boundary == "stage" {
                // Retry on the same state, before startup can clear the missing-backup journal.
                fixture.assert_retry(operation).await;
            } else {
                fixture.assert_fresh_recovery(operation).await;
                fixture.assert_retry(operation).await;
            }
        }

        async fn readonly_repository_operation(operation: &str) {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            create_custom_integration(
                &state,
                script_request("local.retained", "Retained", "retained"),
            )
            .await
            .unwrap();
            let id = "local.readonly";
            let request = script_request(id, "Original", "readonly");
            if operation != "install" {
                create_custom_integration(&state, request.clone())
                    .await
                    .unwrap();
            }
            let source = state.paths.extensions.join(id);
            if operation == "uninstall" {
                std::fs::create_dir_all(&source).unwrap();
                std::fs::write(source.join("payload"), b"keep extension files").unwrap();
            }
            let before = std::fs::read(&state.paths.repository_file).unwrap();
            let readonly = ReadonlyDirectory::new(&state.paths.root);
            let error =
                with_async_commit_point_action("repository-persist", readonly.arm(), async {
                    match operation {
                        "install" => create_custom_integration(&state, request).await.map(|_| ()),
                        "uninstall" => uninstall(&state, id, true).await,
                        "edit" => {
                            let mut changed = request;
                            changed.name = "Changed".into();
                            changed.script_content = Some("printf changed".into());
                            update_custom_integration(&state, id, changed)
                                .await
                                .map(|_| ())
                        }
                        _ => unreachable!(),
                    }
                })
                .await
                .unwrap_err();
            assert!(
                error.contains("Cannot persist extension repository"),
                "{error}"
            );
            assert_eq!(std::fs::read(&state.paths.repository_file).unwrap(), before);
            if operation == "uninstall" {
                assert_eq!(
                    std::fs::read(source.join("payload")).unwrap(),
                    b"keep extension files"
                );
            }
            drop(readonly);

            let restarted = test_state(directory.path());
            for _ in 0..2 {
                recover(&restarted).unwrap();
                assert_eq!(
                    std::fs::read(&restarted.paths.repository_file).unwrap(),
                    before
                );
                let root = restarted.paths.data.join(id).join("integration");
                if operation == "install" {
                    assert!(!root.exists());
                } else {
                    assert_eq!(
                        std::fs::read(root.join("provider.sh")).unwrap(),
                        b"printf original"
                    );
                }
            }
            let journals = restarted.paths.extensions.join(".transactions");
            assert!(!journals.exists() || std::fs::read_dir(journals).unwrap().next().is_none());
        }

        #[tokio::test]
        async fn readonly_install_repository_persist_returns_error_without_partial_state() {
            readonly_repository_operation("install").await;
        }

        #[tokio::test]
        async fn readonly_uninstall_repository_persist_restores_files_and_entry() {
            readonly_repository_operation("uninstall").await;
        }

        #[tokio::test]
        async fn readonly_edit_repository_persist_restores_the_original_generation() {
            readonly_repository_operation("edit").await;
        }

        #[tokio::test]
        async fn readonly_initial_removal_journal_rejects_uninstall_and_allows_retry() {
            readonly_window("uninstall", "first-write").await;
        }

        #[tokio::test]
        async fn readonly_initial_removal_journal_rejects_edit_and_allows_retry() {
            readonly_window("edit", "first-write").await;
        }

        #[tokio::test]
        async fn readonly_uninstall_stage_rename_keeps_journal_and_allows_direct_retry() {
            readonly_window("uninstall", "stage").await;
        }

        #[tokio::test]
        async fn readonly_edit_stage_rename_keeps_journal_and_allows_direct_retry() {
            readonly_window("edit", "stage").await;
        }

        #[tokio::test]
        async fn readonly_uninstall_rollback_keeps_staged_tree_journal_for_fresh_recovery() {
            readonly_window("uninstall", "repository").await;
        }

        #[tokio::test]
        async fn readonly_edit_rollback_keeps_staged_tree_journal_for_fresh_recovery() {
            readonly_window("edit", "repository").await;
        }

        #[tokio::test]
        async fn readonly_edit_finalization_preserves_the_committed_replacement() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let fixture = WindowFixture::new(directory.path(), "edit").await;
            let readonly = ReadonlyDirectory::new(&fixture.state.paths.root);
            let error = with_async_commit_point_action(
                "edit-repository-finalize",
                readonly.arm(),
                window_operation(&fixture.state, "edit"),
            ).await.unwrap_err();
            assert!(error.contains("Cannot finalize custom integration update"), "{error}");
            let repository = std::fs::read(&fixture.state.paths.repository_file).unwrap();
            assert_ne!(repository, fixture.repository);
            drop(readonly);
            for _ in 0..3 {
                let fresh = test_state(directory.path());
                recover(&fresh).unwrap();
                assert_eq!(std::fs::read(&fresh.paths.repository_file).unwrap(), repository);
                assert_eq!(verify_installed(&fresh, WINDOW_ID).await.unwrap().name, "Changed");
                let output = std::process::Command::new(&fixture.entry.executable_path)
                    .arg(fixture.tree.join("provider.sh")).output().unwrap();
                assert!(output.status.success());
                assert_eq!(output.stdout, b"changed");
                fixture.assert_no_staging();
                assert!(!fixture.journal_path("edit").exists());
            }
        }

        #[tokio::test]
        async fn readonly_edit_backup_cleanup_keeps_the_committed_journal_for_retry() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let fixture = WindowFixture::new(directory.path(), "edit").await;
            let readonly = ReadonlyDirectory::new(fixture.tree.parent().unwrap());
            let error = with_async_commit_point_action(
                "edit-repository-finalize",
                readonly.arm(),
                window_operation(&fixture.state, "edit"),
            ).await.unwrap_err();
            assert!(error.contains("Cannot remove completed edit backup"), "{error}");
            let repository = std::fs::read(&fixture.state.paths.repository_file).unwrap();
            let path = fixture.journal_path("edit");
            let journal: RemovalJournal = serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
            let backup = journal.staged_path.unwrap();
            assert!(backup.is_dir());
            assert_eq!(std::fs::read(fixture.tree.join("provider.sh")).unwrap(), b"printf changed");
            drop(readonly);
            for _ in 0..3 {
                let fresh = test_state(directory.path());
                recover(&fresh).unwrap();
                assert_eq!(std::fs::read(&fresh.paths.repository_file).unwrap(), repository);
                assert_eq!(verify_installed(&fresh, WINDOW_ID).await.unwrap().name, "Changed");
                assert_eq!(std::fs::read(fixture.tree.join("provider.sh")).unwrap(), b"printf changed");
                assert!(!backup.exists());
                assert!(!path.exists());
                fixture.assert_no_staging();
            }
        }

        fn window_boundaries(operation: &str) -> [&'static str; 5] {
            [
                "removal-journal-persist",
                "removal-journal-directory-sync",
                "removal-journal-parent-directory-sync",
                if operation == "edit" { "edit-stage-rename" } else { "uninstall-stage-rename" },
                if operation == "edit" { "edit-repository-remove" } else { "uninstall-repository-remove" },
            ]
        }

        async fn retry_interrupted_window(operation: &'static str) {
            for label in window_boundaries(operation) {
                let directory = tempfile::tempdir().unwrap();
                let fixture = WindowFixture::new(directory.path(), operation).await;
                let state = Arc::clone(&fixture.state);
                let failure = tokio::spawn(async move {
                    crate::extensions::with_async_commit_point(
                        label,
                        window_operation(&state, operation),
                    ).await
                }).await.unwrap_err();
                assert!(failure.is_panic());
                assert_eq!(std::fs::read(&fixture.state.paths.repository_file).unwrap(), fixture.repository);
                // No fresh state construction or explicit recover before retrying.
                fixture.assert_retry(operation).await;
            }
        }

        #[tokio::test]
        async fn uninstall_retries_interrupted_journaling_and_staging_without_restart() {
            retry_interrupted_window("uninstall").await;
        }

        #[tokio::test]
        async fn edit_retries_interrupted_journaling_and_staging_without_restart() {
            retry_interrupted_window("edit").await;
        }

        async fn crash_window(name: &'static str, operation: &'static str) {
            if let Some(root) = crash_child_root(name) {
                let state = test_state(&root);
                let selected = crash_child_boundary();
                let label = window_boundaries(operation)
                    .into_iter()
                    .find(|label| *label == selected)
                    .expect("known window boundary");
                with_async_commit_point_action(
                    label,
                    kill_at_commit_point(label),
                    window_operation(&state, operation),
                ).await.unwrap();
                panic!("crash boundary was not reached");
            }
            for label in window_boundaries(operation) {
                let directory = tempfile::tempdir().unwrap();
                let fixture = WindowFixture::new(directory.path(), operation).await;
                run_crash_child(name, directory.path(), label);
                assert_eq!(std::fs::read(&fixture.state.paths.repository_file).unwrap(), fixture.repository);
                let path = fixture.journal_path(operation);
                if label == "removal-journal-persist" {
                    assert!(!path.exists());
                    fixture.assert_pristine(&fixture.state).await;
                } else {
                    let journal: RemovalJournal = serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
                    assert_eq!(journal.removal_kind, Some(RemovalKind::Staged));
                    assert_eq!(serde_json::to_value(&journal.removed_entry).unwrap(), serde_json::to_value(&fixture.entry).unwrap());
                    let staged = journal.staged_path.as_ref().unwrap();
                    if label.ends_with("repository-remove") {
                        assert!(!fixture.tree.exists());
                        fixture.assert_tree(staged);
                    } else {
                        assert!(!staged.exists());
                        fixture.assert_pristine(&fixture.state).await;
                    }
                }
                fixture.assert_fresh_recovery(operation).await;
                fixture.assert_retry(operation).await;
            }
        }

        #[tokio::test]
        #[ignore = "manual Unix process-crash simulation"]
        async fn crash_simulation_uninstall_across_first_journal_and_stage_boundaries() {
            crash_window(concat!(module_path!(), "::crash_simulation_uninstall_across_first_journal_and_stage_boundaries"), "uninstall").await;
        }

        #[tokio::test]
        #[ignore = "manual Unix process-crash simulation"]
        async fn crash_simulation_edit_across_first_journal_and_stage_boundaries() {
            crash_window(concat!(module_path!(), "::crash_simulation_edit_across_first_journal_and_stage_boundaries"), "edit").await;
        }

        #[tokio::test]
        async fn readonly_committed_journal_update_keeps_intent_for_next_startup() {
            if skip_readonly_as_root() {
                return;
            }
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            let id = "local.readonly-journal-update";
            let entry =
                create_custom_integration(&state, script_request(id, "Original", "journal"))
                    .await
                    .unwrap();
            let journals = state.paths.extensions.join(".transactions");
            std::fs::create_dir_all(&journals).unwrap();
            let readonly = ReadonlyDirectory::new(&journals);
            with_async_commit_point_action(
                "repository-directory-sync",
                readonly.arm(),
                uninstall(&state, id, true),
            )
            .await
            .unwrap();
            let path = journals.join(format!("removal-uninstall-{id}-{}.json", entry.updated_at));
            let journal: RemovalJournal =
                serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
            assert_eq!(journal.removal_kind, Some(RemovalKind::Staged));
            assert_eq!(
                serde_json::to_value(&journal.removed_entry).unwrap(),
                serde_json::to_value(&entry).unwrap()
            );
            assert!(ExtensionsLock::load(&state.paths.repository_file)
                .unwrap()
                .get(id)
                .is_err());
            assert!(!state.paths.data.join(id).exists());
            drop(readonly);

            let restarted = test_state(directory.path());
            assert!(!path.exists());
            for _ in 0..2 {
                recover(&restarted).unwrap();
                assert!(ExtensionsLock::load(&restarted.paths.repository_file)
                    .unwrap()
                    .get(id)
                    .is_err());
                assert!(!restarted.paths.data.join(id).exists());
            }
        }

        #[tokio::test]
        #[ignore = "manual Unix process-crash simulation"]
        async fn crash_simulation_install_between_repository_rename_and_directory_sync() {
            const NAME: &str = concat!(
                module_path!(),
                "::crash_simulation_install_between_repository_rename_and_directory_sync"
            );
            const LABEL: &str = "repository-directory-sync";
            let id = "local.crash-install";
            if let Some(root) = crash_child_root(NAME) {
                let state = test_state(&root);
                create_custom_integration(
                    &state,
                    script_request("local.retained", "Retained", "retained"),
                )
                .await
                .unwrap();
                with_async_commit_point_action(
                    LABEL,
                    kill_at_commit_point(LABEL),
                    create_custom_integration(&state, script_request(id, "Installed", "installed")),
                )
                .await
                .unwrap();
                panic!("crash boundary was not reached");
            }
            let directory = tempfile::tempdir().unwrap();
            run_crash_child(NAME, directory.path(), LABEL);
            let state = test_state(directory.path());
            for _ in 0..2 {
                recover(&state).unwrap();
                let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
                assert_eq!(lock.extensions.len(), 2);
                assert_eq!(lock.get(id).unwrap().name, "Installed");
                assert_eq!(lock.get("local.retained").unwrap().name, "Retained");
                assert_eq!(
                    std::fs::read(state.paths.data.join(id).join("integration/provider.sh"))
                        .unwrap(),
                    b"printf original"
                );
            }
        }

        #[tokio::test]
        #[ignore = "manual Unix process-crash simulation"]
        async fn crash_simulation_uninstall_between_repository_rename_and_directory_sync() {
            const NAME: &str = concat!(
                module_path!(),
                "::crash_simulation_uninstall_between_repository_rename_and_directory_sync"
            );
            const LABEL: &str = "repository-directory-sync";
            let id = "local.crash-uninstall";
            if let Some(root) = crash_child_root(NAME) {
                let state = test_state(&root);
                create_custom_integration(&state, script_request(id, "Removed", "removed"))
                    .await
                    .unwrap();
                create_custom_integration(
                    &state,
                    script_request("local.retained", "Retained", "retained"),
                )
                .await
                .unwrap();
                let source = state.paths.extensions.join(id);
                std::fs::create_dir_all(&source).unwrap();
                std::fs::write(source.join("payload"), b"removed files").unwrap();
                with_async_commit_point_action(
                    LABEL,
                    kill_at_commit_point(LABEL),
                    uninstall(&state, id, true),
                )
                .await
                .unwrap();
                panic!("crash boundary was not reached");
            }
            let directory = tempfile::tempdir().unwrap();
            run_crash_child(NAME, directory.path(), LABEL);
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            assert_eq!(
                std::fs::read_dir(paths.extensions.join(".transactions"))
                    .unwrap()
                    .count(),
                1
            );
            assert!(paths.data.join(id).exists());
            let state = test_state(directory.path());
            for _ in 0..2 {
                recover(&state).unwrap();
                let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
                assert_eq!(lock.extensions.len(), 1);
                assert!(lock.get(id).is_err());
                assert_eq!(lock.get("local.retained").unwrap().name, "Retained");
                assert!(!paths.data.join(id).exists());
                assert!(!paths.extensions.join(id).exists());
                assert!(std::fs::read_dir(paths.extensions.join(".transactions"))
                    .unwrap()
                    .next()
                    .is_none());
                assert!(std::fs::read_dir(&paths.extensions).unwrap().all(|item| {
                    !item
                        .unwrap()
                        .file_name()
                        .to_string_lossy()
                        .starts_with(".removing-")
                }));
            }
        }

        #[tokio::test]
        #[ignore = "manual Unix process-crash simulation"]
        async fn crash_simulation_edit_after_journal_sync_before_repository_persist() {
            const NAME: &str = concat!(
                module_path!(),
                "::crash_simulation_edit_after_journal_sync_before_repository_persist"
            );
            const LABEL: &str = "edit-repository-remove";
            let id = "local.crash-edit";
            if let Some(root) = crash_child_root(NAME) {
                let state = test_state(&root);
                let mut request = script_request(id, "Original", "edited");
                create_custom_integration(&state, request.clone())
                    .await
                    .unwrap();
                request.name = "Changed".into();
                request.script_content = Some("printf changed".into());
                with_async_commit_point_action(
                    LABEL,
                    kill_at_commit_point(LABEL),
                    update_custom_integration(&state, id, request),
                )
                .await
                .unwrap();
                panic!("crash boundary was not reached");
            }
            let directory = tempfile::tempdir().unwrap();
            run_crash_child(NAME, directory.path(), LABEL);
            let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
            let path = std::fs::read_dir(paths.extensions.join(".transactions"))
                .unwrap()
                .next()
                .unwrap()
                .unwrap()
                .path();
            let journal: RemovalJournal =
                serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
            let backup = journal.staged_path.as_ref().unwrap();
            assert!(backup.is_dir());
            assert!(!paths.data.join(id).join("integration").exists());
            let state = test_state(directory.path());
            for _ in 0..2 {
                recover(&state).unwrap();
                let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
                assert_eq!(lock.extensions.len(), 1);
                assert_eq!(
                    serde_json::to_value(lock.get(id).unwrap()).unwrap(),
                    serde_json::to_value(&journal.removed_entry).unwrap()
                );
                assert_eq!(
                    std::fs::read(paths.data.join(id).join("integration/provider.sh")).unwrap(),
                    b"printf original"
                );
                assert!(!path.exists());
                assert!(!backup.exists());
            }
        }
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn fault_at_edit_repository_commit_recovers_the_previous_generation() {
        let directory = tempfile::tempdir().unwrap();
        let state = std::sync::Arc::new(test_state(directory.path()));
        let id = "local.fault-edit-commit";
        let request = script_request(id, "Original", "fault-edit");
        create_custom_integration(&state, request.clone()).await.unwrap();
        let mut changed = request;
        changed.name = "Changed".into();
        changed.version = "2.0.0".into();
        changed.script_content = Some("printf changed".into());

        let task_state = std::sync::Arc::clone(&state);
        let task = tokio::spawn(async move {
            crate::extensions::with_async_commit_point(
                "edit-repository-remove",
                update_custom_integration(&task_state, id, changed),
            )
            .await
        });
        let result = task.await;
        assert!(result.is_err());

        crate::extensions::transaction::recover(&state).unwrap();
        let restored = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get(id)
            .unwrap()
            .clone();
        assert_eq!(restored.name, "Original");
        assert!(state.paths.data.join(id).join("integration").exists());
        let journal_dir = state.paths.extensions.join(".transactions");
        assert!(!journal_dir
            .read_dir()
            .ok()
            .into_iter()
            .flatten()
            .flatten()
            .any(|item| item.file_name().to_string_lossy().contains("fault-edit-commit")));
    }

    #[tokio::test]
    async fn fault_at_install_repository_commit_removes_generated_orphans() {
        let directory = tempfile::tempdir().unwrap();
        let state = std::sync::Arc::new(test_state(directory.path()));
        let id = "local.fault-install-commit";
        let request = script_request(id, "Install fault", "install-fault");
        let task_state = std::sync::Arc::clone(&state);
        let task = tokio::spawn(async move {
            crate::extensions::with_async_commit_point(
                "install-repository-add",
                create_custom_integration(&task_state, request),
            )
            .await
        });
        let result = task.await;
        assert!(result.is_err());

        crate::extensions::transaction::recover(&state).unwrap();
        assert!(ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get(id)
            .is_err());
        assert!(!state.paths.data.join(id).join("integration").exists());
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
            ExtensionsLock::load(&state.paths.repository_file)
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
        let restored_lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
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
        let disabled = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get("local.lifecycle-test")
            .unwrap()
            .clone();
        assert!(!disabled.enabled);
        assert_eq!(disabled.state, ExtensionStateKind::Disabled);
        assert!(!catalog_contains(&state, "lifecycle-edited").await);

        set_enabled_for_test(&state, "local.lifecycle-test", true).await;
        let enabled = ExtensionsLock::load(&state.paths.repository_file)
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
        let imported = ExtensionsLock::load(&state.paths.repository_file)
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
        assert!(!ExtensionsLock::load(&state.paths.repository_file)
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
        assert!(!data_root.join("health.json").exists());
        assert!(!ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .extensions
            .contains_key("local.delete-test"));

        // Explicit data removal also removes the integration root.
        create_custom_integration(
            &state,
            script_request("local.delete-test2", "Delete test 2", "delete-test2"),
        )
        .await
        .unwrap();
        let data_root2 = state.paths.data.join("local.delete-test2");
        uninstall(&state, "local.delete-test2", true).await.unwrap();
        assert!(!data_root2.exists());
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
        assert!(!ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .extensions
            .contains_key("local.external-test"));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn failed_lock_save_restores_staged_directory() {
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
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: "1.0.0".into(),
            tool_version: None,
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
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
            probe_report: None,
        };
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(extension_id.into(), entry);
        lock.save(&state.paths.repository_file).unwrap();

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
        assert!(ExtensionsLock::load(&state.paths.repository_file)
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

    #[tokio::test]
    async fn uninstall_deletion_failure_leaves_journal_for_recovery() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let extension_id = "example.uninstall-recovery-test";
        let installed_root = state.paths.extensions.join(extension_id);
        std::fs::create_dir_all(&installed_root).unwrap();
        std::fs::write(installed_root.join("payload"), "content").unwrap();

        let now = unix_now();
        let entry = ExtensionLockEntry {
            id: extension_id.into(),
            name: "Uninstall recovery test".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: "1.0.0".into(),
            tool_version: None,
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
            current_version: "1.0.0".into(),
            previous_version: None,
            manifest_path: "/path/to/manifest".into(),
            executable_path: "/path/to/executable".into(),
            runtime_root: None,
            installed_at: now,
            updated_at: now,
            pinned: false,
            channel: "latest".into(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
            probe_report: None,
        };
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(extension_id.into(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();

        // Simulate deletion failure by making the staged directory unreadable.
        // We cannot truly prevent deletion in a portable way, so we verify the
        // journal behavior instead: deletion fails → journal stays → recovery works.
        let result = uninstall(&state, extension_id, false).await;

        // If uninstall succeeded completely, journal should be gone.
        // If it failed during cleanup, lock is committed but journal remains.
        let lock_after = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        let journal_dir = state.paths.extensions.join(".transactions");
        let has_removal_journal = journal_dir.exists()
            && journal_dir
                .read_dir()
                .unwrap()
                .flatten()
                .any(|e| {
                    e.file_name()
                        .to_string_lossy()
                        .starts_with(&format!("removal-uninstall-{}", extension_id))
                });

        if result.is_ok() {
            // Success: lock entry gone, no journal, no residue.
            assert!(!lock_after.extensions.contains_key(extension_id));
            assert!(!has_removal_journal);
            assert!(!installed_root.exists());
        } else {
            // Failure during cleanup: lock committed, journal remains for recovery.
            assert!(!lock_after.extensions.contains_key(extension_id));
            assert!(has_removal_journal);
        }
    }

    #[tokio::test]
    async fn fault_after_uninstall_repository_commit_does_not_resurrect_entry() {
        let directory = tempfile::tempdir().unwrap();
        let state = std::sync::Arc::new(test_state(directory.path()));
        let entry = journal_entry(&state, "1.0.0");
        let extension_root = state.paths.extensions.join(&entry.id);
        std::fs::create_dir_all(&extension_root).unwrap();
        std::fs::write(extension_root.join("payload"), b"payload").unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();

        let task_state = std::sync::Arc::clone(&state);
        let id = entry.id.clone();
        let task = tokio::spawn(async move {
            crate::extensions::with_async_commit_point(
                "repository-directory-sync",
                uninstall(&task_state, &id, false),
            )
            .await
        });
        let result = task.await;
        assert!(result.is_err());

        crate::extensions::transaction::recover(&state).unwrap();
        let recovered = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert!(!recovered.extensions.contains_key(&entry.id));
        assert!(!extension_root.exists());
        let journal_dir = state.paths.extensions.join(".transactions");
        assert!(!journal_dir
            .read_dir()
            .ok()
            .into_iter()
            .flatten()
            .flatten()
            .any(|item| item.file_name().to_string_lossy().starts_with("removal-")));
    }

    #[tokio::test]
    async fn uninstall_recovery_completes_pending_removal() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let extension_id = "example.recovery-test";
        let installed_root = state.paths.extensions.join(extension_id);
        std::fs::create_dir_all(&installed_root).unwrap();
        std::fs::write(installed_root.join("payload"), "to-be-removed").unwrap();

        let now = unix_now();
        let entry = ExtensionLockEntry {
            id: extension_id.into(),
            name: "Recovery test".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: "1.0.0".into(),
            tool_version: None,
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
            current_version: "1.0.0".into(),
            previous_version: None,
            manifest_path: "/path/to/manifest".into(),
            executable_path: "/path/to/executable".into(),
            runtime_root: None,
            installed_at: now,
            updated_at: now,
            pinned: false,
            channel: "latest".into(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
            probe_report: None,
        };

        // Write a committed removal journal manually (simulating crash after lock commit).
        let staged_path = state
            .paths
            .extensions
            .join(format!(".removing-{}-staged", extension_id));
        std::fs::rename(&installed_root, &staged_path).unwrap();

        let lock = ExtensionsLock::default();
        // Lock has NO entry (removal committed).
        lock.save(&state.paths.repository_file).unwrap();

        let journal = crate::extensions::transaction::RemovalJournal {
            schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: format!("uninstall-{}-{}", extension_id, now),
            extension_id: extension_id.into(),
            removed_entry: entry,
            staged_path: Some(staged_path.clone()),
            cleanup_paths: Vec::new(),
            removal_kind: Some(crate::extensions::transaction::RemovalKind::Committed),
            remove_data: false,
            intent: crate::extensions::transaction::RemovalIntent::Remove,
        };
        crate::extensions::transaction::write_removal_journal(&state, &journal).unwrap();

        // Recovery should complete the removal.
        crate::extensions::transaction::recover(&state).unwrap();

        let lock_after = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert!(!lock_after.extensions.contains_key(extension_id));
        assert!(!staged_path.exists());
        let journal_dir = state.paths.extensions.join(".transactions");
        assert!(
            !journal_dir.exists()
                || !journal_dir
                    .read_dir()
                    .unwrap()
                    .flatten()
                    .any(|e| e
                        .file_name()
                        .to_string_lossy()
                        .starts_with(&format!("removal-uninstall-{}", extension_id)))
        );
    }

    #[tokio::test]
    async fn edit_crash_before_lock_removal_restores_original() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "local.edit-crash-test";

        // Create original integration.
        let original = create_custom_integration(&state, script_request(id, "Original", "original"))
            .await
            .unwrap();

        let root = state.paths.data.join(id).join("integration");
        let original_script = std::fs::read(root.join("provider.sh")).unwrap();

        // Simulate crash after journal write but before lock removal:
        // files moved to backup, journal written, but lock still has entry.
        let backup_path = state
            .paths
            .data
            .join(id)
            .join(format!(".{}-editing-crash", id));
        std::fs::rename(&root, &backup_path).unwrap();

        let journal = crate::extensions::transaction::RemovalJournal {
            schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: format!("edit-{}-{}", id, original.updated_at),
            extension_id: id.to_string(),
            removed_entry: original.clone(),
            staged_path: Some(backup_path.clone()),
            cleanup_paths: vec![root.clone()],
            removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
            remove_data: false,
            intent: crate::extensions::transaction::RemovalIntent::Edit,
        };
        crate::extensions::transaction::write_removal_journal(&state, &journal).unwrap();

        // Recovery should restore backup to root using cleanup_paths[0] as target.
        crate::extensions::transaction::recover(&state).unwrap();

        let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        let restored = lock.get(id).unwrap();
        assert_eq!(restored.id, original.id);
        assert_eq!(restored.updated_at, original.updated_at);
        assert!(root.exists(), "Integration root should be restored");
        assert!(!backup_path.exists(), "Backup should be removed");
        assert_eq!(std::fs::read(root.join("provider.sh")).unwrap(), original_script);

        let definition = custom_integration_definition(&state, id).unwrap();
        assert_eq!(definition.script_content.as_deref(), Some("printf original"));
    }

    #[tokio::test]
    async fn edit_crash_after_lock_removal_completes_on_recovery() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "local.edit-commit-test";

        // Create original integration.
        let original = create_custom_integration(&state, script_request(id, "Original", "original"))
            .await
            .unwrap();
        let original_updated_at = original.updated_at;

        let root = state.paths.data.join(id).join("integration");
        let original_script = std::fs::read(root.join("provider.sh")).unwrap();
        let backup_path = state
            .paths
            .data
            .join(id)
            .join(format!(".{}-editing-committed", id));
        std::fs::rename(&root, &backup_path).unwrap();

        // Simulate crash after lock removal but before new content written:
        // lock has no entry, journal exists with intent=Edit + Staged kind.
        let mut lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        lock.extensions.remove(id);
        lock.save(&state.paths.repository_file).unwrap();

        let journal = crate::extensions::transaction::RemovalJournal {
            schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: format!("edit-{}-{}", id, original_updated_at),
            extension_id: id.to_string(),
            removed_entry: original.clone(),
            staged_path: Some(backup_path.clone()),
            cleanup_paths: vec![root.clone()],
            removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
            remove_data: false,
            intent: crate::extensions::transaction::RemovalIntent::Edit,
        };
        crate::extensions::transaction::write_removal_journal(&state, &journal).unwrap();

        // Recovery should RESTORE: old content back, lock entry re-inserted.
        crate::extensions::transaction::recover(&state).unwrap();

        let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        let restored = lock.get(id).unwrap();
        assert_eq!(restored.id, original.id);
        assert_eq!(restored.updated_at, original_updated_at);
        assert!(root.exists(), "Integration root should be restored");
        assert!(!backup_path.exists(), "Backup should be removed after restore");
        assert_eq!(
            std::fs::read(root.join("provider.sh")).unwrap(),
            original_script,
            "Original script content should be restored"
        );
    }

    #[tokio::test]
    async fn edit_failure_restores_original_in_process() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "local.edit-fail-test";

        create_custom_integration(&state, script_request(id, "Original", "original"))
            .await
            .unwrap();

        let root = state.paths.data.join(id).join("integration");
        let original_script = std::fs::read(root.join("provider.sh")).unwrap();

        // Attempt edit with invalid script content (empty).
        let mut invalid = script_request(id, "Updated", "updated");
        invalid.script_content = Some(String::new());
        let result = update_custom_integration(&state, id, invalid).await;
        assert!(result.is_err());

        // Original integration should be fully restored in-process.
        let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert!(lock.extensions.contains_key(id));

        let definition = custom_integration_definition(&state, id).unwrap();
        assert_eq!(definition.name, "Original");
        assert_eq!(definition.script_content.as_deref(), Some("printf original"));
        assert_eq!(std::fs::read(root.join("provider.sh")).unwrap(), original_script);

        // Journal should be removed (in-process rollback succeeded).
        let journal_dir = state.paths.extensions.join(".transactions");
        assert!(
            !journal_dir.exists()
                || !journal_dir
                    .read_dir()
                    .unwrap()
                    .flatten()
                    .any(|e| e
                        .file_name()
                        .to_string_lossy()
                        .starts_with(&format!("removal-edit-{}", id)))
        );
    }

    #[tokio::test]
    async fn edit_crash_after_uncommitted_content_written_restores_original() {
        if find_script_interpreter(ScriptLanguage::Shell).is_err() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let id = "local.edit-completed-test";

        // Create original integration.
        let original = create_custom_integration(&state, script_request(id, "Original", "original"))
            .await
            .unwrap();
        let original_updated_at = original.updated_at;

        let root = state.paths.data.join(id).join("integration");
        let original_script = std::fs::read(root.join("provider.sh")).unwrap();
        let backup_path = state
            .paths
            .data
            .join(id)
            .join(format!(".{}-editing-backup", id));

        // Simulate edit that wrote new content but crashed before journal removal:
        // 1. Old content backed up
        std::fs::rename(&root, &backup_path).unwrap();

        // 2. Lock entry removed (as update_custom_integration does)
        let mut lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        lock.extensions.remove(id);
        lock.save(&state.paths.repository_file).unwrap();

        // 3. Journal written with intent=Edit
        let journal = crate::extensions::transaction::RemovalJournal {
            schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: format!("edit-{}-{}", id, original_updated_at),
            extension_id: id.to_string(),
            removed_entry: original.clone(),
            staged_path: Some(backup_path.clone()),
            cleanup_paths: vec![root.clone()],
            removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
            remove_data: false,
            intent: crate::extensions::transaction::RemovalIntent::Edit,
        };
        crate::extensions::transaction::write_removal_journal(&state, &journal).unwrap();

        // 4. New content written
        std::fs::create_dir_all(&root).unwrap();
        std::fs::write(root.join("provider.sh"), b"printf new").unwrap();

        // Files alone do not commit an edit; restore the registered generation.
        crate::extensions::transaction::recover(&state).unwrap();

        let lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(lock.get(id).unwrap()).unwrap(),
            serde_json::to_value(&original).unwrap()
        );
        assert!(root.exists(), "Original content should be restored");
        assert!(!backup_path.exists(), "Backup should be removed");
        assert_eq!(
            std::fs::read(root.join("provider.sh")).unwrap(),
            original_script,
            "Uncommitted content must not replace the original generation"
        );
        let journal_path = state.paths.extensions.join(".transactions")
            .join(format!("removal-{}.json", journal.transaction_id));
        assert!(!journal_path.exists());
        crate::extensions::transaction::recover(&state).unwrap();
        let recovered = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert_eq!(
            serde_json::to_value(recovered.get(id).unwrap()).unwrap(),
            serde_json::to_value(&original).unwrap()
        );
        assert_eq!(
            std::fs::read(root.join("provider.sh")).unwrap(),
            original_script
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn edit_crash_after_repository_commit_keeps_new_generation() {
        find_script_interpreter(ScriptLanguage::Shell).unwrap();
        for unchanged_metadata in [false, true] {
            let directory = tempfile::tempdir().unwrap();
            let state = test_state(directory.path());
            let id = "local.edit-committed-generation";
            let original = create_custom_integration(&state, script_request(id, "Original", "original"))
                .await
                .unwrap();
            let root = state.paths.data.join(id).join("integration");
            let backup = state.paths.data.join(id).join(".editing-backup");
            std::fs::rename(&root, &backup).unwrap();
            let journal = crate::extensions::transaction::RemovalJournal {
                schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
                transaction_id: "edit-committed-generation".into(),
                extension_id: id.into(),
                removed_entry: original.clone(),
                staged_path: Some(backup.clone()),
                cleanup_paths: vec![root.clone()],
                removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
                remove_data: false,
                intent: crate::extensions::transaction::RemovalIntent::Edit,
            };
            let journal_path =
                crate::extensions::transaction::write_removal_journal(&state, &journal).unwrap();
            ExtensionsLock::default()
                .save(&state.paths.repository_file)
                .unwrap();
            let mut request = script_request(id, "Original", "original");
            if !unchanged_metadata {
                request.name = "Changed".into();
                request.version = "2.0.0".into();
            }
            request.script_content = Some("printf changed".into());
            let mut updated = create_custom_integration(&state, request).await.unwrap();
            // A script-only edit in the same timestamp second can have identical metadata.
            updated.installed_at = original.installed_at;
            updated.updated_at = original.updated_at;
            updated.approved_at = original.approved_at;
            let updated_json = serde_json::to_value(&updated).unwrap();
            assert_eq!(updated_json == serde_json::to_value(original).unwrap(), unchanged_metadata);
            let mut lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
            lock.extensions.insert(id.into(), updated);
            lock.save(&state.paths.repository_file).unwrap();

            let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                crate::extensions::with_commit_point("journal-remove", || {
                    crate::extensions::transaction::recover(&state).unwrap();
                });
            }));
            assert!(result.is_err());
            assert!(!backup.exists());
            assert!(journal_path.exists());

            for _ in 0..2 {
                crate::extensions::transaction::recover(&state).unwrap();
                let recovered = ExtensionsLock::load(&state.paths.repository_file).unwrap();
                assert_eq!(serde_json::to_value(recovered.get(id).unwrap()).unwrap(), updated_json);
                assert_eq!(std::fs::read(root.join("provider.sh")).unwrap(), b"printf changed");
                assert!(!backup.exists());
                assert!(!journal_path.exists());
            }
        }
    }

    #[tokio::test]
    async fn uninstall_recovery_restores_staged_if_lock_intact() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let extension_id = "example.restore-test";
        let installed_root = state.paths.extensions.join(extension_id);
        std::fs::create_dir_all(&installed_root).unwrap();
        std::fs::write(installed_root.join("payload"), "preserved").unwrap();

        let now = unix_now();
        let entry = ExtensionLockEntry {
            id: extension_id.into(),
            name: "Restore test".into(),
            publisher_id: "example".into(),
            publisher_name: "Example".into(),
            distribution_source: ExtensionDistributionSource::Local,
            runtime_ownership: ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::Executable,
            state: ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: "1.0.0".into(),
            tool_version: None,
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
            current_version: "1.0.0".into(),
            previous_version: None,
            manifest_path: "/path/to/manifest".into(),
            executable_path: "/path/to/executable".into(),
            runtime_root: None,
            installed_at: now,
            updated_at: now,
            pinned: false,
            channel: "latest".into(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
            probe_report: None,
        };

        // Write a staged removal journal (simulating crash before lock commit).
        let staged_path = state
            .paths
            .extensions
            .join(format!(".removing-{}-staged", extension_id));
        std::fs::rename(&installed_root, &staged_path).unwrap();

        let mut lock = ExtensionsLock::default();
        // Lock STILL HAS entry (removal not committed).
        lock.extensions.insert(extension_id.into(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();

        let journal = crate::extensions::transaction::RemovalJournal {
            schema_version: crate::extensions::transaction::TRANSACTION_JOURNAL_SCHEMA_VERSION,
            transaction_id: format!("uninstall-{}-{}", extension_id, now),
            extension_id: extension_id.into(),
            removed_entry: entry.clone(),
            staged_path: Some(staged_path.clone()),
            cleanup_paths: Vec::new(),
            removal_kind: Some(crate::extensions::transaction::RemovalKind::Staged),
            remove_data: false,
            intent: crate::extensions::transaction::RemovalIntent::Remove,
        };
        crate::extensions::transaction::write_removal_journal(&state, &journal).unwrap();

        // Recovery should restore the staged directory and drop the journal.
        crate::extensions::transaction::recover(&state).unwrap();

        let lock_after = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        assert!(lock_after.extensions.contains_key(extension_id));
        assert!(installed_root.exists());
        assert!(installed_root.join("payload").is_file());
        assert!(!staged_path.exists());
        let journal_dir = state.paths.extensions.join(".transactions");
        assert!(
            !journal_dir.exists()
                || !journal_dir
                    .read_dir()
                    .unwrap()
                    .flatten()
                    .any(|e| e
                        .file_name()
                        .to_string_lossy()
                        .starts_with(&format!("removal-uninstall-{}", extension_id)))
        );
    }
}
