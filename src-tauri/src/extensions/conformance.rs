use crate::extensions::config::{self, ConfigurationDescriptor};
use crate::extensions::manifest::{validate_relative_path, ExtensionManifest, Permission, Runtime};
use crate::extensions::provider::ProviderDescription;
use serde::Deserialize;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DocumentReport {
    pub extension_id: String,
    pub provider_version: String,
    pub command_count: usize,
}

pub fn validate_documents(
    manifest_bytes: &[u8],
    description_bytes: &[u8],
    host_version: &str,
) -> Result<DocumentReport, String> {
    let manifest = ExtensionManifest::parse(manifest_bytes)?;
    manifest.validate_compatibility(host_version)?;
    let description = ProviderDescription::parse(description_bytes)?;
    validate_provider_id(&manifest, &description)?;
    validate_execution_contract(&manifest, &description)?;
    Ok(DocumentReport {
        extension_id: manifest.id,
        provider_version: description.provider.version,
        command_count: description.commands.len(),
    })
}

pub fn validate_execution_contract(
    manifest: &ExtensionManifest,
    description: &ProviderDescription,
) -> Result<(), String> {
    for command in &description.commands {
        if command.execution.program == "self" {
            continue;
        }
        validate_relative_path(&command.execution.program, "command execution program")?;
        if !manifest.permissions.contains(&Permission::ProcessSpawn) {
            return Err(format!(
                "Command {} requires the process-spawn permission",
                command.id
            ));
        }
        if !matches!(manifest.runtime, Runtime::Bundled { .. }) {
            return Err(format!(
                "Linked command {} may only execute self",
                command.id
            ));
        }
    }
    Ok(())
}

pub fn validate_provider_id(
    manifest: &ExtensionManifest,
    description: &ProviderDescription,
) -> Result<(), String> {
    if manifest.id == description.provider.id {
        Ok(())
    } else {
        Err(format!(
            "Provider id {} does not match extension id {}",
            description.provider.id, manifest.id
        ))
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ConfigurationEnvelope {
    configuration: ConfigurationDescriptor,
}

pub fn validate_configuration_response(bytes: &[u8]) -> Result<(), String> {
    let envelope: ConfigurationEnvelope = serde_json::from_slice(bytes)
        .map_err(|error| format!("Invalid provider configuration response: {error}"))?;
    config::validate_descriptor(&envelope.configuration)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn fixture(path: &str) -> Vec<u8> {
        let fixtures = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../docs/extensions/sdk/fixtures");
        std::fs::read(fixtures.join(path))
            .unwrap_or_else(|error| panic!("cannot read fixture {path}: {error}"))
    }

    #[test]
    fn valid_fixture_matches_host_contract() {
        let report = validate_documents(
            &fixture("valid/floter.extension.json"),
            &fixture("valid/provider-description.json"),
            env!("CARGO_PKG_VERSION"),
        )
        .unwrap();

        assert_eq!(report.extension_id, "com.example.fixture-tools");
        assert_eq!(report.command_count, 1);
    }

    #[test]
    fn invalid_provider_fixture_matrix_is_rejected() {
        for path in [
            "invalid/provider-flag-takes-value.json",
            "invalid/provider-enum-without-values.json",
            "invalid/provider-host-config-without-schema.json",
            "invalid/provider-tool-config-with-schema.json",
        ] {
            let result = ProviderDescription::parse(&fixture(path));
            assert!(result.is_err(), "fixture unexpectedly passed: {path}");
        }
    }

    #[test]
    fn mismatched_ids_are_rejected() {
        let mut description: serde_json::Value =
            serde_json::from_slice(&fixture("valid/provider-description.json")).unwrap();
        description["provider"]["id"] = "com.example.other".into();

        let error = validate_documents(
            &fixture("valid/floter.extension.json"),
            &serde_json::to_vec(&description).unwrap(),
            env!("CARGO_PKG_VERSION"),
        )
        .unwrap_err();
        assert!(error.contains("does not match extension id"));
    }
}
