//! Shipped tool recommendations.
//!
//! Recommended tools are ordinary recommendation data: a manifest plus a
//! static provider descriptor. They carry no adapter-specific runtime
//! behavior; connecting one materializes the payloads into the standard
//! integration directory and runs the same linked-install pipeline as every
//! other local tool (`install::connect_recommended_tool`).

use crate::extensions::manifest::{Distribution, ExtensionManifest, ProviderKind, Runtime};
use crate::extensions::provider::ProviderDescription;

const RECOMMENDED_PAYLOADS: &[(&str, &[u8], &[u8])] = &[(
    "v-tools",
    include_bytes!("../../../extensions/v-tools/floter.extension.json"),
    include_bytes!("../../../extensions/v-tools/provider-description.json"),
)];

#[derive(Debug, Clone)]
pub struct RecommendedTool {
    pub manifest: ExtensionManifest,
    pub description: ProviderDescription,
    /// Raw payload bytes, retained so connecting writes the shipped JSON
    /// verbatim instead of a serde round-trip that can emit `null` for
    /// omitted optional fields.
    pub manifest_bytes: &'static [u8],
    pub descriptor_bytes: &'static [u8],
}

pub fn load_recommended() -> Result<Vec<RecommendedTool>, String> {
    RECOMMENDED_PAYLOADS
        .iter()
        .map(|(name, manifest_bytes, description_bytes)| {
            load(name, manifest_bytes, description_bytes)
        })
        .collect()
}

fn load(
    name: &str,
    manifest_bytes: &'static [u8],
    description_bytes: &'static [u8],
) -> Result<RecommendedTool, String> {
    let manifest = ExtensionManifest::parse(manifest_bytes)
        .map_err(|error| format!("Invalid recommended tool {name} manifest: {error}"))?;
    if manifest.distribution != Distribution::Local
        || !matches!(manifest.runtime, Runtime::System { .. })
        || manifest.provider.kind != ProviderKind::StaticDescriptor
    {
        return Err(format!(
            "Recommended tool {name} must use a local distribution, system runtime, and static-descriptor provider"
        ));
    }
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let description = ProviderDescription::parse(description_bytes)
        .map_err(|error| format!("Invalid recommended tool {name} description: {error}"))?;
    if description.provider.id != manifest.id {
        return Err(format!(
            "Recommended tool {name} provider id {} does not match manifest id {}",
            description.provider.id, manifest.id
        ));
    }
    Ok(RecommendedTool {
        manifest,
        description,
        manifest_bytes,
        descriptor_bytes: description_bytes,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn loads_the_recommended_v_tools_as_data() {
        let tools = load_recommended().expect("recommended v-tools");
        assert_eq!(tools.len(), 1);
        let tool = &tools[0];
        assert_eq!(tool.manifest.id, "io.github.vst93.v");
        assert_eq!(tool.manifest.distribution, Distribution::Local);
        assert!(matches!(tool.manifest.runtime, Runtime::System { .. }));
        assert_eq!(
            tool.description.commands
                .iter()
                .map(|command| command.id.as_str())
                .collect::<Vec<_>>(),
            ["jv", "diff", "codec", "genpwd", "tt"]
        );
        assert_eq!(tool.description.provider.id, tool.manifest.id);
    }
}
