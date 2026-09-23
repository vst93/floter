// R29 · the generic plugin configuration.
//
// The user's two requirements, verbatim: 「1. 常见配置项通用组件化：对于常见的配置
// 项，做一些通用组件，比如单选、多选、下拉、进度条，以及文本框、数字等组件，做成
// 可定义化的配置。2. 配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，
// 而不是一个新的完全独立的页面。」
//
// The suite pins the declarative schema (the two built-ins' descriptions and the
// control kinds they use), the pure normalization every write goes through, and
// the wiring that turns a schema into the overlay — so a plugin that adds a
// field needs no control code and no new page.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import { BROWSER_PLUGIN_ID, CLIPBOARD_PLUGIN_ID } from "../src/plugin-pages.ts";
import {
  MAX_CLIPBOARD_MAX_ITEMS,
  MIN_CLIPBOARD_MAX_ITEMS,
} from "../src/clipboard-history.ts";
import {
  CLIPBOARD_CONFIG_SCHEMA,
  applyConfigChange,
  browserConfigSchema,
  configDefaults,
  configField,
  configValues,
  configValuesEqual,
  normalizeConfigValue,
  pluginConfigSchema,
  pluginConfigSchemas,
  pluginOptionLabel,
  type PluginConfigField,
} from "../src/plugins/config-schema.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const en = createTranslator("en");

const kinds = (fields: readonly PluginConfigField[]) => new Set(fields.map((field) => field.type));

// ── A · the two built-ins' descriptions ───────────────────────────────────

test("the clipboard schema declares the switch and the capacity slider", () => {
  assert.equal(CLIPBOARD_CONFIG_SCHEMA.pluginId, CLIPBOARD_PLUGIN_ID);
  const enabled = configField(CLIPBOARD_CONFIG_SCHEMA, "enabled");
  assert.ok(enabled && enabled.type === "toggle");
  const max = configField(CLIPBOARD_CONFIG_SCHEMA, "max_items");
  assert.ok(max && max.type === "slider");
  assert.equal(max.min, MIN_CLIPBOARD_MAX_ITEMS);
  assert.equal(max.max, MAX_CLIPBOARD_MAX_ITEMS);
  assert.equal(max.step, 10);
});

test("the browser schema declares every field the plugin block holds", () => {
  const schema = browserConfigSchema();
  assert.equal(schema.pluginId, BROWSER_PLUGIN_ID);
  const byKey = new Map(schema.fields.map((field) => [field.key, field]));
  assert.equal(byKey.get("enabled")?.type, "toggle");
  assert.equal(byKey.get("target")?.type, "select");
  assert.equal(byKey.get("custom_base_dir")?.type, "text");
  assert.equal(byKey.get("history_days")?.type, "number");
  assert.equal(byKey.get("sort_order")?.type, "radio");
  assert.equal(byKey.get("cdp_enabled")?.type, "toggle");
  assert.equal(byKey.get("cdp_port")?.type, "number");
  // The four orderings are all offered.
  const sort = byKey.get("sort_order");
  assert.ok(sort && sort.type === "radio");
  assert.deepEqual(sort.options.map((option) => option.value), [
    "relevance",
    "recent",
    "alphabetical",
    "visits",
  ]);
});

test("the two schemas between them exercise every control kind but checkboxes", () => {
  const used = new Set<string>();
  for (const schema of pluginConfigSchemas()) {
    for (const kind of kinds(schema.fields)) used.add(kind);
  }
  for (const kind of ["toggle", "select", "radio", "slider", "number", "text"]) {
    assert.ok(used.has(kind), `the built-ins must exercise the ${kind} control`);
  }
  // `checkboxes` is a supported kind the two built-ins do not need yet; its
  // control exists and is pinned in the source scan below.
});

test("the browser target dropdown is dynamic, auto first, and named by discovery", () => {
  const schema = browserConfigSchema({
    browserTargets: [
      { id: "chrome", name: "Google Chrome" },
      { id: "firefox", name: "Firefox" },
    ],
  });
  const target = configField(schema, "target");
  assert.ok(target && target.type === "select");
  assert.deepEqual(target.options.map((option) => option.value), ["auto", "chrome", "firefox"]);
  // `auto` is a dictionary word; a discovered browser keeps its own name.
  assert.equal(pluginOptionLabel(target.options[0], en), en("plugins.config.browserTargetAuto"));
  assert.equal(pluginOptionLabel(target.options[1], en), "Google Chrome");
});

test("pluginConfigSchema dispatches by id and refuses an unknown plugin", () => {
  assert.equal(pluginConfigSchema(CLIPBOARD_PLUGIN_ID)?.pluginId, CLIPBOARD_PLUGIN_ID);
  assert.equal(pluginConfigSchema(BROWSER_PLUGIN_ID)?.pluginId, BROWSER_PLUGIN_ID);
  assert.equal(pluginConfigSchema("builtin.nope"), null);
  assert.equal(pluginConfigSchema(null), null);
});

// ── B · the normalization every write goes through ────────────────────────

test("a toggle is a strict boolean", () => {
  const field: PluginConfigField = { key: "t", type: "toggle", labelKey: "plugins.config.enabled" };
  assert.equal(normalizeConfigValue(field, true), true);
  assert.equal(normalizeConfigValue(field, "true"), false);
  assert.equal(normalizeConfigValue(field, 1), false);
});

test("a text field trims and treats blank as unset", () => {
  const field: PluginConfigField = { key: "s", type: "text", labelKey: "plugins.config.customBaseDir" };
  assert.equal(normalizeConfigValue(field, "  /tmp/data  "), "/tmp/data");
  assert.equal(normalizeConfigValue(field, "   "), null);
  assert.equal(normalizeConfigValue(field, 7), null);
});

test("a slider snaps to its step and clamps to its bounds", () => {
  const field: PluginConfigField = {
    key: "n",
    type: "slider",
    labelKey: "plugins.config.clipboardMaxItems",
    min: 10,
    max: 500,
    step: 10,
  };
  assert.equal(normalizeConfigValue(field, 304), 300);
  assert.equal(normalizeConfigValue(field, 0), 10);
  assert.equal(normalizeConfigValue(field, 9999), 500);
  assert.equal(normalizeConfigValue(field, Number.NaN), 10);
});

test("a number field clamps without a step", () => {
  const field: PluginConfigField = {
    key: "p",
    type: "number",
    labelKey: "plugins.config.cdpPort",
    min: 1,
    max: 65535,
  };
  assert.equal(normalizeConfigValue(field, 9222), 9222);
  assert.equal(normalizeConfigValue(field, 0), 1);
  assert.equal(normalizeConfigValue(field, 70000), 65535);
});

test("a select and a radio fall back to their first option", () => {
  const options = [
    { value: "relevance", labelKey: "settings.browserSortRelevance" as const },
    { value: "recent", labelKey: "settings.browserSortRecent" as const },
  ];
  for (const type of ["select", "radio"] as const) {
    const field: PluginConfigField = { key: "o", type, labelKey: "plugins.config.sortOrder", options };
    assert.equal(normalizeConfigValue(field, "recent"), "recent");
    assert.equal(normalizeConfigValue(field, "sideways"), "relevance");
    assert.equal(normalizeConfigValue(field, null), "relevance");
  }
});

test("a checkbox group de-duplicates and keeps the schema's order", () => {
  const field: PluginConfigField = {
    key: "c",
    type: "checkboxes",
    labelKey: "plugins.config.sortOrder",
    options: [
      { value: "a", labelKey: "plugins.config.enabled" },
      { value: "b", labelKey: "plugins.config.enabled" },
      { value: "c", labelKey: "plugins.config.enabled" },
    ],
  };
  assert.deepEqual(normalizeConfigValue(field, ["c", "a", "c", "zzz"]), ["a", "c"]);
  assert.deepEqual(normalizeConfigValue(field, "a"), []);
});

test("a change is normalized, and an unknown key is ignored", () => {
  const schema = CLIPBOARD_CONFIG_SCHEMA;
  const start = configValues(schema, { enabled: true, max_items: 300 });
  const changed = applyConfigChange(schema, start, "max_items", 1234);
  assert.equal(changed.max_items, MAX_CLIPBOARD_MAX_ITEMS);
  assert.equal(changed.enabled, true);
  // A key the schema does not declare cannot enter the value object.
  assert.equal(applyConfigChange(schema, start, "ghost", 1), start);
});

test("the defaults are the shipped values", () => {
  const values = configDefaults(CLIPBOARD_CONFIG_SCHEMA);
  assert.equal(values.enabled, true);
  assert.equal(values.max_items, 300);
  const browser = configDefaults(browserConfigSchema());
  assert.equal(browser.target, "auto");
  assert.equal(browser.history_days, 30);
  assert.equal(browser.cdp_port, 9222);
});

test("configValuesEqual is the dirty check the save path uses", () => {
  const schema = CLIPBOARD_CONFIG_SCHEMA;
  const a = configValues(schema, { enabled: true, max_items: 300 });
  assert.equal(configValuesEqual(schema, a, { ...a }), true);
  assert.equal(configValuesEqual(schema, a, { ...a, enabled: false }), false);
});

// ── C · the wiring, pinned at the source ──────────────────────────────────

test("the controls module implements every declared kind", async () => {
  const source = await read("src/plugins/controls.tsx");
  for (const control of [
    "ToggleControl",
    "SelectControl",
    "RadioControl",
    "CheckboxesControl",
    "NumberControl",
    "SliderControl",
    "TextControl",
  ]) {
    assert.match(source, new RegExp(`function ${control}\\b`), `${control} must exist`);
  }
  assert.match(source, /export function PluginConfigRow/, "the overlay renders rows, not fields");
});

test("the overlay reads and writes each plugin's own narrow settings pair", async () => {
  const source = await read("src/plugins/PluginConfigOverlay.tsx");
  for (const command of [
    "clipboard_get_settings",
    "clipboard_set_settings",
    "browser_get_settings",
    "browser_set_settings",
    "browser_discover",
  ]) {
    assert.match(source, new RegExp(command), `the overlay must use ${command}`);
  }
  assert.match(source, /PluginConfigRow/, "the overlay renders schema fields generically");
});

test("the launcher's plugin mode shows a plugin-settings button, not the sessions entry", async () => {
  const source = await read("src/App.tsx");
  assert.match(source, /collapsed-card__settings--plugin/, "the plugin gear has its own hook");
  assert.match(source, /PluginConfigOverlay/, "the gear opens the generic overlay");
  assert.match(source, /setPluginConfigOpen/, "the overlay's open state is the launcher's");
  // The sessions button is inside the `launcherScope ? … : …` else-branch.
  assert.match(source, /launcherScope \? \(/, "the trailing controls switch on the scope");
});

test("the new configuration copy exists in both languages", async () => {
  const source = await read("src/i18n.ts");
  for (const key of [
    "plugins.config.open",
    "plugins.config.enabled",
    "plugins.config.clipboardMaxItems",
    "plugins.config.browserTarget",
    "plugins.config.historyDays",
    "plugins.config.sortOrder",
    "plugins.config.cdpPort",
    "launcher.scopeBrowser",
    "launcher.scopeClipboard",
    "launcher.pluginLoadingMore",
    "launcher.pluginEnd",
  ]) {
    const occurrences = source.split(`"${key}"`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared in both dictionaries`);
  }
});
