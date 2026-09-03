pub mod artifacts;
pub mod asset_matcher;
pub mod capability_probe;
pub mod catalog;
pub mod config;
pub mod conformance;
pub mod cwd_policy;
pub mod health;
pub mod help_args;
pub mod install;
pub mod inventory;
pub mod lifecycle;
pub mod lock;
pub mod manifest;
pub mod official_index;
pub mod platform;
pub mod probe;
pub mod probe_executor;
pub mod probe_runner;
pub mod profile;
pub mod provider;
mod proxy;
pub mod recommendations;
pub mod registry;
pub mod resolver;
pub mod session_restore;
pub mod sync;
pub mod terminal_capability;
pub mod tool_lock;
pub mod tool_manifests;
pub(crate) mod transaction;

use std::collections::HashMap;
use std::path::PathBuf;
use std::time::{Duration, Instant};

const MAX_EXECUTION_PLANS: usize = 1_024;
const EXECUTION_PLAN_TTL: Duration = Duration::from_secs(30 * 60);

pub use capability_probe::{CapabilityProbe, CapabilityReport};
pub use catalog::{
    CatalogCompletionResponse, CatalogEntry, CatalogSearchRequest, CompletionRequest,
};
pub use config::{ConfigurationDescriptor, ExtensionConfiguration};
pub use cwd_policy::{CwdContext, CwdPolicy};
pub use install::{ExtensionInstallRequest, ExtensionPermissionReview};
pub use inventory::{
    DiscoveryQuality, DiscoverySource, ToolCandidate, ToolInventory, ToolInventorySnapshot,
    ToolLocator,
};
pub use lock::{
    ExtensionDistributionSource, ExtensionLockEntry, ExtensionProviderKind,
    ExtensionRuntimeOwnership, ExtensionStateKind,
};
pub use manifest::{ExtensionManifest, ResolvedManifest};
pub use platform::{PlatformAbi, PlatformArch, PlatformLibc, PlatformOs, PlatformTarget};
pub use probe::{ProbeQuality, ProbeResult, ProviderDescriber};
pub use profile::{Profile, ProfileKind, ProfileStack};
pub use provider::{ExecutionMode, ExecutionPlan, ProviderDescription, ProviderResponse};
pub use resolver::{ResolveRequest, ResolveResult, ScoreBreakdown, ScoredCandidate};
pub use terminal_capability::{
    Da1Report, DecrqmResult, DecrqmState, Negotiation, ProbeReport, TerminalCapability,
    TerminalColor, TerminalIo,
};
pub use tool_lock::{LockState, ToolLock, ToolLockEntry};

#[derive(Debug, Clone)]
pub struct ExtensionPaths {
    pub root: PathBuf,
    pub extensions: PathBuf,
    pub data: PathBuf,
    pub cache: PathBuf,
    pub lock_file: PathBuf,
    pub tool_lock_file: PathBuf,
    pub official_index_state_file: PathBuf,
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
            tool_lock_file: root.join("tool-lock.json"),
            official_index_state_file: root.join("official-index-state.json"),
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
    pub official_index: official_index::OfficialIndexConfig,
    pub provider: provider::ProviderManager,
    pub recommendations: Vec<recommendations::RecommendedTool>,
    pub(crate) mutation_lock: tokio::sync::Mutex<()>,
    pub(crate) provider_commands: catalog::ProviderCommandCache,
    pub tool_inventory: std::sync::Mutex<ToolInventory>,
    pub tool_lock: std::sync::Mutex<ToolLock>,
    pub(crate) accepted_official_index_version: std::sync::Mutex<u64>,
    execution_plans: ExecutionPlanCache,
}

#[derive(Default)]
struct ExecutionPlanCache {
    plans: std::sync::Mutex<HashMap<String, (Instant, provider::ExecutionPlan)>>,
}

impl ExtensionState {
    pub fn new() -> Result<Self, String> {
        let paths = ExtensionPaths::discover()?;
        Self::from_paths(paths)
    }

    pub(crate) fn from_paths(paths: ExtensionPaths) -> Result<Self, String> {
        Self::from_paths_with_official_index(paths, official_index::OfficialIndexConfig::default())
    }

    pub(crate) fn from_paths_with_official_index(
        paths: ExtensionPaths,
        official_index: official_index::OfficialIndexConfig,
    ) -> Result<Self, String> {
        paths.ensure()?;
        let tool_lock = ToolLock::load(&paths.tool_lock_file)?;
        let accepted_official_index_version =
            official_index::load_accepted_version(&paths.official_index_state_file)?;
        let recommendations = recommendations::load_recommended()?;
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
        let state = Self {
            provider: provider::ProviderManager::new(paths.cache.join("providers")),
            paths,
            client,
            official_index,
            recommendations,
            mutation_lock: tokio::sync::Mutex::new(()),
            provider_commands: catalog::ProviderCommandCache::default(),
            tool_inventory: std::sync::Mutex::new(ToolInventory::new()),
            tool_lock: std::sync::Mutex::new(tool_lock),
            accepted_official_index_version: std::sync::Mutex::new(accepted_official_index_version),
            execution_plans: ExecutionPlanCache::default(),
        };
        transaction::recover(&state)?;
        config::recover_configurations(&state.paths.data)?;
        Ok(state)
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

    pub fn check_executable_binding(
        &self,
        binding: &str,
        executable_path: &str,
    ) -> Result<LockState, String> {
        let candidate = inventory::executable_candidate(
            std::path::Path::new(executable_path),
            std::path::Path::new(executable_path)
                .file_name()
                .and_then(|name| name.to_str())
                .unwrap_or(executable_path),
        );
        let mut lock = self
            .tool_lock
            .lock()
            .map_err(|_| "Tool lock is unavailable".to_string())?;
        let snapshot = lock.clone();
        let inserted = !lock.tools.contains_key(binding);
        if inserted {
            lock.bind_locator(
                binding,
                ToolLocator::Executable {
                    path: executable_path.to_string(),
                },
                candidate.fingerprint.clone(),
            );
        }
        let previous = lock.tools[binding].state;
        let current = lock.check(binding, Some(&candidate))?.state;
        if inserted || previous != current || !self.paths.tool_lock_file.exists() {
            if let Err(error) = lock.save(&self.paths.tool_lock_file) {
                *lock = snapshot;
                return Err(error);
            }
        }
        Ok(current)
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

    #[cfg(unix)]
    #[test]
    fn executable_bindings_detect_replacement_and_removal() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().unwrap();
        let state =
            ExtensionState::from_paths(ExtensionPaths::from_root(directory.path().join("config")))
                .unwrap();
        let executable = directory.path().join("tool");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").unwrap();
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755)).unwrap();

        assert_eq!(
            state
                .check_executable_binding("example.tool", &executable.to_string_lossy())
                .unwrap(),
            LockState::Connected
        );

        std::fs::write(&executable, "#!/bin/sh\nprintf replacement\n").unwrap();
        assert_eq!(
            state
                .check_executable_binding("example.tool", &executable.to_string_lossy())
                .unwrap(),
            LockState::ReverifyRequired
        );

        std::fs::remove_file(&executable).unwrap();
        assert_eq!(
            state
                .check_executable_binding("example.tool", &executable.to_string_lossy())
                .unwrap(),
            LockState::ReconnectRequired
        );
    }
}
