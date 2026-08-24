//! Legacy NPM asset-selection records. The NPM distribution path was
//! physically removed; these types remain only so `extensions.lock.json`
//! files written by older builds still deserialize (`assetSelection`).

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
#[allow(dead_code)]
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
#[allow(dead_code)]
pub struct AssetDecision {
    pub name: String,
    pub url: String,
    pub score: Option<u16>,
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
#[allow(dead_code)]
pub struct AssetSelection {
    pub selected: AssetDecision,
    #[serde(default)]
    pub rejected: Vec<AssetDecision>,
}
