use semver::{Version, VersionReq};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeMap;
use std::path::{Component, Path, PathBuf};

const MANIFEST_SCHEMA: &str =
    include_str!("../../../docs/extensions/schemas/floter-extension.schema.json");

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ExtensionManifest {
    pub schema_version: String,
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub description: String,
    pub homepage: Option<String>,
    pub icon: Option<String>,
    pub publisher: Publisher,
    pub compatibility: Compatibility,
    pub runtime: Runtime,
    pub provider: ProviderConfig,
    #[serde(default)]
    pub platform_overrides: BTreeMap<String, PlatformOverride>,
    #[serde(default)]
    pub permissions: Vec<Permission>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Publisher {
    pub id: String,
    pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Compatibility {
    pub floter: String,
    pub provider_protocol: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "lowercase", deny_unknown_fields)]
pub enum Runtime {
    Managed {
        #[serde(rename = "platformPackages")]
        platform_packages: BTreeMap<String, String>,
        executable: String,
    },
    Linked {
        #[serde(rename = "executableNames")]
        executable_names: Vec<String>,
        #[serde(rename = "versionArgs", default)]
        version_args: Vec<String>,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ProviderConfig {
    pub args_prefix: Vec<String>,
    #[serde(default = "default_describe_timeout")]
    pub describe_timeout_ms: u64,
    #[serde(default = "default_complete_timeout")]
    pub complete_timeout_ms: u64,
    #[serde(default)]
    pub environment: BTreeMap<String, String>,
}

fn default_describe_timeout() -> u64 {
    5_000
}

fn default_complete_timeout() -> u64 {
    800
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct PlatformOverride {
    pub provider_args_prefix: Option<Vec<String>>,
    #[serde(default)]
    pub environment: BTreeMap<String, String>,
    pub minimum_os_version: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum Permission {
    Terminal,
    FilesystemRead,
    FilesystemWrite,
    ClipboardRead,
    ClipboardWrite,
    Network,
    Process,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum PlatformOs {
    Darwin,
    Linux,
    Windows,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum PlatformArch {
    Arm64,
    X64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct PlatformTarget {
    pub os: PlatformOs,
    pub arch: PlatformArch,
}

impl PlatformTarget {
    pub fn current() -> Result<Self, String> {
        let os = match std::env::consts::OS {
            "macos" => PlatformOs::Darwin,
            "linux" => PlatformOs::Linux,
            "windows" => PlatformOs::Windows,
            other => return Err(format!("Unsupported extension platform: {other}")),
        };
        let arch = match std::env::consts::ARCH {
            "aarch64" => PlatformArch::Arm64,
            "x86_64" => PlatformArch::X64,
            other => return Err(format!("Unsupported extension architecture: {other}")),
        };
        Ok(Self { os, arch })
    }

    pub fn identifier(&self) -> String {
        format!("{}-{}", self.os_name(), self.arch_name())
    }

    pub fn os_identifier(&self) -> String {
        format!("{}-any", self.os_name())
    }

    fn os_name(&self) -> &'static str {
        match self.os {
            PlatformOs::Darwin => "darwin",
            PlatformOs::Linux => "linux",
            PlatformOs::Windows => "windows",
        }
    }

    fn arch_name(&self) -> &'static str {
        match self.arch {
            PlatformArch::Arm64 => "arm64",
            PlatformArch::X64 => "x64",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResolvedManifest {
    pub manifest: ExtensionManifest,
    pub target: PlatformTarget,
    pub provider: ProviderConfig,
    pub minimum_os_version: Option<String>,
    pub platform_package: Option<String>,
}

impl ResolvedManifest {
    pub fn validate_minimum_os_version(&self) -> Result<(), String> {
        let Some(minimum) = self.minimum_os_version.as_deref() else {
            return Ok(());
        };
        let required = numeric_version(minimum)
            .ok_or_else(|| format!("Invalid minimum OS version: {minimum}"))?;
        let current_text = current_os_version()?;
        let current = numeric_version(&current_text)
            .ok_or_else(|| format!("Cannot parse current OS version: {current_text}"))?;
        if current < required {
            return Err(format!(
                "Extension requires {} {} or newer; current version is {}",
                self.target.os_name(),
                minimum,
                current_text.trim()
            ));
        }
        Ok(())
    }
}

impl ExtensionManifest {
    pub fn load(path: &Path) -> Result<Self, String> {
        let bytes = std::fs::read(path)
            .map_err(|error| format!("Cannot read manifest {}: {error}", path.display()))?;
        Self::parse(&bytes)
    }

    pub fn parse(bytes: &[u8]) -> Result<Self, String> {
        let value: Value = serde_json::from_slice(bytes)
            .map_err(|error| format!("Invalid extension manifest JSON: {error}"))?;
        validate_schema(&value)?;
        let manifest: Self = serde_json::from_value(value)
            .map_err(|error| format!("Invalid extension manifest: {error}"))?;
        manifest.validate_paths()?;
        Ok(manifest)
    }

    pub fn resolve(self, target: PlatformTarget) -> Result<ResolvedManifest, String> {
        let mut provider = self.provider.clone();
        let mut minimum_os_version = None;
        for key in [target.identifier(), target.os_identifier()] {
            if let Some(platform_override) = self.platform_overrides.get(&key) {
                if let Some(prefix) = &platform_override.provider_args_prefix {
                    provider.args_prefix.clone_from(prefix);
                }
                provider
                    .environment
                    .extend(platform_override.environment.clone());
                if platform_override.minimum_os_version.is_some() {
                    minimum_os_version.clone_from(&platform_override.minimum_os_version);
                }
            }
        }
        let platform_package = match &self.runtime {
            Runtime::Managed {
                platform_packages, ..
            } => Some(
                platform_packages
                    .get(&target.identifier())
                    .cloned()
                    .ok_or_else(|| {
                        format!(
                            "Extension {} does not support {}",
                            self.id,
                            target.identifier()
                        )
                    })?,
            ),
            Runtime::Linked { .. } => None,
        };
        Ok(ResolvedManifest {
            manifest: self,
            target,
            provider,
            minimum_os_version,
            platform_package,
        })
    }

    pub fn validate_compatibility(&self, host_version: &str) -> Result<(), String> {
        let host = Version::parse(host_version)
            .map_err(|error| format!("Invalid Floter version {host_version}: {error}"))?;
        let host_requirement = VersionReq::parse(&self.compatibility.floter)
            .map_err(|error| format!("Invalid Floter version requirement: {error}"))?;
        if !host_requirement.matches(&host) {
            return Err(format!(
                "Extension {} requires Floter {}, current version is {}",
                self.id, self.compatibility.floter, host
            ));
        }
        let protocol_requirement = VersionReq::parse(&self.compatibility.provider_protocol)
            .map_err(|error| format!("Invalid provider protocol requirement: {error}"))?;
        let protocol = Version::new(1, 0, 0);
        if !protocol_requirement.matches(&protocol) {
            return Err(format!(
                "Extension {} does not support provider protocol 1.0",
                self.id
            ));
        }
        Ok(())
    }

    fn validate_paths(&self) -> Result<(), String> {
        if let Some(icon) = &self.icon {
            validate_relative_path(icon, "icon")?;
        }
        if let Runtime::Managed { executable, .. } = &self.runtime {
            validate_relative_path(executable, "runtime executable")?;
            #[cfg(target_os = "windows")]
            if !executable.to_ascii_lowercase().ends_with(".exe") {
                return Err("Managed Windows runtimes must use an .exe entry point".to_string());
            }
        }
        Ok(())
    }
}

pub fn validate_relative_path(value: &str, field: &str) -> Result<PathBuf, String> {
    let path = Path::new(value);
    if path.as_os_str().is_empty() || path.is_absolute() {
        return Err(format!("{field} must be a non-empty relative path"));
    }
    for component in path.components() {
        if !matches!(component, Component::Normal(_)) {
            return Err(format!(
                "{field} contains an unsafe path component: {value}"
            ));
        }
    }
    Ok(path.to_path_buf())
}

fn validate_schema(instance: &Value) -> Result<(), String> {
    let schema: Value = serde_json::from_str(MANIFEST_SCHEMA)
        .map_err(|error| format!("Bundled manifest schema is invalid: {error}"))?;
    let validator = jsonschema::validator_for(&schema)
        .map_err(|error| format!("Cannot compile manifest schema: {error}"))?;
    let errors = validator
        .iter_errors(instance)
        .map(|error| error.to_string())
        .collect::<Vec<_>>();
    if errors.is_empty() {
        Ok(())
    } else {
        Err(format!(
            "Manifest schema validation failed: {}",
            errors.join("; ")
        ))
    }
}

fn numeric_version(value: &str) -> Option<Version> {
    let start = value.find(|character: char| character.is_ascii_digit())?;
    let numeric = value[start..]
        .chars()
        .take_while(|character| character.is_ascii_digit() || *character == '.')
        .collect::<String>();
    let mut components = numeric
        .split('.')
        .filter(|component| !component.is_empty())
        .map(str::parse::<u64>);
    let major = components.next()?.ok()?;
    let minor = components.next().transpose().ok()?.unwrap_or(0);
    let patch = components.next().transpose().ok()?.unwrap_or(0);
    Some(Version::new(major, minor, patch))
}

#[cfg(target_os = "macos")]
fn current_os_version() -> Result<String, String> {
    command_version("sw_vers", &["-productVersion"])
}

#[cfg(target_os = "linux")]
fn current_os_version() -> Result<String, String> {
    command_version("uname", &["-r"])
}

#[cfg(target_os = "windows")]
fn current_os_version() -> Result<String, String> {
    use windows::Win32::System::SystemInformation::{GetVersionExW, OSVERSIONINFOW};

    let mut information = OSVERSIONINFOW {
        dwOSVersionInfoSize: std::mem::size_of::<OSVERSIONINFOW>() as u32,
        ..Default::default()
    };
    unsafe { GetVersionExW(&mut information) }
        .map_err(|error| format!("Cannot determine Windows version: {error}"))?;
    Ok(format!(
        "{}.{}.{}",
        information.dwMajorVersion, information.dwMinorVersion, information.dwBuildNumber
    ))
}

#[cfg(not(any(target_os = "macos", target_os = "linux", target_os = "windows")))]
fn current_os_version() -> Result<String, String> {
    Err("Minimum OS version checks are unsupported on this platform".to_string())
}

#[cfg(any(target_os = "macos", target_os = "linux"))]
fn command_version(program: &str, args: &[&str]) -> Result<String, String> {
    let output = std::process::Command::new(program)
        .args(args)
        .output()
        .map_err(|error| format!("Cannot determine OS version with {program}: {error}"))?;
    if !output.status.success() {
        return Err(format!("{program} failed while determining the OS version"));
    }
    String::from_utf8(output.stdout)
        .map_err(|_| format!("{program} returned a non-UTF-8 OS version"))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_reference_manifest() {
        let bytes = include_bytes!("../../../docs/extensions/examples/v/floter.extension.json");
        let manifest = ExtensionManifest::parse(bytes).expect("reference manifest");
        assert_eq!(manifest.id, "io.github.vst93.v");
    }

    #[test]
    fn rejects_parent_paths() {
        assert!(validate_relative_path("../bin/tool", "executable").is_err());
        assert!(validate_relative_path("bin/../tool", "executable").is_err());
    }

    #[test]
    fn system_override_is_applied_after_exact_override() {
        let mut manifest = ExtensionManifest::parse(include_bytes!(
            "../../../docs/extensions/examples/v/floter.extension.json"
        ))
        .unwrap();
        manifest.platform_overrides.insert(
            "linux-x64".into(),
            PlatformOverride {
                provider_args_prefix: Some(vec!["exact".into()]),
                ..Default::default()
            },
        );
        manifest.platform_overrides.insert(
            "linux-any".into(),
            PlatformOverride {
                provider_args_prefix: Some(vec!["system".into()]),
                ..Default::default()
            },
        );
        let resolved = manifest
            .resolve(PlatformTarget {
                os: PlatformOs::Linux,
                arch: PlatformArch::X64,
            })
            .unwrap();
        assert_eq!(resolved.provider.args_prefix, ["system"]);
    }

    #[test]
    fn normalizes_platform_versions_for_comparison() {
        assert_eq!(numeric_version("macOS 15.4"), Some(Version::new(15, 4, 0)));
        assert_eq!(
            numeric_version("6.8.12-custom"),
            Some(Version::new(6, 8, 12))
        );
    }
}
