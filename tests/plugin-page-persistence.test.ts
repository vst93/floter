// The plugin page must survive mode switches. Its host used to be re-mounted
// inside the plugin / collapsed / terminal branches, so React tore the iframe
// down and rebuilt it on every toggle: reopening re-fetched the descriptor and
// page (jank) and closing dropped the keyboard to <body> (focus loss). The fix
// keeps a single host instance alive above the branches. These assertions lock
// that structure in, since nothing else exercises JSX reconciliation.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

test("the plugin page host is mounted exactly once, outside the mode branches", async () => {
  const app = await read("src/App.tsx");
  assert.equal(
    (app.match(/<PluginPageHost\b/g) ?? []).length,
    1,
    "expected a single PluginPageHost mount hoisted above the branches",
  );
  // The one instance is spread into every mode's tree as a stable first sibling.
  assert.equal((app.match(/\{pluginLayer\}/g) ?? []).length, 4);
  assert.equal((app.match(/\{toastHost\}/g) ?? []).length, 4);
  // `data-plugin-layer` was write-only (no selector or query ever targeted it).
  assert.ok(
    !app.includes("data-plugin-layer"),
    "the plugin layer must not carry unused data-plugin-layer markup",
  );
});

test("pluginLayer and the toast host lead every mode tree, before its shell", async () => {
  const app = await read("src/App.tsx");
  const tails = app.split("{pluginLayer}").slice(1);
  assert.equal(tails.length, 4, "pluginLayer must be rendered in all four modes");
  for (const tail of tails) {
    assert.match(
      tail,
      /^\s*\{toastHost\}\s*<div className="(?:settings|collapsed|terminal)-shell">/,
      "pluginLayer must be the first sibling (toast host next, shell after)",
    );
  }
});

test("the plugin layer is only painted while a page is open", async () => {
  const css = await read("src/styles/terminal.css");
  const layer = css.slice(css.indexOf(".plugin-layer {"));
  assert.match(layer, /position:\s*fixed/, ".plugin-layer must be fixed");
  assert.match(layer, /z-index:\s*30/, ".plugin-layer must sit under the toasts (60)");
  assert.match(layer, /display:\s*none/, ".plugin-layer must default to hidden");
  assert.match(
    css,
    /\.plugin-layer\[data-active="true"\]\s*\{\s*display:\s*block/,
    ".plugin-layer[data-active] must reveal the page",
  );
});

test("the plugin layer is clipped to the panel's rounded rectangle", async () => {
  const css = await read("src/styles/terminal.css");
  const layer = css.slice(css.indexOf(".plugin-layer {"), css.indexOf("[data-active"));
  assert.match(
    layer,
    /border-radius:\s*var\(--window-radius\)/,
    "the opaque page must be clipped to the panel radius",
  );
  assert.match(layer, /overflow:\s*hidden/, "the layer must clip its overflowing page");
  // Windows reserves shell padding for the panel's drop shadow; the layer has
  // to sit inside that padding rather than stretch back over it.
  assert.match(
    css,
    /\.platform-windows \.plugin-layer\s*\{\s*inset:\s*10px/,
    "the layer must match the panel geometry on Windows",
  );
});

test("the plugin mode's panel renders only the rounded backdrop, no covered body", async () => {
  const app = await read("src/App.tsx");
  // Anchor to the plugin branch's own comment, then stop at the next mode
  // branch. Anchoring on `mode === "plugin"` alone lands on the earlier
  // mode-effect switch, whose slice never contains the JSX branch at all and
  // so made this assertion vacuous.
  const marker = "The panel renders only as the rounded backdrop";
  const start = app.indexOf(marker);
  assert.ok(start > -1, "the plugin branch's backdrop comment must exist");
  const branch = app.slice(start);
  const nextBranch = branch.indexOf("if (mode ===");
  assert.ok(nextBranch > -1, "the plugin branch must be followed by another branch");
  const jsx = branch.slice(0, nextBranch);
  // F5: the plugin branch renders the panel as a backdrop carrying only the
  // glass veil — the tint that gives the frame its body under the page — and
  // nothing that could be covered. R7-GLASS-DEEP added the veil (the frame
  // used to be transparent because the plugin page painted its own sheet; it
  // still does, and the veil is what the page's translucency now reads
  // against).
  assert.match(
    jsx,
    /<section className="terminal-panel terminal-panel--entered">\s*<div className="terminal-panel__veil" aria-hidden="true" \/>\s*<\/section>/,
    "the plugin branch must render the terminal-panel backdrop with only its glass veil",
  );
  assert.ok(
    !jsx.includes("terminal-panel__body"),
    "the plugin branch must not keep the now-covered empty panel body",
  );
  // ...and it is the only branch that may: the JSX body element occurs exactly
  // once in the whole file, in the terminal branch.
  assert.equal(
    (app.match(/terminal-panel__body/g) ?? []).length,
    1,
    "terminal-panel__body must occur exactly once (the terminal branch)",
  );
});

test("toasts are positioned per surface, inside each window", async () => {
  const css = await read("src/styles/extensions.css");
  const host = css.slice(css.indexOf("#floter-app-toasts {"));
  // Settings keeps the original card-header offset. The containing block is the
  // viewport (neither `#root` nor the card establishes one), and the card
  // starts at the window's top edge, so 64px clears its 56px header.
  assert.match(host, /top:\s*64px/, "settings toasts clear the card header");
  // The collapsed launcher is only ~58px tall, so the stack is pinned to the
  // bottom-LEFT: the bottom keeps it inside the short window and the left edge
  // keeps it clear of the right-hand icon buttons, which the old bottom-right
  // anchor (right: 6px) sat directly over.
  const collapsed = host.slice(host.indexOf('[data-surface="collapsed"]'));
  assert.match(collapsed, /bottom:/, "launcher toasts must anchor to the short window's bottom");
  assert.match(collapsed, /left:\s*6px/, "launcher toasts must anchor to the left edge");
  assert.match(collapsed, /right:\s*auto/, "launcher toasts must not straddle the icon buttons");
  // Only the newest toast is laid out, so even a MAX_TOASTS=3 queue cannot
  // overflow the 58px window.
  assert.match(
    collapsed,
    />\s*\.app-toast:not\(:last-child\)\s*\{\s*display:\s*none/s,
    "collapsed must collapse the stack to its newest toast",
  );
  // The surface is supplied by the host element at runtime.
  const toast = await read("src/components/ToastStack.tsx");
  assert.match(toast, /data-surface=\{dataSurface\}/, "the toast host must carry its surface");
});

test("closing the page hands focus back to the surface it returns to", async () => {
  const app = await read("src/App.tsx");
  const start = app.indexOf("const closePluginPage");
  assert.ok(start > -1, "closePluginPage must exist");
  const body = app.slice(start, app.indexOf("\n  };", start));
  // The collapsed return goes through the single collector's shared beat
  // pattern (0/90/140 + Windows retry), whose last beat is 140ms; the beats
  // themselves live in `collapsed-focus.ts`.
  assert.match(
    body,
    /scheduleCollapsedFocusBeats\(\)/,
    "collapsed return must re-focus through the shared collector",
  );
  assert.match(body, /focusTerminalView\(80\)/, "terminal return must re-focus the canvas");
});

test("the kept-alive iframe relinquishes focus when the page closes", async () => {
  const host = await read("src/plugins/PluginPageHost.tsx");
  assert.match(
    host,
    /frame\.contentWindow\?\.blur\(\)/,
    "a hidden iframe must not keep the keyboard",
  );
  // And it must also clear whatever the (same-origin) inner document focused:
  // a hidden iframe document stays focusable on WebKit and would otherwise
  // re-claim the keyboard after the host already re-homed it.
  assert.match(
    host,
    /contentDocument\?\.activeElement[\s\S]*?\.blur\(\)/,
    "the inner document's focused element must be blurred too",
  );
});
