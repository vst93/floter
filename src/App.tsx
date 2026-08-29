import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { convertFileSrc, invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { check } from "@tauri-apps/plugin-updater";
import { relaunch } from "@tauri-apps/plugin-process";
import {
  AlertCircle,
  Blocks,
  Info,
  Keyboard,
  RefreshCw,
  SlidersHorizontal,
  SquareTerminal,
} from "lucide-react";
import { TerminalCanvas, decodeFrame, type CellPoint, type Selection } from "./terminal/render";
import { PinnedTerminalCard } from "./terminal/PinnedTerminalCard";
import { PINNED_SESSION_ID } from "./terminal/pinState";
import { usePinnedTerminal } from "./hooks/usePinnedTerminal";
import {
  encodeKey,
  FOCUS_IN_OUT,
  isTerminalCompositionKey,
  MOUSE_MOTION,
  shouldUseTerminalTextInput,
  usesMouseReporting,
} from "./terminal/keys";
import {
  createTranslator,
  normalizeLanguage,
  type Language,
  type MessageKey,
} from "./i18n";
import { ExtensionsPanel, type ExtensionExecutionPlan } from "./ExtensionsPanel";
import { PluginPageHost } from "./plugins/PluginPageHost";
import { CLIPBOARD_PLUGIN_ID } from "./plugin-pages";
import { beginRequest, isCurrentRequest } from "./request-generation";
import {
  classifyActionBar,
  completedCommandLine,
  executionWithCompletion,
  launcherShortcutSlots,
  normalizeSearch,
  nextLauncherSelection,
  parseCommandLine,
  recentItems,
  scoreApp,
  shouldDefaultToActionBar,
  type CompletionItem,
  type ExecutionPlan,
} from "./launcher";
import {
  DEFAULT_SHORTCUTS,
  formatResultShortcut,
  formatShortcut,
  IS_LINUX,
  IS_MAC,
  IS_WINDOWS,
  matchesResultShortcut,
  matchesShortcut,
  matchesShortcutModifiers,
  normalizeResultShortcut,
  SHORTCUT_ACTIONS,
  withShortcutDefaults,
  type ShortcutAction,
  type ShortcutMap,
} from "./shortcuts";
import { createSerialSettingsWriter, createSettingsHydration } from "./settings-persistence";
import { GeneralPage, normalizeFontSize, normalizeOpacity } from "./settings/GeneralPage";
import { ShortcutsPage, CLIPBOARD_HOTKEY_ACTION } from "./settings/ShortcutsPage";
import { SessionsPage } from "./settings/SessionsPage";
import { AboutPage } from "./settings/AboutPage";
import {
  LauncherResults,
  appSubtitleKey,
  type ActionBar,
  type CommandWarning,
  type LauncherItem,
  type SystemAction,
} from "./launcher/LauncherResults";
import "./App.css";

if (IS_WINDOWS) {
  document.documentElement.classList.add("platform-windows");
} else if (IS_MAC) {
  document.documentElement.classList.add("platform-macos");
} else if (IS_LINUX) {
  document.documentElement.classList.add("platform-linux");
}

/** Any surface a plugin page can be opened over; it replaces the canvas and
 * returns to the remembered one when dismissed. */
type ViewMode = "collapsed" | "terminal" | "settings" | "plugin";
type SettingsPage = "general" | "shortcuts" | "sessions" | "integrations" | "about";
export type CursorShape = "beam" | "block" | "underline";
type ExternalTerminalOutcome = { session_handed_off: boolean };

export type BrokerSessionInfo = {
  sessionId: string;
  name: string;
  attached: boolean;
  exited: boolean;
  exitCode: number;
  createdAt: string;
  width: number;
  height: number;
  size: string;
  cwd: string;
};

export type LocalApplication = {
  name: string;
  localizedName?: string | null;
  path: string;
  iconPath?: string | null;
  comment?: string | null;
  /** Latin search key built by the backend; see `compute_initials` there. */
  initials: string;
  /** Other names the platform knows the app by — bundle identifier, executable,
   * desktop-entry keywords. Never shown; see `aliases` in the backend. */
  aliases?: string[] | null;
};

/** Answer to `check_applications`: whether a rescan would find anything new. */
type ApplicationsStatus = { upToDate: boolean; count: number };

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

export type AppSettings = {
  hotkey: string;
  hide_on_blur: boolean;
  launch_at_startup: boolean;
  theme: string;
  font_size: number;
  font_family: string;
  cursor_shape: CursorShape;
  language: Language;
  main_opacity: number;
  terminal_opacity: number;
  shortcuts: ShortcutMap;
  /** Whether extension/provider commands appear in launcher search results. */
  show_commands_in_search: boolean;
  /** Whether the built-in clipboard history monitor runs (default on). */
  clipboard_history_enabled: boolean;
  /** Global hotkey that summons the clipboard panel. */
  clipboard_history_hotkey: string;
  /** Application path -> launch count, ranking the empty-query recent list. */
  launch_counts: Record<string, number>;
};

const FALLBACK_FONT_FAMILY =
  "'SF Mono','Menlo','Monaco','Consolas','JetBrains Mono',monospace";
const LINE_HEIGHT = 1.4;
const PADDING_X = 3;
const PADDING_Y = 3;
const INPUT_WINDOW_WIDTH = 720;
const SETTINGS_WINDOW_HEIGHT = 580;
const SETTINGS_MIN_HEIGHT = 420;
const TERMINAL_SIZE_SAVE_DELAY = 280;
const MAX_RESULTS = 6;
const BRACKETED_PASTE = 1 << 4;
/** How long the panel ignores a blur after a Windows drag; see `startDrag`.
 *  `start_dragging()` opens a modal move loop the webview spends unfocused,
 *  just like the old `WM_NCLBUTTONDOWN` path did - so the same grace period is
 *  still needed to keep the hide-on-blur listener from dismissing the panel
 *  out from under the drag. */
const DRAG_BLUR_GRACE = 600;
/** Second attempt at handing the terminal canvas the keyboard on Windows, where
 * the window is still being shown and focused when the first one lands. */
const TERMINAL_FOCUS_RETRY = 180;
/** Idle window before an icon is fetched, so the intermediate result lists that
 * flash past while a query is still being typed cost nothing. */
const ICON_LOAD_DELAY = 250;
const CATALOG_SEARCH_DELAY = 140;
const COMMAND_LINE_SYNTAX = IS_WINDOWS ? "windows" : "posix";

const terminalFontFamily = (value: string): string => {
  const family = value.trim();
  if (!family || family === "monospace") return FALLBACK_FONT_FAMILY;
  const escaped = family.replace(/[\\']/g, "\\$&");
  return `'${escaped}',${FALLBACK_FONT_FAMILY}`;
};

type ModifierEvent = {
  shiftKey: boolean;
  altKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
};

function terminalMouseModifiers(event: ModifierEvent): number {
  return (event.shiftKey ? 4 : 0) | (event.altKey || event.metaKey ? 8 : 0) | (event.ctrlKey ? 16 : 0);
}

/** An application with its searchable names normalized once, up front. */
type SearchableApp = {
  app: LocalApplication;
  names: string[];
  initials: string;
  aliases: string[];
};

/**
 * Best score for a needle across an application's names, its initials and the
 * aliases the platform knows it by.
 *
 * The three are separate keys rather than one list because they need different
 * ceilings. The initials are a shorthand: `wyyyy` *is* the whole of
 * "网易云音乐"'s key, so it would otherwise score a perfect 1000 and outrank an
 * application whose actual name the query spells out in full. An alias is
 * weaker still — nobody looking at the launcher can see that "企业微信" is also
 * `WXWork`, so a match on one is a guess about intent, and it is capped below
 * every visible-name match. Loose subsequence hits are dropped there entirely
 * for the same reason: a bundle identifier is long enough that some scattered
 * subsequence of almost any query can be found in one.
 */
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
];


type FramePayload = { id: string; generation: number; frame: string };
type ExitPayload = { id: string; generation: number; code: number | null };

type DragState =
  | { mode: "none" | "select" | "scroll" }
  | { mode: "mouse"; button: number };

export default function App() {
  const inputRef = useRef<HTMLInputElement>(null);
  /** The launcher card, measured to size the window around it. */
  const collapsedCardRef = useRef<HTMLDivElement>(null);
  const mountRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const terminalTextInputRef = useRef<HTMLTextAreaElement>(null);
  const terminalComposing = useRef(false);
  const rendererRef = useRef<TerminalCanvas | null>(null);
  const frameRef = useRef<Uint8Array | null>(null);
  const blinkRef = useRef(true);
  const dimsRef = useRef<{ cols: number; rows: number }>({ cols: 80, rows: 24 });
  const selectionRef = useRef<Selection | null>(null);
  const dragRef = useRef<DragState>({ mode: "none" });
  const lastScrollAt = useRef(0);
  const lastMouseReportAt = useRef(0);
  const wheelRemainder = useRef(0);
  const terminalSizeSaveTimer = useRef<number | null>(null);
  const pendingTerminalSize = useRef<{ width: number; height: number } | null>(null);
  const settingsSaveTimer = useRef<number | null>(null);
  const settingsHydration = useMemo(() => createSettingsHydration<AppSettings>(), []);
  const settingsLoadPromise = useRef<Promise<void> | null>(null);
  const hydrationSavePromise = useRef<Promise<void> | null>(null);
  const settingsSaveGeneration = useRef(0);
  const appQuitting = useRef(false);
  const clickSeq = useRef({ count: 0, time: 0, col: -1, row: -1 });

  const ptyReady = useRef(false);
  const terminalGeneration = useRef<number | null>(null);
  /** Daemon-side id of the PTY the main view is attached to; captured at
   * spawn/attach so pinning can hand the session to the card without a
   * listing round-trip. */
  const mainBrokerSessionIdRef = useRef<string | null>(null);
  const pinnedRendererRef = useRef<TerminalCanvas | null>(null);
  const pinBusy = useRef(false);
  const nextTerminalGeneration = useRef(Date.now());
  const sessionClosePromise = useRef<Promise<unknown> | null>(null);
  const termOpened = useRef(false);
  const terminalOpening = useRef(false);
  const externalTerminalOpening = useRef(false);
  const systemPowerOpening = useRef(false);
  const launcherFeedbackTimer = useRef<number | null>(null);
  const terminalFeedbackTimer = useRef<number | null>(null);
  const draftBeforeHistory = useRef("");
  const restoringMode = useRef<ViewMode | null>(null);
  /** Guards against two scans overlapping: a cold cache reads as out of date, so
   * a summon during the very first scan would otherwise start a second one. */
  const appScanning = useRef(false);

  const [mode, setMode] = useState<ViewMode>("collapsed");
  /** Ref mirror of `mode` so event listeners registered once can still see the
   * current value (the clipboard hotkey toggles against it). */
  const modeRef = useRef<ViewMode>("collapsed");
  useEffect(() => { modeRef.current = mode; }, [mode]);
  /** Where an open plugin page returns to when dismissed. */
  const pluginReturnMode = useRef<"collapsed" | "terminal">("collapsed");
  /** Which plugin page is showing while `mode === "plugin"` (one at a time). */
  const [pluginPageId, setPluginPageId] = useState<string | null>(null);
  const pluginPageIdRef = useRef<string | null>(null);
  useEffect(() => { pluginPageIdRef.current = pluginPageId; }, [pluginPageId]);
  const [settingsPage, setSettingsPage] = useState<SettingsPage>("general");
  const [query, setQuery] = useState("");
  const [terminalMounted, setTerminalMounted] = useState(false);
  // Which surface owns the keyboard: the main terminal area or the pin card.
  // Clicking a surface claims it; Escape or an outside click returns it to the
  // main view. Mirrored into a ref because the window keydown handler must see
  // the current value without resubscribing.
  const [activeSurface, setActiveSurfaceState] = useState<"main" | "pinned">("main");
  const activeSurfaceRef = useRef<"main" | "pinned">("main");
  const setActiveSurface = useCallback((surface: "main" | "pinned") => {
    activeSurfaceRef.current = surface;
    setActiveSurfaceState(surface);
  }, []);
  /** True while the card holds the only view of a live session (the main slot
   * is empty); drives the placeholder in the terminal panel. */
  const [mainPinnedAway, setMainPinnedAway] = useState(false);
  const { pinState, dispatchPinEvent, geometry: cardGeometry, updateGeometry: updateCardGeometry } = usePinnedTerminal();
  const pinStateRef = useRef(pinState);
  pinStateRef.current = pinState;
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [applications, setApplications] = useState<LocalApplication[]>([]);
  /** True until the first scan settles, whether it hit the cache or not. */
  const [appsLoading, setAppsLoading] = useState(true);
  const [appsError, setAppsError] = useState(false);
  const [appVersion, setAppVersion] = useState("DEV");
  const [appIconUrls, setAppIconUrls] = useState<Record<string, string>>({});
  // Ref mirror of `appIconUrls` so the icon-loading effect can check which
  // icons are already resolved without subscribing to the state change —
  // without this, every icon that resolves re-triggers the effect, which
  // re-sets the timer, which delays every subsequent icon.
  const appIconUrlsRef = useRef(appIconUrls);
  const appIconAttempts = useRef(new Set<string>());
  useEffect(() => { appIconUrlsRef.current = appIconUrls; }, [appIconUrls]);
  const [catalogSuggestions, setCatalogSuggestions] = useState<CatalogSuggestion[]>([]);
  const [terminalSessions, setTerminalSessions] = useState<BrokerSessionInfo[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [sessionsError, setSessionsError] = useState(false);
  const [sessionActionId, setSessionActionId] = useState<string | null>(null);
  // Two-step kill confirmation: while set, that row's kill button shows a
  // short confirm pill instead of the icon. Cleared by a second click, a
  // different row's kill click, or a 3s timeout.
  const [killConfirmId, setKillConfirmId] = useState<string | null>(null);
  // Session refreshes can overlap when the user switches pages or retries
  // quickly. Only the newest response is allowed to update the list and its
  // loading/error state; an older response may otherwise resurrect a session
  // that a later refresh has already removed.
  const sessionsRequestGeneration = useRef(0);
  const catalogRequestGeneration = useRef(0);
  const [selectedResultIndex, setSelectedResultIndex] = useState(0);
  /** Whether the action bar, rather than a row of the result list, is the thing
   * Enter runs. The two selections are exclusive but kept apart, because the
   * action bar is not a result: it is never numbered and never in `Ctrl+N`. */
  const [selectedActionBar, setSelectedActionBar] = useState(false);
  const [settings, setSettings] = useState<AppSettings>({
    hotkey: "Ctrl+Space",
    hide_on_blur: true,
    launch_at_startup: false,
    theme: "dark",
    font_size: 14,
    font_family: "monospace",
    cursor_shape: "beam",
    language: "en",
    main_opacity: 94,
    terminal_opacity: 92,
    shortcuts: DEFAULT_SHORTCUTS,
    show_commands_in_search: false,
    clipboard_history_enabled: true,
    // The clipboard panel ships with NO global hotkey; users may bind one on
    // the shortcuts settings page.
    clipboard_history_hotkey: "",
    launch_counts: {},
  });
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [settingsSaveFailed, setSettingsSaveFailed] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(true);
  const [settingsLoadFailed, setSettingsLoadFailed] = useState(false);
  const settingsRef = useRef(settings);
  useEffect(() => { settingsRef.current = settings; }, [settings]);

  const saveSettings = useMemo(
    () => createSerialSettingsWriter<AppSettings>((next) =>
      invoke("save_settings", { settings: next }),
    ),
    [],
  );

  const commitSettings = (next: AppSettings): Promise<void> => {
    const generation = ++settingsSaveGeneration.current;
    setSettingsSaving(true);
    return saveSettings(next).then(
      () => {
        if (settingsSaveGeneration.current !== generation) return;
        setSettingsSaving(false);
        setSettingsSaveFailed(false);
      },
      (error) => {
        if (settingsSaveGeneration.current === generation) {
          setSettingsSaving(false);
          setSettingsSaveFailed(true);
        }
        throw error;
      },
    );
  };

  // Startup remains interactive while settings load. Delay and coalesce writes
  // until hydration finishes so a default frontend snapshot cannot overwrite
  // fields that have not arrived from disk yet.
  const persistSettings = (): Promise<void> => {
    if (settingsHydration.isReady()) {
      return commitSettings(settingsRef.current);
    }
    if (!hydrationSavePromise.current) {
      const pending = settingsHydration
        .waitUntilReady()
        .then(() => commitSettings(settingsRef.current));
      hydrationSavePromise.current = pending;
      const clearPending = () => {
        if (hydrationSavePromise.current === pending) hydrationSavePromise.current = null;
      };
      void pending.then(clearPending, clearPending);
    }
    return hydrationSavePromise.current;
  };
  const [recordingAction, setRecordingAction] = useState<string | null>(null);
  const [rejectedAction, setRejectedAction] = useState<string | null>(null);
  const [autostartUpdating, setAutostartUpdating] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<{ version: string } | null>(null);
  const [updateDownloading, setUpdateDownloading] = useState(false);
  const [updateProgress, setUpdateProgress] = useState<{ downloaded: number; total: number } | null>(null);
  const [updateFailed, setUpdateFailed] = useState(false);
  const [launcherFeedback, setLauncherFeedback] = useState<MessageKey | null>(null);
  const [terminalFeedback, setTerminalFeedback] = useState<MessageKey | null>(null);
  const isComposing = useRef(false);
  const suppressBlurUntil = useRef(0);
  /** Mirror of the OS focus state as the listener below has seen it. Lives in
   *  a ref rather than the effect so it survives the re-subscription every
   *  mode change makes: a Focused(false) that arrives without a matching
   *  Focused(true) before it is not a blur the user produced, it is GTK
   *  reporting a window that never had keyboard focus — on Wayland the
   *  compositor can refuse the activation request a global-hotkey summon
   *  makes (focus-stealing prevention), the panel maps unfocused, and the
   *  first report it then hands out must not be read as "the user clicked
   *  away". Hiding on that would dismiss the panel the instant it appeared
   *  on exactly the platforms where it never got focus to lose. */
  const windowFocusedRef = useRef(false);

  const language = normalizeLanguage(settings.language);
  const t = useMemo(() => createTranslator(language), [language]);
  const sessionDateFormatter = useMemo(
    () => new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en", {
      dateStyle: "medium",
      timeStyle: "short",
    }),
    [language],
  );
  const shortcuts = useMemo(
    () => withShortcutDefaults(settings.shortcuts),
    [settings.shortcuts],
  );
  const actionBarShortcut = useMemo(
    () => formatResultShortcut(shortcuts.select_result, "Enter"),
    [shortcuts.select_result],
  );

  const showLauncherFeedback = (key: MessageKey) => {
    setLauncherFeedback(key);
    if (launcherFeedbackTimer.current !== null) {
      window.clearTimeout(launcherFeedbackTimer.current);
    }
    launcherFeedbackTimer.current = window.setTimeout(() => {
      launcherFeedbackTimer.current = null;
      setLauncherFeedback(null);
    }, 4500);
  };

  const showTerminalFeedback = (key: MessageKey) => {
    setTerminalFeedback(key);
    if (terminalFeedbackTimer.current !== null) {
      window.clearTimeout(terminalFeedbackTimer.current);
    }
    terminalFeedbackTimer.current = window.setTimeout(() => {
      terminalFeedbackTimer.current = null;
      setTerminalFeedback(null);
    }, 4500);
  };

  // The system appearance, tracked whether or not it is currently being followed.
  // Subscribing unconditionally rather than only in `auto` mode keeps this from
  // going stale: a listener attached on entering `auto` would miss every change
  // that happened while the theme was pinned, and would then report the wrong
  // answer for as long as it took the appearance to change again.
  const [systemTheme, setSystemTheme] = useState<"dark" | "light">(() =>
    window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark",
  );

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: light)");
    const onChange = (event: MediaQueryListEvent) =>
      setSystemTheme(event.matches ? "light" : "dark");
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  // `auto` is resolved here rather than in CSS: the canvas renderer reads the
  // same custom properties through `getComputedStyle` and needs a concrete
  // answer, so a single resolved value drives both and they cannot disagree.
  const resolvedTheme: "dark" | "light" =
    settings.theme === "auto" ? systemTheme : settings.theme === "light" ? "light" : "dark";

  // Keep document metadata in sync with the active locale. Tauri does not show
  // the document title in the main window, but it is still exposed to screen
  // readers, browser tooling, and platform window switchers.
  useEffect(() => {
    document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
    document.title = "Floter";
  }, [language]);

  useEffect(() => {
    const root = document.documentElement.style;
    root.setProperty("--main-opacity", String(normalizeOpacity(settings.main_opacity) / 100));
    root.setProperty("--terminal-opacity", String(normalizeOpacity(settings.terminal_opacity) / 100));
    const renderer = rendererRef.current;
    if (renderer) {
      renderer.updateTheme();
      render();
    }
  }, [settings.main_opacity, settings.terminal_opacity]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", resolvedTheme);
    // After the attribute, never before: the renderer resolves `--terminal-*`
    // off the document element, so it has to be asked once the attribute has
    // selected the palette. Only reachable while the terminal is open — the
    // settings panel unmounts it, and a renderer built later reads the current
    // theme in its constructor.
    const renderer = rendererRef.current;
    if (renderer) {
      renderer.updateTheme();
      render();
      invoke("term_set_theme", { id: "main", theme: resolvedTheme }).catch(() => undefined);
    }
    // The pinned card runs its own emulator instance against its own session;
    // keep its palette in step too.
    invoke("term_set_theme", { id: PINNED_SESSION_ID, theme: resolvedTheme }).catch(() => undefined);
  }, [resolvedTheme]);

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
    const includeSystemCommands = settings.show_commands_in_search;

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
      const search = structuredCommand ? invoke<CatalogEntry[]>("catalog_search", {
        request: {
          query,
          tokens: searchTokens,
          environment: searchLine.environment,
          cwd: null,
          limit: 20,
          includeSystemCommands,
        },
      }) : Promise.resolve<CatalogEntry[]>([]);
      const complete = wantsCompletion
        ? invoke<CatalogCompletionResponse>("catalog_complete", {
            request: {
              command,
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
            entry.aliases.includes(command),
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
  }, [query, settings.show_commands_in_search]);

  /**
   * The numbered result list: applications and the built-in system actions.
   *
   * Running the query as a command used to live in here too, wedged into the
   * second slot. It is the action bar now — a command is not a search result, it
   * is what to do with a search that found nothing, and giving it a row of its
   * own leaves every numbered slot for something that was actually matched.
   */
  const launcherResults = useMemo<LauncherItem[]>(() => {
    const command = query.trim();
    const parsedQuery = parseCommandLine(query, false, COMMAND_LINE_SYNTAX);
    if (!command) {
      // The empty query is the launcher's front door: a summon with nothing
      // typed yet still has something useful to offer. Rank the applications
      // the user actually starts by launch count and render them as ordinary
      // results, so the numbered shortcuts and Enter work unchanged. Typing
      // any character leaves this branch.
      const byPath = new Map(searchableApps.map((entry) => [entry.app.path, entry]));
      const recentPaths = recentItems(
        settings.launch_counts,
        searchableApps.map((entry) => entry.app.path),
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
    const commandLimit = rankedMatches.length
      ? Math.min(3, MAX_RESULTS - 2)
      : MAX_RESULTS - 1;
    const commandCounts = catalogSuggestions.reduce<Map<string, number>>((counts, suggestion) => {
      const command = suggestion.entry.command;
      counts.set(command, (counts.get(command) ?? 0) + 1);
      return counts;
    }, new Map());
    const commandItems: LauncherItem[] = catalogSuggestions
      .slice(0, commandLimit)
      .map((suggestion) => {
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
        return {
          type: "command",
          id: entry.id,
          title: entry.command,
          subtitle: entry.description,
          warnings,
          sourceName: entry.sourceName,
          commandLine: parsedQuery.commandIndex !== null && (
            parsedQuery.commandIndex > 0 ||
            parsedQuery.tokens.length > parsedQuery.commandIndex + 1
          ) ? query : `${entry.command} `,
          execution: entry.execution,
          completion: false,
        };
      });

    // The action bar occupies the final row. Keep at least one local match when
    // applications or power actions matched alongside catalog commands.
    return [...commandItems, ...rankedMatches].slice(0, MAX_RESULTS - 1);
  }, [catalogSuggestions, query, searchableApps, settings.launch_counts, t]);

  const actionBar = useMemo<ActionBar | null>(() => {
    const value = query.trim();
    if (!value) return null;
    const type = classifyActionBar(value);
    const label = type === "url"
      ? t("launcher.openInBrowser")
      : type === "path"
        ? t("launcher.openInFiles")
        : t("launcher.runInShell");
    return { type, label, value };
  }, [query, t]);

  const runnableResultFlags = launcherResults.map(
    (item) => item.type !== "command" || Boolean(item.execution),
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
   * A boolean rather than something the effect below recomputes, so that the
   * effect fires when the *answer* changes and not merely when the result list is
   * rebuilt. A background application refresh can give `launcherResults` a new
   * identity even when nothing about the visible matches changed —
   * depending on the list itself would throw away a selection the user had
   * already moved with the arrow keys.
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

  const refreshTerminalSessions = () => {
    const generation = beginRequest(sessionsRequestGeneration);
    setSessionsLoading(true);
    setSessionsError(false);
    return invoke<BrokerSessionInfo[]>("term_list_sessions")
      .then((sessions) => {
        if (!isCurrentRequest(sessionsRequestGeneration, generation)) return;
        setTerminalSessions(sessions);
        setSessionsError(false);
      })
      .catch(() => {
        if (!isCurrentRequest(sessionsRequestGeneration, generation)) return;
        setTerminalSessions([]);
        setSessionsError(true);
      })
      .finally(() => {
        if (isCurrentRequest(sessionsRequestGeneration, generation)) {
          setSessionsLoading(false);
        }
      });
  };

  useEffect(() => {
    scanApplications(false);
  }, []);

  // A new query starts from its own default: the first result for a name, the
  // action bar for a command line, a URL or a path. See `defaultsToActionBar`.
  useEffect(() => {
    setSelectedResultIndex(firstRunnableResultIndex < 0 ? 0 : firstRunnableResultIndex);
    setSelectedActionBar(defaultsToActionBar);
  }, [defaultsToActionBar, firstRunnableResultIndex, query]);

  useEffect(() => {
    setSelectedResultIndex((index) => {
      if (!launcherResults.length) return 0;
      return Math.min(index, launcherResults.length - 1);
    });
  }, [launcherResults.length]);

  // The launcher window is exactly as tall as the rows inside it, measured
  // rather than predicted.
  //
  // The height used to be added up from constants — one row height, one gap,
  // one padding — and the sum came out a few pixels short of what the
  // stylesheet actually produced, so the bottom of the action bar was cut off
  // by the window edge. Every one of those numbers duplicated a CSS value that
  // could be changed without anyone thinking to come back here; reading the
  // laid-out rows keeps the two in step by construction, and covers the
  // platform differences (Windows draws no card border) for free.
  //
  // `useLayoutEffect` so the measurement happens on the frame the rows were
  // committed, before the window is painted at the old size. Offsets rather
  // than `getBoundingClientRect`, because the shell plays a scale animation on
  // entry and a rect measured mid-animation is a rect scaled by 0.986.
  useLayoutEffect(() => {
    if (mode !== "collapsed") return;
    syncLauncherHeight();
  }, [actionBar, launcherFeedback, launcherResults.length, mode]);

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

  const positionTerminalTextInput = () => {
    const renderer = rendererRef.current;
    const input = terminalTextInputRef.current;
    if (!renderer || !input) return;
    const cursor = renderer.cursorRect();
    input.style.transform = `translate(${cursor.x}px, ${cursor.y}px)`;
    input.style.height = `${cursor.height}px`;
  };

  const render = () => {
    const renderer = rendererRef.current;
    const frame = frameRef.current;
    if (renderer && frame) {
      renderer.draw(frame, blinkRef.current, selectionRef.current);
      positionTerminalTextInput();
    }
  };

  /** The frontend id keystrokes must reach right now: the main view, or the
   * pinned card after its body was clicked. */
  const terminalInputTarget = (): string =>
    activeSurfaceRef.current === "pinned" ? PINNED_SESSION_ID : "main";

  /** The renderer whose emulator mode governs key encoding for the active
   * surface — the two views can run programs with different modes. */
  const activeRenderer = (): TerminalCanvas | null =>
    activeSurfaceRef.current === "pinned" ? pinnedRendererRef.current : rendererRef.current;

  /**
   * Size the launcher window to the rows currently laid out inside it.
   *
   * Measured to the bottom of the card's last child rather than from the card's
   * own box: the card is at least as tall as the window it sits in, so its
   * height says what the window *is* rather than what it should be. `offsetTop`
   * starts inside the card's border, so both borders are added back on — and
   * offsets rather than `getBoundingClientRect`, because the shell plays a
   * scale animation on entry and a rect measured mid-animation is a rect
   * scaled by 0.986.
   */
  const syncLauncherHeight = () => {
    const card = collapsedCardRef.current;
    const last = card?.lastElementChild as HTMLElement | null;
    if (!card || !last) return;
    const style = getComputedStyle(card);
    const frame =
      (parseFloat(style.borderTopWidth) || 0) +
      (parseFloat(style.borderBottomWidth) || 0) +
      (parseFloat(style.paddingBottom) || 0);
    let height = Math.ceil(last.offsetTop + last.offsetHeight + frame);
    if (!height) return;
    // The shell wrapping the card may carry padding (Windows uses it to give
    // the CSS box-shadow room outside the card), and the window has to be
    // that much taller for the padding to actually show.
    const shell = card.parentElement;
    if (shell) {
      const shellStyle = getComputedStyle(shell);
      height +=
        (parseFloat(shellStyle.paddingTop) || 0) +
        (parseFloat(shellStyle.paddingBottom) || 0);
    }
    getCurrentWindow()
      .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height))
      .catch(() => undefined);
  };

  const focusCollapsedInput = (delay = 0) => {
    window.setTimeout(() => {
      const input = inputRef.current;
      if (!input) return;
      input.focus({ preventScroll: true });
      const length = input.value.length;
      input.setSelectionRange(length, length);
    }, delay);
  };

  // The terminal canvas is the active element when collapsed mode is committed.
  // Focus the newly mounted input in that same commit instead of relying only on
  // timers that can race the native terminal-to-launcher resize.
  useLayoutEffect(() => {
    if (mode !== "collapsed") return;
    const input = inputRef.current;
    if (!input) return;
    input.focus({ preventScroll: true });
    const length = input.value.length;
    input.setSelectionRange(length, length);
  }, [mode]);

  const focusTerminalView = (delay = 0) => {
    window.setTimeout(() => {
      relayoutAndResize();
      terminalTextInputRef.current?.focus({ preventScroll: true });
    }, delay);
  };

  const relayoutAndResize = () => {
    const renderer = rendererRef.current;
    const mount = mountRef.current;
    if (!renderer || !mount) return;
    const rect = mount.getBoundingClientRect();
    const layout = renderer.relayout(rect.width, rect.height);
    dimsRef.current = layout;
    positionTerminalTextInput();
    invoke("term_resize", { id: "main", cols: layout.cols, rows: layout.rows });
    render();
  };

  const resetTerminalFrontendState = () => {
    frameRef.current = null;
    selectionRef.current = null;
    dragRef.current = { mode: "none" };
    clickSeq.current = { count: 0, time: 0, col: -1, row: -1 };
  };

  const closeTerminalSession = () => {
    ptyReady.current = false;
    terminalGeneration.current = null;
    resetTerminalFrontendState();
    const closing = invoke("term_close", { id: "main" }).catch(() => undefined);
    sessionClosePromise.current = closing;
    closing.finally(() => {
      if (sessionClosePromise.current === closing) {
        sessionClosePromise.current = null;
      }
    });
  };

  const ensureTerminalSession = async (
    initialCommand: string | null = null,
    execution: ExecutionPlan | null = null,
  ) => {
    if (sessionClosePromise.current) {
      await sessionClosePromise.current;
    }
    if (ptyReady.current) return;
    const { cols, rows } = dimsRef.current;
    const generation = ++nextTerminalGeneration.current;
    terminalGeneration.current = generation;
    try {
      // term_spawn hands back the daemon-side session id (see the Rust
      // command), remembered so pinning can re-attach this PTY to the card.
      const brokerSessionId = await invoke<string>("term_spawn", {
        id: "main",
        generation,
        shell: null,
        initialCommand,
        execution,
        theme: resolvedTheme,
        cols,
        rows,
      });
      if (terminalGeneration.current === generation) {
        ptyReady.current = true;
        mainBrokerSessionIdRef.current = brokerSessionId;
        setMainPinnedAway(false);
      }
    } catch (error) {
      if (terminalGeneration.current === generation) {
        terminalGeneration.current = null;
      }
      throw error;
    }
  };

  const handleTerminalExit = () => {
    closeTerminalSession();
    setTerminalFeedback(null);
    setQuery("");
    setTerminalMounted(false);
    setMode("collapsed");
    focusCollapsedInput(90);
    focusCollapsedInput(140);
  };

  const loadSettings = (): Promise<void> => {
    if (settingsLoadPromise.current) return settingsLoadPromise.current;
    setSettingsLoading(true);
    const request = invoke<AppSettings>("get_settings")
      .then((loaded) => {
        const normalized = {
          ...loaded,
          language: normalizeLanguage(loaded.language),
          launch_at_startup: loaded.launch_at_startup ?? false,
          main_opacity: normalizeOpacity(loaded.main_opacity ?? 94),
          terminal_opacity: normalizeOpacity(loaded.terminal_opacity ?? 92),
          shortcuts: withShortcutDefaults(loaded.shortcuts),
          clipboard_history_enabled: loaded.clipboard_history_enabled ?? true,
          clipboard_history_hotkey: loaded.clipboard_history_hotkey ?? "",
          launch_counts: loaded.launch_counts ?? {},
        };
        const hydrated = settingsHydration.mergeLoaded(settingsRef.current, normalized);
        settingsRef.current = hydrated;
        setSettings(hydrated);
        setSettingsLoadFailed(false);
        settingsHydration.finish();
      })
      .catch(() => {
        settingsHydration.markFailed();
        setSettingsLoadFailed(true);
      })
      .finally(() => {
        if (settingsLoadPromise.current === request) settingsLoadPromise.current = null;
        setSettingsLoading(false);
      });
    settingsLoadPromise.current = request;
    return request;
  };

  useEffect(() => {
    void loadSettings();
  }, [settingsHydration]);

  // The webview's built-in right-click menu must never appear, on any
  // surface. A capture-phase window listener covers every element including
  // nodes without their own React handler (launcher card, settings pages,
  // plugin iframe chrome); the per-element handlers stay as belt-and-braces.
  useEffect(() => {
    const suppressContextMenu = (event: MouseEvent) => {
      event.preventDefault();
    };
    window.addEventListener("contextmenu", suppressContextMenu, true);
    return () => {
      window.removeEventListener("contextmenu", suppressContextMenu, true);
    };
  }, []);

  useEffect(() => {
    invoke<string>("app_version")
      .then(setAppVersion)
      .catch(() => undefined);
  }, []);

  // A cold start with `floter clip` records a pending plugin page in the
  // backend during setup; consume it once this component's listeners are up.
  useEffect(() => {
    invoke<string | null>("take_pending_plugin_page")
      .then((pending) => {
        if (pending) openPluginPage(pending);
      })
      .catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const unlistenFramePromise = listen<FramePayload>("term://frame", (event) => {
      if (
        event.payload.id !== "main" ||
        event.payload.generation !== terminalGeneration.current
      )
        return;
      frameRef.current = decodeFrame(event.payload.frame);
      blinkRef.current = true;
      render();
    });

    const unlistenExitPromise = listen<ExitPayload>("term://exit", (event) => {
      if (
        event.payload.id !== "main" ||
        event.payload.generation !== terminalGeneration.current
      )
        return;
      handleTerminalExit();
    });

    return () => {
      unlistenFramePromise.then((unlisten) => unlisten());
      unlistenExitPromise.then((unlisten) => unlisten());
      rendererRef.current = null;
      frameRef.current = null;
      termOpened.current = false;
      ptyReady.current = false;
      terminalGeneration.current = null;
    };
  }, []);

  // The renderer is bound to the canvas element of the mode that mounted it.
  // Keyed on `mode` too, so leaving the terminal page (for the clipboard page
  // or settings) tears the renderer down with its canvas and re-entering
  // builds a fresh one against the newly mounted node. Frames fully replace
  // each other and `frameRef` survives the flip, so the switch is lossless:
  // the last frame repaints immediately and the embedded PTY never stopped
  // running underneath.
  useEffect(() => {
    if (!terminalMounted || mode === "plugin" || mode === "settings") {
      termOpened.current = false;
      rendererRef.current = null;
      return;
    }
    if (mode !== "terminal") return;
    if (!canvasRef.current || !mountRef.current || termOpened.current) return;

    const renderer = new TerminalCanvas(canvasRef.current, {
      fontFamily: terminalFontFamily(settings.font_family),
      fontSize: normalizeFontSize(settings.font_size),
      lineHeight: LINE_HEIGHT,
      paddingX: PADDING_X,
      paddingY: PADDING_Y,
    });
    rendererRef.current = renderer;
    termOpened.current = true;

    relayoutAndResize();

    const resizeObserver = new ResizeObserver(() => relayoutAndResize());
    resizeObserver.observe(mountRef.current);

    const onWheelNative = (event: WheelEvent) => {
      const renderer = rendererRef.current;
      if (!renderer || event.deltaY === 0) return;
      event.preventDefault();

      const page = renderer.cellHeight * Math.max(1, renderer.rows);
      const pixels =
        event.deltaMode === WheelEvent.DOM_DELTA_LINE
          ? event.deltaY * renderer.cellHeight
          : event.deltaMode === WheelEvent.DOM_DELTA_PAGE
            ? event.deltaY * page
            : event.deltaY;
      const unit = Math.max(24, renderer.cellHeight * 1.5);
      wheelRemainder.current += pixels;
      const rawSteps = Math.trunc(wheelRemainder.current / unit);
      if (rawSteps === 0) return;
      wheelRemainder.current -= rawSteps * unit;

      const point = renderer.pixelToCell(event.offsetX, event.offsetY) ?? {
        col: Math.max(0, Math.min(renderer.cols - 1, Math.floor(event.offsetX / renderer.cellWidth))),
        row: Math.max(0, Math.min(renderer.rows - 1, Math.floor(event.offsetY / renderer.cellHeight))),
      };
      invoke("term_wheel", {
        id: "main",
        delta: Math.max(-8, Math.min(8, -rawSteps)),
        column: point.col,
        row: point.row,
        modifiers: terminalMouseModifiers(event),
      });
    };
    wheelRemainder.current = 0;
    canvasRef.current.addEventListener("wheel", onWheelNative, { passive: false });

    const blink = window.setInterval(() => {
      blinkRef.current = !blinkRef.current;
      render();
    }, 530);

    return () => {
      window.clearInterval(blink);
      resizeObserver.disconnect();
      canvasRef.current?.removeEventListener("wheel", onWheelNative);
      termOpened.current = false;
      rendererRef.current = null;
    };
  }, [settings.font_family, settings.font_size, terminalMounted, mode]);

  // Native edge resizing owns terminal geometry. ResizeObserver keeps the PTY
  // grid current; this listener persists the logical window dimensions after a
  // short idle period, so a single drag writes once rather than every frame.
  useEffect(() => {
    if (!terminalMounted || mode !== "terminal") return;
    const currentWindow = getCurrentWindow();
    let disposed = false;
    let unlisten: (() => void) | undefined;

    currentWindow.onResized(async ({ payload }) => {
      if (disposed) return;
      const scale = await currentWindow.scaleFactor().catch(() => window.devicePixelRatio || 1);
      const width = payload.width / scale;
      const height = payload.height / scale;
      if (!Number.isFinite(width) || !Number.isFinite(height)) return;
      pendingTerminalSize.current = { width, height };
      if (terminalSizeSaveTimer.current !== null) {
        window.clearTimeout(terminalSizeSaveTimer.current);
      }
      terminalSizeSaveTimer.current = window.setTimeout(() => {
        terminalSizeSaveTimer.current = null;
        const pending = pendingTerminalSize.current;
        pendingTerminalSize.current = null;
        if (pending) invoke("save_terminal_size", pending).catch(() => undefined);
      }, TERMINAL_SIZE_SAVE_DELAY);
    }).then((dispose) => {
      if (disposed) {
        dispose();
      } else {
        unlisten = dispose;
      }
    });

    return () => {
      disposed = true;
      unlisten?.();
      if (terminalSizeSaveTimer.current !== null) {
        window.clearTimeout(terminalSizeSaveTimer.current);
        terminalSizeSaveTimer.current = null;
      }
      const pending = pendingTerminalSize.current;
      pendingTerminalSize.current = null;
      if (pending) invoke("save_terminal_size", pending).catch(() => undefined);
    };
  }, [mode, terminalMounted]);

  // Settings is a compact work panel, not a document. Its header stays fixed
  // while the body scrolls; smaller displays get a proportional cap.
  useEffect(() => {
    if (mode !== "settings") return;
    const available = window.screen.availHeight;
    const height = Math.min(
      SETTINGS_WINDOW_HEIGHT,
      Math.max(SETTINGS_MIN_HEIGHT, Math.floor(available * 0.72)),
      Math.max(240, available - 24),
    );
    getCurrentWindow()
      .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height))
      .catch(() => undefined);
  }, [mode]);

  // An armed recorder unmounts with the panel, but the flag that hands it the
  // keyboard lives here. Leaving it set would mute every key handler in the app,
  // so it is cleared on the way out of settings — the panel can be left by the
  // close button, the global toggle or the tray, and each of those would
  // otherwise need its own reset.
  useEffect(() => {
    if (mode === "settings") return;
    setRecordingAction(null);
    setRejectedAction(null);
  }, [mode]);

  useEffect(() => {
    const isRestoring = restoringMode.current === mode;
    suppressBlurUntil.current = Date.now() + 400;

    if (mode === "settings") {
      // Opened from the collapsed card: the window is already visible and keeps
      // its top edge, so only the panel height changes.
      return;
    }

    if (mode === "plugin") {
      // Sizing is owned by the BACKEND on this path: a plugin page is a
      // terminal page and takes exactly the terminal window's saved geometry
      // through the same machinery (`show_plugin_page` in lib.rs). The
      // frontend deliberately never calls setSize while the page is up — one
      // side owns the size, so a stale launcher measurement can never shrink
      // the window out from under a long list again.
      invoke("show_plugin_page").catch(() => undefined);
      return;
    }

    if (mode === "collapsed") {
      if (!isRestoring) {
        // `show_input` resizes to the bare input row, which is the right height
        // for an empty query and a couple of pixels short of one with results
        // (or of a card that draws a border). It lands after the layout effect
        // above, so the measured height is applied again once it has.
        invoke("show_input")
          .then(() => {
            // The clipboard hotkey can summon the panel between this effect's
            // start and the command's completion; its reply must not then
            // shrink the window back to launcher height. Only the surface that
            // is actually showing may size itself.
            if (modeRef.current !== "collapsed") return;
            syncLauncherHeight();
            focusCollapsedInput();
            if (IS_WINDOWS) focusCollapsedInput(TERMINAL_FOCUS_RETRY);
          })
          .catch(() => undefined);
      }
      focusCollapsedInput(90);
      focusCollapsedInput(140);
      const timer = window.setTimeout(() => {
        if (restoringMode.current === "collapsed") {
          restoringMode.current = null;
        }
      }, 160);
      return () => window.clearTimeout(timer);
    }

    if (!isRestoring) {
      invoke("show_terminal");
    }
    focusTerminalView(80);
    // Windows shows and focuses the window around the time that first attempt
    // lands, and a canvas that missed the keyboard swallows the first key
    // pressed into it. Chased a second time, exactly as the collapsed input is.
    if (IS_WINDOWS) focusTerminalView(TERMINAL_FOCUS_RETRY);
    const timer = window.setTimeout(() => {
      if (restoringMode.current === "terminal") {
        restoringMode.current = null;
      }
    }, 160);
    return () => window.clearTimeout(timer);
  }, [mode]);

  // Search results keep the keyboard on the combobox, while Tab may move into
  // the session and settings controls. Focus is only reclaimed when it leaves
  // the card entirely, which covers a click on the window chrome or an element
  // unmounting under the caret without trapping keyboard users in the input.
  //
  // Deferred by a tick because at `focusout` time the incoming element has not
  // been focused yet, and the check has to see where the keyboard ended up.
  useEffect(() => {
    if (mode !== "collapsed") return;
    const onFocusOut = () => {
      window.setTimeout(() => {
        const activeElement = document.activeElement;
        if (
          activeElement === inputRef.current ||
          (activeElement && collapsedCardRef.current?.contains(activeElement))
        ) return;
        focusCollapsedInput();
      }, 0);
    };
    document.addEventListener("focusout", onFocusOut);
    return () => document.removeEventListener("focusout", onFocusOut);
  }, [mode]);

  useEffect(() => {
    const unlistenModePromise = listen<string>("floter://mode", (event) => {
      if (event.payload === "collapsed") {
        closeTerminalSession();
        setQuery("");
        setTerminalMounted(false);
        setMode("collapsed");
      }
    });

    const unlistenRevealPromise = listen<string>("floter://revealed", (event) => {
      if (event.payload === "terminal") {
        restoringMode.current = "terminal";
        setTerminalMounted(true);
        setMode("terminal");
        focusTerminalView(80);
        // The reveal that brought the window back is still settling on Windows;
        // see the mode effect above for why the canvas is chased twice there.
        if (IS_WINDOWS) focusTerminalView(TERMINAL_FOCUS_RETRY);
        window.setTimeout(() => {
          if (restoringMode.current === "terminal") {
            restoringMode.current = null;
          }
        }, 160);
        return;
      }

      // Only the launcher needs applications. The backend coalesces checks
      // inside a platform-specific cooldown and performs any directory walk on
      // a blocking thread; a changed source refreshes behind the existing list.
      invoke<ApplicationsStatus>("check_applications")
        .then((status) => {
          if (!status.upToDate) scanApplications(true);
        })
        .catch(() => undefined);

      restoringMode.current = "collapsed";
      setLauncherFeedback(null);
      setQuery("");
      setTerminalMounted(false);
      setMode("collapsed");
      focusCollapsedInput(90);
      focusCollapsedInput(140);
      window.setTimeout(() => {
        if (restoringMode.current === "collapsed") {
          restoringMode.current = null;
        }
      }, 160);
    });

    return () => {
      unlistenModePromise.then((unlisten) => unlisten());
      unlistenRevealPromise.then((unlisten) => unlisten());
    };
  }, []);

  // A plugin-page request. One internal path serves every trigger: the global
  // hotkey, `floter clip` (running instance or cold start) and the launcher's
  // system entry. `toggle` says the window was already visible when the hotkey
  // went down: only then does pressing it again mean "hide" — and only for the
  // very page that is already showing; any other request always opens.
  useEffect(() => {
    const unlistenPagePromise = listen<{ id: string; toggle: boolean }>("floter://plugin-page", (event) => {
      const { id, toggle } = event.payload;
      if (toggle && modeRef.current === "plugin" && pluginPageIdRef.current === id) {
        invoke("hide_window").catch(() => undefined);
        return;
      }
      openPluginPage(id);
    });

    return () => {
      unlistenPagePromise.then((unlisten) => unlisten());
    };
  }, []);

  // While the card owns the keyboard, any press outside it hands focus back
  // to the main surface (Escape is handled in the keydown path).
  useEffect(() => {
    if (activeSurface !== "pinned") return;
    const onPointerDown = (event: PointerEvent) => {
      if ((event.target as Element | null)?.closest?.("[data-pinned-card]")) return;
      setActiveSurface("main");
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [activeSurface, setActiveSurface]);

  useEffect(() => {
    if (!settings.hide_on_blur) return;

    const currentWindow = getCurrentWindow();
    let mounted = true;
    let unlisten: (() => void) | undefined;

    currentWindow.onFocusChanged(({ payload: focused }) => {
      if (!mounted) return;
      if (focused) {
        windowFocusedRef.current = true;
        if (mode === "collapsed") {
          focusCollapsedInput(20);
          focusCollapsedInput(80);
        } else if (mode === "terminal") {
          focusTerminalView(40);
          if ((rendererRef.current?.mode ?? 0) & FOCUS_IN_OUT) {
            invoke("term_input", { id: terminalInputTarget(), data: [27, 91, 73] });
          }
        }
        return;
      }
      if (mode === "terminal" && (activeRenderer()?.mode ?? 0) & FOCUS_IN_OUT) {
        invoke("term_input", { id: terminalInputTarget(), data: [27, 91, 79] });
      }
      // Only a real focused → unfocused transition may hide the panel; see
      // `windowFocusedRef`. A focus report suppressed by the grace window
      // leaves the flag set on purpose: the window genuinely is unfocused at
      // that point, and the next report the compositor sends should still be
      // allowed to dismiss it.
      if (!windowFocusedRef.current) return;
      windowFocusedRef.current = false;
      if (Date.now() < suppressBlurUntil.current) {
        return;
      }
      invoke("hide_window");
    }).then((dispose) => {
      unlisten = dispose;
    });

    return () => {
      mounted = false;
      unlisten?.();
    };
  }, [mode, settings.hide_on_blur]);

  // ---- selection / scroll helpers ---------------------------------------

  const clampCell = (px: number, py: number): CellPoint | null => {
    const renderer = rendererRef.current;
    if (!renderer) return null;
    let col = Math.floor((px - PADDING_X) / renderer.cellWidth);
    let row = Math.floor((py - PADDING_Y) / renderer.cellHeight);
    col = Math.max(0, Math.min(renderer.cols - 1, col));
    row = Math.max(0, Math.min(renderer.rows - 1, row));
    return { col, row };
  };

  const applyScrollbar = (py: number) => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    const now = Date.now();
    if (now - lastScrollAt.current < 24) return;
    lastScrollAt.current = now;
    invoke("term_scroll_to", { id: "main", offset: renderer.offsetFromDragY(py) });
  };

  const reportTerminalMouse = (
    kind: "press" | "release" | "move",
    button: number,
    clientX: number,
    clientY: number,
    modifiers: ModifierEvent,
  ) => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const cell = clampCell(clientX - rect.left, clientY - rect.top);
    if (!cell) return;
    invoke("term_mouse", {
      id: "main",
      kind,
      button,
      column: cell.col,
      row: cell.row,
      modifiers: terminalMouseModifiers(modifiers),
    });
  };

  const onWindowMouseMove = (event: MouseEvent) => {
    const canvas = canvasRef.current;
    const renderer = rendererRef.current;
    if (!canvas || !renderer) return;
    const rect = canvas.getBoundingClientRect();
    const px = event.clientX - rect.left;
    const py = event.clientY - rect.top;
    const drag = dragRef.current;
    if (drag.mode === "mouse") {
      const now = performance.now();
      if (now - lastMouseReportAt.current >= 16) {
        lastMouseReportAt.current = now;
        reportTerminalMouse("move", drag.button, event.clientX, event.clientY, event);
      }
      return;
    }
    if (drag.mode === "scroll") {
      applyScrollbar(py);
      return;
    }
    if (drag.mode === "select") {
      const cell = clampCell(px, py);
      const sel = selectionRef.current;
      if (sel && cell) {
        selectionRef.current = { ...sel, endCol: cell.col, endRow: cell.row };
        render();
      }
    }
  };

  const onWindowMouseUp = (event: MouseEvent) => {
    const drag = dragRef.current;
    if (drag.mode === "mouse") {
      reportTerminalMouse("release", drag.button, event.clientX, event.clientY, event);
    }
    dragRef.current = { mode: "none" };
    window.removeEventListener("mousemove", onWindowMouseMove);
    window.removeEventListener("mouseup", onWindowMouseUp);
  };

  const beginDrag = () => {
    window.addEventListener("mousemove", onWindowMouseMove);
    window.addEventListener("mouseup", onWindowMouseUp);
  };

  const onCanvasMouseDown = (e: React.MouseEvent) => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    // Clicking the main area always reclaims the keyboard from the card.
    setActiveSurface("main");
    terminalTextInputRef.current?.focus({ preventScroll: true });
    const px = e.nativeEvent.offsetX;
    const py = e.nativeEvent.offsetY;

    if (renderer.hitScrollbar(px, py)) {
      dragRef.current = { mode: "scroll" };
      applyScrollbar(py);
      beginDrag();
      e.preventDefault();
      return;
    }

    const cell = renderer.pixelToCell(px, py);
    if (cell && usesMouseReporting(renderer.mode) && !e.shiftKey) {
      selectionRef.current = null;
      dragRef.current = { mode: "mouse", button: e.button };
      reportTerminalMouse("press", e.button, e.clientX, e.clientY, e);
      beginDrag();
      e.preventDefault();
      return;
    }

    const now = Date.now();
    const seq = clickSeq.current;
    const sameCell = cell && seq.col === cell.col && seq.row === cell.row && now - seq.time < 400;
    const count = sameCell ? seq.count + 1 : 1;
    clickSeq.current = {
      count,
      time: now,
      col: cell?.col ?? -1,
      row: cell?.row ?? -1,
    };

    if (!cell) {
      selectionRef.current = null;
      render();
      return;
    }

    if (count === 2) {
      selectionRef.current = renderer.wordSelection(cell);
      render();
      e.preventDefault();
      return;
    }
    if (count >= 3) {
      selectionRef.current = {
        startCol: 0,
        startRow: cell.row,
        endCol: renderer.cols - 1,
        endRow: cell.row,
      };
      render();
      e.preventDefault();
      return;
    }

    selectionRef.current = {
      startCol: cell.col,
      startRow: cell.row,
      endCol: cell.col,
      endRow: cell.row,
    };
    dragRef.current = { mode: "select" };
    render();
    beginDrag();
    e.preventDefault();
  };

  const onCanvasMouseMove = (event: React.MouseEvent) => {
    const renderer = rendererRef.current;
    if (
      !renderer ||
      dragRef.current.mode !== "none" ||
      event.shiftKey ||
      (renderer.mode & MOUSE_MOTION) === 0
    ) {
      return;
    }
    const now = performance.now();
    if (now - lastMouseReportAt.current < 16) return;
    lastMouseReportAt.current = now;
    reportTerminalMouse("move", 3, event.clientX, event.clientY, event);
  };

  /** Read the system clipboard. The webview's own Clipboard API cannot be
   * relied on here — WebKitGTK ships without `navigator.clipboard`, and
   * WKWebView rejects programmatic reads outside its strict gesture policy —
   * so both directions go through the arboard-backed backend commands and
   * fall back to the JS API only where that exists (browser dev builds). */
  const readSystemClipboard = (): Promise<string> =>
    invoke<string>("clipboard_read_text").catch(() => navigator.clipboard.readText());

  /** Write the system clipboard; see `readSystemClipboard` for why this takes
   * the Rust path first. */
  const writeSystemClipboard = (text: string): Promise<void> =>
    invoke("clipboard_write_text", { text })
      .then(() => undefined)
      .catch(() => navigator.clipboard.writeText(text));

  const copySelection = async () => {
    const renderer = rendererRef.current;
    const sel = selectionRef.current;
    if (!renderer || !sel) return;
    const text = renderer.selectionText(sel);
    if (text) {
      try {
        await writeSystemClipboard(text);
      } catch {
        // Clipboard unavailable; selection remains highlighted.
        return;
      }
      // Where the copy shortcut is Ctrl-based it is also the shell's interrupt,
      // so the highlight is dropped after a copy: the next press then reaches
      // the shell instead of copying the same text again. macOS copies with Cmd
      // and keeps its selection.
      if (!IS_MAC) {
        selectionRef.current = null;
        render();
      }
    }
  };

  const sendTerminalText = (text: string, bracketed = false) => {
    if (!text || !ptyReady.current) return;
    const payload = bracketed ? `\x1b[200~${text}\x1b[201~` : text;
    void invoke("term_input", {
      id: terminalInputTarget(),
      data: Array.from(new TextEncoder().encode(payload)),
    });
  };

  const flushTerminalTextInput = (bracketed = false) => {
    const input = terminalTextInputRef.current;
    if (!input || terminalComposing.current || !input.value) return;
    const text = input.value;
    input.value = "";
    sendTerminalText(text, bracketed);
  };

  const onTerminalTextInput = (event: React.FormEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as InputEvent;
    if (nativeEvent.isComposing || terminalComposing.current) return;
    const bracketedPaste =
      nativeEvent.inputType === "insertFromPaste" &&
      Boolean((activeRenderer()?.mode ?? 0) & BRACKETED_PASTE);
    flushTerminalTextInput(bracketedPaste);
  };

  const pasteClipboard = async () => {
    const renderer = activeRenderer();
    if (!renderer) return;
    let text = "";
    try {
      text = await readSystemClipboard();
    } catch {
      return;
    }
    if (!text) return;
    sendTerminalText(text, (renderer.mode & BRACKETED_PASTE) !== 0);
  };

  // Hand the broker-owned PTY to the system terminal without restarting it.
  const openInTerminal = async () => {
    if (externalTerminalOpening.current || !ptyReady.current) return;
    externalTerminalOpening.current = true;
    setTerminalFeedback(null);
    try {
      const outcome = await invoke<ExternalTerminalOutcome>("open_in_default_terminal", {
        id: "main",
      });
      if (outcome.session_handed_off) {
        restoringMode.current = "collapsed";
        closeTerminalSession();
        setQuery("");
        setTerminalMounted(false);
        setMode("collapsed");
        await invoke("show_input");
      }
      await invoke("hide_window");
    } catch {
      showTerminalFeedback("launcher.error.externalTerminal");
      focusTerminalView();
    } finally {
      externalTerminalOpening.current = false;
    }
  };

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      // The recorder listens in the capture phase; this is only a safety net.
      if (recordingAction) return;

      if (mode === "terminal") {
        // While an IME owns the keyboard, even Enter and configured shortcuts
        // can be part of candidate selection. WebKit may report the confirming
        // key with keyCode 229 after clearing isComposing.
        if (isTerminalCompositionKey(event)) return;

        // App shortcuts first, everything else is forwarded to the shell.
        if (matchesShortcut(event, shortcuts.new_command)) {
          event.preventDefault();
          // On macOS the panel can update its first responder once more when Cmd
          // is released. Reassert the input after the complete shortcut is up.
          const onShortcutRelease = (release: KeyboardEvent) => {
            if (release.metaKey || release.ctrlKey || release.altKey || release.shiftKey) return;
            window.removeEventListener("keyup", onShortcutRelease);
            focusCollapsedInput();
          };
          window.addEventListener("keyup", onShortcutRelease);
          window.setTimeout(() => {
            window.removeEventListener("keyup", onShortcutRelease);
          }, 1500);
          returnToInputMode();
          return;
        }
        if (matchesShortcut(event, shortcuts.open_external_terminal)) {
          event.preventDefault();
          void openInTerminal();
          return;
        }
        // Pin / unpin / move the floating card. Only meaningful while a
        // terminal session view is open (the requirement's precondition).
        if (matchesShortcut(event, shortcuts.pin_terminal)) {
          event.preventDefault();
          void togglePinnedTerminal();
          return;
        }
        // While the card owns the keyboard, Escape hands it back to the main
        // surface instead of reaching the pinned session's shell.
        if (event.key === "Escape" && activeSurfaceRef.current === "pinned") {
          event.preventDefault();
          setActiveSurface("main");
          return;
        }
        // Copy only claims the combination when there is something to copy, so
        // a Ctrl+C binding still interrupts the foreground process otherwise.
        if (selectionRef.current && matchesShortcut(event, shortcuts.copy_selection)) {
          event.preventDefault();
          copySelection();
          return;
        }
        if (matchesShortcut(event, shortcuts.paste)) {
          event.preventDefault();
          pasteClipboard();
          return;
        }
        // macOS keeps swallowing every other Cmd combo: those are window-level
        // shortcuts, never shell input. Ctrl combos on Windows and Linux are
        // the shell's (Ctrl+C, Ctrl+D, Ctrl+L, ...) and fall through.
        if (IS_MAC && event.metaKey && !event.altKey) {
          return;
        }
        if (event.shiftKey && (event.key === "PageUp" || event.key === "PageDown")) {
          event.preventDefault();
          const lines = dimsRef.current.rows;
          invoke("term_scroll", {
            id: terminalInputTarget(),
            delta: event.key === "PageUp" ? lines : -lines,
          });
          return;
        }
        // The shell receives keystrokes only while the terminal's own proxy
        // input holds the keyboard AND the PTY is ready. Anything else is
        // dropped here rather than forwarded: presses that arrive during a
        // launcher→terminal transition, or while focus sits on a header
        // button, would otherwise land at the prompt as phantom commands —
        // exactly the "command not found" reports for text the user typed in
        // the launcher and never meant for the shell.
        if (
          document.activeElement !== terminalTextInputRef.current ||
          !ptyReady.current
        ) {
          return;
        }
        if (
          event.target === terminalTextInputRef.current &&
          shouldUseTerminalTextInput(event)
        ) {
          return;
        }
        const renderer = activeRenderer();
        const encoded = renderer ? encodeKey(event, renderer.mode) : null;
        if (encoded) {
          event.preventDefault();
          invoke("term_input", { id: terminalInputTarget(), data: Array.from(encoded) });
        }
        return;
      }

      if (mode === "settings") {
        // Cmd+W (macOS) / Ctrl+W (other platforms) dismisses the panel — a
        // convention every overlay surface in floter follows.
        if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
          event.preventDefault();
          event.stopPropagation();
          closeSettings();
          return;
        }
        if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
          event.preventDefault();
          closeSettings();
        }
        return;
      }

      if (mode === "plugin") {
        // While the plugin page (a sandboxed iframe) owns focus it claims its
        // own keys; presses that reach here found the host still holding the
        // keyboard and must not fall through to launcher handling. Esc and
        // Cmd/Ctrl+W close, matching what the page itself does with them.
        if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w") {
          event.preventDefault();
          event.stopPropagation();
          closePluginPage();
          return;
        }
        if (event.key === "Escape") {
          event.preventDefault();
          closePluginPage();
        }
        return;
      }

      // Collapsed-mode input handling.
      if (matchesShortcut(event, shortcuts.open_settings)) {
        event.preventDefault();
        openSettings();
        return;
      }

      if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
        event.preventDefault();
        invoke("hide_window");
        return;
      }

      const inputFocused = document.activeElement === inputRef.current;
      const resultNumber = inputFocused ? null : matchesResultShortcut(event, shortcuts.select_result);
      if (resultNumber !== null) {
        const resultIndex = resultShortcutSlots.indexOf(resultNumber);
        if (resultIndex >= 0) {
          event.preventDefault();
          runLauncherItem(launcherResults[resultIndex]);
        }
        return;
      }
      if (inputFocused) return;

      // Tab deliberately moves from the query into the session and settings
      // controls. Once focus is inside the card, leave ordinary button
      // keyboard handling to the browser instead of treating it as accidental.
      const activeElement = document.activeElement;
      if (activeElement && collapsedCardRef.current?.contains(activeElement)) return;

      // Below here the input does not have the keyboard, which in this mode is
      // only intentional while focus is on one of the card's own controls.
      // Anything outside the card takes the field back and then does what it
      // would have done had the field never lost it.
      focusCollapsedInput();

      if (event.metaKey || event.ctrlKey || event.altKey) return;

      if (event.key === "Backspace") {
        event.preventDefault();
        setQuery((current) => current.slice(0, -1));
        return;
      }

      // A printable key is typed into the query by hand: the field was not
      // focused when the press happened, so nothing else will insert it.
      if (event.key.length === 1) {
        event.preventDefault();
        setQuery((current) => `${current}${event.key}`);
        setHistoryIndex(-1);
        return;
      }

      // Enter, the arrows, Tab — the keys the field's own handler owns.
      handleLauncherKey(event);
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [launcherResults, mode, query, recordingAction, shortcuts]);

  const startDrag = (event: React.MouseEvent) => {
    if ((event.target as HTMLElement).closest("button, input, select, textarea, a, summary, [role='dialog'], [data-no-drag]")) {
      return;
    }
    event.preventDefault();
    if (!IS_WINDOWS) {
      invoke("start_drag");
      return;
    }
    // `start_dragging()` on Windows opens a modal move loop the webview spends
    // unfocused, which is indistinguishable from the user leaving - and the
    // hide-on-blur listener would put the panel away out from under the drag.
    // The command returns when the loop ends, so the grace period is armed once
    // for the blur on the way in and once for the focus handed back on the way
    // out.
    suppressBlurUntil.current = Date.now() + DRAG_BLUR_GRACE;
    void invoke("start_drag")
      .catch(() => undefined)
      .finally(() => {
        suppressBlurUntil.current = Date.now() + DRAG_BLUR_GRACE;
      });
  };

  const rememberCommand = (command: string) => {
    setHistory((current) => [command, ...current.filter((entry) => entry !== command)].slice(0, 20));
    setHistoryIndex(-1);
    draftBeforeHistory.current = "";
  };

  const returnToInputMode = async () => {
    // Resizing a native window can temporarily move keyboard focus back to the
    // webview itself. Mark this as an explicit restoration so the mode effect
    // does not race a second `show_input` call, then focus only after the native
    // resize/reveal has completed.
    restoringMode.current = "collapsed";
    suppressBlurUntil.current = Date.now() + 400;
    setTerminalFeedback(null);
    setLauncherFeedback(null);
    closeTerminalSession();
    setQuery("");
    setTerminalMounted(false);
    setMode("collapsed");
    try {
      await invoke("show_input");
      syncLauncherHeight();
    } catch {
      // The DOM still transitions back to a usable launcher even if the native
      // resize failed; keep the keyboard recovery below independent of IPC.
    }
    focusCollapsedInput();
    focusCollapsedInput(80);
    if (IS_WINDOWS) focusCollapsedInput(TERMINAL_FOCUS_RETRY);
  };

  const openSettings = (page?: SettingsPage) => {
    suppressBlurUntil.current = Date.now() + 400;
    const nextPage = page ?? settingsPage;
    setSettingsPage(nextPage);
    if (nextPage === "sessions") void refreshTerminalSessions();
    setMode("settings");
  };

  const killTerminalSession = async (session: BrokerSessionInfo) => {
    if (sessionActionId) return;
    // First click arms the inline confirm instead of blocking with
    // window.confirm (same rhythm as the extensions discard bar).
    if (killConfirmId !== session.sessionId) {
      setKillConfirmId(session.sessionId);
      return;
    }
    setKillConfirmId(null);
    setSessionActionId(session.sessionId);
    try {
      await invoke("term_kill_session", { sessionId: session.sessionId });
      await refreshTerminalSessions();
    } catch {
      await refreshTerminalSessions();
    } finally {
      setSessionActionId(null);
    }
  };

  useEffect(() => {
    if (!killConfirmId) return;
    const timer = window.setTimeout(() => setKillConfirmId(null), 3000);
    return () => window.clearTimeout(timer);
  }, [killConfirmId]);

  useEffect(() => {
    check().then((update) => {
      if (update?.available) {
        setUpdateInfo({ version: update.version });
      }
    }).catch(() => undefined);
  }, []);

  const downloadAndInstallUpdate = async () => {
    if (updateDownloading) return;
    setUpdateDownloading(true);
    setUpdateFailed(false);
    setUpdateProgress(null);
    try {
      const update = await check();
      if (!update?.available) {
        setUpdateDownloading(false);
        return;
      }
      // The plugin reports each chunk's size rather than a running total, so
      // the cumulative progress is summed here; `Started` carries the total.
      let downloaded = 0;
      let total = 0;
      await update.download((event) => {
        if (event.event === "Started") {
          total = event.data.contentLength ?? 0;
          downloaded = 0;
          setUpdateProgress({ downloaded: 0, total });
        } else if (event.event === "Progress") {
          downloaded += event.data.chunkLength;
          setUpdateProgress({ downloaded, total });
        }
      });
      // Download finished; the panel now shows "Installing…" until the
      // relaunch below takes over.
      setUpdateProgress(null);
      await update.install();
      await relaunch();
    } catch {
      setUpdateFailed(true);
      setUpdateDownloading(false);
      setUpdateProgress(null);
    }
  };

  const closeSettings = () => {
    // The window is already anchored; letting the collapsed layout effect restore
    // the height keeps a pending query's result list intact.
    restoringMode.current = "collapsed";
    setMode("collapsed");
  };

  /** Dismiss the plugin page; the mode effect sends the window back onto
   * the remembered surface through its normal restore path (`show_input` /
   * `show_terminal`). */
  const closePluginPage = () => {
    suppressBlurUntil.current = Date.now() + 400;
    setPluginPageId(null);
    setMode(pluginReturnMode.current);
  };

  /** Open a plugin page over whatever surface is showing, remembering it for
   * the return trip. One path for every trigger — hotkey, `floter clip`, the
   * launcher entry, a cold-start request. Deliberately does NOT arm
   * `restoringMode`: sizing belongs to the backend here — the mode effect
   * calls `show_plugin_page`, which applies the same saved geometry terminal
   * mode uses. */
  const openPluginPage = (pluginId: string) => {
    suppressBlurUntil.current = Date.now() + 400;
    pluginReturnMode.current =
      modeRef.current === "terminal" ? "terminal" : "collapsed";
    setPluginPageId(pluginId);
    setMode("plugin");
  };

  const quitApp = async () => {
    if (appQuitting.current) return;
    appQuitting.current = true;
    // Slider changes are debounced. Flush the current full snapshot before the
    // component unmounts, otherwise quitting inside that debounce window drops
    // the user's final value when the cleanup below cancels the timer.
    if (settingsSaveTimer.current !== null) {
      window.clearTimeout(settingsSaveTimer.current);
      settingsSaveTimer.current = null;
    }
    if (settingsHydration.hasFailed()) {
      try {
        await invoke("quit_app");
      } catch {
        appQuitting.current = false;
      }
      return;
    }
    try {
      await persistSettings();
      await invoke("quit_app");
    } catch {
      // The save state above keeps the settings window open with a retry action.
      appQuitting.current = false;
    }
  };

  useEffect(() => () => {
    if (settingsSaveTimer.current !== null) {
      window.clearTimeout(settingsSaveTimer.current);
    }
    if (launcherFeedbackTimer.current !== null) {
      window.clearTimeout(launcherFeedbackTimer.current);
    }
    if (terminalFeedbackTimer.current !== null) {
      window.clearTimeout(terminalFeedbackTimer.current);
    }
  }, []);

  const changeOpacity = (field: "main_opacity" | "terminal_opacity", next: number) => {
    const value = normalizeOpacity(next);
    if (value === settingsRef.current[field]) return;
    const updated: AppSettings = { ...settingsRef.current, [field]: value };
    settingsHydration.markChanged(field);
    settingsRef.current = updated;
    setSettings(updated);
    if (settingsSaveTimer.current !== null) {
      window.clearTimeout(settingsSaveTimer.current);
    }
    settingsSaveTimer.current = window.setTimeout(() => {
      settingsSaveTimer.current = null;
      persistSettings().catch(() => undefined);
    }, 180);
  };

  const changeFontSize = (next: number) => {
    const fontSize = normalizeFontSize(next);
    if (fontSize === settingsRef.current.font_size) return;
    const updated = { ...settingsRef.current, font_size: fontSize };
    settingsHydration.markChanged("font_size");
    settingsRef.current = updated;
    setSettings(updated);
    if (settingsSaveTimer.current !== null) {
      window.clearTimeout(settingsSaveTimer.current);
    }
    settingsSaveTimer.current = window.setTimeout(() => {
      settingsSaveTimer.current = null;
      persistSettings().catch(() => undefined);
    }, 180);
  };

  const changeGeneralSetting = <K extends keyof AppSettings>(field: K, value: AppSettings[K]) => {
    if (settingsRef.current[field] === value) return;
    const updated = { ...settingsRef.current, [field]: value };
    settingsHydration.markChanged(field);
    settingsRef.current = updated;
    setSettings(updated);
    suppressBlurUntil.current = Date.now() + 400;
    persistSettings().catch(() => undefined);
  };

  const toggleCommandsInSearch = () => {
    changeGeneralSetting("show_commands_in_search", !settingsRef.current.show_commands_in_search);
  };

  const changeTheme = (theme: string) => {
    if (theme === settings.theme) return;
    changeGeneralSetting("theme", theme);
  };

  const changeLanguage = (next: Language) => {
    if (next === language) return;
    changeGeneralSetting("language", next);
  };

  const changeLaunchAtStartup = async (enabled: boolean) => {
    if (autostartUpdating || enabled === settingsRef.current.launch_at_startup) return;
    const previous = settingsRef.current.launch_at_startup;
    const updated: AppSettings = { ...settingsRef.current, launch_at_startup: enabled };
    settingsHydration.markChanged("launch_at_startup");
    settingsRef.current = updated;
    setSettings(updated);
    setAutostartUpdating(true);
    suppressBlurUntil.current = Date.now() + 400;
    try {
      await invoke("set_launch_at_startup", { enabled });
      const latest = { ...settingsRef.current, launch_at_startup: enabled };
      settingsRef.current = latest;
      await persistSettings();
    } catch {
      await invoke("set_launch_at_startup", { enabled: previous }).catch(() => undefined);
      setSettings((current) => {
        const rolledBack = current.launch_at_startup === enabled
          ? { ...current, launch_at_startup: previous }
          : current;
        settingsRef.current = rolledBack;
        return rolledBack;
      });
    } finally {
      setAutostartUpdating(false);
    }
  };

  const toggleRecording = (action: string) => {
    setRejectedAction(null);
    setRecordingAction((current) => {
      if (current === action) {
        invoke("resume_shortcuts").catch(() => undefined);
        return null;
      }
      invoke("suspend_shortcuts").catch(() => undefined);
      return action;
    });
  };

  const cancelRecording = () => {
    setRecordingAction(null);
    invoke("resume_shortcuts").catch(() => undefined);
  };

  const restoreDefaultShortcuts = async () => {
    if (recordingAction) {
      await invoke("resume_shortcuts").catch(() => undefined);
    }
    try {
      const shortcuts = await invoke<ShortcutMap>("reset_shortcuts");
      settingsHydration.markChanged("hotkey");
      settingsHydration.markChanged("shortcuts");
      setSettings((current) => {
        const updated = {
          ...current,
          hotkey: shortcuts.toggle_window,
          shortcuts,
        };
        settingsRef.current = updated;
        return updated;
      });
      setRecordingAction(null);
      setRejectedAction(null);
    } catch {
      // Keep the current shortcuts if the system rejects the default toggle.
    }
  };

  // Clear/disable the clipboard panel hotkey from the shortcuts settings
  // page. Optimistic like captureShortcut: persist "" (the backend treats an
  // empty string as unregister-and-disable) and roll back on failure.
  const clearClipboardHotkey = () => {
    const previousHotkey = settingsRef.current.clipboard_history_hotkey;
    if (!previousHotkey) return;
    settingsHydration.markChanged("clipboard_history_hotkey");
    setSettings((current) => {
      const updated = { ...current, clipboard_history_hotkey: "" };
      settingsRef.current = updated;
      return updated;
    });
    setRejectedAction(null);
    invoke("update_clipboard_hotkey", { hotkey: "" }).catch(() => {
      setSettings((current) => {
        const rolledBack = { ...current, clipboard_history_hotkey: previousHotkey };
        settingsRef.current = rolledBack;
        return rolledBack;
      });
      setRejectedAction(CLIPBOARD_HOTKEY_ACTION);
    });
  };

  // Store the new binding optimistically; the backend is the authority on
  // whether a system-wide combination can actually be taken.
  const captureShortcut = (action: string, next: string) => {
    setRecordingAction(null);
    setRejectedAction(null);
    if (action === "select_result") {
      const normalized = normalizeResultShortcut(next);
      if (!normalized) {
        setRejectedAction(action);
        invoke("resume_shortcuts").catch(() => undefined);
        return;
      }
      next = normalized;
    }
    if (action === CLIPBOARD_HOTKEY_ACTION) {
      const previousHotkey = settingsRef.current.clipboard_history_hotkey;
      const conflict = SHORTCUT_ACTIONS.some(
        (candidate) => shortcuts[candidate].toLowerCase() === next.toLowerCase(),
      );
      if (conflict) {
        setRejectedAction(action);
        invoke("resume_shortcuts").catch(() => undefined);
        return;
      }
      if (next === previousHotkey) {
        invoke("resume_shortcuts").catch(() => undefined);
        return;
      }
      settingsHydration.markChanged("clipboard_history_hotkey");
      setSettings((current) => {
        const updated = { ...current, clipboard_history_hotkey: next };
        settingsRef.current = updated;
        return updated;
      });
      suppressBlurUntil.current = Date.now() + 400;
      invoke("update_clipboard_hotkey", { hotkey: next })
        .then(() => {
          invoke("resume_shortcuts").catch(() => undefined);
        })
        .catch(() => {
          setSettings((current) => {
            const rolledBack = { ...current, clipboard_history_hotkey: previousHotkey };
            settingsRef.current = rolledBack;
            return rolledBack;
          });
          setRejectedAction(action);
          invoke("resume_shortcuts").catch(() => undefined);
        });
      return;
    }
    const previous = shortcuts[action as ShortcutAction];
    const conflict = SHORTCUT_ACTIONS.some(
      (candidate) => candidate !== action && shortcuts[candidate].toLowerCase() === next.toLowerCase(),
    );
    if (conflict) {
      setRejectedAction(action);
      invoke("resume_shortcuts").catch(() => undefined);
      return;
    }
    if (next === previous) {
      invoke("resume_shortcuts").catch(() => undefined);
      return;
    }

    settingsHydration.markChanged("shortcuts");
    if (action === "toggle_window") settingsHydration.markChanged("hotkey");
    setSettings((current) => {
      const updated = {
        ...current,
        ...(action === "toggle_window" ? { hotkey: next } : {}),
        shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: next },
      };
      settingsRef.current = updated;
      return updated;
    });
    suppressBlurUntil.current = Date.now() + 400;
    invoke("update_shortcut", { action, shortcut: next }).then(() => {
      invoke("resume_shortcuts").catch(() => undefined);
    }).catch(() => {
      setSettings((current) => {
        const rolledBack = {
          ...current,
          ...(action === "toggle_window" ? { hotkey: previous } : {}),
          shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: previous },
        };
        settingsRef.current = rolledBack;
        return rolledBack;
      });
      setRejectedAction(action);
      invoke("resume_shortcuts").catch(() => undefined);
    });
  };

  const runCommand = async (
    execution: ExecutionPlan | null = null,
    commandLine = query.trim(),
  ) => {
    const command = commandLine.trim();
    if (!command || terminalOpening.current) return;

    terminalOpening.current = true;
    setLauncherFeedback(null);
    setTerminalFeedback(null);
    setTerminalMounted(true);
    setMode("terminal");
    try {
      await ensureTerminalSession(execution ? null : command, execution);
      rememberCommand(command);
      setQuery("");
      if (execution?.mode === "external") {
        await openInTerminal();
      } else {
        focusTerminalView();
      }
    } catch {
      showLauncherFeedback("launcher.error.command");
      setTerminalMounted(false);
      setMode("collapsed");
      focusCollapsedInput(50);
    } finally {
      terminalOpening.current = false;
    }
  };

  const resumeTerminalSession = async (session: BrokerSessionInfo) => {
    if (terminalOpening.current) return;
    terminalOpening.current = true;
    setLauncherFeedback(null);
    setTerminalFeedback(null);
    // Resuming the very session the card is showing would attach a second
    // client to one PTY; hand it back to the main view instead.
    const pinned = pinStateRef.current;
    if (pinned.status === "pinned" && pinned.session.brokerSessionId === session.sessionId) {
      try {
        await invoke("term_close", { id: PINNED_SESSION_ID });
      } catch {
        // The card view may already be gone; either way the resume proceeds.
      }
      dispatchPinEvent({ type: "unpin" });
      setMainPinnedAway(false);
    }
    setTerminalMounted(true);
    setMode("terminal");
    try {
      if (sessionClosePromise.current) await sessionClosePromise.current;
      const { cols, rows } = dimsRef.current;
      const generation = ++nextTerminalGeneration.current;
      terminalGeneration.current = generation;
      const brokerSessionId = await invoke<string>("term_attach_existing", {
        request: {
          id: "main",
          generation,
          brokerSessionId: session.sessionId,
          theme: resolvedTheme,
          cols,
          rows,
        },
      });
      if (terminalGeneration.current === generation) {
        ptyReady.current = true;
        mainBrokerSessionIdRef.current = brokerSessionId;
        setMainPinnedAway(false);
      }
      setQuery("");
      focusTerminalView();
    } catch {
      showLauncherFeedback("launcher.error.session");
      terminalGeneration.current = null;
      ptyReady.current = false;
      setTerminalMounted(false);
      setMode("collapsed");
      refreshTerminalSessions();
      focusCollapsedInput(50);
    } finally {
      terminalOpening.current = false;
    }
  };

  // ---- pin card -----------------------------------------------------------

  /** Look up a human-readable session name for the card header. */
  const lookupSessionLabel = async (brokerSessionId: string): Promise<string | null> => {
    try {
      const sessions = await invoke<BrokerSessionInfo[]>("term_list_sessions");
      return sessions.find((entry) => entry.sessionId === brokerSessionId)?.name || null;
    } catch {
      return null;
    }
  };

  /** Attach `brokerSessionId` to the card's frontend id with a fresh view
   * generation; resolves to that generation. */
  const attachAsPinned = async (brokerSessionId: string): Promise<number> => {
    const generation = ++nextTerminalGeneration.current;
    await invoke("term_attach_existing", {
      request: {
        id: PINNED_SESSION_ID,
        generation,
        brokerSessionId,
        theme: resolvedTheme,
        cols: dimsRef.current.cols,
        rows: dimsRef.current.rows,
      },
    });
    return generation;
  };

  /** Release the main view's session without killing its PTY, so the card can
   * take it over. */
  const detachMainView = async () => {
    terminalGeneration.current = null;
    ptyReady.current = false;
    mainBrokerSessionIdRef.current = null;
    resetTerminalFrontendState();
    setActiveSurface("main");
  };

  /** Pin (or, when something is already pinned and a new main session is
   * running, replace) — the current main session moves into the card. */
  const pinCurrentMain = async () => {
    const brokerSessionId = mainBrokerSessionIdRef.current;
    const generation = terminalGeneration.current;
    if (!ptyReady.current || !brokerSessionId || generation === null) return;
    pinBusy.current = true;
    try {
      // Detach first, then attach the same PTY under the card's id. If the
      // attach fails the session stays alive in the daemon, resumable from the
      // session list.
      await invoke("term_detach_view", { id: "main", generation });
      await detachMainView();
      setMainPinnedAway(true);
      const pinnedGeneration = await attachAsPinned(brokerSessionId);
      dispatchPinEvent({ type: "pin", brokerSessionId, generation: pinnedGeneration });
      void lookupSessionLabel(brokerSessionId).then((label) => {
        if (label) dispatchPinEvent({ type: "label", label });
      });
    } catch {
      showTerminalFeedback("launcher.error.session");
      refreshTerminalSessions();
    } finally {
      pinBusy.current = false;
    }
  };

  /** Reattach a broker session into the main terminal view. Only valid while
   * the main slot is free (`ptyReady` false). */
  const resumeIntoMainView = async (brokerSessionId: string) => {
    const generation = ++nextTerminalGeneration.current;
    terminalGeneration.current = generation;
    try {
      const attachedId = await invoke<string>("term_attach_existing", {
        request: {
          id: "main",
          generation,
          brokerSessionId,
          theme: resolvedTheme,
          cols: dimsRef.current.cols,
          rows: dimsRef.current.rows,
        },
      });
      ptyReady.current = true;
      mainBrokerSessionIdRef.current = attachedId;
      setMainPinnedAway(false);
      focusTerminalView();
    } catch {
      terminalGeneration.current = null;
      showTerminalFeedback("launcher.error.session");
      refreshTerminalSessions();
    }
  };

  /** Dismiss the card; the pinned session returns to the normal flow — back
   * into the main view when that is free, otherwise left detached in the
   * session list. */
  const unpinPinnedSession = async () => {
    const pinned = pinStateRef.current;
    if (pinned.status !== "pinned" || pinBusy.current) return;
    pinBusy.current = true;
    try {
      // Attached views close by detaching only — the PTY survives.
      await invoke("term_close", { id: PINNED_SESSION_ID });
    } catch {
      // Already gone; still drop the card state below.
    }
    const { brokerSessionId } = pinned.session;
    dispatchPinEvent({ type: "unpin" });
    setActiveSurface("main");
    if (!ptyReady.current && mode === "terminal") {
      await resumeIntoMainView(brokerSessionId);
    }
    pinBusy.current = false;
  };

  /** Shortcut entry point: pin / unpin / replace, depending on what is live. */
  const togglePinnedTerminal = async () => {
    if (pinBusy.current) return;
    const pinned = pinStateRef.current;
    if (pinned.status === "pinned") {
      await unpinPinnedSession();
      if (ptyReady.current) {
        // A newer session runs in the main area: move the card to it. The old
        // session was released into the normal list/view flow above.
        await pinCurrentMain();
      }
      return;
    }
    await pinCurrentMain();
  };

  /** The pinned PTY exited on its own: remove the card, nothing to restore. */
  const handlePinnedSessionExit = useCallback(() => {
    const pinned = pinStateRef.current;
    if (pinned.status !== "pinned") return;
    dispatchPinEvent({ type: "sessionClosed", generation: pinned.session.generation });
    setActiveSurface("main");
    setMainPinnedAway(false);
  }, [dispatchPinEvent, setActiveSurface]);

  const launchApplication = async (app: LocalApplication) => {
    setLauncherFeedback(null);
    try {
      await invoke("open_application", { path: app.path });
      setQuery("");
      setHistoryIndex(-1);
      invoke("hide_window");
    } catch {
      showLauncherFeedback("launcher.error.application");
      // Keep the launcher open so the user can revise the query.
    }
  };

  /**
   * Hand the query to the system and close, unless the system refused it.
   *
   * A path that does not exist is the common refusal, and closing on one would
   * throw away the path that has just been typed — so the launcher stays up for
   * it to be corrected, exactly as a failed application launch does.
   */
  const openWithSystem = async (command: "open_url" | "open_path", args: Record<string, string>) => {
    setLauncherFeedback(null);
    try {
      await invoke(command, args);
    } catch {
      showLauncherFeedback(command === "open_url" ? "launcher.error.url" : "launcher.error.path");
      return;
    }
    setQuery("");
    setHistoryIndex(-1);
    invoke("hide_window");
  };

  const executeActionBar = (action: ActionBar) => {
    if (action.type === "url") {
      void openWithSystem("open_url", { url: action.value });
      return;
    }
    if (action.type === "path") {
      void openWithSystem("open_path", { path: action.value });
      return;
    }
    void runCommand();
  };

  const runSystemAction = async (item: Extract<LauncherItem, { type: "system" }>) => {
    // The clipboard page is a plain view flip — no confirmation, no window
    // hiding, just the same open path the global hotkey and `floter clip`
    // take.
    if (item.action === "clipboard") {
      setQuery("");
      setHistoryIndex(-1);
      openPluginPage(CLIPBOARD_PLUGIN_ID);
      return;
    }

    if (systemPowerOpening.current) return;

    const confirmationKey = item.action === "restart"
      ? "system.restartConfirm"
      : "system.shutdownConfirm";
    if (!window.confirm(t(confirmationKey))) {
      focusCollapsedInput();
      return;
    }

    systemPowerOpening.current = true;
    setLauncherFeedback(null);
    try {
      // The launcher is an always-on-top panel. Move it out of the way before
      // macOS presents its own confirmation, or that dialog can appear behind
      // the panel. Linux and Windows execute immediately after this point.
      await invoke("hide_window");
      await invoke("system_power", { action: item.action });
      setQuery("");
      setHistoryIndex(-1);
    } catch {
      // A missing system utility or rejected spawn must not look like success.
      // Restore the launcher with the original query intact so it can be retried.
      setMode("collapsed");
      await invoke("show_input").catch(() => undefined);
      showLauncherFeedback(
        item.action === "restart"
          ? "launcher.error.restart"
          : "launcher.error.shutdown",
      );
      focusCollapsedInput(50);
    } finally {
      systemPowerOpening.current = false;
    }
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

  const runLauncherItem = (item: LauncherItem | undefined) => {
    if (!item) return;
    if (item.type === "app") {
      recordLaunch(item.app.path);
      void launchApplication(item.app);
      return;
    }
    if (item.type === "system") {
      void runSystemAction(item);
      return;
    }
    if (item.execution && item.sourceName) {
      void runCommand(item.execution, item.commandLine);
      return;
    }
    // A catalog row with an unavailable runtime remains visible for discovery,
    // but is not silently reinterpreted by the user's shell.
    showLauncherFeedback("extensions.runtimeUnavailable");
  };

  /**
   * Everything the launcher does with a key press.
   *
   * Reached two ways: from the input's own handler, and from the window
   * listener when the input has somehow lost the keyboard — a stray click, a
   * reveal that landed before the element was there. The second path is why
   * this takes a plain `KeyboardEvent` rather than React's wrapper.
   */
  const handleLauncherKey = (event: KeyboardEvent) => {
    // CJK IME: while composing (user picking candidates), all keys go to the
    // IME. WebKit clears `isComposing` too early for the Enter that confirms a
    // candidate, but keeps the conventional IME keyCode (229) on that event.
    // Checking the event itself avoids leaving a flag behind that swallows the
    // user's next deliberate Enter after composition has already finished.
    if (isComposing.current || event.isComposing || event.keyCode === 229) return;

    // Holding the same modifier as the numbered-result shortcut highlights the
    // command row. It makes Cmd/Ctrl+Enter discoverable without giving the row a
    // competing number.
    if (
      actionBar &&
      ["Meta", "Control", "Alt", "Shift"].includes(event.key) &&
      matchesShortcutModifiers(event, shortcuts.select_result)
    ) {
      setSelectedActionBar(true);
      return;
    }
    // Numbered results only: the action bar has no number, so `Cmd/Ctrl+1` can
    // never run a command by mistake.
    const resultNumber = matchesResultShortcut(event, shortcuts.select_result);
    if (resultNumber !== null) {
      const resultIndex = resultShortcutSlots.indexOf(resultNumber);
      if (resultIndex >= 0) {
        event.preventDefault();
        runLauncherItem(launcherResults[resultIndex]);
      }
      return;
    }

    if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
      event.preventDefault();
      invoke("hide_window");
      return;
    }

    if (
      event.key === "Enter" &&
      actionBar &&
      matchesShortcutModifiers(event, shortcuts.select_result)
    ) {
      event.preventDefault();
      executeActionBar(actionBar);
      return;
    }

    if (event.key === "Enter") {
      event.preventDefault();
      // The list can empty out between a keystroke and the effect that moves the
      // selection off it, so an empty one falls back to the action bar rather
      // than running nothing at all.
      if (actionBar && (selectedActionBar || !launcherResults.length)) {
        executeActionBar(actionBar);
      } else {
        runLauncherItem(launcherResults[selectedResultIndex]);
      }
      return;
    }

    if (event.key === "Tab" && !event.shiftKey && !selectedActionBar) {
      const selected = launcherResults[selectedResultIndex];
      if (
        selected?.type === "command" &&
        selected.execution &&
        selected.commandLine !== query
      ) {
        event.preventDefault();
        setQuery(selected.commandLine);
        setHistoryIndex(-1);
        return;
      }
    }

    if (event.key === "Tab" && event.shiftKey) {
      const controls = collapsedCardRef.current?.querySelectorAll<HTMLButtonElement>(
        ".collapsed-card__input-row button:not(:disabled)",
      );
      const lastControl = controls?.[controls.length - 1];
      if (lastControl) {
        event.preventDefault();
        lastControl.focus();
      }
      return;
    }

    // The results and the action bar are navigated as one loop that wraps at
    // both ends. With no query there is neither, and the arrows fall through to
    // the shell history below.
    if (actionBar || launcherResults.length) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        const selection = nextLauncherSelection(
          runnableResultFlags,
          selectedResultIndex,
          selectedActionBar,
          Boolean(actionBar),
          1,
        );
        setSelectedActionBar(selection.actionBar);
        setSelectedResultIndex(selection.resultIndex);
        return;
      }

      if (event.key === "ArrowUp") {
        event.preventDefault();
        const selection = nextLauncherSelection(
          runnableResultFlags,
          selectedResultIndex,
          selectedActionBar,
          Boolean(actionBar),
          -1,
        );
        setSelectedActionBar(selection.actionBar);
        setSelectedResultIndex(selection.resultIndex);
        return;
      }

    }

    if (event.key === "ArrowUp") {
      event.preventDefault();
      if (!history.length) return;
      if (historyIndex === -1) draftBeforeHistory.current = query;
      const nextIndex = Math.min(historyIndex + 1, history.length - 1);
      setHistoryIndex(nextIndex);
      setQuery(history[nextIndex]);
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      if (historyIndex === -1) return;
      const nextIndex = historyIndex - 1;
      if (nextIndex < 0) {
        setHistoryIndex(-1);
        setQuery(draftBeforeHistory.current);
      } else {
        setHistoryIndex(nextIndex);
        setQuery(history[nextIndex]);
      }
    }
  };

  const onInputKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) =>
    handleLauncherKey(event.nativeEvent);

  // The card is mounted in every mode so its frame stream and renderer stay
  // alive across launcher ↔ terminal window transitions (nothing is missed
  // while the window is small); it is only VISIBLE in terminal mode.
  const pinnedCardElement = pinState.status === "pinned" ? (
    <PinnedTerminalCard
      session={pinState.session}
      fontFamily={terminalFontFamily(settings.font_family)}
      fontSize={normalizeFontSize(settings.font_size)}
      theme={resolvedTheme}
      geometry={cardGeometry}
      onGeometryChange={updateCardGeometry}
      focused={activeSurface === "pinned"}
      hidden={mode !== "terminal"}
      onClose={() => void unpinPinnedSession()}
      onFocusRequest={() => {
        setActiveSurface("pinned");
        terminalTextInputRef.current?.focus({ preventScroll: true });
      }}
      onSessionExit={handlePinnedSessionExit}
      rendererRef={pinnedRendererRef}
      t={t}
    />
  ) : null;

  if (mode === "settings") {
    return (
      <div className="settings-shell">
        {pinnedCardElement}
        <div className="settings-card" onMouseDown={startDrag}>
          <header className="settings-card__header">
            <span className="settings-card__title">
              {t("settings.title")}
              <span className="settings-card__version">v{appVersion}</span>
            </span>
            <div className="settings-card__actions">
              <button
                type="button"
                className="toolbar-button toolbar-button--quit"
                aria-label={t("settings.quit")}
                title={t("settings.quitHint")}
                onClick={() => void quitApp()}
              >
                <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                  <polyline points="16 17 21 12 16 7" />
                  <line x1="21" y1="12" x2="9" y2="12" />
                </svg>
              </button>
              <button
                type="button"
                className="toolbar-button toolbar-button--close"
                aria-label={t("settings.close")}
                title={t("settings.closeHint")}
                onClick={closeSettings}
              >
                ×
              </button>
            </div>
          </header>

          <div className="settings-card__body">
            <nav className="settings-sidebar" aria-label={t("settings.title")} data-no-drag>
              {([
                ["general", SlidersHorizontal],
                ["sessions", SquareTerminal],
                ["shortcuts", Keyboard],
                ["integrations", Blocks],
                ["about", Info],
              ] as const).map(([page, Icon]) => (
                <button
                  key={page}
                  type="button"
                  className={settingsPage === page ? "settings-sidebar__item settings-sidebar__item--active" : "settings-sidebar__item"}
                  aria-current={settingsPage === page ? "page" : undefined}
                  onClick={() => {
                    setSettingsPage(page);
                    if (page === "sessions") void refreshTerminalSessions();
                  }}
                >
                  <Icon size={15} strokeWidth={2} aria-hidden="true" />
                  <span>{t(`settings.menu.${page}`)}</span>
                </button>
              ))}
            </nav>
            <main className="settings-content" data-no-drag>
            {settingsLoadFailed && (
              <div className="settings-save-alert" role="alert">
                <AlertCircle size={16} strokeWidth={2} aria-hidden="true" />
                <span>{t("settings.loadFailed")}</span>
                <button
                  type="button"
                  disabled={settingsLoading}
                  onClick={() => void loadSettings()}
                >
                  <RefreshCw
                    className={settingsLoading ? "settings-save-alert__spinner" : undefined}
                    size={14}
                    strokeWidth={2}
                    aria-hidden="true"
                  />
                  {t(settingsLoading ? "settings.loading" : "settings.retryLoad")}
                </button>
              </div>
            )}
            {settingsSaveFailed && (
              <div className="settings-save-alert" role="alert">
                <AlertCircle size={16} strokeWidth={2} aria-hidden="true" />
                <span>{t("settings.saveFailed")}</span>
                <button
                  type="button"
                  disabled={settingsSaving}
                  onClick={() => void persistSettings().catch(() => undefined)}
                >
                  <RefreshCw
                    className={settingsSaving ? "settings-save-alert__spinner" : undefined}
                    size={14}
                    strokeWidth={2}
                    aria-hidden="true"
                  />
                  {t(settingsSaving ? "settings.saving" : "settings.retrySave")}
                </button>
              </div>
            )}
            {settingsPage === "general" && (
            <GeneralPage
              t={t}
              settings={settings}
              language={language}
              autostartUpdating={autostartUpdating}
              onChangeTheme={changeTheme}
              onChangeLanguage={changeLanguage}
              onChangeGeneralSetting={changeGeneralSetting}
              onChangeLaunchAtStartup={(enabled) => void changeLaunchAtStartup(enabled)}
              onChangeFontSize={changeFontSize}
              onChangeOpacity={changeOpacity}
            />
            )}

            {settingsPage === "shortcuts" && (
            <ShortcutsPage
              t={t}
              shortcuts={shortcuts}
              clipboardHotkey={settings.clipboard_history_hotkey}
              rejectedAction={rejectedAction}
              recordingAction={recordingAction}
              onToggleRecording={toggleRecording}
              onCaptureShortcut={captureShortcut}
              onCancelRecording={cancelRecording}
              onRestoreDefaults={() => void restoreDefaultShortcuts()}
              onClearClipboardHotkey={clearClipboardHotkey}
            />
            )}

            {settingsPage === "sessions" && (
            <SessionsPage
              t={t}
              sessions={terminalSessions}
              loading={sessionsLoading}
              error={sessionsError}
              actionId={sessionActionId}
              dateFormatter={sessionDateFormatter}
              onResume={(session) => void resumeTerminalSession(session)}
              onKill={(session) => void killTerminalSession(session)}
              onRefresh={() => void refreshTerminalSessions()}
            />
            )}

            {settingsPage === "integrations" && (
            <ExtensionsPanel
              t={t}
              locale={language}
              onOpenCommand={(plan: ExtensionExecutionPlan, label: string) => runCommand(plan, label)}
              showCommandsInSearch={settings.show_commands_in_search}
              onToggleCommandsInSearch={toggleCommandsInSearch}
              basePlugins={[
                {
                  id: CLIPBOARD_PLUGIN_ID,
                  titleKey: "settings.clipboardHistory",
                  descriptionKey: "settings.clipboardHistoryHint",
                  enabled: settings.clipboard_history_enabled,
                },
              ]}
              onToggleBasePlugin={(id, enabled) => {
                if (id !== CLIPBOARD_PLUGIN_ID) return;
                changeGeneralSetting("clipboard_history_enabled", enabled);
              }}
            />
            )}

            {settingsPage === "about" && (
            <AboutPage
              t={t}
              appVersion={appVersion}
              updateInfo={updateInfo}
              updateDownloading={updateDownloading}
              updateProgress={updateProgress}
              updateFailed={updateFailed}
              onDownloadUpdate={downloadAndInstallUpdate}
            />
            )}
            </main>
          </div>
        </div>
      </div>
    );
  }

  if (mode === "plugin" && pluginPageId) {
    // A plugin page IS a terminal page: it renders in the very shell the
    // terminal mode uses — same `.terminal-shell` window padding, same
    // `.terminal-panel` card material, radius and platform shadows — shown in
    // place of the terminal canvas while active. The window geometry comes
    // from the backend's `show_plugin_page` (same saved size as terminal
    // mode); the embedded PTY keeps running underneath, untouched. The page
    // itself is whatever HTML the plugin declared, hosted through the generic
    // sandboxed-iframe + bridge pipeline.
    return (
      <div className="terminal-shell">
        {pinnedCardElement}
        <section className="terminal-panel terminal-panel--entered">
          <div className="terminal-panel__body">
            <PluginPageHost
              pluginId={pluginPageId}
              language={language}
              theme={resolvedTheme}
              mainOpacity={normalizeOpacity(settings.main_opacity) / 100}
              terminalOpacity={normalizeOpacity(settings.terminal_opacity) / 100}
              onClose={closePluginPage}
            />
          </div>
        </section>
      </div>
    );
  }

  if (mode === "collapsed") {
    const hasQuery = query.trim().length > 0;
    // The first scan runs before there is anything to search, so the input says
    // so rather than inviting a query that would match nothing.
    const placeholder = appsError
      ? t("input.scanFailed")
      : appsLoading && !applications.length
        ? t("input.scanning")
        : t("input.placeholder");

    return (
      <div className="collapsed-shell">
        {pinnedCardElement}
        <div
          ref={collapsedCardRef}
          className={`collapsed-card${hasQuery ? " collapsed-card--filled" : ""}`}
          onMouseDown={startDrag}
          onClick={() => focusCollapsedInput()}
        >
          <div className="collapsed-card__input-row">
            <div className="collapsed-card__aura" aria-hidden="true" />
            <input
              ref={inputRef}
              className="collapsed-card__input"
              role="combobox"
              aria-label={t("input.placeholder")}
              aria-autocomplete="list"
              aria-expanded={launcherResults.length > 0 || Boolean(actionBar)}
              aria-controls={launcherResults.length > 0 || actionBar ? "launcher-options" : undefined}
              aria-activedescendant={
                selectedActionBar && actionBar
                  ? "launcher-option-action"
                  : launcherResults[selectedResultIndex]
                    ? `launcher-option-${selectedResultIndex}`
                    : undefined
              }
              value={query}
              onChange={(event) => {
                setLauncherFeedback(null);
                setQuery(event.target.value);
                setHistoryIndex(-1);
              }}
              onKeyDown={onInputKeyDown}
              onCompositionStart={() => { isComposing.current = true; }}
              onCompositionEnd={() => {
                isComposing.current = false;
              }}
              onKeyUp={(event) => {
                if (
                  ["Meta", "Control", "Alt", "Shift"].includes(event.key) &&
                  actionBar &&
                  selectedActionBar &&
                  !matchesShortcutModifiers(event.nativeEvent, shortcuts.select_result)
                ) {
                  setSelectedActionBar(false);
                }
              }}
              placeholder={placeholder}
              autoFocus
              spellCheck={false}
              autoCapitalize="off"
              autoCorrect="off"
            />
            <button
              type="button"
              className="collapsed-card__settings"
              aria-label={t("terminal.sessions")}
              title={t("terminal.sessionsOpen")}
              onMouseDown={(event) => {
                event.preventDefault();
                event.stopPropagation();
              }}
              onClick={(event) => {
                event.stopPropagation();
                openSettings("sessions");
              }}
            >
              <SquareTerminal size={16} strokeWidth={1.8} aria-hidden="true" />
            </button>
            <button
              type="button"
              className="collapsed-card__settings"
              aria-label={t("settings.open")}
              title={t("settings.openHint", { shortcut: formatShortcut(shortcuts.open_settings) })}
              onMouseDown={(event) => {
                event.preventDefault();
                event.stopPropagation();
              }}
              onClick={(event) => {
                event.stopPropagation();
                openSettings();
              }}
            >
              <svg
                viewBox="0 0 24 24"
                width="15"
                height="15"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.7"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden="true"
              >
                <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            </button>
          </div>
          {/* The clip wrapper is what animates: grid-template-rows 0fr → 1fr
              collapses/expands the whole bottom area without a hard height
              jump. The content stays mounted, only the class flips. */}
          <div
            className={
              launcherResults.length > 0 || actionBar || launcherFeedback || appsError
                ? "launcher-bottom-clip launcher-bottom-clip--open"
                : "launcher-bottom-clip"
            }
          >
            <div className="launcher-bottom">
              {appsError && (
                <div className="launcher-feedback" role="alert">
                  <AlertCircle className="launcher-feedback__icon" size={15} strokeWidth={1.9} aria-hidden="true" />
                  <span>{t("input.scanFailed")}</span>
                  <button
                    type="button"
                    className="launcher-feedback__retry"
                    aria-label={t("input.retryScan")}
                    title={t("input.retryScan")}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={(event) => {
                      event.stopPropagation();
                      scanApplications(true);
                    }}
                  >
                    <RefreshCw size={13} strokeWidth={2} aria-hidden="true" />
                  </button>
                </div>
              )}
              {launcherFeedback && (
                <div className="launcher-feedback" role="status" aria-live="polite">
                  <AlertCircle className="launcher-feedback__icon" size={15} strokeWidth={1.9} aria-hidden="true" />
                  <span>{t(launcherFeedback)}</span>
                </div>
              )}
              <LauncherResults
                t={t}
                results={launcherResults}
                actionBar={actionBar}
                appIconUrls={appIconUrls}
                selectedResultIndex={selectedResultIndex}
                selectedActionBar={selectedActionBar}
                resultShortcutSlots={resultShortcutSlots}
                actionBarShortcut={actionBarShortcut}
                selectResultShortcut={shortcuts.select_result}
                showRecentTitle={!query.trim()}
                onSelectResult={(index) => {
                  setSelectedActionBar(false);
                  setSelectedResultIndex(index);
                }}
                onSelectActionBar={() => setSelectedActionBar(true)}
                onRunResult={runLauncherItem}
                onRunActionBar={() => {
                  if (actionBar) executeActionBar(actionBar);
                }}
              />
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="terminal-shell">
      {pinnedCardElement}
      <section className="terminal-panel terminal-panel--entered">
        <header className="terminal-bar" onMouseDown={startDrag}>
          <div className="terminal-bar__frost" />
          <div className="terminal-panel__actions">
            <button
              className="toolbar-button toolbar-button--popout"
              aria-label={t("terminal.openInTerminal")}
              title={t("terminal.openInTerminalHint", {
                shortcut: formatShortcut(shortcuts.open_external_terminal),
              })}
              onClick={() => void openInTerminal()}
            >
              ↗
            </button>
            <button
              className="toolbar-button toolbar-button--close"
              aria-label={t("terminal.newCommand")}
              title={t("terminal.newCommandHint", {
                shortcut: formatShortcut(shortcuts.new_command),
              })}
              onClick={returnToInputMode}
            >
              ×
            </button>
          </div>
        </header>

        <div className="terminal-panel__body">
          <div
            ref={mountRef}
            className="terminal-panel__mount"
            onMouseDown={onCanvasMouseDown}
            onMouseMove={onCanvasMouseMove}
            onContextMenu={(event) => event.preventDefault()}
          >
            <canvas ref={canvasRef} className="terminal-canvas" />
            <textarea
              ref={terminalTextInputRef}
              className="terminal-text-input"
              aria-label={t("terminal.input")}
              rows={1}
              spellCheck={false}
              autoCapitalize="off"
              autoCorrect="off"
              onInput={onTerminalTextInput}
              onCompositionStart={() => {
                terminalComposing.current = true;
              }}
              onCompositionEnd={() => {
                terminalComposing.current = false;
                // Some WebKit builds emit the final input event before
                // compositionend. The microtask covers both event orders and
                // sees an empty value when onInput already flushed it.
                queueMicrotask(() => flushTerminalTextInput());
              }}
            />
          </div>
          {mainPinnedAway && (
            <div className="terminal-pinned-note" role="status">
              <span className="terminal-pinned-note__title">{t("terminal.pinnedOverlay")}</span>
              <span>
                {t("terminal.pinnedOverlayHint", {
                  shortcut: formatShortcut(shortcuts.pin_terminal),
                })}
              </span>
            </div>
          )}
          {terminalFeedback && (
            <div className="terminal-feedback" role="status" aria-live="polite">
              <AlertCircle className="terminal-feedback__icon" size={15} strokeWidth={1.9} aria-hidden="true" />
              <span>{t(terminalFeedback)}</span>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
