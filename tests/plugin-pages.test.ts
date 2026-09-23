// Tests for the generic plugin-page bridge protocol and URL building
// (src/plugin-pages.ts). The same predicates gate messages on both sides of
// the sandbox boundary: the host only honors requests it can trust, the page
// only accepts well-formed results.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const root = new URL("../", import.meta.url);

import {
  BRIDGE_TAG,
  BROWSER_PLUGIN_ID,
  BUILTIN_BASE_PLUGINS,
  CLIPBOARD_PLUGIN_ID,
  buildPluginPageUrl,
  commandAllowed,
  pluginPageNeedsSameOrigin,
  isBridgeClose,
  isBridgeDrag,
  isBridgeGlass,
  isBridgeOpacity,
  isBridgeReload,
  isBridgeRequest,
  isBridgeResult,
  isBridgeResultForSession,
  isBridgeTheme,
  isBridgeVisibility,
} from "../src/plugin-pages.ts";

test("bridge session tokens survive requests and replies without admitting malformed tokens", () => {
  const request = { floter: "invoke", id: 1, session: "new-document", command: "clipboard_get_entries" };
  assert.ok(isBridgeRequest(request));
  assert.ok(isBridgeResult({ floter: "result", id: 1, session: request.session, ok: true, value: [] }));
  assert.equal(isBridgeRequest({ ...request, session: {} }), false);
  assert.equal(isBridgeRequest({ ...request, session: "x".repeat(129) }), false);
  assert.equal(isBridgeResult({ floter: "result", id: 1, session: 42, ok: false, error: "failed" }), false);
});

test("bridge visibility carries an explicit boolean for CSS-hidden frames", () => {
  assert.ok(isBridgeVisibility({ floter: "visibility", visible: false }));
  assert.ok(isBridgeVisibility({ floter: "visibility", visible: true }));
  assert.equal(isBridgeVisibility({ floter: "visibility", visible: "false" }), false);
  assert.equal(isBridgeVisibility({ floter: "reload", visible: true }), false);
});

test("a reloaded page rejects an old document's reply even when request ids match", () => {
  const delayed = { floter: "result", id: 1, session: "old-document", ok: true, value: ["old"] };
  assert.equal(isBridgeResultForSession(delayed, "new-document"), false);
  assert.equal(isBridgeResultForSession({ ...delayed, session: "new-document" }, "new-document"), true);
  assert.equal(isBridgeResultForSession({ ...delayed, session: undefined }, "new-document"), false);
});

test("invoke requests are recognized with optional args", () => {
  assert.ok(
    isBridgeRequest({ [BRIDGE_TAG]: "invoke", id: 1, command: "clipboard_get_entries" }),
  );
  assert.ok(
    isBridgeRequest({
      [BRIDGE_TAG]: "invoke",
      id: 2,
      command: "clipboard_set_favorite",
      args: { id: "x", favorite: true },
    }),
  );
  assert.ok(
    isBridgeRequest({
      [BRIDGE_TAG]: "invoke",
      id: 3,
      command: "cmd",
      args: null,
    }),
  );
});

test("malformed invoke requests are rejected, not thrown on", () => {
  const bad = [
    null,
    "invoke",
    {},
    // Missing or wrong tag.
    { id: 1, command: "cmd" },
    { [BRIDGE_TAG]: "other", id: 1, command: "cmd" },
    // Bad correlation ids.
    { [BRIDGE_TAG]: "invoke", id: "1", command: "cmd" },
    { [BRIDGE_TAG]: "invoke", id: Number.NaN, command: "cmd" },
    // Bad commands.
    { [BRIDGE_TAG]: "invoke", id: 1 },
    { [BRIDGE_TAG]: "invoke", id: 1, command: "" },
    // Args must be an object when present.
    { [BRIDGE_TAG]: "invoke", id: 1, command: "cmd", args: "x" },
    { [BRIDGE_TAG]: "invoke", id: 1, command: "cmd", args: 7 },
  ];
  for (const candidate of bad) {
    assert.equal(isBridgeRequest(candidate), false, JSON.stringify(candidate));
  }
});

test("close messages are recognized and nothing else is", () => {
  assert.ok(isBridgeClose({ [BRIDGE_TAG]: "close" }));
  assert.equal(isBridgeClose({ [BRIDGE_TAG]: "invoke", id: 1, command: "c" }), false);
  assert.equal(isBridgeClose(null), false);
});

test("drag messages are recognized and nothing else is", () => {
  // CLIP-DRAG · the payload-free window-drag request a sandboxed page sends
  // when the user presses its blank header.
  assert.ok(isBridgeDrag({ [BRIDGE_TAG]: "drag" }));
  assert.equal(isBridgeDrag({ [BRIDGE_TAG]: "close" }), false, "close is its own message");
  assert.equal(isBridgeDrag({ [BRIDGE_TAG]: "invoke" }), false);
  assert.equal(isBridgeDrag(null), false);
  assert.equal(isBridgeDrag({}), false);
  // The request is payload-free: the recognizer keys only on the tag and the
  // host reads no fields off it, so a page cannot smuggle a position, a window
  // or a size across the sandbox — a message that carries them is still just
  // "drag", and the host ignores the extras.
  assert.ok(isBridgeDrag({ [BRIDGE_TAG]: "drag", x: 10, y: 20 }));
});

test("results must be ok-with-value or error-with-string", () => {
  assert.ok(isBridgeResult({ [BRIDGE_TAG]: "result", id: 1, ok: true, value: [] }));
  assert.ok(isBridgeResult({ [BRIDGE_TAG]: "result", id: 2, ok: false, error: "boom" }));
  assert.equal(
    isBridgeResult({ [BRIDGE_TAG]: "result", id: 3, ok: true }),
    false,
    "ok without a value",
  );
  assert.equal(
    isBridgeResult({ [BRIDGE_TAG]: "result", id: 4, ok: false, error: 9 }),
    false,
    "non-string error",
  );
  assert.equal(isBridgeResult({ [BRIDGE_TAG]: "invoke", id: 5, command: "c" }), false);
});

test("the allowlist decides which commands the host will run", () => {
  const allowed = ["clipboard_get_entries", "clipboard_delete"];
  assert.ok(commandAllowed(allowed, "clipboard_get_entries"));
  assert.equal(commandAllowed(allowed, "open_url"), false);
  // Prefixes do not count as matches.
  assert.equal(commandAllowed(allowed, "clipboard_get"), false);
  assert.equal(commandAllowed([], "anything"), false);
});

test("page URLs resolve against the app base and carry bootstrap params", () => {
  // Packaged shape: tauri protocol root.
  const packaged = buildPluginPageUrl("tauri://localhost/", "plugins/clipboard/index.html", {
    lang: "zh",
    theme: "dark",
    "main-opacity": 0.47,
    "terminal-opacity": 0.46,
  });
  assert.equal(packaged.startsWith("tauri://localhost/plugins/clipboard/index.html"), true);
  assert.ok(packaged.includes("lang=zh"));
  assert.ok(packaged.includes("theme=dark"));
  assert.ok(packaged.includes("main-opacity=0.47"));

  // Dev-server shape: absolute path under localhost.
  const dev = buildPluginPageUrl("http://localhost:1420/", "plugins/clipboard/index.html");
  assert.equal(dev, "http://localhost:1420/plugins/clipboard/index.html");

  // A page outside its plugins/ directory would be a registry bug; URL
  // building itself stays neutral so the test pins the shape only.
  const nested = buildPluginPageUrl("tauri://localhost/", "../escape.html");
  assert.ok(nested.includes("escape.html"));
});

test("opacity messages are recognized with finite values", () => {
  assert.ok(isBridgeOpacity({ [BRIDGE_TAG]: "opacity", mainOpacity: 0.94, terminalOpacity: 0.92 }));
  assert.ok(isBridgeOpacity({ [BRIDGE_TAG]: "opacity", mainOpacity: 0, terminalOpacity: 1 }));
  assert.equal(
    isBridgeOpacity({ [BRIDGE_TAG]: "opacity", mainOpacity: Number.NaN, terminalOpacity: 0.9 }),
    false,
    "NaN rejected",
  );
  assert.equal(
    isBridgeOpacity({ [BRIDGE_TAG]: "opacity", mainOpacity: "0.94", terminalOpacity: 0.9 }),
    false,
    "string rejected",
  );
  assert.equal(isBridgeOpacity({ [BRIDGE_TAG]: "opacity" }), false, "missing fields");
});

test("theme messages are recognized with dark or light", () => {
  assert.ok(isBridgeTheme({ [BRIDGE_TAG]: "theme", theme: "dark" }));
  assert.ok(isBridgeTheme({ [BRIDGE_TAG]: "theme", theme: "light" }));
  assert.equal(
    isBridgeTheme({ [BRIDGE_TAG]: "theme", theme: "auto" }),
    false,
    "auto not a valid page theme",
  );
  assert.equal(isBridgeTheme({ [BRIDGE_TAG]: "theme", theme: null }), false);
  assert.equal(isBridgeTheme({ [BRIDGE_TAG]: "theme" }), false);
});

test("reload messages are recognized", () => {
  assert.ok(isBridgeReload({ [BRIDGE_TAG]: "reload" }));
  assert.equal(isBridgeReload({ [BRIDGE_TAG]: "invoke", id: 1, command: "c" }), false);
  assert.equal(isBridgeReload(null), false);
  assert.equal(isBridgeReload({}), false);
});

test("glass-step messages are recognized with the shipped stop ids only", () => {
  assert.ok(isBridgeGlass({ [BRIDGE_TAG]: "glass", glassStep: "frosted" }));
  assert.ok(isBridgeGlass({ [BRIDGE_TAG]: "glass", glassStep: "regular" }));
  assert.ok(isBridgeGlass({ [BRIDGE_TAG]: "glass", glassStep: "liquid" }));
  for (const bad of [
    { [BRIDGE_TAG]: "glass", glassStep: "clear" },
    // A pre-GLASS-3STOP id is not a *live* step: the bridge speaks the current
    // vocabulary, and the page migrates an old bootstrap param via
    // `normalizeGlassStep` rather than accepting the dead id here.
    { [BRIDGE_TAG]: "glass", glassStep: "low" },
    { [BRIDGE_TAG]: "glass", glassStep: "jelly" },
    { [BRIDGE_TAG]: "glass", glassStep: 0.68 },
    { [BRIDGE_TAG]: "glass", glassStep: null },
    { [BRIDGE_TAG]: "glass" },
    { [BRIDGE_TAG]: "theme", glassStep: "regular" },
    null,
  ]) {
    assert.equal(isBridgeGlass(bad), false, JSON.stringify(bad));
  }
});

// F6 (R8 microfix): the glass step has to reach the plugin page. Before this
// the page's own stylesheet hardcoded the Regular step's fill/top, so a user
// on Clear or Regular-max still saw a Regular clipboard panel — the one
// surface in the same shell that ignored the material control.
test("the plugin host hands the glass step to the page, and the page stops hardcoding it", async () => {
  const { GLASS_STEP_TOKENS, GLASS_SOLID_TOP, glassStepStyle } = await import("../src/glass-material.ts");

  // The bag the host injects is derived from the one token table — no literals
  // at the call site, and it changes with the step.
  const frosted = glassStepStyle("frosted");
  const liquid = glassStepStyle("liquid");
  assert.equal(frosted["--glass-step-dim"], String(GLASS_STEP_TOKENS.frosted.dim));
  assert.equal(frosted["--glass-solid-top"], String(GLASS_SOLID_TOP));
  assert.notEqual(frosted["--glass-step-dim"], liquid["--glass-step-dim"], "the injected haze must track the step");

  // The host spreads that bag onto its container and appends the step to the
  // bootstrap URL, so the page sees it before its first paint even if the
  // bridge message races the load.
  const host = await readFile(new URL("src/plugins/PluginPageHost.tsx", root), "utf8");
  assert.match(host, /glassStepStyle\(glassStep\)/, "the host must inject the step tokens");
  assert.match(host, /data-glass-step=\{glassStep\}/, "the host must mark the step on the container");
  assert.match(host, /"glass-step": glassStep/, "the host must pass the step as a bootstrap param");

  // The page stylesheet mirrors the transparency control but must not restate
  // the step's numbers: those now arrive at runtime.
  const page = await readFile(new URL("src/plugins/clipboard/page.css", root), "utf8");
  assert.ok(
    !/--glass-step-dim\s*:/.test(page) && !/--glass-solid-top\s*:/.test(page) && !/--glass-frame-floor\s*:/.test(page),
    "clipboard/page.css must not hardcode the glass step — the host injects it",
  );

  // …and the page consumes both channels, with a Regular fallback for an
  // older host that sends neither param nor message.
  const main = await readFile(new URL("src/plugins/clipboard/main.ts", root), "utf8");
  assert.match(main, /isBridgeGlass\(data\)/, "the page must handle a live step change");
  assert.match(main, /normalizeGlassStep\(params\.get\("glass-step"\)\)/, "the page must read the bootstrap param");
  assert.match(main, /GLASS_STEP_TOKENS\[step\]/, "the page must resolve the step from the shared table");
});

// R26-C · the settings panel's base-plugins list must carry every registered
// plugin, browser included.
//
// The bug: `App.tsx` assembled the list by hand and only ever named
// `builtin.clipboard`, so when R26-B registered `builtin.browser` (descriptor +
// page + allowlist) the settings panel never showed it — the plugin had no
// entry and no way to open its page. The list now lives in
// `BUILTIN_BASE_PLUGINS` (src/plugin-pages.ts), and this guard pins it to the
// Rust registry in BOTH directions: a descriptor without a row fails, and a row
// naming an unregistered plugin fails.
test("the base-plugin list carries builtin.browser and mirrors the Rust registry", async () => {
  const ids = BUILTIN_BASE_PLUGINS.map((plugin) => plugin.id);
  assert.ok(
    ids.includes(BROWSER_PLUGIN_ID),
    "the base-plugin list must contain builtin.browser (the R26-C regression)",
  );
  assert.ok(ids.includes(CLIPBOARD_PLUGIN_ID), "the base-plugin list must contain builtin.clipboard");

  // The registry's constant names -> values, then the ids `DESCRIPTORS` uses.
  const rust = await readFile(new URL("src-tauri/src/plugin_pages.rs", root), "utf8");
  const constants = new Map<string, string>();
  for (const match of rust.matchAll(/pub const (\w+_PLUGIN_ID): &str = "([^"]+)";/g)) {
    constants.set(match[1], match[2]);
  }
  const descriptorsAt = rust.indexOf("static DESCRIPTORS");
  assert.notEqual(descriptorsAt, -1, "the Rust descriptor registry must exist");
  const registryIds = [...rust.slice(descriptorsAt).matchAll(/id: (\w+_PLUGIN_ID),/g)].map((match) => {
    const value = constants.get(match[1]);
    assert.ok(value, `${match[1]} must be declared before the registry uses it`);
    return value;
  });

  assert.deepEqual(
    [...ids].sort(),
    [...new Set(registryIds)].sort(),
    "the settings list and the Rust registry must name the same plugins",
  );

  // The list is only real if the panel renders it: the hand-written array in
  // App.tsx is gone, replaced by this registry.
  const app = await readFile(new URL("src/App.tsx", root), "utf8");
  assert.match(
    app,
    /basePlugins=\{BUILTIN_BASE_PLUGINS/,
    "App.tsx must render the shared base-plugin list, not a hand-written array",
  );
});

// R26-D · every registered plugin page must announce the protocol.
//
// The user's report, verbatim: 「插件加载失败 此页面未声明插件页协议版本。本版本
// 支持协议 1。请更新页面以发送 frame-ready 握手」. The browser page *did* send
// the handshake — its module never ran, because the host only granted
// `allow-same-origin` to the clipboard id and WebKit refused the opaque-origin
// frame's ES module. The root cause is pinned separately below; this guard is
// the general one: whatever the registry lists, its entry module must send the
// handshake. It is driven off the Rust registry, so a new descriptor cannot
// ship a page that never announces itself.
test("no built-in page is registered, and the retained entries still handshake", async () => {
  const rust = await readFile(new URL("src-tauri/src/plugin_pages.rs", root), "utf8");
  const descriptorsAt = rust.indexOf("static DESCRIPTORS");
  assert.notEqual(descriptorsAt, -1, "the Rust descriptor registry must exist");
  const pages = [...rust.slice(descriptorsAt).matchAll(/page: "([^"]*)"/g)].map((m) => m[1]);
  assert.ok(pages.length >= 2, `expected the registry to list both descriptors, saw ${pages.length}`);
  // R33 · the built-in iframe pages are retired, so no descriptor names a
  // document. The entry modules stay for the retained protocol and must keep
  // sending the handshake if a future loader mounts them.
  for (const page of pages) assert.equal(page, "", "no built-in page path may be registered");
  for (const entryPath of ["src/plugins/clipboard/main.ts", "src/plugins/browser/main.ts"]) {
    const entry = await readFile(new URL(entryPath, root), "utf8");
    assert.match(
      entry,
      /\{ \[BRIDGE_TAG\]: "frame-ready", protocol: PLUGIN_PAGE_PROTOCOL \}/,
      `${entryPath} must keep sending the frame-ready handshake`,
    );
  }
});

// R26-D · the sandbox exception is a *set*, not a clipboard-only special case.
//
// The bug that produced the missing handshake above: `PluginPageHost` wrote
// `descriptor?.id === CLIPBOARD_PLUGIN_ID` inline, so the browser page (a
// built-in that ships bundled assets on the app origin, exactly like the
// clipboard page) was sandboxed with an opaque origin and its module never
// loaded on WebKit. Every built-in page must be in the same-origin set.
test("the same-origin sandbox set is a set, not a clipboard-only special case", () => {
  // R33 · no built-in page is registered any more, so every built-in row is
  // `configurable` (overlay) rather than `hasPage`. The sandbox exception the
  // R26-D bug was about is still a lookup, not an inline id test.
  for (const plugin of BUILTIN_BASE_PLUGINS) {
    assert.equal(plugin.configurable, true, `${plugin.id} opens the generic overlay`);
  }
  assert.equal(pluginPageNeedsSameOrigin(CLIPBOARD_PLUGIN_ID), true);
  assert.equal(pluginPageNeedsSameOrigin(BROWSER_PLUGIN_ID), true);
  assert.equal(pluginPageNeedsSameOrigin("external.example"), false);
  assert.equal(pluginPageNeedsSameOrigin(null), false);
  assert.equal(pluginPageNeedsSameOrigin(undefined), false);
});
