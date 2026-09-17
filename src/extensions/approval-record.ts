// R7-8 · the approval record's pure parts.
//
// The lock entry keeps the Phase 2 audit fields — what was approved, when, and
// the manifest digest the approval is bound to (`lock.rs`,
// `record_permission_approval`). The drawer renders them as a record rather
// than a single timestamp, and these two helpers are the falsifiable bits: a
// digest is shown short, and a changed manifest is *reported*, never acted on.
//
// There is deliberately no rollback/downgrade here: the NPM update chain was
// physically removed (`350e2d6`), so "the manifest changed" can only mean
// "approval no longer covers the bytes on disk" — a prompt to re-approve, not
// a version state to restore.

/**
 * How many characters of a digest the record shows: the 7-character `sha256-`
 * prefix plus 8 hex characters (32 bits of the digest). The previous value of
 * 8 sliced the whole string, so it displayed only a single hex digit — two
 * digests differing in their 8th hex character were indistinguishable.
 */
export const APPROVAL_DIGEST_PREFIX = 15;

/**
 * The short form of a recorded digest. `sha256-abcdef…` → `sha256-abcdef01`;
 * an unknown/empty digest returns null so the caller omits the row instead of
 * printing a placeholder.
 */
export const shortDigest = (digest: string | null | undefined): string | null => {
  const value = digest?.trim();
  if (!value) return null;
  return value.length <= APPROVAL_DIGEST_PREFIX ? value : value.slice(0, APPROVAL_DIGEST_PREFIX);
};

/**
 * True only when both sides are known and differ. A missing recorded digest
 * (old data) or a missing current digest (the manifest could not be read) is
 * *unknown*, not stale — the UI says nothing rather than raising a false flag.
 */
export const approvalIsStale = (
  approvedManifestDigest: string | null | undefined,
  currentManifestDigest: string | null | undefined,
): boolean =>
  Boolean(approvedManifestDigest) &&
  Boolean(currentManifestDigest) &&
  approvedManifestDigest !== currentManifestDigest;
