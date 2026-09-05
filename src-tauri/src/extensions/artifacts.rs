use crate::extensions::lock::{sync_directory, ExtensionDistributionSource, ExtensionLockEntry};
use crate::extensions::manifest::{Artifacts, BinaryRole, ExtensionManifest, Permission};
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::time::Duration;

const BINARY_METADATA_DIR: &str = ".floter-binaries";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct VerifiedBinary {
    pub name: String,
    pub path: String,
    pub role: BinaryRole,
}

pub async fn verify_binaries(
    runtime_root: &Path,
    artifacts: &Artifacts,
    permissions: &[Permission],
) -> Result<Vec<VerifiedBinary>, String> {
    verify_binaries_with_timeout(runtime_root, artifacts, permissions, Duration::from_secs(2)).await
}

async fn verify_binaries_with_timeout(
    runtime_root: &Path,
    artifacts: &Artifacts,
    permissions: &[Permission],
    probe_timeout: Duration,
) -> Result<Vec<VerifiedBinary>, String> {
    if artifacts.is_empty() {
        return Ok(Vec::new());
    }
    let canonical_root = runtime_root.canonicalize().map_err(|error| {
        format!(
            "Cannot resolve artifact runtime root {}: {error}",
            runtime_root.display()
        )
    })?;
    let mut verified = Vec::new();
    for binary in &artifacts.binaries {
        let path = runtime_root.join(&binary.path);
        if !path.is_file() {
            if binary.required {
                return Err(format!(
                    "Required artifact binary {} is missing: {}",
                    binary.name,
                    path.display()
                ));
            }
            continue;
        }
        let canonical = path.canonicalize().map_err(|error| {
            format!("Cannot resolve artifact binary {}: {error}", path.display())
        })?;
        if !canonical.starts_with(&canonical_root) {
            return Err(format!(
                "Artifact binary {} escaped the runtime root",
                binary.name
            ));
        }
        crate::extensions::install::make_executable(&path)?;
        if !binary.version_args.is_empty() {
            let mut command = tokio::process::Command::new(&path);
            command
                .args(&binary.version_args)
                .stdin(Stdio::null())
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .kill_on_drop(true);
            if !permissions.contains(&Permission::Environment) {
                command.env_clear();
            }
            let status = tokio::time::timeout(probe_timeout, command.status())
                .await
                .map_err(|_| format!("Artifact binary {} version probe timed out", binary.name))?
                .map_err(|error| {
                    format!(
                        "Cannot run artifact binary {} version probe: {error}",
                        binary.name
                    )
                })?;
            if !status.success() {
                return Err(format!(
                    "Artifact binary {} version probe failed with status {}",
                    binary.name, status
                ));
            }
        }
        verified.push(VerifiedBinary {
            name: binary.name.clone(),
            path: binary.path.clone(),
            role: binary.role,
        });
    }
    Ok(verified)
}

pub fn prepare_shim_metadata(
    version_root: &Path,
    binaries: &[VerifiedBinary],
) -> Result<(), String> {
    let public = binaries
        .iter()
        .filter(|binary| binary.role == BinaryRole::Public)
        .collect::<Vec<_>>();
    if public.is_empty() {
        return Ok(());
    }
    let directory = version_root.join(BINARY_METADATA_DIR);
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("Cannot create binary shim metadata directory: {error}"))?;
    for binary in public {
        let relative = Path::new("runtime").join(&binary.path);
        let mut content = relative.to_string_lossy().replace('\\', "/");
        content.push('\n');
        atomic_write(
            &directory.join(format!("{}.path", binary.name)),
            content.as_bytes(),
        )?;
    }
    sync_directory(&directory).map_err(|error| format!("Cannot sync binary shim metadata: {error}"))
}

pub fn activate_entry_shims(
    extensions_dir: &Path,
    entry: &ExtensionLockEntry,
) -> Result<(), String> {
    if entry.distribution_source != ExtensionDistributionSource::Npm {
        return Ok(());
    }
    let extension_root = extensions_dir.join(&entry.id);
    let metadata = extension_root
        .join("versions")
        .join(&entry.current_version)
        .join(BINARY_METADATA_DIR);
    if !metadata.is_dir() {
        return Ok(());
    }
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    if manifest.artifacts.is_empty() {
        return Ok(());
    }
    for binary in manifest
        .artifacts
        .binaries
        .iter()
        .filter(|binary| binary.role == BinaryRole::Public)
    {
        if metadata.join(format!("{}.path", binary.name)).is_file() {
            write_stable_shim(&extension_root, &binary.name)?;
        } else if binary.required {
            return Err(format!(
                "Required public binary {} has no shim metadata",
                binary.name
            ));
        }
    }
    Ok(())
}

fn write_stable_shim(extension_root: &Path, name: &str) -> Result<(), String> {
    let directory = extension_root.join("shims");
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("Cannot create extension shim directory: {error}"))?;
    let path = shim_path(&directory, name);
    let script = shim_script(name);
    if path.exists() {
        let existing = std::fs::read(&path)
            .map_err(|error| format!("Cannot inspect existing extension shim: {error}"))?;
        if existing != script.as_bytes() {
            return Err(format!(
                "Refusing to overwrite an unmanaged extension shim: {}",
                path.display()
            ));
        }
        crate::extensions::install::make_executable(&path)?;
        return Ok(());
    }
    atomic_write(&path, script.as_bytes())?;
    crate::extensions::install::make_executable(&path)?;
    sync_directory(&directory).map_err(|error| format!("Cannot sync extension shims: {error}"))
}

fn shim_path(directory: &Path, name: &str) -> PathBuf {
    #[cfg(target_os = "windows")]
    {
        directory.join(format!("{name}.cmd"))
    }
    #[cfg(not(target_os = "windows"))]
    {
        directory.join(name)
    }
}

#[cfg(not(target_os = "windows"))]
fn shim_script(name: &str) -> String {
    format!(
        r#"#!/bin/sh
set -eu
shim_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
pointer="$shim_dir/../current.json"
version=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$pointer")
case "$version" in
  ""|*[!A-Za-z0-9._+-]*) echo "floter: invalid extension version pointer" >&2; exit 126 ;;
esac
metadata="$shim_dir/../versions/$version/{BINARY_METADATA_DIR}/{name}.path"
IFS= read -r relative < "$metadata"
case "$relative" in
  ""|/*|../*|*/../*|*/..) echo "floter: invalid binary shim target" >&2; exit 126 ;;
esac
exec "$shim_dir/../versions/$version/$relative" "$@"
"#
    )
}

#[cfg(target_os = "windows")]
fn shim_script(name: &str) -> String {
    format!(
        "@echo off\r\nsetlocal\r\nset \"SHIM_DIR=%~dp0\"\r\nfor /f \"usebackq delims=\" %%V in (`powershell.exe -NoProfile -NonInteractive -Command \"$p=Join-Path $env:SHIM_DIR '..\\current.json'; (Get-Content -Raw -LiteralPath $p ^| ConvertFrom-Json).version\"`) do set \"FLOTER_VERSION=%%V\"\r\nif not defined FLOTER_VERSION exit /b 126\r\nset \"FLOTER_METADATA=%SHIM_DIR%..\\versions\\%FLOTER_VERSION%\\{BINARY_METADATA_DIR}\\{name}.path\"\r\nset /p FLOTER_RELATIVE=<\"%FLOTER_METADATA%\"\r\nif not defined FLOTER_RELATIVE exit /b 126\r\nset \"FLOTER_TARGET=%SHIM_DIR%..\\versions\\%FLOTER_VERSION%\\%FLOTER_RELATIVE:/=\\%\"\r\n\"%FLOTER_TARGET%\" %*\r\nexit /b %ERRORLEVEL%\r\n"
    )
}

fn atomic_write(path: &Path, bytes: &[u8]) -> Result<(), String> {
    let parent = path.parent().ok_or("Invalid extension shim path")?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create extension shim temporary file: {error}"))?;
    temporary
        .write_all(bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write extension shim: {error}"))?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| format!("Cannot persist extension shim: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::manifest::{ArtifactBinary, Artifacts};

    fn artifact(name: &str, path: &str, role: BinaryRole, required: bool) -> ArtifactBinary {
        ArtifactBinary {
            name: name.into(),
            path: path.into(),
            role,
            version_args: Vec::new(),
            required,
        }
    }

    #[tokio::test]
    async fn required_binaries_are_verified_individually() {
        let directory = tempfile::tempdir().unwrap();
        let runtime = directory.path().join("runtime");
        std::fs::create_dir_all(&runtime).unwrap();
        std::fs::write(runtime.join("primary"), b"primary").unwrap();
        let artifacts = Artifacts {
            binaries: vec![
                artifact("primary", "primary", BinaryRole::Provider, true),
                artifact("helper", "helper", BinaryRole::Helper, true),
            ],
        };

        let error = verify_binaries(&runtime, &artifacts, &[])
            .await
            .unwrap_err();
        assert!(error.contains("Required artifact binary helper is missing"));
    }

    #[tokio::test]
    async fn missing_optional_binaries_are_not_activated() {
        let directory = tempfile::tempdir().unwrap();
        let runtime = directory.path().join("runtime");
        std::fs::create_dir_all(&runtime).unwrap();
        let artifacts = Artifacts {
            binaries: vec![artifact("optional", "optional", BinaryRole::Public, false)],
        };

        assert!(verify_binaries(&runtime, &artifacts, &[])
            .await
            .unwrap()
            .is_empty());
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn failing_version_probe_rejects_the_binary_set() {
        // A concurrently forked child can inherit even a private fixture's write
        // descriptor until exec. Never write the executable during the test run.
        let runtime = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures");
        let mut binary = artifact("tool", "failing-probe.sh", BinaryRole::Public, true);
        binary.version_args = vec!["--version".into()];
        let artifacts = Artifacts {
            binaries: vec![binary],
        };

        let started = std::time::Instant::now();
        let error =
            verify_binaries_with_timeout(&runtime, &artifacts, &[], Duration::from_secs(10))
                .await
                .unwrap_err();
        assert!(
            error.contains("version probe failed"),
            "Unexpected probe result after {:?}: {error}",
            started.elapsed()
        );
    }

    #[test]
    fn unmanaged_shims_are_never_overwritten() {
        let directory = tempfile::tempdir().unwrap();
        let extension = directory.path().join("example.tool");
        let shims = extension.join("shims");
        std::fs::create_dir_all(&shims).unwrap();
        std::fs::write(shim_path(&shims, "tool"), b"unmanaged").unwrap();

        assert!(write_stable_shim(&extension, "tool")
            .unwrap_err()
            .contains("Refusing to overwrite"));
    }

    #[cfg(unix)]
    #[test]
    fn stable_shim_follows_the_current_pointer() {
        let directory = tempfile::tempdir().unwrap();
        let extension = directory.path().join("example.tool");
        for version in ["1.0.0", "2.0.0"] {
            let root = extension.join("versions").join(version);
            let runtime = root.join("runtime");
            std::fs::create_dir_all(&runtime).unwrap();
            let tool = runtime.join("tool");
            std::fs::write(&tool, format!("#!/bin/sh\nprintf '%s' '{version}'\n")).unwrap();
            crate::extensions::install::make_executable(&tool).unwrap();
            prepare_shim_metadata(
                &root,
                &[VerifiedBinary {
                    name: "tool".into(),
                    path: "tool".into(),
                    role: BinaryRole::Public,
                }],
            )
            .unwrap();
        }
        write_stable_shim(&extension, "tool").unwrap();
        let shim = extension.join("shims/tool");

        for version in ["1.0.0", "2.0.0"] {
            std::fs::write(
                extension.join("current.json"),
                format!("{{\n  \"version\": \"{version}\"\n}}"),
            )
            .unwrap();
            let output = std::process::Command::new(&shim).output().unwrap();
            assert!(output.status.success());
            assert_eq!(String::from_utf8(output.stdout).unwrap(), version);
        }
    }
}
