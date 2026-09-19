//! The single answer to "can this entry's runtime run right now?".
//!
//! Audit G5: "is it available" used to be answered by two state machines in
//! parallel — the repository (`ExtensionStateKind` + the persisted error
//! record) and `tool-lock.json` (`LockState`). `commands::extensions`
//! projected `runtime_available` from the lock while the catalog projected a
//! *different* answer into the repository, so a row could read `broken` and
//! `Ready` at the same time, and a catalog-load failure that only
//! `eprintln!`-ed left the row claiming the tool was fine.
//!
//! This module is the one projector. [`RuntimeBinding::project`] is the only
//! place the precedence between the repository's state machine and the live
//! binding is decided, and both read paths derive from it:
//!
//! * the list path (`commands::extensions::ExtensionListItem::installed`)
//!   turns it into the row's `runtime_available` plus the reason fields the UI
//!   renders;
//! * the catalog load path classifies its binding/descriptor/describe
//!   failures with [`failure_code`] and records them in the repository, which
//!   is what makes a previously-silent failure visible on the next list.
//!
//! The repository is the authority. A persisted `broken` state is never
//! reported as "ready" merely because the executable happens to exist and the
//! lock happens to say `Connected`; the live binding can only *narrow* the
//! answer (an executable that disappeared between two catalog loads), never
//! widen it back.

use crate::extensions::error_codes::ProviderErrorCode;
use crate::extensions::lock::{ExtensionLockEntry, ExtensionStateKind};
use crate::extensions::tool_lock::LockState;

/// The projected availability of one entry's runtime.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct RuntimeBinding {
    available: bool,
    code: Option<String>,
    detail: Option<String>,
}

impl RuntimeBinding {
    pub(crate) fn available() -> Self {
        Self {
            available: true,
            code: None,
            detail: None,
        }
    }

    pub(crate) fn unavailable(code: impl Into<String>, detail: impl Into<String>) -> Self {
        Self {
            available: false,
            code: Some(code.into()),
            detail: Some(detail.into()),
        }
    }

    pub(crate) fn is_available(&self) -> bool {
        self.available
    }

    /// The structured reason (`ProviderErrorCode::as_str()` shape) the UI
    /// translates. `None` only when the runtime is available.
    pub(crate) fn code(&self) -> Option<&str> {
        self.code.as_deref()
    }

    /// The human detail recorded with the failure — it names the actual file
    /// or provider error, so the row can say *why* rather than only *that*.
    pub(crate) fn detail(&self) -> Option<&str> {
        self.detail.as_deref()
    }

    /// Project the availability of `entry`, given the effective state of its
    /// executable binding (`None` for entries whose runtime is not a bound
    /// executable).
    pub(crate) fn project(entry: &ExtensionLockEntry, binding_state: Option<LockState>) -> Self {
        Self::project_with(entry, binding_state, || live_runtime(entry))
    }

    /// [`RuntimeBinding::project`] with the live check injected, so the
    /// precedence rules can be asserted without a real manifest/executable
    /// fixture. The production entry point is [`RuntimeBinding::project`].
    fn project_with(
        entry: &ExtensionLockEntry,
        binding_state: Option<LockState>,
        live: impl FnOnce() -> Result<(), (String, String)>,
    ) -> Self {
        // 1. The repository state machine is the authority: a persisted failure
        //    is reported with the reason it was recorded under, whatever the
        //    live binding or the executable on disk says. This is the rule
        //    that removes the old `broken` + `Ready` contradiction.
        if entry.state == ExtensionStateKind::Broken {
            return Self::unavailable(
                entry
                    .last_error_code
                    .clone()
                    .unwrap_or_else(|| ProviderErrorCode::BindingCheckFailed.as_str().to_string()),
                entry
                    .broken_reason
                    .clone()
                    .or_else(|| entry.last_error_detail.clone())
                    .unwrap_or_else(|| {
                        "The integration is unusable until it is repaired".to_string()
                    }),
            );
        }
        // 2. Then the live binding the user actually chose: a lock state that
        //    is not `Connected` is unavailable even before the catalog has had
        //    a chance to record it.
        match binding_state {
            Some(LockState::ReconnectRequired) => {
                return Self::unavailable(
                    ProviderErrorCode::BindingMissing.as_str(),
                    format!(
                        "Executable is no longer available at {}",
                        entry.executable_path
                    ),
                );
            }
            Some(LockState::ReverifyRequired) => {
                return Self::unavailable(
                    ProviderErrorCode::BindingChanged.as_str(),
                    format!(
                        "Executable fingerprint changed at {}",
                        entry.executable_path
                    ),
                );
            }
            Some(LockState::Connected) | None => {}
        }
        // 3. Finally the runtime the binding resolves to has to exist.
        match live() {
            Ok(()) => Self::available(),
            Err((code, detail)) => Self::unavailable(code, detail),
        }
    }
}

/// Whether the executable that `entry`'s binding resolves to is on disk.
fn live_runtime(entry: &ExtensionLockEntry) -> Result<(), (String, String)> {
    match crate::extensions::registry::provider_invocation(entry) {
        Ok(invocation) if invocation.executable.is_file() => Ok(()),
        Ok(invocation) => Err((
            ProviderErrorCode::BindingMissing.as_str().to_string(),
            format!(
                "Executable is no longer available at {}",
                invocation.executable.display()
            ),
        )),
        Err(error) => Err((failure_code(&error).to_string(), error)),
    }
}

/// Whether a `broken` entry's runtime can be proven usable without launching
/// the tool: the same structural validation the catalog applies to a refreshed
/// binding (manifest identity + platform resolution, plus a full descriptor
/// parse for static providers).
///
/// The catalog gates its `broken` -> restorable transition on this, so a
/// descriptor that no longer parses cannot flip the entry back to `enabled`
/// for one load and drop the failure record.
pub(crate) fn recovery_is_proven(entry: &ExtensionLockEntry) -> bool {
    crate::extensions::catalog::validate_refreshed_binding(entry).is_ok()
}

/// The error code a catalog-load failure is recorded under.
///
/// Provider errors that already embed a `[code]` keep it; structural failures
/// (a manifest or descriptor that will not load) are classified by the same
/// kinds of keyword the verify path uses, so a row's code always has a
/// dictionary entry in `src/i18n.ts` (`settings.extensions.errorCode.*`).
pub(crate) fn failure_code(error: &str) -> &'static str {
    if let (Some(code), _) = ProviderErrorCode::extract_from_message(error) {
        return code.as_str();
    }
    let lowered = error.to_ascii_lowercase();
    if lowered.contains("identity")
        || lowered.contains("does not match extension id")
        || lowered.contains("does not match lock entry")
        || lowered.contains("publisher changed")
    {
        ProviderErrorCode::IdentityMismatch.as_str()
    } else if lowered.contains("manifest") {
        ProviderErrorCode::ManifestMissing.as_str()
    } else if lowered.contains("descriptor") {
        ProviderErrorCode::InvalidDescriptor.as_str()
    } else {
        ProviderErrorCode::BindingCheckFailed.as_str()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extensions::error_codes::ProviderErrorCode;

    fn entry(state: ExtensionStateKind) -> ExtensionLockEntry {
        serde_json::from_value(serde_json::json!({
            "id": "local.example",
            "name": "Example",
            "publisherId": "local-user",
            "publisherName": "Local user",
            "distributionSource": "local",
            "runtimeOwnership": "system",
            "providerKind": "static-descriptor",
            "state": match state {
                ExtensionStateKind::Enabled => "enabled",
                ExtensionStateKind::Disabled => "disabled",
                ExtensionStateKind::Broken => "broken",
            },
            "enabled": true,
            "packageName": null,
            "packageVersion": "1.0.0",
            "toolVersion": null,
            "integrity": null,
            "runtimeIntegrity": null,
            "contentIntegrity": null,
            "previousIntegrity": null,
            "previousRuntimeIntegrity": null,
            "previousContentIntegrity": null,
            "signatureVerified": false,
            "previousSignatureVerified": null,
            "officialVerified": false,
            "previousOfficialVerified": null,
            "currentVersion": "1.0.0",
            "previousVersion": null,
            "manifestPath": "/tmp/example/floter.extension.json",
            "executablePath": "/tmp/example/tool",
            "runtimeRoot": null,
            "installedAt": 1,
            "updatedAt": 1,
            "pinned": false,
            "channel": "external",
            "approvedPermissions": [],
            "approvedAt": 0,
            "approvedManifestDigest": null,
            "lastErrorCode": null,
            "lastErrorDetail": null,
            "lastErrorAt": null,
            "brokenReason": null,
            "enabledBeforeBroken": null,
            "configGeneration": 0
        }))
        .unwrap()
    }

    /// The whole point of the projector: a repository failure outweighs a
    /// `Connected` lock. The row must never read `broken` and `Ready` at once.
    #[test]
    fn a_broken_repository_entry_is_unavailable_even_with_a_connected_binding() {
        let mut broken = entry(ExtensionStateKind::Broken);
        broken.last_error_code = Some("describe-failed".into());
        broken.broken_reason = Some("Provider describe failed: exit 2".into());

        let projected =
            RuntimeBinding::project_with(&broken, Some(LockState::Connected), || Ok(()));

        assert!(!projected.is_available());
        assert_eq!(projected.code(), Some("describe-failed"));
        assert_eq!(
            projected.detail(),
            Some("Provider describe failed: exit 2"),
            "the row must be able to say why, not only that"
        );
    }

    /// The live binding narrows a healthy repository row: a tool removed
    /// between two catalog loads is unavailable with the binding reason.
    #[test]
    fn a_non_connected_binding_makes_an_enabled_entry_unavailable() {
        for (state, code) in [
            (LockState::ReconnectRequired, "binding-missing"),
            (LockState::ReverifyRequired, "binding-changed"),
        ] {
            let projected =
                RuntimeBinding::project_with(&entry(ExtensionStateKind::Enabled), Some(state), || {
                    Ok(())
                });
            assert!(!projected.is_available(), "{state:?}");
            assert_eq!(projected.code(), Some(code), "{state:?}");
            assert!(projected
                .detail()
                .is_some_and(|detail| detail.contains("/tmp/example/tool")));
        }
    }

    #[test]
    fn a_resolvable_enabled_entry_is_available() {
        let projected = RuntimeBinding::project_with(
            &entry(ExtensionStateKind::Enabled),
            Some(LockState::Connected),
            || Ok(()),
        );
        assert!(projected.is_available());
        assert_eq!(projected.code(), None);
        assert_eq!(projected.detail(), None);
    }

    /// An unbound (no lock state) but resolvable entry is available: absence
    /// of a binding is not itself a failure, the live runtime decides.
    #[test]
    fn an_entry_without_a_binding_falls_back_to_the_live_runtime() {
        let projected =
            RuntimeBinding::project_with(&entry(ExtensionStateKind::Enabled), None, || Ok(()));
        assert!(projected.is_available());
    }

    #[test]
    fn a_live_runtime_failure_is_projected_with_its_own_reason() {
        let projected = RuntimeBinding::project_with(
            &entry(ExtensionStateKind::Enabled),
            Some(LockState::Connected),
            || {
                Err((
                    ProviderErrorCode::BindingMissing.as_str().to_string(),
                    "Executable is no longer available at /tmp/example/tool".to_string(),
                ))
            },
        );
        assert!(!projected.is_available());
        assert_eq!(projected.code(), Some("binding-missing"));
    }

    /// Every code the catalog can record has to be one the UI dictionary can
    /// translate, or the row's reason is half-rendered. The node test
    /// `tests/runtime-binding.test.ts` pins the other half (the i18n keys).
    #[test]
    fn failure_codes_are_provider_error_codes() {
        for error in [
            "[tool-error] Provider describe exited with 7: boom",
            "[describe-parse-failed] Provider describe returned invalid JSON",
            "Manifest identity does not match lock entry local.example",
            "Cannot read manifest /tmp/example/floter.extension.json: missing",
            "Cannot read static provider descriptor /tmp/example/p.json: missing",
            "Script file is missing: /tmp/example/tool.sh",
        ] {
            let code = failure_code(error);
            assert_eq!(
                ProviderErrorCode::from_str(code).map(ProviderErrorCode::as_str),
                Some(code),
                "{error} -> {code} is not a known provider error code"
            );
        }
        assert_eq!(
            failure_code("[tool-error] Provider describe exited with 7: boom"),
            "tool-error",
            "an embedded code must be preserved verbatim"
        );
        assert_eq!(
            failure_code("Provider id com.other does not match extension id local.example"),
            "identity-mismatch"
        );
    }
}
