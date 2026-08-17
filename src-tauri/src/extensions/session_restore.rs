use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

const SESSION_SCHEMA_VERSION: u32 = 1;

/// Session restore policy.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "policy", rename_all = "camelCase")]
pub enum RestorePolicy {
    /// Reattach to existing session if alive.
    Reattach,
    /// Restart the session if it has exited.
    Restart,
    /// Never restore; always create a new session.
    None,
}

impl Default for RestorePolicy {
    fn default() -> Self {
        RestorePolicy::Reattach
    }
}

/// A minimal session description written to `session.json`
/// per the floter-extension specification.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionDescription {
    pub schema_version: u32,
    pub session_id: String,
    pub tool_id: String,
    pub tool_version: String,
    pub argv: Vec<String>,
    pub cwd: String,
    #[serde(default)]
    pub environment_refs: Vec<String>,
    #[serde(default)]
    pub terminal_profile: Option<String>,
    #[serde(default)]
    pub restore_policy: RestorePolicy,
    pub created_at: String,
}

impl SessionDescription {
    pub fn create(
        tool_id: String,
        tool_version: String,
        argv: Vec<String>,
        cwd: PathBuf,
        environment_refs: Vec<String>,
        terminal_profile: Option<String>,
        restore_policy: RestorePolicy,
    ) -> Self {
        let session_id = uuid::Uuid::new_v4().to_string();
        let created_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs()
            .to_string();
        Self {
            schema_version: SESSION_SCHEMA_VERSION,
            session_id,
            tool_id,
            tool_version,
            argv,
            cwd: cwd.to_string_lossy().into_owned(),
            environment_refs,
            terminal_profile,
            restore_policy,
            created_at,
        }
    }
}

/// Session state stored by the broker.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub enum SessionState {
    Active,
    Exited,
    Detached,
}

/// Information about a session entry in the broker list.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionEntry {
    pub session_id: String,
    pub tool_id: String,
    pub tool_version: String,
    pub cwd: String,
    pub state: SessionState,
    pub created_at: String,
}

/// Resolved session for restore, either a reattach or a restart plan.
#[derive(Debug, Clone)]
pub enum ResolvedSession {
    /// Session is alive; attach to it directly.
    Reattach(SessionDescription),
    /// Session exited; restart with the recorded parameters.
    Restart(SessionDescription),
    /// No session found or restore policy is None.
    New(SessionDescription),
}

/// Session resolver that manages `session.json` for a tool.
pub struct SessionResolver {
    sessions_dir: PathBuf,
}

impl SessionResolver {
    pub fn new(sessions_dir: PathBuf) -> Self {
        Self { sessions_dir }
    }

    /// Find an existing session for the given tool.
    pub fn find_session(&self, tool_id: &str) -> Result<Option<SessionDescription>, String> {
        let path = self.sessions_dir.join(format!("{}.session.json", tool_id));
        if !path.exists() {
            return Ok(None);
        }
        let bytes = std::fs::read(&path)
            .map_err(|error| format!("Cannot read session file: {error}"))?;
        let session: SessionDescription = serde_json::from_slice(&bytes)
            .map_err(|error| format!("Cannot parse session file: {error}"))?;
        Ok(Some(session))
    }

    /// Write a new session description.
    pub fn write_session(&self, session: &SessionDescription) -> Result<(), String> {
        std::fs::create_dir_all(&self.sessions_dir)
            .map_err(|error| format!("Cannot create sessions directory: {error}"))?;
        let path = self
            .sessions_dir
            .join(format!("{}.session.json", session.tool_id));
        let bytes = serde_json::to_vec_pretty(session)
            .map_err(|error| format!("Cannot serialize session: {error}"))?;
        let mut temp = tempfile::NamedTempFile::new_in(&self.sessions_dir)
            .map_err(|error| format!("Cannot create temp file: {error}"))?;
        temp.write_all(&bytes)
            .and_then(|_| temp.flush())
            .and_then(|_| temp.as_file().sync_all())
            .map_err(|error| format!("Cannot write session: {error}"))?;
        temp.persist(&path)
            .map_err(|error| format!("Cannot persist session: {error}"))?;
        Ok(())
    }

    /// Remove a session file.
    pub fn remove_session(&self, tool_id: &str) -> Result<(), String> {
        let path = self.sessions_dir.join(format!("{}.session.json", tool_id));
        if path.exists() {
            std::fs::remove_file(&path)
                .map_err(|error| format!("Cannot remove session file: {error}"))?;
        }
        Ok(())
    }

    /// Resolve what to do for a tool launch: reattach, restart, or create new.
    pub fn resolve(
        &self,
        tool_id: &str,
        tool_version: &str,
        argv: Vec<String>,
        cwd: PathBuf,
        environment_refs: Vec<String>,
        terminal_profile: Option<String>,
        restore_policy: RestorePolicy,
    ) -> Result<ResolvedSession, String> {
        if restore_policy == RestorePolicy::None {
            let session = SessionDescription::create(
                tool_id.to_string(),
                tool_version.to_string(),
                argv,
                cwd,
                environment_refs,
                terminal_profile,
                restore_policy,
            );
            return Ok(ResolvedSession::New(session));
        }

        if let Some(existing) = self.find_session(tool_id)? {
            // Check if the existing session is still alive (simplified).
            // In production, this would query the qscreen broker.
            match restore_policy {
                RestorePolicy::Reattach => {
                    return Ok(ResolvedSession::Reattach(existing));
                }
                RestorePolicy::Restart => {
                    let session = SessionDescription::create(
                        tool_id.to_string(),
                        tool_version.to_string(),
                        argv,
                        cwd,
                        environment_refs,
                        terminal_profile,
                        restore_policy,
                    );
                    return Ok(ResolvedSession::Restart(session));
                }
                RestorePolicy::None => unreachable!(),
            }
        }

        // No existing session; create new
        let session = SessionDescription::create(
            tool_id.to_string(),
            tool_version.to_string(),
            argv,
            cwd,
            environment_refs,
            terminal_profile,
            restore_policy,
        );
        Ok(ResolvedSession::New(session))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn session_creation_generates_id() {
        let session = SessionDescription::create(
            "owner.tool".to_string(),
            "1.4.2".to_string(),
            vec!["tool".to_string(), "ui".to_string()],
            PathBuf::from("/work/project"),
            vec!["profile:default".to_string()],
            Some("truecolor-v1".to_string()),
            RestorePolicy::Reattach,
        );
        assert_eq!(session.tool_id, "owner.tool");
        assert_eq!(session.tool_version, "1.4.2");
        assert!(!session.session_id.is_empty());
        assert_eq!(session.schema_version, SESSION_SCHEMA_VERSION);
    }

    #[test]
    fn write_and_read_session() {
        let temp = TempDir::new().unwrap();
        let resolver = SessionResolver::new(temp.path().to_path_buf());
        let session = SessionDescription::create(
            "test.tool".to_string(),
            "1.0.0".to_string(),
            vec!["test".to_string()],
            PathBuf::from("/tmp"),
            vec![],
            None,
            RestorePolicy::Reattach,
        );
        resolver.write_session(&session).unwrap();
        let loaded = resolver.find_session("test.tool").unwrap();
        assert!(loaded.is_some());
        let loaded = loaded.unwrap();
        assert_eq!(loaded.tool_id, "test.tool");
        assert_eq!(loaded.session_id, session.session_id);
    }

    #[test]
    fn find_nonexistent_session_returns_none() {
        let temp = TempDir::new().unwrap();
        let resolver = SessionResolver::new(temp.path().to_path_buf());
        let result = resolver.find_session("nonexistent").unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn resolve_creates_new_when_no_existing() {
        let temp = TempDir::new().unwrap();
        let resolver = SessionResolver::new(temp.path().to_path_buf());
        let resolved = resolver
            .resolve(
                "new.tool",
                "1.0.0",
                vec!["new".to_string()],
                PathBuf::from("/tmp"),
                vec![],
                None,
                RestorePolicy::Reattach,
            )
            .unwrap();
        match resolved {
            ResolvedSession::New(session) => {
                assert_eq!(session.tool_id, "new.tool");
            }
            _ => panic!("Expected New session"),
        }
    }

    #[test]
    fn resolve_none_policy_always_creates_new() {
        let temp = TempDir::new().unwrap();
        let resolver = SessionResolver::new(temp.path().to_path_buf());
        let resolved = resolver
            .resolve(
                "tool",
                "1.0.0",
                vec!["tool".to_string()],
                PathBuf::from("/tmp"),
                vec![],
                None,
                RestorePolicy::None,
            )
            .unwrap();
        match resolved {
            ResolvedSession::New(_) => {}
            _ => panic!("Expected New session for None policy"),
        }
    }

    #[test]
    fn remove_session() {
        let temp = TempDir::new().unwrap();
        let resolver = SessionResolver::new(temp.path().to_path_buf());
        let session = SessionDescription::create(
            "removable".to_string(),
            "1.0.0".to_string(),
            vec!["rm".to_string()],
            PathBuf::from("/tmp"),
            vec![],
            None,
            RestorePolicy::Reattach,
        );
        resolver.write_session(&session).unwrap();
        assert!(resolver.find_session("removable").unwrap().is_some());
        resolver.remove_session("removable").unwrap();
        assert!(resolver.find_session("removable").unwrap().is_none());
    }
}
