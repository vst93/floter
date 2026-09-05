//! Shared probe execution logic for install and reprobe operations.
//!
//! This module provides the unified probe selection and execution strategy:
//! - If the manifest declares lifecycle probes, execute those
//! - Otherwise fall back to --version/--help for backward compatibility

use crate::extensions::capability_probe::CapabilityProbe;
use crate::extensions::health::HealthReport;
use crate::extensions::lifecycle::CapabilityProbeEntry;
use crate::extensions::manifest::ExtensionManifest;
use crate::extensions::ExtensionState;
use std::path::Path;
use std::time::Duration;

/// Execute capability probes for a tool, using manifest-declared probes if present,
/// otherwise falling back to --version/--help.
pub async fn execute_capability_probes(
    _state: &ExtensionState,
    _tool_id: &str,
    executable: &Path,
    manifest: &ExtensionManifest,
) -> Result<HealthReport, String> {
    if manifest.lifecycle.probes.is_empty() {
        // Backward compatibility: no probes declared → use --version/--help
        execute_default_probes(executable).await
    } else {
        // Execute the declared probes
        execute_manifest_probes(executable, &manifest.lifecycle.probes).await
    }
}

/// Execute the manifest-declared probes.
async fn execute_manifest_probes(
    executable: &Path,
    probe_entries: &[CapabilityProbeEntry],
) -> Result<HealthReport, String> {
    let mut report = HealthReport::new(Default::default());

    for entry in probe_entries {
        let start = std::time::Instant::now();
        let timeout = Duration::from_millis(entry.timeout_ms);

        match crate::extensions::probe_runner::run_single_probe(executable, &entry.args, timeout)
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

    Ok(report)
}

/// Execute the default --version/--help probes for backward compatibility.
async fn execute_default_probes(executable: &Path) -> Result<HealthReport, String> {
    let version_probe = CapabilityProbe::version();
    let help_probe = CapabilityProbe::help();

    let probes = [version_probe, help_probe];
    let required = [true, false]; // version required, help optional

    let mut report = HealthReport::new(Default::default());

    for (i, probe) in probes.iter().enumerate() {
        let start = std::time::Instant::now();
        let timeout = Duration::from_secs(5);

        match crate::extensions::probe_runner::run_single_probe(executable, &probe.args, timeout)
            .await
        {
            Ok(result) => {
                let duration = start.elapsed();
                if result.passed {
                    report.record_pass(&probe.id, duration, result.exit_code);
                } else {
                    report.record_failure(
                        &probe.id,
                        duration,
                        result.exit_code,
                        result.stderr,
                        !required[i],
                    );
                }
            }
            Err(error) => {
                let duration = start.elapsed();
                report.record_failure(&probe.id, duration, None, error, !required[i]);
            }
        }
    }

    let required_ids: Vec<String> = required
        .iter()
        .enumerate()
        .filter(|(_, r)| **r)
        .map(|(i, _)| probes[i].id.clone())
        .collect();
    report.finalize(&required_ids);

    Ok(report)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::health::HealthStatus;
    use crate::extensions::lifecycle::{CapabilityProbeEntry, ToolLifecycle};
    use crate::extensions::manifest::ExtensionManifest;

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

        let report = execute_manifest_probes(&tool, &manifest.lifecycle.probes)
            .await
            .unwrap();

        assert_eq!(report.status, HealthStatus::Healthy);
        assert_eq!(report.probes.len(), 1);
        assert_eq!(report.probes[0].probe_id, "custom-check");
        assert!(report.probes[0].passed);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn manifest_without_probes_falls_back_to_version_and_help() {
        let tool = fixture("capability-probe.sh");

        let manifest = minimal_manifest();
        assert!(manifest.lifecycle.probes.is_empty());

        let report = execute_default_probes(&tool).await.unwrap();

        // Should have attempted version (required) and help (optional)
        assert!(report.probes.iter().any(|p| p.probe_id == "version"));
        let version_probe = report
            .probes
            .iter()
            .find(|p| p.probe_id == "version")
            .unwrap();
        assert!(version_probe.passed);
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

        let report = execute_manifest_probes(&tool, &probes).await.unwrap();

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

        let report = execute_manifest_probes(&tool, &probes).await.unwrap();

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

        let report = execute_manifest_probes(&tool, &probes).await.unwrap();

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

        let report = execute_manifest_probes(&tool, &probes).await.unwrap();

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
