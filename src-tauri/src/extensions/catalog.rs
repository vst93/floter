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
use std::sync::Arc;
use std::time::{Duration, Instant};

// Correctness does NOT depend on this TTL being short: every mutating path
// (enable/disable, connect, reprobe, custom update, config changes) calls
// `ExtensionState::invalidate_provider_commands`, which drops the cached
// entry immediately. The TTL exists purely as a safety net against a missed
// invalidation hook, so it only bounds how long a stale table can survive.
const PROVIDER_COMMAND_CACHE_TTL: Duration = Duration::from_secs(60);

#[derive(Default)]
pub(crate) struct ProviderCommandCache {
    /// Fresh entry plus its creation time; shared with readers via `Arc`.
    cached: tokio::sync::Mutex<Option<(Instant, Arc<Vec<LoadedProviderCommand>>)>>,
    /// Serializes reloads so concurrent misses share ONE disk reload.
    reload: tokio::sync::Mutex<()>,
    /// Test-only: counts explicit invalidations so tests can prove a binding
    /// change did (or did not) drop the cache.
    #[cfg(test)]
    invalidations: std::sync::atomic::AtomicU64,
}

impl ProviderCommandCache {
    pub async fn invalidate(&self) {
        let mut cached = self.cached.lock().await;
        // Nothing cached is already the desired state. This keeps a read path
        // that notices an ongoing divergence (e.g. a listing polled while the
        // catalog has not yet re-read the tool) from churning the counter or
        // re-dropping an already-empty cache on every poll.
        if cached.is_none() {
            return;
        }
        #[cfg(test)]
        self.invalidations
            .fetch_add(1, std::sync::atomic::Ordering::SeqCst);
        *cached = None;
    }

    /// Test-only: how many explicit invalidations have been observed.
    #[cfg(test)]
    pub(crate) fn invalidation_count(&self) -> u64 {
        self.invalidations.load(std::sync::atomic::Ordering::SeqCst)
    }

    /// Test-only: whether a live (unexpired-check-agnostic) entry is cached.
    #[cfg(test)]
    pub(crate) async fn has_cached_entry(&self) -> bool {
        self.cached.lock().await.is_some()
    }

    /// Test-only: push the stored creation time backwards to simulate expiry.
    #[cfg(test)]
    async fn age_entry_for_test(&self, delta: Duration) {
        if let Some((created, _)) = self.cached.lock().await.as_mut() {
            *created = created.checked_sub(delta).unwrap_or_else(Instant::now);
        }
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
    /// R7-11: resolved user aliases (`command name -> alias`). The frontend
    /// already applied [`resolve_command_aliases`] to the raw settings map, so
    /// the backend can score each alias directly. `#[serde(default)]` keeps
    /// older callers that never send the key working.
    #[serde(default)]
    pub command_aliases: HashMap<String, String>,
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
    // R7-11: the conflict policy (a shared alias is locked by the first command
    // in name order) is applied here, not trusted from the caller, so the
    // backend ranks with exactly the map the settings loader would have stored.
    // Calling it on an already-resolved map is idempotent.
    let resolved_aliases =
        crate::commands::config::resolve_command_aliases(&request.command_aliases);
    let mut scored = entries
        .into_iter()
        .filter_map(|entry| {
            if namespace.is_some_and(|namespace| namespace != entry.namespace) {
                return None;
            }
            score_entry(&entry, &needle, &resolved_aliases).map(|score| (entry, score))
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
    let Some(provider) = providers.iter().find(|provider| {
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

/// R39 · one external plugin command as the launcher's plugin modes see it.
///
/// This is the *registry* half of the per-command switch feature: the
/// integrations panel renders one switch per entry, and the launcher resolves
/// a typed trigger word against the enabled subset. It carries only facts the
/// provider descriptor already declares — no new authority, no new field.
#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct PluginCommandInfo {
    /// The owning extension's id (the switch map's outer key).
    pub extension_id: String,
    /// The extension's human name (`source_name`), for the switch's caption.
    pub extension_name: String,
    /// The command id — the string the user types to enter the mode, and the
    /// switch map's inner key.
    pub command_id: String,
    pub name: String,
    pub description: String,
    pub aliases: Vec<String>,
    /// Whether the extension's runtime binding resolves right now. A command
    /// whose runtime is missing may still be listed (the switch keeps its
    /// state), but a run refuses with a named error.
    pub runtime_available: bool,
}

/// R39 · the whole external plugin command registry.
///
/// One entry per command of every extension that loads a provider descriptor —
/// the same table the catalog search reads (`loaded_provider_commands`), so a
/// command can never be searchable without being switchable, or the reverse.
/// Sorted by `(extensionId, commandId)` so the panel's rows and the tests are
/// deterministic regardless of the lock's iteration order.
pub async fn plugin_command_registry(
    state: &ExtensionState,
) -> Result<Vec<PluginCommandInfo>, String> {
    let providers = loaded_provider_commands(state).await?;
    let mut infos: Vec<PluginCommandInfo> = providers
        .iter()
        .map(|provider| PluginCommandInfo {
            extension_id: provider.invocation.extension_id.clone(),
            extension_name: provider.source_name.clone(),
            command_id: provider.descriptor.id.clone(),
            name: provider.descriptor.name.clone(),
            description: provider.descriptor.description.clone(),
            aliases: provider.descriptor.aliases.clone(),
            runtime_available: provider.runtime_available,
        })
        .collect();
    infos.sort_by(|left, right| {
        left.extension_id
            .cmp(&right.extension_id)
            .then_with(|| left.command_id.cmp(&right.command_id))
    });
    Ok(infos)
}

/// R39 · the captured result of one external plugin command run.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PluginCommandOutput {
    pub success: bool,
    pub exit_code: Option<i32>,
    pub duration_ms: u64,
    pub stdout: String,
    pub stderr: String,
    pub truncated: bool,
}

/// R39 · run one external plugin command with the launcher's own argv.
///
/// This is the execution half of the plugin mode. It resolves the command out
/// of the **same** provider table the catalog search uses, builds the plan with
/// the **same** `provider::execution_plan` the catalog uses (which enforces the
/// command's declared `execution.program` and the `process-spawn` permission),
/// and spawns it directly with `std::process::Command`-equivalent argv — there
/// is no shell, no string interpolation, and no new allowlist entry. A command
/// the user disabled is refused *before* this function is reached (the frontend
/// gate), and a command that does not exist is refused here.
///
/// `args` is the launcher field's own text, split into argv items by the
/// frontend. Every item is one argument; nothing is ever joined into a shell
/// string.
pub async fn run_plugin_command(
    state: &ExtensionState,
    extension_id: &str,
    command_id: &str,
    args: Vec<String>,
    cwd: Option<&str>,
) -> Result<PluginCommandOutput, String> {
    let providers = loaded_provider_commands(state).await?;
    let provider = providers
        .iter()
        .find(|provider| {
            provider.invocation.extension_id == extension_id
                && (provider.descriptor.id == command_id
                    || provider
                        .descriptor
                        .aliases
                        .iter()
                        .any(|alias| alias == command_id))
        })
        .ok_or_else(|| format!("Unknown plugin command: {extension_id}:{command_id}"))?;
    if !provider.runtime_available {
        return Err(format!(
            "Plugin command runtime is unavailable: {extension_id}:{command_id}"
        ));
    }
    let mut argv = provider.configured_args.clone();
    argv.extend(args);
    let plan = execution_plan(
        &provider.descriptor,
        &provider.invocation,
        argv,
        cwd.map(Path::new),
    )?;
    let started = Instant::now();
    let (success, exit_code, output) =
        crate::extensions::run::execute_plan_background(plan).await?;
    Ok(PluginCommandOutput {
        success,
        exit_code,
        duration_ms: started.elapsed().as_millis() as u64,
        stdout: output.stdout,
        stderr: output.stderr,
        truncated: output.truncated,
    })
}

async fn provider_entries(
    state: &ExtensionState,
    cwd: Option<&str>,
    user_args: &[String],
) -> Result<Vec<CatalogEntry>, String> {
    let providers = loaded_provider_commands(state).await?;
    let cwd = cwd.map(Path::new);
    let mut entries = Vec::new();
    for provider in providers.iter() {
        let descriptor = &provider.descriptor;
        let runtime_available = provider.runtime_available;
        let mut args = provider.configured_args.clone();
        args.extend_from_slice(user_args);
        let plan = execution_plan(descriptor, &provider.invocation, args, cwd)
            .and_then(|mut plan| {
                plan.user_args_start = plan.args.len().checked_sub(user_args.len());
                state.protect_execution_plan(plan)
            })
            .ok();
        entries.push(CatalogEntry {
            id: format!(
                "provider:{}:{}",
                provider.invocation.extension_id, descriptor.id
            ),
            command: descriptor.id.clone(),
            qualified_command: format!("{}:{}", provider.namespace, descriptor.id),
            namespace: provider.namespace.clone(),
            name: descriptor.name.clone(),
            description: descriptor.description.clone(),
            source_kind: CatalogSourceKind::Provider,
            source_name: provider.source_name.clone(),
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
) -> Result<Arc<Vec<LoadedProviderCommand>>, String> {
    // Fast path: a fresh entry answers with an Arc clone and never touches
    // the reload guard.
    {
        let cache = state.provider_commands.cached.lock().await;
        if let Some((created, commands)) = cache.as_ref() {
            if created.elapsed() <= PROVIDER_COMMAND_CACHE_TTL {
                return Ok(Arc::clone(commands));
            }
        }
    }
    // Singleflight: concurrent misses line up on the reload guard instead of
    // each performing its own full disk reload + parse pass.
    let _reload = state.provider_commands.reload.lock().await;
    // Re-check after acquiring the guard: another task may have refilled
    // the cache while we waited.
    let mut cache = state.provider_commands.cached.lock().await;
    if let Some((created, commands)) = cache.as_ref() {
        if created.elapsed() <= PROVIDER_COMMAND_CACHE_TTL {
            return Ok(Arc::clone(commands));
        }
    }
    let commands = Arc::new(load_provider_commands_uncached(state).await?);
    *cache = Some((Instant::now(), Arc::clone(&commands)));
    Ok(commands)
}

/// Test-only: drive the real cached load path so a test can warm the cache and
/// then assert that a binding change invalidated it. Returns the number of
/// loaded provider commands.
#[cfg(test)]
pub(crate) async fn load_cached_provider_commands_for_test(
    state: &ExtensionState,
) -> Result<usize, String> {
    Ok(loaded_provider_commands(state).await?.len())
}

pub(crate) async fn load_provider_commands_uncached(
    state: &ExtensionState,
) -> Result<Vec<LoadedProviderCommand>, String> {
    let mut lock = ExtensionsLock::load(&state.paths.repository_file)?;
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
            // Resolve the binding in one place: a changed fingerprint at the
            // same path is silently re-bound (after the refreshed provider is
            // validated), while a missing executable or a validation failure
            // stays on the broken path below.
            let binding_state = {
                let mut tool_lock = state
                    .tool_lock
                    .lock()
                    .map_err(|_| "Tool lock is unavailable".to_string())?;
                let snapshot = tool_lock.clone();
                // R11 · a script integration binds its **interpreter by name**:
                // the interpreter is re-resolved through the host search path on
                // every check, so an upgrade that rewrites the binary in place
                // never invalidates the binding. Every other runtime (a system
                // tool, a compiled artifact) keeps the frozen path + fingerprint
                // semantics.
                let interpreter =
                    crate::extensions::registry::entry_script_interpreter_language(&entry);
                let result = match interpreter {
                    Some(language) => crate::extensions::tool_lock::resolve_interpreter_binding(
                        &mut tool_lock,
                        &entry.id,
                        language.as_str(),
                        crate::extensions::registry::script_interpreter_is_available(language),
                        true,
                    ),
                    None => crate::extensions::tool_lock::resolve_executable_binding(
                        &mut tool_lock,
                        &entry.id,
                        &entry.executable_path,
                        || validate_refreshed_binding(&entry),
                    ),
                };
                match result {
                    Ok((binding_state, changed)) => {
                        if changed {
                            if let Err(error) = tool_lock.save(&state.paths.tool_lock_file) {
                                // Persisting the silent rebind failed: roll the
                                // in-memory lock back to the snapshot and fall
                                // back to the same per-entry degradation the
                                // old HEAD behavior used (mark broken and keep
                                // loading the rest of the catalog) instead of
                                // aborting the whole directory load.
                                *tool_lock = snapshot;
                                drop(tool_lock);
                                let code = crate::extensions::error_codes::ProviderErrorCode::BindingCheckFailed;
                                if !already_recorded_broken(
                                    lock.get(&entry.id)?,
                                    code.as_str(),
                                    &error,
                                ) && lock.mark_broken(&entry.id, code.as_str(), &error)?
                                {
                                    lock_changed = true;
                                }
                                continue;
                            }
                        }
                        binding_state
                    }
                    Err(error) => {
                        drop(tool_lock);
                        let code =
                            crate::extensions::error_codes::ProviderErrorCode::BindingCheckFailed;
                        if !already_recorded_broken(lock.get(&entry.id)?, code.as_str(), &error)
                            && lock.mark_broken(&entry.id, code.as_str(), &error)?
                        {
                            lock_changed = true;
                        }
                        continue;
                    }
                }
            };
            match binding_state {
                crate::extensions::tool_lock::LockState::Connected => {
                    // Clearing the persisted failure is itself gated on the
                    // failure's cause being gone: a `broken` entry whose
                    // descriptor no longer parses must not flip back to
                    // enabled for one load and lose its recorded reason.
                    if already_broken
                        && crate::extensions::runtime_binding::recovery_is_proven(&entry)
                        && lock.clear_broken(&entry.id)?
                    {
                        lock_changed = true;
                    }
                }
                crate::extensions::tool_lock::LockState::ReconnectRequired
                | crate::extensions::tool_lock::LockState::ReverifyRequired => {
                    let code = match binding_state {
                        crate::extensions::tool_lock::LockState::ReconnectRequired => {
                            crate::extensions::error_codes::ProviderErrorCode::BindingMissing
                        }
                        crate::extensions::tool_lock::LockState::ReverifyRequired => {
                            crate::extensions::error_codes::ProviderErrorCode::BindingChanged
                        }
                        crate::extensions::tool_lock::LockState::Connected => unreachable!(),
                    };
                    let detail = match binding_state {
                        crate::extensions::tool_lock::LockState::ReconnectRequired => {
                            crate::extensions::runtime_binding::binding_missing_detail(
                                &entry.executable_path,
                            )
                        }
                        crate::extensions::tool_lock::LockState::ReverifyRequired => {
                            crate::extensions::runtime_binding::binding_changed_detail(
                                &entry.executable_path,
                            )
                        }
                        crate::extensions::tool_lock::LockState::Connected => unreachable!(),
                    };
                    if !already_recorded_broken(lock.get(&entry.id)?, code.as_str(), &detail)
                        && lock.mark_broken(&entry.id, code.as_str(), &detail)?
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
                Err(error) => {
                    // A descriptor that will not load is not a transient
                    // catalog miss: it is recorded in the repository so the
                    // row can say *why* it is unavailable. Before G5 this was
                    // an `eprintln!` and the row kept claiming `Ready`.
                    let code = crate::extensions::runtime_binding::failure_code(&error);
                    if !already_recorded_broken(lock.get(&entry.id)?, code, &error)
                        && lock.mark_broken(&entry.id, code, &error)?
                    {
                        lock_changed = true;
                    }
                    tracing::warn!(
                        extension_id = %entry.id,
                        error = %error,
                        "cannot load static integration for the command catalog"
                    );
                }
            }
            continue;
        }
        let mut invocation = match crate::extensions::registry::provider_invocation(&entry) {
            Ok(invocation) => invocation,
            Err(error) => {
                // Same rule as the static branch: a manifest that will not
                // resolve is recorded, not merely logged.
                let code = crate::extensions::runtime_binding::failure_code(&error);
                if !already_recorded_broken(lock.get(&entry.id)?, code, &error)
                    && lock.mark_broken(&entry.id, code, &error)?
                {
                    lock_changed = true;
                }
                tracing::warn!(
                    extension_id = %entry.id,
                    error = %error,
                    "cannot load extension for the command catalog"
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
                let code_str = crate::extensions::runtime_binding::failure_code(&error);
                if !already_recorded_broken(lock.get(&entry.id)?, code_str, &error)
                    && lock.mark_broken(&entry.id, code_str, &error)?
                {
                    lock_changed = true;
                }
                tracing::warn!(
                    extension_id = %entry.id,
                    error = %error,
                    "cannot describe extension for the command catalog"
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
        if let Err(error) = lock.save(&state.paths.repository_file) {
            tracing::warn!(error = %error, "cannot persist extension binding state");
        }
    }
    Ok(result)
}

/// Rebuild the provider description for an entry whose executable fingerprint
/// changed at the same path, purely to prove the refreshed tool is still a
/// valid provider before the binding is silently re-pointed at it.
///
/// The coverage is deliberately asymmetric:
///
/// * A `StaticDescriptor`/`BundledStatic` entry is fully re-validated here
///   (`static_description` re-parses the descriptor, checks the provider id and
///   runs `validate_execution_descriptors`).
/// * An `Executable` entry is only checked *structurally* via
///   `provider_invocation` (manifest identity + platform resolution). This
///   function never launches or `describe`s the refreshed binary.
///
/// For executable providers the real check therefore happens later in the
/// catalog load loop, where a failed `describe` marks the entry broken. The
/// list path (`commands::extensions`) reuses this validator but has no
/// `describe` fallback, so a dynamic binding can read as `Connected` there
/// without the new binary ever having been described. This is not a bypass on
/// the catalog path, but callers on the list path must not assume more than
/// the structural guarantee above.
pub(crate) fn validate_refreshed_binding(
    entry: &crate::extensions::lock::ExtensionLockEntry,
) -> Result<(), String> {
    let manifest = crate::extensions::manifest::ExtensionManifest::load(std::path::Path::new(
        &entry.manifest_path,
    ))?;
    validate_refreshed_binding_with_manifest(entry, &manifest)
}

/// [`validate_refreshed_binding`] reusing an already-parsed manifest, so the
/// list path can validate and then keep using the same parse instead of reading
/// the manifest a second time.
pub(crate) fn validate_refreshed_binding_with_manifest(
    entry: &crate::extensions::lock::ExtensionLockEntry,
    manifest: &crate::extensions::manifest::ExtensionManifest,
) -> Result<(), String> {
    if matches!(
        entry.provider_kind,
        ExtensionProviderKind::StaticDescriptor | ExtensionProviderKind::BundledStatic
    ) {
        crate::extensions::registry::static_description_with_manifest(entry, manifest).map(|_| ())
    } else {
        crate::extensions::registry::provider_invocation_with_manifest(entry, manifest).map(|_| ())
    }
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

/// The three-tier score ladder shared with the frontend
/// (`src/command-aliases.ts`): exact > prefix > contains, with the display name
/// two tiers below. Kept as named constants so a tier cannot be nudged without
/// the round's tests noticing.
const SCORE_EXACT: u32 = 1_000;
const SCORE_PREFIX: u32 = 800;
const SCORE_CONTAINS: u32 = 600;
const SCORE_NAME_PREFIX: u32 = 500;
const SCORE_NAME_CONTAINS: u32 = 300;

/// The tier of one already-lowercased candidate string against `needle`.
fn candidate_score(needle: &str, candidate: &str) -> u32 {
    if needle.is_empty() || candidate.is_empty() || candidate.len() < needle.len() {
        return 0;
    }
    if candidate == needle {
        return SCORE_EXACT;
    }
    if candidate.starts_with(needle) {
        return SCORE_PREFIX - (candidate.len() - needle.len()).min(100) as u32;
    }
    if candidate.contains(needle) {
        return SCORE_CONTAINS;
    }
    0
}

fn score_entry(
    entry: &CatalogEntry,
    needle: &str,
    command_aliases: &HashMap<String, String>,
) -> Option<u32> {
    if needle.is_empty() {
        return Some(1);
    }
    let command = entry.command.to_ascii_lowercase();
    let mut best = candidate_score(needle, &command);
    // R7-11: the user's alias for this command is a candidate string at the
    // *same* tiers as the command name, so an alias exact hit scores exactly
    // what a command-name exact hit scores. `max` is the contract.
    if let Some(alias) = command_aliases.get(&entry.command) {
        best = best.max(candidate_score(needle, &alias.to_ascii_lowercase()));
    }
    if best > 0 {
        return Some(best);
    }
    let name = entry.name.to_ascii_lowercase();
    if name.starts_with(needle) {
        return Some(SCORE_NAME_PREFIX);
    }
    if name.contains(needle)
        || entry
            .aliases
            .iter()
            .any(|alias| alias.to_ascii_lowercase().contains(needle))
    {
        return Some(SCORE_NAME_CONTAINS);
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
            probe_report: None,
            config_generation: 0,
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
        assert_eq!(score_entry(&entry, "jv", &HashMap::new()), Some(1_000));
        entry.command = "other".into();
        assert_eq!(score_entry(&entry, "json", &HashMap::new()), Some(500));
    }

    /// R7-11: an alias exact hit scores exactly what a command-name exact hit
    /// scores — the same tier, not a tier below it. Without the `max` in
    /// `score_entry` the alias would fall through to the name branch and the
    /// row would rank under an unrelated command-name exact match.
    #[test]
    fn alias_exact_shares_the_command_exact_tier() {
        let entry = CatalogEntry {
            id: "provider:x:git".into(),
            command: "git".into(),
            namespace: "x".into(),
            qualified_command: "x:git".into(),
            name: "Git".into(),
            description: String::new(),
            source_kind: CatalogSourceKind::Provider,
            source_name: "x".into(),
            aliases: Vec::new(),
            arguments: Vec::new(),
            execution: None,
            runtime_available: true,
            frequency: 0,
        };
        let aliases = HashMap::from([("git".to_string(), "gfm".to_string())]);
        // Alias exact == command exact.
        assert_eq!(score_entry(&entry, "gfm", &aliases), Some(SCORE_EXACT));
        assert_eq!(score_entry(&entry, "git", &aliases), Some(SCORE_EXACT));
        // Alias prefix sits in the prefix tier, above any name-only match.
        assert_eq!(score_entry(&entry, "gf", &aliases), Some(SCORE_PREFIX - 1));
        // A stale alias key does not score anything new.
        assert_eq!(score_entry(&entry, "gfm", &HashMap::new()), None);
    }

    /// R7-11 end-to-end: the alias actually reaches a real `search` call and
    /// comes back as a row. A local command is the cheapest fixture; the point
    /// is that `search` filters entries *by* `score_entry`, so an alias that
    /// only existed in the settings map would be dropped before ranking.
    #[tokio::test]
    async fn search_surfaces_a_local_command_through_its_alias() {
        let directory = tempfile::tempdir().unwrap();
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        std::fs::write(
            state.paths.root.join("local-commands.json"),
            r#"[{
                "id": "git-commit",
                "command": "git",
                "name": "Git",
                "description": "Version control",
                "program": "git",
                "argsPrefix": ["commit", "-m"]
            }]"#,
        )
        .unwrap();

        let request = |needle: &str, aliases: HashMap<String, String>| CatalogSearchRequest {
            query: needle.into(),
            tokens: vec![needle.into()],
            environment: BTreeMap::new(),
            cwd: None,
            limit: 10,
            include_system_commands: false,
            command_aliases: aliases,
        };

        // With the alias the user story describes, `gfm` finds `git`.
        let by_alias = search(
            &state,
            &request("gfm", HashMap::from([("git".into(), "gfm".into())])),
            &[],
        )
        .await
        .unwrap();
        assert_eq!(by_alias.len(), 1);
        assert_eq!(by_alias[0].command, "git");

        // Without it, the same query finds nothing: the match is the alias, not
        // some incidental subsequence of the name.
        let without = search(&state, &request("gfm", HashMap::new()), &[])
            .await
            .unwrap();
        assert!(without.is_empty());

        // An alias exact hit outranks another command's prefix hit.
        std::fs::write(
            state.paths.root.join("local-commands.json"),
            r#"[
                {"id":"git-commit","command":"git","name":"Git","program":"git"},
                {"id":"gfm-tool","command":"gfm-tool","name":"GFM Tool","program":"gfm-tool"}
            ]"#,
        )
        .unwrap();
        let ranked = search(
            &state,
            &request("gfm", HashMap::from([("git".into(), "gfm".into())])),
            &[],
        )
        .await
        .unwrap();
        assert_eq!(ranked.len(), 2);
        assert_eq!(ranked[0].command, "git", "alias exact must rank first");
        assert_eq!(ranked[1].command, "gfm-tool");
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
            probe_report: None,
            config_generation: 0,
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
        lock.save(&state.paths.repository_file).unwrap();

        // First load binds the executable and exposes the provider commands.
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert_eq!(commands.len(), 5);

        // Removing the bound executable marks the entry broken and hides it.
        std::fs::remove_file(&executable).unwrap();
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert!(commands.is_empty());
        let broken = ExtensionsLock::load(&state.paths.repository_file)
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
        // R11 · the recorded reason is the **keyed** detail
        // (`binding-missing:{"path":…}`): the backend owns the fact, the
        // frontend owns the words, so the row never prints a raw English
        // "no longer available" sentence beside an untranslated code.
        let reason = broken.broken_reason.as_deref().unwrap();
        assert!(
            reason.starts_with("binding-missing:") && reason.contains("/v"),
            "the recorded reason must name the missing executable: {reason}"
        );

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
        let restored = ExtensionsLock::load(&state.paths.repository_file)
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
        lock.save(&state.paths.repository_file).unwrap();
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
        let restored = ExtensionsLock::load(&state.paths.repository_file)
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

    #[cfg(unix)]
    fn write_executable(path: &std::path::Path, contents: &str) {
        use std::os::unix::fs::PermissionsExt;
        std::fs::write(path, contents).unwrap();
        std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o755)).unwrap();
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn fingerprint_change_at_the_same_path_is_rebound_and_stays_connected() {
        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");

        let root = directory.path().join("integration");
        let entry = recommended_local_entry(&root, &executable);
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();

        // First load binds the executable and exposes the provider commands.
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert_eq!(commands.len(), 5);

        // The provider is rebuilt in place (a version upgrade): same path, new
        // bytes and mtime. The binding is silently refreshed instead of the
        // entry being marked broken, and the commands keep loading.
        write_executable(&executable, "#!/bin/sh\nprintf rebuilt\n");
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert_eq!(commands.len(), 5);

        let stored = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get(&entry.id)
            .unwrap()
            .clone();
        assert_eq!(
            stored.state,
            crate::extensions::lock::ExtensionStateKind::Enabled
        );
        assert!(stored.enabled);
        assert_eq!(stored.last_error_code, None);
        assert_eq!(stored.broken_reason, None);

        // Both sources of truth agree: the persisted binding points at the new
        // fingerprint and reads back Connected.
        assert_eq!(
            state
                .check_executable_binding(&entry.id, &entry.executable_path)
                .unwrap(),
            crate::extensions::tool_lock::LockState::Connected
        );
        let binding = state.tool_lock.lock().unwrap();
        assert_eq!(
            binding.tools[&entry.id].state,
            crate::extensions::tool_lock::LockState::Connected
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn fingerprint_change_with_an_invalid_descriptor_falls_back_to_broken() {
        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");

        let root = directory.path().join("integration");
        let entry = recommended_local_entry(&root, &executable);
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
        let _ = load_provider_commands_uncached(&state).await.unwrap();

        // The refreshed executable is paired with a descriptor that no longer
        // validates: the binding must NOT be silently refreshed, and the entry
        // falls back to the original broken behavior.
        std::fs::write(root.join("provider-description.json"), b"{ not json").unwrap();
        write_executable(&executable, "#!/bin/sh\nprintf rebuilt\n");
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert!(commands.is_empty());

        let stored = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get(&entry.id)
            .unwrap()
            .clone();
        assert_eq!(
            stored.state,
            crate::extensions::lock::ExtensionStateKind::Broken
        );
        assert!(!stored.enabled);
        assert_eq!(stored.last_error_code.as_deref(), Some("binding-changed"));

        // The rejected fingerprint was not persisted: the binding still points
        // at the tool the user originally approved.
        let binding = state.tool_lock.lock().unwrap();
        assert_eq!(
            binding.tools[&entry.id].state,
            crate::extensions::tool_lock::LockState::ReverifyRequired
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn a_missing_executable_keeps_the_broken_state() {
        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        write_executable(&executable, "#!/bin/sh\nexit 0\n");

        let root = directory.path().join("integration");
        let entry = recommended_local_entry(&root, &executable);
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
        let _ = load_provider_commands_uncached(&state).await.unwrap();

        // Removing the executable is not a fingerprint change: it still
        // demands a reconnect and stays broken.
        std::fs::remove_file(&executable).unwrap();
        let commands = load_provider_commands_uncached(&state).await.unwrap();
        assert!(commands.is_empty());
        let stored = ExtensionsLock::load(&state.paths.repository_file)
            .unwrap()
            .get(&entry.id)
            .unwrap()
            .clone();
        assert_eq!(
            stored.state,
            crate::extensions::lock::ExtensionStateKind::Broken
        );
        assert_eq!(stored.last_error_code.as_deref(), Some("binding-missing"));
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

    #[cfg(unix)]
    fn cached_provider_state(
        directory: &tempfile::TempDir,
    ) -> (ExtensionState, crate::extensions::lock::ExtensionLockEntry) {
        use std::os::unix::fs::PermissionsExt;

        let executable = directory.path().join("v");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let root = directory.path().join("integration");
        let entry = recommended_local_entry(&root, &executable);
        let state = ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(
            directory.path().join("config"),
        ))
        .unwrap();
        let mut lock = ExtensionsLock::default();
        lock.extensions.insert(entry.id.clone(), entry.clone());
        lock.save(&state.paths.repository_file).unwrap();
        (state, entry)
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn fresh_cache_hits_share_one_arc() {
        let directory = tempfile::tempdir().unwrap();
        let (state, _entry) = cached_provider_state(&directory);

        let first = loaded_provider_commands(&state).await.unwrap();
        assert_eq!(first.len(), 5);
        let second = loaded_provider_commands(&state).await.unwrap();

        assert!(Arc::ptr_eq(&first, &second));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn expired_entry_reloads_after_underlying_data_changes() {
        let directory = tempfile::tempdir().unwrap();
        let (state, entry) = cached_provider_state(&directory);

        let first = loaded_provider_commands(&state).await.unwrap();
        assert_eq!(first.len(), 5);

        // Change the underlying data: disable the integration so a reload
        // must produce a different table.
        let mut lock = ExtensionsLock::load(&state.paths.repository_file).unwrap();
        lock.extensions.get_mut(&entry.id).unwrap().enabled = false;
        lock.save(&state.paths.repository_file).unwrap();

        // Still fresh: the stale table is served (TTL is only a safety net).
        let cached = loaded_provider_commands(&state).await.unwrap();
        assert!(Arc::ptr_eq(&first, &cached));

        // Age the stored entry past the TTL; the next call must reload.
        state
            .provider_commands
            .age_entry_for_test(PROVIDER_COMMAND_CACHE_TTL + Duration::from_secs(1))
            .await;
        let reloaded = loaded_provider_commands(&state).await.unwrap();
        assert!(!Arc::ptr_eq(&first, &reloaded));
        assert!(reloaded.is_empty());
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn concurrent_misses_share_a_single_reload() {
        let directory = tempfile::tempdir().unwrap();
        let (state, _entry) = cached_provider_state(&directory);
        let state = Arc::new(state);

        // All 8 tasks race against a cold cache. Singleflight must serialize
        // the reload so every task observes the exact same shared Arc; without
        // the reload guard each task would get its own freshly loaded table.
        let mut handles = Vec::new();
        for _ in 0..8 {
            let state = Arc::clone(&state);
            handles.push(tokio::spawn(async move {
                loaded_provider_commands(&state).await.unwrap()
            }));
        }
        let mut results = Vec::with_capacity(handles.len());
        for handle in handles {
            results.push(handle.await.unwrap());
        }

        for commands in &results {
            assert_eq!(commands.len(), 5);
            assert!(Arc::ptr_eq(commands, &results[0]));
        }
    }
}
