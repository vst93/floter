// R80 · `wrapIndex` — the wrap-cycle rule, declared once.
//
// Six places move a cursor one step through a list and wrap at both ends:
// the launcher's selection cycle, the settings sidebar's ↑/↓, the overflow
// menu's items, a radio group's arrows, the plugin filter axes' Tab cycle and
// the file-drop action switcher. Five of them wrote the rule by hand; R40's
// `cyclePluginFilter` was the one that had been extracted. They are the same
// arithmetic:
//
//   ((index + delta) % length + length) % length
//
// The double modulo is the point: a single `%` returns a negative index for a
// step off the front (`-1 % 3 === -1`), so the extra `+ length` before the
// second `%` is what makes the cycle wrap instead of fall off. The file-drop
// switcher also *normalizes* its input — a negative, fractional or non-finite
// index is folded into `[0, length)` before the step — and that is the
// reference behaviour this primitive carries, because a raw index is what a
// caller that has not yet found its cursor can hand it.
//
// It is a pure module (no React, no Tauri) so the node suite can pin the rule
// without a DOM.

/**
 * Move one step through a list of `length` positions, wrapping at both ends.
 *
 * `index` is normalized first: a non-finite value becomes `0` and a finite one
 * is truncated toward zero, so a raw index (or one that has drifted) is folded
 * into `[0, length)` before `delta` is added. `delta` is likewise truncated and
 * a non-finite value treated as `0`. The result is always in `[0, length)` for
 * a positive `length`.
 *
 * This is exactly the file-drop switcher's rule (`clampActionIndex` is
 * `wrapIndex(index, 0, length)`; `nextFileActionIndex` is
 * `wrapIndex(index, ±1, length)`), and it agrees with every hand-written
 * `(index + delta + length) % length` cycle the callers already had for the
 * indices they pass.
 */
export const wrapIndex = (index: number, delta: number, length: number): number => {
  const base = Number.isFinite(index) ? Math.trunc(index) : 0;
  const step = Number.isFinite(delta) ? Math.trunc(delta) : 0;
  return ((base + step) % length + length) % length;
};
