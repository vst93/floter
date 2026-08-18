//! Structured execution environments. Profiles never contain activation shell
//! strings; they describe a resolver context and only contribute environment
//! variables when the caller explicitly grants environment access.

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case", tag = "kind")]
pub enum ProfileKind {
    Host,
    Terminal { shell: Option<String> },
    Conda { environment: String },
    Venv { path: String },
    Pyenv { version: String },
    Asdf { tool: String, version: String },
    Mise { tool: String, version: String },
    Nix { profile: Option<String> },
    Docker { image: String },
    Podman { image: String },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Profile {
    pub id: String,
    pub kind: ProfileKind,
    pub cwd: Option<String>,
    #[serde(default)]
    pub environment: BTreeMap<String, String>,
    #[serde(default)]
    pub environment_allowed: bool,
}

impl Profile {
    pub fn host() -> Self {
        Self {
            id: "host".into(),
            kind: ProfileKind::Host,
            cwd: None,
            environment: BTreeMap::new(),
            environment_allowed: false,
        }
    }

    pub fn resolver_environment(&self) -> BTreeMap<String, String> {
        if self.environment_allowed {
            self.environment.clone()
        } else {
            BTreeMap::new()
        }
    }

    pub fn is_container(&self) -> bool {
        matches!(
            self.kind,
            ProfileKind::Docker { .. } | ProfileKind::Podman { .. }
        )
    }
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileStack {
    pub global_default: Option<String>,
    pub integration_default: Option<String>,
    pub workspace_default: Option<String>,
    pub command_override: Option<String>,
    pub invocation_override: Option<String>,
}

impl ProfileStack {
    pub fn selected(&self) -> Option<&str> {
        self.invocation_override
            .as_deref()
            .or(self.command_override.as_deref())
            .or(self.workspace_default.as_deref())
            .or(self.integration_default.as_deref())
            .or(self.global_default.as_deref())
    }
}
