// R7-13c · The user-facing interface-size vocabulary: five steps that write
// one CSS knob.
//
// R7-13b made `--ui-scale` the single truth every box dimension and type step
// is drawn from (`--u: calc(1px * var(--ui-scale))`, `--text-*` off the same
// knob). Leg 2 kept the value at `1`; R7-13c is the one that lets the user
// move it, so this module owns the *mapping* and nothing else:
//
//   tiny     -> 0.8     (-20%)
//   small    -> 0.9     (-10%; +12.5% over tiny)
//   default  -> 1
//   large    -> 1.1     (+10%)
//   larger   -> 1.25    (+13.6% over large, +25% over default)
//
// R43 · the user asked for two steps *below* the shipped default
// (「还可以再提供两个更小的值」). The pair is 0.9 and 0.8, not the 0.85/0.75
// the brief floated, for one measured reason: the type ladder bottoms out at
// `--text-caption`, 10px at the default step, and 0.75 would paint it at
// 7.5px — under the 8px floor where a caption stops being a word and starts
// being a smudge. 0.8 keeps it at 8px, the smallest legible step, and 0.9
// keeps the first downward move the same ~10% the upward `large` uses, so the
// ladder is symmetric: 0.8 → 0.9 → 1 → 1.1 → 1.25 (each adjacent pair a
// visible 10–13% apart, every one above the "reads as a wobble" threshold).
//
// Why these five and not others:
//   * **Raycast parity.** The reference ships Default / Large / Larger, and
//     its "Larger" sits at about a quarter again over the default. 1.25 is that
//     quarter; the round's whole point is that the setting reads the same.
//   * **Each step is visibly its own.** Below ~8% a step reads as a rendering
//     wobble rather than as a choice, and 10%/13.6% per increment clears that
//     with room; the two increments differ, so "Large" and "Larger" are not
//     the same step twice.
//   * **Nothing lands under a device pixel in a way that matters.** `--u` is
//     `calc(1px * scale)`, so 1.1 gives 1.1px units and 1.25 gives 1.25px.
//     Boxes carry odd multipliers (34, 42) and resolve to fractional logical
//     pixels (37.4, 42.5) — which the compositor rasterises correctly on the
//     device-pixel grid. The *hairlines* are deliberately not on `--u` (see
//     base.css), so no 1px rule turns into a 1.25px smear.
//
// The numeric truth for the *CSS* knob lives here and is applied by
// `applyUiScale`; the numeric truth for the *native* reset height lives in
// `src-tauri/src/commands/config.rs` (Rust cannot import TypeScript) and
// `tests/ui-scale-steps.test.ts` pins the two factor tables against each other
// — the same cross-language contract `window-contract.ts` uses for the width.
//
// The knob's *name* lives here too, so no React component ever spells the CSS
// custom property. `tests/ui-scale.test.ts` sweeps `src/` for the literal and
// allows exactly this module to carry it.

/** The five interface-size steps, smallest first. */
export type UiScale = "tiny" | "small" | "default" | "large" | "larger";

/** The steps in the order the settings picker paints them. */
export const UI_SCALE_STEPS: readonly UiScale[] = [
  "tiny",
  "small",
  "default",
  "large",
  "larger",
] as const;

/**
 * Step -> `--ui-scale` multiplier. This is the whole mapping; everything else
 * in the round derives from it.
 */
export const UI_SCALE_FACTORS: Record<UiScale, number> = {
  tiny: 0.8,
  small: 0.9,
  default: 1,
  large: 1.1,
  larger: 1.25,
};

/** The CSS custom property the root carries. Spelled once, here. */
export const UI_SCALE_CSS_VAR = "--ui-scale";

/**
 * Any stored value lands on a shipped step. A missing key (an older settings
 * file), an unknown string or a `null` from a hand-edited file all resolve to
 * `default`, which is the behaviour every earlier build shipped — a scale the
 * user never chose must never be applied.
 */
export const normalizeUiScale = (value: unknown): UiScale =>
  (UI_SCALE_STEPS as readonly unknown[]).includes(value) ? (value as UiScale) : "default";

/** The multiplier for a step (normalizing first, so the caller may pass a
 *  stored string straight through). */
export const uiScaleFactor = (step: unknown): number => UI_SCALE_FACTORS[normalizeUiScale(step)];

/**
 * Write the step onto a root element as `--ui-scale`.
 *
 * The knob is a *written* value and never a read one: every consumer is CSS
 * (`calc(var(--u) * N)` / the `--text-*` ladder), so the frontend must not
 * keep a second scaled copy of any measurement. Written as a string because a
 * custom property is a token, not a number — `String(1)` and `String(1.25)`
 * both parse as the fragment `calc(1px * …)` needs.
 */
export const applyUiScale = (
  root: { style: { setProperty(name: string, value: string): void } },
  step: unknown,
): void => {
  root.style.setProperty(UI_SCALE_CSS_VAR, String(uiScaleFactor(step)));
};
