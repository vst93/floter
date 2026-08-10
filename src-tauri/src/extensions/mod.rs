pub mod catalog;
pub mod config;
pub mod install;
pub mod lock;
pub mod manifest;
pub mod provider;
pub mod static_adapter;

use std::path::PathBuf;

#[allow(unused_imports)]
pub use catalog::{
    CatalogCompletionResponse, CatalogEntry, CatalogSearchRequest, CompletionRequest,
};
#[allow(unused_imports)]
pub use config::{ConfigurationDescriptor, ExtensionConfiguration};
#[allow(unused_imports)]
pub use install::{ExtensionInstallRequest, ExtensionPermissionReview, ExtensionSearchResult};
#[allow(unused_imports)]
pub use lock::{ExtensionInstallType, ExtensionLockEntry, ExtensionStateKind};
#[allow(unused_imports)]
pub use manifest::{ExtensionManifest, PlatformTarget, ResolvedManifest};
#[allow(unused_imports)]
pub use provider::{ExecutionMode, ExecutionPlan, ProviderDescription, ProviderResponse};

#[derive(Debug, Clone)]
pub struct ExtensionPaths {
    pub root: PathBuf,
    pub extensions: PathBuf,
    pub data: PathBuf,
    pub cache: PathBuf,
    pub lock_file: PathBuf,
}

impl ExtensionPaths {
    pub fn discover() -> Result<Self, String> {
        let root = dirs::config_dir()
            .ok_or_else(|| "Cannot find config directory".to_string())?
            .join("floter");
        Ok(Self::from_root(root))
    }

    pub fn from_root(root: PathBuf) -> Self {
        Self {
            extensions: root.join("extensions"),
            data: root.join("extension-data"),
            cache: root.join("extension-cache"),
            lock_file: root.join("extensions.lock.json"),
            root,
        }
    }

    pub fn ensure(&self) -> Result<(), String> {
        for path in [&self.root, &self.extensions, &self.data, &self.cache] {
            std::fs::create_dir_all(path)
                .map_err(|error| format!("Cannot create {}: {error}", path.display()))?;
        }
        Ok(())
    }
}

pub struct ExtensionState {
    pub paths: ExtensionPaths,
    pub client: reqwest::Client,
    pub provider: provider::ProviderManager,
    pub static_adapters: Vec<static_adapter::StaticAdapter>,
    pub(crate) mutation_lock: tokio::sync::Mutex<()>,
}

impl ExtensionState {
    pub fn new() -> Result<Self, String> {
        let paths = ExtensionPaths::discover()?;
        paths.ensure()?;
        if let Err(error) = catalog::migrate_legacy_commands(&paths) {
            eprintln!("failed to migrate legacy custom commands: {error}");
        }
        let static_adapters = static_adapter::load_bundled()?;
        let client = reqwest::Client::builder()
            .user_agent(format!("floter/{}", env!("CARGO_PKG_VERSION")))
            .build()
            .map_err(|error| format!("Cannot initialize HTTP client: {error}"))?;
        Ok(Self {
            provider: provider::ProviderManager::new(paths.cache.join("providers")),
            paths,
            client,
            static_adapters,
            mutation_lock: tokio::sync::Mutex::new(()),
        })
    }
}
