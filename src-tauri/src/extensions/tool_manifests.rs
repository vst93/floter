//! Convention-location manifest discovery (`<config>/floter/tools/*.json`).
//!
//! Users and other tools can drop ordinary floter extension manifests into
//! Floter's config directory; every valid `distribution: local` manifest with
//! an installable runtime becomes a connection suggestion. Scanning is a
//! cheap directory read: unreadable or invalid files are skipped silently so
//! a bad user file can never break the suggestion list.

use crate::extensions::manifest::{Distribution, ExtensionManifest, ProviderKind, Runtime};
use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};

/// Directory scanned for manifest suggestions. Derived from the same config
/// root as every other extension path (`<config>/floter/tools`).
pub fn directory_for_root(root: &Path) -> PathBuf {
    root.join("tools")
}

#[derive(Debug, Clone)]
pub struct DiscoveredManifest {
    /// The manifest file inside the tools directory.
    pub source_path: PathBuf,
    /// File stem of `source_path`, used to locate the sibling descriptor.
    pub stem: String,
    pub manifest: ExtensionManifest,
    /// Raw manifest bytes, retained so connecting writes the authored JSON
    /// verbatim instead of a serde round-trip that could reorder or drop
    /// fields.
    pub manifest_bytes: Vec<u8>,
    /// Raw bytes of the sibling `<stem>.description.json`, when present and
    /// parseable. `None` means connecting falls back to a generated generic
    /// single-command descriptor, exactly like PATH-discovered tools.
    pub descriptor_bytes: Option<Vec<u8>>,
}

impl DiscoveredManifest {
    /// True when connecting needs a static descriptor file beside the
    /// manifest; executable-provider manifests describe themselves at
    /// runtime and never need one.
    pub fn requires_descriptor(&self) -> bool {
        self.manifest.provider.kind == ProviderKind::StaticDescriptor
    }
}

/// Scan the convention-location directory. Deterministic order (sorted file
/// names); the first manifest wins when two files declare the same id.
pub fn scan(directory: &Path) -> Vec<DiscoveredManifest> {
    let Ok(entries) = fs::read_dir(directory) else {
        return Vec::new();
    };
    let mut paths: Vec<PathBuf> = entries
        .flatten()
        .map(|entry| entry.path())
        .filter(|path| is_manifest_path(path))
        .collect();
    paths.sort();
    let mut seen_ids = BTreeSet::new();
    let mut discovered = Vec::new();
    for path in paths {
        if let Some(tool) = load(&path) {
            if seen_ids.insert(tool.manifest.id.clone()) {
                discovered.push(tool);
            }
        }
    }
    discovered
}

/// Find one discovered manifest by its extension id.
pub fn find(directory: &Path, id: &str) -> Option<DiscoveredManifest> {
    scan(directory)
        .into_iter()
        .find(|tool| tool.manifest.id == id)
}

fn is_manifest_path(path: &Path) -> bool {
    let Some(name) = path.file_name().and_then(|value| value.to_str()) else {
        return false;
    };
    // `<stem>.description.json` files are sibling payloads, not manifests.
    name.ends_with(".json") && !name.ends_with(".description.json")
}

fn load(path: &Path) -> Option<DiscoveredManifest> {
    let bytes = fs::read(path).ok()?;
    let manifest = ExtensionManifest::parse(&bytes).ok()?;
    // Only locally distributed tools with a connectable runtime qualify;
    // NPM/built-in distributions do not belong in this directory.
    if manifest.distribution != Distribution::Local {
        return None;
    }
    if !matches!(
        manifest.runtime,
        Runtime::System { .. } | Runtime::Script { .. }
    ) {
        return None;
    }
    manifest
        .validate_compatibility(env!("CARGO_PKG_VERSION"))
        .ok()?;
    let stem = path
        .file_stem()
        .and_then(|value| value.to_str())?
        .to_string();
    // An unparseable sibling falls back to the generic single-command
    // descriptor instead of poisoning a valid manifest.
    let descriptor_bytes =
        sibling_descriptor_bytes(path, &stem).filter(|bytes| {
            crate::extensions::provider::ProviderDescription::parse(bytes).is_ok()
        });
    Some(DiscoveredManifest {
        source_path: path.to_path_buf(),
        stem,
        manifest,
        manifest_bytes: bytes,
        descriptor_bytes,
    })
}

fn sibling_descriptor_bytes(path: &Path, stem: &str) -> Option<Vec<u8>> {
    let descriptor = path.with_file_name(format!("{stem}.description.json"));
    fs::read(descriptor).ok()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::provider::ProviderDescription;

    const MANIFEST_JSON: &str = r#"{
        "schemaVersion": "2.0",
        "id": "local.demo.tool",
        "name": "Demo Tool",
        "publisher": { "id": "demo", "name": "Demo" },
        "compatibility": { "floter": ">=0.1.0", "providerProtocol": "^1.0" },
        "distribution": { "type": "local" },
        "runtime": {
            "type": "system",
            "executableNames": ["demo-tool"],
            "versionArgs": ["--version"]
        },
        "provider": {
            "type": "static-descriptor",
            "descriptor": "provider-description.json",
            "argsPrefix": []
        },
        "permissions": ["environment"]
    }"#;

    const DESCRIPTION_JSON: &str = r#"{
        "protocolVersion": "1.0",
        "provider": {
            "id": "local.demo.tool",
            "name": "Demo Tool",
            "version": "1.2.3",
            "description": "Demo description"
        },
        "commands": [
            {
                "id": "run",
                "name": "Run",
                "description": "Run the demo tool",
                "execution": {
                    "program": "self",
                    "argsPrefix": [],
                    "mode": "pty",
                    "workingDirectory": "current"
                }
            }
        ]
    }"#;

    fn write_tools_dir(files: &[(&str, &[u8])]) -> tempfile::TempDir {
        let directory = tempfile::tempdir().unwrap();
        for (name, contents) in files {
            std::fs::write(directory.path().join(name), contents).unwrap();
        }
        directory
    }

    #[test]
    fn discovers_a_valid_local_manifest() {
        let directory = write_tools_dir(&[("demo.json", MANIFEST_JSON.as_bytes())]);
        let tools = scan(&directory.path().join("tools"));
        assert!(tools.is_empty(), "missing directory yields no suggestions");

        let tools = scan(directory.path());
        assert_eq!(tools.len(), 1);
        let tool = &tools[0];
        assert_eq!(tool.manifest.id, "local.demo.tool");
        assert_eq!(tool.stem, "demo");
        assert_eq!(
            tool.source_path,
            directory.path().join("demo.json"),
            "the authored manifest is honored verbatim, not regenerated"
        );
        assert!(tool.requires_descriptor());
        assert!(tool.descriptor_bytes.is_none());
    }

    #[test]
    fn invalid_files_are_skipped_without_breaking_discovery() {
        let directory = write_tools_dir(&[
            ("broken.json", b"{ not json".as_slice()),
            ("empty.json", b"".as_slice()),
            ("npm.json", br#"{"schemaVersion":"2.0"}"#),
            (
                "wrong-distribution.json",
                br#"{
                    "schemaVersion": "2.0",
                    "id": "local.wrong.dist",
                    "name": "Wrong",
                    "publisher": { "id": "d", "name": "D" },
                    "compatibility": { "floter": ">=0.1.0", "providerProtocol": "^1.0" },
                    "distribution": { "type": "npm" },
                    "runtime": { "type": "system", "executableNames": ["x"] },
                    "provider": { "type": "executable", "argsPrefix": [] }
                }"#,
            ),
            ("demo.json", MANIFEST_JSON.as_bytes()),
            // Sibling descriptors are payloads, not manifests themselves.
            ("demo.description.json", DESCRIPTION_JSON.as_bytes()),
            ("notes.txt", b"ignore me".as_slice()),
        ]);
        let tools = scan(directory.path());
        assert_eq!(tools.len(), 1);
        assert_eq!(tools[0].manifest.id, "local.demo.tool");
    }

    #[test]
    fn duplicate_ids_keep_the_first_file_in_sorted_order() {
        let directory = write_tools_dir(&[
            ("a-first.json", MANIFEST_JSON.as_bytes()),
            (
                "z-second.json",
                br#"{
                    "schemaVersion": "2.0",
                    "id": "local.demo.tool",
                    "name": "Second Demo",
                    "publisher": { "id": "demo", "name": "Demo" },
                    "compatibility": { "floter": ">=0.1.0", "providerProtocol": "^1.0" },
                    "distribution": { "type": "local" },
                    "runtime": { "type": "system", "executableNames": ["other"] },
                    "provider": { "type": "executable", "argsPrefix": [] }
                }"#,
            ),
        ]);
        let tools = scan(directory.path());
        assert_eq!(tools.len(), 1);
        assert_eq!(tools[0].stem, "a-first");
    }

    #[test]
    fn sibling_description_is_honored_when_present() {
        let directory = write_tools_dir(&[
            ("demo.json", MANIFEST_JSON.as_bytes()),
            ("demo.description.json", DESCRIPTION_JSON.as_bytes()),
        ]);
        let tools = scan(directory.path());
        assert_eq!(tools.len(), 1);
        let description = tools[0].descriptor_bytes.as_ref().unwrap();
        let parsed = ProviderDescription::parse(description).unwrap();
        assert_eq!(parsed.provider.id, "local.demo.tool");
        assert_eq!(parsed.provider.version, "1.2.3");
        assert_eq!(parsed.commands.len(), 1);

        // An unparseable sibling falls back to no descriptor instead of
        // poisoning the whole entry.
        let fallback = write_tools_dir(&[
            ("demo.json", MANIFEST_JSON.as_bytes()),
            ("demo.description.json", b"garbage".as_slice()),
        ]);
        let tools = scan(fallback.path());
        assert!(tools[0].descriptor_bytes.is_none());
    }

    #[test]
    fn find_resolves_by_extension_id() {
        let directory = write_tools_dir(&[("demo.json", MANIFEST_JSON.as_bytes())]);
        assert!(find(directory.path(), "local.demo.tool").is_some());
        assert!(find(directory.path(), "local.other.tool").is_none());
    }

    #[test]
    fn executable_provider_manifests_need_no_descriptor() {
        let directory = write_tools_dir(&[(
            "plain.json",
            br#"{
                "schemaVersion": "2.0",
                "id": "local.plain.tool",
                "name": "Plain Tool",
                "publisher": { "id": "demo", "name": "Demo" },
                "compatibility": { "floter": ">=0.1.0", "providerProtocol": "^1.0" },
                "distribution": { "type": "local" },
                "runtime": { "type": "script", "language": "shell", "path": "run.sh" },
                "provider": { "type": "executable", "argsPrefix": [] }
            }"#,
        )]);
        let tools = scan(directory.path());
        assert_eq!(tools.len(), 1);
        assert!(!tools[0].requires_descriptor());
    }
}
