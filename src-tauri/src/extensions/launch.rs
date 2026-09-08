use super::cwd_policy::{CwdContext, CwdPolicy};
use super::lifecycle::{LaunchConfig, TerminalRequirement};
use super::lock::{ExtensionLockEntry, ExtensionsLock};
use super::manifest::{ExtensionManifest, Permission};
use super::provider::{
    self, CommandDescriptor, ExecutionDescriptor, ExecutionMode, ExecutionPlan, ProviderInvocation,
    WorkingDirectory,
};
use super::session_restore::{
    ResolvedSession, RestorePolicy, SessionResolveRequest, SessionResolver,
};
use super::{probe_executor, registry, ExtensionState};
use serde_json::{json, Value};
use std::collections::BTreeMap;
use std::path::Path;

#[cfg(all(test, unix))]
mod tests;

/// Resolve both new and restored launches. Undeclared launches retain the v1
/// response; declared launches also carry a protected term_spawn execution plan.
pub(crate) async fn resolve(
    state: &ExtensionState,
    id: &str,
    argv: Vec<String>,
    cwd: Option<&str>,
) -> Result<Value, String> {
    let _guard = state.mutation_lock.lock().await;
    let mut repository = ExtensionsLock::load(&state.paths.repository_file)?;
    let entry = repository.get(id)?.clone();
    let manifest = match ExtensionManifest::load(Path::new(&entry.manifest_path)) {
        Ok(manifest) => Some(manifest),
        Err(error) => {
            tracing::warn!(
                extension_id = %id, manifest_path = %entry.manifest_path, error = %error,
                "Manifest load failed during launch; using v1 defaults"
            );
            None
        }
    };

    let tool_data_dir = state.paths.data.join(id);
    std::fs::create_dir_all(&tool_data_dir)
        .map_err(|error| format!("Cannot create tool data dir: {error}"))?;
    let context = CwdContext::new(
        cwd.map(Path::new),
        &tool_data_dir,
        entry
            .approved_permissions
            .contains(&Permission::FilesystemRead)
            || entry
                .approved_permissions
                .contains(&Permission::FilesystemWrite),
    );
    let declared = manifest.as_ref().and_then(|manifest| {
        manifest.lifecycle.launch.as_ref().and_then(|config| {
            match declared_execution(&entry, manifest, config, &argv, &context) {
                Ok((execution, invocation)) => Some((config, execution, invocation)),
                Err(error) => {
                    tracing::warn!(
                        extension_id = %id, error = %error,
                        "Invalid lifecycle.launch declaration; using v1 defaults"
                    );
                    None
                }
            }
        })
    });

    let (config, execution) = match declared {
        Some((config, execution, invocation)) => {
            // Use exactly the lifecycle set and invocation that install/repair
            // use. No version/help probes are invented for legacy launches.
            if let Some(manifest) = &manifest {
                if !manifest.lifecycle.probes.is_empty() {
                    let report =
                        probe_executor::execute_capability_probes(&invocation, manifest).await;
                    let problem = probe_executor::verification_error(&report);
                    probe_executor::record_report(&mut repository, id, report)?;
                    repository.save(&state.paths.repository_file)?;
                    state.invalidate_provider_commands().await;
                    if let Some(problem) = problem {
                        return Err(problem);
                    }
                }
            }
            (Some(config), Some(execution))
        }
        None => (None, None),
    };
    let resolved_cwd = match &execution {
        Some(execution) => execution
            .cwd
            .as_deref()
            .map(Path::new)
            .unwrap_or(&tool_data_dir)
            .to_path_buf(),
        None => CwdPolicy::InheritActiveSession.resolve(&context)?,
    };
    let restore_policy = match config.map(|config| config.restore_policy.as_str()) {
        Some("restart") => RestorePolicy::Restart,
        Some("none") => RestorePolicy::None,
        _ => RestorePolicy::Reattach,
    };
    let session_resolver = SessionResolver::new(tool_data_dir.join("sessions"));
    let session = session_resolver.resolve(SessionResolveRequest {
        tool_id: id.into(),
        tool_version: entry.package_version.clone(),
        argv: argv.clone(),
        cwd: resolved_cwd.clone(),
        environment_refs: vec!["profile:default".into()],
        terminal_profile: None,
        restore_policy,
    })?;
    let is_restart = matches!(&session, ResolvedSession::Restart(_));
    let session = match session {
        ResolvedSession::Reattach(session)
        | ResolvedSession::Restart(session)
        | ResolvedSession::New(session) => session,
    };
    session_resolver.write_session(&session)?;

    let terminal = config.and_then(|config| config.terminal.as_ref());
    let mut plan = json!({
        "sessionId": session.session_id,
        "toolId": id,
        "version": entry.package_version,
        "argv": argv,
        "cwd": resolved_cwd.to_string_lossy(),
        "isRestart": is_restart,
        "terminal": terminal_config(terminal),
        "environment": terminal_environment(terminal),
    });
    if let Some(execution) = execution {
        plan["execution"] = json!(state.protect_execution_plan(execution)?);
    }
    Ok(plan)
}

fn declared_execution(
    entry: &ExtensionLockEntry,
    manifest: &ExtensionManifest,
    config: &LaunchConfig,
    argv: &[String],
    context: &CwdContext<'_>,
) -> Result<(ExecutionPlan, ProviderInvocation), String> {
    let cwd = CwdPolicy::from_manifest(&config.cwd_policy)?
        .resolve(context)
        .map_err(|error| format!("lifecycle.launch.cwdPolicy: {error}"))?;
    let mut invocation = registry::provider_invocation_with_manifest(entry, manifest)
        .map_err(|error| format!("lifecycle.launch.command: {error}"))?;
    // Launch consumes the repository's grants, even if the manifest was edited
    // since approval. The registry still supplies platform/runtime resolution.
    invocation
        .permissions
        .clone_from(&entry.approved_permissions);
    let descriptor = CommandDescriptor {
        id: "lifecycle.launch.command".into(),
        name: entry.name.clone(),
        description: String::new(),
        aliases: Vec::new(),
        keywords: Vec::new(),
        arguments: Vec::new(),
        execution: ExecutionDescriptor {
            program: config
                .command
                .as_ref()
                .map(|command| command.program.clone())
                .unwrap_or_else(|| "self".into()),
            args_prefix: config
                .command
                .as_ref()
                .map(|command| command.args.clone())
                .unwrap_or_default(),
            mode: ExecutionMode::Pty,
            working_directory: WorkingDirectory::Current,
        },
    };
    let mut execution =
        provider::execution_plan(&descriptor, &invocation, argv.to_vec(), Some(&cwd))
            .map_err(|error| format!("lifecycle.launch.command: {error}"))?;
    execution
        .environment
        .extend(terminal_environment(config.terminal.as_ref()));
    Ok((execution, invocation))
}

fn terminal_environment(terminal: Option<&TerminalRequirement>) -> BTreeMap<String, String> {
    let (term, colorterm) = match terminal.map(|terminal| terminal.color.as_str()) {
        Some("none") => ("dumb", ""),
        Some("8") => ("xterm", ""),
        Some("256") => ("floter-256color", ""),
        _ => ("floter-256color", "truecolor"),
    };
    BTreeMap::from([
        ("TERM".into(), term.into()),
        ("COLORTERM".into(), colorterm.into()),
        ("TERM_PROGRAM".into(), "floter".into()),
    ])
}

fn terminal_config(terminal: Option<&TerminalRequirement>) -> Value {
    match terminal {
        Some(terminal) => json!({
            "required": terminal.required,
            "color": terminal.color,
            "unicode": terminal.unicode,
            "bracketedPaste": terminal.bracketed_paste,
            "synchronizedOutput": if terminal.synchronized_output { "required" } else { "preferred" },
            "keyboardProtocol": terminal.keyboard_protocol.as_deref().unwrap_or("kitty-preferred"),
            "mouse": terminal.mouse,
        }),
        None => json!({
            "required": true,
            "color": "truecolor",
            "unicode": true,
            "bracketedPaste": true,
            "synchronizedOutput": "preferred",
            "keyboardProtocol": "kitty-preferred",
        }),
    }
}
