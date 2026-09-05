/** Settings pages in sidebar order; also the ↑/↓ navigation cycle. */
export const SETTINGS_PAGES = [
  "general",
  "sessions",
  "shortcuts",
  "integrations",
  "about",
] as const;

export type SettingsPage = (typeof SETTINGS_PAGES)[number];

/** Map a persisted page name onto a known page; anything else falls back. */
export function normalizeSettingsPage(value: unknown): SettingsPage {
  return (SETTINGS_PAGES as readonly string[]).includes(value as string)
    ? (value as SettingsPage)
    : "general";
}

export type SettingsHydration<T extends object> = {
  isReady: () => boolean;
  hasFailed: () => boolean;
  markChanged: (field: keyof T) => void;
  markFailed: () => void;
  mergeLoaded: (current: T, loaded: T) => T;
  finish: () => void;
  waitUntilReady: () => Promise<void>;
};

/**
 * Coordinate the initial settings read (including retries) with edits made
 * while it is in flight. Edited fields win during hydration, while writes wait
 * until the remaining fields have been populated from disk. A failed read does
 * not release writers, because their partial frontend snapshot must never
 * replace a complete persisted configuration.
 */
export function createSettingsHydration<T extends object>(): SettingsHydration<T> {
  const changed = new Set<keyof T>();
  let ready = false;
  let failed = false;
  let resolveReady: (() => void) | null = null;
  const readyPromise = new Promise<void>((resolve) => {
    resolveReady = resolve;
  });

  return {
    isReady: () => ready,
    hasFailed: () => failed,
    markChanged: (field) => {
      if (!ready) changed.add(field);
    },
    markFailed: () => {
      if (!ready) failed = true;
    },
    mergeLoaded: (current, loaded) => {
      const merged = { ...loaded };
      for (const field of changed) merged[field] = current[field];
      return merged;
    },
    finish: () => {
      if (ready) return;
      ready = true;
      failed = false;
      changed.clear();
      resolveReady?.();
      resolveReady = null;
    },
    waitUntilReady: () => readyPromise,
  };
}

/**
 * Serialize full-settings writes so an older snapshot can never finish after a
 * newer one and become the value loaded on the next launch.
 */
export function createSerialSettingsWriter<T>(
  writer: (settings: T) => Promise<void>,
): (settings: T) => Promise<void> {
  let tail = Promise.resolve();

  return (settings: T) => {
    const operation = tail.then(() => writer(settings));
    // Keep the internal chain usable after a failed write. The returned
    // operation still rejects so each caller can handle its own failure.
    tail = operation.catch(() => undefined);
    return operation;
  };
}

/** Revert rejected fields without overwriting edits made after that request. */
export function rollbackRejectedSettings<T extends object>(current: T, attempted: T, confirmed: T): T {
  const next = { ...current };
  for (const field of Object.keys(attempted) as Array<keyof T>) {
    if (Object.is(current[field], attempted[field])) next[field] = confirmed[field];
  }
  return next;
}
