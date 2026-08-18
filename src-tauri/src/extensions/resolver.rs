//! Deterministic candidate selection with explicit ambiguity.

use super::inventory::{DiscoveryQuality, ToolCandidate};
use super::profile::Profile;
use semver::Version;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResolveRequest {
    pub tool: String,
    pub profile: Option<Profile>,
    pub required_version: Option<String>,
    #[serde(default)]
    pub preferred_locator: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", tag = "status")]
pub enum ResolveResult {
    Selected {
        candidate: ToolCandidate,
        score: ScoreBreakdown,
    },
    Ambiguous {
        candidates: Vec<ScoredCandidate>,
    },
    NotFound {
        tool: String,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ScoredCandidate {
    pub candidate: ToolCandidate,
    pub score: ScoreBreakdown,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ScoreBreakdown {
    pub exact_match: u8,
    pub version: u8,
    pub profile: u8,
    pub stability: u8,
    pub history: u8,
}

impl ScoreBreakdown {
    fn key(&self) -> (u8, u8, u8, u8, u8) {
        (
            self.exact_match,
            self.version,
            self.profile,
            self.stability,
            self.history,
        )
    }
}

pub fn resolve(request: &ResolveRequest, candidates: &[ToolCandidate]) -> ResolveResult {
    let query = request.tool.trim().to_ascii_lowercase();
    let mut scored: Vec<ScoredCandidate> = candidates
        .iter()
        .filter(|candidate| {
            candidate.available
                && (candidate.name.eq_ignore_ascii_case(&query)
                    || candidate.id.to_ascii_lowercase().contains(&query)
                    || candidate.locator.normalized().contains(&query))
        })
        .cloned()
        .map(|candidate| {
            let score = score(request, &candidate);
            ScoredCandidate { candidate, score }
        })
        .collect();
    if scored.is_empty() {
        return ResolveResult::NotFound {
            tool: request.tool.clone(),
        };
    }
    scored.sort_by(|a, b| {
        b.score
            .key()
            .cmp(&a.score.key())
            .then_with(|| a.candidate.id.cmp(&b.candidate.id))
    });
    let best_key = scored[0].score.key();
    let best: Vec<ScoredCandidate> = scored
        .into_iter()
        .filter(|candidate| candidate.score.key() == best_key)
        .collect();
    if best.len() > 1 {
        ResolveResult::Ambiguous { candidates: best }
    } else {
        let selected = best.into_iter().next().expect("non-empty");
        ResolveResult::Selected {
            candidate: selected.candidate,
            score: selected.score,
        }
    }
}

fn score(request: &ResolveRequest, candidate: &ToolCandidate) -> ScoreBreakdown {
    let query = request.tool.trim();
    let exact_match =
        if candidate.name.eq_ignore_ascii_case(query) || candidate.id.eq_ignore_ascii_case(query) {
            3
        } else {
            0
        };
    let version = request
        .required_version
        .as_deref()
        .map(|required| {
            let actual = candidate
                .version
                .as_deref()
                .and_then(|v| Version::parse(v.trim_start_matches('v')).ok());
            match (actual, Version::parse(required.trim_start_matches('v'))) {
                (Some(actual), Ok(required)) if actual == required => 3,
                (Some(actual), Ok(required)) if actual >= required => 2,
                _ => 0,
            }
        })
        .unwrap_or(0);
    let profile = if request
        .profile
        .as_ref()
        .is_some_and(|profile| profile.is_container())
        && candidate.locator.normalized().starts_with("docker:")
    {
        3
    } else {
        0
    };
    let stability = match candidate.quality {
        DiscoveryQuality::OfficialAdapter => 4,
        DiscoveryQuality::NativeSupport => 3,
        DiscoveryQuality::AutoDetected => 2,
        DiscoveryQuality::UserDefined => 1,
        DiscoveryQuality::Inferred => 0,
    };
    let history = if request
        .preferred_locator
        .as_ref()
        .is_some_and(|locator| preferred_locator_matches(&candidate.locator, locator))
    {
        2
    } else {
        0
    };
    ScoreBreakdown {
        exact_match,
        version,
        profile,
        stability,
        history,
    }
}

fn preferred_locator_matches(locator: &super::inventory::ToolLocator, preferred: &str) -> bool {
    let normalized = locator.normalized();
    #[cfg(target_os = "windows")]
    {
        normalized.eq_ignore_ascii_case(preferred)
    }
    #[cfg(not(target_os = "windows"))]
    {
        normalized == preferred
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::inventory::{DiscoverySource, ToolLocator};
    fn candidate(id: &str, version: &str) -> ToolCandidate {
        ToolCandidate {
            id: id.into(),
            name: id.into(),
            locator: ToolLocator::Executable {
                path: format!("/usr/bin/{id}"),
            },
            version: Some(version.into()),
            sources: vec![DiscoverySource::Path],
            quality: DiscoveryQuality::AutoDetected,
            available: true,
            fingerprint: None,
        }
    }
    #[test]
    fn equal_candidates_are_not_silently_replaced() {
        let result = resolve(
            &ResolveRequest {
                tool: "tool".into(),
                profile: None,
                required_version: None,
                preferred_locator: None,
            },
            &[candidate("tool", "1.0.0"), candidate("tool", "1.0.0")],
        );
        assert!(matches!(result, ResolveResult::Ambiguous { .. }));
    }

    #[cfg(not(target_os = "windows"))]
    #[test]
    fn preferred_locator_keeps_case_sensitive_unix_paths() {
        let mut candidate = candidate("tool", "1.0.0");
        candidate.locator = ToolLocator::Executable {
            path: "/opt/Floter/Tool".into(),
        };
        let result = resolve(
            &ResolveRequest {
                tool: "tool".into(),
                profile: None,
                required_version: None,
                preferred_locator: Some("/opt/Floter/Tool".into()),
            },
            &[candidate],
        );

        let ResolveResult::Selected { score, .. } = result else {
            panic!("expected a selected candidate");
        };
        assert_eq!(score.history, 2);
    }
}
