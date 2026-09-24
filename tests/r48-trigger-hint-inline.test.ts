// R48 · the trigger nudge moves from a subline into the field row.
//
// R43 drew the ordinary search page's "space enters this plugin" hint as its
// own subline, sharing the chips row's `.launcher-filter` band, and charged the
// window for that band. The user read the line back as one the window should
// not be paying for — 「还有输入框下方，提示"空格进入某个具体插件命令"的内容，放在
// 输入框下占用了一行，似乎不太合适。能不能在输入框当中做些标识或者提示」.
//
// R48 keeps the *judgement* (`externalTriggerHint`) and the *transition*
// (`externalPluginModeEntry`) exactly as they were and moves only the drawing:
// the nudge becomes a shrinkable trailing element of `.collapsed-card__input-row`
// (the R37 56u band), so it costs no `launcherContentHeight` term. CSS ellipsises
// it; `useTriggerHintFit` removes it entirely when the row leaves it less than a
// few `ch`, because a stylesheet cannot ask "is this less than a word?".
//
// Mutations that must turn this file red:
//   * `launcherSubline` (or any `triggerHint` term) coming back into the height
//     call -> the "no band, no resize" assertion fails;
//   * the hint moving back under the field (`.launcher-filter--trigger-hint` /
//     `.launcher-trigger-hint`) -> the inline assertions fail;
//   * the overflow guard (`min-width: 0` / `text-overflow`) dropping off -> the
//     sheet assertion fails;
//   * `triggerHintFits` losing its floor -> the pure boundary test fails.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  TRIGGER_HINT_MIN_CELLS,
  triggerHintFits,
} from "../src/launcher/trigger-hint.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const stripCssComments = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "");

// ── 1 · the fit rule ──────────────────────────────────────────────────────

test("R48 · the hint keeps at least six cells, or none", () => {
  const cell = 6; // px, as one `ch` at the hint's own font
  const floor = TRIGGER_HINT_MIN_CELLS * cell;
  assert.equal(floor, 36);
  assert.equal(triggerHintFits(floor, cell), true, "exactly at the floor: kept");
  assert.equal(triggerHintFits(floor + 1, cell), true);
  assert.equal(triggerHintFits(floor - 1, cell), false, "one pixel under: gone");
  assert.equal(triggerHintFits(0, cell), false);
  // A font that could not be measured keeps the nudge: CSS is already
  // ellipsising it, and a wrong hide would lose the nudge for good.
  assert.equal(triggerHintFits(0, 0), true);
  assert.equal(triggerHintFits(3, Number.NaN), true);
  // The floor is configurable so the boundary is one number, not two.
  assert.equal(triggerHintFits(12, cell, 2), true);
  assert.equal(triggerHintFits(11, cell, 2), false);
  assert.equal(triggerHintFits(11, cell, 3), false);
});

// ── 2 · the render: inline, not a subline ─────────────────────────────────

test("R48 · the hint renders inline in the field row and pays no band", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // The nudge is a child of the field row, drawn from the rendered string.
  assert.match(app, /className="collapsed-card__trigger-hint"/);
  assert.match(app, /collapsed-card__trigger-hint-key/);
  assert.match(app, /collapsed-card__trigger-hint-text/);
  // The subline variant is gone: no `.launcher-filter` band, no subline charge.
  assert.doesNotMatch(app, /launcher-filter--trigger-hint/);
  assert.doesNotMatch(app, /launcher-trigger-hint/);
  assert.doesNotMatch(app, /launcherSubline/);
  // The height call charges the plugin filter row alone; the hint is not a term
  // of it, so a nudge appearing or vanishing cannot move the window.
  assert.match(
    app,
    /const launcherHeight = launcherContentHeight\(\s*launcherHeldUnits,\s*launcherRows,\s*launcherScale,\s*launcherMaxHeight,\s*launcherHasBar,\s*launcherSectionTitle,\s*filterRowVisible,\s*\)/,
    "the window height reads the filter-row predicate, never the hint",
  );
  assert.doesNotMatch(app, /triggerHint[^;]*launcherContentHeight/);
  // The fit rule is measured through the one hook.
  assert.match(app, /from "\.\/hooks\/useTriggerHintFit"/);
  assert.match(app, /useTriggerHintFit\(\s*triggerHintRef,\s*triggerHintText,\s*settings\.ui_scale,\s*\)/);
});

test("R48 · the inline hint is a visual nudge: aria-hidden, no live region", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /className="collapsed-card__trigger-hint"\s+aria-hidden="true"/,
    "the combobox already carries the field's meaning; the hint must not double-announce",
  );
  assert.doesNotMatch(app, /launcher-trigger-hint[\s\S]{0,80}role="status"/);
});

// ── 3 · the stylesheet ────────────────────────────────────────────────────

test("R48 · the hint is a shrinkable caption that ellipsises its name", async () => {
  const css = stripCssComments(await read("src/styles/launcher.css"));
  assert.doesNotMatch(css, /\.launcher-trigger-hint/);
  const rule = css.match(/\.collapsed-card__trigger-hint \{([\s\S]*?)\}/);
  assert.ok(rule, "the inline hint has its own rule");
  const body = rule![1];
  assert.match(body, /flex: 0 1 auto;/, "the nudge gives ground when the row cannot hold it");
  // The field is the grower (`flex: 1 1 0`), so its old `width: 100%` basis —
  // which ate the flex shrink and truncated the hint while the field's empty
  // tail had room — cannot come back.
  const input = css.match(/\.collapsed-card__input \{([\s\S]*?)\}/);
  assert.ok(input, "the field has its own rule");
  assert.match(input![1], /flex: 1 1 0;/, "the field fills the row and yields to the hint only when it must");
  assert.doesNotMatch(input![1], /width: 100%/);
  assert.match(body, /min-width: 0;/, "without the floor the ellipsis cannot engage");
  assert.match(body, /max-width: 50%;/, "a long name never owns the row");
  assert.match(body, /font-size: var\(--text-caption\)/, "one step under the field, and it scales");
  assert.match(body, /color: var\(--text-muted\)/, "muted, never accent");
  assert.match(
    css,
    /\.collapsed-card__trigger-hint-text \{[\s\S]*?min-width: 0;[\s\S]*?text-overflow: ellipsis;/,
    "the name is the part that is cut",
  );
});

// ── 4 · the i18n keys survive, with the wording shortened ─────────────────

test("R48 · the hint keys still name the command, led by it for the ellipsis", async () => {
  const i18n = await read("src/i18n.ts");
  assert.match(i18n, /"launcher\.triggerHint": "\{name\} · enter"/);
  assert.match(i18n, /"launcher\.triggerHintMore": "\{name\} · enter \(\+\{count\} more\)"/);
  assert.match(i18n, /"launcher\.triggerHint": "\{name\} · 进入"/);
  assert.match(i18n, /"launcher\.triggerHintMore": "\{name\} · 进入（还有 \{count\} 个）"/);
});
