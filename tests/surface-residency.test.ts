// R35 · the page-residency window and the chip row's Tab affordance.
//
// The user's report, verbatim: 「对于过滤项的切换，添加下 tab 可以切换的提示或暗示。」
// and 「同时应用应该增加逻辑，所在插件和页面应该维持一段时间，这个时间应用配置页面
// 可以设置。在保持的时间内不要退回到搜索框。这个逻辑适用于所有插件和集成、报错终端
// 页面」.
//
// Two things are pinned here:
//
//   * the residency arithmetic and surface table (`src/surface-residency.ts`),
//     as pure functions the clock is built from; and
//   * the wiring: that the *automatic* summon reset consults the clock while
//     the *explicit* exits (Esc / Cmd+W, the close button, running a result)
//     do not — the report's rule is "the timer may not hold the user in".
//
// Mutations that must turn this file red:
//   * `residencyHolds` using `<=` instead of `<` -> the boundary test fails;
//   * `startResidency` treating `0` as an immediate expiry instead of "off"
//     -> the disabled test fails;
//   * `residencySurface` returning `plugin-mode` while the overlay is open
//     -> the overlay-wins test fails;
//   * dropping the `residency.holds()` guard from the reveal handler -> the
//     "the reset consults the clock" shape test fails;
//   * gating `returnToInputMode` on the clock -> the "explicit exits are not
//     gated" test fails;
//   * changing either language's residency constant -> the cross-language
//     parity test fails.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  DEFAULT_RESIDENCY_SECONDS,
  RESIDENCY_CUSTOM_MAX_SECONDS,
  RESIDENCY_MAX_SECONDS,
  RESIDENCY_MIN_SECONDS,
  RESIDENCY_NEVER_SECONDS,
  RESIDENCY_PRESET_SECONDS,
  normalizeResidencySeconds,
  residencyCustomSeedSeconds,
  residencyFromSelect,
  residencyHolds,
  residencyNever,
  residencySelectValue,
  residencySurface,
  startResidency,
} from "../src/surface-residency.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const stripCssComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");

// ── 1 · the duration domain ───────────────────────────────────────────────

test("the residency duration is a [0, 30] integer with a shipped default of 10", () => {
  assert.equal(RESIDENCY_MIN_SECONDS, 0);
  assert.equal(RESIDENCY_MAX_SECONDS, 30);
  assert.equal(DEFAULT_RESIDENCY_SECONDS, 10);
  // `0` is the disabled state, and it is inside the domain rather than beside
  // it: the select's first option is the same number the arithmetic reads.
  assert.ok(RESIDENCY_MIN_SECONDS < DEFAULT_RESIDENCY_SECONDS);
  assert.ok(DEFAULT_RESIDENCY_SECONDS < RESIDENCY_MAX_SECONDS);
});

test("normalizeResidencySeconds clamps, rounds, and defaults a missing value", () => {
  assert.equal(normalizeResidencySeconds(10), 10);
  assert.equal(normalizeResidencySeconds(0), 0, "off survives");
  assert.equal(normalizeResidencySeconds(30), 30);
  // R41 · 30 is the last *preset*, not the ceiling: a custom value above it is
  // kept up to the day.
  assert.equal(normalizeResidencySeconds(31), 31);
  assert.equal(normalizeResidencySeconds(3_600), 3_600);
  assert.equal(
    normalizeResidencySeconds(RESIDENCY_CUSTOM_MAX_SECONDS),
    RESIDENCY_CUSTOM_MAX_SECONDS,
  );
  assert.equal(
    normalizeResidencySeconds(RESIDENCY_CUSTOM_MAX_SECONDS + 1),
    RESIDENCY_CUSTOM_MAX_SECONDS,
    "the custom ceiling is the ceiling",
  );
  assert.equal(normalizeResidencySeconds(-4), 0, "a negative is off, not a past deadline");
  assert.equal(normalizeResidencySeconds(7.6), 8, "the domain is whole seconds");
  // R41 · the "never" sentinel is not a duration: it passes through the clamp
  // untouched (a value clamped to a day would silently stop being "never").
  assert.equal(
    normalizeResidencySeconds(RESIDENCY_NEVER_SECONDS),
    RESIDENCY_NEVER_SECONDS,
  );
  // A missing/typed value lands on the shipped default, never on `0`: a
  // hand-edited file must not silently disable the round's feature.
  assert.equal(normalizeResidencySeconds(undefined), DEFAULT_RESIDENCY_SECONDS);
  assert.equal(normalizeResidencySeconds(null), DEFAULT_RESIDENCY_SECONDS);
  assert.equal(normalizeResidencySeconds("nonsense"), DEFAULT_RESIDENCY_SECONDS);
  assert.equal(normalizeResidencySeconds(Number.NaN), DEFAULT_RESIDENCY_SECONDS);
});

// ── 2 · the clock ─────────────────────────────────────────────────────────

test("a clock holds strictly inside its window and lapses on the boundary", () => {
  const clock = startResidency("plugin-mode", 1_000, 10);
  assert.ok(clock);
  assert.equal(clock!.surface, "plugin-mode");
  assert.equal(clock!.expiresAt, 11_000);
  assert.equal(residencyHolds(clock, 10_999), true);
  assert.equal(residencyHolds(clock, 11_000), false, "the deadline is exclusive");
  assert.equal(residencyHolds(clock, 11_001), false);
  assert.equal(residencyHolds(null, 0), false, "no clock never holds");
});

test("a zero-second window starts no clock at all", () => {
  // `0` means "off", so there is nothing to hold — not a clock that already
  // lapsed, which would read the same today but would keep a stale surface.
  assert.equal(startResidency("settings", 5_000, 0), null);
  assert.equal(startResidency("settings", 5_000, -1), null);
  assert.equal(residencyHolds(startResidency("terminal", 0, 0), 0), false);
});

// ── 3 · the surface table ─────────────────────────────────────────────────

test("the overlay wins over the plugin mode it sits on", () => {
  const pluginMode = { scope: "browser", kind: "all" };
  assert.equal(
    residencySurface({ mode: "collapsed", pluginMode, pluginConfigOpen: true }),
    "plugin-config",
  );
  assert.equal(
    residencySurface({ mode: "collapsed", pluginMode, pluginConfigOpen: false }),
    "plugin-mode",
  );
});

test("every surface and the ordinary search page map exactly once", () => {
  assert.equal(
    residencySurface({ mode: "settings", pluginMode: null, pluginConfigOpen: false }),
    "settings",
  );
  assert.equal(
    residencySurface({ mode: "terminal", pluginMode: null, pluginConfigOpen: false }),
    "terminal",
  );
  // The ordinary search page is `null` — the one state a summon may reset to.
  assert.equal(
    residencySurface({ mode: "collapsed", pluginMode: null, pluginConfigOpen: false }),
    null,
  );
  // A full-surface mode wins even if a stale overlay flag is set: the flags
  // cannot both be true in the app, but the table must not depend on that.
  assert.equal(
    residencySurface({ mode: "terminal", pluginMode: null, pluginConfigOpen: true }),
    "terminal",
  );
  assert.equal(
    residencySurface({ mode: "settings", pluginMode: { scope: "clipboard", filter: "all" }, pluginConfigOpen: true }),
    "settings",
  );
});

// ── 4 · the wiring: the clock gates the automatic reset only ──────────────

test("the reveal handler consults the clock before it would reset the surface", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The clock is built from the settings duration and keyed on the surface.
  assert.match(
    app,
    /const residency = useSurfaceResidency\(settings\.surface_residency_seconds\)/,
  );
  assert.match(app, /const residencySurfaceNow = residencySurface\(\{ mode, pluginMode, pluginConfigOpen \}\)/);
  assert.match(app, /noteResidency\(residencySurfaceNow\)/);
  // The keep branch: a summon lands back inside the surface and restarts the
  // window. It is guarded on the collapsed mode, so the settings and terminal
  // branches above keep owning their own restores.
  assert.match(
    app,
    /modeRef\.current === "collapsed" &&[\s\S]{0,140}?\(heldSurface === "plugin-mode" \|\| heldSurface === "plugin-config"\) &&[\s\S]{0,60}?residency\.holds\(\)/,
  );
  assert.match(app, /residency\.refresh\(\)/);
  // The ordinary reset is still there, for when the clock has lapsed.
  assert.match(app, /setQueryExitingPlugin\(""\)/);
});

test("the explicit exits are never gated on the clock", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // `returnToInputMode` — the terminal's close button and the new-command
  // shortcut — leaves the surface immediately. It must not ask the clock.
  const returnStart = app.indexOf("const returnToInputMode = async () => {");
  const returnEnd = app.indexOf("const openSettings = (page?: SettingsPage) => {");
  assert.ok(returnStart > -1 && returnEnd > returnStart, "the return path must exist");
  assert.doesNotMatch(
    app.slice(returnStart, returnEnd),
    /residency/,
    "an explicit return must not be held back by the timer",
  );
  // Esc / Cmd+W resolve through `onLauncherDismiss`, which is likewise the
  // user's own gesture and must stay unconditional.
  const dismissStart = app.indexOf("const onLauncherDismiss = useCallback(");
  const dismissEnd = app.indexOf("droppedFiles,", dismissStart);
  assert.ok(dismissStart > -1 && dismissEnd > dismissStart, "the dismiss rule must exist");
  assert.doesNotMatch(
    app.slice(dismissStart, dismissEnd),
    /residency/,
    "Esc / Cmd+W must not be held back by the timer",
  );
});

// ── 5 · the chip row's Tab affordance ─────────────────────────────────────

test("the chip row names the key that cycles it, quietly", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /className="launcher-filter__hint"/);
  assert.match(app, /t\("launcher\.browserFilterHint"\)/);
  // The hint is a sibling of the tablist, not a non-tab child of it.
  assert.match(app, /className="launcher-filter__chips"[\s\S]{0,120}?role="tablist"/);
  const css = stripCssComments(await read("src/styles/launcher.css"));
  const block = css.match(/\.launcher-filter__hint\s*\{([^}]*)\}/);
  assert.ok(block, "the hint needs its own rule");
  // Weaker than even an inactive chip (`--text-muted`, body size): caption size
  // and `--text-tertiary`, so the affordance never competes with the chips.
  assert.match(block![1], /font-size:\s*var\(--text-caption\)/);
  assert.match(block![1], /color:\s*var\(--text-tertiary\)/);
  assert.doesNotMatch(block![1], /--accent/, "the hint spends no accent budget");
});

test("the new copy exists in both languages", async () => {
  const source = await read("src/i18n.ts");
  for (const key of [
    "launcher.browserFilterHint",
    "settings.surfaceResidency",
    "settings.surfaceResidencyHint",
    "settings.surfaceResidencyOff",
    "settings.surfaceResidencyValue",
  ]) {
    const occurrences = source.split(`"${key}"`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared in both dictionaries`);
  }
});

// ── 6 · the settings chain, both sides of the bridge ──────────────────────

test("the duration crosses the bridge: Rust stores it, the frontend reads it", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(rust, /pub surface_residency_seconds: u32/);
  assert.match(rust, /#\[serde\(default = "default_surface_residency_seconds"\)\]/);
  assert.match(rust, /pub const DEFAULT_SURFACE_RESIDENCY_SECONDS: u32 = 10;/);
  // R41 · the Rust ceiling is the custom day, and the "never" sentinel is a
  // named constant on the Rust side too.
  assert.match(rust, /pub const MAX_SURFACE_RESIDENCY_SECONDS: u32 = 86_400;/);
  assert.match(rust, /pub const SURFACE_RESIDENCY_NEVER_SECONDS: u32 = u32::MAX;/);
  // The save path clamps but must NOT map `0` to a default (the opacity
  // helper's rule would turn "off" back on), and must exempt the sentinel.
  assert.match(rust, /!= SURFACE_RESIDENCY_NEVER_SECONDS/);
  assert.match(
    rust,
    /surface_residency_seconds\s*\.min\(MAX_SURFACE_RESIDENCY_SECONDS\)/,
  );

  const app = await read("src/App.tsx");
  assert.match(app, /surface_residency_seconds: number;/);

  const hook = await read("src/hooks/useSettings.ts");
  assert.match(hook, /surface_residency_seconds: DEFAULT_RESIDENCY_SECONDS/);
  assert.match(hook, /surface_residency_seconds: normalizeResidencySeconds\(/);

  const page = await read("src/settings/GeneralPage.tsx");
  assert.match(page, /settings\.surface_residency_seconds/);
  assert.match(page, /onChangeGeneralSetting\("surface_residency_seconds", next\)/);
});

test("the Rust constants and the TypeScript constants agree", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  const rustDefault = rust.match(/DEFAULT_SURFACE_RESIDENCY_SECONDS: u32 = (\d+);/);
  const rustMax = rust.match(/MAX_SURFACE_RESIDENCY_SECONDS: u32 = ([\d_]+);/);
  const rustNever = rust.match(/SURFACE_RESIDENCY_NEVER_SECONDS: u32 = u32::MAX;/);
  assert.ok(rustDefault && rustMax && rustNever, "the Rust constants must be declared");
  assert.equal(Number(rustDefault![1]), DEFAULT_RESIDENCY_SECONDS);
  assert.equal(
    Number(rustMax![1].replaceAll("_", "")),
    RESIDENCY_CUSTOM_MAX_SECONDS,
  );
  assert.equal(RESIDENCY_NEVER_SECONDS, 4_294_967_295);
});

// ── 7 · R41: the extended residency ladder ────────────────────────────────

test("the preset ladder is the round numbers the user asked for", () => {
  // The R35 set (0/5/10/15/20/30) mixed a below-default step and an odd 15 in;
  // R41 replaces it with a longer, rounder ladder. `0` and "never" are separate
  // select options, not members of this list.
  assert.deepEqual([...RESIDENCY_PRESET_SECONDS], [10, 20, 30, 60, 120]);
  assert.ok(!(RESIDENCY_PRESET_SECONDS as readonly number[]).includes(0));
  assert.ok(RESIDENCY_PRESET_SECONDS.includes(DEFAULT_RESIDENCY_SECONDS));
  assert.ok(RESIDENCY_PRESET_SECONDS.includes(RESIDENCY_MAX_SECONDS));
});

test("the select's value is a pure mapping of the stored number", () => {
  assert.equal(residencySelectValue(0), "off");
  assert.equal(residencySelectValue(10), "10");
  assert.equal(residencySelectValue(20), "20");
  assert.equal(residencySelectValue(30), "30");
  assert.equal(residencySelectValue(60), "60");
  assert.equal(residencySelectValue(120), "120");
  assert.equal(residencySelectValue(RESIDENCY_NEVER_SECONDS), "never");
  // A custom duration is not one of the named options, so the select falls back
  // to the custom entry; the inline field carries the real number.
  assert.equal(residencySelectValue(45), "custom");
  assert.equal(residencySelectValue(86_400), "custom");
});

test("a select choice resolves to the number it writes", () => {
  assert.equal(residencyFromSelect("off"), 0);
  assert.equal(residencyFromSelect("never"), RESIDENCY_NEVER_SECONDS);
  assert.equal(residencyFromSelect("30"), 30);
  assert.equal(residencyFromSelect("120"), 120);
  // The custom entry carries no number of its own: the inline field owns it,
  // and the caller must not write until the user commits.
  assert.equal(residencyFromSelect("custom"), null);
  assert.equal(residencyFromSelect("nonsense"), null);
});

test("the inline custom field opens on the value it should edit", () => {
  // An existing custom duration seeds itself, so opening "custom" does not
  // silently reset it.
  assert.equal(residencyCustomSeedSeconds(45), 45);
  assert.equal(residencyCustomSeedSeconds(3_600), 3_600);
  // Off, a preset and "never" have no number to edit; the shipped default is
  // the least surprising starting point.
  assert.equal(residencyCustomSeedSeconds(0), DEFAULT_RESIDENCY_SECONDS);
  assert.equal(residencyCustomSeedSeconds(30), DEFAULT_RESIDENCY_SECONDS);
  assert.equal(
    residencyCustomSeedSeconds(RESIDENCY_NEVER_SECONDS),
    DEFAULT_RESIDENCY_SECONDS,
  );
});

test("\"never\" starts a clock that holds forever, and never lapses", () => {
  const clock = startResidency("plugin-mode", 1_000, RESIDENCY_NEVER_SECONDS);
  assert.ok(clock);
  assert.equal(clock!.expiresAt, Number.POSITIVE_INFINITY);
  assert.equal(residencyHolds(clock, 1_000), true);
  assert.equal(residencyHolds(clock, Number.MAX_SAFE_INTEGER), true);
  assert.equal(residencyNever(RESIDENCY_NEVER_SECONDS), true);
  assert.equal(residencyNever(86_400), false);
  // The explicit exits do not consult the clock at all, so "never" cannot trap
  // the user — that is the R35 red line, unchanged.
});

test("the R41 residency copy exists in both languages", async () => {
  const source = await read("src/i18n.ts");
  for (const key of [
    "settings.surfaceResidencyNever",
    "settings.surfaceResidencyCustom",
    "settings.surfaceResidencyCustomValue",
    "settings.surfaceResidencyCustomLabel",
    "settings.surfaceResidencyCustomUnit",
    "settings.surfaceResidencyCustomApply",
  ]) {
    const occurrences = source.split(`"${key}"`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared in both dictionaries`);
  }
});

test("the settings page renders the select, the never option and the inline custom field", async () => {
  const page = stripJsComments(await read("src/settings/GeneralPage.tsx"));
  assert.match(page, /residencySelectValue\(value\)/);
  assert.match(page, /residencyFromSelect\(choice\)/);
  assert.match(page, /t\("settings\.surfaceResidencyNever"\)/);
  assert.match(page, /t\("settings\.surfaceResidencyCustomValue", \{ seconds: value \}\)/);
  assert.match(page, /residency-control__input/);
});
