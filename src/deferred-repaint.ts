// Trailing-edge repaint scheduler (GLASS-CLIP).
//
// The window-transparency sliders used to do two very different things on
// every single tick of a drag:
//
//   1. write the two CSS custom properties — the *visual feedback*, which has
//      to be immediate or the glass lags the thumb; and
//   2. re-read the palette (`renderer.updateTheme()`, which calls
//      `getComputedStyle`) and repaint the whole terminal canvas
//      (`renderer.draw(...)`).
//
// Step 2 is the expensive one, and it does not depend on the slider at all:
// the canvas paints its background at the frame alpha and the *window* alpha
// is applied by the compositor to the CSS surface, so a 47%→48% nudge changes
// no canvas pixel. Running a full terminal repaint per tick starved the range
// input's own event handling on the main thread — the user's 「透明度的滑杆拖动
// 会中断」.
//
// This module owns the scheduling half of the split: the CSS variables stay
// per-tick, the repaint is coalesced to one trailing call. It is a plain
// factory with injected timers rather than a hook so the node suite can drive
// ten slider ticks through it and count the renders (see
// `tests/glass-clip.test.ts`), which is the falsifiable form of "the repaint
// left the per-tick path".
//
// `flush` exists because a coalesced repaint is a *deferred* one: a surface
// that is about to be hidden or torn down has to land it first, or the last
// frame the user saw keeps the previous palette.

/**
 * How long the repaint waits after the last opacity change. Long enough that a
 * continuous drag fires exactly one trailing repaint, short enough that a
 * keyboard nudge or a single click still repaints within a frame or two of the
 * user letting go. It is deliberately *shorter* than the settings-persistence
 * debounce (`SETTINGS_DEBOUNCE_MS`, 180ms): the two are independent — one
 * guards the canvas, the other guards the disk — and the canvas must not be
 * the slower of the two.
 */
export const TERMINAL_REPAINT_DEBOUNCE_MS = 140;

/** Timer seam, so tests can run on fake time. Mirrors `window.setTimeout`'s
 *  numeric-handle contract, which is what the app actually uses. */
export type RepaintTimers = {
  setTimeout: (fn: () => void, ms: number) => number;
  clearTimeout: (id: number) => void;
};

export type DeferredRepaint = {
  /** Request a repaint after the debounce window; restarts the window. */
  schedule: () => void;
  /** Run a pending repaint *now*, synchronously. A no-op when none is pending. */
  flush: () => void;
  /** Drop a pending repaint without running it. */
  cancel: () => void;
  /** Whether a repaint is waiting. Exposed for tests and for callers that
   *  need to avoid scheduling a second, redundant one. */
  isPending: () => boolean;
};

const defaultTimers: RepaintTimers = {
  setTimeout: (fn, ms) => window.setTimeout(fn, ms),
  clearTimeout: (id) => window.clearTimeout(id),
};

/**
 * Build a trailing-edge scheduler around `run`.
 *
 * Invariants the tests pin:
 *   * N calls to `schedule()` inside one window produce **one** `run()`;
 *   * `flush()` produces it immediately and clears the pending flag;
 *   * `cancel()` produces none;
 *   * a `run()` that throws still clears the pending flag — the next drag must
 *     not be silently swallowed by a stuck timer.
 */
export const createDeferredRepaint = (
  run: () => void,
  delayMs: number = TERMINAL_REPAINT_DEBOUNCE_MS,
  timers: RepaintTimers = defaultTimers,
): DeferredRepaint => {
  let timer: number | null = null;
  const cancel = () => {
    if (timer === null) return;
    timers.clearTimeout(timer);
    timer = null;
  };
  const fire = () => {
    timer = null;
    run();
  };
  return {
    schedule: () => {
      cancel();
      timer = timers.setTimeout(fire, delayMs);
    },
    flush: () => {
      if (timer === null) return;
      cancel();
      fire();
    },
    cancel,
    isPending: () => timer !== null,
  };
};
