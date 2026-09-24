// R51 · the calculator becomes *discoverable*.
//
// R50 shipped the calculator as a real built-in plugin — a mode, history,
// favorites, a schema-driven configuration overlay — but it was not registered
// anywhere a user browses. The R50 report put it plainly: it was absent from
// `BUILTIN_BASE_PLUGINS` and the backend's `DESCRIPTORS` (to keep the
// descriptor-equality guard honest without touching it), and it had no system
// result row, so the only way in was to already know the trigger word. An
// undiscoverable plugin is, for the user, a plugin that does not exist.
//
// This round registers it on both sides and gives it the launcher row the
// clipboard and browser have. Three facts are pinned, each positively — so
// deleting the code, not the prose, is what turns a case red:
//
//   1. the calculator is in both registries, the frontend list *and* the Rust
//      descriptor table, with the shape the other two rows carry;
//   2. it has a `SYSTEM_COMMANDS` row whose Enter enters the plugin's own mode;
//      and
//   3. both languages can name it — in the dictionary and in the notification
//      copy table — so a registered descriptor can never be announced as a raw
//      id.
//
// It is deliberately *not* given an on/off switch (see §4): the plugin has no
// persisted `enabled` field, and the registry's own contract says a plugin
// without a switch draws no switch rather than a dead one.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { createTranslator } from "../src/i18n.ts";
import { NOTIFICATION_PLUGIN_IDS } from "../src/notifications.ts";
import {
  BROWSER_PLUGIN_ID,
  BUILTIN_BASE_PLUGINS,
  CALCULATOR_PLUGIN_ID,
  CLIPBOARD_PLUGIN_ID,
} from "../src/plugin-pages.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

// ── 1 · both registries ───────────────────────────────────────────────────

test("the calculator joins the base-plugin list with the other two built-ins", () => {
  const ids = BUILTIN_BASE_PLUGINS.map((plugin) => plugin.id);
  assert.deepEqual(
    [...ids].sort(),
    [BROWSER_PLUGIN_ID, CALCULATOR_PLUGIN_ID, CLIPBOARD_PLUGIN_ID].sort(),
    "the base-plugin list names exactly the three built-ins now",
  );
  const calculator = BUILTIN_BASE_PLUGINS.find((plugin) => plugin.id === CALCULATOR_PLUGIN_ID);
  assert.ok(calculator, "the calculator must be a row in the base-plugin list");
  assert.equal(calculator.titleKey, "settings.calculator");
  assert.equal(calculator.descriptionKey, "settings.calculatorHint");
  // It has a configuration schema (R50's overlay) …
  assert.equal(calculator.configurable, true, "the row opens the shared configuration overlay");
  // … and no on/off field, so no switch (see §4).
  assert.equal(calculator.toggleable, false, "there is no `enabled` field to switch");
});

test("the Rust descriptor table registers the calculator with the same id", async () => {
  const rust = await read("src-tauri/src/plugin_pages.rs");
  // The id constant exists and the registry references it — the same shape the
  // other two descriptors use, which is what the descriptor-equality guard in
  // `plugin-pages.test.ts` reads in both directions.
  assert.match(
    rust,
    /pub const CALCULATOR_PLUGIN_ID: &str = "builtin\.calculator";/,
    "the backend must declare the calculator's stable id",
  );
  const descriptorsAt = rust.indexOf("static DESCRIPTORS");
  assert.notEqual(descriptorsAt, -1, "the Rust descriptor registry must exist");
  const registry = rust.slice(descriptorsAt);
  assert.match(registry, /id: CALCULATOR_PLUGIN_ID,/, "the registry must list the calculator");
  assert.match(registry, /title_key: "settings\.calculator"/);
  assert.match(registry, /description_key: "settings\.calculatorHint"/);

  // Its allowlist is exactly the calculator commands the plugin already invokes
  // (the mode + the configuration overlay), and it is not empty — the registry's
  // own test forbids an empty allowlist.
  const commands = /const CALCULATOR_COMMANDS: &\[&str\] = &\[([\s\S]*?)\];/.exec(rust);
  assert.ok(commands, "the calculator's command allowlist must exist");
  for (const command of [
    "calculator_get_entries",
    "calculator_add_entry",
    "calculator_set_favorite",
    "calculator_delete",
    "calculator_clear_history",
    "calculator_get_settings",
    "calculator_set_settings",
  ]) {
    assert.match(commands![1], new RegExp(`"${command}"`), `${command} must be allowlisted`);
  }
});

test("the backend reports the calculator available, since it has no switch", async () => {
  const rust = stripJsComments(await read("src-tauri/src/plugin_pages.rs"));
  // The `enabled` field's match arm. A plugin with no persisted field must not
  // fall through the `_ => false` default and describe itself as off.
  assert.match(
    rust,
    /CALCULATOR_PLUGIN_ID => true,/,
    "the calculator is always available; it has no persisted switch to read",
  );
});

// ── 2 · the launcher row ──────────────────────────────────────────────────

test("the catalog declares one calculator system row", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  const table = /const SYSTEM_COMMANDS[\s\S]*?\n\];/.exec(catalog);
  assert.ok(table, "the system-command table must exist");
  assert.match(table![0], /action: "calculator"/, "the calculator is one of its entries");
  assert.match(table![0], /titleKey: "system\.calculator"/);
  assert.match(table![0], /subtitleKey: "system\.calculatorSubtitle"/);
  // The trigger vocabulary — the words the plugin's own mode is entered by —
  // is also what the search matches, so typing what you would type to use it
  // also *finds* it.
  for (const name of ["calculator", "calc", "计算器", "计算"]) {
    assert.match(table![0], new RegExp(`"${name}"`), `${name} must be a search name`);
  }
  assert.match(table![0], /initials: "[a-z]+"/, "the row carries a pinyin/English initials key");
});

test("entering the calculator row enters the plugin's own mode", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  // The row is a door, not a power action: like the browser and clipboard
  // branches it calls `enterPluginMode` and returns before the confirmation
  // path could arm anything.
  assert.match(
    actions,
    /item\.action === "calculator"[\s\S]{0,220}?enterPluginMode\(\{ scope: "calculator", filter: "all" \}\)/,
    "Enter on the calculator row must enter the calculator mode",
  );
  // And the plugin door is not a power confirmation either.
  assert.match(
    actions,
    /item\.action === "clipboard" \|\| item\.action === "browser" \|\| item\.action === "calculator"/,
    "the calculator joins the plugin doors that never reach the power confirmation",
  );
  // The `SystemAction` union names it, and the row gets a real glyph rather
  // than the clipboard fallback.
  const results = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(results, /export type SystemAction =[^;]*"calculator"/);
  assert.match(
    results,
    /action === "calculator" \? \(\s*<CalculatorIcon/,
    "the calculator row gets its own glyph, not the clipboard fallback",
  );
});

// ── 3 · both languages can name it ────────────────────────────────────────

test("the calculator's names exist in both dictionaries", () => {
  assert.equal(createTranslator("en")("system.calculator"), "Calculator");
  assert.equal(createTranslator("zh")("system.calculator"), "计算器");
  assert.equal(createTranslator("en")("settings.calculator"), "Calculator");
  assert.equal(createTranslator("zh")("settings.calculator"), "计算器");
  assert.equal(
    createTranslator("en")("notification.plugin.builtin.calculator"),
    "Calculator",
  );
  assert.equal(
    createTranslator("zh")("notification.plugin.builtin.calculator"),
    "计算器",
  );
});

test("the notification copy table names the calculator on both sides", async () => {
  assert.ok(
    NOTIFICATION_PLUGIN_IDS.includes(CALCULATOR_PLUGIN_ID),
    "a registered descriptor must be nameable in a notification",
  );
  const rust = stripJsComments(await read("src-tauri/src/notifications.rs"));
  assert.match(rust, /CALCULATOR_PLUGIN_ID, false\) => "Calculator"/);
  assert.match(rust, /CALCULATOR_PLUGIN_ID, true\) => "计算器"/);
});

// ── 4 · no switch: the honest absence ─────────────────────────────────────

test("the calculator ships without an `enabled` field, so no dead switch is drawn", async () => {
  // The row is `toggleable: false` and the app reports it `enabled: true`
  // because every registry row must report *something*. That is only honest
  // while there is no persisted field to contradict it — the moment one is
  // added, this case forces the switch (and its soft-close semantics) to be
  // built rather than faked.
  const config = stripJsComments(await read("src-tauri/src/commands/config.rs"));
  const block = /pub struct CalculatorPluginSettings \{([\s\S]*?)\n\}/.exec(config);
  assert.ok(block, "the calculator's settings block must exist");
  assert.ok(
    !/\benabled\b/.test(block![1]),
    "there is no `enabled` field — so the registry row must not draw a switch",
  );
});
