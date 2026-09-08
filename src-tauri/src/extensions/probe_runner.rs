use crate::extensions::capability_probe::{CapabilityProbe, CapabilityReport, ProbeResult};
use crate::extensions::health::HealthReport;
use crate::extensions::process_cleanup::{configure_command, ChildCleanup};
use crate::extensions::ExtensionState;
use std::path::Path;
use std::time::{Duration, Instant};

const PROBE_TIMEOUT: Duration = Duration::from_secs(5);
const MAX_OUTPUT_BYTES: usize = 64 * 1024;

/// Runs capability probes against a tool and produces a health report.
pub async fn run_probes(
    _state: &ExtensionState,
    _tool_id: &str,
    executable: &Path,
    probe_args: &[Vec<String>],
    required_probes: &[bool],
) -> Result<HealthReport, String> {
    let mut report = HealthReport::new(CapabilityReport::default());

    for (i, args) in probe_args.iter().enumerate() {
        let required = required_probes.get(i).copied().unwrap_or(false);
        let start = Instant::now();

        match run_single_probe(executable, args, PROBE_TIMEOUT).await {
            Ok(result) => {
                let duration = start.elapsed();
                if result.passed {
                    report.record_pass(&format!("probe-{i}"), duration, result.exit_code);
                } else {
                    let stderr = result.stderr.clone();
                    report.record_failure(
                        &format!("probe-{i}"),
                        duration,
                        result.exit_code,
                        stderr,
                        !required,
                    );
                }
            }
            Err(error) => {
                let duration = start.elapsed();
                report.record_failure(&format!("probe-{i}"), duration, None, error, !required);
            }
        }
    }

    let required_ids: Vec<String> = required_probes
        .iter()
        .enumerate()
        .filter(|(_, required)| **required)
        .map(|(i, _)| format!("probe-{i}"))
        .collect();
    report.finalize(&required_ids);

    Ok(report)
}

pub async fn run_single_probe(
    executable: &Path,
    args: &[String],
    timeout: Duration,
) -> Result<ProbeResult, String> {
    run_probe_command(
        tokio::process::Command::new(executable),
        executable,
        args,
        timeout,
    )
    .await
}

pub(crate) async fn run_invocation_probe(
    invocation: &crate::extensions::provider::ProviderInvocation,
    args: &[String],
    timeout: Duration,
) -> Result<ProbeResult, String> {
    let mut command = crate::extensions::provider::provider_command(&invocation.executable);
    if !invocation
        .permissions
        .contains(&crate::extensions::manifest::Permission::Environment)
    {
        command.env_clear();
    }
    command.args(&invocation.executable_prefix).envs(
        crate::extensions::proxy::command_environment(
            &invocation.permissions,
            &invocation.config.environment,
        ),
    );
    run_probe_command(command, &invocation.executable, args, timeout).await
}

async fn run_probe_command(
    mut command: tokio::process::Command,
    executable: &Path,
    args: &[String],
    timeout: Duration,
) -> Result<ProbeResult, String> {
    configure_command(&mut command);
    command
        .args(args)
        .stdin(std::process::Stdio::null())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .kill_on_drop(true);

    let mut child = command
        .spawn()
        .map_err(|error| format!("Cannot start probe for {}: {error}", executable.display()))?;
    let mut cleanup = ChildCleanup::new(&child);

    let stdout = match child.stdout.take() {
        Some(stdout) => stdout,
        None => {
            cleanup.kill_and_reap(&mut child).await;
            return Err("Probe stdout is unavailable".to_string());
        }
    };
    let stderr = match child.stderr.take() {
        Some(stderr) => stderr,
        None => {
            cleanup.kill_and_reap(&mut child).await;
            return Err("Probe stderr is unavailable".to_string());
        }
    };

    let stdout_task = tokio::spawn(read_output(stdout, MAX_OUTPUT_BYTES));
    let stderr_task = tokio::spawn(read_output(stderr, MAX_OUTPUT_BYTES));

    let status = match tokio::time::timeout(timeout, child.wait()).await {
        Ok(Ok(status)) => {
            cleanup.finish_after_wait();
            status
        }
        Ok(Err(error)) => {
            cleanup.kill_and_reap(&mut child).await;
            stdout_task.abort();
            stderr_task.abort();
            return Err(format!("Cannot wait for probe: {error}"));
        }
        Err(_) => {
            cleanup.kill_and_reap(&mut child).await;
            let _ = stdout_task.await;
            let _ = stderr_task.await;
            return Err(format!("Probe timed out after {} ms", timeout.as_millis()));
        }
    };

    let stdout_bytes = stdout_task
        .await
        .map_err(|error| format!("stdout task failed: {error}"))??;
    let stderr_bytes = stderr_task
        .await
        .map_err(|error| format!("stderr task failed: {error}"))??;

    let stdout_str = String::from_utf8_lossy(&stdout_bytes).into_owned();
    let stderr_str = String::from_utf8_lossy(&stderr_bytes).into_owned();

    Ok(ProbeResult {
        probe: CapabilityProbe {
            id: "probe".to_string(),
            args: args.to_vec(),
            expected_exit_code: Some(0),
            expected_output: None,
        },
        passed: status.success(),
        stdout: stdout_str,
        stderr: stderr_str,
        exit_code: status.code(),
        reason: if status.success() {
            None
        } else {
            Some(format!("probe exited with code {:?}", status.code()))
        },
    })
}

async fn read_output(
    mut reader: impl tokio::io::AsyncReadExt + std::marker::Unpin + Send + 'static,
    limit: usize,
) -> Result<Vec<u8>, String> {
    let mut buffer = Vec::with_capacity(limit);
    let mut chunk = vec![0u8; 8192];
    loop {
        let n = reader
            .read(&mut chunk)
            .await
            .map_err(|error| format!("Read error: {error}"))?;
        if n == 0 {
            break;
        }
        if buffer.len() + n > limit {
            buffer.extend_from_slice(&chunk[..limit - buffer.len()]);
            break;
        }
        buffer.extend_from_slice(&chunk[..n]);
    }
    Ok(buffer)
}

#[cfg(all(test, unix))]
mod cleanup_tests {
    use super::*;
    use std::fs;
    use std::os::unix::fs::PermissionsExt;
    use std::path::{Path, PathBuf};
    use std::time::Duration;

    struct CleanupFixture {
        _directory: tempfile::TempDir,
        executable: PathBuf,
        parent_pid: PathBuf,
        child_pid: PathBuf,
    }

    impl CleanupFixture {
        fn new() -> Self {
            let directory = tempfile::tempdir().unwrap();
            let executable = directory.path().join("provider-cleanup.sh");
            let parent_pid = directory.path().join("parent.pid");
            let child_pid = directory.path().join("child.pid");
            fs::write(
                &executable,
                "#!/bin/sh\n\nif [ \"$1\" = immediate ]; then exit 0; fi\nprintf '%s\\n' \"$$\" > \"$1\"\n(sleep 30) &\nprintf '%s\\n' \"$!\" > \"$2\"\ntrap '' TERM INT\nwhile :; do sleep 1; done\n",
            )
            .unwrap();
            let mut permissions = fs::metadata(&executable).unwrap().permissions();
            permissions.set_mode(0o755);
            fs::set_permissions(&executable, permissions).unwrap();
            Self {
                _directory: directory,
                executable,
                parent_pid,
                child_pid,
            }
        }

        async fn wait_for_pid(path: &Path) -> u32 {
            for _ in 0..100 {
                if let Ok(value) = fs::read_to_string(path) {
                    if let Ok(pid) = value.trim().parse() {
                        return pid;
                    }
                }
                tokio::time::sleep(Duration::from_millis(10)).await;
            }
            panic!("fixture did not write PID file {}", path.display());
        }
    }

    async fn assert_gone(pid: u32) {
        for _ in 0..150 {
            if !Path::new(&format!("/proc/{pid}")).exists() {
                return;
            }
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
        panic!("process {pid} survived cleanup");
    }

    #[tokio::test]
    async fn timeout_kills_parent_and_grandchild_process_group() {
        let fixture = CleanupFixture::new();
        let result = run_single_probe(
            &fixture.executable,
            &[
                fixture.parent_pid.to_string_lossy().into_owned(),
                fixture.child_pid.to_string_lossy().into_owned(),
            ],
            Duration::from_millis(100),
        )
        .await;
        let parent = CleanupFixture::wait_for_pid(&fixture.parent_pid).await;
        let child = CleanupFixture::wait_for_pid(&fixture.child_pid).await;
        let error = result.unwrap_err();
        assert!(error.contains("timed out"), "{error}");
        assert_gone(parent).await;
        assert_gone(child).await;
    }

    #[tokio::test]
    async fn aborting_probe_future_kills_the_process_group() {
        let fixture = CleanupFixture::new();
        let executable = fixture.executable.clone();
        let args = vec![
            fixture.parent_pid.to_string_lossy().into_owned(),
            fixture.child_pid.to_string_lossy().into_owned(),
        ];
        let handle = tokio::spawn(async move {
            run_single_probe(&executable, &args, Duration::from_secs(30)).await
        });
        let parent = CleanupFixture::wait_for_pid(&fixture.parent_pid).await;
        let child = CleanupFixture::wait_for_pid(&fixture.child_pid).await;
        handle.abort();
        assert!(handle.await.unwrap_err().is_cancelled());
        assert_gone(parent).await;
        assert_gone(child).await;
    }

    #[tokio::test]
    async fn already_exited_probe_does_not_report_cleanup_error() {
        let fixture = CleanupFixture::new();
        let result = run_single_probe(
            &fixture.executable,
            &["immediate".to_string()],
            Duration::from_secs(1),
        )
        .await
        .unwrap();
        assert!(result.passed, "{result:?}");
        assert_eq!(result.exit_code, Some(0));
    }

    #[tokio::test]
    async fn repeated_timeouts_leave_no_fixture_children_or_zombies() {
        let fixture = CleanupFixture::new();
        for attempt in 0..5 {
            let parent_path = fixture
                ._directory
                .path()
                .join(format!("parent-{attempt}.pid"));
            let child_path = fixture
                ._directory
                .path()
                .join(format!("child-{attempt}.pid"));
            let result = run_single_probe(
                &fixture.executable,
                &[
                    parent_path.to_string_lossy().into_owned(),
                    child_path.to_string_lossy().into_owned(),
                ],
                Duration::from_millis(80),
            )
            .await;
            assert!(result.unwrap_err().contains("timed out"));
            let parent = CleanupFixture::wait_for_pid(&parent_path).await;
            let child = CleanupFixture::wait_for_pid(&child_path).await;
            assert_gone(parent).await;
            assert_gone(child).await;
        }
        let children = fs::read_to_string(format!(
            "/proc/{}/task/{}/children",
            std::process::id(),
            std::process::id()
        ))
        .unwrap_or_default();
        assert!(children.trim().is_empty(), "fixture child remains: {children}");
    }
}

#[cfg(test)]
mod tests {
    use crate::extensions::health::{HealthStatus, ProbeFailure, ProbeRecord};

    #[test]
    fn health_status_serialization() {
        let status = HealthStatus::Degraded;
        let json = serde_json::to_string(&status).unwrap();
        assert_eq!(json, r#""degraded""#);
    }

    #[test]
    fn health_status_deserialization() {
        let status: HealthStatus = serde_json::from_str(r#""unhealthy""#).unwrap();
        assert_eq!(status, HealthStatus::Unhealthy);
    }

    #[test]
    fn probe_failure_structure() {
        let failure = ProbeFailure {
            probe: "test".to_string(),
            exit_code: Some(2),
            stderr: "error".to_string(),
            retryable: true,
        };
        let json = serde_json::to_string_pretty(&failure).unwrap();
        assert!(json.contains("\"probe\": \"test\""));
        assert!(json.contains("\"exitCode\": 2"));
        assert!(json.contains("\"retryable\": true"));
    }

    #[test]
    fn probe_record_structure() {
        let record = ProbeRecord {
            probe_id: "version".to_string(),
            passed: true,
            duration_ms: 150,
            exit_code: Some(0),
            stderr: String::new(),
        };
        let json = serde_json::to_string_pretty(&record).unwrap();
        assert!(json.contains("\"probeId\": \"version\""));
        assert!(json.contains("\"passed\": true"));
        assert!(json.contains("\"durationMs\": 150"));
    }
}
