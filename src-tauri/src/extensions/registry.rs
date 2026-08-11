use crate::extensions::lock::ExtensionLockEntry;
use crate::extensions::manifest::{ExtensionManifest, PlatformTarget, Runtime};
use crate::extensions::provider::ProviderInvocation;
use std::path::{Path, PathBuf};

pub fn provider_invocation(entry: &ExtensionLockEntry) -> Result<ProviderInvocation, String> {
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path))?;
    if manifest.id != entry.id || manifest.publisher.id != entry.publisher_id {
        return Err(format!(
            "Manifest identity does not match lock entry {}",
            entry.id
        ));
    }
    let version_args = match &manifest.runtime {
        Runtime::Linked { version_args, .. } => version_args.clone(),
        Runtime::Managed { .. } => Vec::new(),
    };
    let permissions = manifest.permissions.clone();
    let resolved = manifest.resolve(PlatformTarget::current()?)?;
    Ok(ProviderInvocation {
        extension_id: entry.id.clone(),
        executable: PathBuf::from(&entry.executable_path),
        runtime_root: entry.runtime_root.as_ref().map(PathBuf::from),
        package_version: entry.package_version.clone(),
        tool_version_hint: entry.tool_version.clone(),
        version_args,
        config: resolved.provider,
        permissions,
    })
}
