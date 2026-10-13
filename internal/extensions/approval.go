package extensions

import "strings"

// The approval record's pure parts (the old host's R7-8 /
// `src/extensions/approval-record.ts`).
//
// The repository entry keeps the audit fields — what was approved, when, and
// the manifest digest the approval is bound to — and the settings card draws
// them as a record rather than a single timestamp. These two helpers are the
// falsifiable bits: a digest is shown short, and a changed manifest is
// *reported*, never acted on. There is deliberately no rollback/downgrade:
// "the manifest changed" can only mean "the approval no longer covers the
// bytes on disk" — a prompt to re-approve, not a version state to restore.

// ApprovalDigestHexChars is how many hex characters the record shows after
// the `sha256-` prefix: git's short-hash width. 12 hex is 48 bits, so two
// manifests that differ anywhere in the first dozen hex characters get
// distinct display strings. The stored digest is a full SHA-256 (`sha256-` +
// 64 hex, `ManifestDigest`), so 12 is a *display* truncation, never a value
// the comparison uses — `ApprovalIsStale` always sees the full strings.
const ApprovalDigestHexChars = 12

// approvalDigestPrefix is the total shown: the 7-character `sha256-` prefix
// plus the hex characters.
const approvalDigestPrefix = len("sha256-") + ApprovalDigestHexChars

// ShortDigest is the short form of a digest: `sha256-abcdef…` →
// `sha256-abcdef012345`. An unknown/empty digest returns "" so the caller
// omits the row's value instead of printing a placeholder; a digest already
// at or under the width is shown whole (rune-safe: the count is in
// characters, like the old host's `.length`).
func ShortDigest(digest string) string {
	value := strings.TrimSpace(digest)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= approvalDigestPrefix {
		return value
	}
	return string(runes[:approvalDigestPrefix])
}

// ApprovalIsStale is true only when both sides are known and differ. A
// missing recorded digest (old data) or a missing current digest (the
// manifest could not be read) is *unknown*, not stale — the UI says nothing
// rather than raising a false flag.
func ApprovalIsStale(approvedManifestDigest, currentManifestDigest string) bool {
	return approvedManifestDigest != "" &&
		currentManifestDigest != "" &&
		approvedManifestDigest != currentManifestDigest
}
