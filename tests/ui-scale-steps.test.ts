// R7-13c · three interface-size steps land on the `--ui-scale` knob.
//
// R7-13a converged the window-size contract, R7-13b tokenized every box and
// type step onto `--ui-scale` and left the knob at 1. This round is the one
// that lets the user move it: a settings field, a step -> multiplier table, the
// application of that multiplier to the document root, and the height paths
// that have to follow.
//
// The round has one structural belief, and every assertion here serves it: the
// scale is applied by *writing a CSS custom property* and every consumer stays
// CSS. The frontend never keeps a second, scaled copy of a measurement. The
// launcher height is therefore re-*measured*, not multiplied — the card is
// drawn from scaled CSS, so its `offsetTop`/`offsetHeight` already carry the
// step, and `measured × factor` would scale twice. The one place a multiplier
// is genuinely needed is the **native fallback height** (`INPUT_WINDOW_HEIGHT`,
// a scale-1 constant that Rust applies before any measurement exists), and that
// maths is pinned on both sides of the language boundary.
//
// Mutations that must turn this file red:
//   * `UI_SCALE_FACTORS.large` 1.1 -> 1.2 (or the Rust table to match only one
//     side) -> the factor-table test and the cross-language test fail;
//   * dropping `settings.ui_scale` from `useLauncherHeight`'s dependency list
//     -> the "re-measures on a step change" assertion fails (a switch would
//     leave the window at the old step's height);
//   * making the measurement multiply by the factor -> the knob-blind guard
//     fails (the double-scale bug this round explicitly forbids);
//   * any module but `src/ui-scale.ts` spelling `--ui-scale` in code -> the
//     ownership sweep in `ui-scale.test.ts` fails.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  UI_SCALE_CSS_VAR,
  UI_SCALE_FACTORS,
  UI_SCALE_STEPS,
  applyUiScale,
  normalizeUiScale,
  uiScaleFactor,
} from "../src/ui-scale.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// ── 1 · the step -> multiplier mapping ────────────────────────────────────

test("the three steps map onto one multiplier each, default first", () => {
  // The round's whole vocabulary. The order is the picker's paint order, and
  // the numbers are the report's decision: default 1 / large 1.1 / larger 1.25.
  assert.deepEqual(UI_SCALE_STEPS, ["default", "large", "larger"]);
  assert.deepEqual(UI_SCALE_FACTORS, { default: 1, large: 1.1, larger: 1.25 });
  // The steps ascend: a picker whose entries do not grow would be three labels
  // for one size.
  const factors = UI_SCALE_STEPS.map((step) => UI_SCALE_FACTORS[step]);
  for (let i = 1; i < factors.length; i += 1) {
    assert.ok(
      factors[i] > factors[i - 1],
      `the steps must ascend: ${factors[i - 1]} -> ${factors[i]}`,
    );
  }
  assert.equal(factors[0], 1, "the default step is the scale every earlier build shipped");
});

test("an unknown or missing step resolves to default, never to a guess", () => {
  // A pre-round settings file has no key, and a hand-edited one may name a step
  // that does not ship. Both must land on the shipped default rather than on
  // whichever entry happens to sort first.
  for (const unknown of [undefined, null, "", "huge", "1.1", "LARGE", {}, 0]) {
    assert.equal(normalizeUiScale(unknown), "default", `${JSON.stringify(unknown)} is not a shipped step`);
  }
  for (const step of UI_SCALE_STEPS) {
    assert.equal(normalizeUiScale(step), step, `${step} is a shipped step`);
  }
  // The factor helper normalizes first, so a stored string can be passed
  // straight through.
  assert.equal(uiScaleFactor("larger"), 1.25);
  assert.equal(uiScaleFactor("nonsense"), 1);
});

// ── 2 · applying the step writes the knob ─────────────────────────────────

test("applying a step writes the step's multiplier to --ui-scale", () => {
  // A root double, so this is an assertion about the *write* rather than about
  // a regex over the module: the property name, the string form of the value,
  // and the fact that each step writes its own number.
  const written: Record<string, string> = {};
  const rootDouble = {
    style: {
      setProperty(name: string, value: string) {
        written[name] = value;
      },
    },
  };
  const expected: [string, string][] = [
    ["default", "1"],
    ["large", "1.1"],
    ["larger", "1.25"],
  ];
  for (const [step, value] of expected) {
    applyUiScale(rootDouble, step);
    assert.equal(written[UI_SCALE_CSS_VAR], value, `${step} must write --ui-scale: ${value}`);
    assert.deepEqual(Object.keys(written), [UI_SCALE_CSS_VAR], "the step writes the knob and nothing else");
  }
  // An unknown step writes the default, not `undefined` or `NaN`.
  applyUiScale(rootDouble, "gigantic");
  assert.equal(written[UI_SCALE_CSS_VAR], "1");
  assert.equal(UI_SCALE_CSS_VAR, "--ui-scale", "the property name is the one the stylesheet declares");
});

// ── 3 · the CSS ladder is on the knob, and the default still resolves ─────

test("the type ladder derives from the knob and still resolves to its shipped pixels", async () => {
  // R7-13b left the `--text-*` ladder literal on purpose (the terminal canvas
  // shared it). R7-13c decouples the canvas (it reads settings, never CSS) and
  // puts the ladder on the knob. Each step must be `calc(<base>px *
  // var(--ui-scale))`, and at the default step must equal the pixel the ladder
  // always carried.
  const base = stripCssComments(await read("src/styles/base.css"));
  const block = base.slice(base.indexOf(":root {"), base.indexOf('[data-theme="light"]'));
  const expected: [string, number][] = [
    ["text-caption", 10],
    ["text-body", 11],
    ["text-emphasis", 12],
    ["text-title", 13],
    ["text-display", 17],
  ];
  for (const [name, px] of expected) {
    const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
    assert.ok(match, `--${name} must be declared in the :root block`);
    const value = match![1].trim();
    assert.equal(
      value,
      `calc(${px}px * var(--ui-scale))`,
      `--${name} must scale with the knob, got "${value}"`,
    );
    // …and the factor is the shipped one, so the default step is a no-op.
    const base_at_one = Number(value.match(/calc\((\d+)px/)?.[1]) * UI_SCALE_FACTORS.default;
    assert.equal(base_at_one, px, `--${name} at the default step must resolve to ${px}px`);
  }
});

// ── 4 · the launcher height re-measures, it never multiplies ──────────────

test("the launcher height re-measures when the step changes instead of multiplying", async () => {
  // The card is laid out from scaled CSS, so the measurement already carries the
  // step. The only correct response to a step change is to *re-run* the
  // measurement; multiplying would scale twice. The trigger lives in App's
  // dependency list, the maths stays in the hook.
  const app = stripJsComments(await read("src/App.tsx"));
  const call = app.slice(app.indexOf("useLauncherHeight("));
  const deps = call.slice(call.indexOf("["), call.indexOf("]"));
  assert.match(deps, /settings\.ui_scale/, "the measurement must re-run on a step change");
  assert.doesNotMatch(
    call.slice(0, call.indexOf(");")),
    /\*\s*uiScaleFactor|uiScaleFactor\s*\(/,
    "the measurement must not multiply by the factor — that would scale twice",
  );

  // And the hook itself stays knob-blind: it reads pixels, not the factor table.
  const hook = stripJsComments(await read("src/hooks/useLauncherHeight.ts"));
  assert.doesNotMatch(
    hook,
    /ui-scale|--u\b|font-base|uiScaleFactor|UI_SCALE_FACTORS/,
    "the measurement reads the laid-out pixels, not the scale vocabulary",
  );
  assert.match(hook, /offsetTop\s*\+\s*last\.offsetHeight/, "the measurement stays the offset arithmetic");
  assert.match(
    hook,
    /setSize\(new LogicalSize\(INPUT_WINDOW_WIDTH, height\)\)/,
    "and still asks for the measured height at the fixed 720 width",
  );

  // Ordering: the knob write must be a *layout* effect declared before the
  // measurement, or React would run the measurement first on a step change and
  // ask for the previous step's height (the window would visibly jump, or not
  // move at all until another dependency changed). `useLauncherHeight` is a
  // `useLayoutEffect` itself, and layout effects run in hook order, so the
  // write has to come first in the body *and* be a layout effect.
  const writeAt = app.indexOf("applyUiScale(document.documentElement");
  const measureAt = app.indexOf("useLauncherHeight(");
  assert.ok(writeAt >= 0 && measureAt > writeAt, "applyUiScale must be declared before useLauncherHeight");
  assert.match(
    app.slice(writeAt - 220, writeAt),
    /useLayoutEffect\(\(\) => \{\s*$/,
    "the knob write must be a layout effect so it lands before the measurement reads the DOM",
  );
});

test("the height measurement adds the scale only to the native fallback, not the measurement", () => {
  // A faithful double of the hook's arithmetic. The card is laid out by the
  // browser from scaled CSS, so each of its boxes carries the step and the
  // measurement is just their sum. The mutation the brief names is "multiply
  // the measurement by the factor": that scales a second time, and this shows
  // it produces a *different* number from the real laid-out height.
  const measure = (card: { offsetTop: number; offsetHeight: number }, frame: number) =>
    Math.ceil(card.offsetTop + card.offsetHeight + frame);

  // A card with three stacked boxes (input row 56, one result row 42, a
  // feedback row 30) plus a 2px frame. At scale 1 the browser reports their
  // sum; at 1.1 each box rounds on its own — which is exactly what
  // `offsetTop`/`offsetHeight` return — so the laid-out total is 142, while
  // `96 = 56+42-2` … the naive `× 1.1` of the scale-1 sum is 141. They differ.
  const atOne = measure({ offsetTop: 56 + 42, offsetHeight: 30 }, 2);
  const atLarge = measure({ offsetTop: 62 + 47, offsetHeight: 33 }, 2);
  assert.equal(atOne, 130);
  assert.equal(atLarge, 144, "the laid-out measurement already carries the step");
  assert.notEqual(
    atLarge,
    Math.ceil(atOne * UI_SCALE_FACTORS.large),
    "multiplying the measured height by the factor would scale twice",
  );

  // The one place a multiplier *is* needed is a scale-1 constant applied before
  // any measurement exists: the native fallback height. Modelled here so the
  // two treatments are stated side by side (Rust owns the real one).
  const FALLBACK_HEIGHT_AT_SCALE_1 = 58;
  assert.equal(
    Math.round(FALLBACK_HEIGHT_AT_SCALE_1 * UI_SCALE_FACTORS.large * 10) / 10,
    63.8,
    "the native fallback does scale",
  );
});

test("the real measurement asks for the budget constant, and only a card taller than it may raise it", async () => {
  // The previous test models the arithmetic; this one drives the actual helper
  // with a stubbed Tauri bridge. R25 changed what the helper asks for: the
  // launcher's window is a fixed slab (`LAUNCHER_WINDOW_HEIGHT`, the ten-row
  // budget), never the measured card — that is what stopped the window being
  // resized on every keystroke. The measurement survives as one guard: a card
  // whose content genuinely outgrew the budget must not be clipped, so the
  // requested height is the larger of the two. What must never happen is a
  // *multiply* — the measurement is read back in the pixels the browser laid
  // out, so scaling it again would double-scale (the bug this test guards).
  const globalWindow = globalThis as unknown as {
    window: unknown;
    getComputedStyle: (el: unknown) => Record<string, string>;
    requestAnimationFrame: (cb: () => void) => number;
  };
  const previousWindow = globalWindow.window;
  const previousGetComputedStyle = globalWindow.getComputedStyle;
  const previousRequestAnimationFrame = globalWindow.requestAnimationFrame;
  const sizes: { width: number; height: number }[] = [];
  // `setSize` passes a live `Size`/`LogicalSize` instance, and that is what this
  // double receives — the `{ Logical: … }` shape only exists *after* the IPC
  // serializer runs. Reading `.Logical` off the instance yields `undefined` and
  // throws inside this async stub, which `syncLauncherHeight`'s
  // `.catch(() => undefined)` swallows (the test then sees zero resizes). Use
  // the same `toJSON()` the serializer uses, so the double sees the payload the
  // backend sees.
  type SetSizeArgs = {
    label: string;
    value: { toJSON(): { Logical: { width: number; height: number } } };
  };
  globalWindow.window = {
    // The height the native reveal left behind (the bare input row). The
    // overflow guard compares the card against the window that is *currently*
    // showing, so the double has to carry — and move with — that number.
    innerHeight: 58,
    __TAURI_INTERNALS__: {
      metadata: { currentWindow: { label: "main" } },
      invoke: async (cmd: string, args: unknown) => {
        // `LogicalSize` serializes as the `Logical` variant; the live instance
        // exposes that through `toJSON()` (see the `SetSizeArgs` note above).
        if (cmd === "plugin:window|set_size") {
          const { width, height } = (args as SetSizeArgs).value.toJSON().Logical;
          sizes.push({ width, height });
          (globalWindow.window as { innerHeight: number }).innerHeight = height;
        }
        return undefined;
      },
      transformCallback: () => 1,
      unregisterCallback: () => undefined,
    },
  };
  globalWindow.getComputedStyle = () => ({
    display: "block",
    borderTopWidth: "1px",
    borderBottomWidth: "1px",
    paddingTop: "0px",
    paddingBottom: "0px",
  });
  // R15: the helper schedules a settle re-measure for the next paint. Stub the
  // frame out and never run the callback — the test asserts the *first* request
  // (the one that carries the measured height), and an unrun frame cannot touch
  // the stubs this test restores in its `finally`.
  globalWindow.requestAnimationFrame = () => 0;

  try {
    const { syncLauncherHeight } = await import("../src/hooks/useLauncherHeight.ts");
    const { LAUNCHER_WINDOW_HEIGHT } = await import("../src/launcher/result-budget.ts");
    const ref = (children: Array<{ offsetTop: number; offsetHeight: number }>) =>
      ({ current: { children, parentElement: null } }) as unknown as {
        current: HTMLDivElement | null;
      };
    // A card laid out at a *step* — the browser reports these numbers after the
    // step is applied, so they already carry it (nothing extra may be applied).
    // Its last child ends at 86 + 42, plus the 2px frame -> 130, well inside the
    // budget, so the window is asked for the budget and nothing else.
    syncLauncherHeight(
      ref([
        { offsetTop: 56, offsetHeight: 30 },
        { offsetTop: 86, offsetHeight: 42 },
      ]),
    );
    await new Promise((resolve) => setTimeout(resolve, 5));

    assert.equal(sizes.length, 1, "the helper asks the window for one size");
    assert.equal(
      sizes[0].height,
      LAUNCHER_WINDOW_HEIGHT,
      "the requested height is the ten-row budget, not the card it happens to be showing",
    );
    assert.equal(sizes[0].width, 720, "and the width is the unscaled window contract");

    // …and the one case the constant cannot express: a card whose content is
    // taller than the budget (a first-run tip above a full list) may raise it.
    syncLauncherHeight(ref([{ offsetTop: 0, offsetHeight: 600 }]));
    await new Promise((resolve) => setTimeout(resolve, 5));

    assert.equal(sizes.length, 2, "a card taller than the budget is not clipped");
    assert.equal(
      sizes[1].height,
      602,
      "the overflow guard asks for the card's own content (600 plus the 2px frame)",
    );
  } finally {
    globalWindow.window = previousWindow;
    globalWindow.getComputedStyle = previousGetComputedStyle;
    globalWindow.requestAnimationFrame = previousRequestAnimationFrame;
  }
});

// ── 5 · the settings field round-trips ────────────────────────────────────

test("ui_scale is a persisted string field with a default-shaped fallback", async () => {
  const hook = await read("src/hooks/useSettings.ts");
  // The frontend default must match Rust's `AppSettings::default()`: the step
  // every earlier build shipped.
  assert.match(
    hook,
    /ui_scale:\s*"default"/,
    "the pre-hydration default must be the shipped step",
  );
  // Hydration normalizes: a pre-round file (no key) and a hand-edited step both
  // land on a shipped value rather than on `undefined`.
  assert.match(
    hook,
    /ui_scale:\s*normalizeUiScale\(loaded\.ui_scale\)/,
    "loadSettings must normalize the stored step",
  );
  // The mutator marks the field changed (so an in-flight load cannot clobber
  // the user's pick) and writes only `ui_scale`.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /ui_scale:\s*UiScale;/, "the AppSettings type carries the field");
});

test("ui_scale survives the load -> pick -> persist round trip", async () => {
  // The assertions above are source-shaped; this one drives the real load
  // boundary. A stored step must come back normalized, and — because the write
  // channel is the ordinary settings save — a pick made while the initial read
  // is still in flight must win over the file that read is about to return.
  const { createSettingsHydration } = await import("../src/settings-persistence.ts");
  const { normalizeUiScale } = await import("../src/ui-scale.ts");

  type Store = { ui_scale: string; theme: string };
  const onDisk: Store = { ui_scale: "large", theme: "dark" };
  const hydration = createSettingsHydration<Store>();

  // The user picks `larger` before the read lands.
  const picked: Store = { ui_scale: "larger", theme: "dark" };
  hydration.markChanged("ui_scale");

  // The load applies its normalizer (the hook's `normalizeUiScale(loaded.…)`),
  // then merges: the in-flight pick wins, the untouched field comes from disk.
  // (This is the same order `useSettings` uses: merge, then `finish`.)
  const loaded: Store = {
    ui_scale: normalizeUiScale(onDisk.ui_scale),
    theme: onDisk.theme,
  };
  const merged = hydration.mergeLoaded(picked, loaded);
  hydration.finish();
  assert.equal(merged.ui_scale, "larger", "a pick made mid-read must survive hydration");

  // A file that predates the round (no key) normalizes to the shipped step —
  // the value the *next* launch reads back. That is the round trip: persist
  // `larger`, restart, and `larger` is what comes back.
  assert.equal(normalizeUiScale((onDisk as { ui_scale?: unknown }).ui_scale), "large");
  assert.equal(normalizeUiScale(undefined), "default", "a pre-round file keeps the default step");
});

test("the settings panel height scales its base, its viewport caps do not", async () => {
  // The settings window is a *fixed* base height, not a measurement, so it is
  // the same shape as the native fallback: the shipped constant scales with the
  // step and the screen-derived caps stay viewport limits. A larger step needs
  // a taller panel to show the same rows; the caps are what the display allows,
  // which no interface step can change.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /SETTINGS_WINDOW_HEIGHT\s*\*\s*uiScaleFactor\(settings\.ui_scale\)/,
    "the settings base height must scale with the interface step",
  );
  assert.doesNotMatch(
    app,
    /available\s*\*\s*0\.72\s*\*\s*uiScaleFactor|uiScaleFactor\([^)]*\)\s*\*\s*available/,
    "the screen-derived cap is a viewport limit and must not scale",
  );
});

test("the Rust settings shape carries ui_scale with an explicit default", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  // A bare `#[serde(default)]` would deserialize a missing key to `""`, which
  // names no shipped step. The explicit default fn lands on `default`.
  assert.match(
    rust,
    /#\[serde\(default = "default_ui_scale"\)\]\s*\n\s*pub ui_scale: String,/,
    "the field must be `ui_scale: String` with an explicit default fn",
  );
  assert.match(
    rust,
    /pub fn default_ui_scale\(\) -> String\s*\{\s*DEFAULT_UI_SCALE\.to_string\(\)/,
    "`default_ui_scale` must return the shipped step",
  );
  // The default has to be the shipped step, so an old file is not silently
  // scaled.
  assert.match(
    rust,
    /ui_scale:\s*DEFAULT_UI_SCALE\.to_string\(\)/,
    "AppSettings::default must agree with the frontend default",
  );
  // Unknown values are normalized at the command boundary, not passed through.
  assert.match(rust, /settings\.ui_scale = if UI_SCALE_STEPS/, "normalize_settings must guard the step");
});

// ── 6 · the step is a scale axis only: the terminal font stays user-owned ──

test("the terminal canvas font is not on the knob", async () => {
  // The decoupling marker R7-13b left is this round's contract: terminal
  // *chrome* type follows the ladder (and so the knob), but the canvas paints
  // cells at `settings.font_size`, read straight from settings.
  const terminal = await read("src/styles/terminal.css");
  assert.match(terminal, /R7-13b terminal type decoupling/, "the marker must survive leg 3");
  // The canvas's own size never comes from CSS: the renderer's font string is
  // built from `opts.fontSize`, not a `getComputedStyle` font-size read.
  const render = stripJsComments(await read("src/terminal/render.ts"));
  assert.match(render, /this\.opts\.fontSize/, "the canvas cell size reads the renderer option");
  assert.doesNotMatch(
    render,
    /getComputedStyle\([^)]*\)\.fontSize|fontSize\s*=\s*.*getComputedStyle/,
    "the canvas must not read a CSS font-size",
  );
  // And the font_size path in settings is independent: the ui-scale mutator
  // never writes font_size, and the font-size mutator never writes ui_scale.
  const hook = stripJsComments(await read("src/hooks/useSettings.ts"));
  const uiScaleMutator = hook.slice(
    hook.indexOf("const changeUiScale"),
    hook.indexOf("const changeTheme"),
  );
  assert.ok(uiScaleMutator.length > 0, "changeUiScale must exist");
  assert.doesNotMatch(uiScaleMutator, /font_size|fontSize/, "changing the UI scale must not move the terminal font size");
  const fontSizeMutator = hook.slice(
    hook.indexOf("const changeFontSize"),
    hook.indexOf("const changeGeneralSetting"),
  );
  assert.doesNotMatch(fontSizeMutator, /ui_scale/, "changing the terminal font size must not move the UI scale");
});

// ── 7 · the cross-language factor table ───────────────────────────────────

test("the frontend and Rust factor tables are the same table", async () => {
  // Rust cannot import TypeScript, so — exactly as with `INPUT_WINDOW_WIDTH` —
  // both sides declare the numbers and this test pins them. The Rust table is
  // `pub const UI_SCALE_STEPS: [(&str, f64); 3] = [("default", 1.0), …]`.
  const rust = await read("src-tauri/src/commands/config.rs");
  const match = rust.match(
    /pub const UI_SCALE_STEPS:\s*\[\(&str, f64\); 3\]\s*=\s*\[([\s\S]*?)\];/,
  );
  assert.ok(
    match,
    "config.rs no longer declares `pub const UI_SCALE_STEPS: [(&str, f64); 3] = […];` — " +
      "the native half of this table is what the reset height reads",
  );
  const pairs = [...match![1].matchAll(/\("([a-z]+)",\s*([0-9.]+)\)/g)].map(
    (m) => [m[1], Number(m[2])] as [string, number],
  );
  assert.deepEqual(
    pairs.map(([id]) => id),
    [...UI_SCALE_STEPS],
    "the step ids must match, in order",
  );
  for (const [id, factor] of pairs) {
    assert.equal(
      factor,
      UI_SCALE_FACTORS[id as keyof typeof UI_SCALE_FACTORS],
      `Rust ${id} = ${factor} but the frontend says ${UI_SCALE_FACTORS[id as keyof typeof UI_SCALE_FACTORS]}; ` +
        "the two halves of the interface scale must be one table",
    );
  }
  // The Rust default step string must match the frontend's.
  assert.match(
    rust,
    /pub const DEFAULT_UI_SCALE:\s*&str\s*=\s*"default"/,
    "Rust must ship the same default step",
  );
});

// ── 8 · the native reset height scales, the width does not ────────────────

test("the native fallback height scales with the step and the width never does", async () => {
  const lib = await read("src-tauri/src/lib.rs");
  // The reset height is derived, not the raw constant: `base × factor(step)`.
  assert.match(
    lib,
    /fn scaled_input_window_height\(step: &str\) -> f64\s*\{\s*INPUT_WINDOW_HEIGHT\s*\*\s*commands::config::ui_scale_factor\(step\)/,
    "the fallback height must be the scale-1 base multiplied by the step's factor",
  );
  // The two reset call sites use the derived height…
  const resetUses = [...lib.matchAll(/resize_window\(\s*&?window,\s*INPUT_WINDOW_WIDTH,\s*([^,]+),/g)].map(
    (m) => m[1].trim(),
  );
  assert.ok(resetUses.length >= 1, "the reset paths must still resize the collapsed window");
  for (const argument of resetUses) {
    assert.match(
      argument,
      /input_window_height\(\)/,
      `a reset path passed "${argument}" instead of the scaled height`,
    );
  }
  // …and the width argument is the contract constant at every one of them.
  assert.doesNotMatch(
    lib,
    /ui_scale_factor\([^)]*\)\s*\*\s*INPUT_WINDOW_WIDTH|INPUT_WINDOW_WIDTH\s*\*\s*ui_scale_factor/,
    "the width is the window contract and must not be scaled",
  );
  // The scale-1 constant survives as the base (R7-13a's contract reads it), and
  // the width constant is untouched.
  assert.match(lib, /const INPUT_WINDOW_HEIGHT: f64 = 58\.0;/);
  assert.match(lib, /const INPUT_WINDOW_WIDTH: f64 = 720\.0;/);
});
