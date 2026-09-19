// R7-8 · the one vocabulary for "which permissions Floter actually refuses".
//
// `plugin-system-audit.md` §3.1 names the trust-boundary contradiction: the UI
// rendered every declared permission as the same shape, so a user could not
// tell a *host-enforced* check from a *declared-only* one. This module is the
// product's single answer to "is this one really blocked?" — the review
// dialog, the local-install dialog and the custom-integration editor all read
// it, so there is never a second list that can drift.
//
// The distinction is about the **host's own calls**, not about the operating
// system:
//   * `environment` / `process-spawn` — Floter owns the decision (whether to
//     hand over its environment, whether to start another program on a
//     descriptor's behalf) and refuses an unapproved request.
//   * everything else — filesystem, network, clipboard — is what the native
//     command does once it runs. Floter does not intercept it. The declaration
//     is disclosure for the user, and there is no sandbox (Phase 6 is not
//     implemented; the copy must never imply otherwise).

export type PermissionTier = "enforced" | "disclosure";

/** Permissions where Floter, not the OS, owns the yes/no. */
export const HOST_ENFORCED_PERMISSIONS = ["environment", "process-spawn"] as const;

export const permissionTier = (permission: string): PermissionTier =>
  (HOST_ENFORCED_PERMISSIONS as readonly string[]).includes(permission)
    ? "enforced"
    : "disclosure";

/**
 * Split a declared/reviewed permission list into the two tiers, preserving the
 * input order inside each group. A permission the host does not know about is
 * disclosure, never enforced: over-claiming enforcement is the failure mode
 * that would make the UI lie.
 *
 * R7-8a · When the backend sent its own `enforcement` on the wire (the review
 * payload carries one per summary), that value is authoritative and is used
 * directly — the Rust classifier is the source of truth and this map is only
 * the synchronous projection the custom editor needs (it renders checkboxes on
 * every keystroke and cannot await IPC per permission). A permission that
 * arrives without the field still falls back to the local projection, so a
 * caller that builds a list by hand is not silently tiered as enforced.
 */
export const groupPermissions = <T extends { permission: string; enforcement?: string }>(
  permissions: readonly T[],
): { enforced: T[]; disclosure: T[] } => {
  const enforced: T[] = [];
  const disclosure: T[] = [];
  for (const permission of permissions) {
    const tier = permission.enforcement ? wireTier(permission.enforcement) : permissionTier(permission.permission);
    (tier === "enforced" ? enforced : disclosure).push(permission);
  }
  return { enforced, disclosure };
};

/**
 * The backend serializes `PermissionEnforcement` lowercase (`enforced` /
 * `disclosed`); the UI calls the second tier `disclosure`. Normalize here so
 * the wire vocabulary and the UI vocabulary meet in exactly one place. An
 * unrecognized value is disclosure — under-claim by default, never over-claim.
 */
export const wireTier = (enforcement: string): PermissionTier =>
  enforcement === "enforced" ? "enforced" : "disclosure";
