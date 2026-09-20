//! Manual runs of a connected integration (R9-2 slice 1).
//!
//! Until this module there was **no execution entry** for a custom script:
//! the catalog described it, `extensions_launch` only read
//! `lifecycle.launch` (which a generated custom integration never declares),
//! and the frontend never called either. "Run this script and let me see what
//! it printed" therefore did not exist — the user's complaint was not a
//! routing problem, it was a missing action.
//!
//! This module owns exactly two things:
//!
//! 1. **The argv** — one place builds it, as a structured `Vec<String>`.
//!    Nothing here ever joins arguments into a shell string: the terminal
//!    route hands the frontend a *protected* plan (`plan_token`, argv already
//!    stripped) and the background route spawns `program + args` directly.
//!    The frontend never assembles argv.
//! 2. **The route** — `terminal` returns the protected plan for `runCommand`
//!    (terminal page + PTY streaming); `background` runs the process here,
//!    captures both streams into a bounded in-memory record, and reports the
//!    outcome.
//!
//! Parameters (user-filled argv) are slice 2/3; [`validate_flag`] lands here
//! so the argv whitelist exists before anything can use it.

use super::lock::{ExtensionLockEntry, ExtensionsLock};
use super::manifest::{validate_flag, ExtensionManifest, OutputMode, ParamDefinition, ParamKind};
use super::process_cleanup::{command_output, CommandOutputError};
use super::provider::{
    self, CommandDescriptor, ExecutionDescriptor, ExecutionMode, ExecutionPlan, ProviderInvocation,
    WorkingDirectory,
};
use super::registry;
use super::ExtensionState;
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::{Duration, Instant};

/// Upper bound on the bytes retained per stream in the "last output" record.
/// Matches the probe cap (`capability_probe::MAX_PROBE_OUTPUT_BYTES`): a
/// runaway script must not be able to grow the host's memory without bound.
pub(crate) const MAX_RUN_OUTPUT_BYTES: usize = 64 * 1024;

/// The prefix of the refusal a second concurrent run of the same integration
/// receives. The frontend maps it to a localised "a run is already in
/// progress" notice; it is a stable key plus the id, never a prose string.
pub(crate) const RUN_ALREADY_IN_FLIGHT: &str = "run_already_in_flight";

/// Per-integration in-flight marks for manual runs (R9-2 slice 4).
///
/// Deliberately in memory only: a run is a foreground action of *this* host
/// process, so a lock file would outlive the process that could honour it and
/// would need crash cleanup. One `BTreeSet` under one mutex — the critical
/// section is an insert or a remove, so contention is irrelevant.
#[derive(Default)]
pub(crate) struct RunInFlight {
    active: std::sync::Mutex<BTreeSet<String>>,
}

impl RunInFlight {
    /// Claim the run slot for `id`, or refuse when another run of the same
    /// integration is still in flight. The returned guard releases the slot on
    /// drop, so every exit path — success, refusal, timeout, panic unwind —
    /// clears it and a later run is not blocked forever.
    pub(crate) fn begin(&self, id: &str) -> Result<RunInFlightGuard<'_>, String> {
        let mut active = self
            .active
            .lock()
            .map_err(|_| "run registry poisoned".to_string())?;
        if !active.insert(id.to_string()) {
            return Err(format!("{RUN_ALREADY_IN_FLIGHT}:{id}"));
        }
        Ok(RunInFlightGuard {
            registry: self,
            id: id.to_string(),
        })
    }

    #[cfg(test)]
    pub(crate) fn is_active(&self, id: &str) -> bool {
        self.active
            .lock()
            .map(|active| active.contains(id))
            .unwrap_or(false)
    }
}

/// RAII release of one [`RunInFlight`] claim. Mutation: replace `begin` with a
/// no-op guard and the concurrent-run refusal test goes red.
pub(crate) struct RunInFlightGuard<'a> {
    registry: &'a RunInFlight,
    id: String,
}

impl std::fmt::Debug for RunInFlightGuard<'_> {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("RunInFlightGuard")
            .field("id", &self.id)
            .finish_non_exhaustive()
    }
}

impl Drop for RunInFlightGuard<'_> {
    fn drop(&mut self) {
        if let Ok(mut active) = self.registry.active.lock() {
            active.remove(&self.id);
        }
    }
}

/// The argument values a caller filled in for one run, keyed by parameter id.
/// Values are always strings on the wire; the kind conversion happens here.
pub type ParamValues = BTreeMap<String, String>;

/// Characters cmd.exe re-parses. A `.cmd`/`.bat` program is launched through
/// `cmd.exe /D /S /C` (`provider::execution_host`), so an argument containing
/// any of these can still change the command line even though it arrived as its
/// own `argv` entry. The structured-argv guarantee does not extend across
/// cmd.exe, so the run refuses such a value rather than pretending it is safe.
/// This is the only platform-specific hole in the defence and it is closed by
/// refusal, not by quoting.
pub const WINDOWS_CMD_UNSAFE_CHARS: [char; 6] = ['&', '|', '<', '>', '^', '%'];

/// The first character in `value` that cmd.exe would re-parse, if any.
pub fn windows_cmd_unsafe_char(value: &str) -> Option<char> {
    value
        .chars()
        .find(|character| WINDOWS_CMD_UNSAFE_CHARS.contains(character))
}

/// Whether `program` is a Windows command script, which `execution_host`
/// launches through cmd.exe. Always false off Windows: the wrap only happens
/// there, so only there can a value be re-parsed. Kept separate from the check
/// itself so the character set stays testable on every platform.
pub fn program_is_cmd_script(program: &Path) -> bool {
    if !cfg!(windows) {
        return false;
    }
    let extension = program
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or_default();
    extension.eq_ignore_ascii_case("cmd") || extension.eq_ignore_ascii_case("bat")
}

/// Append `flag` (when present) and `value` as **two separate argv items**.
fn push_flagged(args: &mut Vec<String>, flag: Option<&str>, value: &str) {
    if let Some(flag) = flag {
        args.push(flag.to_string());
    }
    args.push(value.to_string());
}

/// Turn the declared parameters plus the caller's answers into argv items.
///
/// This is the whole run-time half of the parameter feature, and it exists in
/// exactly one place so no second argv builder can drift from it. It reads the
/// manifest's `params` as the single source; the descriptor side contributes
/// nothing to this decision. The order (research §1.4) is: the interpreter /
/// script prefix and the descriptor's `argsPrefix` are already in the plan when
/// this function's result is appended, so the final argv is
///
/// ```text
/// executable_prefix ++ args_prefix ++ <param args> ++ <user args, empty today>
/// ```
///
/// Per declared parameter, in declaration order:
///
///   * `flag` then `value` as two separate items when a flag is declared;
///   * `value` alone when it is not;
///   * `flag` alone for a boolean that is true;
///   * nothing for a boolean that is false or absent (a non-boolean that is
///     absent and not required is skipped, mirroring the manifest rule).
///
/// Values are converted per kind: `number` must parse as `i64` or `f64`, a
/// `select` must be one of its options, `path` and `text` are passed through
/// untouched (no expansion). A `required` parameter with neither a value nor a
/// default is refused, naming the id. A value keyed by an id that is not
/// declared is refused too — silently dropping it would hide a caller bug, and
/// treating it as free-form would be an injection path.
///
/// Every value is its own `Vec<String>` element. Nothing here ever joins an
/// argument into a shell string, which is the injection defence: the process
/// receives the value as a single `argv` entry and no shell parses it.
///
/// Mutation: join the flag and value (`format!("{flag} {value}")`) or build a
/// shell string and the hostile-value tests receive a split or executed
/// argument and go red.
pub(crate) fn param_arguments(
    params: &[ParamDefinition],
    values: &ParamValues,
    cmd_script: bool,
) -> Result<Vec<String>, String> {
    for key in values.keys() {
        if !params.iter().any(|param| &param.id == key) {
            return Err(format!("run_param_unknown:{key}"));
        }
    }
    let mut args = Vec::new();
    for param in params {
        let provided = values.get(&param.id).filter(|value| !value.is_empty());
        let raw = match provided {
            Some(value) => value.clone(),
            None => match param.default.as_deref().filter(|value| !value.is_empty()) {
                Some(default) => default.to_string(),
                None => {
                    if param.required {
                        return Err(format!("run_param_required:{}", param.id));
                    }
                    continue;
                }
            },
        };
        if let Some(flag) = &param.flag {
            validate_flag(flag).map_err(|_| format!("run_param_invalid:{}", param.id))?;
        }
        if cmd_script && windows_cmd_unsafe_char(&raw).is_some() {
            return Err(format!("run_param_windows_unsafe:{}", param.id));
        }
        match param.kind {
            ParamKind::Boolean => match raw.trim().to_ascii_lowercase().as_str() {
                "true" => {
                    if let Some(flag) = &param.flag {
                        args.push(flag.clone());
                    }
                }
                "false" => {}
                _ => return Err(format!("run_param_invalid:{}", param.id)),
            },
            ParamKind::Number => {
                let trimmed = raw.trim();
                if trimmed.parse::<i64>().is_err() && trimmed.parse::<f64>().is_err() {
                    return Err(format!("run_param_invalid:{}", param.id));
                }
                push_flagged(&mut args, param.flag.as_deref(), trimmed);
            }
            ParamKind::Select => {
                if !param.options.iter().any(|option| option == &raw) {
                    return Err(format!("run_param_invalid:{}", param.id));
                }
                push_flagged(&mut args, param.flag.as_deref(), &raw);
            }
            ParamKind::Text | ParamKind::Path => {
                push_flagged(&mut args, param.flag.as_deref(), &raw);
            }
        }
    }
    Ok(args)
}

/// How long a background run may take before it is killed. Long enough for a
/// real script, short enough that a hung one does not pin the host forever.
const RUN_TIMEOUT: Duration = Duration::from_secs(300);

/// Where a run's output goes. Serialized lowercase so the frontend reads the
/// same word the manifest stores.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum RunRoute {
    Terminal,
    Background,
}

impl From<OutputMode> for RunRoute {
    fn from(mode: OutputMode) -> Self {
        match mode {
            OutputMode::Terminal => Self::Terminal,
            OutputMode::Background => Self::Background,
        }
    }
}

/// Captured stdout/stderr of one background run.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct RunOutput {
    pub stdout: String,
    pub stderr: String,
    /// True when either stream hit [`MAX_RUN_OUTPUT_BYTES`] and was cut.
    pub truncated: bool,
}

/// The outcome of one manual run. `plan` is present only for the terminal
/// route; `output`/`exit_code`/`success` only for the background route.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RunOutcome {
    pub id: String,
    pub route: RunRoute,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub plan: Option<ExecutionPlan>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exit_code: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub success: Option<bool>,
    pub duration_ms: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output: Option<RunOutput>,
}

/// Resolve the route for a run, honouring an explicit per-run override.
fn resolve_route(
    manifest: &ExtensionManifest,
    output_override: Option<&str>,
) -> Result<RunRoute, String> {
    match output_override {
        None => Ok(manifest.output.into()),
        Some(value) => match value.trim().to_ascii_lowercase().as_str() {
            "terminal" => Ok(RunRoute::Terminal),
            "background" => Ok(RunRoute::Background),
            other => Err(format!(
                "Unknown output mode \"{other}\": expected terminal or background"
            )),
        },
    }
}

/// Snapshot the connected entry and its manifest, refusing anything that is
/// not runnable. The check is deliberately the same one the row renders from
/// (connected + enabled + runtime available) so a button that is visible is a
/// button that works.
fn runnable_entry(
    state: &ExtensionState,
    id: &str,
) -> Result<(ExtensionLockEntry, ExtensionManifest), String> {
    let repository = ExtensionsLock::load(&state.paths.repository_file)?;
    let entry = repository.get(id)?.clone();
    if !entry.enabled {
        return Err(format!("Integration {id} is disabled"));
    }
    if entry.state == crate::extensions::lock::ExtensionStateKind::Broken {
        return Err(format!(
            "Integration {id} is broken: {}",
            entry.broken_reason.as_deref().unwrap_or("repair it first")
        ));
    }
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    Ok((entry, manifest))
}

/// The descriptor command a manual run executes.
///
/// For a Floter-generated integration the authority is the shipped
/// `provider-description.json`: its root command carries the `argsPrefix` the
/// connect form produced (and the help probe derived). Anything else falls
/// back to the manifest's provider prefix, which is what the provider protocol
/// itself uses. Either way the resulting argv is
/// `executable_prefix ++ args_prefix` — the same shape
/// [`provider::execution_plan`] builds for the catalog.
fn run_descriptor(entry: &ExtensionLockEntry, args_prefix: Vec<String>) -> CommandDescriptor {
    CommandDescriptor {
        id: "manual.run".into(),
        name: entry.name.clone(),
        description: String::new(),
        aliases: Vec::new(),
        keywords: Vec::new(),
        arguments: Vec::new(),
        execution: ExecutionDescriptor {
            program: "self".into(),
            args_prefix,
            mode: ExecutionMode::Pty,
            working_directory: WorkingDirectory::Current,
        },
    }
}

/// The `argsPrefix` the run must use, read from the static descriptor when the
/// integration ships one. A descriptor that cannot be read is not fatal here:
/// the run then uses the manifest provider prefix, which for a generated
/// integration is empty and for a publisher descriptor is authored.
fn descriptor_args_prefix(entry: &ExtensionLockEntry, manifest: &ExtensionManifest) -> Vec<String> {
    let Some(relative) = manifest.provider.descriptor.as_deref() else {
        return manifest.provider.args_prefix.clone();
    };
    let Some(root) = Path::new(&entry.manifest_path).parent() else {
        return manifest.provider.args_prefix.clone();
    };
    let path = root.join(relative);
    std::fs::read(&path)
        .ok()
        .and_then(|bytes| provider::ProviderDescription::parse(&bytes).ok())
        .and_then(|description| description.commands.first().cloned())
        .map(|command| command.execution.args_prefix)
        .unwrap_or_else(|| manifest.provider.args_prefix.clone())
}

/// The tool data directory is the run's cwd, exactly as `launch` resolves it:
/// a script that writes beside itself must not write into the host's cwd.
fn run_cwd(state: &ExtensionState, id: &str) -> Result<PathBuf, String> {
    let dir = state.paths.data.join(id);
    std::fs::create_dir_all(&dir)
        .map_err(|error| format!("Cannot create tool data dir: {error}"))?;
    Ok(dir)
}

/// Build the argv for one run. Returns the plan **unprotected**; the terminal
/// route protects it before it crosses IPC.
///
/// The parameter args are appended *after* the descriptor's `argsPrefix` (which
/// `execution_plan` already places) and before the free-form user args (none
/// today), so the one argv shape the run can produce is
/// `executable_prefix ++ args_prefix ++ param args`.
fn build_plan(
    entry: &ExtensionLockEntry,
    manifest: &ExtensionManifest,
    values: &ParamValues,
    cwd: &Path,
) -> Result<ExecutionPlan, String> {
    let mut invocation: ProviderInvocation =
        registry::provider_invocation_with_manifest(entry, manifest)?;
    // A run consumes the repository's grants, even if the manifest was edited
    // since approval — the same rule `launch::resolve` applies, so the run
    // cannot out-permission the approval the user actually gave.
    invocation
        .permissions
        .clone_from(&entry.approved_permissions);
    let args_prefix = descriptor_args_prefix(entry, manifest);
    let descriptor = run_descriptor(entry, args_prefix);
    // A `.cmd`/`.bat` program is launched through cmd.exe, where a value's
    // metacharacters can still be re-parsed; the parameter builder refuses such
    // a value up front. Every other program receives structured argv.
    let cmd_script = program_is_cmd_script(&invocation.executable);
    let param_args = if manifest.params.is_empty() && values.is_empty() {
        Vec::new()
    } else {
        param_arguments(&manifest.params, values, cmd_script)?
    };
    provider::execution_plan(&descriptor, &invocation, param_args, Some(cwd))
}

/// Run a connected integration. `values` carries the caller's answers keyed by
/// parameter id (`None` is the same as an empty map: an integration with no
/// parameters runs exactly as before). `output_override` lets the caller force a
/// route for one run (`"terminal"` / `"background"`); `None` uses the
/// manifest's declared `output` mode.
pub async fn run(
    state: &ExtensionState,
    id: &str,
    values: Option<ParamValues>,
    output_override: Option<String>,
) -> Result<RunOutcome, String> {
    let (entry, manifest) = runnable_entry(state, id)?;
    // One run at a time per integration. The guard is held for the whole
    // function, so a second request for the same id is refused while this one
    // is still executing; it releases on every exit path (R9-2 slice 4).
    let _in_flight = state.begin_run(id)?;
    let route = resolve_route(&manifest, output_override.as_deref())?;
    let cwd = run_cwd(state, id)?;
    let values = values.unwrap_or_default();
    let plan = build_plan(&entry, &manifest, &values, &cwd)?;
    match route {
        RunRoute::Terminal => {
            // The frontend gets only the token: program/args/env are stripped
            // by `protect`, so IPC never carries the argv and the frontend
            // never assembles it.
            let protected = state.protect_execution_plan(plan)?;
            Ok(RunOutcome {
                id: id.to_string(),
                route,
                plan: Some(protected),
                exit_code: None,
                success: None,
                duration_ms: 0,
                output: None,
            })
        }
        RunRoute::Background => {
            let started = Instant::now();
            let outcome = execute_background(plan).await;
            let duration_ms = started.elapsed().as_millis() as u64;
            match outcome {
                Ok((success, exit_code, output)) => {
                    state.remember_run_output(id, output.clone());
                    Ok(RunOutcome {
                        id: id.to_string(),
                        route,
                        plan: None,
                        exit_code,
                        success: Some(success),
                        duration_ms,
                        output: Some(output),
                    })
                }
                Err(error) => Err(error),
            }
        }
    }
}

/// Spawn the plan directly (`program` + structured `args`) and capture both
/// streams under a timeout. No shell is involved at any point.
async fn execute_background(plan: ExecutionPlan) -> Result<(bool, Option<i32>, RunOutput), String> {
    let mut command = tokio::process::Command::new(&plan.program);
    command
        .args(&plan.args)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true);
    if let Some(cwd) = &plan.cwd {
        command.current_dir(cwd);
    }
    if !plan.inherit_environment {
        command.env_clear();
    }
    command.envs(&plan.environment);
    let output = command_output(command, RUN_TIMEOUT)
        .await
        .map_err(|error| match error {
            CommandOutputError::TimedOut(timeout) => {
                format!("Run timed out after {} seconds", timeout.as_secs())
            }
            CommandOutputError::Failed(detail) => detail,
        })?;
    Ok((
        output.status.success(),
        output.status.code(),
        RunOutput {
            stdout: String::from_utf8_lossy(&output.stdout).into_owned(),
            stderr: String::from_utf8_lossy(&output.stderr).into_owned(),
            truncated: false,
        },
    ))
}

/// Truncate one stream to the retained cap, reporting whether it was cut.
/// Character-boundary safe so a multi-byte sequence is never split.
pub(crate) fn truncate_stream(value: &str) -> (String, bool) {
    if value.len() <= MAX_RUN_OUTPUT_BYTES {
        return (value.to_string(), false);
    }
    let mut end = MAX_RUN_OUTPUT_BYTES;
    while end > 0 && !value.is_char_boundary(end) {
        end -= 1;
    }
    (value[..end].to_string(), true)
}

/// A session-level store of the most recent background output per integration.
#[derive(Default)]
pub(crate) struct RunOutputStore {
    entries: std::sync::Mutex<BTreeMap<String, RunOutput>>,
}

impl RunOutputStore {
    pub(crate) fn remember(&self, id: &str, output: RunOutput) {
        let (stdout, stdout_cut) = truncate_stream(&output.stdout);
        let (stderr, stderr_cut) = truncate_stream(&output.stderr);
        let record = RunOutput {
            stdout,
            stderr,
            truncated: output.truncated || stdout_cut || stderr_cut,
        };
        if let Ok(mut entries) = self.entries.lock() {
            entries.insert(id.to_string(), record);
        }
    }

    pub(crate) fn get(&self, id: &str) -> Option<RunOutput> {
        self.entries.lock().ok()?.get(id).cloned()
    }

    #[cfg(test)]
    pub(crate) fn len(&self) -> usize {
        self.entries
            .lock()
            .map(|entries| entries.len())
            .unwrap_or(0)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::manifest::validate_flag;
    use crate::extensions::ExtensionPaths;

    fn test_state(root: &Path) -> ExtensionState {
        ExtensionState::from_paths(ExtensionPaths::from_root(root.to_path_buf())).unwrap()
    }

    #[test]
    fn validate_flag_accepts_real_flags_and_rejects_shell_words() {
        for flag in ["--target", "-v", "--dry-run", "--x_1.2"] {
            validate_flag(flag).unwrap_or_else(|error| panic!("{flag} must pass: {error}"));
        }
        for flag in [
            "target",
            "-",
            "--tar get",
            "--tar;rm",
            "--tar|rm",
            "--tar$HOME",
            "--tar\"x\"",
            "--tar&",
            "--tar>out",
            "--tar'x'",
            "",
        ] {
            assert!(
                validate_flag(flag).is_err(),
                "{flag:?} must be rejected by the flag whitelist"
            );
        }
    }

    #[test]
    fn truncate_stream_is_bounded_and_boundary_safe() {
        let short = "ok";
        assert_eq!(truncate_stream(short), ("ok".to_string(), false));

        let exact = "a".repeat(MAX_RUN_OUTPUT_BYTES);
        assert_eq!(truncate_stream(&exact), (exact.clone(), false));

        // A 3-byte character straddling the cap must not be split.
        let mut value = "a".repeat(MAX_RUN_OUTPUT_BYTES - 1);
        value.push('€');
        let (cut, truncated) = truncate_stream(&value);
        assert!(truncated);
        assert_eq!(cut.len(), MAX_RUN_OUTPUT_BYTES - 1);
        assert_eq!(cut, "a".repeat(MAX_RUN_OUTPUT_BYTES - 1));
    }

    #[test]
    fn run_output_store_keeps_the_latest_per_id_and_bounds_each_stream() {
        let store = RunOutputStore::default();
        assert!(store.get("local.a").is_none());
        store.remember(
            "local.a",
            RunOutput {
                stdout: "first".into(),
                stderr: String::new(),
                truncated: false,
            },
        );
        store.remember(
            "local.a",
            RunOutput {
                stdout: "second".into(),
                stderr: "warn".into(),
                truncated: false,
            },
        );
        store.remember(
            "local.b",
            RunOutput {
                stdout: String::new(),
                stderr: String::new(),
                truncated: false,
            },
        );
        assert_eq!(store.len(), 2);
        let record = store.get("local.a").unwrap();
        assert_eq!(record.stdout, "second");
        assert_eq!(record.stderr, "warn");
        assert!(!record.truncated);

        store.remember(
            "local.big",
            RunOutput {
                stdout: "x".repeat(MAX_RUN_OUTPUT_BYTES + 10),
                stderr: String::new(),
                truncated: false,
            },
        );
        let record = store.get("local.big").unwrap();
        assert_eq!(record.stdout.len(), MAX_RUN_OUTPUT_BYTES);
        assert!(record.truncated);
    }

    #[test]
    fn route_prefers_the_override_then_the_manifest() {
        let mut manifest: ExtensionManifest = serde_json::from_str(
            r#"{
                "schemaVersion": "2.0",
                "id": "test.tool",
                "name": "Test Tool",
                "publisher": {"id": "test", "name": "Test"},
                "compatibility": {"floter": ">=0.1.0", "providerProtocol": "^1.0"},
                "distribution": {"type": "local"},
                "runtime": {"type": "system", "executableNames": ["tool"]},
                "provider": {"type": "executable", "argsPrefix": []}
            }"#,
        )
        .unwrap();
        // Absent `output` defaults to background — the compatibility rule.
        assert_eq!(manifest.output, OutputMode::Background);
        assert_eq!(
            resolve_route(&manifest, None).unwrap(),
            RunRoute::Background
        );
        assert_eq!(
            resolve_route(&manifest, Some("terminal")).unwrap(),
            RunRoute::Terminal
        );
        assert_eq!(
            resolve_route(&manifest, Some("Background")).unwrap(),
            RunRoute::Background
        );
        assert!(resolve_route(&manifest, Some("stdout")).is_err());

        manifest.output = OutputMode::Terminal;
        assert_eq!(resolve_route(&manifest, None).unwrap(), RunRoute::Terminal);
        assert_eq!(
            resolve_route(&manifest, Some("background")).unwrap(),
            RunRoute::Background
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn background_run_captures_both_streams_and_records_the_output() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = script_directory.path().join("runner.sh");
        std::fs::write(
            &executable,
            concat!(
                "#!/bin/sh\n",
                "echo 'hello from stdout'\n",
                "echo 'and stderr' >&2\n",
                "exit 3\n"
            ),
        )
        .unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let id = "local.runner";
        let entry = crate::extensions::install::create_custom_integration(
            &state,
            crate::extensions::install::CustomIntegrationRequest {
                id: id.into(),
                name: "Runner".into(),
                command: "runner".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: Vec::new(),
                version_args: Vec::new(),
                description: None,
                permissions: vec![
                    crate::extensions::manifest::Permission::Environment,
                    crate::extensions::manifest::Permission::ProcessSpawn,
                ],
                platforms: vec![
                    crate::extensions::manifest::PlatformTarget::current()
                        .unwrap()
                        .os,
                ],
                output: OutputMode::Background,
                params: Vec::new(),
            },
        )
        .await
        .unwrap();
        assert_eq!(entry.id, id);

        let outcome = run(&state, id, None, None).await.unwrap();
        assert_eq!(outcome.route, RunRoute::Background);
        assert!(outcome.plan.is_none());
        assert_eq!(outcome.exit_code, Some(3));
        assert_eq!(outcome.success, Some(false));
        let output = outcome.output.clone().unwrap();
        assert_eq!(output.stdout, "hello from stdout\n");
        assert_eq!(output.stderr, "and stderr\n");

        // …and the same record is readable from the session store.
        let remembered = state.run_output(id).unwrap();
        assert_eq!(remembered, output);
        assert!(state.run_output("local.absent").is_none());
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn terminal_run_stacks_prefix_then_parameters_in_one_argv() {
        use std::os::unix::fs::PermissionsExt;

        // The one argv shape the run can produce, end to end:
        // `executable_prefix ++ args_prefix ++ <param flag> ++ <param value>`.
        // Every element is a separate item and the params land AFTER the
        // descriptor prefix, which is the order the research report fixes.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = script_directory.path().join("order.sh");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let mut target = param("target", ParamKind::Text);
        target.flag = Some("--target".into());
        let id = "local.order";
        crate::extensions::install::create_custom_integration(
            &state,
            crate::extensions::install::CustomIntegrationRequest {
                id: id.into(),
                name: "Order".into(),
                command: "order".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: vec!["--prefix".into()],
                version_args: Vec::new(),
                description: None,
                permissions: vec![
                    crate::extensions::manifest::Permission::Environment,
                    crate::extensions::manifest::Permission::ProcessSpawn,
                ],
                platforms: vec![
                    crate::extensions::manifest::PlatformTarget::current()
                        .unwrap()
                        .os,
                ],
                output: OutputMode::Terminal,
                params: vec![target],
            },
        )
        .await
        .unwrap();

        let outcome = run(&state, id, Some(values(&[("target", "example.com")])), None)
            .await
            .unwrap();
        let plan = outcome.plan.unwrap();
        let resolved = state
            .take_execution_plan(&plan.plan_token.unwrap())
            .unwrap();
        assert_eq!(
            resolved.args,
            vec![
                "--prefix".to_string(),
                "--target".to_string(),
                "example.com".to_string()
            ]
        );
        assert!(resolved.program.ends_with("order.sh"));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn terminal_run_returns_a_resolvable_protected_plan_without_argv() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = script_directory.path().join("term.sh");
        std::fs::write(&executable, "#!/bin/sh\necho terminal\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let id = "local.terminal";
        crate::extensions::install::create_custom_integration(
            &state,
            crate::extensions::install::CustomIntegrationRequest {
                id: id.into(),
                name: "Terminal".into(),
                command: "terminal".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: vec!["--from-descriptor".into()],
                version_args: Vec::new(),
                description: None,
                permissions: vec![
                    crate::extensions::manifest::Permission::Environment,
                    crate::extensions::manifest::Permission::ProcessSpawn,
                ],
                platforms: vec![
                    crate::extensions::manifest::PlatformTarget::current()
                        .unwrap()
                        .os,
                ],
                output: OutputMode::Terminal,
                params: Vec::new(),
            },
        )
        .await
        .unwrap();

        let outcome = run(&state, id, None, None).await.unwrap();
        assert_eq!(outcome.route, RunRoute::Terminal);
        let plan = outcome.plan.unwrap();
        // The IPC payload carries the token only — no argv, no environment.
        let token = plan.plan_token.clone().expect("plan token");
        assert!(plan.program.is_empty());
        assert!(plan.args.is_empty());
        assert!(plan.environment.is_empty());
        assert!(plan.cwd.is_none());

        // The protected plan resolves back to the real argv, which is the
        // executable followed by the descriptor's argsPrefix as SEPARATE items.
        let resolved = state.take_execution_plan(&token).unwrap();
        assert_eq!(resolved.args, vec!["--from-descriptor".to_string()]);
        assert!(resolved.program.ends_with("term.sh"));
    }

    // ── R9-2 slice 3 · the parameter assembler ───────────────────────────

    fn param(id: &str, kind: ParamKind) -> ParamDefinition {
        ParamDefinition {
            id: id.into(),
            label: String::new(),
            kind,
            default: None,
            required: false,
            placeholder: None,
            options: Vec::new(),
            flag: None,
        }
    }

    fn values(pairs: &[(&str, &str)]) -> ParamValues {
        pairs
            .iter()
            .map(|(key, value)| (key.to_string(), value.to_string()))
            .collect()
    }

    fn args_for(
        param: &ParamDefinition,
        pairs: &[(&str, &str)],
        cmd_script: bool,
    ) -> Result<Vec<String>, String> {
        param_arguments(std::slice::from_ref(param), &values(pairs), cmd_script)
    }

    #[test]
    fn param_arguments_follow_the_declared_order_with_flags_and_positionals() {
        // Declaration order is argv order, and a flag becomes its OWN item —
        // never `--flag value` joined into one. Mutation: join flag and value
        // (`format!("{flag} {value}")`) and the assertions collapse.
        let mut flagged = param("target", ParamKind::Text);
        flagged.flag = Some("--target".into());
        let positional = param("rest", ParamKind::Text);
        let args = param_arguments(
            &[flagged, positional],
            &values(&[("target", "example.com"), ("rest", "extra")]),
            false,
        )
        .unwrap();
        assert_eq!(args, vec!["--target", "example.com", "extra"]);
        // Each value stays exactly one item even when it contains spaces.
        let mut spaced = param("target", ParamKind::Text);
        spaced.flag = Some("--target".into());
        let args = param_arguments(&[spaced], &values(&[("target", "two words")]), false).unwrap();
        assert_eq!(args, vec!["--target", "two words"]);
    }

    #[test]
    fn param_arguments_skips_absent_optionals_and_refuses_missing_required() {
        let optional = param("note", ParamKind::Text);
        assert!(args_for(&optional, &[], false).unwrap().is_empty());
        // A default seeds the value, so an absent optional with a default is
        // present.
        let mut defaulted = optional.clone();
        defaulted.default = Some("hi".into());
        assert_eq!(
            param_arguments(&[defaulted], &ParamValues::new(), false).unwrap(),
            vec!["hi"]
        );
        // A required parameter with no value and no default is refused, and the
        // error names the id (the frontend maps it to a localised message).
        let mut required = param("note", ParamKind::Text);
        required.required = true;
        let error = param_arguments(&[required.clone()], &ParamValues::new(), false).unwrap_err();
        assert_eq!(error, "run_param_required:note");
        // …but a supplied value satisfies it.
        assert_eq!(
            param_arguments(&[required.clone()], &values(&[("note", "x")]), false).unwrap(),
            vec!["x"]
        );
        // An empty string is "not supplied": a required value cannot be blank.
        let error = param_arguments(&[required], &values(&[("note", "")]), false).unwrap_err();
        assert_eq!(error, "run_param_required:note");
    }

    #[test]
    fn param_arguments_converts_by_kind() {
        // number: integer and float spellings are both accepted, trimmed.
        let count = param("count", ParamKind::Number);
        assert_eq!(
            args_for(&count, &[("count", "12")], false).unwrap(),
            vec!["12"]
        );
        assert_eq!(
            args_for(&count, &[("count", " -3.5 ")], false).unwrap(),
            vec!["-3.5"]
        );
        assert_eq!(
            param_arguments(&[count], &values(&[("count", "three")]), false).unwrap_err(),
            "run_param_invalid:count"
        );
        // select: the value must be one of the options.
        let mut mode = param("mode", ParamKind::Select);
        mode.options = vec!["fast".into(), "slow".into()];
        assert_eq!(
            param_arguments(&[mode.clone()], &values(&[("mode", "fast")]), false).unwrap(),
            vec!["fast"]
        );
        assert_eq!(
            param_arguments(&[mode], &values(&[("mode", "turbo")]), false).unwrap_err(),
            "run_param_invalid:mode"
        );
        // boolean: true pushes the flag (and only the flag); false/absent push
        // nothing. A flagless true is a no-op, not a `"true"` item.
        let mut force = param("force", ParamKind::Boolean);
        force.flag = Some("--force".into());
        assert_eq!(
            param_arguments(&[force.clone()], &values(&[("force", "true")]), false).unwrap(),
            vec!["--force"]
        );
        assert!(
            param_arguments(&[force.clone()], &values(&[("force", "false")]), false)
                .unwrap()
                .is_empty()
        );
        assert!(param_arguments(&[force], &ParamValues::new(), false)
            .unwrap()
            .is_empty());
        // path and text pass through untouched — no `$HOME`/`~` expansion.
        let path = param("dest", ParamKind::Path);
        assert_eq!(
            param_arguments(&[path], &values(&[("dest", "$HOME/x ~/y")]), false).unwrap(),
            vec!["$HOME/x ~/y"]
        );
    }

    #[test]
    fn param_arguments_refuses_an_undeclared_value_key() {
        // Mutation: drop the unknown-key loop and this silently ignores the
        // value instead of refusing it.
        let error = param_arguments(
            &[param("known", ParamKind::Text)],
            &values(&[("known", "ok"), ("sneaky", "--rm")]),
            false,
        )
        .unwrap_err();
        assert_eq!(error, "run_param_unknown:sneaky");
    }

    #[test]
    fn param_arguments_refuses_a_malformed_flag_at_run_time() {
        // `validate_flag` is the manifest single source; the assembler re-runs
        // it so a hand-edited manifest cannot smuggle a space (two argv items)
        // through the run path.
        let mut bad = param("target", ParamKind::Text);
        bad.flag = Some("--target value".into());
        assert_eq!(
            param_arguments(&[bad], &values(&[("target", "x")]), false).unwrap_err(),
            "run_param_invalid:target"
        );
    }

    #[test]
    fn param_arguments_refuses_cmd_metacharacters_for_a_cmd_script() {
        // The Windows `.cmd`/`.bat` hole: the program is launched through
        // cmd.exe, where `& | < > ^ %` are re-parsed even though the value
        // arrived as its own argv entry. The run refuses such a value.
        // Mutation: drop `%` from `WINDOWS_CMD_UNSAFE_CHARS` and the last case
        // below goes green (the value is passed through).
        let text = param("target", ParamKind::Text);
        for value in ["a&b", "a|b", "a<b", "a>b", "a^b", "a%b"] {
            assert_eq!(
                args_for(&text, &[("target", value)], true).unwrap_err(),
                "run_param_windows_unsafe:target",
                "{value:?} must be refused for a cmd script"
            );
            // …and the SAME value is fine for a non-cmd program: the set is
            // platform/extension-scoped, not a blanket rejection.
            assert_eq!(
                args_for(&text, &[("target", value)], false).unwrap(),
                vec![value]
            );
        }
        // The check covers flag+value; a metacharacter anywhere refuses the run.
        let mut flagged = param("target", ParamKind::Text);
        flagged.flag = Some("--target".into());
        assert_eq!(
            param_arguments(&[flagged], &values(&[("target", "ok%not-ok")]), true).unwrap_err(),
            "run_param_windows_unsafe:target"
        );
        // A non-cmd program is never wrapped, so `program_is_cmd_script` must
        // be false for it on every platform (off Windows it is always false).
        assert!(!program_is_cmd_script(Path::new("runner.exe")));
        assert!(!program_is_cmd_script(Path::new("runner.sh")));
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn background_run_passes_arguments_as_separate_argv_items_not_a_shell_string() {
        use std::os::unix::fs::PermissionsExt;

        // The injection defense. Every argument below is a shell word that
        // WOULD change meaning if the argv were ever joined into one string:
        // a space-separated pair, a command substitution, and a `;` chain.
        // The script prints each argument on its own line, so the assertion is
        // about what the process actually received, not about the plan.
        //
        // Mutation: build the command with `sh -c "$program $args"` (or join
        // `args` with spaces) and `$(...)` executes, `;` starts a second
        // command, and `"a b"` collapses into two arguments — all three
        // assertions below go red.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let marker = script_directory
            .path()
            .join("shell-would-have-touched-this");
        let executable = script_directory.path().join("argv.sh");
        std::fs::write(
            &executable,
            concat!(
                "#!/bin/sh\n",
                "for arg in \"$@\"; do printf '[%s]\\n' \"$arg\"; done\n"
            ),
        )
        .unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let hostile = vec![
            "a b".to_string(),
            "$(echo pwned)".to_string(),
            format!("; touch {}", marker.display()),
            "--flag=value with spaces".to_string(),
        ];
        let id = "local.argv-defense";
        crate::extensions::install::create_custom_integration(
            &state,
            crate::extensions::install::CustomIntegrationRequest {
                id: id.into(),
                name: "Argv defense".into(),
                command: "argv-defense".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: hostile.clone(),
                version_args: Vec::new(),
                description: None,
                permissions: vec![
                    crate::extensions::manifest::Permission::Environment,
                    crate::extensions::manifest::Permission::ProcessSpawn,
                ],
                platforms: vec![
                    crate::extensions::manifest::PlatformTarget::current()
                        .unwrap()
                        .os,
                ],
                output: OutputMode::Background,
                params: Vec::new(),
            },
        )
        .await
        .unwrap();

        let outcome = run(&state, id, None, None).await.unwrap();
        assert_eq!(outcome.success, Some(true), "{outcome:?}");
        let stdout = outcome.output.unwrap().stdout;
        let lines: Vec<&str> = stdout.lines().collect();
        assert_eq!(lines.len(), hostile.len(), "{stdout}");
        for (line, expected) in lines.iter().zip(&hostile) {
            assert_eq!(
                line,
                &format!("[{expected}]"),
                "each argument must survive intact"
            );
        }
        // No substitution ran and no `;` split anything: the `$(...)` string
        // arrived literally, and the marker the `;` branch would have created
        // was never touched.
        assert!(
            !marker.exists(),
            "the `;` argument must not have started a second command"
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn background_run_passes_parameter_values_as_separate_argv_items() {
        use std::os::unix::fs::PermissionsExt;

        // The run-time half of the injection defense: a value a user typed into
        // the parameter form is a single argv item, even when it is a shell
        // word. The declared parameter has a flag, so the process must receive
        // `--target` and the value as TWO items (never `--target=<value>` and
        // never one joined string).
        //
        // Mutation: join the value into the flag (or build a shell string) and
        // the `$(...)` runs, the `;` chain touches the marker, and the argv
        // count no longer matches.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let marker = script_directory.path().join("param-shell-marker");
        let executable = script_directory.path().join("params.sh");
        std::fs::write(
            &executable,
            concat!(
                "#!/bin/sh\n",
                "for arg in \"$@\"; do printf '[%s]\\n' \"$arg\"; done\n"
            ),
        )
        .unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let hostile_value = format!("$(touch {}) ; rm -rf / with spaces", marker.display());
        let mut target = param("target", ParamKind::Text);
        target.flag = Some("--target".into());
        let id = "local.param-defense";
        crate::extensions::install::create_custom_integration(
            &state,
            crate::extensions::install::CustomIntegrationRequest {
                id: id.into(),
                name: "Param defense".into(),
                command: "param-defense".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: Vec::new(),
                version_args: Vec::new(),
                description: None,
                permissions: vec![
                    crate::extensions::manifest::Permission::Environment,
                    crate::extensions::manifest::Permission::ProcessSpawn,
                ],
                platforms: vec![
                    crate::extensions::manifest::PlatformTarget::current()
                        .unwrap()
                        .os,
                ],
                output: OutputMode::Background,
                params: vec![target],
            },
        )
        .await
        .unwrap();

        let outcome = run(
            &state,
            id,
            Some(values(&[("target", hostile_value.as_str())])),
            None,
        )
        .await
        .unwrap();
        assert_eq!(outcome.success, Some(true), "{outcome:?}");
        let stdout = outcome.output.unwrap().stdout;
        let lines: Vec<&str> = stdout.lines().collect();
        // Exactly flag + value, as two items, with the value intact.
        let expected = ["--target".to_string(), hostile_value.clone()];
        assert_eq!(lines.len(), expected.len(), "{stdout}");
        for (line, item) in lines.iter().zip(&expected) {
            assert_eq!(line, &format!("[{item}]"), "{stdout}");
        }
        assert!(
            !marker.exists(),
            "the `;` in the parameter value must not have started a second command"
        );
    }

    #[tokio::test]
    async fn run_rejects_a_disconnected_or_disabled_integration() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let error = run(&state, "local.missing", None, None).await.unwrap_err();
        assert!(error.contains("local.missing"), "{error}");
    }

    // ── R9-2 slice 4 · one run at a time per integration ─────────────────

    #[test]
    fn run_in_flight_refuses_a_second_claim_and_releases_on_drop() {
        // Mutation: make `begin` return a guard without inserting (or drop the
        // insert) and the second claim below succeeds instead of refusing.
        let registry = RunInFlight::default();
        assert!(!registry.is_active("local.a"));
        let first = registry.begin("local.a").expect("first claim");
        assert!(registry.is_active("local.a"));
        assert_eq!(
            registry.begin("local.a").unwrap_err(),
            "run_already_in_flight:local.a",
        );
        // A different integration is never blocked by another's run.
        let other = registry.begin("local.b").expect("unrelated claim");
        assert!(registry.is_active("local.b"));
        // Dropping the guard releases exactly its own slot.
        drop(first);
        assert!(!registry.is_active("local.a"));
        assert!(registry.is_active("local.b"));
        assert!(registry.begin("local.a").is_ok());
        drop(other);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn a_second_concurrent_run_of_the_same_integration_is_refused() {
        use std::os::unix::fs::PermissionsExt;

        // End to end: the first run is still executing (it sleeps) when the
        // second request arrives for the same id, so the second is refused
        // with the stable key. Mutation: drop the `begin_run` call in `run`
        // and both futures succeed instead of one failing.
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let script_directory = tempfile::tempdir().unwrap();
        let executable = script_directory.path().join("slow.sh");
        std::fs::write(&executable, "#!/bin/sh\nsleep 1\necho done\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        let id = "local.slow";
        crate::extensions::install::create_custom_integration(
            &state,
            crate::extensions::install::CustomIntegrationRequest {
                id: id.into(),
                name: "Slow".into(),
                command: "slow".into(),
                version: "1.0.0".into(),
                executable_path: executable.to_string_lossy().into_owned(),
                mode: "executable".into(),
                script_language: None,
                script_content: None,
                args_prefix: Vec::new(),
                version_args: Vec::new(),
                description: None,
                permissions: vec![
                    crate::extensions::manifest::Permission::Environment,
                    crate::extensions::manifest::Permission::ProcessSpawn,
                ],
                platforms: vec![
                    crate::extensions::manifest::PlatformTarget::current()
                        .unwrap()
                        .os,
                ],
                output: OutputMode::Background,
                params: Vec::new(),
            },
        )
        .await
        .unwrap();

        let (first, second) =
            tokio::join!(run(&state, id, None, None), run(&state, id, None, None),);
        // Exactly one future wins the slot; the other is refused with the
        // stable key.
        let refusals: Vec<String> = [first, second]
            .into_iter()
            .filter_map(Result::err)
            .collect();
        assert_eq!(
            refusals.len(),
            1,
            "exactly one of the two runs must be refused"
        );
        assert_eq!(refusals[0], format!("run_already_in_flight:{id}"));
        // The slot is released once the surviving run finishes, so the next
        // run is not blocked by a stale mark.
        assert!(!state.run_in_flight(id));
        assert!(run(&state, id, None, None).await.is_ok());
    }
}
