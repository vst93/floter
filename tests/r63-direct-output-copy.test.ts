// R63 · 「直出」 — the direct-output text surface copies on select.
//
// The user's two sentences were 「"打开开关后，可在搜索框输入命令名加空格呼出，随后
// 输入的内容即为命令参数" 这个逻辑命名为嫁接输出，这种情况下展示的文本内容需要选中即
// 复制的逻辑」 and 「嫁接输出 叫 不好听，再换个合适的名称」 — the round named the
// logic **直出 / Direct output** and gave its text the terminal's R44 gesture.
//
// This file pins the halves that can silently drift apart:
//
//   1. the *rule* — which selection belongs to the output block (a pure module
//      a node test can drive);
//   2. the *chokepoint* — one write, one notice, the same phase machine the
//      terminal reports through, and no coupling to `terminal_select_copy`;
//   3. the *docked row* — never a floating layer, and out of the flow so a copy
//      cannot change the window height (the R58 contract);
//   4. the *naming* — the strings the user named, in both dictionaries.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { selectionTextIn } from "../src/launcher/plugin-text-copy.ts";
import { createTranslator } from "../src/i18n.ts";

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

/** Every rule body in one sheet whose selector is exactly `selector`. */
const bodiesFor = (css: string, selector: string) =>
  rules(css)
    .filter((rule) => rule.selector === selector)
    .map((rule) => rule.body);

const bodyFor = (css: string, selector: string) => {
  const bodies = bodiesFor(css, selector);
  assert.ok(bodies.length > 0, `the sheet must define ${selector}`);
  return bodies.join("\n");
};

// ── 1 · the selection rule ────────────────────────────────────────────────

/** A stand-in for `Node`/`Element`: `selectionTextIn` reads only `contains`. */
const containerOf = (...nodes: unknown[]) => ({
  contains: (node: unknown) => nodes.includes(node),
});

const selectionOf = (anchor: unknown, focus: unknown, text: string) => ({
  isCollapsed: false,
  anchorNode: anchor as Node,
  focusNode: focus as Node,
  toString: () => text,
});

test("R63 · a live selection inside the block is the text to copy", () => {
  const start = {};
  const end = {};
  const block = containerOf(start, end);
  assert.equal(selectionTextIn(block, selectionOf(start, end, "hello world")), "hello world");
  // Trailing whitespace from selecting whole lines is kept verbatim: the thing
  // copied must be the selection, not a normalised version of it.
  assert.equal(selectionTextIn(block, selectionOf(start, end, "line one\n")), "line one\n");
});

test("R63 · a gesture that is not this surface's selection copies nothing", () => {
  const inside = {};
  const outside = {};
  const block = containerOf(inside);

  // No selection at all (a mouseup the browser did not turn into a selection).
  assert.equal(selectionTextIn(block, null), null);
  // No block (the view unmounted under the gesture).
  assert.equal(selectionTextIn(null, selectionOf(inside, inside, "x")), null);
  // A collapsed selection — a plain click, not a drag.
  assert.equal(
    selectionTextIn(block, { ...selectionOf(inside, inside, ""), isCollapsed: true }),
    null,
  );
  // A drag that started in the field and ended in the block (or the reverse):
  // half of the query is in the selection, so it is not ours to copy.
  assert.equal(selectionTextIn(block, selectionOf(outside, inside, "half a query")), null);
  assert.equal(selectionTextIn(block, selectionOf(inside, outside, "half a query")), null);
  // A drag across blank space makes a live selection with nothing in it.
  assert.equal(selectionTextIn(block, selectionOf(inside, inside, "")), null);
  assert.equal(selectionTextIn(block, selectionOf(inside, inside, "   \n  ")), null);
});

// ── 2 · the one chokepoint ────────────────────────────────────────────────

test("R63 · the launcher text copies through one function, which reports through R44's notices", async () => {
  const hook = stripJsComments(await read("src/hooks/useLauncherTextCopy.ts"));
  // The write is the backend command the launcher's other copies use.
  assert.match(hook, /invoke\("clipboard_write_text", \{ text \}\)/, "the Rust-backed write");
  // Both outcomes report, and success follows the awaited write (the R44 rule).
  assert.match(hook, /showCopyNotice\("terminal\.copyNotice\.failed"\)/);
  assert.match(hook, /showCopyNotice\("terminal\.copyNotice\.copied"\)/);
  assert.ok(
    hook.indexOf("await invoke") < hook.indexOf('showCopyNotice("terminal.copyNotice.copied")'),
    "the notice follows the write",
  );
  // A blank string is refused before any notice (the pure rule already drops
  // it, and the chokepoint must not depend on that).
  assert.match(hook, /if \(text\.trim\(\)\.length === 0\) return;/);

  // App wires the hook to the *same* notice state the terminal uses — one
  // vocabulary for 「选中即复制」 on both surfaces.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /const \{ copySelection: copyLauncherSelection \} = useLauncherTextCopy\(showCopyNotice\);/);
  assert.match(app, /onCopySelection=\{copyLauncherSelection\}/);
});

test("R63 · the view only answers the question; the caller owns the copy", async () => {
  const view = stripJsComments(await read("src/launcher/PluginTextView.tsx"));
  // The gesture is armed by a mousedown inside the block and consumed by the
  // matching mouseup *on the window*, so a drag that auto-scrolls the inner
  // viewport and is released outside the box still copies (the release point is
  // the selection end — the R44 rule).
  assert.match(view, /window\.addEventListener\("mousedown", onMouseDown\)/);
  assert.match(view, /window\.addEventListener\("mouseup", onMouseUp\)/);
  assert.match(view, /block\.contains\(event\.target as Node\)/, "only a drag that started here arms it");
  assert.match(view, /selectionTextIn\(blockRef\.current, window\.getSelection\(\)\)/);
  assert.match(
    view,
    /window\.removeEventListener\("mouseup", onMouseUp\)/,
    "the listeners leave with the view",
  );
  // The DOM-facing view never writes the clipboard itself: the chokepoint is
  // the caller's. Mutation: invoke `clipboard_write_text` here and this is red.
  assert.doesNotMatch(view, /clipboard_write_text|invoke\(/, "the write stays in the chokepoint");
  assert.doesNotMatch(view, /preventDefault/, "the native selection and scroller are untouched");
});

test("R63 · the launcher gesture is independent of the terminal's copy switch", async () => {
  // The terminal's `terminal_select_copy` governs its canvas; the launcher's
  // text block exists to be read and copied, so it ships on with no setting.
  for (const file of ["src/launcher/PluginTextView.tsx", "src/hooks/useLauncherTextCopy.ts"]) {
    const source = stripJsComments(await read(file));
    assert.doesNotMatch(source, /select_copy|selectCopy/i, `${file} must not read the terminal switch`);
  }
  // And the terminal's own path is untouched: its copy-on-select is still gated
  // by that persisted value.
  const terminal = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  assert.match(terminal, /if \(finishedSelection && selectCopy\) void copySelection\(\);/);
});

// ── 3 · the docked row ────────────────────────────────────────────────────

test("R63 · the notice is a docked status row, never a floating layer", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // It is rendered only with the text surface, from the same phase machine.
  assert.match(app, /className="launcher-copy-notice"/);
  assert.match(app, /data-phase=\{copyNotice\.phase\}/);
  assert.match(
    app,
    /className="launcher-copy-notice"[\s\S]{0,160}role="status"[\s\S]{0,80}aria-live="polite"/,
  );
  assert.match(app, /\{copyNotice\.message && \(/);

  const css = stripJsComments(await read("src/styles/launcher.css"));
  const row = bodyFor(css, ".launcher-copy-notice");
  assert.match(row, /position:\s*absolute/, "out of the flow");
  assert.match(row, /bottom:\s*0/, "pinned to the panel's own edge");
  assert.match(row, /pointer-events:\s*none/, "the text behind it stays selectable");
  assert.doesNotMatch(row, /border-radius/, "no floating card");
  assert.doesNotMatch(row, /box-shadow/, "no floating card");
  assert.doesNotMatch(row, /accent/, "a status line spends no accent budget");
  // The row is not a flex child of the panel: toggling a message cannot resize
  // anything (the R58 formula's invariant).
  assert.doesNotMatch(row, /flex:\s*0 0 auto/);
  // Idle it paints nothing, so no band is left behind (the R54/R55 lesson).
  assert.match(bodyFor(css, ".launcher-copy-notice__text"), /opacity:\s*0/);
  assert.match(
    bodyFor(css, '.launcher-copy-notice[data-phase="visible"] .launcher-copy-notice__text'),
    /opacity:\s*1/,
  );
  assert.match(
    bodyFor(css, '.launcher-copy-notice[data-phase="fade"] .launcher-copy-notice__text'),
    /opacity:\s*0/,
  );
});

test("R63 · the output block is selectable even though the card is not", async () => {
  const css = stripJsComments(await read("src/styles/launcher.css"));
  // The launcher card turns selection off wholesale (it is a control surface)…
  assert.match(bodyFor(css, ".collapsed-card"), /user-select:\s*none/);
  // …so the one text surface must opt back in, or the gesture is impossible.
  assert.match(bodyFor(css, ".launcher-plugin-text"), /user-select:\s*text/);
});

// ── 4 · the naming ────────────────────────────────────────────────────────

test("R63 · the mode is named 直出 / Direct output in both dictionaries", () => {
  const en = createTranslator("en");
  const zh = createTranslator("zh");

  // The command-list hint names the logic (the words the user quoted).
  assert.match(en("settings.extensions.commandsHint"), /direct output/i);
  assert.match(zh("settings.extensions.commandsHint"), /直出/);

  // The output-mode label takes the name the user chose; the two other routes
  // keep their own words.
  assert.equal(en("settings.extensions.customOutput"), "Direct output");
  assert.equal(zh("settings.extensions.customOutput"), "输出：直出");
  assert.equal(en("settings.extensions.customOutputTerminal"), "Run in terminal");
  assert.equal(zh("settings.extensions.customOutputTerminal"), "在终端中运行");
  assert.equal(en("settings.extensions.customOutputBackground"), "Run in background");
  assert.equal(zh("settings.extensions.customOutputBackground"), "后台运行");

  // The section hint carries the same name so the label cannot read as if it
  // governed the launcher route too.
  assert.match(en("settings.extensions.customOutputHint"), /direct output/i);
  assert.match(zh("settings.extensions.customOutputHint"), /直出/);
});

test("R63 · the changed keys exist exactly once per dictionary", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of [
    "settings.extensions.commandsHint",
    "settings.extensions.customOutput",
    "settings.extensions.customOutputTerminal",
    "settings.extensions.customOutputBackground",
    "settings.extensions.customOutputHint",
  ]) {
    const occurrences = i18n.split(`"${key}":`).length - 1;
    assert.equal(occurrences, 2, `${key} must appear once in en and once in zh`);
  }
});
