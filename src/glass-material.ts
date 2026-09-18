// The Liquid Glass material model (R8; re-axised by GLASS-REAXIS; collapsed to
// three stops by GLASS-3STOP).
//
// The **material** is two independent inputs, and they never touch each other
// inside the stylesheet:
//
//   * the **window transparency** values (`main_opacity` / `terminal_opacity`,
//     10-100 percentages) are READABILITY. They are the only inputs a native
//     window-alpha path may read, and at 100% the frame is near-opaque;
//   * the **glass step** (this file) is the LIQUID-GLASS EFFECT: how much the
//     glass distorts/refracts (blur), how much it saturates the desktop behind
//     it, how much haze (dimming) it lays over bright content, and how much
//     lens quality the controls on top of it carry.
//
// GLASS-UNIFY spent the stops on the **tint** axis (each stop wrote a
// `(glass_step, tint)` pair into the two opacity fields). GLASS-REAXIS undid
// that and pointed the stops at the effect axis. GLASS-3STOP is the user's
// follow-up: the five stops were too close together (「每档差异有点小」) and
// the step's fill floor made the transparency slider almost inert
// (「除了 10%，其他档位都基本看不到透明」). So this round:
//
//   * collapses five stops to **three** and pulls them far apart, walking the
//     user's own arc: frosted (磨砂) → regular (标准液态) → liquid (液态拉满,
//     the old jelly's material);
//   * removes the step's `fill` floor from the frame alpha entirely. The
//     transparency slider is the **only** alpha truth again: `--glass-frame-alpha`
//     is `min(solid_top, max(frame_floor, opacity))`, so 10% really is 10%.
//     A step changes blur / saturation / lens / haze — never the frame's
//     thickness. `--glass-step-dim` survives as the *haze* layer that keeps
//     thin frosted glass readable over bright content; it is composited into
//     `--glass-tint-alpha`, not into the frame alpha.
//
// Because the stops must survive a restart as three distinct states and the
// Rust struct carries no extra field, the `glass_step` **value domain** is
// now three ids (`frosted`/`regular`/`liquid`). The Rust field, its default
// and its "not an alpha" contract are unchanged; only the set of accepted
// string values changed. A pre-GLASS-3STOP file (`low`/`mid`/`high`/`deep`/
// `jelly`) is migrated on read — see the legacy map below and its Rust twin.
//
// The step is expressed in CSS as `[data-glass]` on <html>, because switching
// it swaps a *set* of tokens (`--glass-step-*` and `--glass-lens-*` in
// base.css) rather than a single value, and because the accessibility override
// blocks key off the same attribute. This module owns the vocabulary: the
// three step ids, the three effect stops, and the normalization/reverse-lookup
// rules the persistence layer and the settings UI both need.
//
// The numeric truth lives in base.css, not here — the steps' blur, saturation,
// haze and lens values are token overrides so a WebKitGTK build can never see
// a half-applied JavaScript mixture. Keeping the numbers in one place is what
// makes `tests/glass-intensity.test.ts` able to assert the effect table and
// the lens monotonicity without re-deriving them.

/**
 * The three Liquid Glass effect steps, thinnest first. `frosted` is the
 * traditional frosted-glass end (small blur, low saturation, no refraction
 * cue), `regular` is Apple's Liquid Glass resting balance, and `liquid` is the
 * heaviest refraction + control-lens this app draws.
 */
export type GlassStep = "frosted" | "regular" | "liquid";

/** In display order: thinnest effect first. */
export const GLASS_STEPS: readonly GlassStep[] = ["frosted", "regular", "liquid"] as const;

/**
 * The pre-GLASS-3STOP five-stop vocabulary, and the three-stop step each id
 * becomes. The migration is value-based (the ids no longer overlap, so the
 * stored string alone decides): the two thin stops collapse onto `frosted`
 * and `regular`, and every heavy stop collapses onto `liquid` — an upgrading
 * user who was on `deep` or `jelly` must not be demoted to the balanced
 * material. The Rust loader carries the identical table; a file is migrated
 * there on read, and this map is the frontend's own safety net (a bridge
 * message or a mock that still speaks the old vocabulary).
 */
const LEGACY_GLASS_STEPS: Record<string, GlassStep> = {
  low: "frosted",
  mid: "regular",
  high: "liquid",
  deep: "liquid",
  jelly: "liquid",
};

/**
 * An unknown value (older settings file, a future step this build does not
 * know, a hand-edited JSON) rests on the balanced Regular step rather than on
 * whichever variant happens to be first — a wrong-but-plausible material is a
 * better failure than an unintended frosted panel over a photo. A legacy id is
 * migrated rather than treated as unknown.
 */
export const normalizeGlassStep = (value: unknown): GlassStep => {
  if (typeof value !== "string") return "regular";
  const step = value.trim().toLowerCase();
  if ((GLASS_STEPS as readonly string[]).includes(step)) return step as GlassStep;
  return LEGACY_GLASS_STEPS[step] ?? "regular";
};

/**
 * The step's numeric material, mirrored here as the *only* place JavaScript is
 * allowed to know it.
 *
 * `base.css` owns the numbers a surface inside the app document sees (its
 * `[data-glass]` blocks). A sandboxed plugin page is a separate document and
 * cannot read them, so the host has to hand the values across — this table is
 * that hand-off's source, kept next to the step ids so a new step cannot be
 * added without its haze. `tests/plugin-pages.test.ts` asserts these equal the
 * `[data-glass]` blocks, so this mirror can never drift from the stylesheet.
 *
 * `dim` is the step's **haze** layer: the frosted end carries a strong veil so
 * thin, lightly-blurred glass stays readable over bright content, and the
 * liquid end carries none because its heavy blur and saturation already do the
 * work. It is composited *under* the frame in `--glass-tint-alpha`; it is
 * deliberately **not** a fill floor, because the transparency slider is the
 * frame's only alpha truth.
 */
export const GLASS_STEP_TOKENS: Record<GlassStep, { dim: number }> = {
  frosted: { dim: 0.6 },
  regular: { dim: 0.5 },
  liquid: { dim: 0.2 },
};

/**
 * The maxed end of the transparency slider, mirrored for the same hand-off.
 * 0.98 rather than 1 on purpose: see the comment on `--glass-solid-top` in
 * base.css — a fully opaque frame drops the material's sheen and rim.
 */
export const GLASS_SOLID_TOP = 0.98;

/**
 * The frame alpha's floor, mirrored for the same hand-off. It is 0 in normal
 * use — the transparency slider is the only alpha truth — and only the
 * `prefers-contrast: more` override raises it in the host document. The plugin
 * page is a separate document and cannot see that media query, so it gets the
 * same 0 the host ships by default.
 */
export const GLASS_FRAME_FLOOR = 0;

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
  "--glass-step-dim": String(GLASS_STEP_TOKENS[step].dim),
  "--glass-solid-top": String(GLASS_SOLID_TOP),
  "--glass-frame-floor": String(GLASS_FRAME_FLOOR),
});

// ── GLASS-REAXIS: one effect control over blur/saturation/lens ─────────────
//
// R8 exposed the material (a 3-way step) and the window transparency (a
// continuous slider) side by side. GLASS-UNIFY collapsed them into one control
// but spent the stops on the tint axis, which the user rejected: the stop must
// be the *liquid-glass effect* (distortion / lens quality), and the app's
// background transparency must be its own independent pair of sliders. This
// round restores the dual opacity sliders and re-points the three stops at the
// effect axis. Nothing below the UI changes shape: a stop still writes one
// `glass_step` id; it simply no longer writes either opacity field.

/**
 * The window-transparency percentages are clamped to the same 10-100 band the
 * Rust side enforces (`normalize_window_opacity`), so a hand-edited file can
 * never render a 3%-opaque frame.
 */
export const clampWindowOpacity = (value: number): number =>
  Math.round(Math.min(100, Math.max(10, Number.isFinite(value) ? value : 47)));

/** One of three effect stops, thinnest (1) to heaviest (3). */
export type GlassIntensity = 1 | 2 | 3;

/** In display order: thinnest effect first. */
export const GLASS_INTENSITIES: readonly GlassIntensity[] = [1, 2, 3] as const;

/** The control-lens quality level a stop drives: 1 (barely there) to 3 (the
 *  full liquid stack). It indexes the `--glass-lens-*` family in base.css. */
export type GlassLens = 1 | 2 | 3;

/**
 * A stop's frozen effect. `step` is the only settings field it writes; `blur`
 * and `saturate` are the material numbers the CSS blocks must carry (asserted
 * against base.css so they cannot drift); `lens` is the control-lens level;
 * `label` is the i18n key, kept beside the stop so a fourth stop cannot be
 * added without a label.
 *
 * The mapping is the user's arc: 磨砂玻璃 → 苹果液态玻璃 → 液态拉满. The three
 * stops are deliberately far apart — blur 10 / 22 / 28px, saturation
 * 130 / 170 / 200% — so a step change is visible at a glance rather than a
 * subtle tint shift.
 */
export const GLASS_INTENSITY: Record<
  GlassIntensity,
  { step: GlassStep; blur: number; saturate: number; lens: GlassLens; label: string }
> = {
  1: { step: "frosted", blur: 10, saturate: 130, lens: 1, label: "settings.glassIntensity.1" },
  2: { step: "regular", blur: 22, saturate: 170, lens: 2, label: "settings.glassIntensity.2" },
  3: { step: "liquid", blur: 28, saturate: 200, lens: 3, label: "settings.glassIntensity.3" },
};

/**
 * The settings fields a stop writes. **Only** `glass_step`: the stop is the
 * effect axis, and the two opacity fields belong to their own sliders. This is
 * the whole correction GLASS-REAXIS makes to GLASS-UNIFY — the previous round
 * returned `{ glass_step, main_opacity, terminal_opacity }` here, which is
 * exactly the "档位做在 tint 轴上" mistake the user rejected. GLASS-3STOP keeps
 * that contract: the step never writes an alpha.
 */
export const glassIntensitySettings = (level: GlassIntensity): { glass_step: GlassStep } => ({
  glass_step: GLASS_INTENSITY[level].step,
});

/**
 * The reverse lookup the settings panel uses to display a stored step: any
 * `glass_step` lands on exactly one stop, and the *transparency* is
 * deliberately ignored — the stop is the effect axis, so a stored pair such as
 * the GLASS-UNIFY `(high, 98%)` shows the `liquid` stop while the 98% stays in
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
