use crate::extensions::capability_probe::{CapabilityProbe, CapabilityReport, ProbeResult};
use crate::extensions::health::{HealthReport, HealthStatus, ProbeFailure, ProbeRecord};
use crate::extensions::manifest::Permission;
use crate::extensions::ExtensionState;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{Duration, Instant};

const PROBE_TIMEOUT: Duration = Duration::from_secs(5);
const MAX_OUTPUT_BYTES: usize = 64 * 1024;

/// Runs capability probes against a tool and produces a health report.
pub async fn run_probes(
    state: &ExtensionState,
    tool_id: &str,
    executable: &PathBuf,
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
                report.record_failure(
                    &format!("probe-{i}"),
                    duration,
                    None,
                    error,
                    !required,
                );
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
    executable: &PathBuf,
    args: &[String],
    timeout: Duration,
) -> Result<ProbeResult, String> {
    let mut command = tokio::process::Command::new(executable);
    command
        .args(args)
        .stdin(std::process::Stdio::null())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .kill_on_drop(true);

    let mut child = command.spawn().map_err(|error| {
        format!(
            "Cannot start probe for {}: {error}",
            executable.display()
        )
    })?;

    let stdout = child
        .stdout
        .take()
        .ok_or("Probe stdout is unavailable")?;
    let stderr = child
        .stderr
        .take()
        .ok_or("Probe stderr is unavailable")?;

    let stdout_task = tokio::spawn(read_output(stdout, MAX_OUTPUT_BYTES));
    let stderr_task = tokio::spawn(read_output(stderr, MAX_OUTPUT_BYTES));

    let status = match tokio::time::timeout(timeout, child.wait()).await {
        Ok(result) => result.map_err(|error| format!("Cannot wait for probe: {error}"))?,
        Err(_) => {
            let _ = child.kill().await;
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

#[cfg(test)]
mod tests {
    use super::*;

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
