// CLIP-DRAG · the clipboard page stops being a bar-top window and becomes a
// floating one.
//
// The user's second report, verbatim: 「整体要有浮窗感，头部的这种显示的菜单栏不能
// 要，要类似终端页和搜索页的隐式菜单（拖动）栏」. Two changes answer it:
//
//   1. **No visible menu bar.** CLIP-DISSOLVE had already removed the top band's
//      fill and its separator edge; CLIP-DRAG removes the last band-shaped
//      thing on the header — the accent *aura* that lit on focus — and leaves
//      the prompt, the field and the tab strip sitting directly on the page's
//      one glass sheet. The header rows are layout, not a bar.
//
//   2. **An implicit drag handle.** The terminal page drags from its blank
//      header; the launcher drags from anywhere on its card. The clipboard page
//      could not, because it is a sandboxed iframe: a mousedown inside it never
//      reaches the host's `startDrag`. So the page reports the *intent* — a
//      payload-free `drag` bridge message, sent only after it has applied the
//      same interactive-element guard the host uses — and the host runs its one
//      drag body (`beginDrag`, the same one every shell's mousedown funnels
//      through, Windows blur-grace included).
//
// This suite covers the page-side guard (a pure function), the wire message and
// its host routing, and the visual de-banding. The two mutation locks at the
// bottom prove the routing and the de-banding predicates go red on the exact
// reversions the round removed.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
  BRIDGE_TAG,
  isBridgeDrag,
  shouldStartWindowDrag,
} from "../src/plugin-pages.ts";
import { CLIPBOARD_DRAG_EXEMPT_SELECTOR, isClipboardDragTarget } from "../src/clipboard-list.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const code = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};
const rule = (css: string, selector: string) =>
  rules(css).find((r) => r.selector === selector);
const decl = (body: string, prop: string) =>
  body.match(new RegExp(`(?:^|;)\\s*${prop}\\s*:\\s*([^;]+)`))?.[1].trim() ?? null;
const paintsFill = (body: string) => {
  const value = decl(body, "background");
  const color = decl(body, "background-color");
  const image = decl(body, "background-image");
  const transparent = (v: string | null) => v === null || /^(transparent|none)$/.test(v);
  return !transparent(value) || !transparent(color) || !transparent(image);
};
const paintsEdge = (body: string) => {
  for (const prop of ["border", "border-bottom", "border-top", "border-left", "border-right", "border-color"]) {
    const value = decl(body, prop);
    if (value === null || /^(0|none)(\s|$)/.test(value)) continue;
    return true;
  }
  return false;
};

/** A minimal `event.target` stub: `closest(selector)` answers non-null when the
 * element is "inside" any of `matches`, the way a real DOM node matches a
 * comma-joined selector list. */
const stubTarget = (matches: string[]) => ({
  closest: (selector: string) => {
    const parts = selector.split(",").map((part) => part.trim());
    return matches.some((match) => parts.includes(match)) ? {} : null;
  },
});

// ── 1 · the page-side interactive-element guard ───────────────────────────

test("a press on blank chrome may drag; every interactive element may not", () => {
  // Blank header / filter row / footer space: the press starts a drag.
  assert.equal(isClipboardDragTarget(stubTarget([])), true, "blank chrome must drag");
  // The host's own `startDrag` exemption list, carried onto the page.
  for (const exempt of ["button", "input", "textarea", "select", "a", "summary", "[role='dialog']", "[data-no-drag]"]) {    assert.equal(
      isClipboardDragTarget(stubTarget([exempt])),
      false,
      `a press inside \`${exempt}\` must not move the window`,
    );
  }
  // A row is its own press target (`role="option"`), unlike the host's bar: a
  // click on a row copies/selects, it never drags the window.
  assert.equal(isClipboardDragTarget(stubTarget(["[role='option']"])), false, "a row press must not drag");
});

test("the guard is a selector set, and the page joins it into one closest() call", () => {
  assert.ok(CLIPBOARD_DRAG_EXEMPT_SELECTOR.includes("button"), "the set must be a comma-joined selector list");
  assert.ok(CLIPBOARD_DRAG_EXEMPT_SELECTOR.includes("[data-no-drag]"), "the set must honour data-no-drag");
  // A press with no resolvable target is a *no*: a synthetic event cannot be
  // trusted to have landed on blank chrome.
  assert.equal(isClipboardDragTarget(null), false, "a missing target must not drag");
  assert.equal(isClipboardDragTarget(undefined), false, "an undefined target must not drag");
  // The page calls the predicate on `event.target` before it does anything.
  return read("src/plugins/clipboard/main.ts").then((page) => {
    assert.match(
      page,
      /if \(!isClipboardDragTarget\(event\.target as Element \| null\)\) return;/,
      "main.ts must guard on the pressed element before sending the drag",
    );
  });
});

// ── 2 · the wire message ──────────────────────────────────────────────────

test("the drag message is recognized, and a non-drag message is not", () => {
  assert.ok(isBridgeDrag({ [BRIDGE_TAG]: "drag" }), "a drag message must be recognized");
  for (const bad of [
    { [BRIDGE_TAG]: "close" },
    { [BRIDGE_TAG]: "invoke", id: 1, command: "c" },
    { [BRIDGE_TAG]: "opacity", mainOpacity: 1, terminalOpacity: 1 },
    null,
    undefined,
    {},
    "drag",
  ]) {
    assert.equal(isBridgeDrag(bad), false, `${JSON.stringify(bad)} must not be a drag`);
  }
});

test("the page sends one payload-free drag message, from its own postMessage helper", async () => {
  const page = await read("src/plugins/clipboard/main.ts");
  assert.match(page, /const requestWindowDrag = \(\) => \{/, "the page must own the drag request");
  assert.match(
    page,
    /window\.parent\.postMessage\(\{ \[BRIDGE_TAG\]: "drag" \}, "\*"\)/,
    "the message must be exactly `{ floter: 'drag' }` — nothing else crosses the sandbox",
  );
  // The press must be prevented from starting a text selection in the chrome,
  // exactly as the host's startDrag does.
  assert.match(page, /event\.preventDefault\(\);\s*\n\s*requestWindowDrag\(\)/, "the press must preventDefault");
  // Only the primary button drags.
  assert.match(page, /if \(event\.button !== 0\) return;/, "only the left button starts a drag");
  // The scroller opts out so a press on its scrollbar scrolls instead.
  assert.match(page, /clipboard-panel__content" data-no-drag/, "the list scroller must opt out");
});

// ── 3 · the host routes it to the one drag body ───────────────────────────

test("the host honours a drag only from the page that is the live surface", () => {
  const drag = { [BRIDGE_TAG]: "drag" };
  assert.equal(shouldStartWindowDrag(drag, true), true, "the live page's drag must fire");
  assert.equal(
    shouldStartWindowDrag(drag, false),
    false,
    "a kept-alive page hidden behind another one must not move the window on a stale press",
  );
  assert.equal(shouldStartWindowDrag({ [BRIDGE_TAG]: "close" }, true), false, "only a drag routes to the drag body");
  assert.equal(shouldStartWindowDrag(null, true), false, "a null payload must not route");
});

test("the host's drag routing is wired into the message listener and to App's shared drag body", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /isBridgeDrag\(data\)/, "the listener must recognize the drag message");
  assert.match(
    host,
    /shouldStartWindowDrag\(data, Boolean\(activeRef\.current\)\)/,
    "the listener must route through the pure predicate",
  );
  assert.match(host, /windowDragRef\.current\(\)/, "the listener must run the injected drag body");
  assert.match(host, /onWindowDrag: \(\) => void;/, "the host must declare the drag-body prop");
  assert.match(host, /const windowDragRef = useRef\(onWindowDrag\);/, "the body must be read through a ref");

  // App supplies the *same* body every shell's mousedown uses: one platform
  // path, one Windows blur-grace. A second inline `invoke("start_drag")` in the
  // host would be the regression.
  const app = await read("src/App.tsx");
  // R33 · App no longer mounts the host; the drag body stays the one shared
  // callback the shells' own mousedown funnels through.
  assert.ok(!/<PluginPageHost\b/.test(app), "the retired host must not be mounted");
  assert.match(app, /const beginDrag = useCallback\(\(\) => \{/, "the drag body must be one shared callback");
  assert.match(app, /invoke\("start_drag"\)/, "the body must still invoke the OS drag");
  // And the shells' own mousedown still funnels through it.
  assert.match(app, /const startDrag = \(event: React\.MouseEvent\) => \{[\s\S]*?beginDrag\(\);/, "startDrag must delegate");
  // Every `invoke("start_drag")` lives inside the one shared body — the two
  // calls are its Windows and non-Windows branches, not two entry points.
  const body = app.slice(app.indexOf("const beginDrag = useCallback"), app.indexOf("const startDrag ="));
  assert.equal(
    (app.match(/invoke\("start_drag"\)/g) ?? []).length,
    2,
    "the OS drag is invoked from the two platform branches",
  );
  assert.equal(
    (body.match(/invoke\("start_drag"\)/g) ?? []).length,
    2,
    "both branches must live inside the shared `beginDrag` body",
  );
  assert.ok(
    !/invoke\("start_drag"\)/.test(code(host)),
    "the host must not invoke the OS drag itself — it reuses App's body",
  );
});

test("the bridge protocol gained a message type without changing an existing one", async () => {
  const protocol = await read("src/plugin-pages.ts");
  // The new type is declared, recognized, and part of the page → host union.
  assert.match(protocol, /export type BridgeDrag = \{ \[BRIDGE_TAG\]: "drag" \};/);
  assert.match(protocol, /export type BridgeFromPage = [^;]*BridgeDrag[^;]*;/);
  // The message types the round promised not to touch are all still declared
  // with their original tags.
  for (const [name, tag] of [
    ["BridgeClose", "close"],
    ["BridgeNotify", "host-notify"],
    ["BridgeReload", "reload"],
    ["BridgeOpacity", "opacity"],
    ["BridgeTheme", "theme"],
    ["BridgeGlass", "glass"],
    ["BridgeVisibility", "visibility"],
  ] as const) {
    assert.match(protocol, new RegExp(`export type ${name} = [^;]*"${tag}"`), `${name} must keep its tag`);
  }
});

// ── 4 · no visible bar: the header is layout on the sheet ─────────────────

test("nothing above the list is a band: no fill, no edge, no aura, no shadow", async () => {
  const css = stripComments(await read("src/plugins/clipboard/page.css"));
  const topbar = rule(css, ".clipboard-panel__topbar");
  assert.ok(topbar, "page.css must still define the topbar row");
  assert.ok(!paintsFill(topbar!.body), "the header row must paint no fill");
  assert.ok(!paintsEdge(topbar!.body), "the header row must draw no edge");
  assert.ok(
    !/box-shadow\s*:\s*(?!none)/.test(topbar!.body),
    "the header row must not float as a band",
  );
  // CLIP-DRAG retired the focus aura — the last band-shaped thing on the row.
  assert.equal(rule(css, ".clipboard-panel__topbar::before"), undefined, "the aura pseudo-element must be gone");
  assert.equal(rule(css, ".clipboard-panel__topbar::after"), undefined, "no separator may return");
  assert.ok(!/clipboard-panel__topbar::/.test(css), "the row must have no pseudo-element at all");
  // Its wash token went with it, in both palettes.
  assert.ok(!/--accent-wash\s*:/.test(css), "--accent-wash must be retired with the aura");
  // The drag affordance is the terminal bar's own.
  assert.equal(decl(topbar!.body, "cursor"), "grab", "the header is the drag handle");
});

test("the filter row is layout too — the tab strip's own track is the only control shape", async () => {
  const css = stripComments(await read("src/plugins/clipboard/page.css"));
  const filterbar = rule(css, ".clipboard-panel__filterbar");
  assert.ok(filterbar, "page.css must define the filter row");
  assert.ok(!paintsFill(filterbar!.body), "the filter row must not paint a band");
  assert.ok(!paintsEdge(filterbar!.body), "the filter row must not draw a band edge");
  // The tab strip keeps its recessed segmented track (that is a control, not a
  // bar) — the page's one piece of chrome above the list.
  assert.match(rule(css, ".clipboard-panel__tabs")!.body, /var\(--surface-field\)/);
});

// ── 5 · mutation locks ────────────────────────────────────────────────────

test("mutation lock: turning the drag route into a no-op goes red", () => {
  // The exact regression: the listener stops calling the drag body.
  const route = (data: unknown, active: boolean, fire: () => void) => {
    if (shouldStartWindowDrag(data, active)) fire();
  };
  const drag = { [BRIDGE_TAG]: "drag" };
  let fired = 0;
  route(drag, true, () => { fired += 1; });
  assert.equal(fired, 1, "the shipped predicate must fire the drag");

  // A no-op — the listener recognizes the message but never runs the body.
  const noopRoute = (_data: unknown, _active: boolean, _fire: () => void) => {};
  let noopFired = 0;
  noopRoute(drag, true, () => { noopFired += 1; });
  assert.equal(noopFired, 0, "a no-op route must not fire the drag");

  // And the predicate itself must reject what the no-op would have accepted.
  assert.ok(
    shouldStartWindowDrag(drag, true),
    "the shipped predicate must accept a live page's drag",
  );
  assert.ok(
    !shouldStartWindowDrag(drag, false),
    "the predicate must reject a hidden page's drag",
  );
});

test("mutation lock: re-adding a header band (aura or fill) goes red", async () => {
  const css = stripComments(await read("src/plugins/clipboard/page.css"));
  // Restore the aura pseudo-element exactly as CLIP-DISSOLVE left it.
  const withAura = css.replace(
    /(\.clipboard-panel__topbar \{)/,
    `$1\n  background: linear-gradient(90deg, rgba(143, 183, 255, 0.055), transparent 42%);`,
  );
  assert.notEqual(withAura, css, "the mutation must land");
  const band = rule(withAura, ".clipboard-panel__topbar")!.body;
  assert.ok(
    paintsFill(band),
    "the no-fill predicate must reject a re-added header band",
  );
  // The shipped rule passes it.
  assert.ok(!paintsFill(rule(css, ".clipboard-panel__topbar")!.body), "the shipped header must stay flat");
});
