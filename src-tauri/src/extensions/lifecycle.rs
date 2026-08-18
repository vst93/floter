use crate::extensions::lock::{sync_directory, validate_id, ExtensionLockEntry};
use crate::extensions::manifest::{validate_relative_path, ExtensionManifest, Permission};
use crate::extensions::provider::ProviderInvocation;
use crate::extensions::{proxy, ExtensionPaths};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::io::Write;
use std::path::{Component, Path, PathBuf};
use std::process::Stdio;
use std::time::Duration;
use tokio::io::{AsyncRead, AsyncReadExt};

const RECEIPT_SCHEMA_VERSION: u32 = 1;
const MAX_COMPLETION_BYTES: usize = 2 * 1024 * 1024;

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ToolLifecycle {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub completions: Vec<ShellCompletion>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub configuration_templates: Vec<ConfigurationTemplate>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub probes: Vec<CapabilityProbeEntry>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub launch: Option<LaunchConfig>,
}

impl ToolLifecycle {
    pub fn is_empty(&self) -> bool {
        self.completions.is_empty()
            && self.configuration_templates.is_empty()
            && self.probes.is_empty()
            && self.launch.is_none()
    }

    pub fn validate(&self) -> Result<(), String> {
        let mut shells = BTreeSet::new();
        for completion in &self.completions {
            if !shells.insert(completion.shell) {
                return Err(format!(
                    "Lifecycle declares more than one {} completion",
                    completion.shell.as_str()
                ));
            }
            match (&completion.source, completion.args.is_empty()) {
                (Some(source), true) => {
                    validate_relative_path(source, "completion source")?;
                }
                (None, false) => {}
                _ => {
                    return Err(format!(
                        "{} completion must declare exactly one of source or args",
                        completion.shell.as_str()
                    ));
                }
            }
            if completion.args.len() > 16 {
                return Err("Completion generation accepts at most 16 arguments".to_string());
            }
            if !(100..=10_000).contains(&completion.timeout_ms) {
                return Err("Completion timeout must be between 100 and 10000 ms".to_string());
            }
            if let Some(file_name) = completion.file_name.as_deref() {
                validate_file_name(file_name)?;
            }
        }

        let mut targets = BTreeSet::new();
        for template in &self.configuration_templates {
            validate_relative_path(&template.source, "configuration template source")?;
            let target = validate_relative_path(&template.target, "configuration template target")?;
            if !targets.insert(target) {
                return Err(format!(
                    "Lifecycle declares configuration target {} more than once",
                    template.target
                ));
            }
        }

        let mut probe_ids = BTreeSet::new();
        for probe in &self.probes {
            if !probe_ids.insert(probe.id.clone()) {
                return Err(format!(
                    "Lifecycle declares probe '{}' more than once",
                    probe.id
                ));
            }
            if !(100..=30_000).contains(&probe.timeout_ms) {
                return Err(format!(
                    "Probe '{}' timeout must be between 100 and 30000 ms",
                    probe.id
                ));
            }
            if probe.args.len() > 32 {
                return Err(format!("Probe '{}' accepts at most 32 arguments", probe.id));
            }
        }

        if let Some(launch) = &self.launch {
            launch.validate()?;
        }

        Ok(())
    }
}

/// A capability probe declaration in the tool lifecycle.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct CapabilityProbeEntry {
    pub id: String,
    pub args: Vec<String>,
    #[serde(default = "default_probe_timeout")]
    pub timeout_ms: u64,
    #[serde(default = "default_false")]
    pub required: bool,
}

fn default_probe_timeout() -> u64 {
    2000
}

fn default_false() -> bool {
    false
}

/// Launch configuration for a tool.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct LaunchConfig {
    #[serde(default)]
    pub cwd_policy: serde_json::Value,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub terminal: Option<TerminalRequirement>,
    #[serde(default = "default_restore_policy")]
    pub restore_policy: String,
}

impl LaunchConfig {
    pub fn validate(&self) -> Result<(), String> {
        match self.cwd_policy.as_str() {
            Some("inheritActiveSession" | "toolData" | "home") => {}
            _ => {
                // Could be a ProjectRoot or Fixed object, validate as JSON
                if !self.cwd_policy.is_object() && !self.cwd_policy.is_string() {
                    return Err("cwd_policy must be a string or object".to_string());
                }
            }
        }
        if self.restore_policy != "reattach"
            && self.restore_policy != "restart"
            && self.restore_policy != "none"
        {
            return Err(format!(
                "restore_policy must be 'reattach', 'restart', or 'none', got '{}'",
                self.restore_policy
            ));
        }
        Ok(())
    }
}

fn default_restore_policy() -> String {
    "reattach".to_string()
}

/// Terminal requirements for a tool launch.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct TerminalRequirement {
    #[serde(default = "default_true")]
    pub required: bool,
    #[serde(default = "default_color")]
    pub color: String,
    #[serde(default = "default_true")]
    pub unicode: bool,
    #[serde(default)]
    pub mouse: Option<String>,
    #[serde(default = "default_false")]
    pub bracketed_paste: bool,
    #[serde(default = "default_false")]
    pub synchronized_output: bool,
    #[serde(default)]
    pub keyboard_protocol: Option<String>,
}

fn default_true() -> bool {
    true
}

fn default_color() -> String {
    "truecolor".to_string()
}

impl Default for LaunchConfig {
    fn default() -> Self {
        Self {
            cwd_policy: serde_json::Value::String("inheritActiveSession".to_string()),
            terminal: None,
            restore_policy: default_restore_policy(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ShellCompletion {
    pub shell: Shell,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub args: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub file_name: Option<String>,
    #[serde(default = "default_completion_timeout")]
    pub timeout_ms: u64,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "lowercase")]
pub enum Shell {
    Bash,
    Zsh,
    Fish,
    Powershell,
}

impl Shell {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Bash => "bash",
            Self::Zsh => "zsh",
            Self::Fish => "fish",
            Self::Powershell => "powershell",
        }
    }

    fn default_file_name(self, extension_id: &str) -> String {
        match self {
            Self::Bash => extension_id.to_string(),
            Self::Zsh => format!("_{extension_id}"),
            Self::Fish => format!("{extension_id}.fish"),
            Self::Powershell => format!("{extension_id}.ps1"),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ConfigurationTemplate {
    pub source: String,
    pub target: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct LifecycleReport {
    pub generated_completions: Vec<String>,
    pub deployed_templates: Vec<String>,
    pub updated_templates: Vec<String>,
    pub preserved_templates: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct LifecycleReceipt {
    schema_version: u32,
    #[serde(default)]
    completion_files: Vec<String>,
    #[serde(default)]
    templates: BTreeMap<String, TemplateReceipt>,
}

impl Default for LifecycleReceipt {
    fn default() -> Self {
        Self {
            schema_version: RECEIPT_SCHEMA_VERSION,
            completion_files: Vec::new(),
            templates: BTreeMap::new(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct TemplateReceipt {
    digest: String,
    managed: bool,
}

pub async fn activate(
    paths: &ExtensionPaths,
    entry: &ExtensionLockEntry,
) -> Result<LifecycleReport, String> {
    validate_id(&entry.id)?;
    let manifest_path = Path::new(&entry.manifest_path);
    let manifest = ExtensionManifest::load(manifest_path)?;
    if manifest.id != entry.id {
        return Err(format!(
            "Lifecycle manifest id {} does not match lock entry {}",
            manifest.id, entry.id
        ));
    }
    manifest.lifecycle.validate()?;
    let package_root = manifest_path
        .parent()
        .ok_or("Lifecycle manifest has no parent directory")?;
    let data_root = paths.data.join(&entry.id);
    std::fs::create_dir_all(&data_root)
        .map_err(|error| format!("Cannot create lifecycle data directory: {error}"))?;
    reject_symlink_path(&paths.data, &data_root)?;

    let receipt_path = data_root.join("lifecycle.json");
    let mut receipt = load_receipt(&receipt_path)?;
    let mut report = reconcile_templates(
        package_root,
        &data_root.join("tool-data"),
        &manifest.lifecycle.configuration_templates,
        &mut receipt,
    )?;

    let completion_files = reconcile_completions(
        &data_root,
        package_root,
        entry,
        &manifest.lifecycle.completions,
    )
    .await?;
    report.generated_completions = completion_files.clone();
    receipt.completion_files = completion_files;
    save_receipt(&receipt_path, &receipt)?;
    Ok(report)
}

pub fn deactivate(paths: &ExtensionPaths, extension_id: &str) -> Result<(), String> {
    validate_id(extension_id)?;
    let data_root = paths.data.join(extension_id);
    let receipt_path = data_root.join("lifecycle.json");
    let mut receipt = load_receipt(&receipt_path)?;
    let completions = data_root.join("completions");
    if completions.exists() {
        reject_symlink_path(&paths.data, &completions)?;
        std::fs::remove_dir_all(&completions)
            .map_err(|error| format!("Cannot remove shell completions: {error}"))?;
        sync_directory(&data_root)
            .map_err(|error| format!("Cannot sync lifecycle data directory: {error}"))?;
    }
    if !receipt.completion_files.is_empty() {
        receipt.completion_files.clear();
        save_receipt(&receipt_path, &receipt)?;
    }
    Ok(())
}

fn reconcile_templates(
    package_root: &Path,
    tool_data_root: &Path,
    templates: &[ConfigurationTemplate],
    receipt: &mut LifecycleReceipt,
) -> Result<LifecycleReport, String> {
    let mut report = LifecycleReport::default();
    let mut declared = BTreeSet::new();
    for template in templates {
        let source_relative =
            validate_relative_path(&template.source, "configuration template source")?;
        let target_relative =
            validate_relative_path(&template.target, "configuration template target")?;
        declared.insert(template.target.clone());
        let source = package_root.join(source_relative);
        let source = contained_file(package_root, &source, "configuration template")?;
        let bytes = std::fs::read(&source).map_err(|error| {
            format!(
                "Cannot read configuration template {}: {error}",
                source.display()
            )
        })?;
        let computed = digest(&bytes);
        let target = tool_data_root.join(target_relative);
        reject_symlink_path(tool_data_root, &target)?;
        let existing = std::fs::read(&target).ok();
        let previous = receipt.templates.get(&template.target);
        let may_write = match (&existing, previous) {
            (None, _) => true,
            (Some(bytes), Some(previous)) if previous.managed => computed == previous.digest,
            _ => false,
        };
        if may_write {
            let was_present = existing.is_some();
            atomic_write(&target, &bytes)?;
            receipt.templates.insert(
                template.target.clone(),
                TemplateReceipt {
                    digest: computed,
                    managed: true,
                },
            );
            if was_present {
                report.updated_templates.push(template.target.clone());
            } else {
                report.deployed_templates.push(template.target.clone());
            }
        } else {
            receipt.templates.insert(
                template.target.clone(),
                TemplateReceipt {
                    digest: computed,
                    managed: false,
                },
            );
            report.preserved_templates.push(template.target.clone());
        }
    }

    let removed = receipt
        .templates
        .keys()
        .filter(|target| !declared.contains(*target))
        .cloned()
        .collect::<Vec<_>>();
    for target in removed {
        receipt.templates.remove(&target);
    }
    Ok(report)
}

async fn reconcile_completions(
    data_root: &Path,
    package_root: &Path,
    entry: &ExtensionLockEntry,
    completions: &[ShellCompletion],
) -> Result<Vec<String>, String> {
    let final_root = data_root.join("completions");
    if completions.is_empty() {
        if final_root.exists() {
            std::fs::remove_dir_all(&final_root)
                .map_err(|error| format!("Cannot remove stale shell completions: {error}"))?;
        }
        return Ok(Vec::new());
    }

    let staging = tempfile::Builder::new()
        .prefix(".completions-")
        .tempdir_in(data_root)
        .map_err(|error| format!("Cannot create completion staging directory: {error}"))?;
    let invocation = crate::extensions::registry::provider_invocation(entry)?;
    let mut files = Vec::with_capacity(completions.len());
    for completion in completions {
        let file_name = completion
            .file_name
            .clone()
            .unwrap_or_else(|| completion.shell.default_file_name(&entry.id));
        validate_file_name(&file_name)?;
        let relative = PathBuf::from(completion.shell.as_str()).join(file_name);
        let bytes = match completion.source.as_deref() {
            Some(source) => {
                let source =
                    package_root.join(validate_relative_path(source, "completion source")?);
                let source = contained_file(package_root, &source, "completion source")?;
                let bytes = std::fs::read(&source).map_err(|error| {
                    format!(
                        "Cannot read completion source {}: {error}",
                        source.display()
                    )
                })?;
                if bytes.len() > MAX_COMPLETION_BYTES {
                    return Err(format!(
                        "{} completion exceeds {} bytes",
                        completion.shell.as_str(),
                        MAX_COMPLETION_BYTES
                    ));
                }
                bytes
            }
            None => generate_completion(&invocation, completion, package_root).await?,
        };
        atomic_write(&staging.path().join(&relative), &bytes)?;
        files.push(relative.to_string_lossy().into_owned());
    }

    let staged = staging.keep();
    let backup = data_root.join(format!(".completions-old-{}", uuid::Uuid::new_v4()));
    if final_root.exists() {
        std::fs::rename(&final_root, &backup)
            .map_err(|error| format!("Cannot stage old shell completions: {error}"))?;
    }
    if let Err(error) = std::fs::rename(&staged, &final_root) {
        if backup.exists() {
            let _ = std::fs::rename(&backup, &final_root);
        }
        let _ = std::fs::remove_dir_all(&staged);
        return Err(format!("Cannot activate shell completions: {error}"));
    }
    sync_directory(data_root)
        .map_err(|error| format!("Cannot sync lifecycle data directory: {error}"))?;
    if backup.exists() {
        std::fs::remove_dir_all(&backup)
            .map_err(|error| format!("Cannot remove old shell completions: {error}"))?;
    }
    Ok(files)
}

async fn generate_completion(
    invocation: &ProviderInvocation,
    completion: &ShellCompletion,
    cwd: &Path,
) -> Result<Vec<u8>, String> {
    let mut command = provider_command(&invocation.executable);
    if !invocation.permissions.contains(&Permission::Environment) {
        command.env_clear();
    }
    command
        .args(&invocation.executable_prefix)
        .args(&completion.args)
        .envs(proxy::command_environment(
            &invocation.permissions,
            &invocation.config.environment,
        ))
        .current_dir(cwd)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true);
    let mut child = command.spawn().map_err(|error| {
        format!(
            "Cannot generate {} completion: {error}",
            completion.shell.as_str()
        )
    })?;
    let stdout = child
        .stdout
        .take()
        .ok_or("Completion stdout is unavailable")?;
    let stderr = child
        .stderr
        .take()
        .ok_or("Completion stderr is unavailable")?;
    let stdout_task = tokio::spawn(read_limited(stdout, MAX_COMPLETION_BYTES));
    let stderr_task = tokio::spawn(read_limited(stderr, 64 * 1024));
    let timeout = Duration::from_millis(completion.timeout_ms);
    let status = match tokio::time::timeout(timeout, child.wait()).await {
        Ok(result) => {
            result.map_err(|error| format!("Cannot wait for completion generator: {error}"))?
        }
        Err(_) => {
            let _ = child.kill().await;
            let _ = child.wait().await;
            return Err(format!(
                "{} completion generation timed out after {} ms",
                completion.shell.as_str(),
                completion.timeout_ms
            ));
        }
    };
    let stdout = stdout_task
        .await
        .map_err(|error| format!("Completion stdout task failed: {error}"))??;
    let stderr = stderr_task
        .await
        .map_err(|error| format!("Completion stderr task failed: {error}"))??;
    if !status.success() {
        return Err(format!(
            "{} completion generation failed: {}",
            completion.shell.as_str(),
            String::from_utf8_lossy(&stderr).trim()
        ));
    }
    if stdout.is_empty() {
        return Err(format!(
            "{} completion generation returned no output",
            completion.shell.as_str()
        ));
    }
    Ok(stdout)
}

async fn read_limited<R: AsyncRead + Unpin>(reader: R, limit: usize) -> Result<Vec<u8>, String> {
    let mut bytes = Vec::new();
    reader
        .take((limit + 1) as u64)
        .read_to_end(&mut bytes)
        .await
        .map_err(|error| format!("Cannot read lifecycle process output: {error}"))?;
    if bytes.len() > limit {
        return Err(format!("Lifecycle process output exceeds {limit} bytes"));
    }
    Ok(bytes)
}

#[cfg(target_os = "windows")]
fn provider_command(executable: &Path) -> tokio::process::Command {
    let extension = executable
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or_default();
    if extension.eq_ignore_ascii_case("cmd") || extension.eq_ignore_ascii_case("bat") {
        let mut command = tokio::process::Command::new("cmd.exe");
        command.args(["/D", "/S", "/C"]).arg(executable);
        return command;
    }
    tokio::process::Command::new(executable)
}

#[cfg(not(target_os = "windows"))]
fn provider_command(executable: &Path) -> tokio::process::Command {
    tokio::process::Command::new(executable)
}

fn default_completion_timeout() -> u64 {
    3_000
}

fn validate_file_name(value: &str) -> Result<(), String> {
    if value.is_empty()
        || value.len() > 128
        || !value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'_' | b'-'))
    {
        return Err(format!("Invalid completion file name: {value}"));
    }
    Ok(())
}

fn contained_file(root: &Path, path: &Path, field: &str) -> Result<PathBuf, String> {
    let root = std::fs::canonicalize(root)
        .map_err(|error| format!("Cannot resolve lifecycle package root: {error}"))?;
    let path = std::fs::canonicalize(path)
        .map_err(|error| format!("Cannot resolve {field} {}: {error}", path.display()))?;
    if !path.starts_with(&root) || !path.is_file() {
        return Err(format!("{field} escaped the extension package"));
    }
    Ok(path)
}

fn reject_symlink_path(root: &Path, target: &Path) -> Result<(), String> {
    let relative = target
        .strip_prefix(root)
        .map_err(|_| "Lifecycle target escaped its data directory".to_string())?;
    let mut current = root.to_path_buf();
    for component in relative.components() {
        if !matches!(component, Component::Normal(_)) {
            return Err("Lifecycle target contains an unsafe path component".to_string());
        }
        current.push(component);
        match std::fs::symlink_metadata(&current) {
            Ok(metadata) if metadata.file_type().is_symlink() => {
                return Err(format!(
                    "Lifecycle target traverses a symbolic link: {}",
                    current.display()
                ));
            }
            Ok(_) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => break,
            Err(error) => {
                return Err(format!(
                    "Cannot inspect lifecycle target {}: {error}",
                    current.display()
                ));
            }
        }
    }
    Ok(())
}

fn digest(bytes: &[u8]) -> String {
    format!("sha256:{:x}", Sha256::digest(bytes))
}

fn load_receipt(path: &Path) -> Result<LifecycleReceipt, String> {
    let bytes = match std::fs::read(path) {
        Ok(bytes) => bytes,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(LifecycleReceipt::default())
        }
        Err(error) => return Err(format!("Cannot read lifecycle receipt: {error}")),
    };
    let receipt: LifecycleReceipt = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid lifecycle receipt: {error}"))?;
    if receipt.schema_version != RECEIPT_SCHEMA_VERSION {
        return Err(format!(
            "Unsupported lifecycle receipt schema {}",
            receipt.schema_version
        ));
    }
    Ok(receipt)
}

fn save_receipt(path: &Path, receipt: &LifecycleReceipt) -> Result<(), String> {
    let bytes = serde_json::to_vec_pretty(receipt)
        .map_err(|error| format!("Cannot serialize lifecycle receipt: {error}"))?;
    atomic_write(path, &bytes)
}

fn atomic_write(path: &Path, bytes: &[u8]) -> Result<(), String> {
    let parent = path.parent().ok_or("Invalid lifecycle output path")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create lifecycle output directory: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create lifecycle output: {error}"))?;
    temporary
        .write_all(bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write lifecycle output: {error}"))?;
    temporary
        .persist(path)
        .map_err(|error| format!("Cannot persist lifecycle output: {error}"))?;
    sync_directory(parent)
        .map_err(|error| format!("Cannot sync lifecycle output directory: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_lifecycle_is_empty() {
        let lifecycle = ToolLifecycle::default();
        assert!(lifecycle.is_empty());
        assert!(lifecycle.probes.is_empty());
        assert!(lifecycle.launch.is_none());
    }

    #[test]
    fn lifecycle_with_completions_is_not_empty() {
        let lifecycle = ToolLifecycle {
            completions: vec![ShellCompletion {
                shell: Shell::Bash,
                source: Some("completions/bash.sh".into()),
                args: vec![],
                file_name: None,
                timeout_ms: 3000,
            }],
            ..Default::default()
        };
        assert!(!lifecycle.is_empty());
    }

    #[test]
    fn lifecycle_with_probes_is_not_empty() {
        let lifecycle = ToolLifecycle {
            probes: vec![CapabilityProbeEntry {
                id: "version".into(),
                args: vec!["--version".into()],
                timeout_ms: 2000,
                required: true,
            }],
            ..Default::default()
        };
        assert!(!lifecycle.is_empty());
    }

    #[test]
    fn validate_rejects_duplicate_completions() {
        let lifecycle = ToolLifecycle {
            completions: vec![
                ShellCompletion {
                    shell: Shell::Bash,
                    source: Some("comp1.sh".into()),
                    args: vec![],
                    file_name: None,
                    timeout_ms: 3000,
                },
                ShellCompletion {
                    shell: Shell::Bash,
                    source: Some("comp2.sh".into()),
                    args: vec![],
                    file_name: None,
                    timeout_ms: 3000,
                },
            ],
            ..Default::default()
        };
        let result = lifecycle.validate();
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("more than one bash"));
    }

    #[test]
    fn validate_rejects_duplicate_probes() {
        let lifecycle = ToolLifecycle {
            probes: vec![
                CapabilityProbeEntry {
                    id: "version".into(),
                    args: vec!["--version".into()],
                    timeout_ms: 2000,
                    required: true,
                },
                CapabilityProbeEntry {
                    id: "version".into(),
                    args: vec!["-v".into()],
                    timeout_ms: 2000,
                    required: false,
                },
            ],
            ..Default::default()
        };
        let result = lifecycle.validate();
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .contains("probe 'version' more than once"));
    }

    #[test]
    fn validate_rejects_probe_timeout_out_of_range() {
        let lifecycle = ToolLifecycle {
            probes: vec![CapabilityProbeEntry {
                id: "slow".into(),
                args: vec!["--version".into()],
                timeout_ms: 50_000, // exceeds 30000
                required: true,
            }],
            ..Default::default()
        };
        let result = lifecycle.validate();
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("between 100 and 30000"));
    }

    #[test]
    fn validate_rejects_too_many_probe_args() {
        let lifecycle = ToolLifecycle {
            probes: vec![CapabilityProbeEntry {
                id: "many".into(),
                args: (0..40).map(|i| format!("arg{i}")).collect(),
                timeout_ms: 2000,
                required: true,
            }],
            ..Default::default()
        };
        let result = lifecycle.validate();
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("at most 32 arguments"));
    }

    #[test]
    fn validate_accepts_valid_probes() {
        let lifecycle = ToolLifecycle {
            probes: vec![
                CapabilityProbeEntry {
                    id: "version".into(),
                    args: vec!["--version".into()],
                    timeout_ms: 2000,
                    required: true,
                },
                CapabilityProbeEntry {
                    id: "completion".into(),
                    args: vec!["--generate-completion".into(), "bash".into()],
                    timeout_ms: 3000,
                    required: false,
                },
            ],
            ..Default::default()
        };
        assert!(lifecycle.validate().is_ok());
    }

    #[test]
    fn launch_config_default() {
        let launch = LaunchConfig::default();
        assert_eq!(launch.restore_policy, "reattach");
        assert!(launch.terminal.is_none());
    }

    #[test]
    fn launch_config_validate_valid() {
        let launch = LaunchConfig {
            cwd_policy: serde_json::json!("inheritActiveSession"),
            terminal: Some(TerminalRequirement {
                required: true,
                color: "truecolor".into(),
                unicode: true,
                mouse: Some("sgr".into()),
                bracketed_paste: true,
                synchronized_output: false,
                keyboard_protocol: Some("kitty-preferred".into()),
            }),
            restore_policy: "restart".into(),
        };
        assert!(launch.validate().is_ok());
    }

    #[test]
    fn launch_config_validate_invalid_restore_policy() {
        let launch = LaunchConfig {
            restore_policy: "invalid".into(),
            ..Default::default()
        };
        assert!(launch.validate().is_err());
    }

    #[test]
    fn launch_config_validate_invalid_cwd_policy() {
        let launch = LaunchConfig {
            cwd_policy: serde_json::json!(42),
            ..Default::default()
        };
        assert!(launch.validate().is_err());
    }
}
