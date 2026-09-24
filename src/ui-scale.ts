// R7-13c · The user-facing interface-size vocabulary: four steps that write
// one CSS knob.
//
// R7-13b made `--ui-scale` the single truth every box dimension and type step
// is drawn from (`--u: calc(1px * var(--ui-scale))`, `--text-*` off the same
// knob). Leg 2 kept the value at `1`; R7-13c is the one that lets the user
// move it, so this module owns the *mapping* and nothing else:
//
//   tiny     -> 0.8     (-20%)
//   small    -> 0.9     (-10%; +12.5% over tiny)  · shipped default (R47)
//   default  -> 1
//   large    -> 1.1     (+10%)
//
// R47 · the user asked for two things (「当前的界面大小，感觉当前的小好像适合设置成
// 默认。然后把"更大"去掉」): move the shipped default down to `small`, and
// retire `larger` (1.25). The ladder is now four steps. `default` (1) stays a
// *step* — it is the size every build through R46 shipped, so a file that
// explicitly persisted it keeps it — but it is no longer the *default*.
//
// Why `small` and not a new 0.85/0.75: unchanged from R43. The type ladder
// bottoms out at `--text-caption`, 10px at the default step; 0.75 would paint
// it at 7.5px — under the 8px floor where a caption stops being a word and
// starts being a smudge. 0.8 keeps it at 8px, the smallest legible step, and
// 0.9 keeps the first move the same ~10% the upward `large` uses. The four
// steps are 0.8 → 0.9 → 1 → 1.1, each adjacent pair a visible 10–12.5% apart,
// every one above the "reads as a wobble" threshold.
//
// Why these four and not others:
//   * **Each step is visibly its own.** Below ~8% a step reads as a rendering
//     wobble rather than as a choice, and 10%/12.5% per increment clears that
//     with room.
//   * **Nothing lands under a device pixel in a way that matters.** `--u` is
//     `calc(1px * scale)`, so 1.1 gives 1.1px units and 0.9 gives 0.9px. Boxes
//     carry odd multipliers (34, 42) and resolve to fractional logical pixels
//     (37.4, 42.5) — which the compositor rasterises correctly on the
//     device-pixel grid. The *hairlines* are deliberately not on `--u` (see
//     base.css), so no 1px rule turns into a 1.1px smear.
//
// Legacy `larger`: a file that persisted it is mapped to `large` (1.1) rather
// than dropped to the new default, so an upgrading user who deliberately chose
// the biggest size keeps the closest surviving one instead of being shrunk two
// steps. See `UI_SCALE_LEGACY`.
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

/** The four interface-size steps, smallest first. */
export type UiScale = "tiny" | "small" | "default" | "large";

/** The steps in the order the settings picker paints them. */
export const UI_SCALE_STEPS: readonly UiScale[] = [
  "tiny",
  "small",
  "default",
  "large",
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
};

/**
 * R47 · The shipped step. A missing `ui_scale` key (a file written before
 * R7-13c) or an unrecognised string lands here, which — since Rust serialises
 * every field — is the only persistence the frontend and backend can both
 * treat as "the user never chose". Must match `DEFAULT_UI_SCALE` in
 * `src-tauri/src/commands/config.rs`, pinned by `tests/ui-scale-steps.test.ts`.
 */
export const DEFAULT_UI_SCALE: UiScale = "small";

/**
 * R47 · Steps this build no longer ships, and the surviving step each maps to.
 * `larger` was retired at the user's request; a file that persisted it lands on
 * `large` (1.1), the closest step that still exists, so a deliberate choice of
 * the biggest size is honoured rather than collapsed to the new default.
 */
export const UI_SCALE_LEGACY: Readonly<Partial<Record<string, UiScale>>> = {
  larger: "large",
};

/** The CSS custom property the root carries. Spelled once, here. */
export const UI_SCALE_CSS_VAR = "--ui-scale";

/**
 * Any stored value lands on a shipped step, in this order:
 *
 *   1. a shipped step passes through untouched — including `default` (1), so a
 *      file that explicitly persisted the pre-R47 default keeps that size;
 *   2. a retired step ([`UI_SCALE_LEGACY`]) maps to its nearest survivor;
 *   3. anything else — a missing key, an unknown string, a `null` from a
 *      hand-edited file — resolves to [`DEFAULT_UI_SCALE`] (`small`).
 */
export const normalizeUiScale = (value: unknown): UiScale => {
  if ((UI_SCALE_STEPS as readonly unknown[]).includes(value)) return value as UiScale;
  const legacy = typeof value === "string" ? UI_SCALE_LEGACY[value] : undefined;
  return legacy ?? DEFAULT_UI_SCALE;
};

/** The multiplier for a step (normalizing first, so the caller may pass a
 *  stored string straight through). */
export const uiScaleFactor = (step: unknown): number => UI_SCALE_FACTORS[normalizeUiScale(step)];

/**
 * Write the step onto a root element as `--ui-scale`.
 *
 * The knob is a *written* value and never a read one: every consumer is CSS
 * (`calc(var(--u) * N)` / the `--text-*` ladder), so the frontend must not
 * keep a second scaled copy of any measurement. Written as a string because a
 * custom property is a token, not a number — `String(1)` and `String(0.9)`
 * both parse as the fragment `calc(1px * …)` needs.
 */
export const applyUiScale = (
  root: { style: { setProperty(name: string, value: string): void } },
  step: unknown,
): void => {
  root.style.setProperty(UI_SCALE_CSS_VAR, String(uiScaleFactor(step)));
};
