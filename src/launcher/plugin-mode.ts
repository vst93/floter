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
  /** R29 · the pagination state of a list emission. Omitted, the list is
   *  complete in one emission (the pre-R29 behaviour). Text emissions ignore
   *  it — there are no rows to page. */
  readonly page?: PluginPage;
};

/**
 * R29 · where a list emission sits in its own result set.
 *
 * The capability layer never interprets `cursor`: it is the plugin's opaque
 * continuation token, echoed back verbatim on the next request. That keeps the
 * protocol free of a paging strategy — offset-based (a numeric cursor) and
 * key-based (an opaque id) plugins both fit — while `hasMore` is the one bit
 * the launcher needs to draw its footer and to decide whether a scroll to the
 * bottom should ask for another page.
 */
export type PluginPage = {
  /** Opaque continuation token for the *next* page; `null` when this is the
   *  last one. A plugin that pages by offset simply uses the row count. */
  readonly cursor: string | null;
  /** Whether at least one more page exists. The launcher's footer and its
   *  scroll trigger both read this and nothing else. */
  readonly hasMore: boolean;
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

/** R28 · what the launcher should draw for a plugin's emission. R29 · a list
 *  carries its pagination state (`null` when the plugin does not page). */
export type PluginView =
  | { form: "list"; tier: PluginTier; items: LauncherItem[]; page: PluginPage | null }
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
      page: normalizePluginPage(emission.page),
    };
  }
  const text = asPluginText(emission.output);
  if (text === null || text === "") return null;
  return { form: "text", tier: "display", text, metrics: pluginTextMetrics(text) };
};

/** R29 · one page's worth of rows the launcher adds per scroll-to-bottom. The
 *  value is the launcher's own viewport height (`MAX_RESULTS`), so each page
 *  adds exactly one screenful and the box never has to grow for a page. */
export const PLUGIN_PAGE_SIZE = MAX_RESULTS;

/** R29 · how many pages the first emission carries. Two, so the initial list
 *  is taller than the box and the user has something to scroll — with one page
 *  the rows fit exactly and the "scroll to load more" trigger could never
 *  fire. */
export const PLUGIN_INITIAL_PAGES = 2;

/** R29 · how close to the bottom (in CSS pixels) the scroller must come before
 *  the launcher asks for the next page. A row's height, so the request lands as
 *  the last visible row is reached rather than after a rubber-band overshoot. */
export const PLUGIN_LOAD_MORE_THRESHOLD = 48;

/** R29 · accept only a well-formed pagination block. A cursor that is not a
 *  string is dropped (treated as the start), and `hasMore` is strictly boolean:
 *  guessing a continuation from malformed data is how a list silently loops. */
export const normalizePluginPage = (page: PluginPage | undefined): PluginPage | null => {
  if (!page || typeof page !== "object") return null;
  const cursor = typeof page.cursor === "string" ? page.cursor : null;
  return { cursor, hasMore: page.hasMore === true };
};

/** R29 · the pagination state of a list view, or `null` for text and for a
 *  list a plugin never asked to page. */
export const pluginViewPage = (view: PluginView | null): PluginPage | null =>
  view !== null && view.form === "list" ? view.page : null;

/** R29 · whether a scroll to the bottom should ask for another page. */
export const pluginViewHasMore = (view: PluginView | null): boolean =>
  pluginViewPage(view)?.hasMore === true;

/**
 * R29 · append one page of rows to the rows already held.
 *
 * Order is the plugin's; the merge only drops a row whose `id` already
 * appeared. Pages can legitimately overlap — offset pagination over a store
 * that shifted between requests repeats a row — and a list that showed the
 * same bookmark twice would be a bug the plugin cannot see. The first
 * occurrence wins, so an already-selected row keeps its position and index.
 */
export const mergePluginRows = (
  previous: readonly PluginRow[],
  next: readonly PluginRow[],
): PluginRow[] => {
  const seen = new Set(previous.map((row) => row.id));
  const merged = [...previous];
  for (const row of next) {
    if (seen.has(row.id)) continue;
    seen.add(row.id);
    merged.push(row);
  }
  return merged;
};

/**
 * R29 · the window of rows the launcher shows after `pagesLoaded` pages, and
 * the pagination block that goes with it.
 *
 * The window is a prefix, not a slice: page boundaries only ever grow the list,
 * so a row the user already sees never moves. `cursor` is the row count the
 * next page starts at — the offset strategy the two built-ins use — and is
 * `null` once the held rows are exhausted.
 */
export const paginatePluginRows = (
  rows: readonly PluginRow[],
  pagesLoaded: number,
  pageSize: number = PLUGIN_PAGE_SIZE,
): { rows: PluginRow[]; page: PluginPage } => {
  const end = Math.max(1, Math.trunc(pagesLoaded)) * pageSize;
  const visible = rows.slice(0, end);
  const hasMore = rows.length > visible.length;
  return {
    rows: visible,
    page: { cursor: hasMore ? String(visible.length) : null, hasMore },
  };
};

/** R29 · what the list's footer should say, if anything. `null` for a list that
 *  does not page at all; `"loading"` while a page is in flight; `"end"` once
 *  every row is shown. `"more"` is the quiet state — the scroll trigger is the
 *  affordance, not a button. */
export type PluginFooterState = "loading" | "more" | "end" | null;

export const pluginFooterState = (
  page: PluginPage | null,
  loadingMore: boolean,
): PluginFooterState => {
  if (page === null) return null;
  if (loadingMore) return "loading";
  return page.hasMore ? "more" : "end";
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
