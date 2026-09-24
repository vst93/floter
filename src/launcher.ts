import {
  cyclePluginFilter,
  pluginFilterAxis,
  type PluginFilterAxis,
} from "./plugins/filter-axis.ts";
import { splitTriggerWord } from "./plugins/mode-entry.ts";
import { matchesShortcut } from "./shortcuts.ts";
import type { CalculatorModeFilter } from "./calculator.ts";
export type { CalculatorModeFilter } from "./calculator.ts";

export type ExecutionMode = "pty" | "external";

export type ExecutionPlan = {
  program: string;
  args: string[];
  mode: ExecutionMode;
  cwd: string | null;
  environment: Record<string, string>;
  inheritEnvironment: boolean;
  planToken?: string;
  argumentOverride?: string[];
};

export type CompletionItem = {
  value: string;
  label: string;
  description: string;
};

export type ActionBarKind =
  | "shell"
  | "url"
  | "path"
  | "restart"
  | "shutdown"
  | "clipboard"
  // R26-A: the browser plugin's entry point. Like `clipboard`, the action bar
  // claims the bare trigger word and dispatches it to the result-mode switch.
  | "browser"
  // R7-10a: a dropped file's three actions. Not produced by
  // `classifyActionBar` — they are claimed by the file-drop layer — but they
  // are `ActionBarKind`s so the switcher is the existing action bar rather than
  // a second control with its own styling and keyboard rules.
  | "file-open"
  | "file-cd"
  | "file-copy";
export type CommandLineSyntax = "posix" | "windows";

export type LauncherSelection = {
  actionBar: boolean;
  resultIndex: number;
};

/** Number only runnable results, leaving unavailable discovery rows without a slot. */
export const launcherShortcutSlots = (runnableResults: boolean[]): Array<number | null> => {
  let shortcut = 0;
  return runnableResults.map((runnable) => {
    if (!runnable) return null;
    shortcut += 1;
    return shortcut;
  });
};

/** Rank application paths for the empty-query state: most launched first.
 *
 * Only paths with a count take a slot — an application never launched should
 * not crowd out one the user actually starts. Ties keep the scan order of
 * `paths`, so a stable application list yields a stable recent row.
 */
export const recentItems = (
  counts: Record<string, number>,
  paths: string[],
  limit: number,
): string[] =>
  paths
    .map((path, index) => ({ path, index, count: counts[path] ?? 0 }))
    .filter((entry) => entry.count > 0)
    .sort((a, b) => b.count - a.count || a.index - b.index)
    .slice(0, limit)
    .map((entry) => entry.path);

export const COMMAND_WORDS = new Set([
  "cd", "git", "npm", "ls", "cat", "echo", "curl", "wget", "ssh",
  "cp", "mv", "rm", "mkdir", "touch", "chmod", "grep", "find",
  "sed", "awk", "make", "docker", "kubectl", "python", "python3",
  "node", "go", "cargo", "brew", "apt", "yum", "pip", "yarn",
  "pnpm", "tar", "gzip", "unzip", "head", "tail", "wc", "sort",
  "uniq", "diff", "kill", "ps", "top", "df", "du", "free", "uname",
  "whoami", "hostname", "ping", "ifconfig", "ip", "netstat", "lsof",
  "systemctl", "journalctl", "man", "which", "whereis", "export",
  "source", "alias", "history", "sudo",
]);

const URL_QUERY = /^(?:https?|ftp):\/\//i;
const PATH_QUERY = /^[/~.]|^[A-Za-z]:[\\/]|^\\\\/;
const ALIAS_SCORE_CAP = 690;

/**
 * Power / clipboard queries that the action bar should claim and dispatch to
 * the same handler `runLauncherItem` uses for the matching system result.
 *
 * Mirrors the searchable names in `useLauncherCatalog.ts` so the action bar
 * matches what the result list would have shown — the user typing "restart"
 * reaches the power action whether it landed on the action bar or on the
 * numbered row above it.
 */
const SYSTEM_ACTION_QUERIES: Record<string, "restart" | "shutdown" | "clipboard" | "browser"> = {
  restart: "restart",
  reboot: "restart",
  shutdown: "shutdown",
  "shut down": "shutdown",
  "power off": "shutdown",
  clipboard: "clipboard",
  "clipboard history": "clipboard",
  "paste history": "clipboard",
  // R26-A: the browser plugin's trigger words. The bare word lands on the
  // action bar / the system row; typing the word plus a space enters the
  // plugin's result mode (see `useLauncherCatalog`). `history` is deliberately
  // absent: it is the shell's own command, and the browser history search
  // reaches the mode through the `history ` prefix instead.
  browser: "browser",
  bookmarks: "browser",
  bookmark: "browser",
  "浏览器": "browser",
  "书签": "browser",
};

export const normalizeSearch = (value: string): string =>
  value.toLowerCase().normalize("NFKC").replace(/[^\p{L}\p{N}]+/gu, " ").trim();

export type ParsedCommandLine = {
  tokens: string[];
  fragmentStart: number;
  /** Index of the executable token, after any leading NAME=value assignments. */
  commandIndex: number | null;
  /** Leading shell-style environment assignments, decoded as token values. */
  environment: Record<string, string>;
  /** Unquoted shell operators that require handing the whole line to a shell. */
  shellSyntax: boolean;
};

const ENVIRONMENT_ASSIGNMENT = /^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/;

/** Tokenize a command line for discovery without asking a shell to interpret it. */
export const parseCommandLine = (
  value: string,
  trailingEmpty = false,
  syntax: CommandLineSyntax = "posix",
): ParsedCommandLine => {
  const tokens: string[] = [];
  let token = "";
  let tokenStarted = false;
  let tokenStart = value.length;
  let quote: "'" | '"' | null = null;
  let escaped = false;
  let shellSyntax = false;

  const finishToken = () => {
    if (!tokenStarted) return;
    tokens.push(token);
    token = "";
    tokenStarted = false;
  };

  for (let index = 0; index < value.length; index += 1) {
    const char = value[index];
    if (escaped) {
      token += char;
      escaped = false;
      continue;
    }
    // Backslash is a path separator on Windows, not a general escape. Treating
    // it like POSIX shell syntax turns C:\Users into C:Users before a structured
    // execution plan reaches the backend.
    if (syntax === "posix" && char === "\\" && quote !== "'") {
      if (!tokenStarted) tokenStart = index;
      tokenStarted = true;
      escaped = true;
      continue;
    }
    if (char === "'" || char === '"') {
      // Windows completion values quote a literal double quote by doubling it.
      // Recognize that representation while already inside double quotes.
      if (syntax === "windows" && quote === '"' && char === '"' && value[index + 1] === '"') {
        token += '"';
        index += 1;
        continue;
      }
      if (!tokenStarted) tokenStart = index;
      tokenStarted = true;
      quote = quote === char ? null : quote ?? char;
      if (quote !== null && quote !== char) token += char;
      continue;
    }
    if (/\s/.test(char) && quote === null) {
      finishToken();
      continue;
    }
    if (quote === null && "|&;<>()".includes(char)) shellSyntax = true;
    if (!tokenStarted) tokenStart = index;
    tokenStarted = true;
    token += char;
  }
  if (escaped) token += "\\";
  finishToken();
  if (trailingEmpty && /\s$/.test(value)) {
    tokens.push("");
    tokenStart = value.length;
  }
  const environment: Record<string, string> = {};
  let commandIndex: number | null = null;
  for (let index = 0; index < tokens.length; index += 1) {
    const assignment = ENVIRONMENT_ASSIGNMENT.exec(tokens[index]);
    if (commandIndex === null && assignment) {
      environment[assignment[1]] = assignment[2];
      continue;
    }
    commandIndex = index;
    break;
  }
  return { tokens, fragmentStart: tokenStart, commandIndex, environment, shellSyntax };
};

const formatCompletionValue = (value: string, syntax: CommandLineSyntax): string => {
  // Keep simple CLI tokens readable. Everything else is quoted so a value
  // selected with Tab can be parsed back into the exact same argument.
  const simple = syntax === "windows"
    ? /^[\p{L}\p{N}_./\\:@%+=,-]+$/u
    : /^[\p{L}\p{N}_./:@%+=,-]+$/u;
  if (simple.test(value)) return value;
  if (syntax === "windows") return `"${value.replace(/"/g, '""')}"`;
  return `"${value.replace(/(["\\$`])/g, "\\$1")}"`;
};

export const completedCommandLine = (
  query: string,
  fragmentStart: number,
  completion: CompletionItem,
  syntax: CommandLineSyntax = "posix",
): string => {
  const value = formatCompletionValue(completion.value, syntax);
  const keepOpen = /[\\/]$/.test(completion.value);
  return `${query.slice(0, fragmentStart)}${value}${keepOpen ? "" : " "}`;
};

export const executionWithCompletion = <T extends { execution: ExecutionPlan | null }>(
  entry: T,
  currentTokens: string[],
  completion: CompletionItem,
): ExecutionPlan | null => {
  if (!entry.execution) return null;
  const completedArgs = [...currentTokens.slice(1)];
  if (completedArgs.length) completedArgs[completedArgs.length - 1] = completion.value;
  else completedArgs.push(completion.value);
  return {
    ...entry.execution,
    argumentOverride: completedArgs,
  };
};

/** Score two already-normalized strings for exact, prefix, contains and subsequence matches. */
export const scoreNormalized = (needle: string, haystack: string): number => {
  if (!needle || !haystack || haystack.length < needle.length) return 0;
  if (haystack === needle) return 1000;
  if (haystack.startsWith(needle)) return 900 - haystack.length;
  const contained = haystack.indexOf(needle);
  if (contained !== -1) return 700 - contained;

  let score = 0;
  let cursor = 0;
  for (const char of needle) {
    const index = haystack.indexOf(char, cursor);
    if (index === -1) return 0;
    score += index === cursor ? 12 : 5;
    cursor = index + 1;
  }
  return score;
};

export const scoreApp = (
  needle: string,
  names: string[],
  initials: string,
  aliases: string[],
): number => {
  let best = 0;
  for (const name of names) {
    best = Math.max(best, scoreNormalized(needle, name));
  }
  if (initials) {
    const score = scoreNormalized(needle, initials);
    best = Math.max(best, score >= 1000 ? 950 : score);
  }
  for (const alias of aliases) {
    const score = scoreNormalized(needle, alias);
    if (score >= 700) best = Math.max(best, Math.min(score, ALIAS_SCORE_CAP));
  }
  return best;
};

export const classifyActionBar = (value: string): ActionBarKind => {
  // Power and clipboard queries are claimed before the URL / path checks so
  // typing `restart` does not get re-routed through a shell. The action bar
  // dispatches them to the same handler `runLauncherItem` uses for the
  // matching system result row.
  const systemAction = SYSTEM_ACTION_QUERIES[normalizeSearch(value)];
  if (systemAction) return systemAction;
  if (URL_QUERY.test(value)) return "url";
  if (PATH_QUERY.test(value)) return "path";
  return "shell";
};

/** R26-A · the browser plugin's inline result mode. */
export type BrowserMode = {
  /** R32 · the four range filters the mode offers: `all` searches bookmarks and
   *  history together, `bookmarks`/`history` narrow to one source, and `tabs`
   *  shows only the browser's live tabs. The entry trigger picks the first
   *  three (`bookmarks ` / `history ` / `browser `); the chips and Tab switch
   *  between all four afterwards. */
  kind: "all" | "bookmarks" | "history" | "tabs";
  /** The text after the trigger word, already trimmed. Empty is the default
   *  view: the bookmarks bar plus the most recent history. */
  needle: string;
};

/** R32/R40 · the browser mode's four range filters as a declarative axis: the
 *  values Tab cycles and the i18n key each chip prints, in one place. R40 moved
 *  the label map out of the App and onto the axis (see `plugins/filter-axis.ts`)
 *  so the values, their order and their words cannot drift apart. The labels
 *  reuse the plugin's own scope words (`launcher.browserBookmarks`/`History`)
 *  so the chips and the result grouping name the same things the same way. */
export const BROWSER_FILTER_AXIS: PluginFilterAxis<BrowserMode["kind"]> = pluginFilterAxis(
  ["all", "bookmarks", "history", "tabs"] as const,
  {
    all: "launcher.browserAll",
    bookmarks: "launcher.browserBookmarks",
    history: "launcher.browserHistory",
    tabs: "launcher.browserTabs",
  },
);

/** R32 · the browser mode's four range filters, in the order Tab cycles them.
 *  `all` first because that is the shipped default and the entry point. */
export const BROWSER_FILTERS: readonly BrowserMode["kind"][] = BROWSER_FILTER_AXIS.values;

/** R32 · move one step through {@link BROWSER_FILTERS}, wrapping at the ends.
 *  `direction` is `1` for Tab and `-1` for Shift+Tab. R40 · the rule itself is
 *  the shared axis cycle (`plugins/filter-axis.ts`). */
export const cycleBrowserFilter = (
  kind: BrowserMode["kind"],
  direction: 1 | -1,
): BrowserMode["kind"] => cyclePluginFilter(BROWSER_FILTER_AXIS, kind, direction);

/** R32 · whether an empty-word Backspace should leave a plugin mode.
 *
 * The R31 rule is that backspacing the needle to empty *stops* in the mode
 * (「退格删空即停」) — the field's own change handler keeps the mode, and the way
 * out is Esc / Cmd+W. The user asked for one more step: with the field already
 * empty, another Backspace is a deliberate "there is nothing left to delete"
 * and leaves the plugin. This predicate is that rule, so the input's handler and
 * the window-level fallback cannot disagree about it. */
export const pluginModeExitOnBackspace = (
  mode: ActivePluginMode | null,
  needle: string,
): boolean => mode !== null && needle.length === 0;

/** Trigger words that enter the browser mode. Each must be followed by at
 *  least one space, so the bare word stays on the action bar / system row —
 *  the difference between `bookmarks` (a row you Enter) and `bookmarks rust`
 *  (already inside the mode). */
const BROWSER_BOOKMARK_TRIGGERS = new Set(["bookmarks", "bookmark", "书签"]);
const BROWSER_ALL_TRIGGERS = new Set(["browser", "浏览器"]);
// `history` is the shell's own command, so it only enters the mode once it has
// an argument: `history` runs `history`, `history rust` searches the browser.
const BROWSER_HISTORY_TRIGGERS = new Set(["history", "hist", "历史", "历史记录"]);

/**
 * Parse a query into a browser-mode request, or `null` when the query is not in
 * the mode.
 *
 * The trigger word must be followed by whitespace. That one character is what
 * lets the mode be entered deliberately — Enter on the `bookmarks` system row
 * rewrites the query to `bookmarks ` — and left again by deleting the space.
 * R40 · the split itself is the shared rule (`plugins/mode-entry.ts`).
 */
export const parseBrowserMode = (value: string): BrowserMode | null => {
  const split = splitTriggerWord(value);
  if (!split) return null;
  const { word, rest } = split;
  const needle = rest.trim();
  if (BROWSER_BOOKMARK_TRIGGERS.has(word)) return { kind: "bookmarks", needle };
  if (BROWSER_ALL_TRIGGERS.has(word)) return { kind: "all", needle };
  if (BROWSER_HISTORY_TRIGGERS.has(word)) return { kind: "history", needle };
  return null;
};

/** R27 · the clipboard plugin's inline result mode.
 *
 * The browser mode's twin: a trigger word followed by a space puts the
 * launcher's numbered list on the clipboard history, and the input keeps the
 * same field it always had — that *is* the fusion the user asked for
 * (「现有的"剪切板"和"书签搜索"这两个插件需要和搜索框进行融合」). The bare word
 * still runs/opens the panel: the mode is a deliberate place, entered by typing
 * the word and a space, or by the system row's Enter.
 *
 * `clip` is the short trigger; on Windows `clip` is also a real command, so the
 * bare word deliberately stays the shell's — only `clip ` (with the space)
 * enters the mode. */
export type ClipboardMode = {
  /** The text after the trigger word, already trimmed. Empty is the whole
   *  history. */
  needle: string;
  /** R38 · the chips' one selection. `all` is the shipped default; `favorites`
   *  is the retention view (favorites are exempt from pruning); the other four
   *  are the entry's own kind, with a text entry refined into `link` when its
   *  whole content is a URL. A single axis, exactly as the browser mode's
   *  range filter is — the old page's two-axis (scope × type) model is not
   *  resurrected. */
  filter: ClipboardModeFilter;
};

/** R38 · the clipboard mode's six filter chips, in the order the chips row and
 *  Tab cycle them. `all` first because it is the shipped default. */
export type ClipboardModeFilter =
  | "all"
  | "favorites"
  | "text"
  | "image"
  | "link"
  | "files";

/** R38/R40 · the clipboard mode's six filter chips as a declarative axis — the
 *  browser axis's twin. `all`/`favorites` are the mode's own words; the four
 *  kind chips reuse the clipboard panel's type vocabulary (`clipboard.typeText`
 *  …), so the chips and a row's type name the same thing the same way. R40
 *  moved the label map out of the App and onto the axis. */
export const CLIPBOARD_FILTER_AXIS: PluginFilterAxis<ClipboardModeFilter> = pluginFilterAxis(
  ["all", "favorites", "text", "image", "link", "files"] as const,
  {
    all: "launcher.clipboardFilterAll",
    favorites: "launcher.clipboardFilterFavorites",
    text: "clipboard.typeText",
    image: "clipboard.typeImage",
    link: "clipboard.typeLink",
    files: "clipboard.typeFiles",
  },
);

/** R38 · the six filters, in display order. */
export const CLIPBOARD_FILTERS: readonly ClipboardModeFilter[] = CLIPBOARD_FILTER_AXIS.values;

/**
 * R38 · the clipboard mode's favorite key, in the same vocabulary a stored
 * shortcut uses. `CmdOrCtrl` resolves to ⌘ on macOS and Ctrl elsewhere, so the
 * one literal is right on every platform.
 *
 * It is deliberately **not** a member of `ShortcutAction`: it is a mode-local
 * key like R32's Tab (one surface, one meaning), and adding it to the settings
 * map would be a settings migration for a control the user never asked to
 * rebind. `⌘D` is the bookmark convention the star echoes.
 */
export const CLIPBOARD_FAVORITE_SHORTCUT = "CmdOrCtrl+D";

/** R38 · move one step through {@link CLIPBOARD_FILTERS}, wrapping at the ends.
 *  `direction` is `1` for Tab and `-1` for Shift+Tab, the same contract as
 *  {@link cycleBrowserFilter}. R40 · the rule itself is the shared axis cycle. */
export const cycleClipboardFilter = (
  filter: ClipboardModeFilter,
  direction: 1 | -1,
): ClipboardModeFilter => cyclePluginFilter(CLIPBOARD_FILTER_AXIS, filter, direction);

const CLIPBOARD_TRIGGERS = new Set(["clip", "clipboard", "剪贴板", "粘贴板"]);

/**
 * R50 · the calculator plugin's inline result mode.
 *
 * The clipboard mode's twin: a trigger word followed by a space puts the
 * launcher's numbered list on the calculation history, and the input keeps the
 * same field it always had. The field's text is the expression *and* the
 * history's search needle — Enter evaluates a fresh expression and copies the
 * selected history row once the expression has been evaluated (see
 * `calculatorEnterAction`).
 */
export type CalculatorMode = {
  /** The text after the trigger word, already trimmed. It is both the pending
   *  expression and the history's needle. */
  needle: string;
  /** The chips' one selection: everything, or the starred entries. */
  filter: CalculatorModeFilter;
};

/** R50 · the calculator mode's two filter chips, in the order the chips row and
 *  Tab cycle them. `all` first because it is the shipped default. */
export const CALCULATOR_FILTER_AXIS: PluginFilterAxis<CalculatorModeFilter> =
  pluginFilterAxis(["all", "favorites"] as const, {
    all: "launcher.calculatorFilterAll",
    favorites: "launcher.calculatorFilterFavorites",
  });

/** R50 · the two filters, in display order. */
export const CALCULATOR_FILTERS: readonly CalculatorModeFilter[] = CALCULATOR_FILTER_AXIS.values;

/** R50 · move one step through {@link CALCULATOR_FILTERS}, wrapping at the ends
 *  — the browser and clipboard cycles' contract, applied to the calculator's
 *  two chips. */
export const cycleCalculatorFilter = (
  filter: CalculatorModeFilter,
  direction: 1 | -1,
): CalculatorModeFilter => cyclePluginFilter(CALCULATOR_FILTER_AXIS, filter, direction);

/**
 * R50 · the calculator mode's favorite key, the clipboard's twin. `CmdOrCtrl+D`
 * resolves to ⌘D on macOS and Ctrl+D elsewhere. It is deliberately **not** a
 * member of `ShortcutAction`: it is a mode-local key like the browser's Tab.
 */
export const CALCULATOR_FAVORITE_SHORTCUT = "CmdOrCtrl+D";

/**
 * R50/R53/R55 · the delete-one key, shared by the calculator and clipboard
 * history lists. `CmdOrCtrl+Backspace` resolves to ⌘⌫ on macOS and Ctrl+⌫
 * elsewhere — the app modifier, the same key the rest of the launcher's
 * commands use. R53 stepped off ⌘⌫ because macOS binds it to "delete to the
 * start of the line" in a text field, so trimming what was typed armed a
 * destructive row delete. R55 puts it back on the user's explicit request —
 * 「之前改的删除快捷键改成 cmd+删除」 — and mitigates the collision structurally
 * instead of avoiding it: the handler only claims ⌘⌫ while the field is *empty*
 * and the selection sits on a runnable history row, so a hand deleting text
 * (the field has characters) keeps the editing gesture and never arms a row.
 * The full argument lives in `plugins/history-actions.ts`. Pinned here so the
 * key handler, the hint, the clipboard page and the documentation cannot drift.
 */
export const HISTORY_DELETE_SHORTCUT = "CmdOrCtrl+Backspace";

/** Whether a key event's modifier-and-key shape is {@link HISTORY_DELETE_SHORTCUT}.
 *
 * Exported so the clipboard plugin page (`clipboard-list.ts`) recognises the
 * *same* binding the launcher does rather than keeping its own copy of ⌘⌫/Ctrl+⌫
 * — one key, one definition. The input is structurally an event (the page's
 * pure resolver never holds a real `KeyboardEvent`). */
export const isHistoryDeleteKey = (
  event: Pick<KeyboardEvent, "key" | "code" | "ctrlKey" | "metaKey" | "altKey" | "shiftKey">,
): boolean => matchesShortcut(event as KeyboardEvent, HISTORY_DELETE_SHORTCUT);

const CALCULATOR_TRIGGERS = new Set(["calc", "calculator", "计算器", "计算", "="]);

/** Parse a query into a calculator-mode request, or `null` when the query is
 *  not in the mode. Same rule as the other modes: the trigger word must be
 *  followed by whitespace. */
export const parseCalculatorMode = (value: string): CalculatorMode | null => {
  const split = splitTriggerWord(value);
  if (!split) return null;
  if (!CALCULATOR_TRIGGERS.has(split.word)) return null;
  return { needle: split.rest.trim(), filter: "all" };
};

/** Parse a query into a clipboard-mode request, or `null` when the query is not
 *  in the mode. Same rule as the browser mode: the trigger word must be
 *  followed by whitespace, which is what makes entering and leaving it a single
 *  keystroke. */
export const parseClipboardMode = (value: string): ClipboardMode | null => {
  const split = splitTriggerWord(value);
  if (!split) return null;
  if (!CLIPBOARD_TRIGGERS.has(split.word)) return null;
  // R38 · entering the mode always starts on 全部; the chips switch it after.
  return { needle: split.rest.trim(), filter: "all" };
};

/**
 * R31 · the plugin mode the launcher is *in*, held as explicit state rather than
 * re-derived from the query on every render.
 *
 * R26/R27 fused the plugins into the search box by letting the mode word live in
 * the field: `browser ` entered the browser mode, `clip ` the clipboard one, and
 * `parseBrowserMode`/`parseClipboardMode` read the mode back out of the query on
 * every keystroke. That is a clean way to *enter* a mode, but it makes the mode
 * word part of the visible query — so the user types `browser rust` and reads
 * `browser rust` in a box that is already the browser's own search field, with
 * the scope glyph beside it saying the same thing twice (「既然概念上是已经进入插件
 * 了，输入框左侧只保留插件信息就行了，browser 这段就可以不用了」).
 *
 * So the mode is lifted out of the text and stored: entering it strips the
 * trigger word (the needle the user typed stays), the field shows only the
 * needle, and leaving it (Esc / Cmd+W) puts the needle back as ordinary text.
 * The trigger vocabulary still belongs to the parsers above — this type is the
 * *state*, and {@link pluginModeEntry} is the one transition into it.
 */
export type ActivePluginMode =
  | { scope: "browser"; kind: BrowserMode["kind"] }
  | { scope: "clipboard"; filter: ClipboardModeFilter }
  /** R50 · the calculator plugin's mode. The field's text is both the pending
   *  expression and the history filter. */
  | { scope: "calculator"; filter: CalculatorModeFilter }
  /**
   * R39 · an *external* plugin's command mode. The plugin is not a built-in;
   * it is an integration whose provider descriptor declared this command and
   * whose per-command switch the user turned on (see `plugins/external.ts`).
   * The field's text is the command's argv, and Enter runs it.
   */
  | { scope: "external"; extensionId: string; commandId: string };

/**
 * R31 · the transition into a mode: a typed value whose first word is a trigger
 * and whose next character is whitespace enters that plugin, and the rest of the
 * value is the needle. `null` when the value is not a mode entry.
 *
 * This reuses the two parsers rather than restating their vocabulary, so the
 * words that enter a mode and the words the mode was always entered by cannot
 * drift apart. The browser parser owns the three browser triggers
 * (`browser `/`bookmarks `/`history ` and their Chinese spellings); the
 * clipboard parser owns `clip `/`clipboard `/`剪贴板 `/`粘贴板 `.
 *
 * The bare word is deliberately *not* an entry: `history` is the shell's own
 * command, `clip` is a real Windows command, and `bookmarks` is the system row
 * the user Enters — each of those keeps its existing behaviour, and the space is
 * what makes the mode deliberate.
 */
export const pluginModeEntry = (
  value: string,
): { mode: ActivePluginMode; needle: string } | null => {
  const browser = parseBrowserMode(value);
  if (browser) return { mode: { scope: "browser", kind: browser.kind }, needle: browser.needle };
  const clipboard = parseClipboardMode(value);
  if (clipboard) {
    return { mode: { scope: "clipboard", filter: clipboard.filter }, needle: clipboard.needle };
  }
  const calculator = parseCalculatorMode(value);
  if (calculator) {
    return { mode: { scope: "calculator", filter: calculator.filter }, needle: calculator.needle };
  }
  return null;
};

/** R31 · the browser request an active mode + the field's own text stand for.
 *  This is the shape `useLauncherCatalog` reads, so the hook no longer has to
 *  re-parse a query that carries a mode word. `null` outside the browser mode. */
export const browserModeFor = (
  mode: ActivePluginMode | null,
  needle: string,
): BrowserMode | null =>
  mode?.scope === "browser" ? { kind: mode.kind, needle: needle.trim() } : null;

/** R31 · the clipboard request an active mode + the field's own text stand for.
 *  `null` outside the clipboard mode. */
export const clipboardModeFor = (
  mode: ActivePluginMode | null,
  needle: string,
): ClipboardMode | null =>
  mode?.scope === "clipboard"
    ? { needle: needle.trim(), filter: mode.filter }
    : null;

/** R50 · the calculator request an active mode + the field's own text stand
 *  for. `null` outside the calculator mode. */
export const calculatorModeFor = (
  mode: ActivePluginMode | null,
  needle: string,
): CalculatorMode | null =>
  mode?.scope === "calculator"
    ? { needle: needle.trim(), filter: mode.filter }
    : null;

/** R39 · the external plugin request an active mode + the field's own text stand
 *  for. The field's text is split into argv items by the caller
 *  (`splitPluginCommandArgs` in `plugins/external.ts`), because the split is the
 *  external protocol's business and this module stays free of it. `null` outside
 *  an external mode. */
export const externalModeFor = (
  mode: ActivePluginMode | null,
  needle: string,
): { extensionId: string; commandId: string; args: string } | null =>
  mode?.scope === "external"
    ? { extensionId: mode.extensionId, commandId: mode.commandId, args: needle.trim() }
    : null;

/** Decide which row a fresh query should select before the user navigates. */
export const shouldDefaultToActionBar = (
  query: string,
  actionKind: ActionBarKind,
  resultCount: number,
  runnableResultCount: number,
  hasRunnableCommandResult: boolean,
): boolean => {
  const value = query.trim();
  if (!value) return false;
  if (actionKind !== "shell" || resultCount === 0) return true;
  // Discovery keeps commands whose integration is currently unavailable in
  // the list. They must not capture Enter: unlike an application result or an
  // executable catalog plan, those rows cannot perform the launcher's job.
  if (hasRunnableCommandResult) return false;
  if (runnableResultCount === 0) return true;
  if (/\s/.test(value) || /[|>&]/.test(value)) return true;
  return COMMAND_WORDS.has(value.toLowerCase());
};

/** Move through runnable results and the action bar as one wrapping list. */
export const nextLauncherSelection = (
  runnableResults: boolean[],
  selectedResultIndex: number,
  selectedActionBar: boolean,
  hasActionBar: boolean,
  direction: -1 | 1,
): LauncherSelection => {
  const targets: LauncherSelection[] = runnableResults.flatMap((runnable, resultIndex) =>
    runnable ? [{ actionBar: false, resultIndex }] : [],
  );
  if (hasActionBar) targets.push({ actionBar: true, resultIndex: selectedResultIndex });
  if (!targets.length) return { actionBar: selectedActionBar, resultIndex: selectedResultIndex };

  const currentIndex = targets.findIndex((target) =>
    target.actionBar === selectedActionBar &&
    (target.actionBar || target.resultIndex === selectedResultIndex),
  );
  if (currentIndex >= 0) {
    return targets[(currentIndex + direction + targets.length) % targets.length];
  }

  // A result may have become unavailable between renders, or the pointer may
  // be hovering one for discovery. Continue in the requested visual direction
  // instead of making that disabled row part of the keyboard loop.
  if (direction > 0) {
    return targets.find((target) => !target.actionBar && target.resultIndex > selectedResultIndex)
      ?? targets.find((target) => target.actionBar)
      ?? targets[0];
  }
  for (let index = targets.length - 1; index >= 0; index -= 1) {
    const target = targets[index];
    if (!target.actionBar && target.resultIndex < selectedResultIndex) return target;
  }
  return targets.find((target) => target.actionBar) ?? targets[targets.length - 1];
};
