import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

// ── R-FREEZE-3 · G3: the list is a read; repair is an explicit gesture ──────

// The backend list path no longer writes `tool-lock.json` at all (not for a
// missing binding and not for a fingerprint rebind): a read operation has no
// durable side effect. Repair therefore has to be an explicit user gesture, and
// the panel already offers exactly that — the reconnect control on an
// unavailable row and the reprobe controls in the drawer. This test pins the
// frontend half of that contract so a future "refresh auto-heals" shortcut
// cannot quietly re-introduce an implicit repair by rewiring a poll into a
// mutation.
test("the list poll is a plain read: it invokes extensions_list and nothing else", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The load function is the only place that reads the list, and its body must
  // not reach for a mutating command (reconnect / reprobe / enable / disable /
  // install) as a side effect of a refresh.
  const loadAt = panel.indexOf("const loadExtensions = ");
  assert.notEqual(loadAt, -1, "the panel must have a loadExtensions entry point");
  const bodyStart = panel.indexOf("{", panel.indexOf("=>", loadAt));
  let depth = 0;
  let bodyEnd = bodyStart;
  for (let i = bodyStart; i < panel.length; i += 1) {
    if (panel[i] === "{") depth += 1;
    else if (panel[i] === "}") {
      depth -= 1;
      if (depth === 0) {
        bodyEnd = i;
        break;
      }
    }
  }
  const body = panel.slice(bodyStart, bodyEnd);
  assert.match(body, /invoke<[^>]*>\("extensions_list"/, "the refresh must read the list");
  for (const mutating of [
    "extensions_reconnect_system",
    "extensions_reprobe",
    "extensions_reprobe_commands",
    "extensions_enable",
    "extensions_disable",
  ]) {
    assert.ok(
      !body.includes(`"${mutating}"`),
      `a list refresh must not invoke ${mutating} implicitly`,
    );
  }
});

// Explicit repair entrances must stay wired: reconnect for an unavailable row,
// reprobe for the drawer's command table and health report. If these disappear,
// the read-only list would leave a diverged binding with no user-reachable way
// to persist the catch-up (only the startup reconcile would ever write it).
test("repair stays an explicit gesture: reconnect and reprobe remain wired", async () => {
  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  assert.match(row, /onReconnect/, "the row must keep offering the explicit reconnect entrance");

  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  for (const command of [
    "extensions_reconnect_system",
    "extensions_reprobe",
    "extensions_reprobe_commands",
  ]) {
    assert.ok(
      panel.includes(`"${command}"`),
      `the explicit repair command ${command} must remain reachable from the panel`,
    );
  }
});

// The panel must not carry any "list heals bindings" prose implying the read
// writes. This is a documentation-shaped guard against the behavior returning
// through a comment that later becomes a call.
test("no source claims a list refresh performs a binding write", async () => {
  for (const file of ["src/ExtensionsPanel.tsx", "src/extensions/ExtensionRow.tsx"]) {
    const source = stripJsComments(await read(file));
    assert.ok(
      !/refresh[\s\S]{0,40}(rebind|bind_locator|tool-lock)/i.test(source),
      `${file} must not describe a refresh as performing a tool-lock write`,
    );
  }
});
