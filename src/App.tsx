import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
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
import { TerminalCanvas } from "./terminal/render";
import { PinnedTerminalCard } from "./terminal/PinnedTerminalCard";
import { PINNED_SESSION_ID } from "./terminal/pinState";
import { useTerminalView, terminalFontFamily } from "./hooks/useTerminalView";
import { useLauncherCatalog } from "./hooks/useLauncherCatalog";
import { usePinCoordinator } from "./hooks/usePinCoordinator";
import {
  encodeKey,
  FOCUS_IN_OUT,
  isTerminalCompositionKey,
  shouldUseTerminalTextInput,
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
  nextLauncherSelection,
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
import {
  createSerialSettingsWriter,
  createSettingsHydration,
  normalizeSettingsPage,
  SETTINGS_PAGES,
  type SettingsPage,
} from "./settings-persistence";
import { GeneralPage, normalizeFontSize, normalizeOpacity } from "./settings/GeneralPage";
import { ShortcutsPage, CLIPBOARD_HOTKEY_ACTION } from "./settings/ShortcutsPage";
import { SessionsPage } from "./settings/SessionsPage";
import { AboutPage } from "./settings/AboutPage";
import {
  LauncherResults,
  type ActionBar,
  type LauncherItem,
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
export type ViewMode = "collapsed" | "terminal" | "settings" | "plugin";
export type CursorShape = "beam" | "block" | "underline";

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
  /** Settings page that was open last, restored on the next launch. */
  last_settings_page: SettingsPage;
};

const INPUT_WINDOW_WIDTH = 720;
const SETTINGS_WINDOW_HEIGHT = 580;
const SETTINGS_MIN_HEIGHT = 420;
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

/** Identity zone in the terminal bar: a status dot plus the session title
 * (the command the session was launched with, else the broker session name).
 * The exit event flips it to the exited state. */
export type MainSessionIdentity = { title: string; exited: boolean; exitCode: number | null };

export default function App() {
  const inputRef = useRef<HTMLInputElement>(null);
  /** The launcher card, measured to size the window around it. */
  const collapsedCardRef = useRef<HTMLDivElement>(null);
  const mountRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const terminalTextInputRef = useRef<HTMLTextAreaElement>(null);
  const terminalComposing = useRef(false);
  const settingsSaveTimer = useRef<number | null>(null);
  const settingsHydration = useMemo(() => createSettingsHydration<AppSettings>(), []);
  const settingsLoadPromise = useRef<Promise<void> | null>(null);
  const hydrationSavePromise = useRef<Promise<void> | null>(null);
  const settingsSaveGeneration = useRef(0);
  const appQuitting = useRef(false);

  const ptyReady = useRef(false);
  const terminalGeneration = useRef<number | null>(null);
  /** Daemon-side id of the PTY the main view is attached to; captured at
   * spawn/attach so pinning can hand the session to the card without a
   * listing round-trip. */
  const mainBrokerSessionIdRef = useRef<string | null>(null);
  const pinnedRendererRef = useRef<TerminalCanvas | null>(null);
  const nextTerminalGeneration = useRef(Date.now());
  const sessionClosePromise = useRef<Promise<unknown> | null>(null);
  const terminalOpening = useRef(false);
  const systemPowerOpening = useRef(false);
  const launcherFeedbackTimer = useRef<number | null>(null);
  const terminalFeedbackTimer = useRef<number | null>(null);
  const restoringMode = useRef<ViewMode | null>(null);

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
  /** Header identity (status dot + title) for the session in the main terminal
   * view; null until a spawn/attach has described it. */
  const [mainSessionIdentity, setMainSessionIdentity] = useState<MainSessionIdentity | null>(null);
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
  const [appVersion, setAppVersion] = useState("DEV");
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
    last_settings_page: "general",
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

  // Hoisted above the hook calls so the option objects below can reference
  // them; the bodies are unchanged.
  const focusCollapsedInput = (delay = 0) => {
    window.setTimeout(() => {
      const input = inputRef.current;
      if (!input) return;
      input.focus({ preventScroll: true });
      const length = input.value.length;
      input.setSelectionRange(length, length);
    }, delay);
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

  // ---- extracted hooks ----------------------------------------------------
  // Terminal canvas lifecycle, input and selection; launcher data; pin card
  // coordination. Each hook owns the code moved out of this component
  // verbatim; see src/hooks/.

  const {
    setTerminalMounted,
    rendererRef,
    dimsRef,
    selectionRef,
    render,
    terminalInputTarget,
    activeRenderer,
    focusTerminalView,
    closeTerminalSession,
    ensureTerminalSession,
    describeMainSession,
    resetTerminalFrontendState,
    openInTerminal,
    copySelection,
    pasteClipboard,
    onCanvasMouseDown,
    onCanvasMouseMove,
    onTerminalTextInput,
    flushTerminalTextInput,
  } = useTerminalView({
    canvasRef,
    mountRef,
    terminalTextInputRef,
    terminalComposing,
    mode,
    fontFamily: settings.font_family,
    fontSize: settings.font_size,
    resolvedTheme,
    ptyReady,
    terminalGeneration,
    nextTerminalGeneration,
    mainBrokerSessionIdRef,
    pinnedRendererRef,
    activeSurfaceRef,
    setActiveSurface,
    sessionClosePromise,
    restoringMode,
    setMainSessionIdentity,
    setMainPinnedAway,
    setTerminalFeedback,
    setQuery,
    setMode,
    focusCollapsedInput,
    showTerminalFeedback,
    t,
  });

  const {
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
    historyIndex,
    setHistoryIndex,
    draftBeforeHistory,
    rememberCommand,
    recordLaunch,
  } = useLauncherCatalog({
    query,
    launchCounts: settings.launch_counts,
    showCommandsInSearch: settings.show_commands_in_search,
    t,
    settingsRef,
    settingsHydration,
    setSettings,
    persistSettings,
  });

  const {
    pinState,
    pinStateRef,
    dispatchPinEvent,
    cardGeometry,
    updateCardGeometry,
    togglePinnedTerminal,
    unpinPinnedSession,
    handlePinnedSessionExit,
  } = usePinCoordinator({
    mode,
    resolvedTheme,
    ptyReady,
    terminalGeneration,
    nextTerminalGeneration,
    mainBrokerSessionIdRef,
    dimsRef,
    setActiveSurface,
    setMainPinnedAway,
    setMainSessionIdentity,
    describeMainSession,
    focusTerminalView,
    resetTerminalFrontendState,
    showTerminalFeedback,
    refreshTerminalSessions,
  });

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
          last_settings_page: normalizeSettingsPage(loaded.last_settings_page),
        };
        const hydrated = settingsHydration.mergeLoaded(settingsRef.current, normalized);
        settingsRef.current = hydrated;
        setSettings(hydrated);
        // Reopen on the page the user last left settings on. The merged value
        // wins over `normalized`: a page chosen while the load was in flight
        // must not be clobbered by the disk snapshot.
        setSettingsPage(hydrated.last_settings_page);
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

      refreshApplicationsIfStale();

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

  /** Sidebar buttons by page, so ↑/↓ can move focus with the selection. */
  const settingsSidebarButtons = useRef(new Map<SettingsPage, HTMLButtonElement>());
  /** Switch pages and remember the choice for the next launch. */
  const changeSettingsPage = (page: SettingsPage) => {
    setSettingsPage(page);
    if (settingsRef.current.last_settings_page === page) return;
    const updated = { ...settingsRef.current, last_settings_page: page };
    settingsRef.current = updated;
    setSettings(updated);
    // A page chosen while the load is still in flight must survive hydration.
    settingsHydration.markChanged("last_settings_page");
    void persistSettings().catch(() => undefined);
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
                  ref={(node) => {
                    if (node) settingsSidebarButtons.current.set(page, node);
                    else settingsSidebarButtons.current.delete(page);
                  }}
                  className={settingsPage === page ? "settings-sidebar__item settings-sidebar__item--active" : "settings-sidebar__item"}
                  aria-current={settingsPage === page ? "page" : undefined}
                  onClick={() => {
                    changeSettingsPage(page);
                    if (page === "sessions") void refreshTerminalSessions();
                  }}
                  onKeyDown={(event) => {
                    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
                    event.preventDefault();
                    const delta = event.key === "ArrowDown" ? 1 : SETTINGS_PAGES.length - 1;
                    const next = SETTINGS_PAGES[(SETTINGS_PAGES.indexOf(page) + delta) % SETTINGS_PAGES.length];
                    settingsSidebarButtons.current.get(next)?.focus();
                    changeSettingsPage(next);
                    if (next === "sessions") void refreshTerminalSessions();
                  }}
                >
                  <Icon size={15} strokeWidth={2} aria-hidden="true" />
                  <span>{t(`settings.menu.${page}`)}</span>
                </button>
              ))}
            </nav>
            <main className="settings-content" data-no-drag key={settingsPage}>
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

  const identityTitle = mainSessionIdentity
    ? mainSessionIdentity.exited
      ? `${mainSessionIdentity.title} · ${t("terminal.headerExited", {
          code: mainSessionIdentity.exitCode ?? 0,
        })}`
      : mainSessionIdentity.title
    : null;

  return (
    <div className="terminal-shell">
      {pinnedCardElement}
      <section className="terminal-panel terminal-panel--entered">
        <header className="terminal-bar" onMouseDown={startDrag}>
          <div className="terminal-bar__frost" />
          {mainSessionIdentity && (
            <div className="terminal-bar__identity" title={identityTitle ?? undefined}>
              <span
                className={`terminal-bar__dot${mainSessionIdentity.exited ? " terminal-bar__dot--exited" : ""}`}
                aria-hidden="true"
              />
              <span className="terminal-bar__title">{identityTitle}</span>
            </div>
          )}
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
