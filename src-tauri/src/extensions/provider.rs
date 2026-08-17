use crate::extensions::manifest::{Permission, ProviderConfig};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashMap};
use std::path::{Component, Path, PathBuf};
use std::process::Stdio;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant, UNIX_EPOCH};
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWriteExt};

const DESCRIPTION_SCHEMA: &str =
    include_str!("../../../docs/extensions/schemas/provider-description.schema.json");
const MAX_STDOUT_BYTES: usize = 1024 * 1024;
const MAX_STDERR_BYTES: usize = 256 * 1024;

#[derive(Debug, Clone)]
pub struct ProviderInvocation {
    pub extension_id: String,
    pub executable: PathBuf,
    /// Arguments placed before the provider protocol arguments. Script
    /// integrations use this for the interpreter and script path.
    pub executable_prefix: Vec<String>,
    pub runtime_root: Option<PathBuf>,
    pub package_version: String,
    pub tool_version_hint: Option<String>,
    pub version_args: Vec<String>,
    pub config: ProviderConfig,
    pub permissions: Vec<Permission>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderDescription {
    pub protocol_version: String,
    pub provider: ProviderIdentity,
    pub commands: Vec<CommandDescriptor>,
}

impl ProviderDescription {
    pub fn parse(bytes: &[u8]) -> Result<Self, String> {
        let value: Value = serde_json::from_slice(bytes)
            .map_err(|error| format!("Invalid provider description JSON: {error}"))?;
        Self::from_value(value)
    }

    fn from_value(value: Value) -> Result<Self, String> {
        validate_description_schema(&value)?;
        serde_json::from_value(value)
            .map_err(|error| format!("Invalid provider description: {error}"))
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderIdentity {
    pub id: String,
    pub name: String,
    pub version: String,
    #[serde(default)]
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommandDescriptor {
    pub id: String,
    pub name: String,
    pub description: String,
    #[serde(default)]
    pub aliases: Vec<String>,
    #[serde(default)]
    pub keywords: Vec<String>,
    pub execution: ExecutionDescriptor,
    #[serde(default)]
    pub arguments: Vec<ArgumentDescriptor>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExecutionDescriptor {
    #[serde(default = "self_program")]
    pub program: String,
    pub args_prefix: Vec<String>,
    pub mode: ExecutionMode,
    #[serde(default)]
    pub working_directory: WorkingDirectory,
}

fn self_program() -> String {
    "self".to_string()
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ExecutionMode {
    Pty,
    Capture,
    External,
}

impl ExecutionMode {
    pub fn host_mode(self) -> Self {
        match self {
            // Draft 1 advertised capture before the Host had a separate captured-output
            // surface. Keep old manifests compatible without leaking a false mode to IPC.
            Self::Capture => Self::Pty,
            mode => mode,
        }
    }
}

#[derive(Debug, Clone, Copy, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum WorkingDirectory {
    #[default]
    Current,
    Home,
    Inherit,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ArgumentDescriptor {
    pub names: Vec<String>,
    pub kind: ArgumentKind,
    pub description: String,
    #[serde(default)]
    pub takes_value: bool,
    #[serde(default)]
    pub required: bool,
    #[serde(default)]
    pub repeatable: bool,
    #[serde(default)]
    pub values: Vec<String>,
    pub value_hint: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ArgumentKind {
    Flag,
    String,
    Integer,
    Number,
    Path,
    Directory,
    Url,
    Enum,
    Command,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ExecutionPlan {
    pub program: String,
    pub args: Vec<String>,
    pub mode: ExecutionMode,
    pub cwd: Option<String>,
    pub environment: BTreeMap<String, String>,
    pub inherit_environment: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub plan_token: Option<String>,
    #[serde(skip)]
    pub user_args_start: Option<usize>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProviderResponse {
    pub description: ProviderDescription,
    pub runtime_available: bool,
    pub cached: bool,
    pub stderr: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ProviderCache {
    executable: String,
    modified_ms: u128,
    package_version: String,
    tool_version: String,
    description: ProviderDescription,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CompletionItem {
    pub value: String,
    pub label: String,
    #[serde(default)]
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderCompletion {
    pub completions: Vec<ProviderCompletionItem>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ProviderCompletionItem {
    pub label: String,
    pub kind: String,
    pub detail: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiagnoseResponse {
    pub status: String,
    #[serde(default)]
    pub checks: Vec<DiagnoseCheck>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiagnoseCheck {
    pub id: String,
    pub status: String,
    pub message: String,
}

pub struct ProviderManager {
    cache_dir: PathBuf,
    completion_generation: AtomicU64,
    completion_cache: tokio::sync::Mutex<HashMap<String, (Instant, ProviderCompletion)>>,
}

impl ProviderManager {
    pub fn new(cache_dir: PathBuf) -> Self {
        Self {
            cache_dir,
            completion_generation: AtomicU64::new(0),
            completion_cache: tokio::sync::Mutex::new(HashMap::new()),
        }
    }

    pub fn cancel_completions(&self) {
        self.completion_generation.fetch_add(1, Ordering::SeqCst);
    }

    pub async fn describe(
        &self,
        invocation: &ProviderInvocation,
        force: bool,
    ) -> Result<ProviderResponse, String> {
        let cached = self.read_cache(&invocation.extension_id).ok();
        if !invocation.executable.is_file() {
            return cached
                .map(|cache| ProviderResponse {
                    description: cache.description,
                    runtime_available: false,
                    cached: true,
                    stderr: Some(format!(
                        "Provider executable is unavailable: {}",
                        invocation.executable.display()
                    )),
                })
                .ok_or_else(|| {
                    format!(
                        "Provider executable is unavailable: {}",
                        invocation.executable.display()
                    )
                });
        }

        let modified_ms = invocation_modified_ms(invocation)?;
        let tool_version = if invocation.version_args.is_empty() {
            invocation.tool_version_hint.clone()
        } else {
            provider_version(invocation)
                .await
                .or_else(|| invocation.tool_version_hint.clone())
        };
        if !force {
            if let Some(cache) = cached.as_ref() {
                let tool_matches = tool_version
                    .as_ref()
                    .is_none_or(|version| version == &cache.tool_version);
                if cache.executable == invocation.executable.to_string_lossy()
                    && cache.modified_ms == modified_ms
                    && cache.package_version == invocation.package_version
                    && tool_matches
                {
                    return Ok(ProviderResponse {
                        description: cache.description.clone(),
                        runtime_available: true,
                        cached: true,
                        stderr: None,
                    });
                }
            }
        }

        match self
            .call::<Value>(
                invocation,
                "describe",
                None,
                Duration::from_millis(invocation.config.describe_timeout_ms.min(5_000)),
            )
            .await
        {
            Ok((description_value, stderr)) => {
                let description = ProviderDescription::from_value(description_value)?;
                if description.provider.id != invocation.extension_id {
                    return Err(format!(
                        "Provider id {} does not match extension id {}",
                        description.provider.id, invocation.extension_id
                    ));
                }
                validate_execution_descriptors(&description, invocation)?;
                let cache = ProviderCache {
                    executable: invocation.executable.to_string_lossy().into_owned(),
                    modified_ms,
                    package_version: invocation.package_version.clone(),
                    tool_version: tool_version
                        .unwrap_or_else(|| description.provider.version.clone()),
                    description: description.clone(),
                };
                self.write_cache(&invocation.extension_id, &cache)?;
                Ok(ProviderResponse {
                    description,
                    runtime_available: true,
                    cached: false,
                    stderr: (!stderr.is_empty()).then_some(stderr),
                })
            }
            Err(error) => cached
                .map(|cache| ProviderResponse {
                    description: cache.description,
                    runtime_available: false,
                    cached: true,
                    stderr: Some(error.clone()),
                })
                .ok_or(error),
        }
    }

    pub async fn complete(
        &self,
        invocation: &ProviderInvocation,
        request: &Value,
    ) -> Result<ProviderCompletion, String> {
        let generation = self.completion_generation.fetch_add(1, Ordering::SeqCst) + 1;
        let cache_key = format!(
            "{}:{}",
            invocation.extension_id,
            serde_json::to_string(request)
                .map_err(|error| format!("Cannot serialize completion cache key: {error}"))?
        );
        {
            let cache = self.completion_cache.lock().await;
            if let Some((created, response)) = cache.get(&cache_key) {
                if created.elapsed() <= Duration::from_secs(2) {
                    return Ok(response.clone());
                }
            }
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
        if self.completion_generation.load(Ordering::SeqCst) != generation {
            return Err("Provider completion was superseded by a newer request".to_string());
        }
        let call = self.call::<ProviderCompletion>(
            invocation,
            "complete",
            Some(request),
            Duration::from_millis(invocation.config.complete_timeout_ms),
        );
        tokio::pin!(call);
        let response = loop {
            tokio::select! {
                response = &mut call => break response.map(|(response, _)| response)?,
                _ = tokio::time::sleep(Duration::from_millis(20)) => {
                    if self.completion_generation.load(Ordering::SeqCst) != generation {
                        return Err("Provider completion was superseded by a newer request".to_string());
                    }
                }
            }
        };
        let mut cache = self.completion_cache.lock().await;
        cache.retain(|_, (created, _)| created.elapsed() <= Duration::from_secs(2));
        cache.insert(cache_key, (Instant::now(), response.clone()));
        Ok(response)
    }

    pub async fn diagnose(
        &self,
        invocation: &ProviderInvocation,
    ) -> Result<DiagnoseResponse, String> {
        self.call(invocation, "diagnose", None, Duration::from_secs(5))
            .await
            .map(|(response, _)| response)
    }

    pub async fn call_config<T: DeserializeOwned>(
        &self,
        invocation: &ProviderInvocation,
    ) -> Result<T, String> {
        self.call(invocation, "config", None, Duration::from_secs(5))
            .await
            .map(|(response, _)| response)
    }

    async fn call<T: DeserializeOwned>(
        &self,
        invocation: &ProviderInvocation,
        operation: &str,
        input: Option<&Value>,
        timeout: Duration,
    ) -> Result<(T, String), String> {
        let mut command = provider_command(&invocation.executable);
        if !invocation.permissions.contains(&Permission::Environment) {
            command.env_clear();
        }
        command
            .args(&invocation.executable_prefix)
            .args(&invocation.config.args_prefix)
            .arg(operation)
            .args(["--protocol", "1"])
            .envs(&invocation.config.environment)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .kill_on_drop(true);
        let mut child = command.spawn().map_err(|error| {
            format!(
                "Cannot start provider {}: {error}",
                invocation.executable.display()
            )
        })?;
        if let Some(input) = input {
            let bytes = serde_json::to_vec(input)
                .map_err(|error| format!("Cannot serialize provider request: {error}"))?;
            if let Some(mut stdin) = child.stdin.take() {
                stdin
                    .write_all(&bytes)
                    .await
                    .map_err(|error| format!("Cannot write provider stdin: {error}"))?;
            }
        } else {
            drop(child.stdin.take());
        }
        let stdout = child
            .stdout
            .take()
            .ok_or("Provider stdout is unavailable")?;
        let stderr = child
            .stderr
            .take()
            .ok_or("Provider stderr is unavailable")?;
        let stdout_task = tokio::spawn(read_limited(stdout, MAX_STDOUT_BYTES, "stdout"));
        let stderr_task = tokio::spawn(read_limited(stderr, MAX_STDERR_BYTES, "stderr"));

        let status = match tokio::time::timeout(timeout, child.wait()).await {
            Ok(result) => result.map_err(|error| format!("Cannot wait for provider: {error}"))?,
            Err(_) => {
                let _ = child.kill().await;
                let _ = child.wait().await;
                return Err(format!(
                    "Provider {operation} timed out after {} ms",
                    timeout.as_millis()
                ));
            }
        };
        let stdout = stdout_task
            .await
            .map_err(|error| format!("Provider stdout task failed: {error}"))??;
        let stderr = stderr_task
            .await
            .map_err(|error| format!("Provider stderr task failed: {error}"))??;
        let stderr = String::from_utf8(stderr)
            .map_err(|_| "Provider stderr is not valid UTF-8".to_string())?;
        if !status.success() {
            let code = status
                .code()
                .map_or("signal".to_string(), |code| code.to_string());
            return Err(format!(
                "Provider {operation} exited with {code}: {}",
                stderr.trim()
            ));
        }
        let response = serde_json::from_slice(&stdout)
            .map_err(|error| format!("Provider {operation} returned invalid JSON: {error}"))?;
        Ok((response, stderr))
    }

    fn cache_path(&self, extension_id: &str) -> Result<PathBuf, String> {
        if extension_id.is_empty()
            || !extension_id
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || b"._-".contains(&byte))
        {
            return Err("Invalid extension id for provider cache".to_string());
        }
        Ok(self.cache_dir.join(format!("{extension_id}.json")))
    }

    fn read_cache(&self, extension_id: &str) -> Result<ProviderCache, String> {
        let path = self.cache_path(extension_id)?;
        let bytes = std::fs::read(&path)
            .map_err(|error| format!("Cannot read provider cache {}: {error}", path.display()))?;
        serde_json::from_slice(&bytes)
            .map_err(|error| format!("Invalid provider cache {}: {error}", path.display()))
    }

    fn write_cache(&self, extension_id: &str, cache: &ProviderCache) -> Result<(), String> {
        std::fs::create_dir_all(&self.cache_dir).map_err(|error| {
            format!(
                "Cannot create provider cache {}: {error}",
                self.cache_dir.display()
            )
        })?;
        let path = self.cache_path(extension_id)?;
        let bytes = serde_json::to_vec_pretty(cache)
            .map_err(|error| format!("Cannot serialize provider cache: {error}"))?;
        atomic_write(&path, &bytes)
    }
}

pub fn execution_plan(
    command: &CommandDescriptor,
    invocation: &ProviderInvocation,
    user_args: Vec<String>,
    cwd: Option<&Path>,
) -> Result<ExecutionPlan, String> {
    let program = if command.execution.program == "self" {
        invocation.executable.clone()
    } else {
        if !invocation.permissions.contains(&Permission::ProcessSpawn) {
            return Err(format!(
                "Command {} requires the process-spawn permission",
                command.id
            ));
        }
        let relative = Path::new(&command.execution.program);
        if relative.is_absolute()
            || relative
                .components()
                .any(|component| !matches!(component, Component::Normal(_)))
        {
            return Err(format!(
                "Command {} has an unsafe execution program",
                command.id
            ));
        }
        invocation
            .runtime_root
            .as_ref()
            .ok_or_else(|| "Linked providers may only execute self".to_string())?
            .join(relative)
    };
    if !program.is_file() {
        return Err(format!(
            "Execution program does not exist: {}",
            program.display()
        ));
    }
    let self_program = command.execution.program == "self";
    let (program, mut args) = execution_host(program);
    if self_program {
        args.extend(invocation.executable_prefix.clone());
    }
    args.extend(command.execution.args_prefix.clone());
    args.extend(user_args);
    let cwd = match command.execution.working_directory {
        WorkingDirectory::Current => cwd.map(Path::to_path_buf),
        WorkingDirectory::Home => dirs::home_dir(),
        WorkingDirectory::Inherit => None,
    };
    Ok(ExecutionPlan {
        program: program.to_string_lossy().into_owned(),
        args,
        mode: command.execution.mode.host_mode(),
        cwd: cwd.map(|path| path.to_string_lossy().into_owned()),
        environment: invocation.config.environment.clone(),
        inherit_environment: invocation.permissions.contains(&Permission::Environment),
        plan_token: None,
        user_args_start: None,
    })
}

fn execution_host(program: PathBuf) -> (PathBuf, Vec<String>) {
    #[cfg(target_os = "windows")]
    {
        let extension = program
            .extension()
            .and_then(|value| value.to_str())
            .unwrap_or_default();
        if extension.eq_ignore_ascii_case("cmd") || extension.eq_ignore_ascii_case("bat") {
            return (
                PathBuf::from("cmd.exe"),
                vec![
                    "/D".into(),
                    "/S".into(),
                    "/C".into(),
                    program.to_string_lossy().into_owned(),
                ],
            );
        }
    }
    (program, Vec::new())
}

fn provider_command(executable: &Path) -> tokio::process::Command {
    #[cfg(target_os = "windows")]
    {
        let extension = executable
            .extension()
            .and_then(|value| value.to_str())
            .unwrap_or_default();
        if extension.eq_ignore_ascii_case("cmd") || extension.eq_ignore_ascii_case("bat") {
            let mut command = tokio::process::Command::new("cmd.exe");
            command.args(["/D", "/S", "/C"]).arg(executable);
            return command;
        }
    }
    tokio::process::Command::new(executable)
}

async fn provider_version(invocation: &ProviderInvocation) -> Option<String> {
    let mut command = provider_command(&invocation.executable);
    if !invocation.permissions.contains(&Permission::Environment) {
        command.env_clear();
    }
    command
        .args(&invocation.executable_prefix)
        .args(&invocation.version_args)
        .envs(&invocation.config.environment)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .kill_on_drop(true);
    let output = tokio::time::timeout(Duration::from_secs(2), command.output())
        .await
        .ok()?
        .ok()?;
    if !output.status.success() || output.stdout.len() > 16 * 1024 {
        return None;
    }
    String::from_utf8(output.stdout)
        .ok()
        .map(|value| value.trim().chars().take(200).collect())
}

async fn read_limited<R: AsyncRead + Unpin>(
    mut reader: R,
    limit: usize,
    stream: &'static str,
) -> Result<Vec<u8>, String> {
    let mut result = Vec::new();
    let mut buffer = [0_u8; 8192];
    loop {
        let read = reader
            .read(&mut buffer)
            .await
            .map_err(|error| format!("Cannot read provider {stream}: {error}"))?;
        if read == 0 {
            return Ok(result);
        }
        if result.len() + read > limit {
            return Err(format!("Provider {stream} exceeded {limit} bytes"));
        }
        result.extend_from_slice(&buffer[..read]);
    }
}

fn validate_description_schema(instance: &Value) -> Result<(), String> {
    let schema: Value = serde_json::from_str(DESCRIPTION_SCHEMA)
        .map_err(|error| format!("Bundled provider schema is invalid: {error}"))?;
    let validator = jsonschema::validator_for(&schema)
        .map_err(|error| format!("Cannot compile provider schema: {error}"))?;
    let errors = validator
        .iter_errors(instance)
        .map(|error| error.to_string())
        .collect::<Vec<_>>();
    if errors.is_empty() {
        Ok(())
    } else {
        Err(format!(
            "Provider schema validation failed: {}",
            errors.join("; ")
        ))
    }
}

pub(crate) fn validate_execution_descriptors(
    description: &ProviderDescription,
    invocation: &ProviderInvocation,
) -> Result<(), String> {
    for command in &description.commands {
        if command.execution.program != "self" {
            if !invocation.permissions.contains(&Permission::ProcessSpawn) {
                return Err(format!(
                    "Command {} requires the process-spawn permission",
                    command.id
                ));
            }
            let path = Path::new(&command.execution.program);
            if path.is_absolute()
                || path
                    .components()
                    .any(|component| !matches!(component, Component::Normal(_)))
            {
                return Err(format!("Command {} has an unsafe program path", command.id));
            }
            let Some(runtime_root) = &invocation.runtime_root else {
                return Err(format!(
                    "Linked command {} may only execute self",
                    command.id
                ));
            };
            if !runtime_root.join(path).is_file() {
                return Err(format!("Command {} program does not exist", command.id));
            }
        }
    }
    Ok(())
}

fn modified_ms(path: &Path) -> Result<u128, String> {
    let modified = path
        .metadata()
        .and_then(|metadata| metadata.modified())
        .map_err(|error| format!("Cannot read provider metadata {}: {error}", path.display()))?;
    modified
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis())
        .map_err(|_| format!("Provider mtime predates Unix epoch: {}", path.display()))
}

fn invocation_modified_ms(invocation: &ProviderInvocation) -> Result<u128, String> {
    let mut latest = modified_ms(&invocation.executable)?;
    for argument in &invocation.executable_prefix {
        let path = Path::new(argument);
        if path.is_file() {
            latest = latest.max(modified_ms(path)?);
        }
    }
    Ok(latest)
}

fn atomic_write(path: &Path, bytes: &[u8]) -> Result<(), String> {
    let parent = path.parent().ok_or("Invalid cache path")?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create cache temporary file: {error}"))?;
    use std::io::Write;
    temporary
        .write_all(bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write provider cache: {error}"))?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| format!("Cannot persist provider cache: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[cfg(unix)]
    fn mock_invocation(
        executable: PathBuf,
        complete_timeout_ms: u64,
        environment: BTreeMap<String, String>,
    ) -> ProviderInvocation {
        ProviderInvocation {
            extension_id: "dev.floter.mock".into(),
            executable,
            executable_prefix: Vec::new(),
            runtime_root: None,
            package_version: "1.0.0".into(),
            tool_version_hint: None,
            version_args: Vec::new(),
            config: ProviderConfig {
                kind: crate::extensions::manifest::ProviderKind::Executable,
                descriptor: None,
                args_prefix: vec!["--floter".into()],
                describe_timeout_ms: 5_000,
                complete_timeout_ms,
                environment,
            },
            permissions: Vec::new(),
        }
    }

    #[cfg(unix)]
    fn mock_provider(directory: &Path, body: &str) -> PathBuf {
        use std::os::unix::fs::PermissionsExt;

        let executable = directory.join("mock-provider");
        std::fs::write(&executable, format!("#!/bin/sh\n{body}\n")).unwrap();
        let mut permissions = executable.metadata().unwrap().permissions();
        permissions.set_mode(0o755);
        std::fs::set_permissions(&executable, permissions).unwrap();
        executable
    }

    #[test]
    fn parses_reference_description() {
        let value: Value = serde_json::from_slice(include_bytes!(
            "../../../docs/extensions/examples/v/provider-description.json"
        ))
        .unwrap();
        validate_description_schema(&value).unwrap();
        let description: ProviderDescription = serde_json::from_value(value).unwrap();
        assert_eq!(description.commands[0].id, "jv");
    }

    #[test]
    fn description_schema_accepts_versioned_env_var_configuration() {
        let mut value: Value = serde_json::from_slice(include_bytes!(
            "../../../docs/extensions/examples/v/provider-description.json"
        ))
        .unwrap();
        value["configuration"] = serde_json::json!({
            "configVersion": 2,
            "owner": "host",
            "environmentMapping": { "apiKey": "V_API_KEY" },
            "schema": [{
                "key": "apiKey",
                "type": "password",
                "required": true,
                "envVar": "V_API_KEY"
            }, {
                "key": "retries",
                "type": "number",
                "minimum": 0,
                "maximum": 5,
                "default": 2
            }]
        });

        validate_description_schema(&value).unwrap();
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn complete_uses_the_protocol_args_and_wire_format() {
        let directory = tempfile::tempdir().unwrap();
        let args_path = directory.path().join("args");
        let request_path = directory.path().join("request.json");
        let executable = mock_provider(
            directory.path(),
            r#"printf '%s\n' "$@" > "$ARGS_PATH"
cat > "$REQUEST_PATH"
printf '%s' '{"completions":[{"label":"-file","kind":"flag","detail":"Read from file"}]}'"#,
        );
        let environment = BTreeMap::from([
            ("ARGS_PATH".into(), args_path.to_string_lossy().into_owned()),
            (
                "REQUEST_PATH".into(),
                request_path.to_string_lossy().into_owned(),
            ),
        ]);
        let invocation = mock_invocation(executable, 800, environment);
        let manager = ProviderManager::new(directory.path().join("cache"));
        let request = serde_json::json!({
            "command": "jv",
            "args": ["-f"],
            "cwd": "/path"
        });

        let response = manager.complete(&invocation, &request).await.unwrap();

        assert_eq!(
            std::fs::read_to_string(args_path).unwrap(),
            "--floter\ncomplete\n--protocol\n1\n"
        );
        let recorded: Value =
            serde_json::from_slice(&std::fs::read(request_path).unwrap()).unwrap();
        assert_eq!(recorded, request);
        assert_eq!(response.completions.len(), 1);
        assert_eq!(response.completions[0].label, "-file");
        assert_eq!(response.completions[0].kind, "flag");
        assert_eq!(response.completions[0].detail, "Read from file");
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn provider_without_environment_permission_does_not_inherit_host_variables() {
        let directory = tempfile::tempdir().unwrap();
        let executable = mock_provider(
            directory.path(),
            r#"cat >/dev/null
printf '%s' '{"completions":[{"label":"env","kind":"value","detail":"'"${FLOTER_PERMISSION_PARENT-unset}"':'"$EXPLICIT_VALUE"'"}]}'"#,
        );
        std::env::set_var("FLOTER_PERMISSION_PARENT", "host-secret");
        let invocation = mock_invocation(
            executable,
            800,
            BTreeMap::from([("EXPLICIT_VALUE".into(), "configured".into())]),
        );
        let manager = ProviderManager::new(directory.path().join("cache"));

        let response = manager
            .complete(&invocation, &serde_json::json!({"command": "env"}))
            .await
            .unwrap();
        std::env::remove_var("FLOTER_PERMISSION_PARENT");

        assert_eq!(response.completions[0].detail, "unset:configured");
    }

    #[cfg(unix)]
    #[test]
    fn external_execution_program_requires_process_spawn_permission() {
        let directory = tempfile::tempdir().unwrap();
        let executable = mock_provider(directory.path(), "exit 0");
        let child = directory.path().join("child");
        std::fs::write(&child, "child").unwrap();
        let mut invocation = mock_invocation(executable, 800, BTreeMap::new());
        invocation.runtime_root = Some(directory.path().to_path_buf());
        let command = CommandDescriptor {
            id: "child".into(),
            name: "Child".into(),
            description: String::new(),
            aliases: Vec::new(),
            keywords: Vec::new(),
            execution: ExecutionDescriptor {
                program: "child".into(),
                args_prefix: Vec::new(),
                mode: ExecutionMode::Capture,
                working_directory: WorkingDirectory::Current,
            },
            arguments: Vec::new(),
        };

        assert!(execution_plan(&command, &invocation, Vec::new(), None)
            .unwrap_err()
            .contains("process-spawn"));
        invocation.permissions.push(Permission::ProcessSpawn);
        let plan = execution_plan(&command, &invocation, Vec::new(), None).unwrap();
        assert!(!plan.inherit_environment);
        invocation.permissions.push(Permission::Environment);
        let plan = execution_plan(&command, &invocation, Vec::new(), None).unwrap();
        assert!(plan.inherit_environment);
    }

    #[test]
    fn legacy_capture_mode_uses_the_supported_embedded_terminal() {
        assert_eq!(ExecutionMode::Capture.host_mode(), ExecutionMode::Pty);
        assert_eq!(ExecutionMode::External.host_mode(), ExecutionMode::External);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn complete_honors_the_configured_timeout() {
        let directory = tempfile::tempdir().unwrap();
        let executable = mock_provider(directory.path(), "cat >/dev/null\nsleep 1");
        let invocation = mock_invocation(executable, 50, BTreeMap::new());
        let manager = ProviderManager::new(directory.path().join("cache"));

        let error = manager
            .complete(
                &invocation,
                &serde_json::json!({"command": "jv", "args": ["-f"], "cwd": null}),
            )
            .await
            .unwrap_err();

        assert!(error.contains("timed out after 50 ms"), "{error}");
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn complete_reports_a_nonzero_provider_exit() {
        let directory = tempfile::tempdir().unwrap();
        let executable = mock_provider(
            directory.path(),
            "cat >/dev/null\necho 'complete unsupported' >&2\nexit 7",
        );
        let invocation = mock_invocation(executable, 800, BTreeMap::new());
        let manager = ProviderManager::new(directory.path().join("cache"));

        let error = manager
            .complete(
                &invocation,
                &serde_json::json!({"command": "jv", "args": ["-f"], "cwd": null}),
            )
            .await
            .unwrap_err();

        assert!(
            error.contains("exited with 7: complete unsupported"),
            "{error}"
        );
    }
}
