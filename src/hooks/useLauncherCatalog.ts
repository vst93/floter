// React binding for the launcher's data layer: application scanning, icon
// loading, catalog search/completion, the ranked result list and the
// recent-command bookkeeping.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned value
// it touches, so the behaviour is unchanged.

import { useEffect, useMemo, useRef, useState, type Dispatch, type RefObject, type SetStateAction } from "react";
import { convertFileSrc, invoke } from "@tauri-apps/api/core";
import {
  classifyActionBar,
  completedCommandLine,
  executionWithCompletion,
  launcherShortcutSlots,
  normalizeSearch,
  parseBrowserMode,
  parseCommandLine,
  recentItems,
  scoreApp,
  shouldDefaultToActionBar,
  type BrowserMode,
  type CompletionItem,
  type ExecutionPlan,
} from "../launcher";
import {
  type ActionBar,
  type CommandWarning,
  type LauncherItem,
  type SystemAction,
} from "../launcher/LauncherResults";
import { appSubtitleKey } from "../launcher/row-content";
import { createSettingsHydration } from "../settings-persistence";
import { aliasToCommand, candidateMatchScore, commandMatchScore, matchedCommandAlias, rebaseAliasCommandLine, resolveCommandAliases, MATCH_EXACT, type CommandAliases } from "../command-aliases";
import { COMMAND_LIMIT_WITH_MATCHES, MAX_RESULTS } from "../launcher/result-budget";
import { IS_WINDOWS } from "../shortcuts";
import type { AppSettings, LocalApplication } from "../App";
import type { MessageKey, Translate } from "../i18n";

// R10-A/R19: `MAX_RESULTS` is 9 — eight matched result rows plus the one fixed
// clipboard row the App appends to the tail. Every slice below therefore keeps
// its `- 1`, so the catalog itself never returns more than those eight rows. The
// number lives in `launcher/result-budget.ts` beside the row that owns the last
// slot, so the budget and the row cannot drift apart; it is re-exported here
// because this module is where the result budget is read.
export { COMMAND_LIMIT_WITH_MATCHES, MAX_RESULTS };

/** Idle window before an icon is fetched, so the intermediate result lists that
 * flash past while a query is still being typed cost nothing. */
const ICON_LOAD_DELAY = 250;
/** Idle window before a query is searched. Long enough that an ordinary typing
 * cadence produces one result list — and so one window resize — per pause
 * rather than one per keystroke. */
const CATALOG_SEARCH_DELAY = 200;
const COMMAND_LINE_SYNTAX = IS_WINDOWS ? "windows" : "posix";

/** An application with its searchable names normalized once, up front. */
type SearchableApp = {
  app: LocalApplication;
  names: string[];
  initials: string;
  aliases: string[];
};

type CatalogSourceKind = "systemApplication" | "systemCommand" | "local" | "provider";

type CatalogArgument = {
  names: string[];
  kind: "flag" | "string" | "integer" | "number" | "path" | "directory" | "url" | "enum" | "command";
  description: string;
  takesValue: boolean;
  required: boolean;
  repeatable: boolean;
  values: string[];
  valueHint: string | null;
};

type CatalogEntry = {
  id: string;
  command: string;
  namespace: string;
  qualifiedCommand: string;
  name: string;
  description: string;
  sourceKind: CatalogSourceKind;
  sourceName: string;
  aliases: string[];
  arguments: CatalogArgument[];
  execution: ExecutionPlan | null;
  runtimeAvailable: boolean;
  frequency: number;
};

type CatalogCompletionResponse = { items: CompletionItem[]; dynamic: boolean };

type CatalogSuggestion =
  | { kind: "catalog"; entry: CatalogEntry }
  | {
      kind: "completion";
      entry: CatalogEntry;
      completion: CompletionItem;
      commandLine: string;
      execution: ExecutionPlan | null;
      dynamic: boolean;
    };

/** Answer to `check_applications`: whether a rescan would find anything new. */
type ApplicationsStatus = { upToDate: boolean; count: number };

/** R26-A · one row from `browser_search_bookmarks` / `browser_search_history`.
 *  The two commands share a shape; the fields one kind does not use are simply
 *  absent (`#[serde(skip_serializing_if)]` on the Rust side). */
type BrowserSearchRow = {
  id: string;
  title: string;
  url: string;
  profile_key: string;
};

/** R26-B · one row from `browser_list_tabs` — a tab the browser has open now.
 *  `window_index`/`tab_index` identify it for `browser_activate_tab`. */
type BrowserTabRow = {
  browser_id: string;
  window_index: number;
  tab_index: number;
  title: string;
  url: string;
  active: boolean;
};

/** How many browser rows to fetch. The launcher renders at most eight matched
 *  rows (`MAX_RESULTS - 1`), and the merge drops duplicate URLs, so a small
 *  over-fetch keeps the visible list full without an unbounded query. */
const BROWSER_FETCH_LIMIT = 24;

/** R26-B · the ceiling on one browser group. Bookmarks and history share the
 *  first group (bookmarks win, history fills); the live tabs are a second, so
 *  a browser with nothing open cannot hide the bookmarks and a browser with a
 *  full bookmark bar cannot hide the tabs. The launcher list scrolls when the
 *  two groups together outgrow its box. */
const BROWSER_GROUP_LIMIT = MAX_RESULTS - 1;

/**
 * The built-in power actions, searched like applications.
 *
 * `searchNames` carries the wording of *every* language rather than only the
 * current one: the UI language says nothing about the keyboard the query is
 * typed on, so "restart" has to find the entry on a Chinese UI and "关机" on an
 * English one. They are normalized here, once, for the same reason application
 * names are — see [`scoreNormalized`].
 *
 * `initials` is the pinyin key, the same shorthand [`compute_initials`] builds
 * for an application in the backend, and it exists for the same reason: a
 * Chinese name is unreachable from a Latin keyboard otherwise. It is written out
 * by hand because these two entries are the only names the frontend owns, and
 * shipping a pinyin table to the webview to spell four of them would cost more
 * than it saves. Only the Chinese spellings are covered — the English ones are
 * already whole entries in `searchNames`.
 */
const SYSTEM_COMMANDS: {
  action: SystemAction;
  titleKey: MessageKey;
  subtitleKey: MessageKey;
  searchNames: string[];
  initials: string;
}[] = [
  {
    action: "restart",
    titleKey: "system.restart",
    subtitleKey: "system.restartSubtitle",
    searchNames: ["restart", "reboot", "重启", "重新启动"].map(normalizeSearch),
    // 重启 → cq, 重新启动 → cxqd, then both again under 重's other reading. 重 is
    // chóng here and zhòng when it means "heavy", and which one a person reaches
    // for is a coin toss — an IME trains either. Both spellings are keys to the
    // same action, so both are in.
    initials: "cqcxqdzqzxqd",
  },
  {
    action: "shutdown",
    titleKey: "system.shutdown",
    subtitleKey: "system.shutdownSubtitle",
    searchNames: ["shutdown", "shut down", "power off", "关机", "关闭电脑"].map(normalizeSearch),
    // 关机 → gj, 关闭电脑 → gbdn. Neither character has a second reading.
    initials: "gjgbdn",
  },
  {
    action: "clipboard",
    titleKey: "system.clipboardHistory",
    subtitleKey: "system.clipboardHistorySubtitle",
    searchNames: [
      "clipboard",
      "clipboard history",
      "paste history",
      "剪贴板",
      "剪贴板历史",
      "粘贴历史",
    ].map(normalizeSearch),
    // 剪贴板 → jtb, 剪贴板历史 → jtbls, 粘贴历史 → ntls; plus the English
    // initials so `ch` reaches it too.
    initials: "chjtblsjtblsntls",
  },
  {
    // R26-A: the browser plugin's entry row. Entering it rewrites the query to
    // `browser ` and the launcher switches into the browser result mode. Bare
    // `history` is deliberately NOT a search name: it is the shell's command,
    // and the mode is reachable through `browser`, `bookmarks` or `history `.
    action: "browser",
    titleKey: "system.browserSearch",
    subtitleKey: "system.browserSearchSubtitle",
    searchNames: [
      "browser",
      "browser history",
      "browser bookmarks",
      "bookmarks",
      "bookmark",
      "浏览器",
      "浏览器书签",
      "书签",
      "历史记录",
    ].map(normalizeSearch),
    // 浏览器 → llq, 浏览器书签 → llqsq, 书签 → sq, 历史记录 → lsjl.
    initials: "llqllqsqsqlsjl",
  },
];

export function useLauncherCatalog(options: {
  query: string;
  /** `settings.launch_counts` — passed by identity so memoization behaves
   * exactly as when it was read off the settings state. */
  launchCounts: Record<string, number>;
  /** `settings.show_commands_in_search`. */
  showCommandsInSearch: boolean;
  /** `settings.show_recent_in_launcher` — whether the empty query offers the
   * most-launched applications at all. */
  showRecentInLauncher: boolean;
  /** `settings.command_aliases` — the raw per-command alias map. Passed by
   *  identity so an edit re-ranks the visible command rows immediately. */
  commandAliases: CommandAliases;
  t: Translate;
  settingsRef: RefObject<AppSettings>;
  settingsHydration: ReturnType<typeof createSettingsHydration<AppSettings>>;
  setSettings: Dispatch<SetStateAction<AppSettings>>;
  persistSettings: () => Promise<void>;
}) {
  const {
    query,
    launchCounts,
    showCommandsInSearch,
    showRecentInLauncher,
    commandAliases,
    t,
    settingsRef,
    settingsHydration,
    setSettings,
    persistSettings,
  } = options;

  /** Guards against two scans overlapping: a cold cache reads as out of date, so
   * a summon during the very first scan would otherwise start a second one. */
  const appScanning = useRef(false);
  const catalogRequestGeneration = useRef(0);
  const [applications, setApplications] = useState<LocalApplication[]>([]);
  /** True until the first scan settles, whether it hit the cache or not. */
  const [appsLoading, setAppsLoading] = useState(true);
  const [appsError, setAppsError] = useState(false);
  const [appIconUrls, setAppIconUrls] = useState<Record<string, string>>({});
  // Ref mirror of `appIconUrls` so the icon-loading effect can check which
  // icons are already resolved without subscribing to the state change —
  // without this, every icon that resolves re-triggers the effect, which
  // re-sets the timer, which delays every subsequent icon.
  const appIconUrlsRef = useRef(appIconUrls);
  const appIconAttempts = useRef(new Set<string>());
  useEffect(() => { appIconUrlsRef.current = appIconUrls; }, [appIconUrls]);
  const [catalogSuggestions, setCatalogSuggestions] = useState<CatalogSuggestion[]>([]);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const draftBeforeHistory = useRef("");

  // Normalizing every application name is done once per application list rather
  // than once per keystroke: it is the dominant cost of a search, and the list
  // only changes when applications are installed or removed.
  const searchableApps = useMemo<SearchableApp[]>(
    () =>
      applications.map((app) => ({
        app,
        // Deduplicated: an application with no localized name yields the same
        // normalized string from all three candidates.
        names: [
          ...new Set(
            [app.name, app.localizedName, `${app.localizedName ?? ""} ${app.name}`]
              .filter((name): name is string => Boolean(name))
              .map(normalizeSearch)
              .filter(Boolean),
          ),
        ],
        // Already lowercase and separator-free from the backend, so it needs no
        // normalizing of its own. Defensive against a cached list written before
        // the field existed.
        initials: app.initials || "",
        // Normalized like the names, and deduplicated against them: an alias
        // that repeats a name would only score the same match a second time,
        // under a lower ceiling.
        aliases: [
          ...new Set(
            (app.aliases ?? [])
              .map(normalizeSearch)
              .filter(Boolean),
          ),
        ],
      })),
    [applications],
  );

  // Catalog providers can perform I/O while loading their descriptors, so the
  // request shares one debounce window and stale responses are discarded.
  // Provider-connected and local commands are ALWAYS searchable — the user
  // explicitly connected them. Only the noisy system-command discovery stays
  // opt-in behind the Integrations settings toggle.
  useEffect(() => {
    const value = query.trim();
    const generation = ++catalogRequestGeneration.current;
    if (!value) {
      setCatalogSuggestions([]);
      return;
    }
    const includeSystemCommands = showCommandsInSearch;
    // R7-11: the conflict policy is applied once per query, then handed to the
    // backend so its ranking uses exactly the map the rows are ranked with.
    const resolvedAliases = resolveCommandAliases(commandAliases);

    setCatalogSuggestions([]);
    const timer = window.setTimeout(() => {
      const searchLine = parseCommandLine(query, false, COMMAND_LINE_SYNTAX);
      const completionLine = parseCommandLine(query, true, COMMAND_LINE_SYNTAX);
      const commandIndex = completionLine.commandIndex;
      const command = commandIndex === null ? "" : completionLine.tokens[commandIndex] ?? "";
      const structuredCommand = !searchLine.shellSyntax && commandIndex !== null;
      const searchTokens = searchLine.commandIndex === null
        ? []
        : searchLine.tokens.slice(searchLine.commandIndex);
      const completionTokens = commandIndex === null
        ? []
        : completionLine.tokens.slice(commandIndex);
      const wantsCompletion = structuredCommand && completionTokens.length > 1;
      // R7-11: a user alias is a search surface only. Completion is asked of
      // the provider by its *real* command id, so `gfm <Tab>` completes `git`'s
      // arguments rather than offering nothing for a command the descriptor
      // never heard of. The typed token itself is left in the row.
      const completionCommand = aliasToCommand(resolvedAliases)[command.toLowerCase()] ?? command;
      const search = structuredCommand ? invoke<CatalogEntry[]>("catalog_search", {
        request: {
          query,
          tokens: searchTokens,
          environment: searchLine.environment,
          cwd: null,
          limit: 20,
          includeSystemCommands,
          commandAliases: resolvedAliases,
        },
      }) : Promise.resolve<CatalogEntry[]>([]);
      const complete = wantsCompletion
        ? invoke<CatalogCompletionResponse>("catalog_complete", {
            request: {
              command: completionCommand,
              tokens: completionTokens,
              cwd: null,
            },
          }).catch(() => null)
        : Promise.resolve<CatalogCompletionResponse | null>(null);

      Promise.all([search, complete])
        .then(([entries, completion]) => {
          if (catalogRequestGeneration.current !== generation) return;
          // Flag on: everything except application entries (they have their own
          // result list). Flag off: only explicitly connected provider/local
          // commands — system commands stay hidden.
          const commands = entries.filter((entry) =>
            includeSystemCommands
              ? entry.sourceKind !== "systemApplication"
              : entry.sourceKind === "provider" || entry.sourceKind === "local",
          );
          const exact = commands.find((entry) =>
            entry.command === command ||
            entry.qualifiedCommand === command ||
            entry.aliases.includes(command) ||
            resolvedAliases[entry.command]?.toLowerCase() === command.toLowerCase(),
          );
          if (exact && completion?.items.length) {
            setCatalogSuggestions(completion.items.map((item) => ({
              kind: "completion",
              entry: exact,
              completion: item,
              commandLine: completedCommandLine(
                query,
                completionLine.fragmentStart,
                item,
                COMMAND_LINE_SYNTAX,
              ),
              execution: executionWithCompletion(exact, completionTokens, item),
              dynamic: completion.dynamic,
            })));
            return;
          }
          setCatalogSuggestions(commands.map((entry) => ({ kind: "catalog", entry })));
        })
        .catch(() => {
          if (catalogRequestGeneration.current === generation) setCatalogSuggestions([]);
        });
    }, CATALOG_SEARCH_DELAY);

    return () => window.clearTimeout(timer);
  }, [query, showCommandsInSearch, commandAliases]);

  // R26-A · browser result mode.
  //
  // The mode is entered by a trigger word plus a space (`bookmarks `,
  // `browser `, `history rust`). While it is on, the numbered list is the
  // browser's own rows and nothing else — it is a deliberate place, not a
  // ranking input. `parseBrowserMode` owns the trigger vocabulary so the node
  // suite can pin it without a DOM.
  const browserMode = useMemo<BrowserMode | null>(() => parseBrowserMode(query), [query]);
  const [browserRows, setBrowserRows] = useState<LauncherItem[]>([]);
  const browserRequest = useRef(0);

  useEffect(() => {
    const generation = ++browserRequest.current;
    if (!browserMode) {
      setBrowserRows([]);
      return;
    }
    // The early return above already narrowed the type; capturing it keeps that
    // narrowing visible inside the timer callback.
    const mode = browserMode;
    let cancelled = false;
    // The same debounce the catalog search uses: one fetch per typing pause,
    // and a stale response is dropped by the generation check.
    const timer = window.setTimeout(() => {
      const statusRow = (key: MessageKey): LauncherItem[] => [
        {
          type: "browser",
          id: `browser-status-${key}`,
          title: t(key),
          subtitle: "",
          url: "",
          profileKey: "default",
          disabled: true,
        },
      ];
      invoke<string | null>("browser_default_profile")
        .then(async (profileKey) => {
          if (!profileKey) return statusRow("launcher.browserNoProfile");
          const fetch = (command: string): Promise<BrowserSearchRow[]> =>
            invoke<BrowserSearchRow[]>(command, {
              profileKey,
              query: mode.needle,
              limit: BROWSER_FETCH_LIMIT,
            }).catch(() => []);
          const tabRead =
            mode.kind === "bookmarks" || mode.kind === "history"
              ? Promise.resolve({ tabs: [] as BrowserTabRow[], failed: false })
              : invoke<BrowserTabRow[]>("browser_list_tabs", {
                  profileKeyOrBrowser: profileKey,
                  limit: BROWSER_FETCH_LIMIT,
                }).then(
                  (value) => ({ tabs: value, failed: false }),
                  // R26-B · the fetch that is *expected* to fail (no debug port,
                  // browser not running) reports itself rather than throwing:
                  // the two lists above must never be held up by it.
                  () => ({ tabs: [] as BrowserTabRow[], failed: true }),
                );
          const [bookmarks, history, tabResult] = await Promise.all([
            mode.kind === "history" ? Promise.resolve([]) : fetch("browser_search_bookmarks"),
            mode.kind === "bookmarks" ? Promise.resolve([]) : fetch("browser_search_history"),
            tabRead,
          ]);
          const tabs = tabResult.tabs;
          const seen = new Set<string>();
          const rows: LauncherItem[] = [];
          for (const row of [...bookmarks, ...history]) {
            // A URL that is both a bookmark and a recent visit is one result;
            // bookmarks come first, so the curated row wins.
            if (!row.url || seen.has(row.url)) continue;
            seen.add(row.url);
            rows.push({
              type: "browser",
              id: row.id,
              title: row.title || row.url,
              subtitle: row.url,
              url: row.url,
              profileKey: row.profile_key,
            });
            if (rows.length >= BROWSER_GROUP_LIMIT) break;
          }
          // R26-B · the Tabs group. Deliberately NOT merged into `seen`: a tab
          // that is also a bookmark is two different actions (switch to it vs.
          // open it again), so both rows stay.
          for (const tab of tabs) {
            if (!tab.url && !tab.title) continue;
            rows.push({
              type: "browser",
              id: `tab:${tab.browser_id}:${tab.window_index}:${tab.tab_index}`,
              title: tab.title || tab.url,
              subtitle: tab.url,
              url: tab.url,
              profileKey,
              tab: {
                browserId: tab.browser_id,
                windowIndex: tab.window_index,
                tabIndex: tab.tab_index,
              },
            });
            if (rows.length >= BROWSER_GROUP_LIMIT * 2) break;
          }
          // R26-B · the soft landing, made visible: when the tab read failed
          // outright (no debug port, browser closed, AppleScript timed out) the
          // group is empty and this one disabled line says so, rather than the
          // user wondering why a running browser's tabs are missing.
          if (!tabs.length && tabResult.failed) {
            rows.push({
              type: "browser",
              id: "browser-tabs-unavailable",
              title: t("launcher.browserTabsUnavailable"),
              subtitle: "",
              url: "",
              profileKey: "default",
              disabled: true,
            });
          }
          return rows.length ? rows : statusRow("launcher.browserEmpty");
        })
        .then((rows) => {
          if (cancelled || generation !== browserRequest.current) return;
          setBrowserRows(rows);
        })
        .catch(() => {
          if (cancelled || generation !== browserRequest.current) return;
          setBrowserRows(statusRow("launcher.browserEmpty"));
        });
    }, CATALOG_SEARCH_DELAY);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [browserMode, t]);

  /**
   * The numbered result list: applications and the built-in system actions.
   *
   * Running the query as a command used to live in here too, wedged into the
   * second slot. It is the action bar now — a command is not a search result, it
   * is what to do with a search that found nothing, and giving it a row of its
   * own leaves every numbered slot for something that was actually matched.
   */
  const launcherResults = useMemo<LauncherItem[]>(() => {
    // R26-A: the browser mode owns the whole list — no applications, no
    // commands, no ranking against them.
    if (browserMode) return browserRows;
    const command = query.trim();
    const parsedQuery = parseCommandLine(query, false, COMMAND_LINE_SYNTAX);
    if (!command) {
      // The empty query is the launcher's front door: a summon with nothing
      // typed yet still has something useful to offer. Rank the applications
      // the user actually starts by launch count and render them as ordinary
      // results, so the numbered shortcuts and Enter work unchanged. Typing
      // any character leaves this branch. Turning the setting off empties the
      // front door instead: nothing is offered until something is typed.
      if (!showRecentInLauncher) return [];
      const byPath = new Map(searchableApps.map((entry) => [entry.app.path, entry]));
      const recentPaths = recentItems(
        launchCounts,
        searchableApps.map((entry) => entry.app.path),
        // R19: eight matched rows — the ninth visible row is the fixed
        // clipboard row the App appends, not a recent application.
        MAX_RESULTS - 1,
      );
      const recentRows: LauncherItem[] = [];
      for (const path of recentPaths) {
        const entry = byPath.get(path);
        if (!entry) continue;
        const app = entry.app;
        recentRows.push({
          type: "app",
          id: app.path,
          title: app.localizedName || app.name,
          subtitle:
            (app.localizedName && app.name) || app.comment || t(appSubtitleKey(app.path)),
          app,
        });
      }
      return recentRows;
    }

    // A query of nothing but punctuation normalizes away entirely; it can only
    // ever be an action-bar command.
    const needle = normalizeSearch(command);
    if (!needle) return [];

    // Applications and the power actions are scored the same way and ranked
    // against each other, so "restart" reaches the power action while "restic"
    // still reaches the application.
    const matches: { item: LauncherItem; score: number }[] = [];

    for (const entry of searchableApps) {
      const score = scoreApp(needle, entry.names, entry.initials, entry.aliases);
      if (!score) continue;
      const app = entry.app;
      matches.push({
        item: {
          type: "app",
          id: app.path,
          title: app.localizedName || app.name,
          // Showing the original name next to a localized title is the most
          // useful subtitle; failing that, whatever description the platform
          // ships, and only then the generic category.
          subtitle:
            (app.localizedName && app.name) || app.comment || t(appSubtitleKey(app.path)),
          app,
        },
        score,
      });
    }

    for (const entry of SYSTEM_COMMANDS) {
      const title = t(entry.titleKey);
      // Scored exactly like an application: the names in every language, plus the
      // pinyin key that `gj` and `cq` reach the entry through.
      const score = scoreApp(
        needle,
        [normalizeSearch(title), ...entry.searchNames],
        entry.initials,
        // The power actions have no alias to speak of: their names are already
        // written out in every language the launcher searches.
        [],
      );
      if (!score) continue;
      matches.push({
        item: {
          type: "system",
          id: `system-${entry.action}`,
          title,
          subtitle: t(entry.subtitleKey),
          action: entry.action,
        },
        score,
      });
    }

    const rankedMatches = matches
      .sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title))
      .map((match) => match.item);
    // R10-A/R19: eight matched rows. When applications or power actions matched
    // alongside the commands the split keeps its old shape — three catalog
    // commands, five slots for the local matches; when nothing else matched the
    // commands may take all eight.
    const commandLimit = rankedMatches.length
      ? Math.min(COMMAND_LIMIT_WITH_MATCHES, MAX_RESULTS - 3)
      : MAX_RESULTS - 1;
    const commandCounts = catalogSuggestions.reduce<Map<string, number>>((counts, suggestion) => {
      const command = suggestion.entry.command;
      counts.set(command, (counts.get(command) ?? 0) + 1);
      return counts;
    }, new Map());
    // R7-11: the alias is a candidate string at the same tiers as the command
    // name, so the visible rows are ranked by that shared score rather than by
    // the backend's command-only order. This is what makes an exact alias hit
    // sit where an exact command-name hit would; `commandLimit` is applied
    // after the sort so a command displaced by an alias match is the one that
    // drops off, not an arbitrary neighbor.
    const aliasByCommand = resolveCommandAliases(commandAliases);
    const commandNeedle = normalizeSearch(command);
    const commandItems: LauncherItem[] = catalogSuggestions
      .map((suggestion) => ({
        suggestion,
        score: suggestion.kind === "completion"
          ? MATCH_EXACT + 1
          : commandMatchScore(
              commandNeedle,
              suggestion.entry.command,
              aliasByCommand[suggestion.entry.command],
              suggestion.entry.name,
              suggestion.entry.aliases,
            ),
      }))
      .sort((left, right) =>
        right.score - left.score ||
        left.suggestion.entry.command.localeCompare(right.suggestion.entry.command)
      )
      .slice(0, commandLimit)
      .map(({ suggestion }) => {
        const { entry } = suggestion;
        // Warnings stay out of the subtitle string: they render as an
        // always-visible dot beside the source label, so a narrow window can
        // never truncate them away.
        const warnings: CommandWarning[] = [];
        if (!entry.runtimeAvailable) warnings.push("unavailable");
        if ((commandCounts.get(entry.command) ?? 0) > 1) warnings.push("conflict");
        if (suggestion.kind === "completion") {
          const dynamic = suggestion.dynamic
            ? ` · ${t("extensions.dynamicCompletion")}`
            : "";
          return {
            type: "command",
            id: `${entry.id}:completion:${suggestion.completion.value}`,
            title: suggestion.completion.label,
            subtitle: `${suggestion.completion.description}${dynamic}`,
            warnings,
            sourceName: entry.sourceName,
            commandLine: suggestion.commandLine,
            execution: suggestion.execution,
            completion: true,
          };
        }
        // When the user types parameters after the command name, update the
        // execution plan to include them. Without this, "git status" would match
        // the "git" catalog entry but execute with no arguments.
        const hasUserArgs = parsedQuery.commandIndex !== null &&
          parsedQuery.tokens.length > parsedQuery.commandIndex + 1;
        const execution = hasUserArgs && entry.execution
          ? {
              ...entry.execution,
              argumentOverride: parsedQuery.tokens.slice(parsedQuery.commandIndex! + 1),
            }
          : entry.execution;
        // An exact alias is a search/typing surface only: the row still runs
        // the real command. `gfm -m x` therefore executes `git -m x`, never a
        // nonexistent shell command named `gfm`. A prefix/contains alias hit is
        // left as typed, so the user sees what they typed until they commit it.
        const typedAlias = matchedCommandAlias(commandAliases, entry.command, command);
        const rowCommandLine = typedAlias
          ? `${rebaseAliasCommandLine(query, typedAlias, entry.command)}${hasUserArgs ? "" : " "}`
          : hasUserArgs
            ? query
            : `${entry.command} `;
        // The alias that produced this row rides the subtitle, so a user who
        // typed `gf` and got `git` can see *why*: the matching alias explains
        // the row instead of leaving it looking like a stray fuzzy hit. It is
        // shown only when the alias is what carried the match — a query that
        // already names the command does not need the explanation.
        const matchedAlias = aliasByCommand[entry.command];
        const aliasHint = matchedAlias &&
          candidateMatchScore(commandNeedle, matchedAlias.toLowerCase()) >
            candidateMatchScore(commandNeedle, entry.command.toLowerCase())
          ? t("launcher.aliasMatch", { alias: matchedAlias })
          : "";
        return {
          type: "command",
          id: entry.id,
          title: entry.command,
          subtitle: aliasHint ? `${aliasHint} · ${entry.description}` : entry.description,
          warnings,
          sourceName: entry.sourceName,
          commandLine: rowCommandLine,
          execution,
          completion: false,
        };
      });

    // The catalog returns the matched rows only: the App appends the fixed
    // clipboard row after these (R10-A; R19 set the matched ceiling to eight),
    // and the action bar is a row of its own beneath the list. Keep at least one
    // local match when applications or power actions matched alongside catalog
    // commands.
    return [...commandItems, ...rankedMatches].slice(0, MAX_RESULTS - 1);
  }, [browserMode, browserRows, catalogSuggestions, query, searchableApps, launchCounts, showRecentInLauncher, commandAliases, t]);

  const actionBar = useMemo<ActionBar | null>(() => {
    // R26-A: the browser mode is a place of its own; its rows are run by Enter,
    // so there is no shell action to offer underneath them.
    if (browserMode) return null;
    const value = query.trim();
    if (!value) return null;
    const type = classifyActionBar(value);
    const label = type === "url"
      ? t("launcher.openInBrowser")
      : type === "path"
        ? t("launcher.openInFiles")
        : type === "restart"
          ? t("system.restart")
          : type === "shutdown"
            ? t("system.shutdown")
            : type === "clipboard"
              ? t("system.clipboardHistory")
              : type === "browser"
                ? t("system.browserSearch")
                : t("launcher.runInShell");
    return { type, label, value };
  }, [browserMode, query, t]);

  const runnableResultFlags = launcherResults.map((item) =>
    item.type === "command"
      ? Boolean(item.execution)
      : !(item.type === "browser" && item.disabled === true),
  );
  const resultShortcutSlots = launcherShortcutSlots(runnableResultFlags);
  const runnableResultCount = runnableResultFlags.filter(Boolean).length;
  const hasRunnableCommandResult = launcherResults.some(
    (item) => item.type === "command" && Boolean(item.execution),
  );
  const firstRunnableResultIndex = runnableResultFlags.indexOf(true);

  /**
   * Whether a fresh query starts out on the action bar rather than on the first
   * result.
   *
   * A boolean rather than something the effect in `App.tsx` recomputes, so that
   * the effect fires when the *answer* changes and not merely when the result
   * list is rebuilt. A background application refresh can give
   * `launcherResults` a new identity even when nothing about the visible
   * matches changed — depending on the list itself would throw away a
   * selection the user had already moved with the arrow keys.
   */
  const defaultsToActionBar = useMemo(() => {
    if (!actionBar) return false;
    return shouldDefaultToActionBar(
      query,
      actionBar.type,
      launcherResults.length,
      runnableResultCount,
      hasRunnableCommandResult,
    );
  }, [actionBar, hasRunnableCommandResult, launcherResults.length, query, runnableResultCount]);

  // A full scan walks every application directory, so the two callers below
  // share one: the initial load and a refresh after a summon must never end up
  // running at the same time.
  const scanApplications = (forceRefresh: boolean) => {
    if (appScanning.current) return;
    appScanning.current = true;
    setAppsLoading(true);
    setAppsError(false);
    invoke<LocalApplication[]>("list_applications", { forceRefresh })
      .then((nextApplications) => {
        setApplications(nextApplications);
        setAppsError(false);
      })
      .catch(() => setAppsError(true))
      .finally(() => {
        appScanning.current = false;
        setAppsLoading(false);
      });
  };

  useEffect(() => {
    scanApplications(false);
  }, []);

  /**
   * Only the launcher needs applications. The backend coalesces checks inside
   * a platform-specific cooldown and performs any directory walk on a blocking
   * thread; a changed source refreshes behind the existing list.
   *
   * Called from the reveal listener in `App.tsx`.
   */
  const refreshApplicationsIfStale = () => {
    invoke<ApplicationsStatus>("check_applications")
      .then((status) => {
        if (!status.upToDate) scanApplications(true);
      })
      .catch(() => undefined);
  };

  useEffect(() => {
    const missing = launcherResults
      .filter((item): item is Extract<LauncherItem, { type: "app" }> => item.type === "app")
      .filter(
        (item) =>
          !appIconUrlsRef.current[item.app.path] &&
          !appIconAttempts.current.has(item.app.path),
      )
      .slice(0, 6);

    if (!missing.length) return;
    let cancelled = false;

    // Resolving an icon means walking the platform's icon directories, so it
    // waits for the query to settle: every keystroke changes the result list,
    // and the only list worth fetching for is the one the user stops on.
    const timer = window.setTimeout(() => {
      // A missing icon is still a completed lookup. Remember it so an app that
      // has no platform icon does not trigger the same work on every query.
      for (const item of missing) appIconAttempts.current.add(item.app.path);

      // Parallel: all icons resolve at once, and a single state update
      // carries every result so the renderer is not kicked once per icon.
      Promise.all(
        missing.map((item) =>
          invoke<string | null>("application_icon", { path: item.app.path })
            .then((path) => ({ path: item.app.path, icon: path }))
            .catch(() => null),
        ),
      ).then((results) => {
        if (cancelled) return;
        const newIcons: Record<string, string> = {};
        for (const result of results) {
          if (result?.icon) {
            newIcons[result.path] = convertFileSrc(result.icon);
          }
        }
        if (Object.keys(newIcons).length) {
          setAppIconUrls((current) => ({ ...current, ...newIcons }));
        }
      });
    }, ICON_LOAD_DELAY);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [launcherResults]);

  const rememberCommand = (command: string) => {
    setHistory((current) => [command, ...current.filter((entry) => entry !== command)].slice(0, 20));
    setHistoryIndex(-1);
    draftBeforeHistory.current = "";
  };

  /** Count an application launch so the empty-query state can rank it. The
   *  counter rides the ordinary settings persistence; no dedicated command. */
  const recordLaunch = (path: string) => {
    const updated: AppSettings = {
      ...settingsRef.current,
      launch_counts: {
        ...settingsRef.current.launch_counts,
        [path]: (settingsRef.current.launch_counts[path] ?? 0) + 1,
      },
    };
    settingsHydration.markChanged("launch_counts");
    settingsRef.current = updated;
    setSettings(updated);
    persistSettings().catch(() => undefined);
  };

  return {
    applications,
    appsLoading,
    appsError,
    appIconUrls,
    launcherResults,
    actionBar,
    runnableResultFlags,
    resultShortcutSlots,
    firstRunnableResultIndex,
    defaultsToActionBar,
    scanApplications,
    refreshApplicationsIfStale,
    history,
    setHistory,
    historyIndex,
    setHistoryIndex,
    draftBeforeHistory,
    rememberCommand,
    recordLaunch,
  };
}
