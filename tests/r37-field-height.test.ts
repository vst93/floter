// R37 · the search field is one surface at one height, and the clipboard is an
// ordinary result contributor.
//
// Two reports carry the round. The first, verbatim: 「现在搜索页面中 剪切板这项被
// 固定放到末尾，并且总是显示，还固定为了 cmd+0，这不对，它（其他内置插件也是）
// 不应该是个特例，应该和其他项一样匹配了才显示，同时快捷键也要按顺序安排」.
// The second: 「还有头部的输入框整体高度小了些，可以和设置页面头部一样高，这样切换
// 时一体性更好，包括其他内置插件的数据框高度应该都保持一致，他们应该是在主搜索框
// 基础上演变，应该公用一些组件，做到更强的一体性和平台化」.
//
// The de-specialization half is pinned by `launcher-ten-rows.test.ts` and
// `r34-viewport-badges.test.ts` (no reserved slot, no appended tail, pure
// ordering). This file pins the *platform* half: the one height the launcher
// field row, every plugin mode's field and the settings card's header now
// share, and the fact that the sheets and the budget module read it from one
// constant.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  LAUNCHER_ROW_CHROME_UNITS,
  LAUNCHER_WINDOW_HEIGHT,
  LAUNCHER_WINDOW_HEIGHT_UNITS,
  launcherRowUnits,
  MAX_RESULTS,
  ROW_HEIGHT_TWO_LINE,
} from "../src/launcher/result-budget.ts";
import {
  SEARCH_FIELD_HEIGHT_UNITS,
  SEARCH_FIELD_LINE_UNITS,
  searchFieldBreathUnits,
} from "../src/launcher/search-field.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const rule = (css: string, selector: string) => {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, "s").exec(css);
  assert.ok(match, `${selector} must exist`);
  return match![1];
};

const units = (value: string, property: string) => {
  const match = /calc\(var\(--u\)\s*\*\s*(\d+(?:\.\d+)?)\)/.exec(value);
  assert.ok(match, `${property} must be expressed in units, got "${value}"`);
  return Number(match![1]);
};

// ── 1 · the one height, in three places ───────────────────────────────────

test("the launcher field row and the settings header are the same 56u band", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const settings = stripComments(await read("src/styles/settings.css"));

  const row = units(
    /min-height:\s*([^;]+);/.exec(rule(launcher, ".collapsed-card__input-row"))![1],
    "launcher field row min-height",
  );
  const header = units(
    /height:\s*([^;]+);/.exec(rule(settings, ".settings-card__header"))![1],
    "settings card header height",
  );

  assert.equal(row, SEARCH_FIELD_HEIGHT_UNITS, "the launcher row is the shared band");
  assert.equal(header, SEARCH_FIELD_HEIGHT_UNITS, "…and so is the settings header");
  assert.equal(row, header, "switching surfaces must not move the window's first band");
  assert.equal(SEARCH_FIELD_HEIGHT_UNITS, 56, "the band is 56u at the default interface step");
});

test("the band grows around the field's pinned line box", () => {
  assert.equal(SEARCH_FIELD_LINE_UNITS, 22, "the field's own box is the pinned 22u");
  assert.equal(
    searchFieldBreathUnits(),
    17,
    "the extra 14u is 7u a side, because the row flex-centres the line box",
  );
  assert.equal(
    searchFieldBreathUnits(42),
    10,
    "R23's 42u row left 10u a side — the value R37 deliberately restores from",
  );
  assert.equal(
    searchFieldBreathUnits() - searchFieldBreathUnits(42),
    7,
    "the difference is exactly the per-side 7u the row gained",
  );
});

test("the field's own box did not move with the band", async () => {
  const launcher = stripComments(await read("src/styles/launcher.css"));
  const input = rule(launcher, ".collapsed-card__input");
  assert.equal(
    units(/min-height:\s*([^;]+);/.exec(input)![1], "field min-height"),
    SEARCH_FIELD_LINE_UNITS,
    "the line box is still 22u — only the row around it grew",
  );
  assert.match(input, /padding:\s*0;/, "and it is still the element's own box (no UA padding)");
  const row = rule(launcher, ".collapsed-card__input-row");
  assert.match(row, /align-items:\s*center/, "the row's centring is what makes the extra height symmetric");
});

// ── 2 · the budget reads the same constant ────────────────────────────────

test("the window budget's first segment is the shared field height", () => {
  // The row-count chrome is field + breath 4 + panel top 4 + panel tail 2.
  assert.equal(
    LAUNCHER_ROW_CHROME_UNITS,
    SEARCH_FIELD_HEIGHT_UNITS + 10,
    "the field row is the first segment of every row count's height",
  );
  assert.equal(LAUNCHER_ROW_CHROME_UNITS, 66);
  // The full slab: 66 + 10×42 + 45 (action bar) = 531u.
  assert.equal(launcherRowUnits(MAX_RESULTS, true), 531);
  assert.equal(LAUNCHER_WINDOW_HEIGHT_UNITS, 531);
  assert.equal(
    LAUNCHER_WINDOW_HEIGHT_UNITS,
    LAUNCHER_ROW_CHROME_UNITS + MAX_RESULTS * ROW_HEIGHT_TWO_LINE + 45,
    "the slab is the row-count table's top, segment for segment",
  );
  assert.equal(LAUNCHER_WINDOW_HEIGHT, 583, "531u + 52px at the default interface step");
});

test("a field that grows without the slab would be a card taller than its window", () => {
  // The invariant the shared constant buys: if the sheets' band moved and the
  // budget did not, `launcherRowUnits(MAX_RESULTS)` would no longer equal
  // `LAUNCHER_WINDOW_HEIGHT_UNITS`. This is the arithmetic the pin protects.
  const withR37Field = SEARCH_FIELD_HEIGHT_UNITS + 10 + MAX_RESULTS * ROW_HEIGHT_TWO_LINE + 45;
  assert.equal(withR37Field, LAUNCHER_WINDOW_HEIGHT_UNITS);
  const withR23Field = 42 + 10 + MAX_RESULTS * ROW_HEIGHT_TWO_LINE + 45;
  assert.equal(withR23Field, 517, "R23-R36's slab — the 14u R37 adds is exactly the field's growth");
});

// ── 3 · one field for the ordinary page and every plugin mode ─────────────

test("the ordinary launcher and a plugin mode render the one field row", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  // R31 already made the plugin mode a *state* rather than a second query
  // surface, so the field is one element; R37's report asks for exactly that
  // ("他们应该是在主搜索框基础上演变"). The scope glyph is inside the same row,
  // and there is no second field anywhere in the App.
  assert.equal(
    (app.match(/className="collapsed-card__input-row"/g) ?? []).length,
    1,
    "one field row for the ordinary page and every plugin mode",
  );
  assert.equal(
    (app.match(/className="collapsed-card__input"/g) ?? []).length,
    1,
    "one `<input>` — the mode swaps the row's furniture, not the field",
  );
  assert.match(
    app,
    /launcherScope && \(\s*<span className="collapsed-card__scope"/,
    "the plugin scope glyph lives inside that one row",
  );
  // The trailing controls are the mode's difference: two launcher buttons
  // outside a scope, the plugin's one gear inside it.
  assert.match(app, /collapsed-card__settings--plugin/, "the plugin gear is the in-scope trailing control");
});

test("the sheets name the shared constant, so a later edit faces it", async () => {
  // A comment is not a mechanism, but it is where the next reader looks. Both
  // sheets must point at `search-field.ts` beside the number they write, so the
  // three cannot be edited apart without the reviewer seeing the link.
  const launcher = await read("src/styles/launcher.css");
  const settings = await read("src/styles/settings.css");
  assert.match(launcher, /R37[\s\S]{0,400}?search-field\.ts/, "launcher.css names the shared module");
  assert.match(settings, /search-field\.ts/, "settings.css names the shared module");
  assert.match(
    await read("src/launcher/result-budget.ts"),
    /from "\.\/search-field\.ts"/,
    "the budget imports the constant rather than restating 56",
  );
});
