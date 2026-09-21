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

/// The interpreter language this runtime binds **by name**, when the runtime is
/// an *interpreted* script. `None` for every other runtime:
///
/// * a system tool's identity is the external executable the user chose;
/// * a compiled script's identity is the artifact Floter built, which is a
///   real file whose fingerprint is meaningful.
///
/// Only an interpreter is re-resolved through the host search path on every
/// check, because only an interpreter is expected to be replaced in place by a
/// toolchain upgrade.
pub(crate) fn script_interpreter_language(manifest: &ExtensionManifest) -> Option<ScriptLanguage> {
    match &manifest.runtime {
        Runtime::Script { language, .. } if !language.is_compiled() => Some(*language),
        _ => None,
    }
}

/// [`script_interpreter_language`] for an entry, reading (and parsing) the
/// installed manifest. A manifest that will not load yields `None`: the binding
/// then falls back to the frozen-path semantics and the catalog's own manifest
/// validation reports the real failure.
pub(crate) fn entry_script_interpreter_language(
    entry: &ExtensionLockEntry,
) -> Option<ScriptLanguage> {
    let manifest = ExtensionManifest::load(Path::new(&entry.manifest_path)).ok()?;
    script_interpreter_language(&manifest)
}

/// Whether the host's search path currently resolves the language's
/// interpreter. The single answer both the binding check and the run path use,
/// so they cannot disagree about whether a script integration is usable.
pub(crate) fn script_interpreter_is_available(language: ScriptLanguage) -> bool {
    super::install::find_script_interpreter(language).is_ok()
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
            let root = manifest_path
                .parent()
                .ok_or("Script manifest has no parent directory")?;
            let script = root.join(path);
            if !script.is_file() {
                return Err(format!("Script file is missing: {}", script.display()));
            }
            // Compiled languages have no interpreter to hand the source to:
            // the toolchain produced an artifact at connect time and *that* is
            // the program. The source path is not passed as an argument, and
            // `find_script_interpreter` (which would resolve `go`/`rustc`) is
            // deliberately not called here.
            if language.is_compiled() {
                let artifact = super::install::script_build_output(root, *language);
                if !artifact.is_file() {
                    return Err(format!(
                        "Compiled script artifact is missing: {}; save the integration to rebuild it",
                        artifact.display()
                    ));
                }
                return Ok((artifact, Vec::new()));
            }
            let args = match language {
                ScriptLanguage::Powershell => {
                    vec!["-File".into(), script.to_string_lossy().into_owned()]
                }
                // js / shell / python / ruby / php all take the script path as
                // their first positional argument.
                _ => vec![script.to_string_lossy().into_owned()],
            };
            // The plain resolver: its message is shown verbatim by the install
            // path and the details drawer, both of which render a sentence, not
            // a keyed payload. The *run* path pre-checks with the keyed
            // resolver (`run::build_plan`) so a manual run can name the binary
            // and the directories searched (R9-5).
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
    static_description_with_manifest(entry, &manifest)
}

/// Same as [`static_description`] but reuses an already-parsed manifest so
/// read paths that need the manifest for other purposes do not parse it twice.
pub(crate) fn static_description_with_manifest(
    entry: &ExtensionLockEntry,
    manifest: &ExtensionManifest,
) -> Result<(ProviderDescription, ProviderInvocation), String> {
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
    let invocation = provider_invocation_with_manifest(entry, manifest)?;
    validate_execution_descriptors(&description, &invocation)?;
    Ok((description, invocation))
}
