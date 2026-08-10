use crate::extensions::manifest::{ExtensionManifest, PlatformTarget, Runtime};
use crate::extensions::provider::{
    validate_execution_descriptors, ProviderDescription, ProviderInvocation,
};
use std::ffi::OsStr;
use std::path::{Path, PathBuf};

const BUNDLED_ADAPTERS: &[(&str, &[u8], &[u8])] = &[(
    "v-tools",
    include_bytes!("../../../extensions/v-tools/floter.extension.json"),
    include_bytes!("../../../extensions/v-tools/provider-description.json"),
)];

#[derive(Debug, Clone)]
pub struct StaticAdapter {
    pub manifest: ExtensionManifest,
    pub description: ProviderDescription,
    pub invocation: ProviderInvocation,
    pub runtime_available: bool,
}

pub fn load_bundled() -> Result<Vec<StaticAdapter>, String> {
    let search_path = std::env::var_os("PATH");
    BUNDLED_ADAPTERS
        .iter()
        .map(|(name, manifest, description)| {
            load(name, manifest, description, search_path.as_deref())
        })
        .collect()
}

fn load(
    name: &str,
    manifest_bytes: &[u8],
    description_bytes: &[u8],
    search_path: Option<&OsStr>,
) -> Result<StaticAdapter, String> {
    let manifest = ExtensionManifest::parse(manifest_bytes)
        .map_err(|error| format!("Invalid static adapter {name} manifest: {error}"))?;
    if !matches!(manifest.runtime, Runtime::Linked { .. }) {
        return Err(format!("Static adapter {name} must use a linked runtime"));
    }
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let description = ProviderDescription::parse(description_bytes)
        .map_err(|error| format!("Invalid static adapter {name} description: {error}"))?;
    if description.provider.id != manifest.id {
        return Err(format!(
            "Static adapter {name} provider id {} does not match manifest id {}",
            description.provider.id, manifest.id
        ));
    }

    let executable = find_linked_executable(&manifest, search_path);
    let runtime_available = executable.is_some();
    let fallback_executable = match &manifest.runtime {
        Runtime::Linked {
            executable_names, ..
        } => PathBuf::from(&executable_names[0]),
        Runtime::Managed { .. } => unreachable!(),
    };
    let resolved = manifest.clone().resolve(PlatformTarget::current()?)?;
    resolved.validate_minimum_os_version()?;
    let version_args = match &manifest.runtime {
        Runtime::Linked { version_args, .. } => version_args.clone(),
        Runtime::Managed { .. } => unreachable!(),
    };
    let invocation = ProviderInvocation {
        extension_id: manifest.id.clone(),
        executable: executable.unwrap_or(fallback_executable),
        runtime_root: None,
        package_version: description.provider.version.clone(),
        tool_version_hint: Some(description.provider.version.clone()),
        version_args,
        config: resolved.provider,
    };
    validate_execution_descriptors(&description, &invocation)?;

    Ok(StaticAdapter {
        manifest,
        description,
        invocation,
        runtime_available,
    })
}

fn find_linked_executable(
    manifest: &ExtensionManifest,
    search_path: Option<&OsStr>,
) -> Option<PathBuf> {
    let Runtime::Linked {
        executable_names, ..
    } = &manifest.runtime
    else {
        return None;
    };
    let search_path = search_path?;
    for directory in std::env::split_paths(search_path) {
        for name in executable_names {
            for candidate_name in linked_candidate_names(name) {
                let candidate = directory.join(candidate_name);
                if is_linked_executable(&candidate) {
                    return Some(candidate);
                }
            }
        }
    }
    None
}

fn linked_candidate_names(name: &str) -> Vec<String> {
    #[cfg(target_os = "windows")]
    {
        if Path::new(name).extension().is_some() {
            vec![name.to_string()]
        } else {
            [".exe", ".cmd", ".bat"]
                .into_iter()
                .map(|extension| format!("{name}{extension}"))
                .collect()
        }
    }
    #[cfg(not(target_os = "windows"))]
    {
        vec![name.to_string()]
    }
}

fn is_linked_executable(path: &Path) -> bool {
    if !path.is_file() {
        return false;
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        path.metadata()
            .is_ok_and(|metadata| metadata.permissions().mode() & 0o111 != 0)
    }
    #[cfg(windows)]
    {
        path.extension()
            .and_then(|extension| extension.to_str())
            .is_some_and(|extension| {
                matches!(
                    extension.to_ascii_lowercase().as_str(),
                    "exe" | "cmd" | "bat"
                )
            })
    }
    #[cfg(not(any(unix, windows)))]
    {
        true
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn loads_the_bundled_v_adapter() {
        let adapter = load(
            BUNDLED_ADAPTERS[0].0,
            BUNDLED_ADAPTERS[0].1,
            BUNDLED_ADAPTERS[0].2,
            None,
        )
        .expect("bundled V adapter");
        let commands = adapter
            .description
            .commands
            .iter()
            .map(|command| command.id.as_str())
            .collect::<Vec<_>>();
        assert_eq!(commands, ["jv", "diff", "codec", "genpwd", "tt"]);
        assert!(!adapter.runtime_available);
    }

    #[cfg(unix)]
    #[test]
    fn resolves_a_linked_executable_from_path() {
        use crate::extensions::provider::execution_plan;
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let executable = directory.path().join("v");
        std::fs::write(&executable, b"#!/bin/sh\n").unwrap();
        let mut permissions = executable.metadata().unwrap().permissions();
        permissions.set_mode(0o755);
        std::fs::set_permissions(&executable, permissions).unwrap();

        let adapter = load(
            BUNDLED_ADAPTERS[0].0,
            BUNDLED_ADAPTERS[0].1,
            BUNDLED_ADAPTERS[0].2,
            Some(directory.path().as_os_str()),
        )
        .unwrap();
        assert!(adapter.runtime_available);
        assert_eq!(adapter.invocation.executable, executable);

        let command = adapter
            .description
            .commands
            .iter()
            .find(|command| command.id == "jv")
            .unwrap();
        let plan = execution_plan(
            command,
            &adapter.invocation,
            vec!["-file".into(), "data.json".into()],
            Some(directory.path()),
        )
        .unwrap();
        assert_eq!(plan.program, executable.to_string_lossy());
        assert_eq!(plan.args, ["jv", "-file", "data.json"]);
    }
}
