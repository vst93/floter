use crate::commands::apps::LocalApplication;
use crate::extensions::lock::ExtensionsLock;
use crate::extensions::manifest::{ExtensionManifest, PlatformTarget};
use crate::extensions::provider::{
    execution_plan, ArgumentKind, CommandDescriptor, CompletionItem, ExecutionMode, ExecutionPlan,
    ProviderCompletion, ProviderInvocation,
};
use crate::extensions::{ExtensionPaths, ExtensionState};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::collections::{BTreeMap, HashMap, HashSet};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub enum CatalogSourceKind {
    SystemApplication,
    SystemCommand,
    Local,
    Provider,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CatalogEntry {
    pub id: String,
    pub command: String,
    pub namespace: String,
    pub qualified_command: String,
    pub name: String,
    pub description: String,
    pub source_kind: CatalogSourceKind,
    pub source_name: String,
    pub aliases: Vec<String>,
    pub arguments: Vec<crate::extensions::provider::ArgumentDescriptor>,
    pub execution: Option<ExecutionPlan>,
    pub runtime_available: bool,
    pub frequency: u64,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CatalogSearchRequest {
    pub query: String,
    #[serde(default)]
    pub tokens: Vec<String>,
    pub cwd: Option<String>,
    #[serde(default = "default_limit")]
    pub limit: usize,
    #[serde(default = "default_true")]
    pub include_system_commands: bool,
}

fn default_limit() -> usize {
    30
}

fn default_true() -> bool {
    true
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CompletionRequest {
    pub command: String,
    pub tokens: Vec<String>,
    pub cwd: Option<String>,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct CatalogCompletionResponse {
    pub items: Vec<CompletionItem>,
    pub dynamic: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LocalCommand {
    id: String,
    command: String,
    name: String,
    #[serde(default)]
    description: String,
    #[serde(default)]
    aliases: Vec<String>,
    program: String,
    #[serde(default)]
    args_prefix: Vec<String>,
    #[serde(default = "default_local_mode")]
    mode: ExecutionMode,
    #[serde(default)]
    environment: BTreeMap<String, String>,
}

fn default_local_mode() -> ExecutionMode {
    ExecutionMode::Pty
}

pub async fn search(
    state: &ExtensionState,
    request: &CatalogSearchRequest,
    applications: &[LocalApplication],
) -> Result<Vec<CatalogEntry>, String> {
    let mut entries = application_entries(applications);
    entries.extend(local_entries(&state.paths)?);
    entries.extend(
        provider_entries(
            state,
            request.cwd.as_deref(),
            request.tokens.get(1..).unwrap_or_default(),
        )
        .await?,
    );
    if request.include_system_commands {
        entries.extend(system_command_entries(&request.query));
    }
    for entry in &mut entries {
        if matches!(
            entry.source_kind,
            CatalogSourceKind::SystemCommand | CatalogSourceKind::Local
        ) {
            if let Some(execution) = &mut entry.execution {
                execution
                    .args
                    .extend_from_slice(request.tokens.get(1..).unwrap_or_default());
                execution.cwd.clone_from(&request.cwd);
            }
        }
    }
    let usage = load_usage(&state.paths);
    for entry in &mut entries {
        entry.frequency = usage.get(&entry.id).copied().unwrap_or(0);
    }

    let query = request.query.trim();
    let (namespace, needle) = query
        .split_once(':')
        .map_or((None, query), |(namespace, command)| {
            (Some(namespace), command)
        });
    let needle = needle
        .split_whitespace()
        .next()
        .unwrap_or_default()
        .to_ascii_lowercase();
    let mut scored = entries
        .into_iter()
        .filter_map(|entry| {
            if namespace.is_some_and(|namespace| namespace != entry.namespace) {
                return None;
            }
            score_entry(&entry, &needle).map(|score| (entry, score))
        })
        .collect::<Vec<_>>();
    scored.sort_by(|(left, left_score), (right, right_score)| {
        right_score
            .cmp(left_score)
            .then_with(|| right.frequency.cmp(&left.frequency))
            .then_with(|| left.command.cmp(&right.command))
            .then_with(|| left.namespace.cmp(&right.namespace))
    });
    Ok(scored
        .into_iter()
        .take(request.limit.clamp(1, 200))
        .map(|(entry, _)| entry)
        .collect())
}

pub async fn complete(
    state: &ExtensionState,
    request: &CompletionRequest,
) -> Result<CatalogCompletionResponse, String> {
    let providers = loaded_provider_commands(state).await?;
    let (requested_namespace, command_name) = request
        .command
        .split_once(':')
        .map_or((None, request.command.as_str()), |(namespace, command)| {
            (Some(namespace), command)
        });
    let Some((descriptor, invocation, _, _, _, _, supports_dynamic_complete)) =
        providers.into_iter().find(|(command, _, ns, _, _, _, _)| {
            requested_namespace.is_none_or(|requested| requested == ns)
                && (command.id == command_name
                    || command.aliases.iter().any(|name| name == command_name))
        })
    else {
        return Ok(CatalogCompletionResponse {
            items: Vec::new(),
            dynamic: false,
        });
    };
    let static_items = static_completions(&descriptor, request);
    if !supports_dynamic_complete {
        return Ok(CatalogCompletionResponse {
            items: static_items,
            dynamic: false,
        });
    }

    let provider_request = json!({
        "command": descriptor.id,
        "args": request.tokens.get(1..).unwrap_or_default(),
        "cwd": request.cwd,
    });
    let dynamic = state
        .provider
        .complete(&invocation, &provider_request)
        .await;
    Ok(completion_response(static_items, dynamic))
}

fn static_completions(
    descriptor: &CommandDescriptor,
    request: &CompletionRequest,
) -> Vec<CompletionItem> {
    let fragment = request
        .tokens
        .last()
        .map(String::as_str)
        .unwrap_or_default();
    let previous = request
        .tokens
        .len()
        .checked_sub(2)
        .and_then(|index| request.tokens.get(index));
    let value_argument = previous.and_then(|name| {
        descriptor
            .arguments
            .iter()
            .find(|argument| argument.takes_value && argument.names.contains(name))
    });

    if let Some(argument) = value_argument {
        match argument.kind {
            ArgumentKind::Enum => {
                return argument
                    .values
                    .iter()
                    .filter(|value| value.starts_with(fragment))
                    .map(|value| CompletionItem {
                        value: value.clone(),
                        label: value.clone(),
                        description: argument.description.clone(),
                    })
                    .collect();
            }
            ArgumentKind::Path | ArgumentKind::Directory => {
                return path_completions(
                    fragment,
                    request.cwd.as_deref(),
                    argument.kind == ArgumentKind::Directory,
                );
            }
            ArgumentKind::Command => return Vec::new(),
            _ => {}
        }
    }

    let mut seen = HashSet::new();
    let mut items = Vec::new();
    for argument in &descriptor.arguments {
        for name in &argument.names {
            if name.starts_with(fragment) && seen.insert(name.clone()) {
                items.push(CompletionItem {
                    value: name.clone(),
                    label: name.clone(),
                    description: argument.description.clone(),
                });
            }
        }
    }
    items.sort_by(|left, right| left.value.cmp(&right.value));
    items
}

fn merge_completions(
    static_items: Vec<CompletionItem>,
    dynamic: ProviderCompletion,
) -> Vec<CompletionItem> {
    let mut items = static_items
        .into_iter()
        .map(|item| (item.value.clone(), item))
        .collect::<BTreeMap<_, _>>();
    for completion in dynamic.completions {
        items.insert(
            completion.label.clone(),
            CompletionItem {
                value: completion.label.clone(),
                label: completion.label,
                description: completion.detail,
            },
        );
    }
    items.into_values().collect()
}

fn completion_response(
    static_items: Vec<CompletionItem>,
    dynamic: Result<ProviderCompletion, String>,
) -> CatalogCompletionResponse {
    match dynamic {
        Ok(dynamic) => CatalogCompletionResponse {
            items: merge_completions(static_items, dynamic),
            dynamic: true,
        },
        Err(_) => CatalogCompletionResponse {
            items: static_items,
            dynamic: false,
        },
    }
}

async fn provider_entries(
    state: &ExtensionState,
    cwd: Option<&str>,
    user_args: &[String],
) -> Result<Vec<CatalogEntry>, String> {
    let providers = loaded_provider_commands(state).await?;
    let cwd = cwd.map(Path::new);
    let mut entries = Vec::new();
    for (descriptor, invocation, namespace, source_name, runtime_available, configured_args, _) in
        providers
    {
        let mut args = configured_args;
        args.extend_from_slice(user_args);
        let plan = execution_plan(&descriptor, &invocation, args, cwd).ok();
        entries.push(CatalogEntry {
            id: format!("provider:{}:{}", invocation.extension_id, descriptor.id),
            command: descriptor.id.clone(),
            qualified_command: format!("{namespace}:{}", descriptor.id),
            namespace,
            name: descriptor.name.clone(),
            description: descriptor.description.clone(),
            source_kind: CatalogSourceKind::Provider,
            source_name,
            aliases: descriptor.aliases.clone(),
            arguments: descriptor.arguments.clone(),
            execution: plan,
            runtime_available,
            frequency: 0,
        });
    }
    Ok(entries)
}

async fn loaded_provider_commands(
    state: &ExtensionState,
) -> Result<Vec<LoadedProviderCommand>, String> {
    let lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let installed_ids = lock.extensions.keys().cloned().collect::<HashSet<_>>();
    let mut result = Vec::new();
    for entry in lock.extensions.values().filter(|entry| entry.enabled) {
        let mut invocation = match invocation_from_entry(entry) {
            Ok(invocation) => invocation,
            Err(_) => continue,
        };
        let configured_args = crate::extensions::config::apply_persisted_configuration(
            &state.paths.data,
            &mut invocation,
        )
        .unwrap_or_default();
        let response = match state.provider.describe(&invocation, false).await {
            Ok(response) => response,
            Err(_) => continue,
        };
        let namespace = namespace_for(&entry.id);
        let runtime_available = response.runtime_available;
        let source_name = response.description.provider.name.clone();
        result.extend(response.description.commands.into_iter().map(|command| {
            (
                command,
                invocation.clone(),
                namespace.clone(),
                source_name.clone(),
                runtime_available,
                configured_args.clone(),
                runtime_available,
            )
        }));
    }
    result.extend(static_provider_commands(
        &state.static_adapters,
        &installed_ids,
        &state.paths.data,
    ));
    Ok(result)
}

type LoadedProviderCommand = (
    CommandDescriptor,
    ProviderInvocation,
    String,
    String,
    bool,
    Vec<String>,
    bool,
);

fn static_provider_commands(
    adapters: &[crate::extensions::static_adapter::StaticAdapter],
    installed_ids: &HashSet<String>,
    data_root: &Path,
) -> Vec<LoadedProviderCommand> {
    let mut result = Vec::new();
    for adapter in adapters
        .iter()
        .filter(|adapter| !installed_ids.contains(&adapter.manifest.id))
    {
        let mut invocation = adapter.invocation.clone();
        let configured_args =
            crate::extensions::config::apply_persisted_configuration(data_root, &mut invocation)
                .unwrap_or_default();
        let namespace = namespace_for(&adapter.manifest.id);
        let source_name = adapter.description.provider.name.clone();
        result.extend(adapter.description.commands.iter().cloned().map(|command| {
            (
                command,
                invocation.clone(),
                namespace.clone(),
                source_name.clone(),
                adapter.runtime_available,
                configured_args.clone(),
                false,
            )
        }));
    }
    result
}

fn application_entries(applications: &[LocalApplication]) -> Vec<CatalogEntry> {
    applications
        .iter()
        .map(|application| CatalogEntry {
            id: format!("application:{}", application.path),
            command: application.name.clone(),
            namespace: "app".into(),
            qualified_command: format!("app:{}", application.name),
            name: application
                .localized_name
                .clone()
                .unwrap_or_else(|| application.name.clone()),
            description: application
                .comment
                .clone()
                .unwrap_or_else(|| "Application".into()),
            source_kind: CatalogSourceKind::SystemApplication,
            source_name: "System applications".into(),
            aliases: application.aliases.clone(),
            arguments: Vec::new(),
            execution: None,
            runtime_available: true,
            frequency: 0,
        })
        .collect()
}

fn system_command_entries(query: &str) -> Vec<CatalogEntry> {
    let command_query = query
        .split_once(':')
        .map_or(query, |(_, command)| command)
        .split_whitespace()
        .next()
        .unwrap_or_default()
        .to_ascii_lowercase();
    if command_query.is_empty() {
        return Vec::new();
    }
    let mut seen = HashSet::new();
    let mut result = Vec::new();
    let Some(path) = std::env::var_os("PATH") else {
        return result;
    };
    for directory in std::env::split_paths(&path) {
        let Ok(entries) = std::fs::read_dir(directory) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if !is_executable(&path) {
                continue;
            }
            let Some(name) = path.file_name().and_then(|name| name.to_str()) else {
                continue;
            };
            let command = executable_command_name(name);
            if !command.to_ascii_lowercase().contains(&command_query)
                || !seen.insert(command.clone())
            {
                continue;
            }
            let (program, args) = system_execution(&path);
            result.push(CatalogEntry {
                id: format!("system:{command}"),
                qualified_command: format!("system:{command}"),
                command: command.clone(),
                namespace: "system".into(),
                name: command,
                description: path.to_string_lossy().into_owned(),
                source_kind: CatalogSourceKind::SystemCommand,
                source_name: "PATH".into(),
                aliases: Vec::new(),
                arguments: Vec::new(),
                execution: Some(ExecutionPlan {
                    program,
                    args,
                    mode: ExecutionMode::Pty,
                    cwd: None,
                    environment: BTreeMap::new(),
                }),
                runtime_available: true,
                frequency: 0,
            });
        }
    }
    result
}

fn local_entries(paths: &ExtensionPaths) -> Result<Vec<CatalogEntry>, String> {
    let path = paths.root.join("local-commands.json");
    if !path.exists() {
        return Ok(Vec::new());
    }
    let commands: Vec<LocalCommand> = serde_json::from_slice(
        &std::fs::read(&path)
            .map_err(|error| format!("Cannot read {}: {error}", path.display()))?,
    )
    .map_err(|error| format!("Invalid {}: {error}", path.display()))?;
    Ok(commands
        .into_iter()
        .map(|command| {
            let (program, mut args) = system_execution(Path::new(&command.program));
            args.extend(command.args_prefix);
            CatalogEntry {
                id: format!("local:{}", command.id),
                qualified_command: format!("local:{}", command.command),
                namespace: "local".into(),
                name: command.name,
                description: command.description,
                source_kind: CatalogSourceKind::Local,
                source_name: "Local configuration".into(),
                aliases: command.aliases,
                arguments: Vec::new(),
                execution: Some(ExecutionPlan {
                    program,
                    args,
                    mode: command.mode,
                    cwd: None,
                    environment: command.environment,
                }),
                runtime_available: true,
                frequency: 0,
                command: command.command,
            }
        })
        .collect())
}

fn path_completions(
    fragment: &str,
    cwd: Option<&str>,
    directories_only: bool,
) -> Vec<CompletionItem> {
    let raw = Path::new(fragment);
    let base = if raw.is_absolute() {
        raw.parent().unwrap_or(Path::new("/")).to_path_buf()
    } else {
        let cwd = cwd
            .map(PathBuf::from)
            .or_else(|| std::env::current_dir().ok());
        cwd.unwrap_or_default()
            .join(raw.parent().unwrap_or(Path::new("")))
    };
    let prefix = raw
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default();
    let display_parent = raw.parent().unwrap_or(Path::new(""));
    let Ok(entries) = std::fs::read_dir(base) else {
        return Vec::new();
    };
    let mut items = entries
        .flatten()
        .filter_map(|entry| {
            let name = entry.file_name().to_string_lossy().into_owned();
            let file_type = entry.file_type().ok()?;
            if !name.starts_with(prefix) || (directories_only && !file_type.is_dir()) {
                return None;
            }
            let suffix = if file_type.is_dir() {
                std::path::MAIN_SEPARATOR_STR
            } else {
                ""
            };
            let value = display_parent.join(format!("{name}{suffix}"));
            Some(CompletionItem {
                value: value.to_string_lossy().into_owned(),
                label: format!("{name}{suffix}"),
                description: if file_type.is_dir() {
                    "Directory"
                } else {
                    "File"
                }
                .into(),
            })
        })
        .collect::<Vec<_>>();
    items.sort_by(|left, right| left.label.cmp(&right.label));
    items.truncate(100);
    items
}

fn score_entry(entry: &CatalogEntry, needle: &str) -> Option<u32> {
    if needle.is_empty() {
        return Some(1);
    }
    let command = entry.command.to_ascii_lowercase();
    if command == needle {
        return Some(1_000);
    }
    if command.starts_with(needle) {
        return Some(800 - (command.len() - needle.len()).min(100) as u32);
    }
    if command.contains(needle) {
        return Some(600);
    }
    let name = entry.name.to_ascii_lowercase();
    if name.starts_with(needle) {
        return Some(500);
    }
    if name.contains(needle)
        || entry
            .aliases
            .iter()
            .any(|alias| alias.to_ascii_lowercase().contains(needle))
    {
        return Some(300);
    }
    None
}

fn namespace_for(id: &str) -> String {
    id.rsplit(['.', '_', '-'])
        .find(|part| !part.is_empty())
        .unwrap_or(id)
        .to_ascii_lowercase()
}

fn load_usage(paths: &ExtensionPaths) -> HashMap<String, u64> {
    std::fs::read(paths.root.join("catalog-usage.json"))
        .ok()
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
        .unwrap_or_default()
}

fn executable_command_name(name: &str) -> String {
    #[cfg(target_os = "windows")]
    for extension in [".exe", ".cmd", ".bat"] {
        if name.to_ascii_lowercase().ends_with(extension) {
            return name[..name.len() - extension.len()].to_string();
        }
    }
    name.to_string()
}

fn system_execution(path: &Path) -> (String, Vec<String>) {
    #[cfg(target_os = "windows")]
    {
        let extension = path
            .extension()
            .and_then(|extension| extension.to_str())
            .unwrap_or_default();
        if extension.eq_ignore_ascii_case("cmd") || extension.eq_ignore_ascii_case("bat") {
            return (
                "cmd.exe".into(),
                vec![
                    "/D".into(),
                    "/S".into(),
                    "/C".into(),
                    path.to_string_lossy().into_owned(),
                ],
            );
        }
    }
    (path.to_string_lossy().into_owned(), Vec::new())
}

fn is_executable(path: &Path) -> bool {
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

pub fn invocation_from_entry(
    entry: &crate::extensions::lock::ExtensionLockEntry,
) -> Result<ProviderInvocation, String> {
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    if manifest.id != entry.id || manifest.publisher.id != entry.publisher_id {
        return Err(format!(
            "Manifest identity does not match lock entry {}",
            entry.id
        ));
    }
    let version_args = match &manifest.runtime {
        crate::extensions::manifest::Runtime::Linked { version_args, .. } => version_args.clone(),
        crate::extensions::manifest::Runtime::Managed { .. } => Vec::new(),
    };
    let permissions = manifest.permissions.clone();
    let resolved = manifest.resolve(PlatformTarget::current()?)?;
    Ok(ProviderInvocation {
        extension_id: entry.id.clone(),
        executable: PathBuf::from(&entry.executable_path),
        runtime_root: entry.runtime_root.as_ref().map(PathBuf::from),
        package_version: entry.package_version.clone(),
        tool_version_hint: entry.tool_version.clone(),
        version_args,
        config: resolved.provider,
        permissions,
    })
}

pub fn migrate_legacy_commands(paths: &ExtensionPaths) -> Result<usize, String> {
    let source = paths.root.join("commands.json");
    let target = paths.root.join("local-commands.json");
    if !source.exists() || target.exists() {
        return Ok(0);
    }
    #[derive(Deserialize)]
    struct LegacyCommand {
        id: String,
        name: String,
        command: String,
    }
    let legacy: Vec<LegacyCommand> = serde_json::from_slice(
        &std::fs::read(&source).map_err(|error| format!("Cannot read legacy commands: {error}"))?,
    )
    .map_err(|error| format!("Invalid legacy commands: {error}"))?;
    let mut migrated = Vec::new();
    for command in legacy {
        if command
            .command
            .chars()
            .any(|character| "|&;<>()$`\n\r\"'".contains(character))
        {
            continue;
        }
        let mut words = command.command.split_whitespace();
        let Some(program) = words.next() else {
            continue;
        };
        migrated.push(LocalCommand {
            id: command.id,
            command: command.name.to_ascii_lowercase().replace(' ', "-"),
            name: command.name,
            description: "Migrated from commands.json".into(),
            aliases: Vec::new(),
            program: program.into(),
            args_prefix: words.map(String::from).collect(),
            mode: ExecutionMode::Pty,
            environment: BTreeMap::new(),
        });
    }
    let bytes = serde_json::to_vec_pretty(&migrated)
        .map_err(|error| format!("Cannot serialize migrated commands: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(&paths.root)
        .map_err(|error| format!("Cannot create migration file: {error}"))?;
    use std::io::Write;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write migrated commands: {error}"))?;
    temporary
        .persist(&target)
        .map_err(|error| format!("Cannot persist {}: {error}", target.display()))?;
    Ok(migrated.len())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn completion_item(value: &str, description: &str) -> CompletionItem {
        CompletionItem {
            value: value.into(),
            label: value.into(),
            description: description.into(),
        }
    }

    #[test]
    fn namespace_uses_last_provider_id_component() {
        assert_eq!(namespace_for("io.github.vst93.v"), "v");
    }

    #[test]
    fn exact_commands_rank_above_name_matches() {
        let mut entry = CatalogEntry {
            id: "provider:x:jv".into(),
            command: "jv".into(),
            namespace: "x".into(),
            qualified_command: "x:jv".into(),
            name: "JSON Viewer".into(),
            description: String::new(),
            source_kind: CatalogSourceKind::Provider,
            source_name: "x".into(),
            aliases: Vec::new(),
            arguments: Vec::new(),
            execution: None,
            runtime_available: true,
            frequency: 0,
        };
        assert_eq!(score_entry(&entry, "jv"), Some(1_000));
        entry.command = "other".into();
        assert_eq!(score_entry(&entry, "json"), Some(500));
    }

    #[test]
    fn bundled_static_adapter_is_loaded_without_an_install_lock() {
        let directory = tempfile::tempdir().unwrap();
        let paths = ExtensionPaths::from_root(directory.path().to_path_buf());
        paths.ensure().unwrap();
        let adapters = crate::extensions::static_adapter::load_bundled().unwrap();
        let commands = static_provider_commands(&adapters, &HashSet::new(), &paths.data);

        assert_eq!(commands.len(), 5);
        assert_eq!(commands[0].0.id, "jv");
        assert_eq!(commands[0].2, "v");
        assert_eq!(commands[0].3, "V Tools");
    }

    #[test]
    fn static_complete_returns_matching_arguments() {
        let adapters = crate::extensions::static_adapter::load_bundled().unwrap();
        let descriptor = adapters[0]
            .description
            .commands
            .iter()
            .find(|command| command.id == "jv")
            .unwrap();
        let items = static_completions(
            descriptor,
            &CompletionRequest {
                command: "jv".into(),
                tokens: vec!["jv".into(), "-f".into()],
                cwd: None,
            },
        );

        assert_eq!(
            items
                .iter()
                .map(|item| item.value.as_str())
                .collect::<Vec<_>>(),
            ["-f", "-file"]
        );
    }

    #[test]
    fn dynamic_timeout_falls_back_to_static_completions() {
        let static_items = vec![completion_item("-file", "Static file")];

        let response = completion_response(
            static_items.clone(),
            Err("Provider complete timed out after 800 ms".into()),
        );

        assert_eq!(response.items, static_items);
        assert!(!response.dynamic);
    }

    #[test]
    fn dynamic_error_falls_back_to_static_completions() {
        let static_items = vec![completion_item("-file", "Static file")];

        let response = completion_response(
            static_items.clone(),
            Err("Provider complete exited with 2: unsupported".into()),
        );

        assert_eq!(response.items, static_items);
        assert!(!response.dynamic);
    }

    #[test]
    fn dynamic_and_static_completions_are_merged_deduplicated_and_sorted() {
        let static_items = vec![
            completion_item("-file", "Static file"),
            completion_item("-f", "Format JSON"),
        ];
        let dynamic = ProviderCompletion {
            completions: vec![
                crate::extensions::provider::ProviderCompletionItem {
                    label: "-fresh".into(),
                    kind: "flag".into(),
                    detail: "Dynamic only".into(),
                },
                crate::extensions::provider::ProviderCompletionItem {
                    label: "-file".into(),
                    kind: "flag".into(),
                    detail: "Dynamic file".into(),
                },
            ],
        };

        let response = completion_response(static_items, Ok(dynamic));

        assert!(response.dynamic);
        assert_eq!(
            response
                .items
                .iter()
                .map(|item| item.value.as_str())
                .collect::<Vec<_>>(),
            ["-f", "-file", "-fresh"]
        );
        assert_eq!(response.items[1].description, "Dynamic file");
    }
}
