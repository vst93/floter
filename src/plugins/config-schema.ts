// R29 · the declarative configuration of a plugin.
//
// The user asked for two things at once:
//
//   「1. 常见配置项通用组件化：对于常见的配置项，做一些通用组件，比如单选、多选、
//     下拉、进度条，以及文本框、数字等组件，做成可定义化的配置。
//     2. 配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个
//     新的完全独立的页面。」
//
// — so a plugin no longer ships an HTML page that paints its own settings card.
// It ships a *description*: an ordered list of fields, each naming a control
// kind (`toggle`, `select`, `radio`, `slider`, `number`, `text`, `checkboxes`),
// a label, optional help, and the bounds or options the control needs. The
// generic overlay (`plugins/PluginConfigOverlay.tsx`) renders that description
// with the generic controls (`plugins/controls.tsx`); the plugin contributes no
// pixels and no event wiring.
//
// This module is pure — no React, no Tauri, no DOM — for the same reason
// `launcher/plugin-mode.ts` is: the schema *is* the contract, and a node test
// can only pin it if it can import it without the app runtime.
//
// The browser plugin's target dropdown is the one field whose options are not
// known until the machine has been scanned, so the schema is a function of a
// small context (the discovered browsers) rather than a frozen constant. Every
// other field is static.

import {
  MAX_CLIPBOARD_MAX_ITEMS,
  MIN_CLIPBOARD_MAX_ITEMS,
  DEFAULT_CLIPBOARD_MAX_ITEMS,
} from "../clipboard-history.ts";
import {
  CALCULATOR_RETENTION_DAYS,
  DEFAULT_CALCULATOR_MAX_ITEMS,
  DEFAULT_CALCULATOR_RETENTION_DAYS,
  MIN_CALCULATOR_MAX_ITEMS,
  MAX_CALCULATOR_MAX_ITEMS,
} from "../calculator.ts";
import {
  BROWSER_SEARCH_FIELDS,
  BROWSER_SORT_ORDERS,
  DEFAULT_BROWSER_SEARCH_FIELD,
  DEFAULT_CDP_PORT,
  MAX_HISTORY_DAYS,
  type BrowserSearchField,
  type BrowserSortOrder,
} from "../browser-page.ts";
import type { MessageKey } from "../i18n.ts";
import { BROWSER_PLUGIN_ID, CALCULATOR_PLUGIN_ID, CLIPBOARD_PLUGIN_ID } from "../plugin-pages.ts";

/** One choice of a `select`, `radio` or `checkboxes` field. */
export type PluginConfigOption = {
  value: string;
  labelKey: MessageKey;
  /** A discovered browser's own name. The one place a schema may carry
   *  user-visible text: the platform named it, and there is no dictionary key
   *  for a browser the user installed after this build shipped. */
  name?: string;
};

/**
 * One field of a plugin's configuration.
 *
 * `key` is both the field's identity in the value object and the name the
 * overlay hands back on a change. The control kinds are the ones the user
 * listed: a switch (`toggle`), single choice (`select` / `radio`), multiple
 * choice (`checkboxes`), a bounded value (`slider` / `number`) and free text
 * (`text`).
 */
/** The half of a field every control kind shares. R59 lifted it out so the
 *  `sectionKey` grouping half can be stated once: a field names the section it
 *  belongs to, and the overlay renders consecutive fields under one heading. */
type PluginConfigFieldBase = {
  key: string;
  labelKey: MessageKey;
  helpKey?: MessageKey;
  /** R59 · the section this field belongs to. Fields carry the same key in
   *  schema order and the overlay renders them under one heading, inside one
   *  `SettingsCard`; a new `sectionKey` (or the first field) starts a new
   *  group. The key is a dictionary key, so the two languages name a group the
   *  same way the rest of the overlay is named. Leaving it off puts the field
   *  in the card's untitled default group. */
  sectionKey?: MessageKey;
};

export type PluginConfigField = PluginConfigFieldBase &
  (
    | { type: "toggle" }
    | { type: "select"; options: readonly PluginConfigOption[] }
    | { type: "radio"; options: readonly PluginConfigOption[] }
    | { type: "checkboxes"; options: readonly PluginConfigOption[] }
    | {
        type: "slider" | "number";
        min: number;
        max: number;
        /** The increment; defaults to 1. A slider with a step of 10 lands on
         *  round numbers, which is what the clipboard capacity wants. */
        step?: number;
        /** A trailing unit word ("days", "items") rendered after the value. */
        unitKey?: MessageKey;
      }
    | { type: "text"; placeholderKey?: MessageKey }
    /**
     * R38 · a destructive command, not a value. The control is a button that arms
     * on first press and runs on the second (the overlay's own two-step confirm —
     * no system dialog), so a plugin can offer "clear history" without the
     * overlay growing a plugin-specific branch. `command` is the bridge command
     * the overlay invokes; the four keys are its label, help, confirm, cancel and
     * failure wording. An `action` field holds no value (`normalizeConfigValue`
     * returns `null`), so it never round-trips through the settings block.
     */
    | {
        type: "action";
        confirmKey: MessageKey;
        cancelKey: MessageKey;
        failedKey: MessageKey;
        command: string;
      }
  );

/** A field's value, as the overlay stores it. */
export type PluginConfigValue = boolean | string | number | string[] | null;

/** One plugin's whole configuration: an ordered field list. */
export type PluginConfigSchema = {
  pluginId: string;
  /** The overlay's title. */
  titleKey: MessageKey;
  fields: readonly PluginConfigField[];
};

/** R59 · one rendered section: a heading (when the group is named) and the
 *  fields under it. `configSections` is the only place the grouping rule is
 *  written down, so the overlay and the height budget cannot group a schema two
 *  different ways. */
export type PluginConfigSection = {
  /** The heading's dictionary key, or `null` for the untitled default group. */
  key: MessageKey | null;
  fields: PluginConfigField[];
};

/**
 * R59 · split a schema's ordered fields into sections.
 *
 * A new section starts at the first field and whenever `sectionKey` changes to
 * a value not already open *adjacent* to the previous field. Two fields with
 * the same key are one section; a field with no key joins the untitled default
 * section. Order is schema order, so a section is always contiguous — the
 * grouping never reorders a plugin's fields and cannot move a stored key.
 */
export const configSections = (
  schema: PluginConfigSchema,
): PluginConfigSection[] => {
  const sections: PluginConfigSection[] = [];
  for (const field of schema.fields) {
    const key = field.sectionKey ?? null;
    const last = sections[sections.length - 1];
    if (last && last.key === key) last.fields.push(field);
    else sections.push({ key, fields: [field] });
  }
  return sections;
};

/** What the schema needs to know about the machine. Only the browser target
 *  dropdown reads it; a plugin with no dynamic options ignores it. */
export type PluginConfigContext = {
  /** The discovered browsers, in discovery order (see `browserTargets`). */
  browserTargets?: readonly { id: string; name: string }[];
};

/** The clipboard plugin's configuration. `enabled` is the long-standing
 *  `clipboard_history_enabled` field; `max_items` is the plugin block's one
 *  value. */
export const CLIPBOARD_CONFIG_SCHEMA: PluginConfigSchema = {
  pluginId: CLIPBOARD_PLUGIN_ID,
  titleKey: "settings.clipboardHistory",
  fields: [
    {
      key: "enabled",
      type: "toggle",
      labelKey: "plugins.config.enabled",
      helpKey: "plugins.config.enabledHint",
    },
    {
      key: "max_items",
      type: "slider",
      labelKey: "plugins.config.clipboardMaxItems",
      helpKey: "plugins.config.clipboardMaxItemsHint",
      min: MIN_CLIPBOARD_MAX_ITEMS,
      max: MAX_CLIPBOARD_MAX_ITEMS,
      step: 10,
      unitKey: "plugins.config.unitItems",
    },
    // R38 · the plugin's one destructive action lives here, not on the list.
    {
      key: "clear_history",
      type: "action",
      labelKey: "plugins.config.clearHistory",
      helpKey: "plugins.config.clearHistoryHint",
      confirmKey: "plugins.config.clearHistoryConfirm",
      cancelKey: "plugins.config.clearHistoryCancel",
      failedKey: "clipboard.clearFailed",
      command: "clipboard_clear_history",
    },
  ],
};

/** The browser plugin's configuration, with its target dropdown built from the
 *  discovered browsers. `auto` is always first: it is the shipped default and
 *  the one choice that stays valid after a browser is uninstalled. */
export const browserConfigSchema = (
  context: PluginConfigContext = {},
): PluginConfigSchema => {
  const targets = context.browserTargets ?? [];
  return {
    pluginId: BROWSER_PLUGIN_ID,
    titleKey: "settings.browser",
    fields: [
      {
        key: "enabled",
        type: "toggle",
        labelKey: "plugins.config.enabled",
        helpKey: "plugins.config.enabledHint",
        sectionKey: "plugins.config.sectionGeneral",
      },
      {
        key: "target",
        type: "select",
        labelKey: "plugins.config.browserTarget",
        helpKey: "plugins.config.browserTargetHint",
        sectionKey: "plugins.config.sectionData",
        options: [
          { value: "auto", labelKey: "plugins.config.browserTargetAuto" },
          ...targets.map((target) => ({
            value: target.id,
            // A discovered browser's name is its own word, not a dictionary key.
            labelKey: "plugins.config.browserTargetAuto" as MessageKey,
            name: target.name,
          })),
        ],
      },
      {
        key: "custom_base_dir",
        type: "text",
        labelKey: "plugins.config.customBaseDir",
        helpKey: "plugins.config.customBaseDirHint",
        sectionKey: "plugins.config.sectionData",
        placeholderKey: "plugins.config.customBaseDirPlaceholder",
      },
      {
        key: "history_days",
        type: "number",
        labelKey: "plugins.config.historyDays",
        helpKey: "plugins.config.historyDaysHint",
        sectionKey: "plugins.config.sectionSearch",
        min: 0,
        max: MAX_HISTORY_DAYS,
        step: 1,
        unitKey: "plugins.config.unitDays",
      },
      {
        key: "sort_order",
        type: "radio",
        labelKey: "plugins.config.sortOrder",
        helpKey: "plugins.config.sortOrderHint",
        sectionKey: "plugins.config.sectionSearch",
        options: BROWSER_SORT_ORDERS.map((order) => ({
          value: order,
          labelKey: SORT_ORDER_KEYS[order],
        })),
      },
      {
        // R32 · which fields the launcher's browser search matches. The
        // clipboard mode deliberately does not read it (no URLs to search).
        key: "search_fields",
        type: "radio",
        labelKey: "plugins.config.searchFields",
        helpKey: "plugins.config.searchFieldsHint",
        sectionKey: "plugins.config.sectionSearch",
        options: BROWSER_SEARCH_FIELDS.map((field) => ({
          value: field,
          labelKey: SEARCH_FIELD_KEYS[field],
        })),
      },
      {
        key: "cdp_enabled", type: "toggle", labelKey: "plugins.config.cdpEnabled", helpKey: "plugins.config.cdpEnabledHint",
        sectionKey: "plugins.config.sectionTabs",
      },
      {
        key: "cdp_port",
        type: "number",
        labelKey: "plugins.config.cdpPort",
        helpKey: "plugins.config.cdpPortHint",
        sectionKey: "plugins.config.sectionTabs",
        min: 1,
        max: 65535,
        step: 1,
      },
    ],
  };
};

/** The i18n key each sort order prints, reusing the settings screen's own
 *  labels so the overlay and the settings page name the four the same way. */
const SORT_ORDER_KEYS: Record<BrowserSortOrder, MessageKey> = {
  relevance: "settings.browserSortRelevance",
  recent: "settings.browserSortRecent",
  alphabetical: "settings.browserSortAlphabetical",
  visits: "settings.browserSortVisits",
};

/** The i18n key each search field prints. */
const SEARCH_FIELD_KEYS: Record<BrowserSearchField, MessageKey> = {
  all: "plugins.config.searchFieldsAll",
  title: "plugins.config.searchFieldsTitle",
  url: "plugins.config.searchFieldsUrl",
};

/** The i18n key each age window prints. */
const CALCULATOR_RETENTION_KEYS: Record<number, MessageKey> = {
  0: "plugins.config.calculatorRetentionNever",
  1: "plugins.config.calculatorRetentionDay",
  7: "plugins.config.calculatorRetentionWeek",
  30: "plugins.config.calculatorRetentionMonth",
};

/** R50 · the calculator plugin's configuration. The capacity and the age
 *  window are the retention axes; `copy_mode` is what Enter copies; the action
 *  is the same two-step "clear history" control the clipboard uses (favorites
 *  survive it). */
export const CALCULATOR_CONFIG_SCHEMA: PluginConfigSchema = {
  pluginId: CALCULATOR_PLUGIN_ID,
  titleKey: "settings.calculator",
  fields: [
    {
      key: "max_items",
      type: "slider",
      labelKey: "plugins.config.calculatorMaxItems",
      helpKey: "plugins.config.calculatorMaxItemsHint",
      min: MIN_CALCULATOR_MAX_ITEMS,
      max: MAX_CALCULATOR_MAX_ITEMS,
      step: 10,
      unitKey: "plugins.config.unitItems",
    },
    {
      key: "retention_days",
      type: "select",
      labelKey: "plugins.config.calculatorRetention",
      helpKey: "plugins.config.calculatorRetentionHint",
      options: CALCULATOR_RETENTION_DAYS.map((days) => ({
        value: String(days),
        labelKey: CALCULATOR_RETENTION_KEYS[days],
      })),
    },
    {
      key: "copy_mode",
      type: "radio",
      labelKey: "plugins.config.calculatorCopyMode",
      helpKey: "plugins.config.calculatorCopyModeHint",
      options: [
        { value: "full", labelKey: "plugins.config.calculatorCopyFull" },
        { value: "result", labelKey: "plugins.config.calculatorCopyResult" },
      ],
    },
    {
      key: "clear_history",
      type: "action",
      labelKey: "plugins.config.clearHistory",
      helpKey: "plugins.config.clearHistoryHint",
      confirmKey: "plugins.config.clearHistoryConfirm",
      cancelKey: "plugins.config.clearHistoryCancel",
      failedKey: "calculator.clearFailed",
      command: "calculator_clear_history",
    },
  ],
};

/** Resolve a plugin id to its schema. `null` for a plugin that has no
 *  declarative configuration (there is nothing to show and no settings
 *  button). */
export const pluginConfigSchema = (
  pluginId: string | null | undefined,
  context: PluginConfigContext = {},
): PluginConfigSchema | null => {
  if (pluginId === CLIPBOARD_PLUGIN_ID) return CLIPBOARD_CONFIG_SCHEMA;
  if (pluginId === BROWSER_PLUGIN_ID) return browserConfigSchema(context);
  if (pluginId === CALCULATOR_PLUGIN_ID) return CALCULATOR_CONFIG_SCHEMA;
  return null;
};

/** Every built-in schema, for the node suite that pins them and for a caller
 *  that has to cover the registry. */
export const pluginConfigSchemas = (
  context: PluginConfigContext = {},
): readonly PluginConfigSchema[] => [
  CLIPBOARD_CONFIG_SCHEMA,
  browserConfigSchema(context),
  CALCULATOR_CONFIG_SCHEMA,
];

/** A display label for an option. A discovered browser carries its own `name`;
 *  everything else is a dictionary key. `name` is deliberately not part of
 *  {@link PluginConfigOption} — a plugin cannot smuggle user-visible text into
 *  a schema field the overlay would render without a translation. */
export const pluginOptionLabel = (
  option: PluginConfigOption & { name?: string },
  t: (key: MessageKey) => string,
): string => option.name ?? t(option.labelKey);

const clampStep = (value: number, field: Extract<PluginConfigField, { min: number }>): number => {
  const step = field.step && field.step > 0 ? field.step : 1;
  const snapped = Math.round((value - field.min) / step) * step + field.min;
  const clamped = Math.min(field.max, Math.max(field.min, snapped));
  // Floating steps can leave a 0.30000000000000004; round to the step's own
  // precision so a slider never writes a value the number input cannot show.
  return Math.round(clamped * 1e6) / 1e6;
};

/**
 * Coerce one raw value to the field's own type and bounds.
 *
 * Every write goes through here — the overlay's change handler, and the load
 * that reads the backend's answer — so a value the backend normalized and a
 * value the user picked are the same shape by the time they reach a control.
 * The rules mirror the Rust normalizers (`clampClipboardMaxItems`,
 * `normalizeBrowserSettings`): a malformed value falls back rather than
 * writing something the backend will silently change.
 */
export const normalizeConfigValue = (
  field: PluginConfigField,
  raw: unknown,
): PluginConfigValue => {
  switch (field.type) {
    case "toggle":
      return raw === true;
    case "action":
      // A command, not a value: nothing is stored and nothing round-trips.
      return null;
    case "text":
      return typeof raw === "string" && raw.trim() ? raw.trim() : null;
    case "slider":
    case "number": {
      const value = typeof raw === "number" && Number.isFinite(raw) ? raw : field.min;
      return clampStep(value, field);
    }
    case "select":
    case "radio": {
      const value = typeof raw === "string" ? raw : "";
      return field.options.some((option) => option.value === value)
        ? value
        : (field.options[0]?.value ?? "");
    }
    case "checkboxes": {
      if (!Array.isArray(raw)) return [];
      const allowed = new Set(field.options.map((option) => option.value));
      const picked = raw.filter((entry): entry is string => typeof entry === "string" && allowed.has(entry));
      // De-duplicate while preserving the schema's option order, so the stored
      // set reads the same however the user toggled the boxes.
      return field.options.map((option) => option.value).filter((value) => picked.includes(value));
    }
  }
};

/** The value object a schema starts from, given whatever the backend returned
 *  (or `{}` before it answers). Every field is present and normalized, so a
 *  control never receives `undefined`. */
export const configValues = (
  schema: PluginConfigSchema,
  raw: Record<string, unknown> = {},
): Record<string, PluginConfigValue> => {
  const values: Record<string, PluginConfigValue> = {};
  for (const field of schema.fields) values[field.key] = normalizeConfigValue(field, raw[field.key]);
  return values;
};

/** One field by key. */
export const configField = (
  schema: PluginConfigSchema,
  key: string,
): PluginConfigField | null => schema.fields.find((field) => field.key === key) ?? null;

/**
 * Apply one control's change to the value object.
 *
 * A change to a key the schema does not declare is ignored rather than added:
 * the value object is the schema's, and a stray key would be a field the
 * overlay can never show or clear.
 */
export const applyConfigChange = (
  schema: PluginConfigSchema,
  values: Record<string, PluginConfigValue>,
  key: string,
  raw: unknown,
): Record<string, PluginConfigValue> => {
  const field = configField(schema, key);
  if (!field) return values;
  return { ...values, [key]: normalizeConfigValue(field, raw) };
};

/** The shipped defaults for a schema, used before the backend answers and when
 *  an answer is unreadable. */
export const configDefaults = (
  schema: PluginConfigSchema,
): Record<string, PluginConfigValue> => {
  if (schema.pluginId === CLIPBOARD_PLUGIN_ID) {
    return configValues(schema, { enabled: true, max_items: DEFAULT_CLIPBOARD_MAX_ITEMS });
  }
  if (schema.pluginId === CALCULATOR_PLUGIN_ID) {
    return configValues(schema, {
      max_items: DEFAULT_CALCULATOR_MAX_ITEMS,
      retention_days: String(DEFAULT_CALCULATOR_RETENTION_DAYS),
      copy_mode: "full",
    });
  }
  return configValues(schema, {
    enabled: true,
    target: "auto",
    custom_base_dir: null,
    history_days: 30,
    sort_order: "relevance",
    search_fields: DEFAULT_BROWSER_SEARCH_FIELD,
    cdp_enabled: false,
    cdp_port: DEFAULT_CDP_PORT,
  });
};

/** Whether two value objects differ on any field the schema declares — the
 *  dirty check the overlay's save path uses to skip a no-op write. */
export const configValuesEqual = (
  schema: PluginConfigSchema,
  a: Record<string, PluginConfigValue>,
  b: Record<string, PluginConfigValue>,
): boolean =>
  schema.fields.every((field) => {
    const left = a[field.key];
    const right = b[field.key];
    if (Array.isArray(left) && Array.isArray(right)) {
      return left.length === right.length && left.every((value, index) => value === right[index]);
    }
    return left === right;
  });
