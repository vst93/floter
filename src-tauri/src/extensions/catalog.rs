use crate::commands::apps::LocalApplication;
use crate::extensions::lock::{ExtensionProviderKind, ExtensionsLock};
use crate::extensions::provider::{
    execution_plan, ArgumentKind, CommandDescriptor, CompletionItem, ExecutionMode, ExecutionPlan,
    ProviderCompletion, ProviderInvocation,
};
use crate::extensions::{ExtensionPaths, ExtensionState};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::collections::{BTreeMap, HashMap, HashSet};
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};

const PROVIDER_COMMAND_CACHE_TTL: Duration = Duration::from_secs(2);

#[derive(Default)]
pub(crate) struct ProviderCommandCache {
    commands: tokio::sync::Mutex<Option<(Instant, Vec<LoadedProviderCommand>)>>,
}

impl ProviderCommandCache {
    pub async fn invalidate(&self) {
        *self.commands.lock().await = None;
    }
}

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
    #[serde(default)]
    pub environment: BTreeMap<String, String>,
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
        entries.extend(system_command_entries(
            request
                .tokens
                .first()
                .map(String::as_str)
                .unwrap_or_default(),
        ));
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
                execution.environment.extend(request.environment.clone());
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
    let needle = request
        .tokens
        .first()
        .map(String::as_str)
        .unwrap_or_else(|| needle.split_whitespace().next().unwrap_or_default())
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
    let Some(provider) = providers.into_iter().find(|provider| {
        requested_namespace.is_none_or(|requested| requested == provider.namespace)
            && (provider.descriptor.id == command_name
                || provider
                    .descriptor
                    .aliases
                    .iter()
                    .any(|name| name == command_name))
    }) else {
        return Ok(CatalogCompletionResponse {
            items: Vec::new(),
            dynamic: false,
        });
    };
    let static_items = static_completions(&provider.descriptor, request);
    if !provider.dynamic_completion_available {
        return Ok(CatalogCompletionResponse {
            items: static_items,
            dynamic: false,
        });
    }

    let provider_request = json!({
        "command": provider.descriptor.id,
        "args": request.tokens.get(1..).unwrap_or_default(),
        "cwd": request.cwd,
    });
    let dynamic = state
        .provider
        .complete(&provider.invocation, &provider_request)
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
    for provider in providers {
        let descriptor = provider.descriptor;
        let invocation = provider.invocation;
        let namespace = provider.namespace;
        let source_name = provider.source_name;
        let runtime_available = provider.runtime_available;
        let mut args = provider.configured_args;
        args.extend_from_slice(user_args);
        let plan = execution_plan(&descriptor, &invocation, args, cwd)
            .and_then(|mut plan| {
                plan.user_args_start = plan.args.len().checked_sub(user_args.len());
                state.protect_execution_plan(plan)
            })
            .ok();
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
    let mut cache = state.provider_commands.commands.lock().await;
    if let Some((created, commands)) = cache.as_ref() {
        if created.elapsed() <= PROVIDER_COMMAND_CACHE_TTL {
            return Ok(commands.clone());
        }
    }
    let commands = load_provider_commands_uncached(state).await?;
    *cache = Some((Instant::now(), commands.clone()));
    Ok(commands)
}

pub(crate) async fn load_provider_commands_uncached(
    state: &ExtensionState,
) -> Result<Vec<LoadedProviderCommand>, String> {
    let mut lock = ExtensionsLock::load(&state.paths.lock_file)?;
    let entries: Vec<crate::extensions::lock::ExtensionLockEntry> =
        lock.extensions.values().cloned().collect();
    let mut lock_changed = false;
    let mut result = Vec::new();
    for entry in entries {
        let already_broken = entry.state == crate::extensions::lock::ExtensionStateKind::Broken;
        // Broken entries are rechecked so a restored binding can clear the
        // persisted broken state without requiring a manual repair first.
        if !entry.enabled && !already_broken {
            continue;
        }
        if entry.runtime_ownership == crate::extensions::lock::ExtensionRuntimeOwnership::System {
            match state.check_executable_binding(&entry.id, &entry.executable_path) {
                Ok(crate::extensions::tool_lock::LockState::Connected) => {
                    if already_broken && lock.clear_broken(&entry.id)? {
                        lock_changed = true;
                    }
                }
                Ok(binding_state) => {
                    let (code, detail) = match binding_state {
                        crate::extensions::tool_lock::LockState::ReconnectRequired => (
                            "binding-missing",
                            format!(
                                "Executable is no longer available at {}",
                                entry.executable_path
                            ),
                        ),
                        crate::extensions::tool_lock::LockState::ReverifyRequired => (
                            "binding-changed",
                            format!(
                                "Executable fingerprint changed at {}",
                                entry.executable_path
                            ),
                        ),
                        crate::extensions::tool_lock::LockState::Connected => unreachable!(),
                    };
                    if !already_recorded_broken(lock.get(&entry.id)?, code, &detail)
                        && lock.mark_broken(&entry.id, code, &detail)?
                    {
                        lock_changed = true;
                    }
                    continue;
                }
                Err(error) => {
                    if !already_recorded_broken(
                        lock.get(&entry.id)?,
                        "binding-check-failed",
                        &error,
                    ) && lock.mark_broken(&entry.id, "binding-check-failed", &error)?
                    {
                        lock_changed = true;
                    }
                    continue;
                }
            }
        }
        if !entry.enabled {
            // A recovered binding is restored to the disabled state, not
            // silently re-enabled; an explicit reconnect does that.
            continue;
        }
        if matches!(
            entry.provider_kind,
            ExtensionProviderKind::StaticDescriptor | ExtensionProviderKind::BundledStatic
        ) {
            match crate::extensions::registry::static_description(&entry) {
                Ok((description, invocation)) => {
                    let namespace = namespace_for(&entry.id);
                    let source_name = description.provider.name.clone();
                    result.extend(description.commands.into_iter().map(|descriptor| {
                        LoadedProviderCommand {
                            descriptor,
                            invocation: invocation.clone(),
                            namespace: namespace.clone(),
                            source_name: source_name.clone(),
                            runtime_available: crate::extensions::registry::runtime_available(
                                &entry,
                            ),
                            configured_args: Vec::new(),
                            dynamic_completion_available: false,
                        }
                    }));
                }
                Err(error) => eprintln!(
                    "floter: cannot load static integration {}: {error}",
                    entry.id
                ),
            }
            continue;
        }
        let mut invocation = match crate::extensions::registry::provider_invocation(&entry) {
            Ok(invocation) => invocation,
            Err(error) => {
                eprintln!(
                    "floter: cannot load extension {} for command catalog: {error}",
                    entry.id
                );
                continue;
            }
        };
        let configured_args = match crate::extensions::config::apply_persisted_configuration(
            &state.paths.data,
            &mut invocation,
        ) {
            Ok(args) => args,
            Err(error) => {
                eprintln!(
                    "floter: cannot apply configuration for extension {}: {error}",
                    entry.id
                );
                Vec::new()
            }
        };
        let response = match state.provider.describe(&invocation, false).await {
            Ok(response) => response,
            Err(error) => {
                eprintln!(
                    "floter: cannot describe extension {} for command catalog: {error}",
                    entry.id
                );
                continue;
            }
        };
        let namespace = namespace_for(&entry.id);
        let runtime_available = response.runtime_available;
        let source_name = response.description.provider.name.clone();
        result.extend(response.description.commands.into_iter().map(|command| {
            LoadedProviderCommand {
                descriptor: command,
                invocation: invocation.clone(),
                namespace: namespace.clone(),
                source_name: source_name.clone(),
                runtime_available,
                configured_args: configured_args.clone(),
                dynamic_completion_available: runtime_available,
            }
        }));
    }
    if lock_changed {
        if let Err(error) = lock.save(&state.paths.lock_file) {
            eprintln!("floter: cannot persist extension binding state: {error}");
        }
    }
    Ok(result)
}

fn already_recorded_broken(
    entry: &crate::extensions::lock::ExtensionLockEntry,
    code: &str,
    detail: &str,
) -> bool {
    entry.state == crate::extensions::lock::ExtensionStateKind::Broken
        && entry.last_error_code.as_deref() == Some(code)
        && entry.last_error_detail.as_deref() == Some(detail)
}

#[derive(Clone)]
pub(crate) struct LoadedProviderCommand {
    descriptor: CommandDescriptor,
    invocation: ProviderInvocation,
    namespace: String,
    source_name: String,
    runtime_available: bool,
    configured_args: Vec<String>,
    dynamic_completion_available: bool,
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
                    inherit_environment: true,
                    plan_token: None,
                    user_args_start: None,
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
                    mode: command.mode.host_mode(),
                    cwd: None,
                    environment: command.environment,
                    inherit_environment: true,
                    plan_token: None,
                    user_args_start: None,
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

    #[cfg(unix)]
    fn recommended_local_entry(
        root: &std::path::Path,
        executable: &std::path::Path,
    ) -> crate::extensions::lock::ExtensionLockEntry {
        let tools = crate::extensions::recommendations::load_recommended().unwrap();
        let tool = &tools[0];
        std::fs::create_dir_all(root).unwrap();
        std::fs::write(root.join("floter.extension.json"), tool.manifest_bytes).unwrap();
        std::fs::write(
            root.join("provider-description.json"),
            tool.descriptor_bytes,
        )
        .unwrap();
        crate::extensions::lock::ExtensionLockEntry {
            id: tool.manifest.id.clone(),
            name: tool.manifest.name.clone(),
            publisher_id: tool.manifest.publisher.id.clone(),
            publisher_name: tool.manifest.publisher.name.clone(),
            distribution_source: crate::extensions::lock::ExtensionDistributionSource::Local,
            runtime_ownership: crate::extensions::lock::ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::StaticDescriptor,
            state: crate::extensions::lock::ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: tool.description.provider.version.clone(),
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
            current_version: tool.description.provider.version.clone(),
            previous_version: None,
            manifest_path: root
                .join("floter.extension.json")
                .to_string_lossy()
                .into_owned(),
            executable_path: executable.to_string_lossy().into_owned(),
            runtime_root: None,
            installed_at: 1,
            updated_at: 1,
            pinned: false,
            channel: "external".into(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
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

    #[cfg(unix)]
    #[test]
    fn connected_recommendation_commands_resolve_execution_plans() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let tools = crate::extensions::recommendations::load_recommended().unwrap();
        let tool = &tools[0];
        let root = directory.path().join("integration");
        std::fs::create_dir_all(&root).unwrap();
        std::fs::write(root.join("floter.extension.json"), tool.manifest_bytes).unwrap();
        std::fs::write(
            root.join("provider-description.json"),
            tool.descriptor_bytes,
        )
        .unwrap();

        let entry = crate::extensions::lock::ExtensionLockEntry {
            id: tool.manifest.id.clone(),
            name: tool.manifest.name.clone(),
            publisher_id: tool.manifest.publisher.id.clone(),
            publisher_name: tool.manifest.publisher.name.clone(),
            distribution_source: crate::extensions::lock::ExtensionDistributionSource::Local,
            runtime_ownership: crate::extensions::lock::ExtensionRuntimeOwnership::System,
            provider_kind: ExtensionProviderKind::StaticDescriptor,
            state: crate::extensions::lock::ExtensionStateKind::Enabled,
            enabled: true,
            package_name: None,
            package_version: tool.description.provider.version.clone(),
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
            current_version: tool.description.provider.version.clone(),
            previous_version: None,
            manifest_path: root
                .join("floter.extension.json")
                .to_string_lossy()
                .into_owned(),
            executable_path: executable.to_string_lossy().into_owned(),
            runtime_root: None,
            installed_at: 1,
            updated_at: 1,
            pinned: false,
            channel: "external".into(),
            approved_permissions: Vec::new(),
            approved_at: 0,
            approved_manifest_digest: None,
            last_error_code: None,
            last_error_detail: None,
            last_error_at: None,
            broken_reason: None,
            enabled_before_broken: None,
        };

        let (description, invocation) =
            crate::extensions::registry::static_description(&entry).unwrap();
        assert_eq!(
            description
                .commands
                .iter()
                .map(|command| command.id.as_str())
                .collect::<Vec<_>>(),
            ["jv", "diff", "codec", "genpwd", "tt"]
        );
        let command = description
            .commands
            .iter()
            .find(|command| command.id == "jv")
            .unwrap();
        let plan = crate::extensions::provider::execution_plan(
            command,
            &invocation,
            vec!["-file".into(), "data.json".into()],
            Some(directory.path()),
        )
        .unwrap();
        assert_eq!(plan.program, executable.to_string_lossy());
        assert_eq!(plan.args, ["jv", "-file", "data.json"]);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn missing_binding_marks_broken_and_restored_binding_clears_it() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        let original_modified = std::fs::metadata(&executable).unwrap().modified().unwrap();

        let root = directory.path().join("integration");
        let entry = recommended_local_entry(&root, &executable);
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.lock_file).unwrap();

        // First load binds the executable and exposes the provider commands.
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert_eq!(commands.len(), 5);

        // Removing the bound executable marks the entry broken and hides it.
        std::fs::remove_file(&executable).unwrap();
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert!(commands.is_empty());
        let broken = ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .get(&entry.id)
            .unwrap()
            .clone();
        assert_eq!(
            broken.state,
            crate::extensions::lock::ExtensionStateKind::Broken
        );
        assert!(!broken.enabled);
        assert_eq!(broken.last_error_code.as_deref(), Some("binding-missing"));
        assert!(broken
            .broken_reason
            .as_deref()
            .unwrap()
            .contains("no longer available"));

        // Restore the exact bytes and mtime so the recorded fingerprint
        // matches, then reload. Broken clears and the entry returns to its
        // pre-broken intent: it was enabled before, so it comes back enabled
        // without a manual re-enable.
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        std::fs::File::open(&executable)
            .unwrap()
            .set_times(std::fs::FileTimes::new().set_modified(original_modified))
            .unwrap();
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert!(commands.is_empty());
        let restored = ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .get(&entry.id)
            .unwrap()
            .clone();
        assert_eq!(
            restored.state,
            crate::extensions::lock::ExtensionStateKind::Enabled
        );
        assert!(restored.enabled);
        assert_eq!(restored.enabled_before_broken, None);
        assert_eq!(restored.last_error_code, None);
        assert_eq!(restored.broken_reason, None);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn disabled_before_broken_stays_disabled_after_recovery() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        let original_modified = std::fs::metadata(&executable).unwrap().modified().unwrap();

        let root = directory.path().join("integration");
        let mut entry = recommended_local_entry(&root, &executable);
        // The user had disabled the tool before its binding went missing.
        entry.enabled = false;
        entry.state = crate::extensions::lock::ExtensionStateKind::Disabled;
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.mark_broken(&entry.id, "binding-missing", "Executable is gone")
            .unwrap();
        lock.save(&state.paths.lock_file).unwrap();
        assert_eq!(
            lock.get(&entry.id).unwrap().enabled_before_broken,
            Some(false)
        );

        // Recovery keeps the disabled intent instead of forcing enabled.
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();
        std::fs::File::open(&executable)
            .unwrap()
            .set_times(std::fs::FileTimes::new().set_modified(original_modified))
            .unwrap();
        load_provider_commands_uncached(&state).await.unwrap();
        let restored = ExtensionsLock::load(&state.paths.lock_file)
            .unwrap()
            .get(&entry.id)
            .unwrap()
            .clone();
        assert_eq!(
            restored.state,
            crate::extensions::lock::ExtensionStateKind::Disabled
        );
        assert!(!restored.enabled);
        assert_eq!(restored.enabled_before_broken, None);
        assert_eq!(restored.broken_reason, None);
    }

    #[test]
    fn static_complete_returns_matching_arguments() {
        let tools = crate::extensions::recommendations::load_recommended().unwrap();
        let descriptor = tools[0]
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
