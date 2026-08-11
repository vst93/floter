pub mod catalog;
pub mod config;
pub mod install;
pub mod lock;
pub mod manifest;
pub mod provider;
pub mod registry;
pub mod static_adapter;
pub mod sync;

use std::collections::HashMap;
use std::path::PathBuf;
use std::time::{Duration, Instant};

const MAX_EXECUTION_PLANS: usize = 1_024;
const EXECUTION_PLAN_TTL: Duration = Duration::from_secs(30 * 60);

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
    pub(crate) provider_commands: catalog::ProviderCommandCache,
    execution_plans: ExecutionPlanCache,
}

#[derive(Default)]
struct ExecutionPlanCache {
    plans: std::sync::Mutex<HashMap<String, (Instant, provider::ExecutionPlan)>>,
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
            .redirect(reqwest::redirect::Policy::custom(|attempt| {
                if attempt.url().scheme() == "https" {
                    attempt.follow()
                } else {
                    attempt.error("refusing redirect to a non-HTTPS URL")
                }
            }))
            .build()
            .map_err(|error| format!("Cannot initialize HTTP client: {error}"))?;
        Ok(Self {
            provider: provider::ProviderManager::new(paths.cache.join("providers")),
            paths,
            client,
            static_adapters,
            mutation_lock: tokio::sync::Mutex::new(()),
            provider_commands: catalog::ProviderCommandCache::default(),
            execution_plans: ExecutionPlanCache::default(),
        })
    }

    pub fn protect_execution_plan(
        &self,
        plan: provider::ExecutionPlan,
    ) -> Result<provider::ExecutionPlan, String> {
        self.execution_plans.protect(plan)
    }

    pub fn take_execution_plan(&self, token: &str) -> Result<provider::ExecutionPlan, String> {
        self.execution_plans.take(token)
    }

    pub async fn invalidate_provider_commands(&self) {
        self.provider_commands.invalidate().await;
    }
}

impl ExecutionPlanCache {
    fn protect(&self, plan: provider::ExecutionPlan) -> Result<provider::ExecutionPlan, String> {
        let token = uuid::Uuid::new_v4().to_string();
        let mut plans = self
            .plans
            .lock()
            .map_err(|_| "Extension execution plan cache is unavailable".to_string())?;
        plans.retain(|_, (created, _)| created.elapsed() <= EXECUTION_PLAN_TTL);
        if plans.len() >= MAX_EXECUTION_PLANS {
            if let Some(oldest) = plans
                .iter()
                .min_by_key(|(_, (created, _))| *created)
                .map(|(token, _)| token.clone())
            {
                plans.remove(&oldest);
            }
        }
        plans.insert(token.clone(), (Instant::now(), plan.clone()));

        let mut protected = plan;
        protected.program.clear();
        protected.args.clear();
        protected.environment.clear();
        protected.cwd = None;
        protected.plan_token = Some(token);
        Ok(protected)
    }

    fn take(&self, token: &str) -> Result<provider::ExecutionPlan, String> {
        let mut plans = self
            .plans
            .lock()
            .map_err(|_| "Extension execution plan cache is unavailable".to_string())?;
        let (created, plan) = plans.remove(token).ok_or_else(|| {
            "Extension execution plan is missing or has already been used".to_string()
        })?;
        if created.elapsed() > EXECUTION_PLAN_TTL {
            return Err("Extension execution plan has expired".to_string());
        }
        Ok(plan)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::provider::{ExecutionMode, ExecutionPlan};
    use std::collections::BTreeMap;

    #[test]
    fn protected_execution_plans_keep_secrets_out_of_ipc_and_are_single_use() {
        let cache = ExecutionPlanCache::default();
        let plan = ExecutionPlan {
            program: "/bin/tool".into(),
            args: vec!["--token".into(), "secret".into()],
            mode: ExecutionMode::Pty,
            cwd: Some("/tmp".into()),
            environment: BTreeMap::from([("API_TOKEN".into(), "secret".into())]),
            inherit_environment: false,
            plan_token: None,
            user_args_start: Some(2),
        };

        let protected = cache.protect(plan.clone()).unwrap();
        assert!(protected.program.is_empty());
        assert!(protected.args.is_empty());
        assert!(protected.environment.is_empty());
        assert!(protected.cwd.is_none());
        let token = protected.plan_token.unwrap();
        let restored = cache.take(&token).unwrap();
        assert_eq!(restored.program, plan.program);
        assert_eq!(restored.args, plan.args);
        assert_eq!(restored.environment, plan.environment);
        assert!(cache.take(&token).is_err());
    }
}
