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
  buildPluginPageUrl,
  commandAllowed,
  isBridgeClose,
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

test("glass-step messages are recognized with the three shipped ids only", () => {
  assert.ok(isBridgeGlass({ [BRIDGE_TAG]: "glass", glassStep: "low" }));
  assert.ok(isBridgeGlass({ [BRIDGE_TAG]: "glass", glassStep: "mid" }));
  assert.ok(isBridgeGlass({ [BRIDGE_TAG]: "glass", glassStep: "high" }));
  for (const bad of [
    { [BRIDGE_TAG]: "glass", glassStep: "clear" },
    { [BRIDGE_TAG]: "glass", glassStep: 0.68 },
    { [BRIDGE_TAG]: "glass", glassStep: null },
    { [BRIDGE_TAG]: "glass" },
    { [BRIDGE_TAG]: "theme", glassStep: "mid" },
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
  const low = glassStepStyle("low");
  const high = glassStepStyle("high");
  assert.equal(low["--glass-step-fill"], String(GLASS_STEP_TOKENS.low.fill));
  assert.equal(low["--glass-step-dim"], String(GLASS_STEP_TOKENS.low.dim));
  assert.equal(low["--glass-solid-top"], String(GLASS_SOLID_TOP));
  assert.notEqual(low["--glass-step-fill"], high["--glass-step-fill"], "the injected fill must track the step");

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
    !/--glass-step-fill\s*:/.test(page) && !/--glass-solid-top\s*:/.test(page),
    "clipboard/page.css must not hardcode the glass step — the host injects it",
  );

  // …and the page consumes both channels, with a Regular fallback for an
  // older host that sends neither param nor message.
  const main = await readFile(new URL("src/plugins/clipboard/main.ts", root), "utf8");
  assert.match(main, /isBridgeGlass\(data\)/, "the page must handle a live step change");
  assert.match(main, /normalizeGlassStep\(params\.get\("glass-step"\)\)/, "the page must read the bootstrap param");
  assert.match(main, /GLASS_STEP_TOKENS\[step\]/, "the page must resolve the step from the shared table");
});
