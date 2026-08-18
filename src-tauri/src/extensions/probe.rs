//! Capability probing chain. Results from `--help` are explicitly inferred.

use serde::{Deserialize, Serialize};
use std::path::Path;
use std::time::Duration;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ProbeQuality {
    Provider,
    MachineReadable,
    Completion,
    HelpInferred,
    Static,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProbeResult {
    pub quality: ProbeQuality,
    pub command: Option<String>,
    pub output: Option<String>,
    pub capabilities: Vec<String>,
    pub inferred: bool,
}

pub trait ProviderDescriber {
    fn describe(&self) -> Result<ProbeResult, String>;
}

pub async fn probe(
    executable: &Path,
    provider: Option<&dyn ProviderDescriber>,
    static_descriptor: Option<ProbeResult>,
) -> ProbeResult {
    if let Some(provider) = provider {
        if let Ok(mut result) = provider.describe() {
            result.quality = ProbeQuality::Provider;
            result.inferred = false;
            return result;
        }
    }
    for args in [
        vec!["--floter-describe".to_string()],
        vec!["--json-help".to_string()],
        vec![
            "completion".to_string(),
            "--format".to_string(),
            "json".to_string(),
        ],
    ] {
        let Ok(output) = crate::extensions::probe_runner::run_single_probe(
            executable,
            &args,
            Duration::from_secs(5),
        )
        .await
        else {
            continue;
        };
        if output.passed {
            if let Ok(json) = serde_json::from_str::<serde_json::Value>(&output.stdout) {
                let capabilities = json
                    .get("capabilities")
                    .and_then(|v| v.as_array())
                    .map(|values| {
                        values
                            .iter()
                            .filter_map(|v| v.as_str().map(str::to_string))
                            .collect()
                    })
                    .unwrap_or_default();
                return ProbeResult {
                    quality: if args[0] == "completion" {
                        ProbeQuality::Completion
                    } else {
                        ProbeQuality::MachineReadable
                    },
                    command: Some(args.join(" ")),
                    output: Some(output.stdout),
                    capabilities,
                    inferred: false,
                };
            }
        }
    }
    if let Ok(output) = crate::extensions::probe_runner::run_single_probe(
        executable,
        &["--help".to_string()],
        Duration::from_secs(5),
    )
    .await
    {
        let text = if output.stdout.trim().is_empty() {
            output.stderr
        } else {
            output.stdout
        };
        if output.passed || !text.trim().is_empty() {
            return ProbeResult {
                quality: ProbeQuality::HelpInferred,
                command: Some("--help".into()),
                capabilities: parse_help(&text),
                output: Some(text),
                inferred: true,
            };
        }
    }
    static_descriptor.unwrap_or(ProbeResult {
        quality: ProbeQuality::Static,
        command: None,
        output: None,
        capabilities: Vec::new(),
        inferred: false,
    })
}

pub fn parse_help(text: &str) -> Vec<String> {
    let mut result = Vec::new();
    for token in text.split_whitespace() {
        let token =
            token.trim_matches(|c: char| !c.is_ascii_alphanumeric() && c != '-' && c != '_');
        if token.len() > 1
            && (token.starts_with('-')
                || token
                    .chars()
                    .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_'))
            && !result.iter().any(|item| item == token)
        {
            result.push(token.to_string());
        }
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn help_is_inferred() {
        assert!(parse_help("--version --verbose").contains(&"--version".into()));
    }
}
