use semver::{Version, VersionReq};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::path::{Component, Path, PathBuf};

use crate::extensions::lifecycle::ToolLifecycle;

#[allow(unused_imports)]
pub use crate::extensions::platform::{PlatformArch, PlatformOs, PlatformTarget};

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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub homepage: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub icon: Option<String>,
    pub publisher: Publisher,
    pub compatibility: Compatibility,
    pub distribution: Distribution,
    pub runtime: Runtime,
    pub provider: ProviderConfig,
    /// Optional OS allow-list. An omitted list keeps backwards compatibility
    /// and means all supported host operating systems.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub platforms: Vec<PlatformOs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signatures: Option<SignatureConfig>,
    #[serde(default)]
    pub platform_overrides: BTreeMap<String, PlatformOverride>,
    #[serde(default)]
    pub permissions: Vec<Permission>,
    #[serde(default, skip_serializing_if = "ToolLifecycle::is_empty")]
    pub lifecycle: ToolLifecycle,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "type", rename_all = "kebab-case", deny_unknown_fields)]
pub enum Distribution {
    Local,
    BuiltIn,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SignatureConfig {
    pub url: String,
    pub public_key: String,
    pub algorithm: SignatureAlgorithm,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SignatureAlgorithm {
    Ed25519,
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
#[serde(tag = "type", rename_all = "kebab-case", deny_unknown_fields)]
pub enum Runtime {
    System {
        #[serde(rename = "executableNames")]
        executable_names: Vec<String>,
        #[serde(rename = "versionArgs", default)]
        version_args: Vec<String>,
    },
    Script {
        language: ScriptLanguage,
        path: String,
        #[serde(rename = "versionArgs", default)]
        version_args: Vec<String>,
    },
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ScriptLanguage {
    Js,
    Shell,
    Powershell,
    Python,
    Ruby,
    Php,
    Go,
    Rust,
}

impl ScriptLanguage {
    /// Every variant, in the order the editor offers them. Kept here (next to
    /// the enum) so a new language is a compile error away from every consumer
    /// rather than a silently missing entry in one of them.
    pub const ALL: [ScriptLanguage; 8] = [
        ScriptLanguage::Js,
        ScriptLanguage::Shell,
        ScriptLanguage::Powershell,
        ScriptLanguage::Python,
        ScriptLanguage::Ruby,
        ScriptLanguage::Php,
        ScriptLanguage::Go,
        ScriptLanguage::Rust,
    ];

    /// The wire/manifest spelling (`"go"`, `"powershell"`). Also the cache key
    /// for the PATH scan, so it must stay stable.
    pub fn as_str(self) -> &'static str {
        match self {
            ScriptLanguage::Js => "js",
            ScriptLanguage::Shell => "shell",
            ScriptLanguage::Powershell => "powershell",
            ScriptLanguage::Python => "python",
            ScriptLanguage::Ruby => "ruby",
            ScriptLanguage::Php => "php",
            ScriptLanguage::Go => "go",
            ScriptLanguage::Rust => "rust",
        }
    }

    /// Compiled languages are *source-distributed build scripts*: there is no
    /// interpreter to hand the source to. `go`/`rustc` are the **toolchain**
    /// that turns the source into the artifact Floter actually runs, and the
    /// artifact is what the runtime binding resolves to. Every branch that
    /// treats a script as "interpreter + source path" has to ask this first.
    pub fn is_compiled(self) -> bool {
        matches!(self, ScriptLanguage::Go | ScriptLanguage::Rust)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ProviderConfig {
    #[serde(rename = "type")]
    pub kind: ProviderKind,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub descriptor: Option<String>,
    pub args_prefix: Vec<String>,
    #[serde(default = "default_describe_timeout")]
    pub describe_timeout_ms: u64,
    #[serde(default = "default_complete_timeout")]
    pub complete_timeout_ms: u64,
    #[serde(default)]
    pub environment: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ProviderKind {
    Executable,
    StaticDescriptor,
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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub provider_args_prefix: Option<Vec<String>>,
    #[serde(default)]
    pub environment: BTreeMap<String, String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub minimum_os_version: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "kebab-case")]
pub enum Permission {
    FilesystemRead,
    FilesystemWrite,
    NetworkFetch,
    ProcessSpawn,
    ClipboardRead,
    ClipboardWrite,
    Environment,
}

/// Whether the Host itself decides a permission, or merely discloses it.
///
/// R7-8a · The trust-boundary distinction the review UI draws. `environment`
/// and `process-spawn` are the only permissions the Host refuses at execution
/// time (`conformance.rs`/`provider.rs` gate descriptor-driven program starts,
/// `install.rs`/`lifecycle.rs` clear a probe's environment). Everything else —
/// filesystem, network, clipboard — is a declaration the user reviews; the
/// provider still runs with the Host's own operating-system rights, there is
/// no sandbox.
///
/// This is the authority the review UI mirrors. The TypeScript tier vocabulary
/// (`src/extensions/permission-tiers.ts`) and the canonical name table below
/// are pinned against each other by `tests/permission-tiers.test.ts`, and the
/// classifier is locked against that table by a unit test here — so the two
/// languages cannot drift on which permissions the Host really blocks.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum PermissionEnforcement {
    /// The Host owns the yes/no and refuses an unapproved request.
    Enforced,
    /// Declared for review; the Host does not intercept it.
    Disclosed,
}

/// Canonical kebab-case names of every host-enforced permission, in a stable
/// order. Greppable on purpose: the node parity test reads this declaration as
/// text and compares it to `HOST_ENFORCED_PERMISSIONS`.
pub const HOST_ENFORCED_PERMISSION_NAMES: [&str; 2] = ["environment", "process-spawn"];

/// Classify one permission by *who owns the decision*. The match is exhaustive
/// over the closed enum, so a newly added permission cannot silently default to
/// the flattering `Enforced` — adding a variant is a compile error until it is
/// classified on purpose.
pub const fn permission_enforcement(permission: Permission) -> PermissionEnforcement {
    match permission {
        Permission::Environment | Permission::ProcessSpawn => PermissionEnforcement::Enforced,
        Permission::FilesystemRead
        | Permission::FilesystemWrite
        | Permission::NetworkFetch
        | Permission::ClipboardRead
        | Permission::ClipboardWrite => PermissionEnforcement::Disclosed,
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResolvedManifest {
    pub manifest: ExtensionManifest,
    pub target: PlatformTarget,
    pub provider: ProviderConfig,
    pub minimum_os_version: Option<String>,
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

    /// Load a manifest and return the SHA-256 of its exact bytes. Install and
    /// approval flows use the digest to bind a permission approval to the
    /// manifest version the user actually saw.
    pub fn load_with_digest(path: &Path) -> Result<(Self, String), String> {
        let bytes = std::fs::read(path)
            .map_err(|error| format!("Cannot read manifest {}: {error}", path.display()))?;
        let digest = Self::digest_of(&bytes);
        Ok((Self::parse(&bytes)?, digest))
    }

    /// Lowercase hex SHA-256 with the standard algorithm prefix. The same
    /// format is stored in the lock's `approvedManifestDigest`.
    pub fn digest_of(bytes: &[u8]) -> String {
        let digest = Sha256::digest(bytes);
        let mut hex = String::with_capacity(digest.len() * 2 + 7);
        hex.push_str("sha256-");
        for byte in digest {
            hex.push_str(&format!("{byte:02x}"));
        }
        hex
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
        if !self.platforms.is_empty() && !self.platforms.contains(&target.os) {
            return Err(format!(
                "Extension {} does not support {}",
                self.id,
                target.os_name()
            ));
        }
        let mut provider = self.provider.clone();
        let mut minimum_os_version = None;
        for key in target.override_identifiers() {
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
        Ok(ResolvedManifest {
            manifest: self,
            target,
            provider,
            minimum_os_version,
        })
    }

    pub fn validate_compatibility(&self, host_version: &str) -> Result<(), String> {
        let host = Version::parse(host_version)
            .map_err(|error| format!("Invalid Floter version {host_version}: {error}"))?;
        // Pre-release versions (for example, "0.3.0-preview") do not match
        // comparison requirements such as ">=0.2.3" per the semver spec.
        // Strip the pre-release suffix for compatibility checking so preview
        // builds can still load extensions targeting the same release line.
        let host_for_match = if host.pre.is_empty() {
            host.clone()
        } else {
            Version::new(host.major, host.minor, host.patch)
        };
        let host_requirement = VersionReq::parse(&self.compatibility.floter)
            .map_err(|error| format!("Invalid Floter version requirement: {error}"))?;
        if !host_requirement.matches(&host_for_match) {
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
        match (self.distribution, &self.runtime, self.provider.kind) {
            (
                Distribution::Local,
                Runtime::System { .. },
                ProviderKind::Executable | ProviderKind::StaticDescriptor,
            )
            | (
                Distribution::Local,
                Runtime::Script { .. },
                ProviderKind::Executable | ProviderKind::StaticDescriptor,
            )
            | (Distribution::BuiltIn, Runtime::System { .. }, ProviderKind::StaticDescriptor) => {}
            _ => {
                return Err(format!(
                    "Unsupported manifest combination: distribution={:?}, runtime={:?}, provider={:?}",
                    self.distribution, self.runtime, self.provider.kind
                ));
            }
        }
        if let Some(icon) = &self.icon {
            validate_relative_path(icon, "icon")?;
        }
        if let Runtime::Script { path, .. } = &self.runtime {
            validate_relative_path(path, "runtime script")?;
        }
        if let Some(descriptor) = &self.provider.descriptor {
            validate_relative_path(descriptor, "provider descriptor")?;
        }
        self.lifecycle.validate()?;
        if self.distribution == Distribution::Local
            && self.provider.kind == ProviderKind::StaticDescriptor
            && self.provider.descriptor.is_none()
        {
            return Err("Static descriptor providers require provider.descriptor".to_string());
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
        .map(|error| format!("{}: {error}", error.instance_path))
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
        assert_eq!(manifest.schema_version, "2.0");
        assert_eq!(manifest.distribution, Distribution::Local);
        assert_eq!(manifest.provider.kind, ProviderKind::Executable);
        assert!(matches!(manifest.runtime, Runtime::System { .. }));
        assert_eq!(
            manifest.permissions,
            vec![
                Permission::FilesystemRead,
                Permission::FilesystemWrite,
                Permission::NetworkFetch,
                Permission::ProcessSpawn,
                Permission::ClipboardRead,
                Permission::ClipboardWrite,
                Permission::Environment,
            ]
        );
    }

    #[test]
    fn parses_v2_static_system_integration() {
        let bytes = include_bytes!("../../../extensions/v-tools/floter.extension.json");
        let manifest = ExtensionManifest::parse(bytes).unwrap();

        assert_eq!(manifest.schema_version, "2.0");
        assert_eq!(manifest.distribution, Distribution::Local);
        assert_eq!(manifest.provider.kind, ProviderKind::StaticDescriptor);
        assert!(matches!(manifest.runtime, Runtime::System { .. }));
    }

    #[test]
    fn parses_local_script_runtime_and_rejects_an_unlisted_platform() {
        let value = serde_json::json!({
            "schemaVersion": "2.0",
            "id": "local.script-tool",
            "name": "Script tool",
            "publisher": { "id": "local-user", "name": "Local user" },
            "compatibility": { "floter": ">=0.3.2", "providerProtocol": "^1.0" },
            "distribution": { "type": "local" },
            "runtime": { "type": "script", "language": "js", "path": "provider.js" },
            "provider": { "type": "executable", "argsPrefix": ["--floter"] },
            "platforms": ["linux"]
        });
        let manifest = ExtensionManifest::parse(&serde_json::to_vec(&value).unwrap()).unwrap();
        assert!(matches!(
            manifest.runtime,
            Runtime::Script {
                language: ScriptLanguage::Js,
                ..
            }
        ));
        let unsupported = PlatformTarget::new(PlatformOs::Windows, PlatformArch::X64);
        assert!(manifest
            .resolve(unsupported)
            .unwrap_err()
            .contains("does not support windows"));
    }

    /// The NPM distribution dimension (and with it the bundled runtime,
    /// platform packages, and artifact shim chain) was physically removed. A
    /// legacy manifest that still declares `"type": "npm"` no longer resolves:
    /// the closed enum rejects the variant instead of silently accepting a
    /// distribution the Host cannot install. This is the serde compatibility
    /// lock for old manifest files.
    #[test]
    fn rejects_the_removed_npm_distribution_variant() {
        let value = serde_json::json!({
            "schemaVersion": "2.0",
            "id": "legacy.npm-tool",
            "name": "Legacy NPM tool",
            "publisher": { "id": "example", "name": "Example" },
            "compatibility": { "floter": ">=0.3.0", "providerProtocol": "^1.0" },
            "distribution": { "type": "npm" },
            "runtime": { "type": "system", "executableNames": ["tool"] },
            "provider": { "type": "executable", "argsPrefix": [] }
        });
        let error = ExtensionManifest::parse(&serde_json::to_vec(&value).unwrap()).unwrap_err();
        assert!(
            error.contains("npm"),
            "the removed npm distribution must surface a clear schema error: {error}"
        );
    }

    /// The manifest-level rejection above is satisfied by the JSON schema, so
    /// it would stay green if someone merely revived the enum variant. This is
    /// the layer-specific lock: deserializing the `Distribution` type itself
    /// must reject `npm`, so a revived variant turns this red even while the
    /// schema still guards the full parse.
    #[test]
    fn distribution_serde_rejects_the_removed_npm_variant_without_the_schema() {
        for distribution in ["npm", "local", "built-in"] {
            let value = serde_json::json!({ "type": distribution });
            let parsed = serde_json::from_value::<Distribution>(value);
            if distribution == "npm" {
                let error = parsed.expect_err("npm must not deserialize into Distribution");
                assert!(
                    error.to_string().contains("npm"),
                    "the error must name the rejected variant: {error}"
                );
            } else {
                assert!(parsed.is_ok(), "{distribution} must still deserialize");
            }
        }
    }

    /// The bundled runtime went with the NPM distribution. A manifest that
    /// still carries `"type": "bundled"` (and its platform packages) is now an
    /// unknown runtime variant rather than a silently accepted no-op.
    #[test]
    fn rejects_the_removed_bundled_runtime_variant() {
        let value = serde_json::json!({
            "schemaVersion": "2.0",
            "id": "legacy.bundled-tool",
            "name": "Legacy bundled tool",
            "publisher": { "id": "example", "name": "Example" },
            "compatibility": { "floter": ">=0.3.0", "providerProtocol": "^1.0" },
            "distribution": { "type": "local" },
            "runtime": {
                "type": "bundled",
                "platformPackages": { "linux-x86_64-gnu": "floter-v-linux-x64" },
                "executable": "bin/tool"
            },
            "provider": { "type": "executable", "argsPrefix": [] }
        });
        assert!(ExtensionManifest::parse(&serde_json::to_vec(&value).unwrap()).is_err());
    }

    /// An `artifacts` block was only meaningful for a bundled runtime, which no
    /// longer exists. `ExtensionManifest` is closed and carries no `artifacts`
    /// field, so a manifest that still declares one is rejected rather than
    /// having its binary list silently ignored.
    #[test]
    fn rejects_the_removed_artifacts_block() {
        let value = serde_json::json!({
            "schemaVersion": "2.0",
            "id": "legacy.artifacts-tool",
            "name": "Legacy artifacts tool",
            "publisher": { "id": "example", "name": "Example" },
            "compatibility": { "floter": ">=0.3.0", "providerProtocol": "^1.0" },
            "distribution": { "type": "local" },
            "runtime": { "type": "system", "executableNames": ["tool"] },
            "artifacts": {
                "binaries": [
                    { "name": "tool", "path": "bin/tool", "role": "public" }
                ]
            },
            "provider": { "type": "executable", "argsPrefix": [] }
        });
        assert!(ExtensionManifest::parse(&serde_json::to_vec(&value).unwrap()).is_err());
    }

    #[test]
    fn parses_optional_ed25519_signature_config() {
        let mut value: Value = serde_json::from_slice(include_bytes!(
            "../../../docs/extensions/examples/v/floter.extension.json"
        ))
        .unwrap();
        value["signatures"] = serde_json::json!({
            "url": "https://example.com/floter-v-tools-1.0.0.sig",
            "publicKey": "ed25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
            "algorithm": "ed25519"
        });

        let manifest = ExtensionManifest::parse(&serde_json::to_vec(&value).unwrap()).unwrap();
        let signatures = manifest.signatures.expect("signature config");
        assert_eq!(signatures.algorithm, SignatureAlgorithm::Ed25519);
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
            "linux-x86_64-gnu".into(),
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
                libc: Some(crate::extensions::platform::PlatformLibc::Gnu),
                abi: None,
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

    #[test]
    fn accepts_pre_release_host_versions_for_compatible_extensions() {
        let mut manifest = ExtensionManifest::parse(include_bytes!(
            "../../../docs/extensions/examples/v/floter.extension.json"
        ))
        .unwrap();
        manifest.compatibility.floter = ">=0.2.3".to_string();

        assert!(manifest.validate_compatibility("0.3.0-preview").is_ok());
    }

    // R7-8a · the classification is the source of truth the review UI mirrors,
    // so every kind is pinned explicitly. A new permission variant must be
    // classified here on purpose — the exhaustive match is the compile-time
    // half, this is the behavioural half.
    #[test]
    fn classifies_every_permission_by_who_owns_the_decision() {
        assert_eq!(
            permission_enforcement(Permission::Environment),
            PermissionEnforcement::Enforced
        );
        assert_eq!(
            permission_enforcement(Permission::ProcessSpawn),
            PermissionEnforcement::Enforced
        );
        for permission in [
            Permission::FilesystemRead,
            Permission::FilesystemWrite,
            Permission::NetworkFetch,
            Permission::ClipboardRead,
            Permission::ClipboardWrite,
        ] {
            assert_eq!(
                permission_enforcement(permission),
                PermissionEnforcement::Disclosed,
                "{permission:?} is declared, not intercepted"
            );
        }
    }

    // The canonical name list is what the TypeScript vocabulary is compared to,
    // so it must name exactly the permissions the classifier calls enforced.
    // Change the classifier alone and the list disagrees; change the list alone
    // and the sweep below disagrees.
    #[test]
    fn the_canonical_enforced_names_match_the_classifier() {
        let mut enforced: Vec<&str> = ALL_PERMISSIONS
            .iter()
            .filter(|permission| {
                permission_enforcement(**permission) == PermissionEnforcement::Enforced
            })
            .map(|permission| permission_wire_name(*permission))
            .collect();
        // The enum declaration order is not the presentation order; the claim
        // under test is *which* permissions are enforced, so compare as a set.
        enforced.sort_unstable();
        let mut canonical = HOST_ENFORCED_PERMISSION_NAMES.to_vec();
        canonical.sort_unstable();
        assert_eq!(enforced, canonical);
        // …and the serialized names really are the kebab-case the UI keys on.
        for name in HOST_ENFORCED_PERMISSION_NAMES {
            assert_eq!(
                serde_json::to_value(permission_from_wire_name(name)).unwrap(),
                serde_json::Value::String(name.to_string()),
                "{name} must round-trip through the serde representation"
            );
        }
    }

    // Local helpers for the two tests above: the enum is closed, so a table in
    // the test is the honest way to enumerate it without a nightly iterator.
    const ALL_PERMISSIONS: [Permission; 7] = [
        Permission::FilesystemRead,
        Permission::FilesystemWrite,
        Permission::NetworkFetch,
        Permission::ProcessSpawn,
        Permission::ClipboardRead,
        Permission::ClipboardWrite,
        Permission::Environment,
    ];

    fn permission_wire_name(permission: Permission) -> &'static str {
        match permission {
            Permission::FilesystemRead => "filesystem-read",
            Permission::FilesystemWrite => "filesystem-write",
            Permission::NetworkFetch => "network-fetch",
            Permission::ProcessSpawn => "process-spawn",
            Permission::ClipboardRead => "clipboard-read",
            Permission::ClipboardWrite => "clipboard-write",
            Permission::Environment => "environment",
        }
    }

    fn permission_from_wire_name(name: &str) -> Permission {
        ALL_PERMISSIONS
            .into_iter()
            .find(|permission| permission_wire_name(*permission) == name)
            .expect("the canonical name list names a real permission")
    }
}
