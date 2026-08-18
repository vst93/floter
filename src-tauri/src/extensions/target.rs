//! Isolated execution targets. Container images remain candidates until a
//! resolver explicitly chooses them and are never treated as host executables.

use super::inventory::ToolLocator;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Target {
    pub runtime: TargetRuntime,
    pub locator: ToolLocator,
    pub cwd: Option<String>,
    #[serde(default)]
    pub mounts: Vec<Mount>,
    pub network: NetworkPolicy,
    pub user: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum TargetRuntime {
    Host,
    Docker,
    Podman,
    Flatpak,
    Snap,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Mount {
    pub host: String,
    pub container: String,
    pub read_only: bool,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum NetworkPolicy {
    Inherit,
    None,
    Restricted,
}

impl Target {
    pub fn docker(reference: impl Into<String>, digest: Option<String>) -> Self {
        Self {
            runtime: TargetRuntime::Docker,
            locator: ToolLocator::DockerImage {
                reference: reference.into(),
                digest,
            },
            cwd: None,
            mounts: Vec::new(),
            network: NetworkPolicy::Restricted,
            user: None,
        }
    }

    pub fn command(&self, program: &str, args: &[String]) -> Result<std::process::Command, String> {
        match (&self.runtime, &self.locator) {
            (TargetRuntime::Host, ToolLocator::Executable { path }) => Ok({
                let mut command = std::process::Command::new(path);
                command.arg(program).args(args);
                command
            }),
            (TargetRuntime::Docker, ToolLocator::DockerImage { reference, digest }) => {
                let image = digest
                    .as_deref()
                    .map(|digest| format!("{reference}@{digest}"))
                    .unwrap_or_else(|| reference.clone());
                let mut command = std::process::Command::new("docker");
                command.arg("run").arg("--rm");
                if self.network == NetworkPolicy::None {
                    command.args(["--network", "none"]);
                }
                for mount in &self.mounts {
                    command.arg("-v").arg(format!(
                        "{}:{}{}",
                        mount.host,
                        mount.container,
                        if mount.read_only { ":ro" } else { "" }
                    ));
                }
                command.arg(image).arg(program).args(args);
                Ok(command)
            }
            (TargetRuntime::Podman, ToolLocator::DockerImage { reference, digest }) => {
                let image = digest
                    .as_deref()
                    .map(|digest| format!("{reference}@{digest}"))
                    .unwrap_or_else(|| reference.clone());
                let mut command = std::process::Command::new("podman");
                command
                    .arg("run")
                    .arg("--rm")
                    .arg(image)
                    .arg(program)
                    .args(args);
                Ok(command)
            }
            (TargetRuntime::Flatpak, ToolLocator::Flatpak { app_id }) => Ok({
                let mut command = std::process::Command::new("flatpak");
                command.arg("run").arg(app_id).arg(program).args(args);
                command
            }),
            (TargetRuntime::Snap, ToolLocator::Snap { name }) => Ok({
                let mut command = std::process::Command::new(name);
                command.args(args);
                command
            }),
            _ => Err("Target locator does not match its runtime".into()),
        }
    }
}
