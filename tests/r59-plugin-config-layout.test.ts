// R59 · the plugin configuration overlay's grouped-card face.
//
// The user's report, verbatim: 「一些设置页面布局有问题」 — over a screenshot of
// the browser plugin's configuration overlay. Read against the DOM, it was four
// defects in one surface:
//
//   1. no card face: every field line sat directly on the card's glass, so the
//      wallpaper read through the whole panel and the overlay looked like a
//      different product from the settings pages;
//   2. collapsed rhythm: the text field's lower edge and the next group's title
//      visually touched;
//   3. no group hierarchy: the plugin's name and a field label were the same
//      weight, with nothing separating groups of fields;
//   4. ragged control widths: a select, a number-plus-unit, and a full-width
//      text box each chose their own right edge, and the number's unit hung
//      outside its box.
//
// What this file locks:
//   * the schema groups (browser: four titled sections; clipboard/calculator:
//     one untitled card);
//   * the overlay renders sections as `SettingsCard`s and fields as
//     `SettingsRow`s (`plugins/controls.tsx`), the settings app's own card
//     language, rather than bespoke field divs;
//   * the sheet paints the result panel's recess and the settings tokens;
//   * the window budget follows the new geometry — R58's "window == content"
//     for the two short forms, capped for the browser's long one.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import type { PluginConfigSchema } from "../src/plugins/config-schema.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, "");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const rules = (css: string) => {
  const out: { selector: string; body: string }[] = [];
  for (const match of stripComments(css).matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    out.push({ selector: match[1].trim().replace(/\s+/g, " "), body: match[2] });
  }
  return out;
};

const ruleFor = (css: string, selector: string) => {
  const found = rules(css).find(({ selector: s }) =>
    s.split(",").some((part) => part.trim() === selector),
  );
  assert.ok(found, `the sheet must still define ${selector}`);
  return found!;
};

/** Every declaration body whose selector group contains `selector`, joined — so
 *  a property split across a grouped rule and a solo rule is still found. */
const bodiesFor = (css: string, selector: string) =>
  rules(css)
    .filter(({ selector: s }) => s.split(",").some((part) => part.trim() === selector))
    .map(({ body }) => body)
    .join(";");

// ── A · the schema groups ─────────────────────────────────────────────────

test("R59 · the browser schema declares four sections in schema order", async () => {
  const { browserConfigSchema, configSections } = await import("../src/plugins/config-schema.ts");
  const sections = configSections(browserConfigSchema());
  assert.deepEqual(
    sections.map((section) => [section.key, section.fields.map((field) => field.key)]),
    [
      ["plugins.config.sectionGeneral", ["enabled"]],
      ["plugins.config.sectionData", ["target", "custom_base_dir"]],
      ["plugins.config.sectionSearch", ["history_days", "sort_order", "search_fields"]],
      ["plugins.config.sectionTabs", ["cdp_enabled", "cdp_port"]],
    ],
    "the eight browser fields group into four titled sections, in schema order",
  );
});

test("R59 · the two short schemas are one untitled card each", async () => {
  const {
    CLIPBOARD_CONFIG_SCHEMA,
    CALCULATOR_CONFIG_SCHEMA,
    configSections,
  } = await import("../src/plugins/config-schema.ts");
  for (const [schema, keys] of [
    [CLIPBOARD_CONFIG_SCHEMA, ["enabled", "max_items", "clear_history"]],
    [CALCULATOR_CONFIG_SCHEMA, ["max_items", "retention_days", "copy_mode", "clear_history"]],
  ] as const) {
    const sections = configSections(schema);
    assert.equal(sections.length, 1, "a short form is a light grouping: one card, no heading");
    assert.equal(sections[0].key, null);
    assert.deepEqual(sections[0].fields.map((field) => field.key), keys);
  }
});

test("R59 · configSections groups only adjacent fields with the same key", async () => {
  const { configSections } = await import("../src/plugins/config-schema.ts");
  const schema: PluginConfigSchema = {
    pluginId: "test",
    titleKey: "settings.calculator",
    fields: [
      { key: "a", type: "toggle", labelKey: "plugins.config.enabled", sectionKey: "plugins.config.sectionData" },
      { key: "b", type: "toggle", labelKey: "plugins.config.enabled", sectionKey: "plugins.config.sectionData" },
      { key: "c", type: "toggle", labelKey: "plugins.config.enabled", sectionKey: "plugins.config.sectionTabs" },
      { key: "d", type: "toggle", labelKey: "plugins.config.enabled", sectionKey: "plugins.config.sectionData" },
    ],
  };
  assert.deepEqual(
    configSections(schema).map((section) => [section.key, section.fields.map((field) => field.key)]),
    [
      ["plugins.config.sectionData", ["a", "b"]],
      ["plugins.config.sectionTabs", ["c"]],
      ["plugins.config.sectionData", ["d"]],
    ],
    "the grouping is a run, not a re-sort: field order is render order",
  );
});

// ── B · the overlay renders the card language ─────────────────────────────

test("R59 · the overlay groups the schema into SettingsCards", async () => {
  const overlay = stripJsComments(await read("src/plugins/PluginConfigOverlay.tsx"));
  assert.match(overlay, /import \{ SettingsCard \} from "\.\.\/settings\/SettingsRows"/, "the settings card is imported, not re-implemented");
  assert.match(overlay, /configSections\(schema\)/, "the overlay uses the one grouping rule");
  assert.match(overlay, /sections\.map\(/, "it maps sections, not raw fields");
  assert.match(overlay, /<SettingsCard>/, "each section is a SettingsCard");
  assert.match(overlay, /plugin-config__section-title/, "a titled section draws its heading");
  assert.match(overlay, /\{section\.key && \(/, "an untitled section draws no heading");
});

test("R59 · a row is a SettingsRow, with the settings selection controls reused", async () => {
  const controls = stripJsComments(await read("src/plugins/controls.tsx"));
  assert.match(controls, /import \{ SegmentedChoice, SettingsRow \} from "\.\.\/settings\/SettingsRows"/);
  assert.match(controls, /<SettingsRow\b/, "the field row is the settings primitive");
  assert.match(controls, /<SegmentedChoice\b/, "a radio is the settings segmented control");
  assert.match(controls, /className="settings-select plugin-config-select"/, "a select is the settings select face");
  // The bespoke row and radio markup is gone.
  assert.doesNotMatch(controls, /plugin-config-field__label/, "the hand-rolled label span is retired");
  assert.doesNotMatch(controls, /plugin-config-radio__option/, "the bespoke radio pill is retired");
  // Stacked kinds take the settings stacked row; the rest trail right.
  assert.match(controls, /const STACKED_KINDS = new Set<PluginConfigField\["type"\]>\(\[/);
  assert.match(controls, /"text",\s*"radio",\s*"checkboxes",\s*"slider",/, "the full-width kinds stack");
});

// ── C · the sheet: a face, not glass, and one control language ─────────────

test("R59 · the overlay paints the result panel's recess, not bare glass", async () => {
  const css = await read("src/styles/plugin-config.css");
  assert.match(
    ruleFor(css, ".plugin-config").body,
    /background:\s*var\(--surface-sunken\)/,
    "the overlay is a face (the same recess as the result panel and the settings body)",
  );
  assert.doesNotMatch(ruleFor(css, ".plugin-config").body, /backdrop-filter/, "no second material");
});

test("R59 · the section heading is the settings group title's face", async () => {
  const body = ruleFor(await read("src/styles/plugin-config.css"), ".plugin-config__section-title").body;
  assert.match(body, /font-size:\s*var\(--text-title\)/, "one step above a field label");
  assert.match(body, /font-weight:\s*620/, "the settings group title's weight");
  assert.match(body, /color:\s*var\(--text-strong\)/, "stronger than the muted meta line");
  const overlay = ruleFor(await read("src/styles/plugin-config.css"), ".plugin-config").body;
  assert.match(overlay, /--settings-card-inset:\s*calc\(var\(--u\) \* 14\)/, "the heading reads the card's own inset");
});

test("R59 · the field rows and their cards breathe", async () => {
  const css = await read("src/styles/plugin-config.css");
  const section = ruleFor(css, ".plugin-config__section").body;
  assert.match(section, /gap:\s*calc\(var\(--u\) \* 8\)/, "heading to card: the settings pages' 8u");
  const fields = ruleFor(css, ".plugin-config__fields").body;
  assert.match(fields, /gap:\s*calc\(var\(--u\) \* 12\)/, "card to card: 12u between sections");
  assert.match(fields, /padding:\s*calc\(var\(--u\) \* 4\)[^;]*calc\(var\(--u\) \* 12\)/, "the scroller keeps a top and bottom inset");
  assert.match(fields, /overflow-y:\s*auto/, "the long form still scrolls inside the face");
  assert.doesNotMatch(fields, /scrollbar-color/, "the global hover bar is left to match the settings page");
});

test("R59 · the controls take the settings field tokens and one right edge", async () => {
  const css = await read("src/styles/plugin-config.css");
  const text = bodiesFor(css, ".plugin-config-text");
  assert.match(text, /width:\s*100%/, "a text field is full width in its stacked row");
  assert.match(text, /height:\s*calc\(var\(--u\) \* 28\)/, "the settings field height");
  assert.match(text, /border:\s*1px solid var\(--glass-control-edge\)/, "the settings field hairline");
  assert.match(text, /box-shadow:\s*var\(--glass-field-shadow\)/, "the settings field inset shadow");
  // The number is a compound box: the unit rides inside the bordered control.
  const number = bodiesFor(css, ".plugin-config-number");
  assert.match(number, /border:\s*1px solid var\(--glass-control-edge\)/);
  assert.ok(
    rules(css).some(({ selector }) => selector === ".plugin-config-number:focus-within"),
    "the compound owns the focus ring",
  );
  const unit = ruleFor(css, ".plugin-config-number__unit").body;
  assert.match(unit, /color:\s*var\(--text-muted\)/, "the unit is a suffix label, not a loose word");
  assert.match(ruleFor(css, ".plugin-config-number__input").body, /text-align:\s*right/, "the digits hug the unit");
});

// ── D · the window budget follows the new geometry (R58) ──────────────────

test("R59 · the short forms land on their own height; the browser form caps at the slab", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const schemas = await import("../src/plugins/config-schema.ts");
  const { pluginConfigContentHeight, launcherWindowHeight } = budget;
  const slab = launcherWindowHeight(1);

  const clipboard = pluginConfigContentHeight(schemas.CLIPBOARD_CONFIG_SCHEMA.fields, 1, slab);
  const calculator = pluginConfigContentHeight(schemas.CALCULATOR_CONFIG_SCHEMA.fields, 1, slab);
  const browser = pluginConfigContentHeight(schemas.browserConfigSchema().fields, 1, slab);

  assert.ok(clipboard < slab, "the three-field clipboard form is shorter than the slab");
  assert.ok(calculator < slab, "the four-field calculator form is shorter than the slab");
  assert.equal(browser, slab, "the browser's eight fields roll past the slab and scroll");

  // A titled section is taller than an untitled one by exactly its heading.
  const untitled = [{ key: "x", type: "toggle" as const, labelKey: "plugins.config.enabled" as const }];
  const titled = [{ ...untitled[0], sectionKey: "plugins.config.sectionGeneral" as const }];
  assert.equal(
    pluginConfigContentHeight(titled, 1, 10_000) - pluginConfigContentHeight(untitled, 1, 10_000),
    budget.PLUGIN_CONFIG_SECTION_HEADING_UNITS,
    "the heading is charged only when the section is titled",
  );
});

test("R59 · the budget groups a schema the way the sheet does", async () => {
  const budget = await import("../src/launcher/result-budget.ts");
  const schemas = await import("../src/plugins/config-schema.ts");
  const { pluginConfigContentHeight, PLUGIN_CONFIG_CARD_BORDER_CHROME } = budget;
  // Two titled sections cost one more card edge than one section of the same
  // fields: the section edges are the only chrome the budget adds per section.
  const fields = schemas.browserConfigSchema().fields;
  const oneCard = fields.map((field) => ({ ...field, sectionKey: undefined }));
  const height = pluginConfigContentHeight(fields, 1, 10_000);
  const flat = pluginConfigContentHeight(oneCard, 1, 10_000);
  const titledSections = 4;
  assert.equal(
    height - flat,
    titledSections * budget.PLUGIN_CONFIG_SECTION_HEADING_UNITS +
      (titledSections - 1) * budget.PLUGIN_CONFIG_SECTION_GAP_UNITS +
      (titledSections - 1) * PLUGIN_CONFIG_CARD_BORDER_CHROME,
    "sections add headings, gaps and one card edge each beyond the first",
  );
});
