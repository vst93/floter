use super::*;
use crate::commands::terminal::TerminalExecutionPlan;
use crate::extensions::health::HealthStatus;
use crate::extensions::manifest::PlatformTarget;
use crate::extensions::ExtensionPaths;
use std::io::Write;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tracing::instrument::WithSubscriber;

const ID: &str = "test.launch";

struct Fixture {
    root: tempfile::TempDir,
    state: ExtensionState,
    manifest: Value,
    manifest_path: PathBuf,
    executable: PathBuf,
    record: PathBuf,
    probes: PathBuf,
}

impl Fixture {
    fn new(script: bool) -> Self {
        let root = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(root.path().join("state")))
                .unwrap();
        let executable =
            Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/launch-record.sh");
        crate::extensions::install::make_executable(&executable).unwrap();
        let manifest_path = root.path().join("floter.extension.json");
        let record = root.path().join("spawn-record");
        let probes = root.path().join("probe-record");
        let runtime = if script {
            // The interpreter reads this copy; no newly written inode is exec'd.
            std::fs::write(
                root.path().join("tool.sh"),
                include_bytes!("../../../tests/fixtures/launch-record.sh"),
            )
            .unwrap();
            json!({"type": "script", "language": "shell", "path": "tool.sh"})
        } else {
            json!({"type": "system", "executableNames": ["launch-record.sh"]})
        };
        let manifest = json!({
            "schemaVersion": "2.0", "id": ID, "name": "Launch fixture",
            "publisher": {"id": "test", "name": "Test"},
            "compatibility": {"floter": ">=0.1.0", "providerProtocol": "^1.0"},
            "distribution": {"type": "local"}, "runtime": runtime,
            "provider": {
                "type": "executable", "argsPrefix": ["--provider-only"],
                "environment": {
                    "FLOTER_LAUNCH_RECORD": record, "FLOTER_LAUNCH_PROBES": probes,
                    "FLOTER_LAUNCH_VALUE": "base"
                }
            },
            "platformOverrides": {
                PlatformTarget::current().unwrap().identifier(): {
                    "providerArgsPrefix": ["--platform-provider-only"],
                    "environment": {"FLOTER_LAUNCH_VALUE": "platform"}
                }
            },
            "lifecycle": {"launch": {
                "command": {"program": "self", "args": ["--launch", "declared arg"]},
                "cwdPolicy": "toolData", "restorePolicy": "restart",
                "terminal": {"required": false, "color": "256", "unicode": false,
                    "mouse": "sgr", "bracketedPaste": true, "synchronizedOutput": true,
                    "keyboardProtocol": "kitty-strict"}
            }}
        });
        let entry: ExtensionLockEntry = serde_json::from_value(json!({
            "id": ID, "name": "Launch fixture", "publisherId": "test", "publisherName": "Test",
            "distributionSource": "local", "runtimeOwnership": "system",
            "providerKind": "executable", "state": "enabled", "enabled": true,
            "packageVersion": "1.2.3", "currentVersion": "1.2.3",
            "manifestPath": manifest_path,
            "executablePath": if script { root.path().join("must-not-execute-raw-path") } else { executable.clone() },
            "installedAt": 1, "updatedAt": 1
        })).unwrap();
        let mut repository = ExtensionsLock::default();
        repository.extensions.insert(ID.into(), entry);
        repository.save(&state.paths.repository_file).unwrap();
        let fixture = Self {
            root,
            state,
            manifest,
            manifest_path,
            executable,
            record,
            probes,
        };
        fixture.save();
        fixture
    }

    fn save(&self) {
        std::fs::write(
            &self.manifest_path,
            serde_json::to_vec(&self.manifest).unwrap(),
        )
        .unwrap();
    }

    fn entry(&self) -> ExtensionLockEntry {
        ExtensionsLock::load(&self.state.paths.repository_file)
            .unwrap()
            .get(ID)
            .unwrap()
            .clone()
    }

    fn data_dir(&self) -> PathBuf {
        self.state.paths.data.join(ID)
    }

    fn session_path(&self) -> PathBuf {
        self.data_dir()
            .join("sessions")
            .join(format!("{ID}.session.json"))
    }

    async fn launch(&self) -> Value {
        resolve(&self.state, ID, user_args(), self.root.path().to_str())
            .await
            .unwrap()
    }

    async fn logged_launch(&self) -> (Result<Value, String>, String) {
        let logs = Logs::default();
        let writer = logs.clone();
        let subscriber = tracing_subscriber::fmt()
            .with_max_level(tracing::Level::WARN)
            .with_ansi(false)
            .without_time()
            .with_writer(move || writer.clone())
            .finish();
        let result = resolve(&self.state, ID, user_args(), self.root.path().to_str())
            .with_subscriber(subscriber)
            .await;
        let text = String::from_utf8(logs.0.lock().unwrap().clone()).unwrap();
        (result, text)
    }

    async fn spawn(&self, plan: &Value) -> Vec<String> {
        assert!(plan["execution"]["environment"]
            .as_object()
            .unwrap()
            .is_empty());
        let execution: TerminalExecutionPlan =
            serde_json::from_value(plan["execution"].clone()).unwrap();
        let (cwd, command) = execution.resolve(&self.state).unwrap();
        let payload = serde_json::to_vec(&command).unwrap();
        // The same payload consumer and PTY spawn used by broker New requests,
        // without an IPC socket or a second implementation of process spawning.
        let session = qscreen_daemon::session::Session::new_with_cwd_and_command(
            uuid::Uuid::new_v4().to_string(),
            "launch-fixture".into(),
            80,
            24,
            None,
            cwd.as_deref(),
            Some(serde_json::from_slice(&payload).unwrap()),
        )
        .unwrap();
        let mut exited = session.subscribe_exit();
        let result = tokio::time::timeout(Duration::from_secs(10), async {
            if !*exited.borrow() {
                exited.changed().await.unwrap();
            }
        })
        .await;
        session.close();
        result.unwrap();
        assert_eq!(*session.exit_code.lock().unwrap(), Some(0));
        let bytes = std::fs::read(&self.record).unwrap();
        bytes
            .strip_suffix(&[0])
            .unwrap()
            .split(|byte| *byte == 0)
            .map(|part| String::from_utf8(part.to_vec()).unwrap())
            .collect()
    }
}

#[derive(Clone, Default)]
struct Logs(Arc<Mutex<Vec<u8>>>);

impl Write for Logs {
    fn write(&mut self, bytes: &[u8]) -> std::io::Result<usize> {
        self.0.lock().unwrap().extend_from_slice(bytes);
        Ok(bytes.len())
    }
    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}

fn user_args() -> Vec<String> {
    vec!["user arg".into(), "$(touch must-not-exist); *".into()]
}

fn assert_v1(plan: &Value, cwd: &Path) {
    // Golden response from f5976c9; only its nondeterministic UUID is supplied
    // by the result. Compare serialized bytes, including absent optional keys.
    uuid::Uuid::parse_str(plan["sessionId"].as_str().unwrap()).unwrap();
    let expected = json!({
        "sessionId": plan["sessionId"], "toolId": ID, "version": "1.2.3",
        "argv": user_args(), "cwd": cwd, "isRestart": false,
        "terminal": {"required": true, "color": "truecolor", "unicode": true,
            "bracketedPaste": true, "synchronizedOutput": "preferred", "keyboardProtocol": "kitty-preferred"},
        "environment": {"TERM": "floter-256color", "COLORTERM": "truecolor", "TERM_PROGRAM": "floter"}
    });
    assert_eq!(
        serde_json::to_vec(plan).unwrap(),
        serde_json::to_vec(&expected).unwrap()
    );
}

#[tokio::test]
async fn declared_system_command_cwd_and_terminal_reach_real_pty_spawn() {
    let fixture = Fixture::new(false);
    let repository = std::fs::read(&fixture.state.paths.repository_file).unwrap();
    let plan = fixture.launch().await;
    assert_eq!(
        plan["terminal"],
        json!({"required": false, "color": "256", "unicode": false,
        "mouse": "sgr", "bracketedPaste": true, "synchronizedOutput": "required", "keyboardProtocol": "kitty-strict"})
    );
    let record = fixture.spawn(&plan).await;
    assert_eq!(
        record,
        vec![
            fixture.executable.to_string_lossy().into_owned(),
            fixture.data_dir().to_string_lossy().into_owned(),
            "floter-256color".into(),
            "".into(),
            "floter".into(),
            "platform".into(),
            "--launch".into(),
            "declared arg".into(),
            user_args()[0].clone(),
            user_args()[1].clone()
        ]
    );
    assert_eq!(
        std::fs::read(&fixture.state.paths.repository_file).unwrap(),
        repository
    );
    assert!(!fixture.data_dir().join("must-not-exist").exists());
}

#[tokio::test]
async fn script_launch_and_verification_share_registry_invocation() {
    let mut fixture = Fixture::new(true);
    fixture.manifest["lifecycle"]["probes"] = json!([
        {"id": "health", "args": ["--health"], "required": true},
        {"id": "optional", "args": ["--optional"]}
    ]);
    fixture.save();
    assert!(!Path::new(&fixture.entry().executable_path).exists());
    let plan = fixture.launch().await;
    let record = fixture.spawn(&plan).await;
    assert_eq!(
        record[0],
        fixture.root.path().join("tool.sh").to_string_lossy()
    );
    assert_eq!(
        &record[6..],
        &[vec!["--launch".into(), "declared arg".into()], user_args()].concat()
    );
    assert_eq!(record[5], "platform");
    assert_eq!(
        std::fs::read_to_string(&fixture.probes).unwrap(),
        "--health|platform\n--optional|platform\n"
    );
    let report = fixture.entry().probe_report.unwrap();
    assert_eq!(report.status, HealthStatus::Healthy);
    assert_eq!(
        report
            .probes
            .iter()
            .map(|probe| probe.probe_id.as_str())
            .collect::<Vec<_>>(),
        ["health", "optional"]
    );
    assert!(!fixture.data_dir().join("health.json").exists());
}

#[tokio::test]
async fn undeclared_launch_is_byte_identical_and_does_not_probe() {
    let mut fixture = Fixture::new(false);
    fixture.manifest["lifecycle"]
        .as_object_mut()
        .unwrap()
        .remove("launch");
    fixture.manifest["lifecycle"]["probes"] =
        json!([{"id": "unused", "args": ["--health"], "required": true}]);
    fixture.save();
    let (plan, logs) = fixture.logged_launch().await;
    let plan = plan.unwrap();
    assert_v1(&plan, fixture.root.path());
    let second = fixture.launch().await;
    assert_eq!(plan, second);
    assert!(logs.is_empty(), "{logs}");
    assert!(!fixture.probes.exists());
    assert!(fixture.entry().probe_report.is_none());
}

#[tokio::test]
async fn undeclared_launch_preserves_cwd_fallback_chain() {
    let mut fixture = Fixture::new(false);
    fixture
        .manifest
        .as_object_mut()
        .unwrap()
        .remove("lifecycle");
    fixture.save();
    let plan = resolve(&fixture.state, ID, user_args(), None)
        .await
        .unwrap();
    assert_v1(&plan, &fixture.data_dir());
    std::fs::write(fixture.root.path().join(".git"), b"").unwrap();
    let plan = resolve(
        &fixture.state,
        ID,
        user_args(),
        fixture.root.path().join("missing").to_str(),
    )
    .await
    .unwrap();
    assert_v1(&plan, fixture.root.path());
}

#[tokio::test]
async fn bundled_command_uses_declared_program_and_approved_grants() {
    let mut fixture = Fixture::new(false);
    let helper = fixture.root.path().join("helper.sh");
    std::os::unix::fs::symlink(&fixture.executable, &helper).unwrap();
    fixture.manifest["runtime"] = json!({
        "type": "bundled", "executable": "provider.sh",
        "platformPackages": {PlatformTarget::current().unwrap().identifier(): "launch-runtime"}
    });
    fixture.manifest["distribution"]["type"] = json!("npm");
    fixture.manifest["permissions"] = json!(["process-spawn"]);
    fixture.manifest["lifecycle"]["launch"]["command"]["program"] = json!("helper.sh");
    fixture.save();
    ExtensionManifest::load(&fixture.manifest_path).unwrap();
    let mut repository = ExtensionsLock::load(&fixture.state.paths.repository_file).unwrap();
    let entry = repository.extensions.get_mut(ID).unwrap();
    entry.distribution_source = crate::extensions::lock::ExtensionDistributionSource::Npm;
    entry.runtime_ownership = crate::extensions::lock::ExtensionRuntimeOwnership::Bundled;
    entry.runtime_root = Some(fixture.root.path().to_string_lossy().into_owned());
    entry.approved_permissions = vec![Permission::ProcessSpawn];
    repository
        .save(&fixture.state.paths.repository_file)
        .unwrap();
    let plan = fixture.launch().await;
    assert_eq!(fixture.spawn(&plan).await[0], helper.to_string_lossy());

    std::fs::remove_file(&helper).unwrap();
    let (fallback, logs) = fixture.logged_launch().await;
    assert_v1(&fallback.unwrap(), fixture.root.path());
    assert!(
        logs.contains("WARN")
            && logs.contains("lifecycle.launch.command")
            && logs.contains("Execution program does not exist"),
        "{logs}"
    );
}

#[tokio::test]
async fn protected_launch_plan_ignores_ipc_tampering_and_is_single_use() {
    let fixture = Fixture::new(false);
    let mut plan = fixture.launch().await;
    let original_execution = plan["execution"].clone();
    plan["execution"]["program"] = json!("/does-not-exist");
    plan["execution"]["args"] = json!(["wrong-argument"]);
    plan["execution"]["cwd"] = json!("/does-not-exist");
    let record = fixture.spawn(&plan).await;
    assert_eq!(record[0], fixture.executable.to_string_lossy());
    assert_eq!(record[1], fixture.data_dir().to_string_lossy());
    assert_eq!(record[7], "declared arg");
    let execution: TerminalExecutionPlan = serde_json::from_value(original_execution).unwrap();
    let error = execution.resolve(&fixture.state).err().unwrap();
    assert!(
        error.contains("missing or has already been used"),
        "{error}"
    );
}

#[tokio::test]
async fn missing_manifest_warns_and_returns_real_v1_plan() {
    let fixture = Fixture::new(false);
    std::fs::remove_file(&fixture.manifest_path).unwrap();
    let (plan, logs) = fixture.logged_launch().await;
    assert_v1(&plan.unwrap(), fixture.root.path());
    assert!(
        logs.contains("WARN")
            && logs.contains("Manifest load failed during launch; using v1 defaults")
            && logs.contains(ID)
            && logs.contains("Cannot read manifest"),
        "{logs}"
    );
}

#[tokio::test]
async fn corrupt_manifest_warns_and_returns_real_v1_plan() {
    let fixture = Fixture::new(false);
    std::fs::write(&fixture.manifest_path, b"not JSON{{{").unwrap();
    let (plan, logs) = fixture.logged_launch().await;
    assert_v1(&plan.unwrap(), fixture.root.path());
    assert!(
        logs.contains("WARN")
            && logs.contains("Manifest load failed during launch; using v1 defaults")
            && logs.contains(ID)
            && logs.contains("Invalid extension manifest JSON"),
        "{logs}"
    );
}

#[tokio::test]
async fn invalid_cwd_on_loaded_manifest_warns_and_discards_entire_declaration() {
    let mut fixture = Fixture::new(false);
    fixture.manifest["lifecycle"]["launch"]["cwdPolicy"] =
        json!({"policy": "fixed", "path": fixture.root.path().join("missing")});
    fixture.save();
    ExtensionManifest::load(&fixture.manifest_path).unwrap();
    let (plan, logs) = fixture.logged_launch().await;
    assert_v1(&plan.unwrap(), fixture.root.path());
    assert!(
        logs.contains("WARN")
            && logs.contains("Invalid lifecycle.launch declaration")
            && logs.contains("cwdPolicy"),
        "{logs}"
    );
}

#[tokio::test]
async fn unknown_terminal_and_malformed_commands_warn_and_use_defaults() {
    let fixture = Fixture::new(false);
    for (field, value, diagnostic) in [
        (
            "terminal",
            json!({"color": "imaginary"}),
            "/lifecycle/launch/terminal/color",
        ),
        (
            "terminal",
            json!("imaginary-terminal"),
            "/lifecycle/launch/terminal",
        ),
        (
            "command",
            json!({"program": "../escape"}),
            "lifecycle.launch.command.program",
        ),
        (
            "command",
            json!({"program": "self", "args": ["bad\0arg"]}),
            "lifecycle.launch.command.args",
        ),
        (
            "command",
            json!({"program": "self", "args": "not-an-array"}),
            "/lifecycle/launch/command/args",
        ),
        (
            "restorePolicy",
            json!("unknown"),
            "/lifecycle/launch/restorePolicy",
        ),
    ] {
        let mut manifest = fixture.manifest.clone();
        manifest["lifecycle"]["launch"][field] = value;
        std::fs::write(
            &fixture.manifest_path,
            serde_json::to_vec(&manifest).unwrap(),
        )
        .unwrap();
        let (plan, logs) = fixture.logged_launch().await;
        assert_v1(&plan.unwrap(), fixture.root.path());
        assert!(
            logs.contains("WARN")
                && logs.contains("using v1 defaults")
                && logs.contains(diagnostic),
            "{logs}"
        );
    }
}

#[tokio::test]
async fn unapproved_command_permission_on_loaded_manifest_falls_back() {
    let mut fixture = Fixture::new(false);
    fixture.manifest["permissions"] = json!(["process-spawn"]);
    fixture.manifest["lifecycle"]["launch"]["command"]["program"] = json!("helper.sh");
    fixture.save();
    ExtensionManifest::load(&fixture.manifest_path).unwrap();
    let (plan, logs) = fixture.logged_launch().await;
    assert_v1(&plan.unwrap(), fixture.root.path());
    assert!(
        logs.contains("lifecycle.launch.command") && logs.contains("process-spawn"),
        "{logs}"
    );
}

#[tokio::test]
async fn session_restart_resolves_current_manifest_before_spawning_again() {
    let mut fixture = Fixture::new(true);
    let first = fixture.launch().await;
    fixture.spawn(&first).await;
    fixture.manifest["lifecycle"]["launch"]["command"]["args"] =
        json!(["--launch", "after-restore"]);
    fixture.manifest["lifecycle"]["launch"]["cwdPolicy"] = json!("inheritActiveSession");
    fixture.manifest["lifecycle"]["launch"]["terminal"]["color"] = json!("none");
    fixture.save();
    let restored = fixture.launch().await;
    assert_eq!(restored["isRestart"], true);
    assert_ne!(restored["sessionId"], first["sessionId"]);
    let record = fixture.spawn(&restored).await;
    assert_eq!(record[1], fixture.root.path().to_string_lossy());
    assert_eq!(record[2], "dumb");
    assert_eq!(record[7], "after-restore");
    assert_eq!(
        record.len(),
        10,
        "prefixes and caller args must not accumulate on restore"
    );
    let stored = SessionResolver::new(fixture.data_dir().join("sessions"))
        .find_session(ID)
        .unwrap()
        .unwrap();
    assert_eq!(stored.restore_policy, RestorePolicy::Restart);
    assert_eq!(stored.argv, user_args());
    assert_eq!(stored.session_id, restored["sessionId"]);
}

#[tokio::test]
async fn reattach_and_none_policies_share_the_launch_resolver() {
    let mut fixture = Fixture::new(false);
    for policy in ["reattach", "none"] {
        fixture.manifest["lifecycle"]["launch"]["restorePolicy"] = json!(policy);
        fixture.save();
        let first = fixture.launch().await;
        let second = fixture.launch().await;
        assert_eq!(
            first["sessionId"] == second["sessionId"],
            policy == "reattach"
        );
        assert_eq!(second["isRestart"], false);
        let record = fixture.spawn(&second).await;
        assert_eq!(record[7], "declared arg");
    }
}

#[tokio::test]
async fn corrupt_optional_session_warns_and_launches_with_manifest_settings() {
    let fixture = Fixture::new(false);
    let first = fixture.launch().await;
    std::fs::write(fixture.session_path(), b"corrupt session").unwrap();
    let (plan, logs) = fixture.logged_launch().await;
    let plan = plan.unwrap();
    assert!(
        logs.contains("WARN")
            && logs.contains("Session load failed during launch; creating a new session"),
        "{logs}"
    );
    assert_ne!(first["sessionId"], plan["sessionId"]);
    assert_eq!(plan["isRestart"], false);
    assert_eq!(fixture.spawn(&plan).await[7], "declared arg");
}

#[tokio::test]
async fn declared_required_probe_failure_blocks_launch_and_is_recorded_in_repository() {
    let mut fixture = Fixture::new(true);
    fixture.manifest["lifecycle"]["probes"] =
        json!([{"id": "ready", "args": ["--health"], "required": true}]);
    fixture.manifest["provider"]["environment"]["FLOTER_LAUNCH_FAIL"] = json!("--health");
    fixture.save();
    let error = resolve(&fixture.state, ID, user_args(), None)
        .await
        .unwrap_err();
    assert!(error.contains("Lifecycle probe 'ready' failed"), "{error}");
    assert!(!fixture.record.exists());
    assert!(!fixture.session_path().exists());
    assert_eq!(
        fixture.entry().probe_report.unwrap().status,
        HealthStatus::Unhealthy
    );
    assert_eq!(
        std::fs::read_to_string(&fixture.probes).unwrap(),
        "--health|platform\n"
    );
}

#[tokio::test]
async fn declared_optional_probe_failure_still_launches() {
    let mut fixture = Fixture::new(false);
    fixture.manifest["lifecycle"]["probes"] = json!([{"id": "optional", "args": ["--optional"]}]);
    fixture.manifest["provider"]["environment"]["FLOTER_LAUNCH_FAIL"] = json!("--optional");
    fixture.save();
    let plan = fixture.launch().await;
    assert_eq!(fixture.spawn(&plan).await[7], "declared arg");
    assert_eq!(
        fixture.entry().probe_report.unwrap().status,
        HealthStatus::Degraded
    );
    assert!(fixture.entry().enabled);
}

#[tokio::test]
async fn project_root_max_depth_and_fixed_directory_follow_schema() {
    let mut fixture = Fixture::new(false);
    std::fs::write(fixture.root.path().join(".launch-root"), b"").unwrap();
    let nested = fixture.root.path().join("nested");
    std::fs::create_dir_all(&nested).unwrap();
    for (max_depth, expected) in [(1, nested.clone()), (2, fixture.root.path().to_path_buf())] {
        fixture.manifest["lifecycle"]["launch"]["cwdPolicy"] =
            json!({"policy": "projectRoot", "markers": [".launch-root"], "maxDepth": max_depth});
        fixture.save();
        let plan = resolve(&fixture.state, ID, user_args(), nested.to_str())
            .await
            .unwrap();
        assert_eq!(fixture.spawn(&plan).await[1], expected.to_string_lossy());
    }
    let fixed = fixture.data_dir().join("workspace");
    std::fs::create_dir_all(&fixed).unwrap();
    fixture.manifest["lifecycle"]["launch"]["cwdPolicy"] =
        json!({"policy": "fixed", "path": fixed});
    fixture.save();
    assert_eq!(
        fixture.spawn(&fixture.launch().await).await[1],
        fixed.to_string_lossy()
    );
}

#[tokio::test]
async fn command_only_declaration_defaults_cwd_restore_and_terminal() {
    let mut fixture = Fixture::new(false);
    fixture.manifest["lifecycle"]["launch"] = json!({"command": {"args": ["--launch"]}});
    fixture.save();
    let first = fixture.launch().await;
    let second = fixture.launch().await;
    assert_eq!(first["sessionId"], second["sessionId"]);
    let record = fixture.spawn(&second).await;
    assert_eq!(record[1], fixture.root.path().to_string_lossy());
    assert_eq!(record[2], "floter-256color");
    assert_eq!(record[3], "truecolor");
    assert_eq!(
        &record[6..],
        &[vec!["--launch".into()], user_args()].concat()
    );
}

#[tokio::test]
async fn cwd_only_declaration_uses_registry_runtime_and_caller_args() {
    let mut fixture = Fixture::new(true);
    fixture.manifest["lifecycle"]["launch"] =
        json!({"cwdPolicy": "home", "terminal": {"color": "8"}});
    fixture.save();
    let plan = resolve(&fixture.state, ID, vec!["--launch".into()], None)
        .await
        .unwrap();
    let record = fixture.spawn(&plan).await;
    assert_eq!(
        record[0],
        fixture.root.path().join("tool.sh").to_string_lossy()
    );
    assert_eq!(record[1], dirs::home_dir().unwrap().to_string_lossy());
    assert_eq!(record[2], "xterm");
    assert_eq!(record[3], "");
    assert_eq!(&record[6..], &["--launch"]);
}
