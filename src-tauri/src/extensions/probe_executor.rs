//! Manifest lifecycle probes shared by install, repair and reprobe.

use crate::extensions::health::{HealthReport, HealthStatus};
use crate::extensions::lock::ExtensionsLock;
use crate::extensions::manifest::ExtensionManifest;
use crate::extensions::provider::ProviderInvocation;
use std::time::Duration;

/// Probe arguments are complete tool arguments, independent of the provider
/// protocol prefix. Script/interpreter prefixes and platform environment come
/// from the same resolved invocation used by installation and verification.
/// Empty lifecycle declarations execute nothing, including for v1 manifests.
pub async fn execute_capability_probes(
    invocation: &ProviderInvocation,
    manifest: &ExtensionManifest,
) -> HealthReport {
    let probe_entries = &manifest.lifecycle.probes;
    let mut report = HealthReport::new(Default::default());

    for entry in probe_entries {
        let start = std::time::Instant::now();
        let timeout = Duration::from_millis(entry.timeout_ms);

        match crate::extensions::probe_runner::run_invocation_probe(
            invocation,
            &entry.args,
            timeout,
        )
        .await
        {
            Ok(result) => {
                let duration = start.elapsed();
                if result.passed {
                    report.record_pass(&entry.id, duration, result.exit_code);
                } else {
                    report.record_failure(
                        &entry.id,
                        duration,
                        result.exit_code,
                        result.stderr,
                        !entry.required,
                    );
                }
            }
            Err(error) => {
                let duration = start.elapsed();
                report.record_failure(&entry.id, duration, None, error, !entry.required);
            }
        }
    }

    let required_ids: Vec<String> = probe_entries
        .iter()
        .filter(|entry| entry.required)
        .map(|entry| entry.id.clone())
        .collect();
    report.finalize(&required_ids);

    report
}

pub(crate) fn verification_error(report: &HealthReport) -> Option<String> {
    let failures = report
        .failures
        .iter()
        .filter(|failure| !failure.retryable)
        .map(|failure| {
            format!(
                "Lifecycle probe '{}' failed (exit code {:?}): {}",
                failure.probe, failure.exit_code, failure.stderr
            )
        })
        .collect::<Vec<_>>();
    (report.status == HealthStatus::Unhealthy).then(|| failures.join("; "))
}

/// Update the report and Phase 2 state in memory for a single repository save.
/// An empty probe set proves nothing and cannot erase an earlier failure.
pub(crate) fn record_report(
    lock: &mut ExtensionsLock,
    id: &str,
    report: HealthReport,
) -> Result<(), String> {
    if report.status == HealthStatus::Unknown {
        return Ok(());
    }
    let problem = verification_error(&report);
    let entry = lock
        .extensions
        .get_mut(id)
        .ok_or_else(|| format!("Extension is not installed: {id}"))?;
    entry.probe_report = Some(report);
    entry.updated_at = crate::extensions::lock::unix_now();
    if let Some(problem) = problem {
        lock.mark_broken(
            id,
            &crate::extensions::install::classify_verify_error(&problem),
            &problem,
        )?;
    } else {
        lock.clear_broken(id)?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::health::HealthStatus;
    use crate::extensions::lifecycle::{CapabilityProbeEntry, ToolLifecycle};
    use crate::extensions::manifest::ExtensionManifest;
    use std::path::Path;

    fn minimal_manifest() -> ExtensionManifest {
        let json = r#"{
            "schemaVersion": "2.0",
            "id": "test.tool",
            "name": "Test Tool",
            "publisher": {"id": "test", "name": "Test"},
            "compatibility": {"floter": ">=0.1.0", "providerProtocol": "^1.0"},
            "distribution": {"type": "local"},
            "runtime": {"type": "system", "executableNames": ["tool"]},
            "provider": {"type": "executable", "argsPrefix": []}
        }"#;
        serde_json::from_str(json).unwrap()
    }

    async fn probe_fixture(tool: &Path, probes: &[CapabilityProbeEntry]) -> HealthReport {
        let mut manifest = minimal_manifest();
        manifest.lifecycle.probes = probes.to_vec();
        let invocation = ProviderInvocation {
            extension_id: manifest.id.clone(),
            executable: tool.to_path_buf(),
            executable_prefix: Vec::new(),
            runtime_root: None,
            package_version: "1.0.0".into(),
            tool_version_hint: None,
            version_args: Vec::new(),
            config: manifest.provider.clone(),
            permissions: manifest.permissions.clone(),
        };
        execute_capability_probes(&invocation, &manifest).await
    }

    #[cfg(unix)]
    fn fixture(name: &str) -> std::path::PathBuf {
        let tool = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("tests/fixtures")
            .join(name);
        // Never open an executable inode for writing while other tests fork.
        crate::extensions::install::make_executable(&tool).unwrap();
        tool
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn manifest_with_probes_executes_declared_probes() {
        let tool = fixture("capability-probe.sh");

        let mut manifest = minimal_manifest();
        manifest.lifecycle = ToolLifecycle {
            probes: vec![CapabilityProbeEntry {
                id: "custom-check".to_string(),
                args: vec!["--check".to_string()],
                timeout_ms: 2000,
                required: true,
            }],
            ..Default::default()
        };

        let report = probe_fixture(&tool, &manifest.lifecycle.probes).await;

        assert_eq!(report.status, HealthStatus::Healthy);
        assert_eq!(report.probes.len(), 1);
        assert_eq!(report.probes[0].probe_id, "custom-check");
        assert!(report.probes[0].passed);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn manifest_without_probes_does_not_attempt_to_start_a_missing_tool() {
        let directory = tempfile::tempdir().unwrap();
        let report = probe_fixture(&directory.path().join("missing"), &[]).await;
        assert_eq!(report.status, HealthStatus::Unknown);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn required_probe_failure_results_in_unhealthy() {
        let tool = fixture("failing-probe.sh");

        let probes = vec![CapabilityProbeEntry {
            id: "required-check".to_string(),
            args: vec!["--check".to_string()],
            timeout_ms: 2000,
            required: true,
        }];

        let report = probe_fixture(&tool, &probes).await;

        assert_eq!(report.status, HealthStatus::Unhealthy);
        assert_eq!(report.failures.len(), 1);
        assert_eq!(report.failures[0].probe, "required-check");
        assert!(!report.failures[0].retryable);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn optional_probe_failure_results_in_degraded() {
        let tool = fixture("failing-probe.sh");

        let probes = vec![CapabilityProbeEntry {
            id: "optional-check".to_string(),
            args: vec!["--feature".to_string()],
            timeout_ms: 2000,
            required: false,
        }];

        let report = probe_fixture(&tool, &probes).await;

        assert_eq!(report.status, HealthStatus::Degraded);
        assert_eq!(report.failures.len(), 1);
        assert_eq!(report.failures[0].probe, "optional-check");
        assert!(report.failures[0].retryable);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn probe_timeout_is_recorded_as_failure() {
        let tool = fixture("capability-probe.sh");

        let probes = vec![CapabilityProbeEntry {
            id: "timeout-check".to_string(),
            args: vec!["--sleep".to_string()],
            timeout_ms: 100, // Short timeout to avoid test hanging
            required: true,
        }];

        let report = probe_fixture(&tool, &probes).await;

        assert_eq!(report.status, HealthStatus::Unhealthy);
        assert_eq!(report.failures.len(), 1);
        assert!(
            report.failures[0].stderr.contains("timed out"),
            "{report:?}"
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn mixed_required_and_optional_probes() {
        let tool = fixture("capability-probe.sh");

        let probes = vec![
            CapabilityProbeEntry {
                id: "required-probe".to_string(),
                args: vec!["--required".to_string()],
                timeout_ms: 2000,
                required: true,
            },
            CapabilityProbeEntry {
                id: "optional-probe".to_string(),
                args: vec!["--optional".to_string()],
                timeout_ms: 2000,
                required: false,
            },
        ];

        let report = probe_fixture(&tool, &probes).await;

        // Required passed, optional failed → Degraded
        assert_eq!(report.status, HealthStatus::Degraded);
        assert_eq!(report.probes.len(), 2);

        let required = report
            .probes
            .iter()
            .find(|p| p.probe_id == "required-probe")
            .unwrap();
        assert!(required.passed);

        let optional = report
            .probes
            .iter()
            .find(|p| p.probe_id == "optional-probe")
            .unwrap();
        assert!(!optional.passed);

        assert_eq!(report.failures.len(), 1);
        assert_eq!(report.failures[0].probe, "optional-probe");
    }
}
