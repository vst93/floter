// R7-4a/b · the plugin page's host-owned chrome and the filter consolidation.
//
// Two things this round had to get right, and one it had to prove:
//
//   * R7-4a — a plugin page gets a 28px host topbar (title + close). The hard
//     red line is that it is *host chrome inside the plugin surface*, not a
//     second layer: the kept-alive iframe's position is what stops React from
//     remounting it on every mode flip, and a new sibling of `pluginLayer`
//     would re-key it. The bar is material-free (it paints no fill and no
//     blur); the glass behind it is the shell's single blur.
//
//   * R7-4b — the clipboard page's own `backdrop-filter` was inert. A sandboxed
//     iframe's backdrop is its own document, not the host's pixels, so the
//     filter blurred a flat fill and spent a slot of the same-screen filter
//     budget for nothing. It is removed; the material still travels host → page
//     through the R8 bridge.
//
// These assertions are structural (read the files) because the node suite has
// no DOM. The pixel proof for R7-4b lives in the round report.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim(), body: match[2] });
  }
  return out;
};

// ── R7-4a · host topbar ──────────────────────────────────────────────────

test("the topbar is rendered inside the plugin host, never as a new sibling layer", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  const app = await read("src/App.tsx");
  // R33 · the built-in pages are retired, so App no longer mounts the host or
  // its layer. The component keeps its own chrome contract for the external
  // pages the published protocol still exists to host.
  assert.equal(
    (app.match(/<PluginPageHost\b/g) ?? []).length,
    0,
    "the retired host must not be mounted",
  );
  assert.equal((app.match(/\{pluginLayer\}/g) ?? []).length, 0);
  // The topbar markup is inside the host component's own tree. The host's root
  // is `.plugin-page-host`; the header must appear after that opening tag and
  // before the component's closing brace, i.e. within the same subtree.
  const rootTag = host.indexOf('className="plugin-page-host"');
  assert.ok(rootTag > -1, "the host must render its root element");
  const topbar = host.indexOf("plugin-page-host__topbar");
  assert.ok(topbar > rootTag, "the topbar must live inside .plugin-page-host");
});

test("the topbar is 28px chrome with a title and a close control on the type ramp", async () => {
  const css = stripComments(await read("src/styles/terminal.css"));
  const bar = rules(css).find(({ selector }) => selector === ".plugin-page-host__topbar");
  assert.ok(bar, "terminal.css must define .plugin-page-host__topbar");
  assert.match(bar!.body, /height:\s*28px/, "the chrome is 28px high");
  assert.match(bar!.body, /cursor:\s*grab/, "the bar is the drag handle");
  // The title uses a named type-scale step, never a literal px size.
  const title = rules(css).find(({ selector }) => selector === ".plugin-page-host__topbar-title");
  assert.ok(title, "terminal.css must define the topbar title");
  assert.match(title!.body, /font-size:\s*var\(--text-(caption|body|emphasis)\)/);
  assert.ok(
    !/font-size:\s*\d/.test(title!.body),
    "the title must consume the type ramp, not a literal size",
  );
  // The close affordance reuses the app's toolbar button, which is the same
  // control the settings and terminal headers use — no new key semantics.
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /className="toolbar-button toolbar-button--close"/);
  assert.match(host, /aria-label=\{t\("plugin\.close"\)\}/);
  assert.match(host, /onClick=\{onClose\}/);
});

test("the topbar carries no material of its own — zero literal blur, zero fill, zero filter", async () => {
  const css = stripComments(await read("src/styles/terminal.css"));
  const bar = rules(css).find(({ selector }) => selector === ".plugin-page-host__topbar");
  assert.ok(bar, "terminal.css must define .plugin-page-host__topbar");
  assert.ok(!/backdrop-filter/.test(bar!.body), "the topbar must not filter the backdrop");
  assert.ok(!/blur\(/.test(bar!.body), "the topbar must not carry a literal blur");
  assert.ok(!/\bbackground(-color)?\s*:/.test(bar!.body), "the topbar paints no fill of its own");
  // Its one painted edge is the shared gradient hairline.
  const edge = rules(css).find(({ selector }) => selector === ".plugin-page-host__topbar::after");
  assert.ok(edge, "the topbar separator must be a painted band");
  assert.match(edge!.body, /var\(--hairline-fade\)/);
  assert.ok(!/border/.test(edge!.body), "the separator is a gradient band, not a border");
});

test("the topbar reuses the host's drag handler instead of inventing a second one", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(host, /onDragStart:\s*\(event: ReactMouseEvent\)\s*=>\s*void/);
  assert.match(host, /onMouseDown=\{onDragStart\}/);
  // R33 · App no longer mounts the host, so the prop is threaded by whichever
  // external page loader mounts it; the host's own contract is what stays.
  const app = await read("src/App.tsx");
  assert.ok(
    !/<PluginPageHost\b/.test(app),
    "the retired host must not be mounted from App",
  );
  // No parallel Tauri drag-region attribute, which would be a second,
  // differently-behaving drag mechanism. Check the code, not the prose: the
  // comment above deliberately names the attribute it is rejecting.
  const code = (src: string) => src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
  assert.ok(
    !/data-tauri-drag-region/.test(code(app)) && !/data-tauri-drag-region/.test(code(host)),
    "the plugin chrome must reuse startDrag, not add a data-tauri-drag-region",
  );
});

test("the plugin error state no longer borrows the update banner's button", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.ok(
    !/update-banner__button/.test(host),
    "the error state must not reuse a class from the settings sheet",
  );
  assert.match(host, /className="plugin-page-host__button"/);
  const css = stripComments(await read("src/styles/terminal.css"));
  const button = rules(css).find(({ selector }) => selector === ".plugin-page-host__button");
  assert.ok(button, "the host must define its own error button");
  assert.ok(!/backdrop-filter/.test(button!.body), "the error button is a control, not a blur");
  assert.match(button!.body, /var\(--glass-control\)/);
});

// ── R7-4b · filter consolidation ─────────────────────────────────────────

test("R7-4b: the plugin page sheet declares no backdrop-filter at all", async () => {
  const page = stripComments(await read("src/plugins/clipboard/page.css"));
  const filters = [...page.matchAll(/(?:-webkit-)?backdrop-filter\s*:/g)];
  assert.equal(filters.length, 0, "clipboard/page.css must declare zero backdrop-filters");
  // The page still paints the host-injected material — the subtraction removed
  // an inert filter, not the glass.
  const panel = rules(page).find(({ selector }) => selector === ".clipboard-panel");
  assert.ok(panel, "clipboard/page.css must define .clipboard-panel");
  assert.match(panel!.body, /var\(--panel-bg\)/);
});

test("R7-4b: the page sheet only shrank — no new filter, no new blur literal", async () => {
  const page = stripComments(await read("src/plugins/clipboard/page.css"));
  // The red line for the plugin page this round: "only subtract". A literal
  // blur anywhere in the sheet is the shape a re-added filter takes.
  for (const match of page.matchAll(/blur\(\s*[\d.]+px/g)) {
    assert.fail(`clipboard/page.css still carries a literal blur: ${match[0]}`);
  }
});

test("R7-4b: the plugin surface's only filter is the terminal shell's single blur", async () => {
  const host = stripComments(await read("src/styles/terminal.css"));
  // The plugin layer itself, the host, and the topbar are all material-free.
  for (const selector of [".plugin-layer", ".plugin-page-host", ".plugin-page-host__topbar", ".plugin-page-host__topbar-title", ".plugin-page-host__button"]) {
    const rule = rules(host).find(({ selector: s }) => s === selector);
    if (!rule) continue;
    assert.ok(
      !/backdrop-filter/.test(rule.body),
      `${selector} must not filter — the shell owns the surface's one blur`,
    );
  }
  // And the shell that does filter uses the aliased token, never a literal.
  const shell = rules(host).find(({ selector }) => selector === ".terminal-panel");
  assert.ok(shell, "terminal.css must define .terminal-panel");
  assert.match(shell!.body, /backdrop-filter:\s*blur\(var\(--glass-blur-terminal\)\)\s*saturate\(var\(--glass-saturate\)\)/);
  assert.ok(!/blur\(\s*[\d.]+px/.test(shell!.body), "the shell must not use a literal blur");
});
