// The Liquid Glass material model (R8).
//
// Two orthogonal controls, and the whole point of the round is that they never
// touch each other:
//
//   * the **window transparency** slider (`main_opacity` / `terminal_opacity`,
//     a 10-100 percentage) is READABILITY. It is the only input a native
//     window-alpha path may read, and at 100% the frame is near-opaque;
//   * the **glass step** (this file) is MATERIAL. It picks one of three
//     variants and nothing else.
//
// The step is expressed in CSS as `[data-glass]` on <html>, because switching
// it swaps a *set* of tokens (`--glass-step-*` in base.css) rather than a
// single value, and because the accessibility override blocks key off the same
// attribute. This module owns the vocabulary: the three ids, the human-facing
// labels' i18n keys, and the one normalization rule the persistence layer and
// the settings UI both need.
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
 * The i18n keys for the three steps, kept beside the ids so a new step cannot
 * be added in the UI without a label and a caption for it.
 */
export const GLASS_STEP_LABELS: Record<GlassStep, { label: string; description: string }> = {
  low: { label: "settings.glassStep.low", description: "settings.glassStep.lowHint" },
  mid: { label: "settings.glassStep.mid", description: "settings.glassStep.midHint" },
  high: { label: "settings.glassStep.high", description: "settings.glassStep.highHint" },
};

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
