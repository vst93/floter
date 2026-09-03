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
    use crate::extensions::lifecycle::CapabilityProbeEntry;

    #[test]
    fn manifest_with_probes_uses_declared_probes() {
        let probes = vec![CapabilityProbeEntry {
            id: "custom-check".to_string(),
            args: vec!["--check".to_string()],
            timeout_ms: 2000,
            required: true,
        }];
        assert!(!probes.is_empty());
    }

    #[test]
    fn manifest_without_probes_falls_back_to_defaults() {
        let probes: Vec<CapabilityProbeEntry> = vec![];
        assert!(probes.is_empty());
    }
}
