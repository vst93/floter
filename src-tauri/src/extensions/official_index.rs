use crate::extensions::manifest::SignatureConfig;
use crate::extensions::ExtensionState;
use base64::Engine;
use chrono::{DateTime, Utc};
use ed25519_dalek::{Signature, VerifyingKey};
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::Path;

const INDEX_SCHEMA_VERSION: u32 = 1;
const MAX_INDEX_BYTES: usize = 1024 * 1024;

// The official index is hosted in this repository under
// extensions/official-index/index.json (a signed envelope) and fetched from
// the GitHub `main` branch. Edit payload.json and re-sign with
// scripts/sign-official-index.mjs before pushing; the root key below is the
// trust anchor that rejects any repository tampering.
pub const DEVELOPMENT_ROOT_PUBLIC_KEY: &str =
    "ed25519:BOvvP8Yib+JZDozuzknO5FTXRpsKAokIOlF/ONXqABQ=";
pub const DEFAULT_INDEX_URL: &str =
    "https://raw.githubusercontent.com/vst93/floter/main/extensions/official-index/index.json";

// This development index is the payload committed at
// extensions/official-index/payload.json. It is kept in the binary so the
// allow-list is reviewable, but it is never used as a network-failure fallback.
pub const DEVELOPMENT_INDEX_PAYLOAD: &str = r#"{
  "schemaVersion": 1,
  "indexVersion": 1,
  "expiresAt": "2030-01-01T00:00:00Z",
  "entries": [
    {
      "extensionId": "io.github.vst93.v",
      "npmPackage": "v-tools",
      "publisher": "vst93",
      "signingKeys": [
        "ed25519:PUAXyCQgW7K2fOhE6RtAHAEgXKM2ZK6E3YpOQ2OMX2c="
      ]
    }
  ]
}"#;
pub const DEVELOPMENT_INDEX_SIGNATURE: &str =
    "eDpD/tJ04QmVo/JkiLeuchKKiI43kwQwzsoIGK7kKk9AfGXmysSfO2OBVONA+nh+7kFGhvKJ/ntEbCX9UvNMBw==";

#[derive(Debug, Clone)]
pub struct OfficialIndexConfig {
    pub url: String,
    pub root_public_keys: Vec<String>,
}

impl Default for OfficialIndexConfig {
    fn default() -> Self {
        debug_assert!(serde_json::from_str::<OfficialIndex>(DEVELOPMENT_INDEX_PAYLOAD).is_ok());
        debug_assert!(verify_ed25519(
            DEVELOPMENT_ROOT_PUBLIC_KEY,
            DEVELOPMENT_INDEX_SIGNATURE,
            DEVELOPMENT_INDEX_PAYLOAD.as_bytes()
        )
        .unwrap_or(false));
        Self {
            url: DEFAULT_INDEX_URL.to_string(),
            root_public_keys: vec![DEVELOPMENT_ROOT_PUBLIC_KEY.to_string()],
        }
    }
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SignedIndexEnvelope {
    pub payload: String,
    pub signatures: Vec<IndexSignature>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct IndexSignature {
    pub key_id: String,
    pub algorithm: String,
    pub signature: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct OfficialIndex {
    pub schema_version: u32,
    pub index_version: u64,
    pub expires_at: String,
    pub entries: Vec<OfficialIndexEntry>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct OfficialIndexEntry {
    pub extension_id: String,
    pub npm_package: String,
    pub publisher: String,
    pub signing_keys: Vec<String>,
}

impl OfficialIndex {
    pub fn authorizes(
        &self,
        extension_id: &str,
        package: &str,
        publisher: &str,
        signature: Option<&SignatureConfig>,
    ) -> bool {
        let Some(signature) = signature else {
            return false;
        };
        self.entries.iter().any(|entry| {
            entry.extension_id == extension_id
                && entry.npm_package == package
                && entry.publisher == publisher
                && entry
                    .signing_keys
                    .iter()
                    .any(|key| key == &signature.public_key)
        })
    }
}

pub async fn fetch(state: &ExtensionState) -> Result<OfficialIndex, String> {
    let url = reqwest::Url::parse(&state.official_index.url)
        .map_err(|error| format!("Invalid official extension index URL: {error}"))?;
    if url.scheme() != "https" {
        return Err("Official extension index URL must use HTTPS".to_string());
    }
    let mut response = state
        .client
        .get(url)
        .timeout(std::time::Duration::from_secs(5))
        .send()
        .await
        .map_err(|error| format!("Cannot download official extension index: {error}"))?
        .error_for_status()
        .map_err(|error| format!("Cannot download official extension index: {error}"))?;
    if response.url().scheme() != "https" {
        return Err("Official extension index redirected to a non-HTTPS URL".to_string());
    }
    if response
        .content_length()
        .is_some_and(|length| length > MAX_INDEX_BYTES as u64)
    {
        return Err("Official extension index exceeds the size limit".to_string());
    }
    let mut bytes = Vec::new();
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|error| format!("Cannot read official extension index: {error}"))?
    {
        if bytes
            .len()
            .checked_add(chunk.len())
            .is_none_or(|length| length > MAX_INDEX_BYTES)
        {
            return Err("Official extension index exceeds the size limit".to_string());
        }
        bytes.extend_from_slice(&chunk);
    }
    let index = verify(&bytes, &state.official_index.root_public_keys, Utc::now())?;
    accept_version(
        &state.paths.official_index_state_file,
        &state.accepted_official_index_version,
        index.index_version,
    )?;
    Ok(index)
}

const INDEX_STATE_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct OfficialIndexState {
    schema_version: u32,
    highest_accepted_version: u64,
}

pub(crate) fn load_accepted_version(path: &Path) -> Result<u64, String> {
    if !path.exists() {
        return Ok(0);
    }
    let bytes = std::fs::read(path)
        .map_err(|error| format!("Cannot read official index state: {error}"))?;
    let state: OfficialIndexState = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid official index state: {error}"))?;
    if state.schema_version != INDEX_STATE_SCHEMA_VERSION {
        return Err(format!(
            "Unsupported official index state schema version {}",
            state.schema_version
        ));
    }
    Ok(state.highest_accepted_version)
}

fn accept_version(
    path: &Path,
    accepted: &std::sync::Mutex<u64>,
    version: u64,
) -> Result<(), String> {
    let mut highest = accepted
        .lock()
        .map_err(|_| "Official index version state is unavailable".to_string())?;
    if version < *highest {
        return Err(format!(
            "Official extension index rollback rejected: version {version} is older than {}",
            *highest
        ));
    }
    if version == *highest {
        return Ok(());
    }
    let parent = path.parent().ok_or("Invalid official index state path")?;
    std::fs::create_dir_all(parent)
        .map_err(|error| format!("Cannot create official index state directory: {error}"))?;
    let bytes = serde_json::to_vec_pretty(&OfficialIndexState {
        schema_version: INDEX_STATE_SCHEMA_VERSION,
        highest_accepted_version: version,
    })
    .map_err(|error| format!("Cannot serialize official index state: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create official index state: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write official index state: {error}"))?;
    temporary
        .persist(path)
        .map_err(|error| format!("Cannot persist official index state: {error}"))?;
    crate::extensions::lock::sync_directory(parent)
        .map_err(|error| format!("Cannot sync official index state directory: {error}"))?;
    *highest = version;
    Ok(())
}

pub fn verify(
    envelope_bytes: &[u8],
    root_public_keys: &[String],
    now: DateTime<Utc>,
) -> Result<OfficialIndex, String> {
    let envelope: SignedIndexEnvelope = serde_json::from_slice(envelope_bytes)
        .map_err(|error| format!("Invalid official extension index envelope: {error}"))?;
    let payload = base64::engine::general_purpose::STANDARD
        .decode(&envelope.payload)
        .map_err(|_| "Official extension index payload is not valid Base64".to_string())?;
    let trusted = envelope.signatures.iter().any(|candidate| {
        candidate.algorithm == "ed25519"
            && root_public_keys.iter().any(|root| {
                verify_ed25519(root, candidate.signature.as_str(), &payload).unwrap_or(false)
            })
    });
    if !trusted {
        return Err("Official extension index signature verification failed".to_string());
    }
    let index: OfficialIndex = serde_json::from_slice(&payload)
        .map_err(|error| format!("Invalid official extension index payload: {error}"))?;
    if index.schema_version != INDEX_SCHEMA_VERSION {
        return Err(format!(
            "Unsupported official extension index schema version {}",
            index.schema_version
        ));
    }
    if index.index_version == 0 {
        return Err("Official extension index version must be positive".to_string());
    }
    let expires_at = DateTime::parse_from_rfc3339(&index.expires_at)
        .map_err(|error| format!("Invalid official extension index expiry: {error}"))?
        .with_timezone(&Utc);
    if expires_at <= now {
        return Err("Official extension index has expired".to_string());
    }
    if index.entries.iter().any(|entry| {
        entry.extension_id.is_empty()
            || entry.npm_package.is_empty()
            || entry.publisher.is_empty()
            || entry.signing_keys.is_empty()
            || entry.signing_keys.iter().any(|key| parse_key(key).is_err())
    }) {
        return Err("Official extension index contains an invalid entry".to_string());
    }
    Ok(index)
}

fn verify_ed25519(public_key: &str, signature: &str, message: &[u8]) -> Result<bool, String> {
    let verifying_key = VerifyingKey::from_bytes(&parse_key(public_key)?)
        .map_err(|error| format!("Invalid official index root key: {error}"))?;
    let signature = signature.strip_prefix("ed25519:").unwrap_or(signature);
    let signature = base64::engine::general_purpose::STANDARD
        .decode(signature)
        .map_err(|_| "Official extension index signature is not valid Base64".to_string())?;
    let signature = Signature::from_slice(&signature)
        .map_err(|error| format!("Invalid official extension index signature: {error}"))?;
    Ok(verifying_key.verify_strict(message, &signature).is_ok())
}

fn parse_key(value: &str) -> Result<[u8; 32], String> {
    let encoded = value
        .strip_prefix("ed25519:")
        .ok_or("Official extension index keys must use the ed25519: prefix")?;
    base64::engine::general_purpose::STANDARD
        .decode(encoded)
        .map_err(|_| "Official extension index key is not valid Base64".to_string())?
        .try_into()
        .map_err(|_| "Official extension index key must contain exactly 32 bytes".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};

    fn signed_envelope(index: &OfficialIndex, keys: &[SigningKey]) -> Vec<u8> {
        let payload = serde_json::to_vec(index).unwrap();
        let signatures = keys
            .iter()
            .enumerate()
            .map(|(position, key)| IndexSignature {
                key_id: format!("root-{position}"),
                algorithm: "ed25519".into(),
                signature: base64::engine::general_purpose::STANDARD
                    .encode(key.sign(&payload).to_bytes()),
            })
            .collect();
        serde_json::to_vec(&SignedIndexEnvelope {
            payload: base64::engine::general_purpose::STANDARD.encode(payload),
            signatures,
        })
        .unwrap()
    }

    fn fixture(keys: Vec<String>) -> OfficialIndex {
        OfficialIndex {
            schema_version: 1,
            index_version: 7,
            expires_at: "2030-01-01T00:00:00Z".into(),
            entries: vec![OfficialIndexEntry {
                extension_id: "io.example.tool".into(),
                npm_package: "floter-example-tool".into(),
                publisher: "example".into(),
                signing_keys: keys,
            }],
        }
    }

    #[test]
    fn repository_index_file_is_accepted_by_the_pinned_root() {
        // The signed envelope committed at extensions/official-index/index.json
        // must verify against the pinned root key; otherwise discovery marks
        // every package as community and installs lose the official badge.
        let envelope = include_str!("../../../extensions/official-index/index.json");
        assert!(verify(
            envelope.as_bytes(),
            &[DEVELOPMENT_ROOT_PUBLIC_KEY.into()],
            DateTime::parse_from_rfc3339("2026-08-13T00:00:00Z")
                .unwrap()
                .with_timezone(&Utc)
        )
        .is_ok());
    }

    #[test]
    fn bundled_development_index_contains_v_tools_binding() {
        let index: OfficialIndex = serde_json::from_str(DEVELOPMENT_INDEX_PAYLOAD).unwrap();
        let entry = index
            .entries
            .iter()
            .find(|entry| entry.extension_id == "io.github.vst93.v")
            .unwrap();
        assert_eq!(entry.npm_package, "v-tools");
        assert_eq!(entry.publisher, "vst93");
        assert!(!entry.signing_keys.is_empty());
    }

    #[test]
    fn bundled_development_index_matches_the_pinned_development_root() {
        let envelope = SignedIndexEnvelope {
            payload: base64::engine::general_purpose::STANDARD
                .encode(DEVELOPMENT_INDEX_PAYLOAD.as_bytes()),
            signatures: vec![IndexSignature {
                key_id: "development-root-1".into(),
                algorithm: "ed25519".into(),
                signature: DEVELOPMENT_INDEX_SIGNATURE.into(),
            }],
        };
        assert!(verify(
            &serde_json::to_vec(&envelope).unwrap(),
            &[DEVELOPMENT_ROOT_PUBLIC_KEY.into()],
            DateTime::parse_from_rfc3339("2026-08-13T00:00:00Z")
                .unwrap()
                .with_timezone(&Utc)
        )
        .is_ok());
    }

    fn public_key(key: &SigningKey) -> String {
        format!(
            "ed25519:{}",
            base64::engine::general_purpose::STANDARD.encode(key.verifying_key().to_bytes())
        )
    }

    #[test]
    fn rejects_a_tampered_index_payload() {
        let root = SigningKey::from_bytes(&[1; 32]);
        let mut envelope: serde_json::Value = serde_json::from_slice(&signed_envelope(
            &fixture(vec![public_key(&root)]),
            std::slice::from_ref(&root),
        ))
        .unwrap();
        envelope["payload"] =
            serde_json::Value::String(base64::engine::general_purpose::STANDARD.encode(b"{}"));
        assert!(verify(
            &serde_json::to_vec(&envelope).unwrap(),
            &[public_key(&root)],
            Utc::now()
        )
        .is_err());
    }

    #[test]
    fn rejects_an_expired_index() {
        let root = SigningKey::from_bytes(&[2; 32]);
        let mut index = fixture(vec![public_key(&root)]);
        index.expires_at = "2020-01-01T00:00:00Z".into();
        assert_eq!(
            verify(
                &signed_envelope(&index, std::slice::from_ref(&root)),
                &[public_key(&root)],
                Utc::now()
            )
            .unwrap_err(),
            "Official extension index has expired"
        );
    }

    #[test]
    fn accepts_old_and_new_publisher_keys_during_rotation() {
        let old = SigningKey::from_bytes(&[3; 32]);
        let new = SigningKey::from_bytes(&[4; 32]);
        let index = fixture(vec![public_key(&old), public_key(&new)]);
        let signature = |key: &SigningKey| SignatureConfig {
            url: "https://example.com/tool.sig".into(),
            public_key: public_key(key),
            algorithm: crate::extensions::manifest::SignatureAlgorithm::Ed25519,
        };
        assert!(index.authorizes(
            "io.example.tool",
            "floter-example-tool",
            "example",
            Some(&signature(&old))
        ));
        assert!(index.authorizes(
            "io.example.tool",
            "floter-example-tool",
            "example",
            Some(&signature(&new))
        ));
    }

    #[test]
    fn accepts_old_and_new_root_keys_during_rotation() {
        let old = SigningKey::from_bytes(&[6; 32]);
        let new = SigningKey::from_bytes(&[7; 32]);
        let index = fixture(vec![public_key(&old)]);
        let roots = vec![public_key(&old), public_key(&new)];
        assert!(verify(
            &signed_envelope(&index, std::slice::from_ref(&old)),
            &roots,
            Utc::now()
        )
        .is_ok());
        assert!(verify(&signed_envelope(&index, &[new]), &roots, Utc::now()).is_ok());
    }

    #[test]
    fn verifies_an_official_identity_with_a_pinned_publisher_key() {
        let publisher = SigningKey::from_bytes(&[5; 32]);
        let index = fixture(vec![public_key(&publisher)]);
        let signature = SignatureConfig {
            url: "https://example.com/tool.sig".into(),
            public_key: public_key(&publisher),
            algorithm: crate::extensions::manifest::SignatureAlgorithm::Ed25519,
        };
        assert!(index.authorizes(
            "io.example.tool",
            "floter-example-tool",
            "example",
            Some(&signature)
        ));
        assert!(!index.authorizes(
            "io.example.other",
            "floter-example-tool",
            "example",
            Some(&signature)
        ));
    }

    #[test]
    fn accepted_index_version_is_persisted_and_cannot_roll_back() {
        let directory = tempfile::tempdir().unwrap();
        let path = directory.path().join("official-index-state.json");
        let accepted = std::sync::Mutex::new(0);

        accept_version(&path, &accepted, 7).unwrap();
        assert_eq!(load_accepted_version(&path).unwrap(), 7);
        accept_version(&path, &accepted, 7).unwrap();
        assert!(accept_version(&path, &accepted, 6)
            .unwrap_err()
            .contains("rollback rejected"));
        assert_eq!(load_accepted_version(&path).unwrap(), 7);

        accept_version(&path, &accepted, 8).unwrap();
        assert_eq!(load_accepted_version(&path).unwrap(), 8);
    }
}
