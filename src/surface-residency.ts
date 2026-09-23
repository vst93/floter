// R35 · the page-residency window.
//
// The user's report, verbatim: 「同时应用应该增加逻辑，所在插件和页面应该维持一段
// 时间，这个时间应用配置页面可以设置。在保持的时间内不要退回到搜索框。这个逻辑适
// 用于所有插件和集成、报错终端页面」.
//
// A *surface* here is anything that is not the ordinary search page: a plugin
// mode (browser / clipboard), the plugin configuration overlay, the settings
// panel, and the terminal view. Before R35 a summon after a blur reset the
// collapsed surface unconditionally — `setQueryExitingPlugin("")` cleared the
// plugin mode — so a user who glanced at another window came back to the search
// box and had lost the page they were on. Settings and terminal happened to
// survive (the reveal handler special-cases them, and the native
// `terminal_mode` flag outlives a hide), but the rule was a coincidence of two
// branches rather than a policy.
//
// This module makes it a policy: entering a surface starts a clock; while the
// clock holds, the *automatic* return paths stand down. Explicit user gestures
// — Esc / Cmd+W, the close button, running a result — are never gated: the
// clock only answers "may the app throw this surface away on its own?".
//
// The module is pure (no React, no DOM, no timers) so the node suite can pin
// the arithmetic and the surface table. The clock itself is held by
// `useSurfaceResidency` in `src/hooks/useSurfaceResidency.ts`.

/** The surfaces whose residency is tracked. `plugin-mode` is the browser or
 *  clipboard list; `plugin-config` is the generic configuration overlay that
 *  opens on top of it; `settings` and `terminal` are the two full surfaces. */
export type ResidencySurface = "plugin-mode" | "plugin-config" | "settings" | "terminal";

/** `0` disables the window: the pre-R35 behaviour, where a summon always
 *  returns to the search box. */
export const RESIDENCY_MIN_SECONDS = 0;

/** The ceiling. A residency longer than this stops being a "recently used"
 *  window and starts being a mode the user cannot leave by dismissing — the
 *  explicit gestures still work, but the surface would outlive the
 *  interaction it belonged to. */
export const RESIDENCY_MAX_SECONDS = 30;

/**
 * The shipped duration. Ten seconds is long enough to cover the loop the
 * report describes — the panel hides on blur, the user turns back, the
 * summon must land where they left — and short enough that a summon a
 * minute later is honestly a fresh launcher. It is deliberately not `0`:
 * the report asked for the behaviour, not for a switch that ships off.
 */
export const DEFAULT_RESIDENCY_SECONDS = 10;

/** Clamp a stored/typed value onto the `[0, 30]` integer domain. A missing or
 *  non-numeric value lands on the shipped default rather than on `0`, so a
 *  hand-edited settings file does not silently disable the window. */
export function normalizeResidencySeconds(value: unknown): number {
  // `null` / `undefined` / an empty string are *missing*, not zero: only a
  // real numeric `0` means "off". (`Number(null)` is `0`, which is why this
  // check is explicit rather than folded into the coercion below.)
  if (value === null || value === undefined || value === "") {
    return DEFAULT_RESIDENCY_SECONDS;
  }
  const seconds = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(seconds)) return DEFAULT_RESIDENCY_SECONDS;
  return Math.round(
    Math.min(RESIDENCY_MAX_SECONDS, Math.max(RESIDENCY_MIN_SECONDS, seconds)),
  );
}

/** A started clock: which surface, and the wall-clock instant it lapses. */
export type ResidencyClock = {
  readonly surface: ResidencySurface;
  /** Milliseconds since the epoch. */
  readonly expiresAt: number;
};

/**
 * Start a clock for `surface` at `now`. `0` (or a negative duration) is the
 * disabled state and yields no clock — callers keep that `null` rather than a
 * clock that already lapsed, so `holds` is false for the same reason whether
 * the window is off or simply over.
 */
export function startResidency(
  surface: ResidencySurface,
  now: number,
  seconds: number,
): ResidencyClock | null {
  if (!(seconds > 0)) return null;
  return { surface, expiresAt: now + seconds * 1000 };
}

/** Whether the clock is still inside its window at `now`. A lapsed clock is
 *  not cleared by this predicate; the caller decides when to drop it. */
export function residencyHolds(clock: ResidencyClock | null, now: number): boolean {
  return clock !== null && now < clock.expiresAt;
}

/**
 * The surface the app is currently *on*, derived from the three pieces of
 * state that define it. `null` means the ordinary search page — the one place
 * a summon is allowed to reset to.
 *
 * The order matters: the configuration overlay sits on top of the plugin mode
 * and is the more specific surface, so it wins while it is open. Settings and
 * terminal are full-surface modes and are read first for the same reason.
 */
export function residencySurface(state: {
  mode: "collapsed" | "terminal" | "settings";
  /** The active plugin mode, if any. Only its presence is read. */
  pluginMode: unknown;
  pluginConfigOpen: boolean;
}): ResidencySurface | null {
  if (state.mode === "settings") return "settings";
  if (state.mode === "terminal") return "terminal";
  if (state.mode !== "collapsed") return null;
  if (state.pluginConfigOpen) return "plugin-config";
  if (state.pluginMode) return "plugin-mode";
  return null;
}
