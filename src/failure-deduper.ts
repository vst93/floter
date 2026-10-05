// Per-key failure deduplication for the app's automatic triggers.
//
// R101 · this used to share one module with the base-plugin registry, under a
// name that named neither subject honestly; the two were split so each file
// names one thing. The registry now lives in `src/builtin-plugins.ts`.
//
// Kept free of React and Tauri so the node test suite can exercise it directly.

/**
 * How long a key stays quiet after it has been raised once. Long enough that a
 * 2s background poll cannot fill the stack (fifteen failures per toast), short
 * enough that the same failure coming back later is announced again. */
export const FAILURE_NOTIFY_DEDUP_MS = 30_000;

export type FailureDeduper = {
  /**
   * Whether `key` may be raised now, and if so, mark it as raised. The second
   * and later calls inside the window return false; the first call after it
   * elapses returns true again. `now` is injectable so the node suite can
   * drive a 5-failure burst without waiting wall-clock time.
   */
  allow: (key: string, now?: number) => boolean;
  /** Re-arm a key: the failure that comes after this is news again. */
  clear: (key: string) => void;
};

/**
 * Deduper for failures raised by an automatic trigger.
 *
 * Why this exists: an automatic failure — a 2s poll against a backend that
 * stays down, a `floter://` link a hostile page retries in a loop — would
 * otherwise raise one toast per attempt forever, three toasts churning on
 * screen, none readable. Why it is scoped per key rather than wrapping every
 * failure report: a failure caused by a *user gesture* (a copy, a delete) must
 * still report each time the user asks, or the second click fails silently.
 * Only the automatic triggers need coalescing, and each names one key.
 */
export const createFailureDeduper = (
  windowMs: number = FAILURE_NOTIFY_DEDUP_MS,
): FailureDeduper => {
  const raisedAt = new Map<string, number>();
  return {
    allow: (key, now = Date.now()) => {
      const last = raisedAt.get(key);
      if (last !== undefined && now - last < windowMs) return false;
      raisedAt.set(key, now);
      return true;
    },
    clear: (key) => {
      raisedAt.delete(key);
    },
  };
};
