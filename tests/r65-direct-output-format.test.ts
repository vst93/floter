// R65 · direct-output commands in the search list: the row nudge, and the
// formatting the text form now reads.
//
// The user's ask, verbatim: 「当前搜索页面下，搜索已配置直出的命令时，列表上对于项
// 需要增加相关提示来提醒用户空格可以直接进入该命令，同时这种直出命令需要在非列表模式
// 下（文本模式下）兼容输出文本的一些格式，有些是带格式的终端字符」.
//
// Two halves, and each can silently drift from the surface it is meant to drive:
//
//   1. **the row nudge** — a search result for a command whose per-command
//      switch is on wears a bare space keycap, and the one string that links the
//      two surfaces is the catalog row's `provider:{extensionId}:{commandId}` id
//      (which Rust builds and `directOutputCommandForRow` resolves);
//   2. **the text form** — ANSI SGR sequences are parsed into styled spans
//      instead of printed as `^[[31m` garbage, and every other escape is
//      dropped, so a captured frame's cursor/erase chatter cannot leak into the
//      block.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  ANSI_DEFAULT_STYLE,
  indexedAnsiColor,
  namedAnsiColor,
  parseAnsi,
  rgbAnsiColor,
  stripAnsi,
} from "../src/launcher/ansi.ts";
import { createTranslator } from "../src/i18n.ts";
import {
  directOutputCommandForRow,
  providerCommandRowId,
  type ExternalPluginCommand,
} from "../src/plugins/external.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const stripCssComments = (source: string) => source.replace(/\/\*[\s\S]*?\*\//g, "");

const ESC = "\u001b";

// ── 1 · the ANSI grammar ──────────────────────────────────────────────────

test("R65 · plain text is one run and reaches the DOM unchanged", () => {
  const spans = parseAnsi("plain output\nsecond line");
  assert.deepEqual(spans, [
    { text: "plain output\nsecond line", style: ANSI_DEFAULT_STYLE },
  ]);
  assert.equal(stripAnsi("plain output"), "plain output");
});

test("R65 · SGR colours, weight and reset are read as the terminal would read them", () => {
  const spans = parseAnsi(`${ESC}[31mred${ESC}[0mplain`);
  assert.equal(spans.length, 2);
  assert.equal(spans[0].text, "red");
  assert.equal(spans[0].style.fg, "var(--ansi-red)");
  assert.equal(spans[1].text, "plain");
  assert.equal(spans[1].style, ANSI_DEFAULT_STYLE, "SGR 0 restores the defaults");

  const [bold] = parseAnsi(`${ESC}[1;34mblue`);
  assert.equal(bold.style.bold, true);
  assert.equal(bold.style.fg, "var(--ansi-blue)");

  const [bright] = parseAnsi(`${ESC}[92mbright`);
  assert.equal(bright.style.fg, "var(--ansi-bright-green)");

  const [onRed] = parseAnsi(`${ESC}[41mx`);
  assert.equal(onRed.style.bg, "var(--ansi-red)");

  const [struck] = parseAnsi(`${ESC}[4;9mx`);
  assert.equal(struck.style.underline, true);
  assert.equal(struck.style.strike, true);

  const [off] = parseAnsi(`${ESC}[1;2;3m${ESC}[22;23mx`);
  assert.equal(off.style.bold, false);
  assert.equal(off.style.dim, false);
  assert.equal(off.style.italic, false);
});

test("R65 · the 256-colour cube and 24-bit truecolour resolve to concrete rgb()", () => {
  // 208 is cube (5, 2, 0): level 5 = 0xff, level 2 = 0x87, level 0 = 0x00.
  const [cube] = parseAnsi(`${ESC}[38;5;208mx`);
  assert.equal(cube.style.fg, "rgb(255, 135, 0)");
  const [ramp] = parseAnsi(`${ESC}[48;5;240mx`);
  assert.equal(ramp.style.bg, "rgb(88, 88, 88)");
  const [trueColor] = parseAnsi(`${ESC}[38;2;10;20;30mx`);
  assert.equal(trueColor.style.fg, "rgb(10, 20, 30)");
  // The named forms go through the helpers, so a test can call them directly.
  assert.equal(namedAnsiColor(9), "var(--ansi-bright-red)");
  assert.equal(indexedAnsiColor(255), "rgb(238, 238, 238)");
  assert.equal(rgbAnsiColor(300, -4, 9.9), "rgb(255, 0, 9)");
});

test("R65 · every non-SGR escape is dropped, not printed", () => {
  // A truncated frame's erase-line, a cursor move and a window-title OSC.
  const input = `${ESC}[2K${ESC}[1;1Hclean${ESC}]0;a title\u0007!`;
  const spans = parseAnsi(input);
  assert.equal(stripAnsi(input), "clean!");
  assert.deepEqual(spans, [{ text: "clean!", style: ANSI_DEFAULT_STYLE }]);
  // A lone trailing ESC (a capture cut mid-sequence) never eats the text before it.
  assert.equal(stripAnsi("before" + ESC), "before");
  assert.equal(stripAnsi("before" + ESC + "[38;"), "before");
});

test("R65 · consecutive characters under one style are one run", () => {
  const spans = parseAnsi(`${ESC}[31ma${ESC}[31mbc`);
  // The second SGR re-states red; the run may split there, but no character is
  // lost and every styled run is red.
  assert.equal(spans.map((span) => span.text).join(""), "abc");
  for (const span of spans) assert.equal(span.style.fg, "var(--ansi-red)");
});

// ── 2 · the text view renders the runs, and the copy stays plain ───────────

test("R65 · the text block draws parsed runs instead of the raw string", async () => {
  const view = stripJsComments(await read("src/launcher/PluginTextView.tsx"));
  assert.match(view, /import \{ ANSI_DEFAULT_STYLE, parseAnsi, type AnsiStyle \} from "\.\/ansi\.ts"/);
  assert.match(view, /useMemo\(\(\) => parseAnsi\(text\), \[text\]\)/, "parsed once per output");
  assert.match(view, /spans\.map\(\(span, index\) =>/, "one element per run");
  assert.match(view, /<span key=\{index\} style=\{style\}>/);
  // The old verbatim render is gone: a raw `{text}` would print the escapes.
  assert.doesNotMatch(view, /\{text\}/, "the block must not print the raw string");
});

test("R65 · the palette lives in the theme, and both light blocks restate it", async () => {
  const css = stripCssComments(await read("src/styles/base.css"));
  // Dark defines all 16 named slots the parser can name.
  for (const name of [
    "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
    "bright-black", "bright-red", "bright-green", "bright-yellow",
    "bright-blue", "bright-magenta", "bright-cyan", "bright-white",
  ]) {
    const declarations = css.split(`--ansi-${name}:`).length - 1;
    assert.equal(declarations, 3, `--ansi-${name} must be declared in dark, light and the pre-hydration block`);
  }
});

// ── 3 · the row nudge ─────────────────────────────────────────────────────

const command = (overrides: Partial<ExternalPluginCommand> = {}): ExternalPluginCommand => ({
  extensionId: "local.tool",
  extensionName: "Tool",
  commandId: "search",
  name: "Search",
  description: "search things",
  aliases: [],
  runtimeAvailable: true,
  ...overrides,
});

test("R65 · a row id resolves to an enabled direct-output command, or nothing", () => {
  const search = command();
  const commands = [search];
  assert.equal(
    providerCommandRowId("local.tool", "search"),
    "provider:local.tool:search",
    "the one string the two surfaces agree on",
  );
  assert.equal(directOutputCommandForRow("provider:local.tool:search", commands), search);
  // Absence is off: a command the enabled subset does not contain stays plain.
  assert.equal(directOutputCommandForRow("provider:local.tool:other", commands), null);
  assert.equal(directOutputCommandForRow("system:git", commands), null);
});

test("R65 · the id format is the one the Rust catalog builds", async () => {
  // `provider_entries` writes `format!("provider:{}:{}", extension_id, id)`; if
  // either side changes spelling the link silently dies, so this pins them.
  const rust = await read("src-tauri/src/extensions/catalog.rs");
  assert.match(rust, /"provider:\{\}:\{\}"/, "the Rust catalog still builds provider ids this way");
});

test("R65 · the nudge is wired from the hook to the row, and gated on the switch", async () => {
  const hook = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(
    hook,
    /directOutputCommandForRow\(entry\.id, enabledExternalCommands\)/,
    "the row asks the enabled registry, not a prefix or a name",
  );
  assert.match(hook, /enabledExternalCommands: readonly ExternalPluginCommand\[\]/);

  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /enabledExternalCommands: enabledExternalCommandList/, "App hands over the enabled subset");

  const row = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(row, /item\.type === "command" && item\.directOutput/);
  assert.match(row, /className="launcher-result__direct"/);
  assert.match(row, /aria-hidden="true"/, "the badge is a nudge, not a control");
  assert.match(row, /title=\{t\("launcher\.directOutputHintTitle"\)\}/);
  assert.doesNotMatch(row, /directOutputBadge/, "no label text on the row");

  const types = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(types, /directOutput\?: boolean;/);
});

test("R65 · the nudge is one bare space keycap, and nothing else", async () => {
  // The user asked for it three times; the third, verbatim: 「提示还是很丑，就笼统点，
  // 简洁点，让用户意识到可以按空格就行了」. So the row shows a key and no words.
  const row = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  // Between the keycap's opening tag and its close there is the glyph — and
  // nothing else. No `{t(…)}` label, no phrase.
  assert.match(
    row,
    /className="launcher-result__direct"[\s\S]{0,260}?>\s*␣\s*<\/span>/,
    "the keycap's only child is the space glyph",
  );

  const css = stripCssComments(await read("src/styles/launcher.css"));
  const rule = css.match(/\.launcher-result__direct \{([\s\S]*?)\}/);
  assert.ok(rule, "the keycap has its own rule");
  const body = rule![1];
  // It wears the terminal empty-state key's bevel (a token, not a literal
  // shadow), so it reads as a keyboard key rather than a floating chip.
  assert.match(body, /box-shadow: var\(--elev-keycap\)/);
  assert.match(body, /border-radius: var\(--radius-xs\)/);
  assert.match(body, /pointer-events: none;/, "the space bar is pressed in the field");
  assert.doesNotMatch(body, /accent/, "the accent budget stays with the selection");
  // The two earlier attempts' rules are gone (no label span, no ellipsised text).
  assert.doesNotMatch(css, /\.launcher-result__direct-text/);
});

// ── 4 · the copy ──────────────────────────────────────────────────────────

test("R65 · the tooltip is the one string, and it names no feature", async () => {
  const i18n = await read("src/i18n.ts");
  assert.equal(
    i18n.split('"launcher.directOutputHintTitle":').length - 1,
    2,
    "the tooltip must appear in en and zh",
  );
  assert.equal(
    i18n.split('"launcher.directOutputBadge":').length - 1,
    0,
    "the row label key is retired — the keycap carries no text",
  );
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  assert.match(en("launcher.directOutputHintTitle"), /space/i);
  assert.match(zh("launcher.directOutputHintTitle"), /空格/);
  assert.doesNotMatch(zh("launcher.directOutputHintTitle"), /直出/);
});
