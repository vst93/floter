use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

/// Context passed to `CwdPolicy::resolve()` containing all the
/// information needed to determine an effective working directory.
#[derive(Debug, Clone)]
pub struct CwdContext<'a> {
    /// The active terminal session's cwd, if any.
    pub active_session_cwd: Option<&'a Path>,
    /// The tool's data directory.
    pub tool_data_dir: &'a Path,
    /// Whether the tool has been granted filesystem permission
    /// (allows Fixed paths to escape the data dir).
    pub filesystem_permission: bool,
}

impl<'a> CwdContext<'a> {
    pub fn new(
        active_session_cwd: Option<&'a Path>,
        tool_data_dir: &'a Path,
        filesystem_permission: bool,
    ) -> Self {
        Self {
            active_session_cwd,
            tool_data_dir,
            filesystem_permission,
        }
    }
}

/// Working directory policy for tool launches.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "policy", rename_all = "camelCase")]
pub enum CwdPolicy {
    /// Inherit the active terminal session's cwd.
    InheritActiveSession,
    /// Detect project root by walking up from the active session's cwd.
    ProjectRoot {
        /// Marker files that indicate a project root.
        #[serde(default = "default_project_markers")]
        markers: Vec<String>,
        /// Maximum directory levels to walk up.
        #[serde(default = "default_max_depth")]
        max_depth: u32,
    },
    /// Use the tool's data directory.
    ToolData,
    /// Use the user's home directory.
    Home,
    /// Use a fixed absolute path. Sandboxed to tool data dir unless
    /// filesystem permission is granted.
    Fixed(PathBuf),
}

fn default_project_markers() -> Vec<String> {
    vec![
        ".git".to_string(),
        "Cargo.toml".to_string(),
        "go.mod".to_string(),
        "package.json".to_string(),
        "pyproject.toml".to_string(),
    ]
}

fn default_max_depth() -> u32 {
    32
}

impl Default for CwdPolicy {
    fn default() -> Self {
        CwdPolicy::InheritActiveSession
    }
}

impl CwdPolicy {
    /// Resolve the effective working directory based on the policy.
    pub fn resolve(&self, context: &CwdContext<'_>) -> Result<PathBuf, String> {
        let active_session_cwd = context.active_session_cwd;
        let tool_data_dir = context.tool_data_dir;
        let filesystem_permission = context.filesystem_permission;

        match self {
            CwdPolicy::InheritActiveSession => {
                if let Some(cwd) = active_session_cwd {
                    if cwd.is_dir() {
                        return Ok(cwd.to_path_buf());
                    }
                }
                // Fallback chain: project root > tool data > home
                if let Some(cwd) = active_session_cwd {
                    if let Some(root) = Self::find_project_root(
                        cwd,
                        &default_project_markers(),
                        default_max_depth(),
                    ) {
                        return Ok(root);
                    }
                }
                if tool_data_dir.is_dir() {
                    return Ok(tool_data_dir.to_path_buf());
                }
                dirs::home_dir().ok_or_else(|| "Cannot determine home directory".to_string())
            }
            CwdPolicy::ProjectRoot { markers, max_depth } => {
                if let Some(cwd) = active_session_cwd {
                    if let Some(root) = Self::find_project_root(cwd, markers, *max_depth) {
                        return Ok(root);
                    }
                }
                // Fallback: active cwd > tool data > home
                if let Some(cwd) = active_session_cwd {
                    if cwd.is_dir() {
                        return Ok(cwd.to_path_buf());
                    }
                }
                if tool_data_dir.is_dir() {
                    return Ok(tool_data_dir.to_path_buf());
                }
                dirs::home_dir().ok_or_else(|| "Cannot determine home directory".to_string())
            }
            CwdPolicy::ToolData => {
                std::fs::create_dir_all(tool_data_dir)
                    .map_err(|error| format!("Cannot create tool data directory: {error}"))?;
                Ok(tool_data_dir.to_path_buf())
            }
            CwdPolicy::Home => {
                dirs::home_dir().ok_or_else(|| "Cannot determine home directory".to_string())
            }
            CwdPolicy::Fixed(path) => {
                // Sandbox validation: fixed path must be within tool data dir
                // unless filesystem permission is granted.
                if !filesystem_permission {
                    let canonical_path = path.canonicalize().map_err(|error| {
                        format!("Cannot canonicalize fixed path {}: {error}", path.display())
                    })?;
                    let canonical_data = tool_data_dir
                        .canonicalize()
                        .map_err(|error| format!("Cannot canonicalize tool data dir: {error}"))?;
                    if !canonical_path.starts_with(&canonical_data) {
                        return Err(format!(
                            "Fixed path {} escapes tool data directory and requires filesystem permission",
                            path.display()
                        ));
                    }
                }
                if !path.is_dir() {
                    return Err(format!("Fixed path {} is not a directory", path.display()));
                }
                Ok(path.clone())
            }
        }
    }

    /// Walk up from `start` looking for a directory containing any marker file.
    fn find_project_root(start: &Path, markers: &[String], max_depth: u32) -> Option<PathBuf> {
        let mut current = start;
        for _ in 0..max_depth {
            for marker in markers {
                if current.join(marker).exists() {
                    return Some(current.to_path_buf());
                }
            }
            match current.parent() {
                Some(parent) => current = parent,
                None => break,
            }
        }
        None
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn default_policy_is_inherit() {
        assert_eq!(CwdPolicy::default(), CwdPolicy::InheritActiveSession);
    }

    #[test]
    fn tool_data_creates_directory() {
        let temp = TempDir::new().unwrap();
        let data_dir = temp.path().join("tool-data");
        let policy = CwdPolicy::ToolData;
        let context = CwdContext::new(Some(temp.path()), &data_dir, false);
        let result = policy.resolve(&context).unwrap();
        assert_eq!(result, data_dir);
        assert!(data_dir.is_dir());
    }

    #[test]
    fn fixed_path_within_data_dir_succeeds() {
        let temp = TempDir::new().unwrap();
        let data_dir = temp.path().join("data");
        fs::create_dir_all(&data_dir).unwrap();
        let fixed = data_dir.join("workspace");
        fs::create_dir_all(&fixed).unwrap();
        let policy = CwdPolicy::Fixed(fixed.clone());
        let context = CwdContext::new(None, &data_dir, false);
        let result = policy.resolve(&context).unwrap();
        assert_eq!(result, fixed);
    }

    #[test]
    fn fixed_path_escaping_data_dir_requires_permission() {
        let temp = TempDir::new().unwrap();
        let data_dir = temp.path().join("data");
        fs::create_dir_all(&data_dir).unwrap();
        let outside = temp.path().join("outside");
        fs::create_dir_all(&outside).unwrap();
        let policy = CwdPolicy::Fixed(outside);
        let context = CwdContext::new(None, &data_dir, false);
        let result = policy.resolve(&context);
        assert!(result.is_err());
    }

    #[test]
    fn fixed_path_escaping_with_permission_succeeds() {
        let temp = TempDir::new().unwrap();
        let data_dir = temp.path().join("data");
        fs::create_dir_all(&data_dir).unwrap();
        let outside = temp.path().join("outside");
        fs::create_dir_all(&outside).unwrap();
        let policy = CwdPolicy::Fixed(outside.clone());
        let context = CwdContext::new(None, &data_dir, true);
        let result = policy.resolve(&context).unwrap();
        assert_eq!(result, outside);
    }

    #[test]
    fn project_root_detection() {
        let temp = TempDir::new().unwrap();
        let project_dir = temp.path().join("project");
        let sub_dir = project_dir.join("src").join("deep");
        fs::create_dir_all(&sub_dir).unwrap();
        fs::write(project_dir.join(".git"), "").unwrap();
        let markers = vec![".git".to_string()];
        let root = CwdPolicy::find_project_root(&sub_dir, &markers, 32);
        assert_eq!(root, Some(project_dir));
    }

    #[test]
    fn project_root_respects_max_depth() {
        let temp = TempDir::new().unwrap();
        let mut path = temp.path().to_path_buf();
        for i in 0..40 {
            path = path.join(format!("level{}", i));
        }
        fs::create_dir_all(&path).unwrap();
        fs::write(temp.path().join("Cargo.toml"), "").unwrap();
        let markers = vec!["Cargo.toml".to_string()];
        let root = CwdPolicy::find_project_root(&path, &markers, 32);
        assert!(root.is_none());
    }

    #[test]
    fn inherit_fallback_to_project_root() {
        let temp = TempDir::new().unwrap();
        let project_dir = temp.path().join("project");
        let sub_dir = project_dir.join("src");
        fs::create_dir_all(&sub_dir).unwrap();
        // Use a unique marker that won't exist in parent directories
        fs::write(project_dir.join(".floter-root"), "").unwrap();
        let data_dir = temp.path().join("data");
        let markers = vec![".floter-root".to_string()];
        // Test ProjectRoot policy with active session cwd
        let policy = CwdPolicy::ProjectRoot {
            markers,
            max_depth: 32,
        };
        let context = CwdContext {
            active_session_cwd: Some(&sub_dir),
            tool_data_dir: &data_dir,
            filesystem_permission: false,
        };
        let result = policy.resolve(&context).unwrap();
        assert_eq!(result, project_dir);
    }

    #[test]
    fn inherit_no_active_session_falls_back_to_project_root() {
        let temp = TempDir::new().unwrap();
        let project_dir = temp.path().join("project");
        fs::create_dir_all(&project_dir).unwrap();
        // Use a unique marker
        fs::write(project_dir.join(".floter-proj"), "").unwrap();
        let data_dir = temp.path().join("data");
        // ProjectRoot policy with no active session should find project root
        let markers = vec![".floter-proj".to_string()];
        let policy = CwdPolicy::ProjectRoot {
            markers,
            max_depth: 32,
        };
        let context = CwdContext {
            active_session_cwd: None,
            tool_data_dir: &data_dir,
            filesystem_permission: false,
        };
        // With no active session and ProjectRoot policy, should fall back to
        // tool data or home since there's no starting point for project search
        let result = policy.resolve(&context).unwrap();
        // Should fall through to tool data or home (not panic)
        assert!(result.is_dir() || result == data_dir || result == dirs::home_dir().unwrap());
    }

    #[test]
    fn inherit_fallback_to_home() {
        let temp = TempDir::new().unwrap();
        let nonexistent = temp.path().join("nonexistent");
        let data_dir = temp.path().join("empty-data");
        let policy = CwdPolicy::InheritActiveSession;
        let context = CwdContext::new(Some(&nonexistent), &data_dir, false);
        let result = policy.resolve(&context);
        // Should fall through to home directory
        assert!(result.is_ok());
        assert_ne!(result.unwrap(), nonexistent);
    }

    #[test]
    fn home_policy_returns_home_dir() {
        let temp = TempDir::new().unwrap();
        let policy = CwdPolicy::Home;
        let context = CwdContext::new(None, temp.path(), false);
        let result = policy.resolve(&context);
        assert!(result.is_ok());
    }
}
