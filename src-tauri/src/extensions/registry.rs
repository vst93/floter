use crate::extensions::lock::ExtensionLockEntry;
use crate::extensions::manifest::{ExtensionManifest, PlatformTarget, Runtime, ScriptLanguage};
use crate::extensions::provider::{
    validate_execution_descriptors, ProviderDescription, ProviderInvocation,
};
use std::path::{Path, PathBuf};

pub fn provider_invocation(entry: &ExtensionLockEntry) -> Result<ProviderInvocation, String> {
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    provider_invocation_with_manifest(entry, &manifest)
}

pub(crate) fn provider_invocation_with_manifest(
    entry: &ExtensionLockEntry,
    manifest: &ExtensionManifest,
) -> Result<ProviderInvocation, String> {
    if manifest.id != entry.id || manifest.publisher.id != entry.publisher_id {
        return Err(format!(
            "Manifest identity does not match lock entry {}",
            entry.id
        ));
    }
    let version_args = match &manifest.runtime {
        Runtime::System { version_args, .. } => version_args.clone(),
        Runtime::Script { version_args, .. } => version_args.clone(),
        Runtime::Bundled { .. } => Vec::new(),
    };
    let permissions = manifest.permissions.clone();
    let resolved = manifest.clone().resolve(PlatformTarget::current()?)?;
    let (executable, executable_prefix) = resolve_runtime_target(
        manifest,
        Path::new(&entry.manifest_path),
        Path::new(&entry.executable_path),
    )?;
    Ok(ProviderInvocation {
        extension_id: entry.id.clone(),
        executable,
        executable_prefix,
        runtime_root: entry.runtime_root.as_ref().map(PathBuf::from),
        package_version: entry.package_version.clone(),
        tool_version_hint: entry.tool_version.clone(),
        version_args,
        config: resolved.provider,
        permissions,
    })
}

pub(crate) fn resolve_runtime_target(
    manifest: &ExtensionManifest,
    manifest_path: &Path,
    executable: &Path,
) -> Result<(PathBuf, Vec<String>), String> {
    match &manifest.runtime {
        Runtime::Script { language, path, .. } => {
            let script = manifest_path
                .parent()
                .ok_or("Script manifest has no parent directory")?
                .join(path);
            if !script.is_file() {
                return Err(format!("Script file is missing: {}", script.display()));
            }
            let args = match language {
                ScriptLanguage::Js | ScriptLanguage::Shell => {
                    vec![script.to_string_lossy().into_owned()]
                }
                ScriptLanguage::Powershell => {
                    vec!["-File".into(), script.to_string_lossy().into_owned()]
                }
            };
            Ok((super::install::find_script_interpreter(*language)?, args))
        }
        _ => Ok((executable.to_path_buf(), Vec::new())),
    }
}

pub fn runtime_available(entry: &ExtensionLockEntry) -> bool {
    provider_invocation(entry).is_ok_and(|invocation| invocation.executable.is_file())
}

pub fn static_description(
    entry: &ExtensionLockEntry,
) -> Result<(ProviderDescription, ProviderInvocation), String> {
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    let descriptor = manifest
        .provider
        .descriptor
        .as_deref()
        .ok_or("Static provider descriptor path is missing")?;
    let path = Path::new(&entry.manifest_path)
        .parent()
        .ok_or("Static provider manifest has no parent directory")?
        .join(descriptor);
    let description = ProviderDescription::parse(&std::fs::read(&path).map_err(|error| {
        format!(
            "Cannot read static provider descriptor {}: {error}",
            path.display()
        )
    })?)?;
    if description.provider.id != entry.id {
        return Err(format!(
            "Provider id {} does not match extension id {}",
            description.provider.id, entry.id
        ));
    }
    let invocation = provider_invocation(entry)?;
    validate_execution_descriptors(&description, &invocation)?;
    Ok((description, invocation))
}
