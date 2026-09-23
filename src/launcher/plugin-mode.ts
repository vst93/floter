// R28 · Plugin interaction as a base capability.
//
// Until this round each built-in plugin fused with the search box on its own
// terms: R26-A gave the browser mode an inline result list, R27 gave the
// clipboard mode a twin of it, and the two of them each built their own
// `LauncherItem` rows inside the catalog hook. The user asked for that
// arrangement to become a *capability* instead —
//
//   「需要把插件可在搜索框页面交互作为一种基础能力，这个技术能力需要支持两种展现
//     形式：1. 纯文本…… 2. 现在这种列表……」
//
// — so this module is the one place that answers the three questions a plugin
// no longer answers for itself:
//
//   1. **Form.** The plugin emits its output; the capability layer decides
//      whether that output is a *list* (an array of standard rows — or a JSON
//      string of one) or *text* (anything else, printed verbatim under the
//      field). 「按定义好的标准格式输出的，就以列表形式展示；否则，就全当普通
//      文本」.
//   2. **Tier.** A list is either `interactive` (rows carry ⌘N badges and run
//      on Enter — the R26/R27 behaviour) or `display` (rows are information
//      only: no selection, no shortcuts, nothing to run). A plugin may say
//      which; failing that, a list whose every row is a status line is
//      display-only by construction.
//   3. **Height.** Text gets a min height and a max height, scrolls past the
//      max, and folds into the launcher's discrete band table exactly as the
//      list does (see `result-budget.ts`), so a plugin's output can never
//      resize the window per keystroke.
//
// It is a pure module — no React, no Tauri, no DOM — for the same reason
// `result-budget.ts` and `file-drops.ts` are: the decisions above are the ones
// a review can silently delete, and a node test can only pin them if it can
// import them without dragging the app's runtime in.

import type { ClipboardEntry } from "../clipboard-history.ts";
import type { LauncherItem } from "./LauncherResults.tsx";
import { MAX_RESULTS, ROW_HEIGHT_TWO_LINE } from "./result-budget.ts";

/** The two shapes a plugin's output can take. */
export type PluginForm = "text" | "list";

/** The list form's two interaction tiers. The text form has only one — it is
 *  always display-only, because there are no rows to select. */
export type PluginTier = "display" | "interactive";

/**
 * R28 · one row of the standard structure a plugin emits.
 *
 * The capability layer knows two families — the browser plugin's rows and the
 * clipboard plugin's — and maps each to the matching `LauncherItem` variant
 * (see {@link pluginRowToItem}). The row is *data*: a plugin that emits rows
 * in this shape gets the launcher's list for free; a plugin that emits
 * anything else gets its output printed as text.
 */
export type PluginRow =
  | {
      family: "browser";
      id: string;
      title: string;
      subtitle?: string;
      /** A status line ("no browser found", "the plugin is off"), not a door. */
      disabled?: boolean;
      /** What Enter opens; `profileKey` says in which browser. */
      url: string;
      profileKey: string;
      /** Set on a row that is a tab the browser has open *now*: Enter switches
       *  to it rather than opening the URL a second time. */
      tab?: { browserId: string; windowIndex: number; tabIndex: number };
    }
  | {
      family: "clipboard";
      id: string;
      title: string;
      subtitle?: string;
      disabled?: boolean;
      /** What Enter copies back to the system clipboard. */
      entry?: ClipboardEntry;
    };

/**
 * R28 · what a plugin hands the launcher for one query.
 *
 * `output` is the plugin's own product and is deliberately `unknown`: the
 * capability layer — not the plugin — is what decides whether it reads as a
 * list. `tier` is the one piece of *intent* the layer cannot infer from the
 * data, and even it is optional (see {@link resolvePluginView}).
 */
export type PluginEmission = {
  readonly output: unknown;
  readonly tier?: PluginTier;
};

/** R28 · the text form's measurements, all in `--u` units. */
export type PluginTextMetrics = {
  /** How many lines the output has (explicit newlines only; wrapping is the
   *  layout's business and is handled by the block's own CSS). */
  lines: number;
  /** The block's height, clamped into `minUnits`..`maxUnits`. */
  heightUnits: number;
  /** The block's floor: never a one-pixel sliver of glass. */
  minUnits: number;
  /** The block's ceiling: past it the text scrolls inside the block. */
  maxUnits: number;
  /** Whether the output is taller than the ceiling. */
  scrolls: boolean;
  /** The band-table row count this block stands for (see
   *  {@link pluginViewRows}). */
  rows: number;
};

/** R28 · what the launcher should draw for a plugin's emission. */
export type PluginView =
  | { form: "list"; tier: PluginTier; items: LauncherItem[] }
  | { form: "text"; tier: "display"; text: string; metrics: PluginTextMetrics };

/** R28 · a text line, in `--u` units. A code-ish line at the body size: the
 *  same rhythm a row's subtitle has, without the row's padding. */
export const PLUGIN_TEXT_LINE_UNITS = 24;

/** R28 · the text block's minimum: three lines. The user's first requirement is
 *  explicit about it — 「一个最小高度也要限制」 — because a one-line command
 *  output in a zero-height box reads as a rendering bug. */
export const PLUGIN_TEXT_MIN_UNITS = PLUGIN_TEXT_LINE_UNITS * 3;

/** R28 · the text block's maximum: the same ceiling the nine-row list has, so a
 *  long output scrolls inside the block rather than growing the window. */
export const PLUGIN_TEXT_MAX_UNITS = MAX_RESULTS * ROW_HEIGHT_TWO_LINE;

/** Count the explicit lines of a block of text. An empty string has none. */
export const pluginTextLineCount = (text: string): number =>
  text.length === 0 ? 0 : text.split("\n").length;

/** The text form's measurements: the min/max clamp and the band row count. */
export const pluginTextMetrics = (text: string): PluginTextMetrics => {
  const lines = pluginTextLineCount(text);
  const raw = lines * PLUGIN_TEXT_LINE_UNITS;
  const heightUnits = Math.max(
    PLUGIN_TEXT_MIN_UNITS,
    Math.min(raw, PLUGIN_TEXT_MAX_UNITS),
  );
  return {
    lines,
    heightUnits,
    minUnits: PLUGIN_TEXT_MIN_UNITS,
    maxUnits: PLUGIN_TEXT_MAX_UNITS,
    scrolls: raw > PLUGIN_TEXT_MAX_UNITS,
    rows: Math.ceil(heightUnits / ROW_HEIGHT_TWO_LINE),
  };
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

/** Whether one value conforms to the standard row structure. */
const isPluginRow = (value: unknown): value is PluginRow => {
  if (!isRecord(value)) return false;
  if (typeof value.id !== "string" || typeof value.title !== "string") return false;
  if (value.family === "browser") {
    return typeof value.url === "string" && typeof value.profileKey === "string";
  }
  if (value.family === "clipboard") return true;
  return false;
};

/**
 * Parse a plugin's output as the standard structure.
 *
 * The two accepted shapes are an array of {@link PluginRow}s and a *string* of
 * one — a command-line plugin prints JSON, and this is where that JSON becomes
 * a list. An array with a single non-conforming element is not a list: the
 * whole output falls to the text form, which is the rule the user set
 * (「否则，就全当普通文本」) and the one thing a half-parsed list would break.
 */
export const asPluginRows = (output: unknown): PluginRow[] | null => {
  const value = typeof output === "string" ? parseJson(output) : output;
  if (!Array.isArray(value) || value.length === 0) return null;
  return value.every(isPluginRow) ? (value as PluginRow[]) : null;
};

/** `JSON.parse` only when the string even looks like an array, so an ordinary
 *  line of command output is never mistaken for a malformed one. */
const parseJson = (text: string): unknown => {
  const trimmed = text.trim();
  if (!trimmed.startsWith("[")) return null;
  try {
    return JSON.parse(trimmed);
  } catch {
    return null;
  }
};

/**
 * The text form's payload: the output as a printable string, or `null` when
 * there is nothing to print.
 *
 * A non-conforming structure is printed as pretty JSON rather than dropped —
 * 「全当普通文本」 means the user still sees what the plugin produced, even
 * when the launcher cannot make a list of it.
 */
export const asPluginText = (output: unknown): string | null => {
  if (output === null || output === undefined) return null;
  if (typeof output === "string") return output;
  if (typeof output === "number" || typeof output === "boolean" || typeof output === "bigint") {
    return String(output);
  }
  try {
    return JSON.stringify(output, null, 2);
  } catch {
    return null;
  }
};

/** The `LauncherItem` a standard row becomes. This is the row→launcher mapping
 *  R26-A/R26-B/R27 each kept for themselves; it lives here now, so the two
 *  plugins emit data and this one function is what the launcher reads. */
export const pluginRowToItem = (row: PluginRow): LauncherItem => {
  if (row.family === "browser") {
    return {
      type: "browser",
      id: row.id,
      title: row.title,
      subtitle: row.subtitle ?? "",
      url: row.url,
      profileKey: row.profileKey,
      ...(row.disabled ? { disabled: true } : {}),
      ...(row.tab ? { tab: row.tab } : {}),
    };
  }
  return {
    type: "clipboard",
    id: row.id,
    title: row.title,
    subtitle: row.subtitle ?? "",
    ...(row.entry ? { entry: row.entry } : {}),
    ...(row.disabled ? { disabled: true } : {}),
  };
};

/**
 * The list's tier, when the plugin did not name one: a list whose every row is
 * a status line is display-only. That is exactly the shape of the soft-closed
 * states — "no browser found", "nothing copied yet", "the plugin is off" — and
 * they should not pretend to be selectable.
 */
export const pluginTierFor = (rows: readonly PluginRow[], declared?: PluginTier): PluginTier =>
  declared ?? (rows.every((row) => row.disabled === true) ? "display" : "interactive");

/**
 * R28 · the capability layer's one entry point: turn a plugin's emission into
 * the view the launcher draws, or `null` when the plugin has nothing to say.
 *
 * Structure wins: an emission whose output conforms to the standard structure
 * becomes a list; everything else becomes text.
 */
export const resolvePluginView = (emission: PluginEmission | null): PluginView | null => {
  if (!emission) return null;
  // An empty structure is a list with nothing in it: the plugin has nothing to
  // say, and the launcher draws nothing rather than the literal `[]` as text.
  // A plugin with an empty state emits a status row instead (both built-ins do).
  if (Array.isArray(emission.output) && emission.output.length === 0) return null;
  const rows = asPluginRows(emission.output);
  if (rows) {
    return {
      form: "list",
      tier: pluginTierFor(rows, emission.tier),
      items: rows.map(pluginRowToItem),
    };
  }
  const text = asPluginText(emission.output);
  if (text === null || text === "") return null;
  return { form: "text", tier: "display", text, metrics: pluginTextMetrics(text) };
};

/** The rows a view stands for, for the band table. A text block stands for the
 *  band rows its clamped height occupies; a list for its own row count. */
export const pluginViewRows = (view: PluginView | null): number =>
  view === null ? 0 : view.form === "list" ? view.items.length : view.metrics.rows;

/** The `LauncherItem`s a view contributes to the numbered list. Text
 *  contributes none — it is drawn by its own block, not as rows. */
export const pluginViewItems = (view: PluginView | null): LauncherItem[] =>
  view !== null && view.form === "list" ? view.items : [];

/** Whether the view's rows take the keyboard: selection, Enter and ⌘N badges
 *  are the interactive tier's, and the display tier keeps none of them. */
export const pluginViewInteractive = (view: PluginView | null): boolean =>
  view !== null && view.form === "list" && view.tier === "interactive";
