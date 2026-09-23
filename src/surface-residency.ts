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

/** R41 · the ceiling for a *custom* duration. The R35 ceiling of 30s made
 *  sense while the only choices were presets; the user asked for a free number
 *  (「再加一个自定义时间」) and a day is the longest span that still reads as
 *  "this page is still where I left it" rather than "the launcher is stuck".
 *  A stored value above it is clamped here and in Rust. */
export const RESIDENCY_CUSTOM_MAX_SECONDS = 86_400;

/**
 * R41 · the sentinel for "never": the surface survives any dismissal until the
 * user leaves it explicitly. It is the top of `u32` so the persisted field can
 * stay `u32` (the R35 compatibility promise) while the frontend only ever
 * compares against this constant. The Rust normalizer exempts it from the
 * custom ceiling — a value it did not understand would otherwise be clamped
 * down to a day and silently stop being "never".
 */
export const RESIDENCY_NEVER_SECONDS = 4_294_967_295;

/**
 * R35's ceiling is retained as the *preset* ceiling: the last preset step. It
 * is no longer the domain's maximum (R41 added {@link RESIDENCY_CUSTOM_MAX_SECONDS}
 * and {@link RESIDENCY_NEVER_SECONDS}), but keeping the name and the value
 * pins the preset table and the old tests against drift.
 */
export const RESIDENCY_MAX_SECONDS = 30;

/**
 * The shipped duration. Ten seconds is long enough to cover the loop the
 * report describes — the panel hides on blur, the user turns back, the
 * summon must land where they left — and short enough that a summon a
 * minute later is honestly a fresh launcher. It is deliberately not `0`:
 * the report asked for the behaviour, not for a switch that ships off.
 */
export const DEFAULT_RESIDENCY_SECONDS = 10;

/** R41 · the preset steps the select offers, in seconds. The R35 set was
 *  0/5/10/15/20/30; the user asked for a longer, rounder ladder and a custom
 *  entry, so the step below 10 (5) and the odd 15 are gone and 60/120 replace
 *  them — every preset is a value a person says out loud. `0` (off) and
 *  "never" are separate select options, not members of this list. */
export const RESIDENCY_PRESET_SECONDS = [10, 20, 30, 60, 120] as const;

/**
 * Clamp a stored/typed value onto the residency domain. The domain is now
 * "`0`, any whole second in `[1, {@link RESIDENCY_CUSTOM_MAX_SECONDS}]`, or
 * {@link RESIDENCY_NEVER_SECONDS}". A missing or non-numeric value lands on
 * the shipped default rather than on `0`, so a hand-edited settings file does
 * not silently disable the window; a negative is `0` (off), and a value above
 * the custom ceiling is the ceiling. The sentinel passes through untouched.
 */
export function normalizeResidencySeconds(value: unknown): number {
  // `null` / `undefined` / an empty string are *missing*, not zero: only a
  // real numeric `0` means "off". (`Number(null)` is `0`, which is why this
  // check is explicit rather than folded into the coercion below.)
  if (value === null || value === undefined || value === "") {
    return DEFAULT_RESIDENCY_SECONDS;
  }
  const seconds = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(seconds)) return DEFAULT_RESIDENCY_SECONDS;
  if (seconds === RESIDENCY_NEVER_SECONDS) return RESIDENCY_NEVER_SECONDS;
  return Math.round(
    Math.min(RESIDENCY_CUSTOM_MAX_SECONDS, Math.max(RESIDENCY_MIN_SECONDS, seconds)),
  );
}

/**
 * R41 · the select's value for a stored duration: `"off"`, a preset's own
 * number as a string, `"never"`, or `"custom"` for anything else (a custom
 * value is not one of the named options, so the select falls back to the
 * custom entry and the inline number field carries the real number). Pure so
 * the node suite can pin every branch without a DOM.
 */
export function residencySelectValue(seconds: number): string {
  const value = normalizeResidencySeconds(seconds);
  if (value === RESIDENCY_MIN_SECONDS) return "off";
  if (value === RESIDENCY_NEVER_SECONDS) return "never";
  if ((RESIDENCY_PRESET_SECONDS as readonly number[]).includes(value)) {
    return String(value);
  }
  return "custom";
}

/**
 * R41 · the number a select choice stands for. `null` means "this choice
 * carries no number" — the custom entry, whose real value lives in the inline
 * field and is committed by the user. Any other choice resolves to a concrete
 * stored value, so the caller can write it straight through.
 */
export function residencyFromSelect(choice: string): number | null {
  if (choice === "off") return RESIDENCY_MIN_SECONDS;
  if (choice === "never") return RESIDENCY_NEVER_SECONDS;
  if (choice === "custom") return null;
  const numeric = Number(choice);
  if (!Number.isFinite(numeric)) return null;
  return normalizeResidencySeconds(numeric);
}

/**
 * R41 · whether a stored value is the sentinel. Kept as a named predicate so
 * the "never" branch is a single decision in both the clock and the select.
 */
export const residencyNever = (seconds: number): boolean =>
  seconds === RESIDENCY_NEVER_SECONDS;

/**
 * R41 · the number the inline custom field opens on, given the stored value.
 * A value that is already a plain custom duration (a positive number that is
 * not a preset and not the sentinel) seeds the field with itself, so opening
 * "custom" on an existing custom value does not silently reset it. Everything
 * else — off, a preset, "never", a hand-edited out-of-range number — seeds the
 * shipped default, which is the least surprising number to edit from.
 */
export function residencyCustomSeedSeconds(seconds: number): number {
  const value = normalizeResidencySeconds(seconds);
  if (value === RESIDENCY_MIN_SECONDS) return DEFAULT_RESIDENCY_SECONDS;
  if (value === RESIDENCY_NEVER_SECONDS) return DEFAULT_RESIDENCY_SECONDS;
  if ((RESIDENCY_PRESET_SECONDS as readonly number[]).includes(value)) {
    return DEFAULT_RESIDENCY_SECONDS;
  }
  return value;
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
 *
 * R41 · {@link RESIDENCY_NEVER_SECONDS} yields a clock whose deadline is
 * `+Infinity`: it never lapses, so `holds` is true for every `now`. The
 * explicit exits (Esc / Cmd+W, the close button, running a result) are still
 * ungated — the R35 red line — because they never consult the clock at all.
 */
export function startResidency(
  surface: ResidencySurface,
  now: number,
  seconds: number,
): ResidencyClock | null {
  if (seconds === RESIDENCY_NEVER_SECONDS) {
    return { surface, expiresAt: Number.POSITIVE_INFINITY };
  }
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
