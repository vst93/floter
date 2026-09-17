// Freshness projection behind the integration drawer's "Freshness" section.
//
// Pure and DOM-free so the node suite can drive the exact unknown/known and
// delta cases without a renderer. The rule the whole module exists to keep is
// that **"we do not know" is never painted as a number**: a missing sidecar, a
// first probe with nothing to compare against, and a genuine zero all have to
// look different, because a fabricated "0" or "no change" is a lie the user
// cannot see through.
//
// Nothing here is a version/upgrade concept. `commandCount` moves because the
// *tool's own* `--help` output changed under Floter's feet; the drawer reports
// that observation, it never offers to fetch anything (see AGENT-NOTES: no
// store-style update centre, no version lists, no upgrade semantics).

/** Where the displayed timestamp came from, so the drawer can label it. */
export type FreshnessSource = "probe" | "health" | "none";

/** What the last probe did. `degraded` is the health report's "only optional
 *  probes failed" — reported as its own word rather than rounded to success or
 *  failure, both of which would be wrong. */
export type FreshnessResult = "running" | "failed" | "degraded" | "success" | "unknown";

export type CommandDeltaKind = "increase" | "decrease" | "unchanged" | "unknown";

export type Freshness = {
  /** Unix seconds, or `null` when nothing has ever probed this integration. */
  atSeconds: number | null;
  source: FreshnessSource;
  result: FreshnessResult;
  delta: { kind: CommandDeltaKind; count: number | null };
  commandCount: number | null;
  previousCommandCount: number | null;
};

export type FreshnessInput = {
  /** `lastProbeAt` from the list item (the probe sidecar). */
  lastProbeAt?: number | null;
  /** `checkedAt` of the persisted health report (RFC 3339), if any. */
  healthCheckedAt?: string | null;
  healthStatus?: "healthy" | "degraded" | "unhealthy" | "unknown" | null;
  /** `lastErrorCode` of the persisted failure record, if any. */
  errorCode?: string | null;
  /** A probe is in flight right now (manual re-probe, or an op-progress frame). */
  running?: boolean;
  commandCount?: number | null;
  previousCommandCount?: number | null;
};

/** Parse an RFC 3339 timestamp to unix seconds; `null` when unparsable. */
export const unixSeconds = (value: string | null | undefined): number | null => {
  if (!value) return null;
  const millis = Date.parse(value);
  return Number.isFinite(millis) ? Math.floor(millis / 1000) : null;
};

/**
 * Compare the last probe's command count with the one before it.
 *
 * `previous === null` (or `current === null`) is `unknown`, never `unchanged`:
 * a first probe has nothing to compare against, and saying "no change" about a
 * comparison that never happened is exactly the kind of confident-but-empty
 * status this round is trying to remove.
 */
export const commandDelta = (
  previous: number | null | undefined,
  current: number | null | undefined,
): { kind: CommandDeltaKind; count: number | null } => {
  if (typeof previous !== "number" || typeof current !== "number") {
    return { kind: "unknown", count: null };
  }
  const difference = current - previous;
  if (difference > 0) return { kind: "increase", count: difference };
  if (difference < 0) return { kind: "decrease", count: -difference };
  return { kind: "unchanged", count: 0 };
};

/**
 * Collapse the two independent freshness sources into one row of facts.
 *
 * The probe sidecar wins over the health report because it records the *command*
 * probe specifically (which is what this section is about); the health report is
 * the fallback for every integration that has no sidecar — a publisher
 * descriptor, a script, or a row probed before the sidecar existed.
 */
export const freshnessOf = (input: FreshnessInput): Freshness => {
  const probeSeconds = typeof input.lastProbeAt === "number" && input.lastProbeAt > 0
    ? input.lastProbeAt
    : null;
  const healthSeconds = unixSeconds(input.healthCheckedAt);
  const atSeconds = probeSeconds ?? healthSeconds;
  const source: FreshnessSource = probeSeconds !== null
    ? "probe"
    : healthSeconds !== null
      ? "health"
      : "none";

  let result: FreshnessResult;
  if (input.running) {
    result = "running";
  } else if (input.errorCode || input.healthStatus === "unhealthy") {
    result = "failed";
  } else if (input.healthStatus === "degraded") {
    result = "degraded";
  } else if (atSeconds !== null) {
    result = "success";
  } else {
    result = "unknown";
  }

  const commandCount = typeof input.commandCount === "number" ? input.commandCount : null;
  const previousCommandCount = typeof input.previousCommandCount === "number"
    ? input.previousCommandCount
    : null;

  return {
    atSeconds,
    source,
    result,
    delta: commandDelta(previousCommandCount, commandCount),
    commandCount,
    previousCommandCount,
  };
};

/**
 * "3 minutes ago" in the user's language, from the platform's own
 * `Intl.RelativeTimeFormat` — no dependency, no hand-rolled plural table, and
 * correct in both of Floter's languages. `justNowKey` is the caller's
 * translated string for the sub-minute case, which every locale words
 * differently enough that a table of our own would be worse than one key.
 */
export const relativeTime = (
  seconds: number,
  now: number,
  locale: "en" | "zh",
  justNow: string,
): string => {
  const elapsed = Math.max(0, now - seconds);
  if (elapsed < 45) return justNow;
  const format = (value: number, unit: Intl.RelativeTimeFormatUnit) =>
    new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" }).format(-value, unit);
  if (elapsed < 3_600) return format(Math.round(elapsed / 60), "minute");
  if (elapsed < 86_400) return format(Math.round(elapsed / 3_600), "hour");
  if (elapsed < 2_592_000) return format(Math.round(elapsed / 86_400), "day");
  if (elapsed < 31_536_000) return format(Math.round(elapsed / 2_592_000), "month");
  return format(Math.round(elapsed / 31_536_000), "year");
};

/**
 * Session-scoped "tell the user once" gate for the silent drift re-probe.
 *
 * G2's background re-probe is deliberately invisible; R7-7 makes it visible
 * exactly once per actual change. The backend emits one notice frame per
 * successful re-probe, but a listing loop can legitimately re-probe several
 * times in a session (a tool upgraded twice, or a retried probe that finally
 * succeeds), and a user should never be told the same thing twice.
 *
 * The key is `id · toolVersion · delta`, so a *different* change still
 * notifies. Nothing is persisted: a fresh app launch re-arms, which is the
 * right semantics for "your commands just changed" — it is news, not a
 * read/unread flag.
 */
export const createReprobeNoticeGate = () => {
  const seen = new Set<string>();
  return {
    /** True the first time this key is offered; false every time after. */
    allow: (key: string): boolean => {
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    },
    size: () => seen.size,
  };
};

/** Stable identity of one drift notice, for [`createReprobeNoticeGate`]. */
export const reprobeNoticeKey = (
  extensionId: string,
  toolVersion: string | null | undefined,
  delta: { kind: CommandDeltaKind; count: number | null },
): string => `${extensionId}\u0000${toolVersion ?? ""}\u0000${delta.kind}:${delta.count ?? ""}`;

/** What a drift notice frame carries, as it arrives on the wire. */
export type ReprobeNoticeFrame = {
  extensionId: string;
  toolVersion?: string | null;
  previousCommandCount?: number | null;
  commandCount?: number | null;
};

/** The two independent outcomes a notice frame can produce. */
export type DriftNoticeDecision = {
  /** The display facts for the drawer, whenever a re-probe completed —
   *  including the ones that are not worth a toast. */
  display: {
    extensionId: string;
    toolVersion: string | null;
    commandCount: number | null;
    previousCommandCount: number | null;
  };
  /** Non-null only when this frame should raise exactly one toast. */
  announce: { delta: { kind: "increase" | "decrease"; count: number }; key: string } | null;
};

/**
 * Decide what one drift notice frame means.
 *
 * Three rules, in this order, and the order is the whole point:
 *
 * 1. **No change, no toast.** A re-probe that left the command count alone has
 *    nothing a user could act on. This is checked *before* the gate is
 *    consulted, so a no-op frame can never burn the once-only slot and silence
 *    the real change that follows.
 * 2. **No name, no toast.** Without the row's name the message would have to say
 *    "an integration", which is worse than waiting; the gate is therefore
 *    consulted only once the caller has supplied a name.
 * 3. **Once per change, per session.** The gate keys on
 *    (integration, version, direction, magnitude), so a re-offered identical
 *    frame stays silent while a genuinely different change still speaks.
 *
 * `extensionName` is `null` when the row is not in the list yet; the caller
 * passes it in because only it can resolve ids to names.
 */
export const decideDriftNotice = (
  frame: ReprobeNoticeFrame,
  extensionName: string | null,
  gate: { allow: (key: string) => boolean },
): DriftNoticeDecision => {
  const previous = typeof frame.previousCommandCount === "number" ? frame.previousCommandCount : null;
  const current = typeof frame.commandCount === "number" ? frame.commandCount : null;
  const display = {
    extensionId: frame.extensionId,
    toolVersion: frame.toolVersion ?? null,
    commandCount: current,
    previousCommandCount: previous,
  };
  if (previous === null || current === null || previous === current) {
    return { display, announce: null };
  }
  if (!extensionName) {
    return { display, announce: null };
  }
  const kind = current > previous ? "increase" : "decrease";
  const delta = { kind, count: Math.abs(current - previous) } as const;
  const key = reprobeNoticeKey(frame.extensionId, frame.toolVersion, delta);
  if (!gate.allow(key)) {
    return { display, announce: null };
  }
  return { display, announce: { delta, key } };
};
