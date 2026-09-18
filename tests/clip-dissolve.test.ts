// CLIP-DISSOLVE · the clipboard page's top band dissolves into the panel.
//
// The reviewer's finding, verbatim: "剪切板页面加个头部干什么 … 我们是一个悬浮型
// 的窗口化应用，你搞个这种头部状态栏很奇葩。应该做得更优雅，就和剪切板还有搜索框
// 页面一样". The clipboard page had grown a 56px `.clipboard-panel__topbar` that
// painted its own two-layer gradient fill, drew a `border-bottom: 1px solid
// var(--hairline)` and turned that edge accent-blue on focus — a status bar
// glued to the top of a floating glass window.
//
// The launcher's own reference is `.collapsed-card__input-row` +
// `.collapsed-card__aura`: the field is borderless text sitting directly on the
// card's one material, and focus is an accent wash fading in from the leading
// edge, never a new edge. This round makes the clipboard row that same thing.
//
// These assertions read the source (the node suite has no DOM, and the page
// imports CSS) and are written as predicates so the two mutation locks at the
// bottom can prove they go red on the exact re-additions the round removed.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};
const rule = (css: string, selector: string) =>
  rules(css).find((r) => r.selector === selector);
/** The value of one declaration in a rule body, or null. */
const decl = (body: string, prop: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${prop}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;
/** Whether a body paints any fill at all (background / background-color /
 * background-image), as opposed to only setting `background: transparent`. */
const paintsFill = (body: string) => {
  const value = decl(body, "background");
  const color = decl(body, "background-color");
  const image = decl(body, "background-image");
  const isTransparent = (v: string | null) => v === null || /^(transparent|none)$/.test(v);
  return !isTransparent(value) || !isTransparent(color) || !isTransparent(image);
};
/** Whether a body draws any edge. `border: 0` / `border: none` is the
 * borderless form and does not count. */
const paintsEdge = (body: string) => {
  for (const prop of ["border", "border-bottom", "border-top", "border-left", "border-right", "border-color", "border-width", "border-style"]) {
    const value = decl(body, prop);
    if (value === null) continue;
    if (/^(0|none)(\s|$)/.test(value)) continue;
    return true;
  }
  return false;
};
/** Whether a body paints any fill at all, counting gradients and shadows. */
const paintsBoxShadow = (body: string) => {
  const value = decl(body, "box-shadow");
  return value !== null && !/^none$/.test(value);
};

const page = async () => stripComments(await read("src/plugins/clipboard/page.css"));
const launcher = async () => stripComments(await read("src/styles/launcher.css"));

// ── A · the band is gone ──────────────────────────────────────────────────

test("the topbar is a layout row, not a band: no fill, no edge, no shadow", async () => {
  const css = await page();
  const topbar = rule(css, ".clipboard-panel__topbar");
  assert.ok(topbar, "page.css must still define the topbar row");
  assert.ok(
    !paintsFill(topbar!.body),
    `the topbar must not paint a fill of its own, got background: ${decl(topbar!.body, "background")}`,
  );
  assert.ok(
    !paintsEdge(topbar!.body),
    "the topbar must not draw an edge of its own — the panel's own radius is the only shape",
  );
  assert.ok(!paintsBoxShadow(topbar!.body), "the topbar must not float");
  // The row is still a row: the height and the flex basis survive as layout.
  assert.match(topbar!.body, /height:\s*56px/);
  assert.match(topbar!.body, /flex:\s*0 0 56px/);
});

test("the topbar carries no separator pseudo-element and never regains one", async () => {
  const css = await page();
  // REVIEW-CORRECTED (r1): page.css NEVER had a `::after` — the separator band
  // lived only in the dead host mirror (terminal.css). Asserting its absence in
  // the page sheet was vacuous (true at HEAD too). The live band sources are the
  // topbar's own fill/edge, asserted by the two tests above; this test now only
  // pins the forward-looking fact: no topbar pseudo-element may appear later.
  assert.ok(
    !/clipboard-panel__topbar::(after|before(?!-))/.test(css.replace(/\.clipboard-panel__topbar::before/g, "")),
    "no topbar ::after may appear; ::before is reserved for the aura",
  );
});

test("focus is an aura, not an accent edge: no focus-within face on the row", async () => {
  const css = await page();
  const focus = rule(css, ".clipboard-panel__topbar:focus-within");
  assert.equal(
    focus,
    undefined,
    "the topbar must not repaint itself on focus — the aura pseudo-element carries the state",
  );
});

// ── B · the aura matches the launcher's recipe ────────────────────────────

test("the focus aura is the launcher's exact gradient, faded in from the leading edge", async () => {
  const css = await page();
  const aura = rule(css, ".clipboard-panel__topbar::before");
  assert.ok(aura, "the topbar must own an aura pseudo-element");
  const gradient = decl(aura!.body, "background");
  assert.equal(
    gradient,
    "linear-gradient(90deg, var(--accent-wash), transparent 42%)",
    "the aura must be the .collapsed-card__aura recipe verbatim",
  );
  assert.equal(decl(aura!.body, "opacity"), "0", "the aura rests invisible");
  assert.equal(decl(aura!.body, "pointer-events"), "none", "the aura must not eat clicks or drags");
  const lit = rule(css, ".clipboard-panel__topbar:focus-within::before");
  assert.ok(lit, "the focus state must light the aura");
  assert.equal(decl(lit!.body, "opacity"), "1", "focus fades the aura fully in");
});

test("the aura recipe is byte-identical to the launcher's", async () => {
  const [pageCss, launcherCss] = await Promise.all([page(), launcher()]);
  const recipe = (body: string) => decl(body, "background");
  assert.equal(
    recipe(rule(pageCss, ".clipboard-panel__topbar::before")!.body),
    recipe(rule(launcherCss, ".collapsed-card__aura")!.body),
    "the two aura gradients must not drift apart",
  );
  // REVIEW-NIT-2 (r1): the easing drifted too (page had ease-out, launcher ease).
  // Pin the full transition curve so the motion matches, not just the duration.
  const transition = (body: string) => decl(body, "transition");
  assert.match(
    transition(rule(pageCss, ".clipboard-panel__topbar::before")!.body)!,
    /opacity\s+160ms\s+ease(?!-)/,
    "aura easing must be the launcher's `ease`, not a different curve",
  );
});

test("the aura's wash token exists in both palettes and is visible on light", async () => {
  const css = await page();
  const rootWash = css.match(/:root\s*\{[^}]*--accent-wash:\s*([^;]+);/)?.[1].trim();
  const lightWash = css.match(/\[data-theme="light"\]\s*\{[^}]*--accent-wash:\s*([^;]+);/)?.[1].trim();
  assert.ok(rootWash, "the dark palette must declare --accent-wash");
  assert.ok(lightWash, "the light palette must declare --accent-wash");
  // The light wash carries more alpha, exactly as base.css's light block does:
  // a blue laid over near-white barely moves the pixel at the dark alpha.
  const alpha = (value: string) => Number(value.match(/,\s*([\d.]+)\s*\)$/)?.[1] ?? NaN);
  assert.ok(
    alpha(lightWash!) > alpha(rootWash!),
    `the light wash must be stronger than the dark one (dark ${rootWash}, light ${lightWash})`,
  );
  assert.equal(lightWash, "rgba(7, 94, 216, 0.085)", "the light wash must derive from this page's own light accent (#075ed8) at the launcher's light alpha");
});

// ── C · the field is launcher-shaped ──────────────────────────────────────

test("the search field is borderless and transparent, like the launcher's", async () => {
  const [pageCss, launcherCss] = await Promise.all([page(), launcher()]);
  const search = rule(pageCss, ".clipboard-panel__search");
  const cardInput = rule(launcherCss, ".collapsed-card__input");
  assert.ok(search, "page.css must define the filter field");
  assert.ok(cardInput, "launcher.css must define its input");
  for (const [name, body] of [["page search", search!.body], ["launcher input", cardInput!.body]] as const) {
    assert.ok(!paintsEdge(body), `${name} must be borderless`);
    assert.equal(decl(body, "background"), "transparent", `${name} must sit on the material, not in a box`);
    assert.equal(decl(body, "box-shadow"), null, `${name} must not float in a box`);
    // The launcher's field inherits the host's global `input:focus { outline:
    // none }`; the plugin page is a separate document and restates it. Either
    // way the resolved state is "no boxed outline".
    const outline = decl(body, "outline");
    assert.ok(
      outline === null || /^none$/.test(outline),
      `${name} must not draw a boxed outline, got ${outline}`,
    );
  }
  // The two share the launcher's type step and weight — the row is one voice.
  assert.equal(decl(search!.body, "font-size"), "17px");
  assert.equal(decl(search!.body, "font-weight"), "460");
});

test("every control in the row is lifted above the aura and eats no pointer events", async () => {
  const css = await page();
  for (const selector of [
    ".clipboard-panel__prompt",
    ".clipboard-panel__search",
    ".clipboard-panel__filter-clear",
    ".clipboard-panel__tabs",
  ]) {
    const r = rule(css, selector);
    assert.ok(r, `page.css must define ${selector}`);
    assert.equal(decl(r!.body, "position"), "relative", `${selector} must establish a stacking context above the aura`);
    assert.equal(decl(r!.body, "z-index"), "1", `${selector} must sit above the aura`);
  }
});

test("the filter-clear × and the 全部/收藏 tabs ride the same material (no band dependency)", async () => {
  const css = await page();
  const tabs = rule(css, ".clipboard-panel__tabs");
  const clear = rule(css, ".clipboard-panel__filter-clear");
  assert.ok(tabs, "the tabs must still be styled");
  assert.ok(clear, "the clear × must still be styled");
  // Neither may reach for a topbar-relative band: their own boxes are the
  // whole story now.
  for (const [name, body] of [["tabs", tabs!.body], ["clear", clear!.body]] as const) {
    assert.ok(!/topbar/.test(body), `${name} must not depend on a topbar band`);
    assert.ok(!/border-bottom/.test(body), `${name} must not carry a band edge`);
  }
  // The recorded divergence stays recorded: the host mirror is a pill
  // (ROUND-PASS), the live page still uses the page's own radii. This test
  // pins the *fact* of the divergence so it cannot silently become an
  // accident — see the report's honest-gaps list.
  const host = stripComments(await read("src/styles/terminal.css"));
  assert.match(rule(host, ".clipboard-panel__tabs")!.body, /border-radius:\s*999px/);
  assert.match(tabs!.body, /border-radius:\s*7px/);
});

// ── D · the neighbours do not regress ─────────────────────────────────────

test("the footer keeps its own gradient hairline — only the topband was dissolved", async () => {
  const css = await page();
  const footer = rule(css, ".clipboard-panel__footer");
  assert.ok(footer, "the footer must still be styled");
  assert.match(footer!.body, /border-top:\s*1px solid var\(--hairline\)/);
});

test("the panel keeps its one sheet of material and the list its scroll edge", async () => {
  const css = await page();
  const panel = rule(css, ".clipboard-panel");
  assert.match(panel!.body, /var\(--panel-bg\)/, "the panel is still the one material");
  // The list scrolls under the row rather than under a band: the content
  // scroller and its padding are unchanged, so the first row still clears the
  // row's height.
  const content = rule(css, ".clipboard-panel__content");
  assert.match(content!.body, /overflow-y:\s*auto/);
});

test("the empty/failure states and the tab badge are untouched by the dissolve", async () => {
  const css = await page();
  for (const selector of [
    ".clipboard-panel__empty-title",
    ".clipboard-panel__empty-actions",
    // GLASS-CLIP-2 renamed the tab badge (`.clipboard-panel__tab-count` →
    // `__type-count`) when the single 全部/收藏 strip became a type bar with
    // its own count slot. Same role, new name — the dissolve is still not what
    // touched it, which is what this assertion pins.
    ".clipboard-panel__type-count",
    ".clipboard-panel__hints",
  ]) {
    assert.ok(rule(css, selector), `page.css must still define ${selector}`);
  }
});

// ── E · mutation locks ────────────────────────────────────────────────────

test("mutation lock: a re-added topbar border-bottom goes red", async () => {
  const css = await page();
  const mutated = css.replace(
    /(\.clipboard-panel__topbar \{)/,
    "$1\n  border-bottom: 1px solid var(--hairline);",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  const body = rule(mutated, ".clipboard-panel__topbar")!.body;
  assert.ok(paintsEdge(body), "the borderless predicate must reject a re-added border-bottom");
});

test("mutation lock: a re-added topbar fill goes red", async () => {
  const css = await page();
  const mutated = css.replace(
    /(\.clipboard-panel__topbar \{)/,
    "$1\n  background: rgba(255, 255, 255, 0.018);",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  const body = rule(mutated, ".clipboard-panel__topbar")!.body;
  assert.ok(paintsFill(body), "the fill predicate must reject a re-added band fill");
});

test("mutation lock: a re-boxed search field goes red", async () => {
  const css = await page();
  const mutated = css.replace(
    /(\.clipboard-panel__search \{[^}]*?border:\s*)0/,
    "$1 1px solid var(--hairline)",
  );
  assert.notEqual(mutated, css, "the mutation must land");
  const body = rule(mutated, ".clipboard-panel__search")!.body;
  assert.ok(paintsEdge(body), "the borderless predicate must reject a boxed search field");
});
