// R54 · the terminal's row rhythm, and the two ways out of its settings drawer.
//
// The user's two sentences were 「内置终端的配置中行距需要增加小于 1 的选择，比如到
// 最小 0.5，同时默认行距设置为 1.2」 and 「再就是在终端页面下进行设置时应该在配置
// 模块上也加上关闭按钮。同时现在没打开配置时配置模块还是显示了一行空白 div」.
//
// This file pins the four things that could drift from that intent:
//
//   1. the line-height domain: a 0.5 floor, a 1.2 default, the 0.05 grid — with
//      the Rust copy of the constants in step (the same two-table arrangement
//      R42 established for the rest of the appearance axes);
//   2. the clip guard that makes a sub-1.0 multiple safe rather than sliced:
//      `measureCell` keeps taking the max of the requested row and the face's
//      own ink floor;
//   3. the drawer's own close control — the same `toolbar-button` the bar's
//      gear/X uses, wired to the *existing* open/close state;
//   4. the blank band. The always-mounted copy-notice row was being painted
//      like a control in every phase (fill + hairline), so with the settings
//      drawer closed the bottom of the terminal read as「一行空白 div」. The
//      row's paint is phase-owned now: idle draws the canvas's own colour at
//      the canvas's own alpha, the live phases draw the strip.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  DEFAULT_LINE_HEIGHT,
  LINE_HEIGHT_STEP,
  MAX_LINE_HEIGHT,
  MIN_LINE_HEIGHT,
  TERMINAL_PALETTES,
  TERMINAL_THEMES,
  normalizeLineHeight,
  packedHex,
  terminalCanvasFill,
} from "../src/terminal/terminal-appearance.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

/** Every rule body whose selector list contains `selector` as one part. */
const bodiesFor = (css: string, selector: string) =>
  rules(css)
    .filter((rule) => rule.selector.split(",").some((part) => part.trim() === selector))
    .map((rule) => rule.body);

const bodyFor = (css: string, selector: string) => {
  const bodies = bodiesFor(css, selector);
  assert.ok(bodies.length > 0, `the sheet must define ${selector}`);
  return bodies.join("\n");
};

// ── 1 · the line-height domain ────────────────────────────────────────────

test("R54 · the line height reaches 0.5, defaults to 1.2, and keeps the 0.05 grid", () => {
  assert.equal(MIN_LINE_HEIGHT, 0.5, "the floor is the value the user asked for");
  assert.equal(MAX_LINE_HEIGHT, 2, "the ceiling is unchanged (R42)");
  assert.equal(LINE_HEIGHT_STEP, 0.05);
  assert.equal(DEFAULT_LINE_HEIGHT, 1.2, "the shipped default moved down from 1.4");

  // The floor is a real step on the grid, not a clamp that snaps to 0.55.
  assert.equal(normalizeLineHeight(0.5), 0.5);
  assert.equal(normalizeLineHeight(0.52), 0.5);
  assert.equal(normalizeLineHeight(0.575), 0.6, "an off-grid value still snaps to the nearest step");
  assert.equal(normalizeLineHeight(0.2), MIN_LINE_HEIGHT, "below the floor clamps up");
  assert.equal(normalizeLineHeight(Number.NaN), DEFAULT_LINE_HEIGHT, "a non-finite value falls back");
  assert.equal(normalizeLineHeight(1.2), 1.2, "the default survives the grid exactly");
  // 1.4 is still a legal step, so a persisted value needs no migration.
  assert.equal(normalizeLineHeight(1.4), 1.4);
  // A persisted value *below* the old floor (impossible before R54, hand-edit
  // only) is lifted to the new floor rather than to 1.0.
  assert.equal(normalizeLineHeight(0.5), 0.5);
});

test("R54 · both sliders read the domain, so the card and the strip cannot drift", async () => {
  const source = stripJsComments(await read("src/settings/TerminalAppearance.tsx"));
  // The one `rows` table feeds both variants; the line-height row's min/step
  // come from the domain module, never from a literal.
  assert.match(source, /min=\{MIN_LINE_HEIGHT\}/);
  assert.match(source, /max=\{MAX_LINE_HEIGHT\}/);
  assert.match(source, /step=\{LINE_HEIGHT_STEP\}/);
  assert.doesNotMatch(source, /min=\{1\}/, "no surface may reintroduce the old 1.0 floor");
});

test("R54 · Rust mirrors the floor and the default", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(rust, /const MIN_LINE_HEIGHT: f64 = 0\.5;/);
  assert.match(rust, /const DEFAULT_LINE_HEIGHT: f64 = 1\.2;/);
  assert.match(rust, /const MAX_LINE_HEIGHT: f64 = 2\.0;/);
  // The serde fallback, the struct default and the guard all read the const,
  // so there is no second literal to forget.
  assert.match(rust, /pub fn default_terminal_line_height\(\) -> f64 \{\s*DEFAULT_LINE_HEIGHT/);
  assert.match(rust, /terminal_line_height: DEFAULT_LINE_HEIGHT/);
  assert.match(rust, /\.clamp\(MIN_LINE_HEIGHT, MAX_LINE_HEIGHT\)/);
  assert.doesNotMatch(rust, /MIN_LINE_HEIGHT: f64 = 1\.0/);
  // A sub-1.0 value is a legitimate setting on both sides of the wire.
  assert.match(rust, /terminal_line_height: 0\.6,/);
});

// ── 2 · the clip guard ────────────────────────────────────────────────────

test("R54 · a tight multiple saturates at the face's ink floor instead of clipping", async () => {
  const render = stripJsComments(await read("src/terminal/render.ts"));
  // The row height is the max of the requested multiple and the face's own
  // ink box (+1px of breathing room): that is what makes a 0.5 multiple safe
  // rather than sliced. Measured on WebKitGTK 2.52.3 with the shipped default
  // stack at 14px the floor is 17px, so 0.5 through 1.20 all paint 17px.
  assert.match(render, /const ink = metrics\.fontBoundingBoxAscent \+ metrics\.fontBoundingBoxDescent;/);
  assert.match(render, /const inkFloor = Number\.isFinite\(ink\) \? Math\.ceil\(ink\) \+ CELL_INK_PADDING : 0;/);
  assert.match(
    render,
    /this\.cellHeight = Math\.max\(\s*1,\s*Math\.ceil\(this\.opts\.fontSize \* this\.opts\.lineHeight\),\s*inkFloor,\s*\);/,
    "the ink floor still takes part in the row height",
  );
  assert.match(render, /const CELL_INK_PADDING = 1;/);
});

// ── 3 · the drawer's own close ────────────────────────────────────────────

test("R54 · the settings drawer can be dismissed from itself", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  const drawer = app.slice(app.indexOf('className="terminal-settings-drawer"'));
  const close = drawer.slice(0, drawer.indexOf("</div>", drawer.indexOf("TerminalAppearanceSettings")));
  // Same icon-button vocabulary as the bar's gear/X, in the drawer's corner.
  assert.match(close, /className="toolbar-button terminal-settings-drawer__close"/);
  assert.match(close, /<X size=\{15\} strokeWidth=\{1\.8\}/);
  // The existing i18n key names it — no second label for one action.
  assert.match(close, /aria-label=\{t\("terminal\.settingsClose"\)\}/);
  assert.match(close, /title=\{t\("terminal\.settingsClose"\)\}/);
  // And it runs the *existing* open/close state, not a second piece of state.
  assert.match(close, /onClick=\{\(\) => setTerminalSettingsOpen\(false\)\}/);
  assert.doesNotMatch(close, /useState/);
  // The bar entry still toggles the same flag, so the two exits agree.
  assert.match(app, /onClick=\{\(\) => setTerminalSettingsOpen\(\(open\) => !open\)\}/);

  // The CSS gives the button the corner and keeps the grid flexible.
  const css = stripJsComments(await read("src/styles/terminal.css"));
  const drawerCss = bodyFor(css, ".terminal-settings-drawer");
  assert.match(drawerCss, /display:\s*flex/);
  assert.match(drawerCss, /align-items:\s*flex-start/, "the control rides the drawer's top line");
  assert.match(bodyFor(css, ".terminal-settings-drawer > .terminal-settings"), /flex:\s*1 1 auto/);
  assert.match(bodyFor(css, ".terminal-settings-drawer__close"), /flex:\s*none/);
});

test("R54 · the close label is translated in both languages", async () => {
  const source = await read("src/i18n.ts");
  const en = source.slice(source.indexOf("const en = {"), source.indexOf("export type MessageKey"));
  const zh = source.slice(source.indexOf("const zh: Record<MessageKey, string> = {"));
  for (const table of [en, zh]) {
    assert.match(table, /"terminal\.settingsClose":/);
  }
});

// ── 4 · the blank band ────────────────────────────────────────────────────

test("R54 · the palette helper hands the canvas's own colour to the CSS", () => {
  assert.equal(terminalCanvasFill("inherit"), "var(--terminal-bg)", "inherit reads the document token");
  for (const id of TERMINAL_THEMES) {
    if (id === "inherit") continue;
    assert.equal(
      terminalCanvasFill(id),
      packedHex(TERMINAL_PALETTES[id].bg),
      `${id} must hand over its packed background`,
    );
  }
  // A hand-edited unknown id resolves like `inherit` rather than throwing.
  assert.equal(terminalCanvasFill("solarized"), "var(--terminal-bg)");
});

test("R54/R55 · the closed-drawer band is gone structurally, not by colour-matching", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // R55 · the notice lives *inside* the canvas mount (the mount ref wraps it),
  // and no longer carries the canvas's colour: idle it paints nothing.
  const mount = app.slice(app.indexOf("className=\"terminal-panel__mount\""));
  assert.ok(
    mount.indexOf("className=\"terminal-copy-notice\"") < mount.indexOf("</div>", mount.indexOf("className=\"terminal-copy-notice\"")),
  );
  assert.equal(/terminalCanvasFill/.test(app), false, "no colour-matching left in the panel");

  const css = stripJsComments(await read("src/styles/terminal.css"));
  const base = bodyFor(css, ".terminal-copy-notice");
  // The overlay takes no layout height, so a copy never resizes the canvas/PTY.
  assert.match(base, /position:\s*absolute/);
  assert.match(base, /bottom:\s*0/);
  assert.match(base, /height:\s*var\(--terminal-status-height\)/);
  // The idle overlay draws nothing at all: the canvas below is the only thing
  // composited, so there is no 1-alpha seam left to see.
  assert.match(base, /border-top:\s*1px solid transparent/, "no hairline on the idle row");
  assert.match(base, /background:\s*transparent/, "the idle row draws no pane fill");
  assert.doesNotMatch(base, /glass-control/, "not even a token it could later be repainted with");
  assert.match(base, /transition:\s*background-color var\(--dur-3\) ease, border-color var\(--dur-3\) ease/);

  // The R54 `::before` inset is retired: there is no reserved row to paint.
  assert.equal(bodiesFor(css, ".terminal-copy-notice::before").length, 0);

  // The two live phases own the band's material, so a message still reads as a
  // status line pinned to the canvas's edge.
  for (const phase of ["visible", "fade"]) {
    const live = bodyFor(css, `.terminal-copy-notice[data-phase="${phase}"]`);
    assert.match(live, /border-top-color:\s*var\(--glass-control-edge\)/);
    assert.match(live, /background-color:\s*var\(--glass-control\)/);
  }

  // The notice's text still wins over the row's own paint.
  const text = bodyFor(css, ".terminal-copy-notice__text");
  assert.match(text, /position:\s*relative/);
  assert.match(text, /opacity:\s*0/);
});
