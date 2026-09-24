// R55 · custom global shortcuts + the clipboard panel joins the shortcut map.
//
// Two feedbacks share this file because they are the same shape of change —
// one registry, one persistence path:
//
//   4 · the clipboard panel's trigger stops being a bespoke settings field and
//       becomes an ordinary `SHORTCUT_ACTIONS` member (empty = disabled);
//   5 · users can add N custom global shortcuts, each a key plus a launcher
//       action, registered with the OS and executed silently (plugins open
//       normally).
//
// Mutations that must turn this file red:
//   * splitting the clipboard trigger back out of the map;
//   * a custom-shortcut classifier that opens a plugin silently, or runs a
//     command through the visible terminal;
//   * dropping the duplicate-key / empty-action normalization.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  CUSTOM_SHORTCUT_PICKER,
  classifyCustomShortcutAction,
  customShortcutKeysEqual,
  duplicateCustomShortcutKey,
  normalizeCustomShortcuts,
} from "../src/custom-shortcuts.ts";
import { DEFAULT_SHORTCUTS, SHORTCUT_ACTIONS } from "../src/shortcuts.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── 4 · the clipboard panel is a map action ──────────────────────────────

test("R55 · the clipboard panel is a shortcut action, disabled by default", () => {
  assert.ok(SHORTCUT_ACTIONS.includes("clipboard_panel"));
  assert.equal(DEFAULT_SHORTCUTS.clipboard_panel, "", "empty is the legitimate off state");
});

test("R55 · the clipboard row shares the map's restore and capture paths", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  // The row is rendered by the one `SHORTCUT_ACTIONS.map`, not a bespoke block.
  assert.match(page, /SHORTCUT_ACTIONS\.map\(/);
  // The only bespoke bit is the clear control, gated on the clipboard action.
  assert.match(page, /action === "clipboard_panel"/);
  assert.match(page, /onClearShortcut\(action\)/);
  // The old pseudo-action id is gone.
  assert.equal(/CLIPBOARD_HOTKEY_ACTION/.test(page), false);

  const capture = stripJsComments(await read("src/hooks/useShortcutCapture.ts"));
  assert.equal(/CLIPBOARD_HOTKEY_ACTION/.test(capture), false, "no separate clipboard plumbing");
  assert.match(capture, /invoke\("update_shortcut", \{ action, shortcut: "" \}\)/);
});

// ── 5 · the custom-shortcut vocabulary ───────────────────────────────────

test("R55 · a plugin action is classified as a plugin; anything else is a command", () => {
  assert.deepEqual(classifyCustomShortcutAction("plugin:clipboard"), {
    kind: "plugin",
    value: "clipboard",
  });
  assert.deepEqual(classifyCustomShortcutAction("action:new_command"), {
    kind: "action",
    value: "new_command",
  });
  assert.deepEqual(classifyCustomShortcutAction("say done"), {
    kind: "command",
    value: "say done",
  });
  assert.deepEqual(classifyCustomShortcutAction("  docker ps  "), {
    kind: "command",
    value: "docker ps",
  });
});

test("R55 · the picker offers plugins and app actions, never a duplicate id", () => {
  const actions = CUSTOM_SHORTCUT_PICKER.map((entry) => entry.action);
  assert.ok(actions.includes("plugin:clipboard"));
  assert.ok(actions.includes("plugin:browser"));
  assert.ok(actions.includes("plugin:calculator"));
  assert.ok(actions.includes("action:open_settings"));
  assert.equal(new Set(actions).size, actions.length, "no duplicate action in the picker");
});

test("R55 · key comparison resolves the app modifier and ignores order/case", () => {
  assert.ok(customShortcutKeysEqual("Cmd+Shift+K", "Shift+Cmd+K"));
  assert.ok(customShortcutKeysEqual("Cmd+Shift+K", "Cmd+Shift+k"));
  assert.equal(customShortcutKeysEqual("Cmd+K", "Ctrl+K"), false);
  assert.equal(customShortcutKeysEqual("Cmd+Shift+K", "Cmd+K"), false);
});

test("R55 · duplicate detection can skip the row being edited", () => {
  const keys = ["Cmd+K", "Ctrl+K", "Cmd+Shift+K"];
  assert.equal(duplicateCustomShortcutKey("Cmd+Shift+K", keys), "Cmd+Shift+K");
  assert.equal(duplicateCustomShortcutKey("Cmd+Shift+K", keys, 2), null);
  assert.equal(duplicateCustomShortcutKey("Cmd+Alt+K", keys), null);
  assert.equal(duplicateCustomShortcutKey("", keys), null, "an unset key is not a duplicate");
});

test("R55 · normalization drops empty keys/actions and duplicate keys", () => {
  assert.deepEqual(
    normalizeCustomShortcuts([
      { key: "Cmd+K", action: "plugin:clipboard" },
      { key: "", action: "plugin:browser" },
      { key: "Cmd+L", action: "  " },
      { key: "cmd+K", action: "action:new_command" },
      { key: "Cmd+M", action: "say hi" },
    ]),
    [
      { key: "Cmd+K", action: "plugin:clipboard" },
      { key: "Cmd+M", action: "say hi" },
    ],
  );
});

// ── 5 · the execution semantics ──────────────────────────────────────────

test("R55 · the trigger opens plugins and runs commands silently", async () => {
  const rawApp = await read("src/App.tsx");
  const app = stripJsComments(rawApp);
  // One listener for every custom key. (Checked on the raw source: the comment
  // stripper eats the `//` inside the event-name string.)
  assert.match(rawApp, /custom-shortcut:\/\/trigger/);
  // Plugins come forward and enter their mode (the three built-ins).
  assert.match(app, /enterPluginMode\(\{ scope: "clipboard"/);
  assert.match(app, /enterPluginMode\(\{ scope: "browser"/);
  assert.match(app, /enterPluginMode\(\{ scope: "calculator"/);
  // A command line goes to the silent runner — never the terminal window.
  assert.match(app, /invoke\("run_silent_command", \{ command: binding\.value \}\)/);
  // The classifier drives the branch, so the vocabulary has one owner.
  assert.match(app, /classifyCustomShortcutAction\(action\)/);
});

test("R55 · the backend claims the key globally and reports OS refusals", async () => {
  const rust = await read("src-tauri/src/lib.rs");
  assert.match(rust, /pub fn register_custom_shortcut/);
  assert.match(rust, /emit_to\("main", "custom-shortcut:\/\/trigger"/);
  const config = await read("src-tauri/src/commands/config.rs");
  assert.match(config, /pub fn set_custom_shortcuts/);
  // Two rejection reasons: an app-internal conflict and an OS-occupied key.
  assert.match(config, /reason: "conflict"/);
  assert.match(config, /reason: "occupied"/);
  // The command persists only what the OS actually accepted.
  assert.match(config, /updated\.custom_shortcuts = registered;/);
});

test("R55 · the custom list round-trips and is owned by its command", async () => {
  const rust = await read("src-tauri/src/commands/config.rs");
  assert.match(rust, /pub custom_shortcuts: Vec<CustomShortcut>/);
  assert.match(rust, /\[serde\(default\)\]\s*\n\s*pub custom_shortcuts/);
  // A whole-app save carries the stored list back rather than submitting a
  // stale one.
  assert.match(rust, /submitted\.custom_shortcuts = stored\.custom_shortcuts\.clone\(\);/);
  assert.match(rust, /settings\.custom_shortcuts = normalize_custom_shortcuts/);
});

test("R55 · the settings UI renders one row per custom shortcut", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  assert.match(page, /settings\.group\.customShortcuts/);
  assert.match(page, /customShortcuts\.map\(/);
  assert.match(page, /invoke\("suspend_shortcuts"\)/);
  assert.match(page, /invoke\("resume_shortcuts"\)/);
  assert.match(page, /<ShortcutRecorder/);
});
