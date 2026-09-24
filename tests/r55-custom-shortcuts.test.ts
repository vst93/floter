// R55 · custom global shortcuts + the clipboard panel joins the shortcut map.
//
// R56 revised the first half: the clipboard panel's global trigger was removed
// outright (no hidden default binding), and the custom-shortcut *interaction*
// changed — a draft row appears immediately, key and action are independent,
// the built-in group precedes the custom one, and the empty list explains
// itself. The custom-shortcut *semantics* (register / conflict / silent run)
// are untouched.
//
// Two feedbacks share this file because they are the same shape of change —
// one registry, one persistence path:
//
//   4 · the clipboard panel's trigger stops being a bespoke settings field and
//       becomes an ordinary `SHORTCUT_ACTIONS` member (empty = disabled);
//       R56 then removes it entirely, and R57 retires `pin_terminal` — the map
//       is seven uniform actions again.
//   5 · users can add N custom global shortcuts, each a key plus a launcher
//       action, registered with the OS and executed silently (plugins open
//       normally).
//
// Mutations that must turn this file red:
//   * a custom-shortcut classifier that opens a plugin silently, or runs a
//     command through the visible terminal;
//   * dropping the duplicate-key / empty-action normalization;
//   * letting an unfinished draft reach the OS or the settings file.

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

// ── 4 · the clipboard panel's trigger is gone (R56) ──────────────────────

test("R56/R57 · the clipboard panel and pin_terminal are not shortcut actions any more", () => {
  assert.equal(SHORTCUT_ACTIONS.includes("clipboard_panel" as never), false);
  assert.equal("clipboard_panel" in DEFAULT_SHORTCUTS, false);
  // R57 · pin_terminal retired with the pinned-terminal feature.
  assert.equal(SHORTCUT_ACTIONS.includes("pin_terminal" as never), false);
  assert.equal("pin_terminal" in DEFAULT_SHORTCUTS, false);
  assert.equal(SHORTCUT_ACTIONS.length, 7, "the map is seven uniform actions again");
});

test("R56 · no shortcut row carries a bespoke clear path", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  // The rows are still rendered by the one `SHORTCUT_ACTIONS.map`.
  assert.match(page, /SHORTCUT_ACTIONS\.map\(/);
  // The clipboard-only clear control is gone with its action.
  assert.equal(/action === "clipboard_panel"/.test(page), false);
  assert.equal(/onClearShortcut/.test(page), false);
  assert.equal(/CLIPBOARD_HOTKEY_ACTION/.test(page), false);

  const capture = stripJsComments(await read("src/hooks/useShortcutCapture.ts"));
  assert.equal(/CLIPBOARD_HOTKEY_ACTION/.test(capture), false, "no separate clipboard plumbing");
  assert.equal(/clearShortcut/.test(capture), false, "the clear path is gone with its only action");

  const rust = await read("src-tauri/src/commands/config.rs");
  assert.equal(/CLIPBOARD_PANEL/.test(rust), false, "the action id is retired");
  assert.equal(/DEFAULT_CLIPBOARD_HOTKEY/.test(rust), false);
  assert.equal(/fn update_clipboard_hotkey/.test(rust), false);
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
  assert.match(page, /rows\.map\(/);
  assert.match(page, /invoke\("suspend_shortcuts"\)/);
  assert.match(page, /invoke\("resume_shortcuts"\)/);
  assert.match(page, /<ShortcutRecorder/);
});

// ── R56 · the interaction cleanup ────────────────────────────────────────

test("R56 · the built-in group precedes the custom group", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  // The custom group lives in its own component; compare the built-in section
  // (its heading string) against the point the page renders the custom one.
  const builtin = page.indexOf("settings.group.shortcuts");
  const custom = page.indexOf("<CustomShortcutsSection");
  assert.notEqual(builtin, -1);
  assert.notEqual(custom, -1);
  assert.ok(builtin < custom, "shipped shortcuts first, extensions second");
  // Both group headings carry their action in the same slot and language.
  assert.match(page, /<SettingsAction/);
  const actions = page.match(/<SettingsAction/g) ?? [];
  assert.ok(actions.length >= 3, "restore-defaults, add, and the empty-state add");
});

test("R56 · Add drops a draft row instead of forcing a key first", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  // The add path appends a keyless row with the default action — no recording.
  assert.match(page, /const addDraft = \(\) => \{/);
  assert.match(page, /key: "",/);
  assert.match(page, /action: CUSTOM_SHORTCUT_PICKER\[0\]\.action/);
  // The draft add does not suspend the global shortcuts; only the recorder does.
  const addBody = page.slice(page.indexOf("const addDraft"), page.indexOf("const beginRecording"));
  assert.equal(/suspend_shortcuts/.test(addBody), false,
    "adding a draft must not enter key capture");
});

test("R56 · only a complete row is persisted; a draft stays local", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  // Completeness is key AND action.
  assert.match(page, /const rowIsComplete = \(row: CustomRow\): boolean =>/);
  assert.match(page, /row\.key\.trim\(\) && row\.action\.trim\(\)/);
  // The commit path filters to complete rows before persisting.
  assert.match(page, /next\.filter\(rowIsComplete\)\.map\(rowToEntry\)/);
  // And no commit leaves the OS unchanged when nothing complete changed.
  assert.match(page, /if \(serialized === committedRef\.current\) return;/);
});

test("R56 · the empty list explains itself with an inline entry", async () => {
  const page = stripJsComments(await read("src/settings/ShortcutsPage.tsx"));
  assert.match(page, /<SettingsEmpty/);
  assert.match(page, /settings\.customShortcutsEmptyHint/);
  assert.match(page, /settings\.customShortcutsAddFirst/);
  // The entry is a slot on the empty primitive, not a floating layer.
  assert.match(page, /action=\{\s*<SettingsAction/);
  const primitives = stripJsComments(await read("src/settings/SettingsRows.tsx"));
  assert.match(primitives, /settings-empty__action/);
});
