// React binding for the launcher's data layer: application scanning, icon
// loading, catalog search/completion, the ranked result list and the
// recent-command bookkeeping.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned value
// it touches, so the behaviour is unchanged.

import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type RefObject, type SetStateAction } from "react";
import { convertFileSrc, invoke } from "@tauri-apps/api/core";
import {
  classifyActionBar,
  completedCommandLine,
  executionWithCompletion,
  launcherShortcutSlots,
  normalizeSearch,
  parseCommandLine,
  recentItems,
  scoreApp,
  shouldDefaultToActionBar,
  type BrowserMode,
  type ClipboardMode,
  type CompletionItem,
  type ExecutionPlan,
} from "../launcher";
import { normalizeEntries, MAX_CLIPBOARD_MAX_ITEMS, type ClipboardEntry } from "../clipboard-history";
import {
  PLUGIN_INITIAL_PAGES,
  pagePluginEmission,
  pluginViewItems,
  resolvePluginView,
  type PluginEmission,
  type PluginView,
} from "../launcher/plugin-mode";
import {
  externalRunText,
  type ExternalPluginCommand,
  type ExternalRunState,
} from "../plugins/external";
import {
  BROWSER_FETCH_LIMIT,
  browserSearchRows,
  browserStatusRow,
  type BrowserSearchRow,
  type BrowserTabRow,
} from "../plugins/browser/mode";
import { clipboardModeRows, clipboardStatusRow } from "../plugins/clipboard/mode";
import {
  type ActionBar,
  type CommandWarning,
  type LauncherItem,
  type SystemAction,
} from "../launcher/LauncherResults";
import { appSubtitleKey } from "../launcher/row-content";
import type { BrowserSearchField } from "../browser-page";
import { createSettingsHydration } from "../settings-persistence";
import { aliasToCommand, candidateMatchScore, commandMatchScore, matchedCommandAlias, rebaseAliasCommandLine, resolveCommandAliases, MATCH_EXACT, type CommandAliases } from "../command-aliases";
import { COMMAND_LIMIT_WITH_MATCHES, MAX_RESULTS } from "../launcher/result-budget";
import { IS_WINDOWS } from "../shortcuts";
import type { AppSettings, LocalApplication } from "../App";
import type { MessageKey, Translate } from "../i18n";

// R10-A/R19/R36/R37: `MAX_RESULTS` is 10 — and since R37 that is **ten
// matched result rows**. Through R36 every slice below kept a `- 1` because the
// tenth row was the App's fixed clipboard row, not a match; the fixed row is
// gone (the clipboard is an ordinary contributor, see `launcher/result-budget.ts`),
// so the catalog may now fill the whole budget. The number lives in
// `launcher/result-budget.ts` beside the slot map that numbers the rows, so the
// budget and the keys cannot drift apart; it is re-exported here because this
// module is where the result budget is read.
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

/** R32 · the browser mode's fetched rows, or the one status line that stands in
 *  for them when there is no profile (or the read failed outright). The three
 *  sources are held raw — the needle is applied later, in memory — so typing
 *  inside the mode re-renders without a fetch. */
type BrowserFetchResult =
  | {
      ok: true;
      profileKey: string;
      bookmarks: BrowserSearchRow[];
      history: BrowserSearchRow[];
      tabs: BrowserTabRow[];
    }
  | { ok: false; statusId: string; statusKey: MessageKey };

// R28 · the row shapes the two plugins emit live in their own modules
// (`plugins/browser/mode.ts`, `plugins/clipboard/mode.ts`) and the mapping from
// those rows to `LauncherItem`s lives in the capability layer
// (`launcher/plugin-mode.ts`). This hook only fetches, hands the output to the
// layer and reads back a view.

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
  /** R31 · the browser request the launcher is in, or `null`. Computed by the
   *  App from its explicit plugin-mode state + the field's text
   *  (`browserModeFor`), so this hook never re-parses a mode word out of the
   *  query — the query is the field's own text now. */
  browserMode: BrowserMode | null;
  /** R31 · the clipboard request the launcher is in, or `null` (see
   *  {@link browserMode}). */
  clipboardMode: ClipboardMode | null;
  /** R39 · the external plugin request the launcher is in, or `null`. The
   *  `args` field is the field's raw text; the hook does not split it (the run
   *  does, in the App). */
  externalMode: { extensionId: string; commandId: string; args: string } | null;
  /** R39 · the external command the mode belongs to, for the row source word
   *  and the status rows' wording. `null` outside the mode. */
  externalCommand: ExternalPluginCommand | null;
  /** R39 · the mode's run state. Drives the emission: idle/running/failed are
   *  status rows, a finished run's output goes through the dual-form
   *  pipeline. */
  externalRun: ExternalRunState;
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
  /** R26-D · `settings.browser_plugin.enabled`. When off, the browser entry row
   *  is a disabled note and the browser result mode fetches nothing: the
   *  plugin's whole surface soft-closes. */
  browserEnabled: boolean;
  /** R32 · `settings.browser_plugin.search_fields`. Which fields the browser
   *  mode's needle is matched against (`all` / `title` / `url`). The clipboard
   *  mode deliberately does not read it: a clipboard entry has no URL. */
  browserSearchField: BrowserSearchField;
  /** R27 · `settings.clipboard_history_enabled`. The clipboard mode's switch,
   *  read from the same long-standing field the plugin list and the panel use. */
  clipboardEnabled: boolean;
  /** R38 · the launcher's one-line feedback under the list. The clipboard
   *  mode's favorite toggle reports a failed write through it (the star has
   *  already rolled back by then). */
  showLauncherFeedback: (key: MessageKey, duration?: number) => void;
  t: Translate;
  settingsRef: RefObject<AppSettings>;
  settingsHydration: ReturnType<typeof createSettingsHydration<AppSettings>>;
  setSettings: Dispatch<SetStateAction<AppSettings>>;
  persistSettings: () => Promise<void>;
}) {
  const {
    query,
    browserMode,
    clipboardMode,
    externalMode,
    externalCommand,
    externalRun,
    launchCounts,
    showCommandsInSearch,
    showRecentInLauncher,
    commandAliases,
    browserEnabled,
    browserSearchField,
    clipboardEnabled,
    showLauncherFeedback,
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
  // R31 · the mode is explicit state now (see `ActivePluginMode` in
  // `launcher.ts`): the App resolves it once from the field's text and passes
  // the request in. This hook no longer parses the query — the query is the
  // field's own needle, with the trigger word stripped on entry — so the
  // vocabulary that enters a mode lives in exactly one place (`pluginModeEntry`)
  // and the hook only fetches.
  //
  // R32 · the three sources are fetched **once per range filter**, with an
  // empty backend query, and the needle is applied in memory
  // (`browserSearchRows`). That is what lets a multi-word query mean "all of
  // these tokens" instead of the literal phrase, and it is why typing inside
  // the mode now costs no IPC at all — the same shape the clipboard mode has
  // had since R27. The fetch is keyed on the filter (`kind`), never on the
  // needle, so a keystroke cannot re-read a 500-row history.
  const [browserFetch, setBrowserFetch] = useState<BrowserFetchResult | null>(null);
  const browserRequest = useRef(0);
  const browserKind = browserMode?.kind ?? null;

  useEffect(() => {
    const generation = ++browserRequest.current;
    if (!browserMode || !browserEnabled) {
      // Nothing to fetch: leaving the mode clears the held rows, and the
      // switched-off plugin's one status line is drawn by the memo below.
      setBrowserFetch(null);
      return;
    }
    const kind = browserMode.kind;
    let cancelled = false;
    // The same debounce the catalog search uses: one fetch per filter, and a
    // stale response is dropped by the generation check.
    const timer = window.setTimeout(() => {
      invoke<string | null>("browser_default_profile")
        .then(async (profileKey): Promise<BrowserFetchResult> => {
          if (!profileKey) {
            return { ok: false, statusId: "browser-no-profile", statusKey: "launcher.browserNoProfile" };
          }
          const fetch = (command: string): Promise<BrowserSearchRow[]> =>
            invoke<BrowserSearchRow[]>(command, {
              profileKey,
              // R32 · an empty query fetches the whole (bounded) group; the
              // needle's AND tokens are applied in memory, where all four
              // sources can share one rule.
              query: "",
              limit: BROWSER_FETCH_LIMIT,
            }).catch(() => []);
          const tabRead =
            kind === "bookmarks" || kind === "history"
              ? Promise.resolve<BrowserTabRow[]>([])
              : invoke<BrowserTabRow[]>("browser_list_tabs", {
                  profileKeyOrBrowser: profileKey,
                  limit: BROWSER_FETCH_LIMIT,
                }).catch(
                  // R26-B · the fetch that is *expected* to fail (no debug port,
                  // browser not running) reports itself as an empty group rather
                  // than throwing: the two lists above must never be held up by
                  // it. R31 · it no longer emits a note into the list either —
                  // the guidance lives on the settings field that fixes it (see
                  // `browserSearchRows`).
                  () => [] as BrowserTabRow[],
                );
          const [bookmarks, history, tabs] = await Promise.all([
            kind === "history" || kind === "tabs"
              ? Promise.resolve([])
              : fetch("browser_search_bookmarks"),
            kind === "bookmarks" || kind === "tabs"
              ? Promise.resolve([])
              : fetch("browser_search_history"),
            tabRead,
          ]);
          return { ok: true, profileKey, bookmarks, history, tabs };
        })
        .then((result) => {
          if (cancelled || generation !== browserRequest.current) return;
          setBrowserFetch(result);
        })
        .catch(() => {
          if (cancelled || generation !== browserRequest.current) return;
          setBrowserFetch({ ok: false, statusId: "browser-empty", statusKey: "launcher.browserEmpty" });
        });
    }, CATALOG_SEARCH_DELAY);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
    // The filter, the switch and the language own the fetch; the needle does
    // not — it is applied in the memo below.
  }, [browserKind, browserEnabled, t]);

  // R28 · the merge, the group ceilings and the soft landings are the plugin's
  // own output rules (see `plugins/browser/mode.ts`); this hook hands the three
  // fetched sources over and reads back a view. R32 · the needle and the
  // configured search field are applied here, in memory.
  const browserEmission = useMemo<PluginEmission | null>(() => {
    if (!browserMode) return null;
    if (!browserEnabled) {
      // R26-D · the plugin is switched off. The mode word is still parseable (a
      // stale query can carry `browser `), but nothing is fetched and one
      // disabled line says why — soft-closed, not an error.
      return {
        output: [browserStatusRow("browser-disabled", "launcher.browserDisabled", t)],
      };
    }
    if (!browserFetch) return null;
    if (!browserFetch.ok) {
      return { output: [browserStatusRow(browserFetch.statusId, browserFetch.statusKey, t)] };
    }
    return {
      output: browserSearchRows({
        bookmarks: browserFetch.bookmarks,
        history: browserFetch.history,
        tabs: browserFetch.tabs,
        profileKey: browserFetch.profileKey,
        t,
        limit: BROWSER_FETCH_LIMIT,
        needle: browserMode.needle,
        searchField: browserSearchField,
      }),
    };
  }, [browserMode, browserEnabled, browserFetch, browserSearchField, t]);

  // R27 · clipboard result mode — the browser mode's twin, over the clipboard
  // history the panel shows. R31 · like the browser mode, the request arrives
  // resolved (`clipboardModeFor`) rather than being parsed here. The entries are
  // fetched **once** when the mode opens (the history is local and the whole list
  // arrives in one call); the needle filters in memory, so typing inside the mode
  // costs no IPC and no window resize.
  const clipboardActive = clipboardMode !== null;
  const [clipboardEntries, setClipboardEntries] = useState<ClipboardEntry[]>([]);
  // R38 · a manual refetch trigger. The config overlay's "clear history"
  // action mutates the store behind this hook's back, so the App bumps this
  // counter and the fetch below runs again — the mode's list is never left
  // showing rows the user just deleted.
  const [clipboardRevision, setClipboardRevision] = useState(0);
  const reloadClipboardEntries = useCallback(() => {
    setClipboardRevision((revision) => revision + 1);
  }, []);

  useEffect(() => {
    if (!clipboardActive || !clipboardEnabled) {
      setClipboardEntries([]);
      return;
    }
    let cancelled = false;
    // `filter: null` is the whole history, newest first — the same call the
    // panel's own load makes. The mode's needle is applied below, in memory.
    invoke<unknown[]>("clipboard_get_entries", { filter: null })
      .then((rows) => {
        if (!cancelled) setClipboardEntries(normalizeEntries(rows));
      })
      .catch(() => {
        if (!cancelled) setClipboardEntries([]);
      });
    return () => {
      cancelled = true;
    };
  }, [clipboardActive, clipboardEnabled, clipboardRevision]);

  const clipboardEmission = useMemo<PluginEmission | null>(() => {
    if (!clipboardMode) return null;
    if (!clipboardEnabled) {
      return {
        output: [clipboardStatusRow("clipboard-disabled", "clipboard.pageUnavailable", t)],
      };
    }
    // The entries are fetched once and filtered in memory, so typing inside the
    // mode costs no IPC. R28 · the rows are the plugin's *output*; the
    // capability layer decides they are a list and how tall the window is.
    return { output: clipboardModeRows(clipboardEntries, clipboardMode, t, Date.now(), MAX_CLIPBOARD_MAX_ITEMS) };
  }, [clipboardMode, clipboardEnabled, clipboardEntries, t]);

  // R39 · the external plugin mode's emission. The command's own output is the
  // product: a conforming list (a JSON array of rows) becomes the launcher's
  // list, anything else becomes text — the *same* decision the built-ins go
  // through, made in `resolvePluginView` below. The three non-output states
  // (not run yet, running, the run itself failed) are status rows, so the field
  // never sits over an empty box with no explanation.
  const externalEmission = useMemo<PluginEmission | null>(() => {
    if (!externalMode || !externalCommand) return null;
    const sourceName = externalCommand.extensionName;
    const status = (id: string, title: string): PluginEmission => ({
      output: [{ family: "plugin", id, title, kind: "status", disabled: true }],
      sourceName,
    });
    if (externalRun.status === "idle") return status("external-idle", t("launcher.externalIdle"));
    if (externalRun.status === "running") {
      return status("external-running", t("launcher.externalRunning"));
    }
    if (externalRun.status === "failed") {
      return status("external-failed", externalRun.message || t("launcher.externalFailed"));
    }
    const text = externalRunText(externalRun.output);
    if (text === null) return status("external-empty", t("launcher.externalEmpty"));
    return { output: text, sourceName };
  }, [externalMode, externalCommand, externalRun, t]);

  // R38 · the favorite toggle, owned here because this hook owns the entries.
  // Optimistic: the star flips on the next paint and the write follows; a
  // refused write rolls the one row back and the launcher's feedback line says
  // why. The mode refetches on the next open, so the store stays the authority
  // without a reload here. A ref mirrors the entries so the callback can read
  // the row's current flag without being rebuilt per entry change.
  const clipboardEntriesRef = useRef<ClipboardEntry[]>([]);
  clipboardEntriesRef.current = clipboardEntries;
  const toggleClipboardFavorite = useCallback(
    (id: string) => {
      const current = clipboardEntriesRef.current.find((entry) => entry.id === id);
      if (!current) return;
      const next = !current.favorite;
      const paint = (favorite: boolean) =>
        setClipboardEntries((entries) =>
          entries.map((entry) => (entry.id === id ? { ...entry, favorite } : entry)),
        );
      paint(next);
      invoke("clipboard_set_favorite", { id, favorite: next })
        .then(() => {
          // R38 · un-favoriting hands the entry back to the retention policy,
          // and the backend prunes inside the same write — so an entry that had
          // outlived the window can be gone by the time this resolves. Refetch
          // then, rather than leaving a row whose Enter would fail. Favoriting
          // can never prune (it only removes an entry from the non-favorite
          // pool), so it needs no refetch.
          if (!next) reloadClipboardEntries();
        })
        .catch(() => {
          paint(current.favorite);
          showLauncherFeedback("clipboard.favoriteFailed");
        });
    },
    [reloadClipboardEntries, showLauncherFeedback],
  );

  // R29 · client-side pagination of the inline plugin list.
  //
  // The two built-ins fetch their whole (bounded) result set in one call —
  // `browser_search_*` with a 200-row ceiling, the clipboard history capped at
  // its own 500 — and the launcher windows it. That keeps the *display*
  // incremental (one viewport per scroll-to-bottom) while adding no
  // offset/cursor parameter to the backend commands and no new entry to their
  // allowlist: the protocol's cursor is honoured, but for these plugins it is an
  // offset into rows the frontend already holds. A plugin that genuinely streams
  // pages resolves the same `loadMorePluginPage` against its own backend.
  const [pluginPages, setPluginPages] = useState(PLUGIN_INITIAL_PAGES);
  const [pluginLoadingMore, setPluginLoadingMore] = useState(false);
  const pluginLoadingRef = useRef(false);
  // A new mode or a new needle is a new result set: the window resets to its
  // first pages. `browserMode`/`clipboardMode` are rebuilt per query, so their
  // identity is the signal. R39 · the external mode's list only changes when a
  // run lands, so its signal is the run state, not the per-keystroke argument
  // text.
  useEffect(() => {
    setPluginPages(PLUGIN_INITIAL_PAGES);
  }, [browserMode, clipboardMode, externalRun]);

  /**
   * R30 · hand the capability layer the rows this page count has *loaded*, plus
   * the pagination block that says whether more exist. The rule lives in the
   * protocol module (`pagePluginEmission`) so the node suite can pin it without
   * a DOM; this hook only supplies the page count and the emission.
   */
  const windowedEmission = (emission: PluginEmission | null): PluginEmission | null =>
    pagePluginEmission(emission, pluginPages);

  // R28 · the capability layer's answer for whichever plugin mode is on. One
  // view, both plugins: the form (list or text), the tier (interactive or
  // display) and — for text — the min/max height all come from here, so neither
  // plugin decides how the launcher draws it. R29 · the list is windowed here.
  const pluginView = useMemo<PluginView | null>(() => {
    if (browserMode) return resolvePluginView(windowedEmission(browserEmission));
    if (clipboardMode) return resolvePluginView(windowedEmission(clipboardEmission));
    if (externalMode) return resolvePluginView(windowedEmission(externalEmission));
    return null;
  }, [browserMode, clipboardMode, externalMode, browserEmission, clipboardEmission, externalEmission, pluginPages]);

  // R29 · the scroll-to-bottom trigger. `pluginHasMore` is the one bit the
  // launcher reads; the ref lets the once-created callback see the current
  // value without being rebuilt (and without re-binding the scroller).
  const pluginHasMore =
    pluginView !== null &&
    pluginView.form === "list" &&
    pluginView.page !== null &&
    pluginView.page.hasMore;
  const pluginHasMoreRef = useRef(pluginHasMore);
  pluginHasMoreRef.current = pluginHasMore;

  const loadMorePluginPage = useCallback(() => {
    if (pluginLoadingRef.current || !pluginHasMoreRef.current) return;
    pluginLoadingRef.current = true;
    setPluginLoadingMore(true);
    // The built-ins page in memory, so the append lands on the next frame —
    // which is exactly where an async plugin's fetch would resolve. The footer
    // reads `pluginLoadingMore` for its loading line.
    window.requestAnimationFrame(() => {
      setPluginPages((pages) => pages + 1);
      pluginLoadingRef.current = false;
      setPluginLoadingMore(false);
    });
  }, []);

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
    // commands, no ranking against them. R27: the clipboard mode does the same.
    // R28 · both reach the list through the one capability layer: the view it
    // resolved contributes the rows (and nothing at all in the text form). While
    // the mode is on but its output has not arrived yet (the fetch debounce),
    // the list is empty rather than falling back to the ordinary search.
    if (browserMode || clipboardMode || externalMode) return pluginView ? pluginViewItems(pluginView) : [];
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
        // R19/R36/R37: the empty query's front door shows a full budget of
        // recent applications — the tenth row is no longer a fixed clipboard
        // row the App appends, so a recent app may take it.
        MAX_RESULTS,
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
      // R26-D · a switched-off browser plugin keeps its entry row but marks
      // it: the row is a note (“the plugin is turned off”), not a door.
      // R36 · the clipboard's launcher row follows the same rule. R37 · it is
      // now the *only* clipboard row — the fixed tail that carried the same
      // soft-close is gone, so this entry is the whole of the clipboard's
      // presence in the list.
      const pluginOff =
        (entry.action === "browser" && !browserEnabled) ||
        (entry.action === "clipboard" && !clipboardEnabled);
      matches.push({
        item: {
          type: "system",
          id: `system-${entry.action}`,
          title,
          subtitle: pluginOff
            ? t(
                entry.action === "browser"
                  ? "launcher.browserDisabled"
                  : "clipboard.pageUnavailable",
              )
            : t(entry.subtitleKey),
          action: entry.action,
          ...(pluginOff ? { disabled: true } : {}),
        },
        score,
      });
    }

    const rankedMatches = matches
      .sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title))
      .map((match) => match.item);
    // R10-A/R19/R36/R37: the matched rows are the whole budget. When
    // applications or power actions matched alongside the commands the split
    // keeps its old shape — three catalog commands, seven slots for the local
    // matches; when nothing else matched the commands may take all ten.
    const commandLimit = rankedMatches.length
      ? Math.min(COMMAND_LIMIT_WITH_MATCHES, MAX_RESULTS - 3)
      : MAX_RESULTS;
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

    // The catalog returns the matched rows only, up to the whole budget: R37
    // removed the App's fixed clipboard tail (R10-A; R19 set the matched
    // ceiling to eight, R36 to nine, R37 to ten), and the action bar is a row of
    // its own beneath the list. Keep at least one local match when applications
    // or power actions matched alongside catalog commands.
    return [...commandItems, ...rankedMatches].slice(0, MAX_RESULTS);
  }, [pluginView, browserMode, clipboardMode, externalMode, catalogSuggestions, query, searchableApps, launchCounts, showRecentInLauncher, commandAliases, browserEnabled, clipboardEnabled, t]);

  const actionBar = useMemo<ActionBar | null>(() => {
    // R26-A: the browser mode is a place of its own; its rows are run by Enter,
    // so there is no shell action to offer underneath them. R27: the clipboard
    // mode is the same kind of place.
    if (browserMode || clipboardMode || externalMode) return null;
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
  }, [browserMode, clipboardMode, externalMode, query, t]);

  const runnableResultFlags = launcherResults.map((item) =>
    item.type === "command"
      ? Boolean(item.execution)
      : !(
          // R30 · a plugin status line is never a result: the renderer draws it
          // as a note, so it must not take a numbered slot, a selection step or
          // Enter either.
          item.type === "status" ||
          (item.type === "browser" && item.disabled === true) ||
          (item.type === "clipboard" && item.disabled === true) ||
          // R39 · an external plugin row with no action is information only.
          (item.type === "plugin" && (item.disabled === true || item.action === undefined)) ||
          (item.type === "system" && item.disabled === true)
        ),
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
    pluginView,
    pluginHasMore,
    pluginLoadingMore,
    loadMorePluginPage,
    toggleClipboardFavorite,
    reloadClipboardEntries,
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
