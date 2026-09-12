pub mod artifacts;
pub mod asset_matcher;
pub mod capability_probe;
pub mod catalog;
pub mod config;
pub mod conformance;
pub mod cwd_policy;
pub mod data_ownership;
pub mod error_codes;
pub mod export_schema;
pub mod health;
pub mod help_args;
pub mod install;
pub mod inventory;
pub mod launch;
pub mod lifecycle;
pub mod lock;
pub mod manifest;
pub mod official_index;
pub(crate) mod operation;
pub mod platform;
pub mod probe;
pub mod probe_executor;
pub mod probe_runner;
pub(crate) mod process_cleanup;
pub mod profile;
pub mod provider;
mod proxy;
pub mod recommendations;
pub mod registry;
pub mod repository;
pub mod resolver;
pub mod session_restore;
pub mod sync;
#[cfg(test)]
mod sync_tests_phase5;
pub mod terminal_capability;
pub mod tool_lock;
pub mod tool_manifests;
pub(crate) mod transaction;
pub mod uninstall;

/// Test-only crash injection hook used at durable state commit boundaries.
///
/// In production this compiles to a no-op. A test scope arms one label; the
/// matching call consumes it immediately before that boundary. Tests can panic
/// or run a scoped action; production never holds fault state.
pub(crate) fn commit_point(label: &str) {
    #[cfg(test)]
    {
        let fault = COMMIT_POINT
            .try_with(|armed| {
                let mut armed = armed.borrow_mut();
                if armed.as_ref().is_some_and(|fault| fault.label == label) {
                    armed.take()
                } else {
                    None
                }
            })
            .unwrap_or(None);
        if let Some(fault) = fault {
            match fault.action {
                CommitPointAction::Panic => panic!("injected crash at commit point {label}"),
                #[cfg(unix)]
                CommitPointAction::Run(action) => action(),
            }
        }
    }
    #[cfg(not(test))]
    let _ = label;
}

#[cfg(test)]
struct CommitPointFault {
    label: &'static str,
    action: CommitPointAction,
}

#[cfg(test)]
enum CommitPointAction {
    Panic,
    #[cfg(unix)]
    Run(Box<dyn FnOnce() + Send>),
}

#[cfg(test)]
tokio::task_local! {
    static COMMIT_POINT: std::cell::RefCell<Option<CommitPointFault>>;
}

#[cfg(test)]
pub(crate) fn with_commit_point<T>(label: &'static str, operation: impl FnOnce() -> T) -> T {
    COMMIT_POINT.sync_scope(
        std::cell::RefCell::new(Some(CommitPointFault {
            label,
            action: CommitPointAction::Panic,
        })),
        operation,
    )
}

#[cfg(test)]
pub(crate) async fn with_async_commit_point<T>(
    label: &'static str,
    operation: impl std::future::Future<Output = T>,
) -> T {
    COMMIT_POINT
        .scope(
            std::cell::RefCell::new(Some(CommitPointFault {
                label,
                action: CommitPointAction::Panic,
            })),
            operation,
        )
        .await
}

#[cfg(all(test, unix))]
pub(crate) fn with_commit_point_action<T>(
    label: &'static str,
    action: impl FnOnce() + Send + 'static,
    operation: impl FnOnce() -> T,
) -> T {
    COMMIT_POINT.sync_scope(
        std::cell::RefCell::new(Some(CommitPointFault {
            label,
            action: CommitPointAction::Run(Box::new(action)),
        })),
        operation,
    )
}

#[cfg(all(test, unix))]
pub(crate) async fn with_async_commit_point_action<T>(
    label: &'static str,
    action: impl FnOnce() + Send + 'static,
    operation: impl std::future::Future<Output = T>,
) -> T {
    COMMIT_POINT
        .scope(
            std::cell::RefCell::new(Some(CommitPointFault {
                label,
                action: CommitPointAction::Run(Box::new(action)),
            })),
            operation,
        )
        .await
}

#[cfg(all(test, unix))]
pub(crate) mod fault_test_support {
    use std::os::unix::fs::PermissionsExt;
    use std::os::unix::process::ExitStatusExt;
    use std::path::{Path, PathBuf};
    use std::sync::Arc;

    pub(crate) fn skip_readonly_as_root() -> bool {
        // geteuid has no preconditions and does not change process credentials.
        if unsafe { libc::geteuid() } == 0 {
            eprintln!("SKIPPED readonly-fs test: euid 0 bypasses chmod permissions");
            true
        } else {
            false
        }
    }

    pub(crate) struct ReadonlyDirectory {
        path: PathBuf,
        original: std::fs::Permissions,
    }

    impl ReadonlyDirectory {
        pub(crate) fn new(path: &Path) -> Arc<Self> {
            Arc::new(Self {
                path: path.to_path_buf(),
                original: std::fs::metadata(path).unwrap().permissions(),
            })
        }

        pub(crate) fn arm(self: &Arc<Self>) -> impl FnOnce() + Send + 'static {
            let guard = Arc::clone(self);
            move || {
                std::fs::set_permissions(&guard.path, std::fs::Permissions::from_mode(0o555))
                    .unwrap();
            }
        }
    }

    impl Drop for ReadonlyDirectory {
        fn drop(&mut self) {
            if let Err(error) = std::fs::set_permissions(&self.path, self.original.clone()) {
                eprintln!(
                    "Cannot restore test directory {}: {error}",
                    self.path.display()
                );
            }
        }
    }

    #[derive(Clone, Copy, Debug)]
    pub(crate) enum Corruption {
        Truncated,
        Flipped,
        Garbage,
    }

    impl Corruption {
        pub(crate) const ALL: [Self; 3] = [Self::Truncated, Self::Flipped, Self::Garbage];

        pub(crate) fn apply(self, path: &Path) -> Vec<u8> {
            let mut bytes = std::fs::read(path).unwrap();
            let middle = bytes.len() / 2;
            match self {
                Self::Truncated => bytes.truncate(middle),
                Self::Flipped => bytes[middle] ^= 0xff,
                Self::Garbage => bytes = b"not a JSON document\0\xff".to_vec(),
            }
            assert!(serde_json::from_slice::<serde_json::Value>(&bytes).is_err());
            std::fs::write(path, &bytes).unwrap();
            bytes
        }
    }

    pub(crate) fn crash_child_root(name: &str) -> Option<PathBuf> {
        (std::env::var("FLOTER_CRASH_TEST").as_deref() == Ok(name)).then(|| {
            PathBuf::from(std::env::var_os("FLOTER_CRASH_ROOT").expect("child fixture root"))
        })
    }

    pub(crate) fn kill_at_commit_point(label: &'static str) -> impl FnOnce() + Send {
        move || {
            eprintln!("crash boundary reached: {label}");
            // Kill only this re-executed child, without unwinding or a core dump.
            unsafe { libc::raise(libc::SIGKILL) };
            unreachable!("SIGKILL must terminate the crash child");
        }
    }

    pub(crate) fn crash_child_boundary() -> String {
        std::env::var("FLOTER_CRASH_BOUNDARY").expect("child crash boundary")
    }

    pub(crate) fn run_crash_child(name: &str, root: &Path, label: &str) {
        // module_path! includes the crate name; libtest's exact names omit it.
        let test_name = name
            .strip_prefix(concat!(env!("CARGO_CRATE_NAME"), "::"))
            .unwrap_or(name);
        let output = std::process::Command::new(std::env::current_exe().unwrap())
            .args(["--ignored", "--exact", test_name, "--nocapture"])
            .env("FLOTER_CRASH_TEST", name)
            .env("FLOTER_CRASH_ROOT", root)
            .env("FLOTER_CRASH_BOUNDARY", label)
            .output()
            .unwrap();
        let stderr = String::from_utf8_lossy(&output.stderr);
        assert_eq!(
            output.status.signal(),
            Some(libc::SIGKILL),
            "child status: {}; stdout: {}; stderr: {stderr}",
            output.status,
            String::from_utf8_lossy(&output.stdout)
        );
        assert!(
            stderr.contains(&format!("crash boundary reached: {label}")),
            "{stderr}"
        );
    }
}

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
    /// Read-only input for startup migration; never an active state source.
    pub legacy_lock_file: PathBuf,
    pub repository_file: PathBuf,
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
            legacy_lock_file: root.join("extensions.lock.json"),
            repository_file: root.join("extension-repository.json"),
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
    /// AppHandle used to emit operation progress events; absent in unit tests.
    pub(crate) app_handle: std::sync::OnceLock<tauri::AppHandle>,
    /// Cancel token for the currently running long operation, if any.
    pub(crate) active_cancel: std::sync::Mutex<Option<operation::CancelToken>>,
    /// In-process progress listener used by unit tests (no AppHandle there).
    progress_listener:
        std::sync::Mutex<Option<Box<dyn Fn(operation::OperationProgress) + Send + 'static>>>,
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
            app_handle: std::sync::OnceLock::new(),
            active_cancel: std::sync::Mutex::new(None),
            progress_listener: std::sync::Mutex::new(None),
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

    /// Emit an operation progress event if an AppHandle is registered.
    pub(crate) fn emit_progress(&self, progress: operation::OperationProgress) {
        use tauri::Emitter;
        if let Some(app) = self.app_handle.get() {
            let _ = app.emit("extension-op-progress", &progress);
        }
        if let Some(listener) = self
            .progress_listener
            .lock()
            .expect("Progress lock poisoned")
            .as_ref()
        {
            listener(progress);
        }
    }

    /// Check if the current operation was cancelled; returns Err if so.
    pub(crate) fn check_cancelled(&self) -> Result<(), String> {
        let guard = self
            .active_cancel
            .lock()
            .map_err(|_| "Cancel lock poisoned")?;
        if let Some(token) = guard.as_ref() {
            if token.is_cancelled() {
                return Err(format!(
                    "[{}] Operation cancelled",
                    crate::extensions::error_codes::ProviderErrorCode::Cancelled.as_str()
                ));
            }
        }
        Ok(())
    }

    /// Mint an operation id and register a fresh cancel token for the
    /// duration of a long operation. The id is only informational (progress
    /// events carry the extension id); uniqueness comes from the counter.
    pub(crate) fn start_operation(&self) -> String {
        use std::sync::atomic::{AtomicU64, Ordering};
        static COUNTER: AtomicU64 = AtomicU64::new(1);
        let id = format!("op-{}", COUNTER.fetch_add(1, Ordering::Relaxed));
        *self.active_cancel.lock().expect("Cancel lock poisoned") =
            Some(operation::CancelToken::new());
        id
    }

    /// Clear the active cancel token when an operation finishes (success,
    /// error, or cancellation).
    pub(crate) fn end_operation(&self, _operation_id: &str) {
        *self.active_cancel.lock().expect("Cancel lock poisoned") = None;
    }

    /// Flip the active cancel token (if any) so the running operation stops
    /// at its next cancellation checkpoint.
    pub(crate) fn cancel_operation(&self, _operation_id: &str) {
        if let Some(token) = self
            .active_cancel
            .lock()
            .expect("Cancel lock poisoned")
            .as_ref()
        {
            token.cancel();
        }
    }

    /// Register an in-process progress listener (unit tests use this because
    /// they have no AppHandle). Replaces any previous listener.
    #[cfg(test)]
    pub(crate) fn set_progress_listener(
        &self,
        listener: Box<dyn Fn(operation::OperationProgress) + Send + 'static>,
    ) {
        *self
            .progress_listener
            .lock()
            .expect("Progress lock poisoned") = Some(listener);
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

    #[cfg(unix)]
    #[test]
    fn commit_point_actions_are_nested_one_shot_and_thread_scoped() {
        use std::sync::atomic::{AtomicUsize, Ordering};
        use std::sync::Arc;

        let calls = Arc::new(AtomicUsize::new(0));
        let outer = Arc::clone(&calls);
        with_commit_point_action(
            "action-scope",
            move || {
                outer.fetch_add(1, Ordering::SeqCst);
            },
            || {
                std::thread::spawn(|| commit_point("action-scope"))
                    .join()
                    .unwrap();
                commit_point("another-label");
                assert_eq!(calls.load(Ordering::SeqCst), 0);
                let inner = Arc::clone(&calls);
                with_commit_point_action(
                    "action-scope",
                    move || {
                        inner.fetch_add(10, Ordering::SeqCst);
                    },
                    || {
                        commit_point("action-scope");
                        commit_point("action-scope");
                    },
                );
                commit_point("action-scope");
                commit_point("action-scope");
            },
        );
        assert_eq!(calls.load(Ordering::SeqCst), 11);
        with_commit_point_action("action-scope", || panic!("unconsumed action leaked"), || {});
        let unwind = std::panic::catch_unwind(|| {
            with_commit_point_action(
                "action-scope",
                || panic!("unwound action leaked"),
                || {
                    panic!("unrelated panic");
                },
            );
        });
        assert!(unwind.is_err());
        commit_point("action-scope");
    }

    #[cfg(unix)]
    #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
    async fn commit_point_actions_follow_tasks_and_drop_on_cancellation() {
        use std::sync::atomic::{AtomicUsize, Ordering};
        use std::sync::Arc;

        let calls = Arc::new(AtomicUsize::new(0));
        let action_calls = Arc::clone(&calls);
        let (ready, armed) = tokio::sync::oneshot::channel();
        let (release, resume) = tokio::sync::oneshot::channel();
        let task = tokio::spawn(with_async_commit_point_action(
            "action-scope",
            move || {
                action_calls.fetch_add(1, Ordering::SeqCst);
            },
            async move {
                ready.send(()).unwrap();
                resume.await.unwrap();
                commit_point("action-scope");
                commit_point("action-scope");
            },
        ));
        armed.await.unwrap();
        commit_point("action-scope");
        assert_eq!(calls.load(Ordering::SeqCst), 0);
        release.send(()).unwrap();
        task.await.unwrap();
        assert_eq!(calls.load(Ordering::SeqCst), 1);

        let cancelled_calls = Arc::clone(&calls);
        tokio::select! {
            biased;
            _ = with_async_commit_point_action("action-scope", move || {
                cancelled_calls.fetch_add(1, Ordering::SeqCst);
            }, std::future::pending::<()>()) => unreachable!(),
            _ = std::future::ready(()) => {},
        }
        commit_point("action-scope");
        assert_eq!(calls.load(Ordering::SeqCst), 1);
        assert_eq!(Arc::strong_count(&calls), 1);
    }

    #[cfg(unix)]
    #[test]
    fn readonly_directory_guard_restores_permissions_after_panic() {
        use fault_test_support::{skip_readonly_as_root, ReadonlyDirectory};
        use std::os::unix::fs::PermissionsExt;

        if skip_readonly_as_root() {
            return;
        }
        let directory = tempfile::tempdir().unwrap();
        let original = std::fs::metadata(directory.path())
            .unwrap()
            .permissions()
            .mode();
        let result = std::panic::catch_unwind(|| {
            let readonly = ReadonlyDirectory::new(directory.path());
            with_commit_point_action("readonly-guard", readonly.arm(), || {
                commit_point("readonly-guard");
                assert_eq!(
                    std::fs::metadata(directory.path())
                        .unwrap()
                        .permissions()
                        .mode()
                        & 0o777,
                    0o555
                );
                assert_eq!(
                    std::fs::write(directory.path().join("blocked"), b"blocked")
                        .unwrap_err()
                        .kind(),
                    std::io::ErrorKind::PermissionDenied
                );
                panic!("exercise permission restoration during unwind");
            });
        });
        let panic = result.unwrap_err();
        assert_eq!(
            panic.downcast_ref::<&str>().copied(),
            Some("exercise permission restoration during unwind")
        );
        assert_eq!(
            std::fs::metadata(directory.path())
                .unwrap()
                .permissions()
                .mode(),
            original
        );
        std::fs::write(directory.path().join("restored"), b"writable").unwrap();
    }

    #[test]
    fn commit_point_scopes_isolate_threads_and_match_once() {
        with_commit_point("scope-test", || {
            std::thread::spawn(|| commit_point("scope-test"))
                .join()
                .unwrap();
            commit_point("another-label");
            assert!(std::panic::catch_unwind(|| commit_point("scope-test")).is_err());
            commit_point("scope-test");
        });
        with_commit_point("scope-test", || {});
        commit_point("scope-test");
        assert!(std::panic::catch_unwind(|| {
            with_commit_point("scope-test", || panic!("unrelated panic"));
        })
        .is_err());
        commit_point("scope-test");
    }

    async fn assert_async_commit_point_isolation() {
        let (armed_sender, armed_receiver) = tokio::sync::oneshot::channel();
        let (release_sender, release_receiver) = tokio::sync::oneshot::channel();
        let task = tokio::spawn(with_async_commit_point("scope-test", async move {
            armed_sender.send(()).unwrap();
            release_receiver.await.unwrap();
            commit_point("scope-test");
        }));
        armed_receiver.await.unwrap();
        commit_point("scope-test");
        release_sender.send(()).unwrap();
        let error = task.await.unwrap_err();
        assert!(error.is_panic());
        assert_eq!(
            error
                .into_panic()
                .downcast_ref::<String>()
                .map(String::as_str),
            Some("injected crash at commit point scope-test")
        );
        commit_point("scope-test");
    }

    #[tokio::test]
    async fn commit_point_scopes_isolate_tasks_on_one_thread() {
        assert_async_commit_point_isolation().await;
    }

    #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
    async fn commit_point_scopes_isolate_tasks_across_worker_threads() {
        assert_async_commit_point_isolation().await;
    }

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
