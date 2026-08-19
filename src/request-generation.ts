/**
 * Monotonic request generations for async UI work.
 *
 * A response is allowed to update state only while its generation is still
 * current. Keeping the comparison in one tiny helper makes race handling easy
 * to test without mounting the Tauri-backed component.
 */
export const beginRequest = (current: { current: number }): number => {
  current.current += 1;
  return current.current;
};

export const isCurrentRequest = (current: { current: number }, generation: number): boolean =>
  current.current === generation;
