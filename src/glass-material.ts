// The Liquid Glass material model (R8; re-axised by GLASS-REAXIS).
//
// The **material** is two independent inputs, and they never touch each other
// inside the stylesheet:
//
//   * the **window transparency** values (`main_opacity` / `terminal_opacity`,
//     10-100 percentages) are READABILITY. They are the only inputs a native
//     window-alpha path may read, and at 100% the frame is near-opaque;
//   * the **glass step** (this file) is the LIQUID-GLASS EFFECT: how much the
//     glass distorts/refracts (blur), how much it saturates the desktop behind
//     it, and how much lens quality the controls on top of it carry.
//
// GLASS-UNIFY (the previous round) collapsed the UI to a single five-stop
// control but — wrongly — spent the stops on the **tint** axis: each stop
// wrote a `(glass_step, tint)` pair into the two opacity fields. The user's
// verdict was that the stop must control the *effect* (扭曲程度、组件的透视
// 效果、控件透镜质感) while the app's background transparency is a separate,
// independently configured knob for the launcher/settings surface and the
// terminal. GLASS-REAXIS restores that split:
//
//   * the five stops now write **only** `glass_step` (no opacity field), and
//     each stop carries its own blur / saturation / control-lens level;
//   * the two opacity fields are again written by two independent sliders and
//     are *never* touched by the stop control;
//   * the reverse lookup is therefore opacity-independent: any stored
//     `glass_step` lands on exactly one stop no matter what the sliders hold.
//
// Because the stops must survive a restart as five distinct states and the
// Rust struct carries no extra field, the `glass_step` **value domain** is
// extended from three ids to five (`low`/`mid`/`high`/`deep`/`jelly`). The
// Rust field, its default, its migration and its "not an alpha" contract are
// unchanged; only the set of accepted string values grows. A GLASS-UNIFY file
// (which only ever wrote `low`/`mid`/`high`) reads back on stops 1-3 with its
// old tint preserved as the slider value — see the migration note below.
//
// The step is expressed in CSS as `[data-glass]` on <html>, because switching
// it swaps a *set* of tokens (`--glass-step-*` and `--glass-lens-*` in
// base.css) rather than a single value, and because the accessibility override
// blocks key off the same attribute. This module owns the vocabulary: the five
// step ids, the five effect stops, and the normalization/reverse-lookup rules
// the persistence layer and the settings UI both need.
//
// The numeric truth lives in base.css, not here — the steps' blur, saturation,
// fill and lens values are token overrides so a WebKitGTK build can never see
// a half-applied JavaScript mixture. Keeping the numbers in one place is what
// makes `tests/glass-intensity.test.ts` able to assert the effect table and
// the lens monotonicity without re-deriving them.

/**
 * The five Liquid Glass effect steps, thinnest first. `low`/`mid`/`high` are
 * the R8/HIG variants (Clear / Regular / Regular-max); `deep` and `jelly`
 * carry the heaviest saturation and the strongest control lens this app draws.
 */
export type GlassStep = "low" | "mid" | "high" | "deep" | "jelly";

/** In display order: thinnest effect first. */
export const GLASS_STEPS: readonly GlassStep[] = ["low", "mid", "high", "deep", "jelly"] as const;

/**
 * An unknown value (older settings file, a future step this build does not
 * know, a hand-edited JSON) rests on the balanced Regular step rather than on
 * whichever variant happens to be first — a wrong-but-plausible material is a
 * better failure than an unintended Clear panel over a photo.
 */
export const normalizeGlassStep = (value: unknown): GlassStep =>
  typeof value === "string" && (GLASS_STEPS as readonly string[]).includes(value)
    ? (value as GlassStep)
    : "mid";

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
 *
 * The three heaviest steps share the same fill floor: GLASS-REAXIS moved the
 * stop control onto the *effect* axis, so a stop changes blur, saturation and
 * lens quality — never the frame's tint. The fill floor is the step's own
 * material floor (HIG's Clear variant is genuinely thinner); the transparency
 * sliders are what move the frame between that floor and near-solid.
 */
export const GLASS_STEP_TOKENS: Record<GlassStep, { fill: number; dim: number }> = {
  low: { fill: 0.3, dim: 0.35 },
  mid: { fill: 0.68, dim: 0 },
  high: { fill: 0.8, dim: 0 },
  deep: { fill: 0.8, dim: 0 },
  jelly: { fill: 0.8, dim: 0 },
};

/**
 * The maxed end of the transparency slider, mirrored for the same hand-off.
 * 0.98 rather than 1 on purpose: see the comment on `--glass-solid-top` in
 * base.css — a fully opaque frame drops the material's sheen and rim.
 */
export const GLASS_SOLID_TOP = 0.98;

/**
 * The step tokens as a plain style bag, for injecting into the plugin page's
 * container.
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

// ── GLASS-REAXIS: one effect control over blur/saturation/lens ─────────────
//
// R8 exposed the material (a 3-way step) and the window transparency (a
// continuous slider) side by side. GLASS-UNIFY collapsed them into one control
// but spent the stops on the tint axis, which the user rejected: the stop must
// be the *liquid-glass effect* (distortion / lens quality), and the app's
// background transparency must be its own independent pair of sliders. This
// round restores the dual opacity sliders and re-points the five stops at the
// effect axis. Nothing below the UI changes shape: a stop still writes one
// `glass_step` id; it simply no longer writes either opacity field.

/**
 * The window-transparency percentages are clamped to the same 10-100 band the
 * Rust side enforces (`normalize_window_opacity`), so a hand-edited file can
 * never render a 3%-opaque frame.
 */
export const clampWindowOpacity = (value: number): number =>
  Math.round(Math.min(100, Math.max(10, Number.isFinite(value) ? value : 47)));

/** One of five effect stops, thinnest (1) to heaviest (5). */
export type GlassIntensity = 1 | 2 | 3 | 4 | 5;

/** In display order: thinnest effect first. */
export const GLASS_INTENSITIES: readonly GlassIntensity[] = [1, 2, 3, 4, 5] as const;

/** The control-lens quality level a stop drives: 1 (barely there) to 5 (the
 *  full liquid stack). It indexes the `--glass-lens-*` family in base.css. */
export type GlassLens = 1 | 2 | 3 | 4 | 5;

/**
 * A stop's frozen effect. `step` is the only settings field it writes; `blur`
 * and `saturate` are the material numbers the CSS blocks must carry (asserted
 * against base.css so they cannot drift); `lens` is the control-lens level;
 * `label` is the i18n key, kept beside the stop so a sixth stop cannot be
 * added without a label.
 *
 * The mapping is the user's table: Clear is a micro-lens, Balanced the resting
 * balance, Strong a visible lens, Deep a heavy one, Jelly the strongest liquid
 * this app draws. Blur tops out at the 28px budget (three stops share it) and
 * saturation climbs 135 → 165 → 180 → 190 → 200%.
 */
export const GLASS_INTENSITY: Record<
  GlassIntensity,
  { step: GlassStep; blur: number; saturate: number; lens: GlassLens; label: string }
> = {
  1: { step: "low", blur: 10, saturate: 135, lens: 1, label: "settings.glassIntensity.1" },
  2: { step: "mid", blur: 24, saturate: 165, lens: 2, label: "settings.glassIntensity.2" },
  3: { step: "high", blur: 28, saturate: 180, lens: 3, label: "settings.glassIntensity.3" },
  4: { step: "deep", blur: 28, saturate: 190, lens: 4, label: "settings.glassIntensity.4" },
  5: { step: "jelly", blur: 28, saturate: 200, lens: 5, label: "settings.glassIntensity.5" },
};

/**
 * The settings fields a stop writes. **Only** `glass_step`: the stop is the
 * effect axis, and the two opacity fields belong to their own sliders. This is
 * the whole correction GLASS-REAXIS makes to GLASS-UNIFY — the previous round
 * returned `{ glass_step, main_opacity, terminal_opacity }` here, which is
 * exactly the "档位做在 tint 轴上" mistake the user rejected.
 */
export const glassIntensitySettings = (level: GlassIntensity): { glass_step: GlassStep } => ({
  glass_step: GLASS_INTENSITY[level].step,
});

/**
 * The reverse lookup the settings panel uses to display a stored step: any
 * `glass_step` lands on exactly one stop, and the *transparency* is
 * deliberately ignored — the stop is the effect axis, so a stored pair such as
 * the GLASS-UNIFY `(high, 98%)` shows the `high` stop while the 98% stays in
 * the transparency slider the user can still nudge.
 *
 * `transparency` is accepted (not removed) so the opacity-independence is a
 * thing a test can *prove*: `glassIntensityOf(step, t)` must return the same
 * stop for every `t`. A later round that reintroduced a tint-based lookup
 * would have to read this argument, and the sweep in
 * `tests/glass-intensity.test.ts` would go red.
 */
export const glassIntensityOf = (step: GlassStep, _transparency?: number): GlassIntensity => {
  const index = GLASS_STEPS.indexOf(normalizeGlassStep(step));
  return (index >= 0 ? index + 1 : 2) as GlassIntensity;
};
