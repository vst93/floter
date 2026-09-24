// R42 · the terminal page's settings panel.
//
// The round adds an in-terminal settings surface for the terminal's own
// appearance: line height, padding, font family/size, cursor shape and blink,
// a canvas palette and the scrollbar switch. It ships in two places — the
// settings page's Terminal appearance card and the terminal page's own docked
// panel — from one schema. This file pins the pieces that would let the two
// drift or the settings stop being real:
//
//   1. the domain normalizers (line height snaps to the 0.05 grid; padding and
//      theme reject unknown ids rather than sorting to a variant);
//   2. the Rust shape: every new field, its serde default and its normalize
//      guard, plus the cross-language table agreement;
//   3. the renderer wiring: the canvas reads its cell size from `opts.fontSize`
//      (the R7-13b pin) and the new options exist on `RendererOptions`;
//   4. session preservation: the appearance effects repaint/relayout, they do
//      not close or reset the session;
//   5. the panel is a sibling of the canvas mount, not a floating layer over
//      it (the user's standing objection to overlays), and both surfaces render
//      the one shared component;
//   6. i18n symmetry for the new keys.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  BOLD_MODE_OPTIONS,
  BOLD_MODES,
  CURSOR_SHAPE_OPTIONS,
  DEFAULT_BOLD_MODE,
  DEFAULT_LINE_HEIGHT,
  DEFAULT_PASTE_SAFE,
  DEFAULT_SELECT_COPY,
  DEFAULT_TERMINAL_PADDING,
  DEFAULT_TERMINAL_THEME,
  DEFAULT_WHEEL_LINES,
  LINE_HEIGHT_STEP,
  MAX_LINE_HEIGHT,
  MAX_WHEEL_LINES,
  MIN_LINE_HEIGHT,
  MIN_WHEEL_LINES,
  TERMINAL_PADDING_STEPS,
  TERMINAL_PALETTES,
  TERMINAL_THEMES,
  TERMINAL_THEME_OPTIONS,
  fontFamilyOptions,
  normalizeBoldMode,
  normalizeCursorShape,
  normalizeFontSize,
  normalizeLineHeight,
  normalizePasteSafe,
  normalizeSelectCopy,
  normalizeTerminalPadding,
  normalizeTerminalTheme,
  normalizeWheelLines,
  packedHex,
  terminalPaddingPx,
} from "../src/terminal/terminal-appearance.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

// ── 1 · the domain ─────────────────────────────────────────────────────────

test("line height snaps to the 0.05 grid inside 0.5–2.0", () => {
  assert.equal(normalizeLineHeight(1.4), 1.4, "the shipped value survives exactly");
  assert.equal(normalizeLineHeight(1.37), 1.35, "an off-grid value snaps to the nearest step");
  assert.equal(normalizeLineHeight(1.38), 1.4);
  assert.equal(normalizeLineHeight(0.2), MIN_LINE_HEIGHT, "below the floor clamps up");
  assert.equal(normalizeLineHeight(9), MAX_LINE_HEIGHT, "above the ceiling clamps down");
  assert.equal(normalizeLineHeight(Number.NaN), DEFAULT_LINE_HEIGHT, "a non-finite value falls back");
  // The grid is real: every step is representable without float noise.
  assert.equal(normalizeLineHeight(1.05), 1.05);
  assert.equal(normalizeLineHeight(1.95), 1.95);
  assert.equal(LINE_HEIGHT_STEP, 0.05);
});

test("padding is three named steps with `regular` at the shipped 3px", () => {
  assert.deepEqual(Object.keys(TERMINAL_PADDING_STEPS), ["compact", "regular", "relaxed"]);
  assert.equal(terminalPaddingPx("regular"), 3, "the default is the 3px every earlier build shipped");
  assert.ok(terminalPaddingPx("compact") < terminalPaddingPx("regular"));
  assert.ok(terminalPaddingPx("relaxed") > terminalPaddingPx("regular"));
  assert.equal(normalizeTerminalPadding("regular"), "regular");
  assert.equal(normalizeTerminalPadding("nonsense"), DEFAULT_TERMINAL_PADDING);
});

test("the palette is a canvas-only override, `inherit` reads the document tokens", () => {
  assert.deepEqual([...TERMINAL_THEMES], [
    "inherit",
    "contrast",
    "paper",
    "ink",
    "fog",
    "forest",
    "dusk",
    "mist",
    "amber",
  ]);
  assert.equal(normalizeTerminalTheme("paper"), "paper");
  assert.equal(normalizeTerminalTheme("amber"), "amber");
  assert.equal(normalizeTerminalTheme("solarized"), DEFAULT_TERMINAL_THEME);
  // `inherit` deliberately has no palette: the renderer reads `--terminal-*`.
  assert.equal("inherit" in TERMINAL_PALETTES, false);
  // R43 · every non-inherit theme has a full palette, and the packed colours
  // are real 24-bit values.
  assert.equal(Object.keys(TERMINAL_PALETTES).length, TERMINAL_THEMES.length - 1);
  for (const theme of TERMINAL_THEMES) {
    if (theme === "inherit") continue;
    const palette = TERMINAL_PALETTES[theme];
    assert.equal(typeof palette.bg, "number", `${theme}.bg is a packed colour`);
    assert.equal(typeof palette.fg, "number");
    assert.equal(typeof palette.cursor, "number");
    assert.ok(palette.bg >= 0 && palette.bg <= 0xffffff, `${theme}.bg is 24-bit`);
    assert.ok(palette.fg >= 0 && palette.fg <= 0xffffff, `${theme}.fg is 24-bit`);
    assert.notEqual(palette.bg, palette.fg, `${theme} must not paint text on its own colour`);
    assert.match(palette.selection, /^rgba\(/, `${theme}.selection stays translucent`);
    assert.match(palette.scrollbar, /^rgba\(/);
  }
});

test("the option lists name the shipped ids and every label is a message key", () => {
  assert.deepEqual(CURSOR_SHAPE_OPTIONS.map((option) => option.value), ["beam", "block", "underline"]);
  assert.deepEqual(TERMINAL_THEME_OPTIONS.map((option) => option.value), [...TERMINAL_THEMES]);
  assert.equal(normalizeCursorShape("underline"), "underline");
  assert.equal(normalizeCursorShape("square"), "beam");
  assert.equal(normalizeFontSize(200), 48);
  assert.equal(normalizeFontSize(1), 8);
});

test("the font picker never drops the current face and falls back to the static list", () => {
  // No probe result: the shipped static list is returned.
  const fallback = fontFamilyOptions([], "monospace");
  assert.equal(fallback[0].value, "monospace");
  assert.ok(fallback.length > 1, "the fallback list is more than the system stack");
  // A detected subset narrows the list but keeps the system stack first.
  const detected = fontFamilyOptions(["JetBrains Mono"], "JetBrains Mono");
  assert.equal(detected[0].value, "monospace");
  assert.ok(detected.some((option) => option.value === "JetBrains Mono"));
  assert.ok(!detected.some((option) => option.value === "Menlo"), "undetected candidates are gone");
  // A value that is neither detected nor the system stack stays selectable.
  const withCurrent = fontFamilyOptions(["JetBrains Mono"], "Comic Mono");
  assert.ok(withCurrent.some((option) => option.value === "Comic Mono"));
});

// ── 2 · the renderer wiring ────────────────────────────────────────────────

test("the renderer exposes the new options and keeps the R7-13b pin", async () => {
  const render = stripJsComments(await read("src/terminal/render.ts"));
  for (const option of ["cursorBlink", "showScrollbar", "theme", "wheelLines", "boldMode"]) {
    assert.match(render, new RegExp(`${option}[?:]`), `RendererOptions must carry ${option}`);
  }
  assert.match(render, /setOptions\(next: Partial<RendererOptions>\)/, "an in-place option merge exists");
  // The R7-13b pin: the cell size is built from `opts.fontSize`, never a CSS
  // font-size read. The new axes must not have opened a second source.
  assert.match(render, /this\.opts\.fontSize/);
  assert.doesNotMatch(
    render,
    /getComputedStyle\([^)]*\)\.fontSize|fontSize\s*=\s*.*getComputedStyle/,
    "the canvas still must not read a CSS font-size",
  );
  // The palette override resolves through the shared table, not a local copy.
  assert.match(render, /TERMINAL_PALETTES/, "the canvas palette comes from the schema module");
  // The blink veto is OR'd into the program's request, never replacing it.
  assert.match(render, /!this\.opts\.cursorBlink \|\| !cursorBlinking \|\| blinkOn/);
  // The scrollbar switch gates both the paint and the hit test.
  assert.match(render, /showScrollbar\) return;[\s\S]*ALT_SCREEN/);
});

// ── 3 · session preservation ───────────────────────────────────────────────

test("appearance changes never reset the session", async () => {
  const hook = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  // The two appearance effects merge options and repaint/relayout — nothing
  // more. They must not reach for the session teardown paths.
  const geometry = hook.slice(hook.indexOf("renderer.setOptions({"), hook.indexOf("}, [fontFamily, fontSize, lineHeight, padding])"));
  assert.ok(geometry.length > 0, "the geometry effect must exist");
  assert.match(geometry, /relayoutAndResize\(\)/);
  const palette = hook.slice(hook.indexOf("renderer.setOptions({ cursorBlink"));
  const paletteEffect = palette.slice(
    0,
    palette.indexOf("}, [cursorBlink, showScrollbar, terminalTheme, wheelLines, boldMode])"),
  );
  assert.ok(paletteEffect.length > 0, "the palette effect must exist");
  assert.match(paletteEffect, /render\(\)/);
  for (const forbidden of ["closeTerminalSession", "resetTerminalFrontendState", "frameRef.current = null", "setTerminalResident"]) {
    assert.doesNotMatch(paletteEffect, new RegExp(forbidden), `the palette effect must not ${forbidden}`);
  }
});

// ── 4 · the entry point and the panel ──────────────────────────────────────

test("the terminal page carries a settings entry that flips glyph to close", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /terminalSettingsOpen/, "the panel has its own open state");
  assert.match(app, /toolbar-button--settings/, "the bar's action cluster carries the entry");
  // R41's idiom: the gear becomes an X while the panel is up, and the same
  // spot closes it.
  assert.match(app, /terminalSettingsOpen \? \([\s\S]{0,200}<X /, "the open state renders the X");
  assert.match(app, /SlidersHorizontal/, "the closed state renders the sliders glyph");
});

test("the panel is a sibling of the canvas mount, not a floating layer", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The drawer renders inside the terminal body, after the mount and the
  // resident/feedback notices — a docked strip, not an overlay component.
  const body = app.slice(app.indexOf("terminal-panel__body"));
  assert.ok(
    body.indexOf('ref={mountRef}') < body.indexOf("<TerminalAppearanceSettings"),
    "the settings panel renders after the canvas mount in the body",
  );
  assert.match(app, /terminal-panel__body--settings/, "the body switches to a flex column when open");
  // And it is the shared component, not a second, inlined control set.
  const terminalAppearance = await read("src/settings/TerminalAppearance.tsx");
  assert.match(terminalAppearance, /export function TerminalAppearanceSettings/);
  assert.match(app, /import \{ TerminalAppearanceSettings \}/);
  // The settings page renders the same component (one schema, one truth).
  const general = await read("src/settings/GeneralPage.tsx");
  assert.match(general, /<TerminalAppearanceSettings/);
  assert.match(general, /variant="card"/);
});

// ── 5 · the Rust shape ─────────────────────────────────────────────────────

test("Rust stores every new field with a default and a normalize guard", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  for (const field of [
    "terminal_line_height",
    "terminal_padding",
    "terminal_cursor_blink",
    "terminal_theme",
    "terminal_scrollbar",
    "terminal_wheel_lines",
    "terminal_bold",
    "terminal_select_copy",
    "terminal_paste_safe",
  ]) {
    assert.match(rust, new RegExp(`pub ${field}:`), `AppSettings must carry ${field}`);
  }
  // A pre-round settings file must deserialize to the shipped behaviour, not
  // to `Default::default()` (0.0 would collapse every row; false would turn
  // the cursor and scrollbar off for every existing user).
  assert.match(rust, /#\[serde\(default = "default_terminal_line_height"\)\]/);
  assert.match(rust, /#\[serde\(default = "default_terminal_padding"\)\]/);
  assert.match(rust, /#\[serde\(default = "default_true"\)\]\n\s*pub terminal_cursor_blink/);
  assert.match(rust, /#\[serde\(default = "default_terminal_theme"\)\]/);
  assert.match(rust, /#\[serde\(default = "default_true"\)\]\n\s*pub terminal_scrollbar/);
  assert.match(rust, /pub fn default_terminal_line_height\(\) -> f64\s*\{\s*DEFAULT_LINE_HEIGHT/);
  assert.match(rust, /pub fn default_terminal_padding\(\) -> String\s*\{\s*DEFAULT_TERMINAL_PADDING\.to_string\(\)/);
  assert.match(rust, /pub fn default_terminal_theme\(\) -> String\s*\{\s*DEFAULT_TERMINAL_THEME\.to_string\(\)/);
  // Normalization clamps the number and rejects unknown ids.
  assert.match(rust, /settings\.terminal_line_height = if settings\.terminal_line_height\.is_finite\(\)/);
  assert.match(rust, /TERMINAL_PADDINGS\.contains\(&settings\.terminal_padding\.as_str\(\)\)/);
  assert.match(rust, /TERMINAL_THEMES\.contains\(&settings\.terminal_theme\.as_str\(\)\)/);
  // The shipped defaults agree with the frontend table.
  assert.match(rust, /terminal_line_height: DEFAULT_LINE_HEIGHT/);
  assert.match(rust, /terminal_padding: DEFAULT_TERMINAL_PADDING\.to_string\(\)/);
  assert.match(rust, /terminal_theme: DEFAULT_TERMINAL_THEME\.to_string\(\)/);
});

test("the Rust and TypeScript domains are the same table", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  for (const id of TERMINAL_THEMES) {
    assert.match(rust, new RegExp(`"${id}"`), `Rust must know the terminal theme ${id}`);
  }
  for (const id of Object.keys(TERMINAL_PADDING_STEPS)) {
    assert.match(rust, new RegExp(`"${id}"`), `Rust must know the padding step ${id}`);
  }
  assert.match(rust, /const MIN_LINE_HEIGHT: f64 = 0\.5;/);
  assert.match(rust, /const MAX_LINE_HEIGHT: f64 = 2\.0;/);
  assert.match(rust, /const DEFAULT_LINE_HEIGHT: f64 = 1\.2;/);
});

test("the cursor shape is applied live, not only at spawn", async () => {
  const session = stripJsComments(await read("src-tauri/src/terminal/session.rs"));
  assert.match(session, /pub fn set_cursor_style\(&self, shape: &str\)/, "the session exposes a live setter");
  assert.match(session, /term\.set_options\(config\)/, "it uses alacritty's own live config path");
  const commands = await read("src-tauri/src/commands/terminal.rs");
  assert.match(commands, /pub fn term_set_cursor_style\(/);
  const lib = await read("src-tauri/src/lib.rs");
  assert.match(lib, /term_set_cursor_style/, "the command is registered");
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /invoke\("term_set_cursor_style"/, "the frontend applies a shape change live");
});

// ── 6 · i18n ───────────────────────────────────────────────────────────────

test("every new key is declared in both dictionaries", async () => {
  const source = await read("src/i18n.ts");
  const enStart = source.indexOf("const en = {");
  const enEnd = source.indexOf("export type MessageKey");
  const zhStart = source.indexOf("const zh: Record<MessageKey, string> = {");
  const zhEnd = source.indexOf("const messages: Record<Language");
  const en = source.slice(enStart, enEnd);
  const zh = source.slice(zhStart, zhEnd);
  const keys = [
    "terminal.settingsOpen",
    "terminal.settingsOpenHint",
    "terminal.settingsClose",
    "settings.terminalLineHeight",
    "settings.terminalPadding",
    "settings.terminalPadding.compact",
    "settings.terminalPadding.regular",
    "settings.terminalPadding.relaxed",
    "settings.terminalCursorBlink",
    "settings.terminalTheme",
    "settings.terminalTheme.inherit",
    "settings.terminalTheme.contrast",
    "settings.terminalTheme.paper",
    "settings.terminalTheme.ink",
    "settings.terminalTheme.fog",
    "settings.terminalTheme.forest",
    "settings.terminalTheme.dusk",
    "settings.terminalTheme.mist",
    "settings.terminalTheme.amber",
    "settings.terminalBold",
    "settings.terminalBold.font",
    "settings.terminalBold.bright",
    "settings.terminalWheelLines",
    "settings.terminalWheelLinesValue",
    "settings.terminalSelectCopy",
    "settings.terminalPasteSafe",
    "settings.terminalScrollbar",
  ];
  for (const key of keys) {
    assert.ok(en.includes(`"${key}":`), `en must declare ${key}`);
    assert.ok(zh.includes(`"${key}":`), `zh must declare ${key}`);
  }
});

// ── 7 · R43 · the interaction axes ────────────────────────────────────────

test("R43/R44 · wheel lines clamp to 1–8, bold is one of two ids, the switches read only a real boolean", () => {
  assert.equal(MIN_WHEEL_LINES, 1);
  assert.equal(MAX_WHEEL_LINES, 8);
  assert.equal(DEFAULT_WHEEL_LINES, 3);
  assert.equal(normalizeWheelLines(3), 3);
  assert.equal(normalizeWheelLines(0), MIN_WHEEL_LINES, "below the floor clamps up");
  assert.equal(normalizeWheelLines(99), MAX_WHEEL_LINES, "above the ceiling clamps down");
  assert.equal(normalizeWheelLines(3.6), 4, "an off-grid value rounds");
  assert.equal(normalizeWheelLines(Number.NaN), DEFAULT_WHEEL_LINES);

  assert.deepEqual([...BOLD_MODES], ["font", "bright"]);
  assert.equal(DEFAULT_BOLD_MODE, "font", "the shipped face behaviour is the default");
  assert.equal(normalizeBoldMode("bright"), "bright");
  assert.equal(normalizeBoldMode("nonsense"), DEFAULT_BOLD_MODE);
  assert.deepEqual(BOLD_MODE_OPTIONS.map((option) => option.value), [...BOLD_MODES]);

  // R44 · copy-on-select now ships ON (the user asked for the selection to be
  // copied without a second gesture), while safe paste still ships off. Only a
  // real boolean `true` turns a switch on; `normalizeSelectCopy` is unchanged,
  // because `false` remains a legitimate value a user can persist.
  assert.equal(DEFAULT_SELECT_COPY, true);
  assert.equal(DEFAULT_PASTE_SAFE, false);
  assert.equal(normalizeSelectCopy(true), true);
  assert.equal(normalizeSelectCopy("true"), false, "a hand-edited string is not a switch");
  assert.equal(normalizeSelectCopy(undefined), false);
  assert.equal(normalizePasteSafe(true), true);
  assert.equal(normalizePasteSafe(1), false);
});

test("R43 · the packed palette colour becomes a CSS hex string", () => {
  assert.equal(packedHex(0x000000), "#000000");
  assert.equal(packedHex(0xffffff), "#ffffff");
  assert.equal(packedHex(0x00ff9c), "#00ff9c");
  assert.equal(packedHex(0x0b0d10), "#0b0d10");
  // The mask keeps it 24-bit even if a caller passes something wider.
  assert.equal(packedHex(0xff0b0d10), "#0b0d10");
});

test("R43 · the preview is an inline block painted from the palette, not a second terminal", async () => {
  const source = stripJsComments(await read("src/settings/TerminalAppearance.tsx"));
  assert.match(source, /export function TerminalPalettePreview/, "the preview is its own component");
  // `inherit` reads the document tokens (so it follows the app's theme);
  // an override paints its packed colours.
  assert.match(source, /theme === "inherit" \? null : TERMINAL_PALETTES\[theme\]/);
  assert.match(source, /background: "var\(--terminal-bg\)"/);
  assert.match(source, /packedHex\(palette\.bg\)/);
  assert.match(source, /--preview-cursor/);
  // It is never positioned — the standing objection to overlays.
  assert.doesNotMatch(source, /position:\s*["']?(absolute|fixed)/);
  // No canvas / renderer / PTY is instantiated here.
  assert.doesNotMatch(source, /TerminalCanvas|new .*Renderer|term_input/);
  // The palette row is the select (nine options do not fit a segmented track).
  assert.match(source, /<select[\s\S]{0,400}?TERMINAL_THEME_OPTIONS\.map/);
  // The CSS is inline, scaled by `--u` / the type ladder, and spends no accent.
  const css = stripJsComments(await read("src/styles/settings.css"));
  const preview = /\.terminal-palette-preview\s*\{([^}]*)\}/.exec(css)?.[1] ?? "";
  assert.match(preview, /font-size:\s*var\(--text-caption\)/, "the preview text scales with the type ladder");
  assert.match(preview, /padding:\s*calc\(var\(--u\)/, "and its box scales with the interface step");
  assert.doesNotMatch(preview, /accent/, "the preview spends no accent budget");
});

test("R43 · the Rust domains match the frontend, with defaults and guards", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  // Nine palettes, in the frontend's order.
  const themeTable = /const TERMINAL_THEMES:\s*\[&str;\s*(\d+)\]\s*=\s*\[([\s\S]*?)\];/.exec(rust);
  assert.ok(themeTable, "Rust declares the palette table");
  assert.equal(Number(themeTable![1]), TERMINAL_THEMES.length, "the Rust table is the same length");
  for (const id of TERMINAL_THEMES) {
    assert.match(rust, new RegExp(`"${id}"`), `Rust must know the terminal theme ${id}`);
  }
  // The wheel band and the bold modes.
  assert.match(rust, /const MIN_WHEEL_LINES: u32 = 1;/);
  assert.match(rust, /const MAX_WHEEL_LINES: u32 = 8;/);
  assert.match(rust, /const DEFAULT_WHEEL_LINES: u32 = 3;/);
  assert.match(rust, /const BOLD_MODES: \[&str; 2\] = \["font", "bright"\];/);
  // Every field has a serde default and a normalize guard.
  for (const field of ["terminal_wheel_lines", "terminal_bold", "terminal_select_copy", "terminal_paste_safe"]) {
    assert.match(rust, new RegExp(`pub ${field}:`), `AppSettings must carry ${field}`);
  }
  assert.match(rust, /#\[serde\(default = "default_terminal_wheel_lines"\)\]\n\s*pub terminal_wheel_lines/);
  assert.match(rust, /#\[serde\(default = "default_terminal_bold"\)\]\n\s*pub terminal_bold/);
  // R44 · copy-on-select ships on, so its serde fallback has to be
  // `default_true`: a bare `#[serde(default)]` would read every pre-R44
  // settings file as an explicit `false` and silently undo the new default.
  assert.match(rust, /#\[serde\(default = "default_true"\)\]\n\s*pub terminal_select_copy/);
  assert.match(rust, /settings\.terminal_wheel_lines = settings\s*\.terminal_wheel_lines\s*\.clamp\(MIN_WHEEL_LINES, MAX_WHEEL_LINES\)/);
  assert.match(rust, /BOLD_MODES\.contains\(&settings\.terminal_bold\.as_str\(\)\)/);
  assert.match(rust, /terminal_wheel_lines: DEFAULT_WHEEL_LINES/);
  assert.match(rust, /terminal_bold: DEFAULT_BOLD_MODE\.to_string\(\)/);
});

test("R43 · the renderer reads the wheel count and the bold mode", async () => {
  const render = stripJsComments(await read("src/terminal/render.ts"));
  assert.match(render, /get wheelLines\(\): number/, "the renderer exposes the wheel count to the wheel math");
  assert.match(render, /renderer\.cellHeight \* renderer\.wheelLines/, "the wheel unit is the cell height times the count");
  assert.match(render, /boldMode === "bright"/, "the bright mode spends bold on the foreground");
  assert.match(render, /useBoldFace = bold && !brightBold/, "the face and the colour are separate decisions");
});
