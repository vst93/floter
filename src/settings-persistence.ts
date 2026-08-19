export type SettingsHydration<T extends object> = {
  isReady: () => boolean;
  markChanged: (field: keyof T) => void;
  mergeLoaded: (current: T, loaded: T) => T;
  finish: () => void;
  waitUntilReady: () => Promise<void>;
};

/**
 * Coordinate the one-time settings read with edits made while it is in flight.
 * Edited fields win during hydration, while writes wait until the remaining
 * fields have been populated from disk.
 */
export function createSettingsHydration<T extends object>(): SettingsHydration<T> {
  const changed = new Set<keyof T>();
  let ready = false;
  let resolveReady: (() => void) | null = null;
  const readyPromise = new Promise<void>((resolve) => {
    resolveReady = resolve;
  });

  return {
    isReady: () => ready,
    markChanged: (field) => {
      if (!ready) changed.add(field);
    },
    mergeLoaded: (current, loaded) => {
      const merged = { ...loaded };
      for (const field of changed) merged[field] = current[field];
      return merged;
    },
    finish: () => {
      if (ready) return;
      ready = true;
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
