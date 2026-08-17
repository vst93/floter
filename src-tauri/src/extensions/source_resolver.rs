use crate::extensions::gitlab_source;
use crate::extensions::source_bundle;
use crate::extensions::ExtensionState;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Deserialize)]
#[serde(tag = "provider", rename_all = "lowercase")]
pub enum SourceResolveRequest {
    Gitlab(GitlabSourceRequest),
    Bundle(BundleSourceRequest),
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GitlabSourceRequest {
    pub server_url: String,
    pub project: String,
    pub reference: Option<String>,
    pub access_token: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BundleSourceRequest {
    pub path: String,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct SourceResolution {
    pub provider: SourceProvider,
    pub server_url: String,
    pub project: String,
    pub requested_reference: Option<String>,
    pub resolved_reference: String,
    pub revision: String,
    pub project_root: String,
    pub archive_sha256: String,
    pub cached: bool,
}

#[derive(Debug, Clone, Copy, Serialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum SourceProvider {
    Gitlab,
    Bundle,
}

pub async fn resolve(
    state: &ExtensionState,
    request: SourceResolveRequest,
) -> Result<SourceResolution, String> {
    match request {
        SourceResolveRequest::Gitlab(request) => gitlab_source::resolve(state, request).await,
        SourceResolveRequest::Bundle(request) => {
            let cache_root = state.paths.cache.clone();
            tokio::task::spawn_blocking(move || source_bundle::import(&cache_root, &request))
                .await
                .map_err(|error| format!("Source bundle import task failed: {error}"))?
        }
    }
}
