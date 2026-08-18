use crate::extensions::platform::{PlatformArch, PlatformOs, PlatformTarget};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AssetKind {
    Deb,
    Rpm,
    Tar,
    Zip,
    AppImage,
    Source,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct AssetCandidate {
    pub name: String,
    pub url: String,
    pub kind: AssetKind,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
}

impl AssetCandidate {
    pub fn exact_archive(
        name: impl Into<String>,
        url: impl Into<String>,
        target: &PlatformTarget,
    ) -> Self {
        Self {
            name: name.into(),
            url: url.into(),
            kind: AssetKind::Tar,
            target: Some(target.identifier()),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct AssetDecision {
    pub name: String,
    pub url: String,
    pub score: Option<u16>,
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct AssetSelection {
    pub selected: AssetDecision,
    #[serde(default)]
    pub rejected: Vec<AssetDecision>,
}

pub struct AssetMatcher<'a> {
    target: &'a PlatformTarget,
}

impl<'a> AssetMatcher<'a> {
    pub fn new(target: &'a PlatformTarget) -> Self {
        Self { target }
    }

    pub fn select(&self, candidates: &[AssetCandidate]) -> Result<AssetSelection, String> {
        let mut compatible = Vec::new();
        let mut rejected = Vec::new();
        for candidate in candidates {
            match self.score(candidate) {
                Ok((score, reason)) => compatible.push(AssetDecision {
                    name: candidate.name.clone(),
                    url: candidate.url.clone(),
                    score: Some(score),
                    reason,
                }),
                Err(reason) => rejected.push(AssetDecision {
                    name: candidate.name.clone(),
                    url: candidate.url.clone(),
                    score: None,
                    reason,
                }),
            }
        }
        compatible.sort_by(|left, right| {
            right
                .score
                .cmp(&left.score)
                .then_with(|| left.name.cmp(&right.name))
                .then_with(|| left.url.cmp(&right.url))
        });
        let selected = compatible
            .first()
            .cloned()
            .ok_or_else(|| format!("No release asset matches {}", self.target.identifier()))?;
        rejected.extend(compatible.into_iter().skip(1).map(|mut decision| {
            decision.reason = format!(
                "Rejected in favor of {} (score {}): {}",
                selected.name,
                selected.score.unwrap_or_default(),
                decision.reason
            );
            decision
        }));
        rejected.sort_by(|left, right| left.name.cmp(&right.name).then(left.url.cmp(&right.url)));
        Ok(AssetSelection { selected, rejected })
    }

    fn score(&self, candidate: &AssetCandidate) -> Result<(u16, String), String> {
        if let Some(declared) = candidate.target.as_deref() {
            if declared != self.target.identifier() {
                return Err(format!(
                    "Declared target {declared} does not match {}",
                    self.target.identifier()
                ));
            }
        }
        match candidate.kind {
            AssetKind::Deb | AssetKind::Rpm => {
                if self.target.os != PlatformOs::Linux {
                    return Err("deb/rpm assets are only compatible with Linux".to_string());
                }
                if candidate.target.is_none() && !matches_arch(&candidate.name, self.target.arch) {
                    return Err(format!(
                        "Package architecture does not match {}",
                        self.target.arch.canonical_name()
                    ));
                }
                Ok((
                    100,
                    "Native deb/rpm package for the exact target".to_string(),
                ))
            }
            AssetKind::Tar | AssetKind::Zip => {
                if candidate.target.is_none() && !matches_exact_target(&candidate.name, self.target)
                {
                    return Err(format!(
                        "Archive name does not identify exact target {}",
                        self.target.identifier()
                    ));
                }
                Ok((90, "Exact-platform tar/zip archive".to_string()))
            }
            AssetKind::AppImage => {
                if self.target.os != PlatformOs::Linux {
                    return Err("AppImage assets are only compatible with Linux".to_string());
                }
                if candidate.target.is_none() && !matches_arch(&candidate.name, self.target.arch) {
                    return Err(format!(
                        "AppImage architecture does not match {}",
                        self.target.arch.canonical_name()
                    ));
                }
                Ok((75, "Compatible AppImage asset".to_string()))
            }
            AssetKind::Source => Ok((60, "Portable source archive fallback".to_string())),
        }
    }
}

fn matches_exact_target(name: &str, target: &PlatformTarget) -> bool {
    matches_os(name, target.os) && matches_arch(name, target.arch)
}

fn matches_os(name: &str, os: PlatformOs) -> bool {
    let name = name.to_ascii_lowercase();
    let aliases: &[&str] = match os {
        PlatformOs::Darwin => &["darwin", "macos", "osx"],
        PlatformOs::Linux => &["linux"],
        PlatformOs::Windows => &["windows", "win32", "win64"],
    };
    aliases.iter().any(|alias| has_token(&name, alias))
}

fn matches_arch(name: &str, arch: PlatformArch) -> bool {
    let name = name.to_ascii_lowercase();
    let aliases: &[&str] = match arch {
        PlatformArch::Arm64 => &["aarch64", "arm64"],
        PlatformArch::X64 => &["x86_64", "amd64", "x64"],
        PlatformArch::Armv7 => &["armv7", "armv7l", "armhf"],
    };
    aliases.iter().any(|alias| has_token(&name, alias))
}

fn has_token(value: &str, token: &str) -> bool {
    value.match_indices(token).any(|(start, _)| {
        let end = start + token.len();
        let before = value[..start].chars().next_back();
        let after = value[end..].chars().next();
        before.is_none_or(|character| !character.is_ascii_alphanumeric())
            && after.is_none_or(|character| !character.is_ascii_alphanumeric())
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn linux_x64() -> PlatformTarget {
        PlatformTarget::new(PlatformOs::Linux, PlatformArch::X64)
    }

    fn candidate(name: &str, kind: AssetKind) -> AssetCandidate {
        AssetCandidate {
            name: name.into(),
            url: format!("https://example.invalid/{name}"),
            kind,
            target: None,
        }
    }

    #[test]
    fn native_packages_rank_above_archives_appimages_and_source() {
        let target = linux_x64();
        let selection = AssetMatcher::new(&target)
            .select(&[
                candidate("tool-linux-x86_64.tar.gz", AssetKind::Tar),
                candidate("tool-x86_64.AppImage", AssetKind::AppImage),
                candidate("tool-source.tar.gz", AssetKind::Source),
                candidate("tool_1.0_amd64.deb", AssetKind::Deb),
            ])
            .unwrap();

        assert_eq!(selection.selected.name, "tool_1.0_amd64.deb");
        assert_eq!(selection.selected.score, Some(100));
        assert_eq!(
            selection
                .rejected
                .iter()
                .map(|decision| (decision.name.as_str(), decision.score))
                .collect::<Vec<_>>(),
            vec![
                ("tool-linux-x86_64.tar.gz", Some(90)),
                ("tool-source.tar.gz", Some(60)),
                ("tool-x86_64.AppImage", Some(75)),
            ]
        );
    }

    #[test]
    fn incompatible_high_score_assets_are_rejected() {
        let target = PlatformTarget::new(PlatformOs::Darwin, PlatformArch::Arm64);
        let selection = AssetMatcher::new(&target)
            .select(&[
                candidate("tool_amd64.deb", AssetKind::Deb),
                candidate("tool-darwin-arm64.zip", AssetKind::Zip),
            ])
            .unwrap();

        assert_eq!(selection.selected.score, Some(90));
        assert_eq!(selection.rejected[0].score, None);
        assert!(selection.rejected[0]
            .reason
            .contains("only compatible with Linux"));
    }

    #[test]
    fn ties_are_resolved_deterministically() {
        let target = linux_x64();
        let selection = AssetMatcher::new(&target)
            .select(&[
                candidate("z-linux-x86_64.zip", AssetKind::Zip),
                candidate("a-linux-x86_64.tar.gz", AssetKind::Tar),
            ])
            .unwrap();

        assert_eq!(selection.selected.name, "a-linux-x86_64.tar.gz");
        assert!(selection.rejected[0].reason.contains("Rejected in favor"));
    }

    #[test]
    fn explicit_target_supports_registry_archives_without_platform_tokens() {
        let target = linux_x64();
        let candidate = AssetCandidate::exact_archive(
            "@example/tool-linux",
            "https://registry.example/tool-1.0.0.tgz",
            &target,
        );
        let selection = AssetMatcher::new(&target).select(&[candidate]).unwrap();

        assert_eq!(selection.selected.score, Some(90));
        assert!(selection.rejected.is_empty());
    }
}
