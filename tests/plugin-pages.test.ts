// Tests for the generic plugin-page bridge protocol and URL building
// (src/plugin-pages.ts). The same predicates gate messages on both sides of
// the sandbox boundary: the host only honors requests it can trust, the page
// only accepts well-formed results.
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  BRIDGE_TAG,
  buildPluginPageUrl,
  commandAllowed,
  isBridgeClose,
  isBridgeOpacity,
  isBridgeRequest,
  isBridgeResult,
  isBridgeTheme,
} from "../src/plugin-pages.ts";

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
    "main-opacity": 0.94,
    "terminal-opacity": 0.92,
  });
  assert.equal(packaged.startsWith("tauri://localhost/plugins/clipboard/index.html"), true);
  assert.ok(packaged.includes("lang=zh"));
  assert.ok(packaged.includes("theme=dark"));
  assert.ok(packaged.includes("main-opacity=0.94"));

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
