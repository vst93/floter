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
