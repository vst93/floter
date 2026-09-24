// R44 · copy-on-select ships on, and every copy reports itself.
//
// The user's two sentences were 「终端中的"选中即复制"这个选项可以默认开启，复制成功
// 之后需要有一个相应的提示」. This file pins the two halves that could silently
// drift apart from the intent:
//
//   1. the default flip — one truth in `terminal-appearance.ts`, mirrored by
//      the Rust `Default` impl *and* its serde fallback, with `false` still a
//      legitimate value a user can persist;
//   2. the notice — a pure phase machine (`terminal/copy-notice.ts`), one choke
//      point that every copy path reports through, and a docked status row that
//      is never a floating overlay and never changes the panel's height.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  COPY_NOTICE_FADE_MS,
  COPY_NOTICE_IDLE,
  COPY_NOTICE_PHASES,
  COPY_NOTICE_VISIBLE_MS,
  copyNoticeAdvance,
  copyNoticeDelay,
  copyNoticeShow,
} from "../src/terminal/copy-notice.ts";
import { DEFAULT_SELECT_COPY, normalizeSelectCopy } from "../src/terminal/terminal-appearance.ts";

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

// ── 1 · the default flip ──────────────────────────────────────────────────

test("R44 · copy-on-select ships on, and only a real boolean moves it", () => {
  assert.equal(DEFAULT_SELECT_COPY, true, "the selection is copied without a second gesture");
  // The normalizer is deliberately untouched: `false` is still a value the
  // user can persist (the switch is a veto, not a constant).
  assert.equal(normalizeSelectCopy(true), true);
  assert.equal(normalizeSelectCopy(false), false, "an explicit off survives");
  assert.equal(normalizeSelectCopy("true"), false, "a hand-edited string is not a switch");
  assert.equal(normalizeSelectCopy(1), false);
  assert.equal(normalizeSelectCopy(undefined), false);
});

test("R44 · the frontend has one default and both surfaces read it", async () => {
  const settings = stripJsComments(await read("src/hooks/useSettings.ts"));
  // The pre-hydration frame must match the backend, and it takes the value
  // from the constant rather than repeating a literal.
  assert.match(settings, /terminal_select_copy: DEFAULT_SELECT_COPY/);
  assert.match(settings, /loaded\.terminal_select_copy \?\? DEFAULT_SELECT_COPY/);
  // A pre-round file that lacks the key is normalized from the same constant.
  assert.match(settings, /terminal_select_copy: normalizeSelectCopy\(/);
  // The settings UI's unset state reads the same normalizer, so the switch and
  // the behaviour cannot disagree about what "unset" means.
  const appearance = stripJsComments(await read("src/settings/TerminalAppearance.tsx"));
  assert.match(appearance, /checked=\{normalizeSelectCopy\(settings\.terminal_select_copy\)\}/);
  // And the terminal view takes the persisted value, not a second default.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /selectCopy: settings\.terminal_select_copy/);
});

test("R44 · the Rust default and its serde fallback both flip to on", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(rust, /terminal_select_copy: true,/, "Default::default() ships it on");
  // The field's fallback must be `default_true`: a bare `#[serde(default)]`
  // would read a pre-R44 file's *absence* of the key as an explicit `false`
  // and silently undo the new default for every existing user.
  assert.match(rust, /#\[serde\(default = "default_true"\)\]\s*\n\s*pub terminal_select_copy: bool/);
  assert.doesNotMatch(
    rust,
    /#\[serde\(default\)\]\s*\n\s*pub terminal_select_copy: bool/,
    "the bare serde default would fall back to false",
  );
  // Normalization leaves the switch alone — `false` is legal.
  assert.doesNotMatch(rust, /settings\.terminal_select_copy\s*=/);
});

// ── 2 · the phase machine ─────────────────────────────────────────────────

test("R44 · the notice machine steps idle → visible → fade → idle", () => {
  assert.deepEqual([...COPY_NOTICE_PHASES], ["idle", "visible", "fade"]);
  assert.equal(COPY_NOTICE_IDLE.phase, "idle");
  assert.equal(COPY_NOTICE_IDLE.message, null);

  const visible = copyNoticeShow(COPY_NOTICE_IDLE, "terminal.copyNotice.copied");
  assert.equal(visible.phase, "visible");
  assert.equal(visible.message, "terminal.copyNotice.copied");

  const fade = copyNoticeAdvance(visible);
  assert.equal(fade.phase, "fade");
  assert.equal(fade.message, visible.message, "the message survives the fade");

  const idle = copyNoticeAdvance(fade);
  assert.deepEqual(idle, COPY_NOTICE_IDLE);
  // Idle is absorbing: advancing again is a no-op, not a new state.
  assert.equal(copyNoticeAdvance(idle), idle);
});

test("R44 · a repeat copy restarts the clock without blinking", () => {
  const first = copyNoticeShow(COPY_NOTICE_IDLE, "terminal.copyNotice.copied");
  const fading = copyNoticeAdvance(first);
  const again = copyNoticeShow(fading, "terminal.copyNotice.copied");
  assert.equal(again.phase, "visible", "a copy during the fade pulls the message back");
  // A fresh object is what restarts the hook's timer even when the phase did
  // not change (React compares the state by identity).
  assert.notEqual(again, fading);
  assert.notEqual(copyNoticeShow(first, "terminal.copyNotice.copied"), first);
  // A different message simply replaces the one on screen.
  assert.equal(
    copyNoticeShow(first, "terminal.copyNotice.failed").message,
    "terminal.copyNotice.failed",
  );
});

test("R44 · each phase owns its delay, and idle owns none", () => {
  assert.equal(copyNoticeDelay(COPY_NOTICE_IDLE), null, "idle arms no timer");
  assert.equal(copyNoticeDelay(copyNoticeShow(COPY_NOTICE_IDLE, "terminal.copyNotice.copied")), COPY_NOTICE_VISIBLE_MS);
  assert.equal(copyNoticeDelay(copyNoticeAdvance(copyNoticeShow(COPY_NOTICE_IDLE, "terminal.copyNotice.copied"))), COPY_NOTICE_FADE_MS);
  assert.equal(COPY_NOTICE_VISIBLE_MS, 1500, "the brief's ~1.5s");
  // The fade window has to outlast the CSS transition it exists to run.
  assert.ok(COPY_NOTICE_FADE_MS > 160, "the text is mounted for the whole fade");
});

// ── 3 · the wiring ────────────────────────────────────────────────────────

test("R44 · every copy path reports through the one choke point", async () => {
  const view = stripJsComments(await read("src/hooks/useTerminalView.ts"));
  const copy = view.slice(view.indexOf("const copySelection = async () => {"));
  const body = copy.slice(0, copy.indexOf("const sendTerminalText"));
  assert.match(body, /showCopyNotice\("terminal\.copyNotice\.copied"\)/, "success reports");
  assert.match(body, /showCopyNotice\("terminal\.copyNotice\.failed"\)/, "a failed write reports");
  // The success notice fires only after the clipboard write resolved, so
  // "Copied" can never be shown for a write that did not land.
  assert.ok(
    body.indexOf("await writeSystemClipboard") < body.indexOf('showCopyNotice("terminal.copyNotice.copied")'),
    "the notice follows the write",
  );
  // The copy-on-select gesture calls that same function...
  assert.match(view, /if \(finishedSelection && selectCopy\) void copySelection\(\);/);
  // ...and the explicit shortcut does too, so both share the notice.
  const keyboard = stripJsComments(await read("src/hooks/useAppKeyboard.ts"));
  assert.match(keyboard, /copySelection\(\);/);
  assert.doesNotMatch(keyboard, /showCopyNotice/, "the shortcut must not report on its own");
  // The row is painted by the panel, so the notice state is App-owned and
  // handed to the view that triggers it.
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /const \{ copyNotice, showCopyNotice \} = useCopyNotice\(\);/);
  assert.match(app, /showCopyNotice,\n\s*t,\n\s*\}\);/);
});

test("R44/R55 · the notice is a status line, never a floating toast", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(app, /className="terminal-copy-notice"/);
  assert.match(app, /data-phase=\{copyNotice\.phase\}/);
  // Announced like the panel's other status lines, and only the text is
  // conditional — the row itself is always mounted.
  assert.match(
    app,
    /className="terminal-copy-notice"[\s\S]{0,160}role="status"[\s\S]{0,80}aria-live="polite"/,
  );
  assert.match(app, /\{copyNotice\.message && \(/);

  const css = stripJsComments(await read("src/styles/terminal.css"));
  const row = bodyFor(css, ".terminal-copy-notice");
  // R55 · it *is* a layer now — but a bottom-anchored one pinned inside the
  // canvas region, not a floating toast: no radius, no shadow, and it spends
  // no accent budget.
  assert.match(row, /position:\s*absolute/);
  assert.match(row, /bottom:\s*0/);
  assert.match(row, /height:\s*var\(--terminal-status-height\)/);
  assert.match(row, /color:\s*var\(--text-muted\)/);
  assert.doesNotMatch(row, /accent/);
  assert.doesNotMatch(row, /border-radius/);
  assert.doesNotMatch(row, /box-shadow/);
  // It takes no pointer events, so the canvas behind the panel is untouched.
  assert.match(row, /pointer-events:\s*none/);
});

test("R44/R55 · the notice is out of the flow, so the PTY is never resized by a copy", async () => {
  const css = stripJsComments(await read("src/styles/terminal.css"));
  const body = bodyFor(css, ".terminal-panel__body");
  assert.match(body, /display:\s*flex/);
  assert.match(body, /flex-direction:\s*column/);
  // One declaration of the strip's height, shared by the notices that stack
  // above it — so the row cannot silently grow and shrink the canvas.
  assert.match(body, /--terminal-status-height:\s*\d+px/);
  const mount = bodyFor(css, ".terminal-panel__mount");
  assert.match(mount, /flex:\s*1 1 auto/);
  assert.match(mount, /min-height:\s*0/);
  assert.doesNotMatch(mount, /height:\s*100%/, "the mount takes the leftover height now");
  // R55 · the notice has no `flex` of its own: it is absolutely positioned, so
  // toggling a message cannot change the mount's height (the R44 contract).
  assert.doesNotMatch(bodyFor(css, ".terminal-copy-notice"), /flex:\s*0 0 auto/);
  // The resident exit notice keeps its inset from the strip's top edge rather
  // than being painted over it.
  const resident = bodyFor(css, ".terminal-resident");
  assert.match(resident, /bottom:\s*calc\(var\(--terminal-status-height\) \+ 12px\)/);
  // The text's opacity is the only thing the phases move.
  assert.match(bodyFor(css, ".terminal-copy-notice__text"), /opacity:\s*0/);
  assert.match(
    bodyFor(css, '.terminal-copy-notice[data-phase="visible"] .terminal-copy-notice__text'),
    /opacity:\s*1/,
  );
  assert.match(
    bodyFor(css, '.terminal-copy-notice[data-phase="fade"] .terminal-copy-notice__text'),
    /opacity:\s*0/,
  );
});

// ── 4 · i18n ──────────────────────────────────────────────────────────────

test("R44 · both notices are translated in both languages", async () => {
  const source = await read("src/i18n.ts");
  const en = source.slice(source.indexOf("const en = {"), source.indexOf("export type MessageKey"));
  const zh = source.slice(
    source.indexOf("const zh: Record<MessageKey, string> = {"),
    source.indexOf("const messages: Record<Language"),
  );
  for (const key of ["terminal.copyNotice.copied", "terminal.copyNotice.failed"]) {
    assert.ok(en.includes(`"${key}":`), `en must declare ${key}`);
    assert.ok(zh.includes(`"${key}":`), `zh must declare ${key}`);
  }
  // Translated, not copied: the two languages must differ for each key.
  const valueOf = (dictionary: string, key: string) =>
    new RegExp(`"${key.replace(/\./g, "\\.")}":\\s*"([^"]*)"`).exec(dictionary)?.[1];
  for (const key of ["terminal.copyNotice.copied", "terminal.copyNotice.failed"]) {
    const english = valueOf(en, key);
    const chinese = valueOf(zh, key);
    assert.ok(english && chinese && english !== chinese, `${key} must be translated`);
  }
});
