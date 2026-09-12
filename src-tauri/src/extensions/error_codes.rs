//! Standard error codes for provider/runtime operations.
//!
//! Error codes are embedded in error messages using the format:
//! `"[error-code] Human-readable message"`. This allows structured error
//! handling while preserving backwards compatibility with string errors.

/// Standard error codes for provider operations.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ProviderErrorCode {
    /// Provider protocol version not supported by this host
    ProtocolUnsupported,
    /// Provider describe output failed JSON parsing
    DescribeParseFailed,
    /// Provider stdout contains non-JSON content or ANSI codes
    StdoutContaminated,
    /// Provider exited with code 2 (protocol error)
    ProtocolError,
    /// Provider exited with non-zero, non-2 code (tool error)
    ToolError,
    /// Provider operation timed out
    Timeout,
    /// Operation was cancelled by user
    Cancelled,
    /// Provider executable is missing or unavailable
    BindingMissing,
    /// Provider executable fingerprint changed
    BindingChanged,
    /// Binding check failed (other error)
    BindingCheckFailed,
    /// Provider id does not match extension id
    IdentityMismatch,
    /// Extension manifest is missing or invalid
    ManifestMissing,
    /// Provider descriptor validation failed
    InvalidDescriptor,
}

impl ProviderErrorCode {
    /// Returns the string representation of this error code.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::ProtocolUnsupported => "protocol-unsupported",
            Self::DescribeParseFailed => "describe-parse-failed",
            Self::StdoutContaminated => "stdout-contaminated",
            Self::ProtocolError => "protocol-error",
            Self::ToolError => "tool-error",
            Self::Timeout => "timeout",
            Self::Cancelled => "cancelled",
            Self::BindingMissing => "binding-missing",
            Self::BindingChanged => "binding-changed",
            Self::BindingCheckFailed => "binding-check-failed",
            Self::IdentityMismatch => "identity-mismatch",
            Self::ManifestMissing => "manifest-missing",
            Self::InvalidDescriptor => "invalid-descriptor",
        }
    }

    /// Parse an error code from its string representation.
    pub fn from_str(code: &str) -> Option<Self> {
        match code {
            "protocol-unsupported" => Some(Self::ProtocolUnsupported),
            "describe-parse-failed" => Some(Self::DescribeParseFailed),
            "stdout-contaminated" => Some(Self::StdoutContaminated),
            "protocol-error" => Some(Self::ProtocolError),
            "tool-error" => Some(Self::ToolError),
            "timeout" => Some(Self::Timeout),
            "cancelled" => Some(Self::Cancelled),
            "binding-missing" => Some(Self::BindingMissing),
            "binding-changed" => Some(Self::BindingChanged),
            "binding-check-failed" => Some(Self::BindingCheckFailed),
            "identity-mismatch" => Some(Self::IdentityMismatch),
            "manifest-missing" => Some(Self::ManifestMissing),
            "invalid-descriptor" => Some(Self::InvalidDescriptor),
            _ => None,
        }
    }

    /// Extract error code from a formatted error message.
    /// Returns (Option<ProviderErrorCode>, message_without_code).
    pub fn extract_from_message(message: &str) -> (Option<Self>, String) {
        if let Some(rest) = message.strip_prefix('[') {
            if let Some(close_idx) = rest.find(']') {
                let code_str = &rest[..close_idx];
                let remaining = rest[close_idx + 1..].trim_start();
                if let Some(code) = Self::from_str(code_str) {
                    return (Some(code), remaining.to_string());
                }
            }
        }
        (None, message.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_error_code_roundtrip() {
        let codes = [
            ProviderErrorCode::ProtocolUnsupported,
            ProviderErrorCode::DescribeParseFailed,
            ProviderErrorCode::StdoutContaminated,
            ProviderErrorCode::ProtocolError,
            ProviderErrorCode::ToolError,
            ProviderErrorCode::Timeout,
            ProviderErrorCode::Cancelled,
            ProviderErrorCode::BindingMissing,
            ProviderErrorCode::BindingChanged,
            ProviderErrorCode::BindingCheckFailed,
            ProviderErrorCode::IdentityMismatch,
            ProviderErrorCode::ManifestMissing,
            ProviderErrorCode::InvalidDescriptor,
        ];
        for code in codes {
            let s = code.as_str();
            assert_eq!(ProviderErrorCode::from_str(s), Some(code));
        }
    }

    #[test]
    fn test_extract_from_message() {
        let (code, msg) = ProviderErrorCode::extract_from_message(
            "[timeout] Provider describe timed out after 5000 ms",
        );
        assert_eq!(code, Some(ProviderErrorCode::Timeout));
        assert_eq!(msg, "Provider describe timed out after 5000 ms");

        let (code, msg) = ProviderErrorCode::extract_from_message("Plain error without code");
        assert_eq!(code, None);
        assert_eq!(msg, "Plain error without code");

        let (code, msg) = ProviderErrorCode::extract_from_message("[unknown-code] Some error");
        assert_eq!(code, None);
        assert_eq!(msg, "[unknown-code] Some error");
    }
}
