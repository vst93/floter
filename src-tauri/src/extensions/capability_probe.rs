//! Capability probing for linked system tools.
//!
//! A [`CapabilityProbe`] describes a single probe: which arguments to pass to a
//! tool, and which exit code / stdout content confirms the capability. The
//! [`CapabilityProbe::probe`] method runs the tool, and [`CapabilityReport`]
//! aggregates the results of several probes into a version string plus the set
//! of supported features and detected limitations.

use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::Duration;

const VERSION_PROBE_ID: &str = "version";
const HELP_PROBE_ID: &str = "help";
const UNKNOWN_VERSION: &str = "unknown";

/// Upper bound on stdout bytes retained from a single probe run.
const MAX_PROBE_OUTPUT_BYTES: usize = 64 * 1024;
/// Upper bound on how long a single probe may run before it is aborted.
const PROBE_TIMEOUT: Duration = Duration::from_secs(5);

/// A single capability check against a tool.
///
/// The probe passes when the observed exit code matches
/// [`expected_exit_code`](Self::expected_exit_code) (when set) and the captured
/// stdout contains [`expected_output`](Self::expected_output) (when set).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CapabilityProbe {
    /// Identifier used to reference this probe in a [`CapabilityReport`].
    pub id: String,
    /// Arguments passed to the tool, e.g. `["--version"]`.
    pub args: Vec<String>,
    /// Exit code that confirms the capability. `None` accepts any exit code
    /// (some tools exit nonzero from `--help`).
    pub expected_exit_code: Option<i32>,
    /// Substring that must appear in stdout to confirm the capability.
    pub expected_output: Option<String>,
}

impl CapabilityProbe {
    /// Probe that asks the tool for its version via `--version`.
    pub fn version() -> Self {
        Self {
            id: VERSION_PROBE_ID.into(),
            args: vec!["--version".into()],
            expected_exit_code: Some(0),
            expected_output: None,
        }
    }

    /// Probe that asks the tool for usage via `--help`.
    pub fn help() -> Self {
        Self {
            id: HELP_PROBE_ID.into(),
            args: vec!["--help".into()],
            expected_exit_code: Some(0),
            expected_output: None,
        }
    }

    /// Probe for an arbitrary feature, e.g. `custom("json", ["--output",
    /// "json"])`.
    ///
    /// By default the probe requires a zero exit code; use
    /// [`expect_output`](Self::expect_output) and
    /// [`expect_exit_code`](Self::expect_exit_code) to tighten the check.
    pub fn custom(
        id: impl Into<String>,
        args: impl IntoIterator<Item = impl Into<String>>,
    ) -> Self {
        Self {
            id: id.into(),
            args: args.into_iter().map(Into::into).collect(),
            expected_exit_code: Some(0),
            expected_output: None,
        }
    }

    /// Require a specific exit code. Pass `None` to accept any exit code.
    pub fn expect_exit_code(mut self, exit_code: Option<i32>) -> Self {
        self.expected_exit_code = exit_code;
        self
    }

    /// Require `substring` to appear in the tool's stdout.
    pub fn expect_output(mut self, substring: impl Into<String>) -> Self {
        self.expected_output = Some(substring.into());
        self
    }

    /// Run the probe against `executable` and report whether the capability is
    /// present.
    ///
    /// Returns an `Err` only when the tool cannot be executed at all (missing
    /// binary, timeout, ...); a tool that runs but does not satisfy the
    /// expectations yields an `Ok` result with `passed == false`.
    pub async fn probe(&self, executable: &Path) -> Result<ProbeResult, String> {
        let mut command = tokio::process::Command::new(executable);
        command
            .args(&self.args)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .kill_on_drop(true);
        let output = tokio::time::timeout(PROBE_TIMEOUT, command.output())
            .await
            .map_err(|_| {
                format!(
                    "probe '{}' timed out after {} seconds",
                    self.id,
                    PROBE_TIMEOUT.as_secs()
                )
            })?
            .map_err(|error| format!("cannot run '{}': {error}", executable.display()))?;

        let mut stdout = String::from_utf8_lossy(&output.stdout).into_owned();
        stdout.truncate(MAX_PROBE_OUTPUT_BYTES);
        let mut stderr = String::from_utf8_lossy(&output.stderr).into_owned();
        stderr.truncate(MAX_PROBE_OUTPUT_BYTES);
        let exit_code = output.status.code();

        let mut reasons = Vec::new();
        if let Some(expected) = self.expected_exit_code {
            if exit_code != Some(expected) {
                let actual = match exit_code {
                    Some(code) => code.to_string(),
                    None => "terminated by signal".to_string(),
                };
                reasons.push(format!("exit code {actual} (expected {expected})"));
            }
        }
        if let Some(expected) = &self.expected_output {
            if !stdout.contains(expected.as_str()) {
                reasons.push(format!("output does not contain {expected:?}"));
            }
        }

        let passed = reasons.is_empty();
        let reason = if passed {
            None
        } else {
            Some(reasons.join("; "))
        };
        Ok(ProbeResult {
            probe: self.clone(),
            exit_code,
            stdout,
            stderr,
            passed,
            reason,
        })
    }
}

/// Outcome of a single [`CapabilityProbe`] run.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProbeResult {
    /// The probe that produced this result.
    pub probe: CapabilityProbe,
    /// Process exit code, or `None` if the process was terminated by a signal.
    pub exit_code: Option<i32>,
    /// Captured stdout (truncated to [`MAX_PROBE_OUTPUT_BYTES`]).
    pub stdout: String,
    /// Captured stderr (truncated to [`MAX_PROBE_OUTPUT_BYTES`]).
    pub stderr: String,
    /// Whether the probe satisfied all expectations.
    pub passed: bool,
    /// Human-readable reason for failure, when `passed == false`.
    pub reason: Option<String>,
}

/// Aggregated view of a tool's capabilities.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct CapabilityReport {
    /// Version reported by the tool, or [`UNKNOWN_VERSION`] when the version
    /// probe did not pass.
    pub version: String,
    /// Identifiers of every probe that passed.
    pub supported_features: Vec<String>,
    /// Human-readable description of every probe that failed.
    pub limitations: Vec<String>,
}

impl Default for CapabilityReport {
    fn default() -> Self {
        Self {
            version: "unknown".to_string(),
            supported_features: Vec::new(),
            limitations: Vec::new(),
        }
    }
}

impl CapabilityReport {
    /// Build a report from probe results.
    ///
    /// `supported_features` lists every probe id that passed (including the
    /// structural `version` and `help` probes); `limitations` describes each
    /// failed probe together with its arguments and failure reason.
    pub fn from_probes(results: &[ProbeResult]) -> Self {
        let version = results
            .iter()
            .find(|result| result.probe.id == VERSION_PROBE_ID && result.passed)
            .and_then(|result| result.stdout.lines().map(str::trim).find(|line| !line.is_empty()))
            .unwrap_or(UNKNOWN_VERSION)
            .to_string();

        let supported_features = results
            .iter()
            .filter(|result| result.passed)
            .map(|result| result.probe.id.clone())
            .collect();

        let limitations = results
            .iter()
            .filter(|result| !result.passed)
            .map(|result| {
                let invocation = if result.probe.args.is_empty() {
                    result.probe.id.clone()
                } else {
                    format!("{} {}", result.probe.id, result.probe.args.join(" "))
                };
                match &result.reason {
                    Some(reason) => format!("{invocation}: {reason}"),
                    None => format!("{invocation}: failed"),
                }
            })
            .collect();

        Self {
            version,
            supported_features,
            limitations,
        }
    }
}

/// Runs a set of [`CapabilityProbe`]s against a single executable and produces
/// a [`CapabilityReport`].
#[derive(Debug, Clone)]
pub struct CapabilityScanner {
    executable: PathBuf,
}

impl CapabilityScanner {
    pub fn new(executable: impl Into<PathBuf>) -> Self {
        Self {
            executable: executable.into(),
        }
    }

    /// Probe the standard `--version` and `--help` capabilities.
    pub async fn scan(&self) -> Result<CapabilityReport, String> {
        self.scan_with(&[CapabilityProbe::version(), CapabilityProbe::help()])
            .await
    }

    /// Run `probes` against the executable and aggregate the results.
    ///
    /// Fails if any probe cannot be executed at all.
    pub async fn scan_with(&self, probes: &[CapabilityProbe]) -> Result<CapabilityReport, String> {
        let mut results = Vec::with_capacity(probes.len());
        for probe in probes {
            results.push(probe.probe(&self.executable).await?);
        }
        Ok(CapabilityReport::from_probes(&results))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::sync::Once;

    #[cfg(not(windows))]
    fn write_fixture_script(path: &Path) {
        use std::os::unix::fs::PermissionsExt;
        let script = r#"#!/bin/sh
case "$1" in
  --version)  echo "floter-tool 1.2.3"; exit 0 ;;
  --help)     echo "Usage: floter-tool [options]"; exit 0 ;;
  --features) echo "json markdown"; exit 0 ;;
  --defunct)  echo "not supported"; exit 3 ;;
  *) echo "unknown flag: $1" >&2; exit 1 ;;
esac
"#;
        fs::write(path, script).unwrap();
        let mut permissions = fs::metadata(path).unwrap().permissions();
        permissions.set_mode(0o755);
        fs::set_permissions(path, permissions).unwrap();
    }

    #[cfg(windows)]
    fn write_fixture_script(path: &Path) {
        let script = r#"@echo off
if "%1"=="--version" (echo floter-tool 1.2.3 & exit /b 0)
if "%1"=="--help" (echo Usage: floter-tool [options] & exit /b 0)
if "%1"=="--features" (echo json markdown & exit /b 0)
if "%1"=="--defunct" (echo not supported & exit /b 3)
echo unknown flag: %1 1>&2
exit /b 1
"#;
        fs::write(path, script).unwrap();
    }

    /// A tiny executable that mimics a tool supporting `--version`, `--help`
    /// and `--features`, and rejecting `--defunct` with exit code 3. Written
    /// once per process to keep parallel tests from racing on the same file.
    fn fixture_script() -> PathBuf {
        #[cfg(windows)]
        let name = format!("floter-capability-probe-{}.cmd", std::process::id());
        #[cfg(not(windows))]
        let name = format!("floter-capability-probe-{}.sh", std::process::id());
        let path = std::env::temp_dir().join(name);
        static WRITTEN: Once = Once::new();
        WRITTEN.call_once(|| write_fixture_script(&path));
        path
    }

    #[test]
    fn custom_probe_builder_sets_expectations() {
        let probe = CapabilityProbe::custom("json", ["--format", "json"])
            .expect_output("ok")
            .expect_exit_code(None);
        assert_eq!(probe.id, "json");
        assert_eq!(probe.args, vec!["--format", "json"]);
        assert_eq!(probe.expected_exit_code, None);
        assert_eq!(probe.expected_output.as_deref(), Some("ok"));
    }

    #[tokio::test]
    async fn version_probe_reports_tool_version() {
        let result = CapabilityProbe::version().probe(&fixture_script()).await.unwrap();
        assert!(result.passed, "{:?}", result.reason);
        assert_eq!(result.exit_code, Some(0));
        assert!(result.stdout.contains("floter-tool 1.2.3"));
    }

    #[tokio::test]
    async fn help_probe_detects_usage_text() {
        let result = CapabilityProbe::help().probe(&fixture_script()).await.unwrap();
        assert!(result.passed, "{:?}", result.reason);
        assert!(result.stdout.contains("Usage"));
    }

    #[tokio::test]
    async fn custom_probe_confirms_supported_feature() {
        let probe = CapabilityProbe::custom("features", ["--features"]).expect_output("json");
        let result = probe.probe(&fixture_script()).await.unwrap();
        assert!(result.passed, "{:?}", result.reason);
        assert!(result.stdout.contains("markdown"));
    }

    #[tokio::test]
    async fn probe_fails_when_exit_code_mismatches() {
        let result = CapabilityProbe::custom("defunct", ["--defunct"])
            .probe(&fixture_script())
            .await
            .unwrap();
        assert!(!result.passed);
        assert_eq!(result.exit_code, Some(3));
        let reason = result.reason.as_deref().unwrap_or_default();
        assert!(reason.contains("exit code"), "{reason}");
    }

    #[tokio::test]
    async fn probe_fails_when_expected_output_is_missing() {
        let probe = CapabilityProbe::custom("features", ["--help"]).expect_output("json");
        let result = probe.probe(&fixture_script()).await.unwrap();
        assert!(!result.passed);
        let reason = result.reason.as_deref().unwrap_or_default();
        assert!(reason.contains("output"), "{reason}");
    }

    #[tokio::test]
    async fn probe_captures_stderr_from_the_tool() {
        let result = CapabilityProbe::custom("unknown-flag", ["--nope"])
            .probe(&fixture_script())
            .await
            .unwrap();
        assert!(!result.passed);
        assert!(result.stderr.contains("unknown flag"), "{:?}", result.stderr);
    }

    #[tokio::test]
    async fn probe_errors_when_executable_is_missing() {
        let missing = std::env::temp_dir().join(format!(
            "floter-no-such-tool-{}",
            std::process::id()
        ));
        assert!(CapabilityProbe::version().probe(&missing).await.is_err());
    }

    #[tokio::test]
    async fn default_scan_checks_version_and_help() {
        let report = CapabilityScanner::new(fixture_script()).scan().await.unwrap();
        assert_eq!(report.version, "floter-tool 1.2.3");
        assert_eq!(
            report.supported_features,
            vec!["version".to_string(), "help".to_string()]
        );
        assert!(report.limitations.is_empty());
    }

    #[tokio::test]
    async fn report_aggregates_version_features_and_limitations() {
        let probes = [
            CapabilityProbe::version(),
            CapabilityProbe::help(),
            CapabilityProbe::custom("features", ["--features"]).expect_output("json"),
            CapabilityProbe::custom("defunct", ["--defunct"]),
        ];
        let scanner = CapabilityScanner::new(fixture_script());
        let report = scanner.scan_with(&probes).await.unwrap();
        assert_eq!(report.version, "floter-tool 1.2.3");
        assert!(report.supported_features.contains(&"version".to_string()));
        assert!(report.supported_features.contains(&"help".to_string()));
        assert!(report.supported_features.contains(&"features".to_string()));
        assert_eq!(report.limitations.len(), 1);
        assert!(report.limitations[0].contains("defunct"));
        assert!(report.limitations[0].contains("exit code"));
    }

    #[test]
    fn report_uses_unknown_version_when_version_probe_fails() {
        let result = ProbeResult {
            probe: CapabilityProbe::version(),
            exit_code: Some(2),
            stdout: String::new(),
            stderr: String::new(),
            passed: false,
            reason: Some("exit code 2 (expected 0)".into()),
        };
        let report = CapabilityReport::from_probes(&[result]);
        assert_eq!(report.version, UNKNOWN_VERSION);
        assert!(report.supported_features.is_empty());
        assert_eq!(report.limitations.len(), 1);
    }
}
