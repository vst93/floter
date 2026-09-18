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
  // CLIP-DRAG (r2): the row no longer has ANY pseudo-element. CLIP-DISSOLVE had
  // already removed the separator; the aura was the last band-shaped thing on
  // the row and the user's follow-up (「头部的这种显示的菜单栏不能要」) took it too.
  // This pins the forward-looking fact: nothing above the list may paint a band.
  assert.ok(
    !/clipboard-panel__topbar::/.test(css),
    "the topbar must have no pseudo-element at all — no separator, no aura",
  );
});

test("focus is not repainted on the row: no focus-within face and no aura pseudo-element", async () => {
  const css = await page();
  const focus = rule(css, ".clipboard-panel__topbar:focus-within");
  assert.equal(
    focus,
    undefined,
    "the topbar must not repaint itself on focus — there is no band left to light",
  );
  assert.equal(
    rule(css, ".clipboard-panel__topbar::before"),
    undefined,
    "the focus aura was retired in CLIP-DRAG: focus is the caret and the field now",
  );
});

// ── B · the header is not a bar: no aura, an implicit drag handle ────────

test("the header rows are the window's implicit drag handle, not a band", async () => {
  const css = await page();
  const topbar = rule(css, ".clipboard-panel__topbar");
  assert.ok(topbar, "page.css must still define the topbar row");
  // The terminal/launcher affordance for "you can drag this".
  assert.equal(decl(topbar!.body, "cursor"), "grab", "the header must advertise the drag affordance");
  // Nothing on the row paints material or a band. The panel's own material is
  // the only surface and the rows sit directly on it.
  assert.ok(!paintsFill(topbar!.body), "the topbar must not paint a fill");
  assert.ok(!paintsEdge(topbar!.body), "the topbar must not draw an edge");
  assert.ok(!paintsBoxShadow(topbar!.body), "the topbar must not float");
  // The page's own aura token is gone with the aura, in BOTH palettes.
  assert.ok(
    !/--accent-wash\s*:/.test(css),
    "--accent-wash must be retired with the aura it fed",
  );
});

test("the drag handle is exempt from every interactive control on the page", async () => {
  const list = stripComments(await read("src/clipboard-list.ts"));
  const exempt = list.match(/CLIPBOARD_DRAG_EXEMPT_SELECTOR = \[([\s\S]*?)\]\.join/);
  assert.ok(exempt, "clipboard-list.ts must declare the drag exemption set");
  // The host's own `startDrag` guard, carried onto the page: a press on any of
  // these must never move the window.
  for (const needle of ["button", "input", "textarea", "select", "a", "summary", "[role='option']", "[data-no-drag]"]) {
    assert.ok(exempt![1].includes(needle), `the drag guard must exempt \`${needle}\``);
  }
  // The list scroller opts out so its scrollbar press scrolls, not drags.
  const pageTs = await read("src/plugins/clipboard/main.ts");
  assert.match(pageTs, /clipboard-panel__content" data-no-drag/, "the scroller must opt out of the drag");
});

test("the page asks the host to drag over the bridge, and the host routes it to its own startDrag", async () => {
  const pageTs = await read("src/plugins/clipboard/main.ts");
  // Page side: a payload-free `drag` message, sent only for a non-interactive
  // press (the guard runs first).
  assert.match(pageTs, /requestWindowDrag/, "the page must own the drag request");
  assert.match(
    pageTs,
    /window\.parent\.postMessage\(\{ \[BRIDGE_TAG\]: "drag" \}, "\*"\)/,
    "the drag message must be payload-free",
  );
  assert.match(
    pageTs,
    /if \(!isClipboardDragTarget\(event\.target as Element \| null\)\) return;/,
    "the interactive-element guard must run before the message is sent",
  );
  // Host side: the new type is recognized and routed to the same drag the
  // shells use.
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /isBridgeDrag\(data\)/, "the host must recognize the drag message");
  assert.match(host, /windowDragRef\.current\(\)/, "the host must run its drag handler");
  // The handler is App's own `beginDrag`, the body every shell's mousedown
  // funnels through, so the Windows blur-grace is not duplicated.
  const app = await read("src/App.tsx");
  assert.match(app, /onWindowDrag=\{beginDrag\}/, "App must pass the shared drag body to the host");
  assert.match(app, /const beginDrag = useCallback\(\(\) => \{/, "the drag body must be one shared callback");
  assert.match(app, /invoke\("start_drag"\)/, "the body must still invoke the OS drag");
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
