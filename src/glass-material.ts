// The Liquid Glass material model (R8; unified into a single control by
// GLASS-UNIFY).
//
// The **material** is still two independent inputs, and they still never touch
// each other inside the stylesheet:
//
//   * the **window transparency** value (`main_opacity` / `terminal_opacity`,
//     a 10-100 percentage) is READABILITY. It is the only input a native
//     window-alpha path may read, and at 100% the frame is near-opaque;
//   * the **glass step** (this file) is MATERIAL. It picks one of three
//     variants and nothing else.
//
// GLASS-UNIFY collapses the R8 *UI-layer* split: the user judged the two knobs to
// be one perceived axis ("玻璃有多重"), so the settings panel now exposes a
// single five-stop **glass intensity** segmented control. Each stop writes one
// fixed `(glass_step, tint)` pair into the two fields above — the Rust struct,
// the migration and the CSS formulas are untouched. A stored pair that no stop
// wrote (an upgraded 94→47% + high file, a hand-edited JSON) is read back by
// `glassIntensityOf`, which lands it on the nearest stop by the composited
// tint (midpoints as boundaries).
//
// The step is expressed in CSS as `[data-glass]` on <html>, because switching
// it swaps a *set* of tokens (`--glass-step-*` in base.css) rather than a
// single value, and because the accessibility override blocks key off the same
// attribute. This module owns the vocabulary: the three step ids, the five
// intensity stops, and the normalization/reverse-lookup rules the persistence
// layer and the settings UI both need.
//
// The numeric truth lives in base.css, not here — the steps' blur, saturation,
// fill and dimming values are token overrides so a WebKitGTK build can never
// see a half-applied JavaScript mixture. Keeping the numbers in one place is
// what makes `tests/glass-material.test.ts` able to assert the variant bands
// and the monotonicity without re-deriving them.

/** HIG's two Liquid Glass variants plus a readability-max variant above them. */
export type GlassStep = "low" | "mid" | "high";

/** In display order: thinnest material first. */
export const GLASS_STEPS: readonly GlassStep[] = ["low", "mid", "high"] as const;

/**
 * An unknown value (older settings file, a future step this build does not
 * know, a hand-edited JSON) rests on the balanced Regular step rather than on
 * whichever variant happens to be first — a wrong-but-plausible material is a
 * better failure than an unintended Clear panel over a photo.
 */
export const normalizeGlassStep = (value: unknown): GlassStep =>
  value === "low" || value === "high" ? value : "mid";

/**
 * The step's numeric material, mirrored here as the *only* place JavaScript is
 * allowed to know it.
 *
 * `base.css` owns the numbers a surface inside the app document sees (its
 * `[data-glass]` blocks). A sandboxed plugin page is a separate document and
 * cannot read them, so the host has to hand the values across — this table is
 * that hand-off's source, kept next to the step ids so a new step cannot be
 * added without its fill/dim. `tests/plugin-pages.test.ts` asserts these
 * equal the `[data-glass]` blocks, so this mirror can never drift from the
 * stylesheet.
 */
export const GLASS_STEP_TOKENS: Record<GlassStep, { fill: number; dim: number }> = {
  low: { fill: 0.3, dim: 0.35 },
  mid: { fill: 0.68, dim: 0 },
  high: { fill: 0.8, dim: 0 },
};

/**
 * The maxed end of the transparency slider, mirrored for the same hand-off.
 * 0.98 rather than 1 on purpose: see the comment on `--glass-solid-top` in
 * base.css — a fully opaque frame drops the material's sheen and rim.
 */
export const GLASS_SOLID_TOP = 0.98;

/**
 * The three step tokens as a plain style bag, for injecting into the plugin
 * page's container.
 *
 * A sandboxed cross-origin page (and even the same-origin built-in one, whose
 * stylesheet lives in its own document) cannot read the host's `:root`, so
 * `clipboard/page.css` used to hardcode the step's fill and top. That pinned
 * the page to Regular forever. The host now hands the values across instead,
 * and this function is the single place they come from — `GLASS_STEP_TOKENS`,
 * not literals at the call site.
 */
export const glassStepStyle = (step: GlassStep): Record<string, string> => ({
  "--glass-step-fill": String(GLASS_STEP_TOKENS[step].fill),
  "--glass-step-dim": String(GLASS_STEP_TOKENS[step].dim),
  "--glass-solid-top": String(GLASS_SOLID_TOP),
});

// ── GLASS-UNIFY: one intensity control over the (step, tint) pair ──────────
//
// R8 exposed the material (a 3-way step) and the window transparency (a
// continuous slider) side by side. The user's verdict was that the two read as
// one axis — "效果强度和透明度…感觉不出来它们的独立" — so the settings panel now
// offers a single five-stop intensity control and the continuous slider is
// gone. Nothing below the UI changed: a stop is just a fixed `(glass_step,
// tint)` pair written into the same two fields, and the reverse lookup turns
// any stored pair back into the stop it is closest to.

/**
 * The window-transparency percentages are clamped to the same 10-100 band the
 * Rust side enforces (`normalize_window_opacity`), so a hand-edited file can
 * never render a 3%-opaque frame. The old `normalizeOpacity` also snapped to
 * four presets; with a discrete stop control there is nothing to snap, so the
 * clamp is all that is left of it.
 */
export const clampWindowOpacity = (value: number): number =>
  Math.round(Math.min(100, Math.max(10, Number.isFinite(value) ? value : 47)));

/**
 * The frame's composited tint — the number a person actually perceives as
 * "how much glass is there". It is the shipped CSS composition
 * (`--glass-tint-alpha` in base.css) evaluated from the same step table the
 * plugin page receives, so this is not a second model:
 *
 *   frame = fill + (solidTop - fill) · t
 *   tint  = 1 - (1 - dim · (1 - t)) · (1 - frame)
 *
 * `t` is the transparency fraction (0-1), `dim` the variant's dimming veil.
 */
export const glassEffectiveTint = (step: GlassStep, transparency: number): number => {
  const { fill, dim } = GLASS_STEP_TOKENS[step];
  const t = Math.min(1, Math.max(0, transparency));
  const frame = fill + (GLASS_SOLID_TOP - fill) * t;
  const veil = dim * (1 - t);
  return 1 - (1 - veil) * (1 - frame);
};

/** One of five stops, thinnest (1) to near-solid (5). */
export type GlassIntensity = 1 | 2 | 3 | 4 | 5;

/** In display order: thinnest material first. */
export const GLASS_INTENSITIES: readonly GlassIntensity[] = [1, 2, 3, 4, 5] as const;

/**
 * A stop's frozen material. `tint` is the window-transparency fraction written
 * to **both** opacity fields; `label` is the i18n key, kept beside the stop
 * so a sixth stop cannot be added without a label.
 *
 * The anchors were calibrated from the shipped material formula: stop 1 sits at
 * the thinnest end a person reads as "liquid glass" (the Clear variant with
 * just enough tint to keep the rim), stop 5 is the old slider at its 0.98 top
 * (a near-solid panel), and 2-4 are the intermediate composites. The pairs are
 * asserted in `tests/glass-intensity.test.ts`, so moving one turns a test red.
 */
export const GLASS_INTENSITY: Record<
  GlassIntensity,
  { step: GlassStep; tint: number; label: string }
> = {
  1: { step: "low", tint: 0.22, label: "settings.glassIntensity.1" },
  2: { step: "mid", tint: 0.4, label: "settings.glassIntensity.2" },
  3: { step: "high", tint: 0.58, label: "settings.glassIntensity.3" },
  4: { step: "high", tint: 0.78, label: "settings.glassIntensity.4" },
  5: { step: "high", tint: 0.98, label: "settings.glassIntensity.5" },
};

/**
 * The settings fields a stop writes. Both opacity values move together: the
 * control is one axis, so the launcher/settings frame and the terminal frame
 * stay in step. Percentages are rounded to the integers the Rust `u8` fields
 * hold.
 */
export const glassIntensitySettings = (
  level: GlassIntensity,
): { glass_step: GlassStep; main_opacity: number; terminal_opacity: number } => {
  const { step, tint } = GLASS_INTENSITY[level];
  const percent = Math.round(tint * 100);
  return { glass_step: step, main_opacity: percent, terminal_opacity: percent };
};

/**
 * The stop a composited tint belongs to, by nearest effective tint with the
 * midpoints between adjacent stops as boundaries. A tie (a stored value that
 * sits exactly on a midpoint) falls to the **lower** stop, which is the
 * conservative choice: a thinner panel keeps more of the material visible.
 */
export const glassIntensityForTint = (effective: number): GlassIntensity => {
  for (let i = 0; i < GLASS_INTENSITIES.length - 1; i += 1) {
    const current = GLASS_INTENSITY[GLASS_INTENSITIES[i]];
    const next = GLASS_INTENSITY[GLASS_INTENSITIES[i + 1]];
    const boundary =
      (glassEffectiveTint(current.step, current.tint) +
        glassEffectiveTint(next.step, next.tint)) /
      2;
    if (effective <= boundary) return GLASS_INTENSITIES[i];
  }
  return GLASS_INTENSITIES[GLASS_INTENSITIES.length - 1];
};

/**
 * The reverse lookup the settings panel uses to display a stored pair: any
 * `(step, transparency)` — including the 47%+high an upgraded pre-R8 file
 * migrates to — lands on exactly one stop.
 */
export const glassIntensityOf = (step: GlassStep, transparency: number): GlassIntensity =>
  glassIntensityForTint(glassEffectiveTint(step, transparency));
