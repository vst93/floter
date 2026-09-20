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
use super::manifest::{ExtensionManifest, OutputMode};
use super::process_cleanup::{command_output, CommandOutputError};
use super::provider::{
    self, CommandDescriptor, ExecutionDescriptor, ExecutionMode, ExecutionPlan, ProviderInvocation,
    WorkingDirectory,
};
use super::registry;
use super::ExtensionState;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::{Duration, Instant};

/// Upper bound on the bytes retained per stream in the "last output" record.
/// Matches the probe cap (`capability_probe::MAX_PROBE_OUTPUT_BYTES`): a
/// runaway script must not be able to grow the host's memory without bound.
pub(crate) const MAX_RUN_OUTPUT_BYTES: usize = 64 * 1024;

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
fn build_plan(
    entry: &ExtensionLockEntry,
    manifest: &ExtensionManifest,
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
    provider::execution_plan(&descriptor, &invocation, Vec::new(), Some(cwd))
}

/// Run a connected integration. `output_override` lets the caller force a
/// route for one run (`"terminal"` / `"background"`); `None` uses the
/// manifest's declared `output` mode.
pub async fn run(
    state: &ExtensionState,
    id: &str,
    output_override: Option<String>,
) -> Result<RunOutcome, String> {
    let (entry, manifest) = runnable_entry(state, id)?;
    let route = resolve_route(&manifest, output_override.as_deref())?;
    let cwd = run_cwd(state, id)?;
    let plan = build_plan(&entry, &manifest, &cwd)?;
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

        let outcome = run(&state, id, None).await.unwrap();
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

        let outcome = run(&state, id, None).await.unwrap();
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

        let outcome = run(&state, id, None).await.unwrap();
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

    #[tokio::test]
    async fn run_rejects_a_disconnected_or_disabled_integration() {
        let directory = tempfile::tempdir().unwrap();
        let state = test_state(directory.path());
        let error = run(&state, "local.missing", None).await.unwrap_err();
        assert!(error.contains("local.missing"), "{error}");
    }
}
