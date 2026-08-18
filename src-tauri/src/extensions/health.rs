use crate::extensions::capability_probe::CapabilityReport;
use chrono::Utc;
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::Path;
use std::time::Duration;

const HEALTH_SCHEMA_VERSION: u32 = 1;

/// Overall tool health.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub enum HealthStatus {
    /// All required probes passed.
    Healthy,
    /// Only optional probes failed; tool is usable with degraded features.
    Degraded,
    /// A required probe failed; tool cannot be activated.
    Unhealthy,
    /// Health has not been checked yet.
    Unknown,
}

/// A single probe failure recorded in [`HealthReport`].
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProbeFailure {
    pub probe: String,
    pub exit_code: Option<i32>,
    pub stderr: String,
    pub retryable: bool,
}

/// Result of running a single probe (passed or failed).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProbeRecord {
    pub probe_id: String,
    pub passed: bool,
    pub duration_ms: u64,
    #[serde(default)]
    pub exit_code: Option<i32>,
    /// Truncated stderr captured during probe.
    #[serde(default)]
    pub stderr: String,
}

/// Aggregated health report written to `health.json`.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HealthReport {
    pub schema_version: u32,
    pub status: HealthStatus,
    pub checked_at: String,
    #[serde(default)]
    pub capabilities: CapabilityReport,
    #[serde(default)]
    pub probes: Vec<ProbeRecord>,
    #[serde(default)]
    pub failures: Vec<ProbeFailure>,
}

impl HealthReport {
    pub fn new(capabilities: CapabilityReport) -> Self {
        Self {
            schema_version: HEALTH_SCHEMA_VERSION,
            status: HealthStatus::Unknown,
            checked_at: Utc::now().to_rfc3339(),
            capabilities,
            probes: Vec::new(),
            failures: Vec::new(),
        }
    }

    /// Add a passed probe record.
    pub fn record_pass(&mut self, probe_id: &str, duration: Duration, exit_code: Option<i32>) {
        self.probes.push(ProbeRecord {
            probe_id: probe_id.to_string(),
            passed: true,
            duration_ms: duration.as_millis() as u64,
            exit_code,
            stderr: String::new(),
        });
    }

    /// Add a failed probe record.
    pub fn record_failure(
        &mut self,
        probe_id: &str,
        duration: Duration,
        exit_code: Option<i32>,
        stderr: String,
        retryable: bool,
    ) {
        self.probes.push(ProbeRecord {
            probe_id: probe_id.to_string(),
            passed: false,
            duration_ms: duration.as_millis() as u64,
            exit_code,
            stderr: stderr.clone(),
        });
        self.failures.push(ProbeFailure {
            probe: probe_id.to_string(),
            exit_code,
            stderr,
            retryable,
        });
    }

    /// Determine overall health based on probe results and a set of required probe IDs.
    pub fn finalize(&mut self, required_probes: &[String]) {
        let any_required_failed = self
            .failures
            .iter()
            .any(|f| required_probes.contains(&f.probe));
        let any_optional_failed = self
            .failures
            .iter()
            .any(|f| !required_probes.contains(&f.probe));

        self.status = if any_required_failed {
            HealthStatus::Unhealthy
        } else if any_optional_failed {
            HealthStatus::Degraded
        } else if self.probes.is_empty() {
            HealthStatus::Unknown
        } else {
            HealthStatus::Healthy
        };
    }
}

/// Writes a health report to the tool's health.json file.
pub fn write_health_report(health_dir: &Path, report: &HealthReport) -> Result<(), String> {
    std::fs::create_dir_all(health_dir)
        .map_err(|error| format!("Cannot create health directory: {error}"))?;
    let path = health_dir.join("health.json");
    let bytes = serde_json::to_vec_pretty(report)
        .map_err(|error| format!("Cannot serialize health report: {error}"))?;
    let mut temp = tempfile::NamedTempFile::new_in(health_dir)
        .map_err(|error| format!("Cannot create temp file: {error}"))?;
    temp.write_all(&bytes)
        .and_then(|_| temp.flush())
        .and_then(|_| temp.as_file().sync_all())
        .map_err(|error| format!("Cannot write health report: {error}"))?;
    temp.persist(&path)
        .map_err(|error| format!("Cannot persist health report: {error}"))?;
    Ok(())
}

/// Reads a health report from the tool's health.json file.
pub fn read_health_report(health_dir: &Path) -> Result<Option<HealthReport>, String> {
    let path = health_dir.join("health.json");
    if !path.exists() {
        return Ok(None);
    }
    let bytes =
        std::fs::read(&path).map_err(|error| format!("Cannot read health report: {error}"))?;
    let report: HealthReport = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Cannot parse health report: {error}"))?;
    Ok(Some(report))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::capability_probe::CapabilityReport;
    use tempfile::TempDir;

    #[test]
    fn health_report_new_starts_unknown() {
        let report = HealthReport::new(CapabilityReport::default());
        assert_eq!(report.status, HealthStatus::Unknown);
        assert_eq!(report.schema_version, HEALTH_SCHEMA_VERSION);
    }

    #[test]
    fn finalize_all_pass_is_healthy() {
        let mut report = HealthReport::new(CapabilityReport::default());
        report.record_pass("version", Duration::from_millis(100), Some(0));
        record_pass(
            &mut report,
            "completion",
            Duration::from_millis(200),
            Some(0),
        );
        let required = vec!["version".to_string()];
        report.finalize(&required);
        assert_eq!(report.status, HealthStatus::Healthy);
    }

    #[test]
    fn finalize_required_fail_is_unhealthy() {
        let mut report = HealthReport::new(CapabilityReport::default());
        record_pass(&mut report, "version", Duration::from_millis(100), Some(0));
        report.record_failure(
            "completion",
            Duration::from_millis(200),
            Some(2),
            "error".into(),
            false,
        );
        let required = vec!["completion".to_string()];
        report.finalize(&required);
        assert_eq!(report.status, HealthStatus::Unhealthy);
    }

    #[test]
    fn finalize_optional_fail_is_degraded() {
        let mut report = HealthReport::new(CapabilityReport::default());
        record_pass(&mut report, "version", Duration::from_millis(100), Some(0));
        report.record_failure(
            "completion",
            Duration::from_millis(200),
            Some(2),
            "error".into(),
            true,
        );
        let required = vec!["version".to_string()];
        report.finalize(&required);
        assert_eq!(report.status, HealthStatus::Degraded);
    }

    #[test]
    fn write_and_read_health_report() {
        let temp = TempDir::new().unwrap();
        let mut report = HealthReport::new(CapabilityReport::default());
        record_pass(&mut report, "version", Duration::from_millis(100), Some(0));
        let required = vec!["version".to_string()];
        report.finalize(&required);
        write_health_report(temp.path(), &report).unwrap();
        let loaded = read_health_report(temp.path()).unwrap();
        assert!(loaded.is_some());
        let loaded = loaded.unwrap();
        assert_eq!(loaded.status, HealthStatus::Healthy);
        assert_eq!(loaded.probes.len(), 1);
    }

    #[test]
    fn read_nonexistent_returns_none() {
        let temp = TempDir::new().unwrap();
        let result = read_health_report(temp.path()).unwrap();
        assert!(result.is_none());
    }

    fn record_pass(
        report: &mut HealthReport,
        id: &str,
        duration: Duration,
        exit_code: Option<i32>,
    ) {
        report.record_pass(id, duration, exit_code);
    }
}
