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
  X,
} from "lucide-react";
import { TerminalCanvas } from "./terminal/render";
import { PinnedTerminalCard } from "./terminal/PinnedTerminalCard";
import { PINNED_SESSION_ID } from "./terminal/pinState";
import { useTerminalView, terminalFontFamily } from "./hooks/useTerminalView";
import { useLauncherCatalog } from "./hooks/useLauncherCatalog";
import { usePinCoordinator } from "./hooks/usePinCoordinator";
import { useTimedFeedback } from "./hooks/useTimedFeedback";
import { ToastHost } from "./components/ToastStack";
import { appendToast, removeToast, type AppToast, type ToastAction, type ToastKind } from "./toast-state";
import { useLauncherActions } from "./hooks/useLauncherActions";
import { useAppKeyboard } from "./hooks/useAppKeyboard";
import {
  applySurfaceFocusOnEntry,
  focusSettingsSidebar,
  settingsSidebarTabIndex,
  surfaceFocusBeats,
} from "./surface-policy";
import { useSettings } from "./hooks/useSettings";
import { useShortcutCapture } from "./hooks/useShortcutCapture";
import { useLauncherHeight, syncLauncherHeight } from "./hooks/useLauncherHeight";
import {
  createCollapsedFocusController,
  setCollapsedFocusReassert,
  type CollapsedFocusController,
} from "./collapsed-focus";
import { useSessionManagement } from "./hooks/useSessionManagement";
import { useTimedReset } from "./hooks/useTimedReset";
import {
  FOCUS_IN_OUT,
} from "./terminal/keys";
import {
  createTranslator,
  isMessageKey,
  normalizeLanguage,
  type Language,
} from "./i18n";
import {
  DEEP_LINK_CONNECT_EVENT,
  DEEP_LINK_REJECT_EVENT,
  deepLinkRejectGate,
  type DeepLinkConnectRequest,
} from "./deep-link";
import { ExtensionsPanel, type ExtensionExecutionPlan } from "./ExtensionsPanel";
import { PluginPageHost } from "./plugins/PluginPageHost";
import { CLIPBOARD_PLUGIN_ID } from "./plugin-pages";
import {
  formatResultShortcut,
  formatShortcut,
  IS_LINUX,
  IS_MAC,
  IS_WINDOWS,
  matchesShortcutModifiers,
  withShortcutDefaults,
  type ShortcutMap,
} from "./shortcuts";
import { type SettingsPage } from "./settings-persistence";
import { GeneralPage, normalizeFontSize, normalizeOpacity } from "./settings/GeneralPage";
import { ShortcutsPage } from "./settings/ShortcutsPage";
import { SessionsPage } from "./settings/SessionsPage";
import { AboutPage } from "./settings/AboutPage";
import {
  LauncherResults,
  type LauncherItem,
} from "./launcher/LauncherResults";
import "./styles/launcher.css";
import "./styles/terminal.css";
import "./styles/settings.css";
import "./styles/extensions.css";
import "./styles/pinned-card.css";
import "./styles/base.css";

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
// The Liquid Glass vocabulary lives in its own React-free module so the node
// test runner can import it directly; App re-exports it for the components.
import type { GlassStep } from "./glass-material";
export { GLASS_STEPS, normalizeGlassStep } from "./glass-material";
export type { GlassStep } from "./glass-material";

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
  glass_step: GlassStep;
  shortcuts: ShortcutMap;
  /** Whether extension/provider commands appear in launcher search results. */
  show_commands_in_search: boolean;
  /** Whether the empty launcher offers the most-launched applications
   * (default on). Off leaves the result list empty until something is typed. */
  show_recent_in_launcher: boolean;
  /** Whether the built-in clipboard history monitor runs (default on). */
  clipboard_history_enabled: boolean;
  /** Global hotkey that summons the clipboard panel. */
  clipboard_history_hotkey: string;
  /** Application path -> launch count, ranking the empty-query recent list. */
  launch_counts: Record<string, number>;
  /** Settings page that was open last, restored on the next launch. */
  last_settings_page: SettingsPage;
  /** Whether the first-run onboarding tip has been dismissed. */
  seen_tip: boolean;
}

const INPUT_WINDOW_WIDTH = 720;
const SETTINGS_WINDOW_HEIGHT = 580;
const SETTINGS_MIN_HEIGHT = 420;
/** How long the panel ignores a blur after a Windows drag; see `startDrag`.
 *  `start_dragging()` opens a modal move loop the webview spends unfocused,
 *  just like the old `WM_NCLBUTTONDOWN` path did - so the same grace period is
 *  still needed to keep the hide-on-blur listener from dismissing the panel
 *  out from under the drag. */
const DRAG_BLUR_GRACE = 600;
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
  const appQuitting = useRef(false);

  const ptyReady = useRef(false);
  /** The card's counterpart to `ptyReady`. Both live here, next to the other
   * broker-side bookkeeping, because the pin coordinator writes this one and the
   * terminal view's input gates read it. `ptyReady` describes the MAIN slot
   * only, and pinning empties that slot by design. */
  const pinnedReady = useRef(false);
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
  const [selectedResultIndex, setSelectedResultIndex] = useState(0);
  /** Whether the action bar, rather than a row of the result list, is the thing
   * Enter runs. The two selections are exclusive but kept apart, because the
   * action bar is not a result: it is never numbered and never in `Ctrl+N`. */
  const [selectedActionBar, setSelectedActionBar] = useState(false);
  // Two-step system power confirmation: armed while the restart/shutdown row
  // shows an inline confirmation. Input Enter is inert; Escape cancels.
  const [pendingSystemAction, setPendingSystemAction] = useState<Extract<LauncherItem, { type: "system" }> | null>(null);
  useTimedReset(pendingSystemAction, () => setPendingSystemAction(null));
  /** First-run onboarding tip: shown once in the launcher until dismissed. */
  const [showOnboardingTip, setShowOnboardingTip] = useState(false);
  const [autostartUpdating, setAutostartUpdating] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<{ version: string } | null>(null);
  const [updateDownloading, setUpdateDownloading] = useState(false);
  const updateBusy = useRef(false);
  const [updateProgress, setUpdateProgress] = useState<{ downloaded: number; total: number } | null>(null);
  const [updateFailed, setUpdateFailed] = useState(false);
  /** A validated `floter://connect` request, waiting for the integrations
   *  panel to turn it into a review dialog. The backend already checked the
   *  path and the manifest structure; this is only the hand-off. */
  const [pendingDeepLink, setPendingDeepLink] = useState<DeepLinkConnectRequest | null>(null);
  const isComposing = useRef(false);
  const suppressBlurUntil = useRef(0);

  // Settings: state, hydration, persistence, and the change* mutators. The
  // hook owns every ref/state above the line and exposes them; downstream
  // hooks (`useLauncherCatalog`) take the refs/saves it returns.
  const {
    settings,
    setSettings,
    settingsPage,
    setSettingsPage,
    settingsSaving,
    settingsSaveFailed,
    dismissSaveError,
    settingsLoading,
    settingsLoadFailed,
    settingsRef,
    settingsHydration,
    persistSettings,
    loadSettings,
    changeOpacity,
    changeGlassStep,
    changeFontSize,
    changeGeneralSetting,
    changeTheme,
    changeLanguage,
    changeLaunchAtStartup,
  } = useSettings({
    suppressBlurUntil,
    autostartUpdating,
    setAutostartUpdating,
  });
  useTimedReset(settingsSaveFailed, dismissSaveError, 5000);
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
  /** Mirror of the translator for listeners registered once: they must see the
   *  current language without re-subscribing (see the deep-link effect). */
  const tRef = useRef(t);
  tRef.current = t;
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

  const toggleCommandsInSearch = () => {
    changeGeneralSetting("show_commands_in_search", !settingsRef.current.show_commands_in_search);
  };

  // Imperative shortcut recording & capture: the row of the settings page
  // flips between idle / recording / rejected, and an in-flight capture
  // optimistically swaps the binding in `settings.shortcuts` (or
  // `clipboard_history_hotkey`) before asking the backend to take it.
  const {
    toggle: toggleRecording,
    cancel: cancelRecording,
    capture: captureShortcut,
    clearClipboardHotkey,
    restoreDefaults: restoreDefaultShortcuts,
    reset: resetRecording,
    rejectedAction,
    recordingAction,
    saving: shortcutsSaving,
  } = useShortcutCapture({
    settings,
    setSettings,
    settingsRef,
    shortcuts,
    settingsHydration,
    suppressBlurUntil,
  });
  const actionBarShortcut = useMemo(
    () => formatResultShortcut(shortcuts.select_result, "Enter"),
    [shortcuts.select_result],
  );

  const {
    launcherFeedback,
    terminalFeedback,
    setLauncherFeedback,
    setTerminalFeedback,
    showLauncherFeedback,
    showTerminalFeedback,
  } = useTimedFeedback();

  // App-level toast stack. Every surface (currently the integrations panel,
  // later others) pushes feedback here so it renders once, pinned to the card
  // and never inside a scroll container.
  const [toasts, setToasts] = useState<AppToast[]>([]);
  const toastIdRef = useRef(0);
  const notify = useCallback(
    (kind: ToastKind, text: string, action?: ToastAction) => {
      const id = ++toastIdRef.current;
      setToasts((current) => appendToast(current, { id, kind, text, action }));
    },
    [],
  );
  const dismissToast = useCallback((id: number) => {
    setToasts((current) => removeToast(current, id));
  }, []);
  // The toast host is mounted on every mode's shell (see the branches below),
  // so the stack survives mode switches with the rest of the surface. Its
  // dismiss timers therefore never unmount mid-flight; the queue empties on its
  // own as each toast times out or is dismissed.

  const {
    sessions: terminalSessions,
    loading: sessionsLoading,
    error: sessionsError,
    actionId: sessionActionId,
    refreshSessions: refreshTerminalSessions,
    scheduleRefresh: scheduleSessionRefresh,
    killSession: killTerminalSession,
  } = useSessionManagement();


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

  // The launcher's input is the keyboard's home on the collapsed surface.
  // One collector owns that guarantee (see `collapsed-focus.ts`); its `focus`
  // schedules the multi-beat pattern and `reassert` is chained off every
  // native resize, while `attach` installs the focusout / window-focus
  // watchers. `focusCollapsedInput` stays as the scheduling entry point every
  // existing call site already uses.
  const collapsedFocusRef = useRef<CollapsedFocusController | null>(null);
  if (!collapsedFocusRef.current) {
    collapsedFocusRef.current = createCollapsedFocusController({
      refs: { modeRef, inputRef, cardRef: collapsedCardRef },
      // macOS only: a DOM focus() alone leaves the caret invisible until the
      // `WKWebView` is the window's AppKit first responder. An iframe that held
      // the keyboard leaves the responder on a view inside it, so ask the
      // native side to re-arm the web view every time focus lands here. The
      // command is a no-op-shaped `set_focus` on the other engines, but it is
      // not sent there — Linux (WebKitGTK) and Windows (WebView2) draw the
      // caret from DOM focus already, and skipping the IPC keeps their tested
      // behaviour byte-for-byte. See `refocus_webview` in lib.rs.
      nativeRefocus: IS_MAC ? () => { void invoke("refocus_webview").catch(() => undefined); } : undefined,
    });
  }
  const collapsedFocus = collapsedFocusRef.current;
  const focusCollapsedInput = (delay = 0) => {
    collapsedFocus.focus(delay);
  };
  // The standard beat pattern for a commit that lands on the collapsed
  // surface: the commit instant plus two later attempts that ride out the
  // platform's reveal/autoFocus races (and a Windows retry, where the window
  // is still being shown when the first attempt fires). The rhythm itself is
  // the collapsed row of `SURFACE_FOCUS_POLICY`, whose `beats` reference
  // `COLLAPSED_FOCUS_BEATS_MS` in `collapsed-focus.ts` — the collector that
  // enforces it. Every path back to the launcher schedules through here, plus
  // the resize-settled reassert in `syncLauncherHeight`, so no path can forget
  // a beat.
  const scheduleCollapsedFocusBeats = () => {
    for (const beat of surfaceFocusBeats("collapsed")) focusCollapsedInput(beat);
  };
  // Let the leaf `syncLauncherHeight` helper re-run the collector when a
  // native launcher resize settles; the controller is stable for the app's
  // lifetime, so this is registered once below (not on every render).
  useEffect(() => {
    setCollapsedFocusReassert(() => collapsedFocus.reassert());
    return () => setCollapsedFocusReassert(null);
  }, [collapsedFocus]);

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
      syncLauncherHeight(collapsedCardRef);
    } catch {
      // The DOM still transitions back to a usable launcher even if the native
      // resize failed; keep the keyboard recovery below independent of IPC.
    }
    scheduleCollapsedFocusBeats();
  };

  const openSettings = (page?: SettingsPage) => {
    suppressBlurUntil.current = Date.now() + 400;
    const nextPage = page ?? settingsPage;
    setSettingsPage(nextPage);
    if (nextPage === "sessions") scheduleSessionRefresh();
    setMode("settings");
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
    // The iframe is kept alive across mode switches (see `pluginLayer`), so it
    // is no longer torn down — and therefore no longer blurs itself — when the
    // page closes. It stays hidden but can still hold the keyboard while the
    // plugin id is null (a hidden iframe document is still focusable on
    // WebKit), which would swallow the first keystroke aimed at the surface
    // underneath. `scheduleCollapsedFocusBeats` runs the standard commit-instant
    // + later-beat pattern, the resize-settled reassert chains off
    // `syncLauncherHeight`, and the collector's focusout/window-focus watchers
    // reclaim the input if the iframe (or anything else) takes the keyboard
    // back later still.
    if (pluginReturnMode.current === "terminal") {
      focusTerminalView(0);
      focusTerminalView(80);
    } else {
      scheduleCollapsedFocusBeats();
    }
  };

  /** Open a plugin page over whatever surface is showing, remembering it for
   * the return trip. One path for every trigger — hotkey, `floter clip`, the
   * launcher entry, a cold-start request. Deliberately does NOT arm
   * `restoringMode`: sizing belongs to the backend here — the mode effect
   * calls `show_plugin_page`, which applies the same saved geometry terminal
   * mode uses. */
  const openPluginPage = (pluginId: string) => {
    suppressBlurUntil.current = Date.now() + 400;
    if (modeRef.current !== "plugin") {
      pluginReturnMode.current = modeRef.current === "terminal" ? "terminal" : "collapsed";
    }
    modeRef.current = "plugin";
    pluginPageIdRef.current = pluginId;
    setPluginPageId(pluginId);
    setMode("plugin");
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
    surfaceReady,
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
    pinnedReady,
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
    scheduleCollapsedFocusBeats,
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
    setHistory,
    historyIndex,
    setHistoryIndex,
    draftBeforeHistory,
    rememberCommand,
    recordLaunch,
  } = useLauncherCatalog({
    query,
    launchCounts: settings.launch_counts,
    showCommandsInSearch: settings.show_commands_in_search,
    showRecentInLauncher: settings.show_recent_in_launcher,
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
    pinnedReady,
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

  const {
    runCommand,
    resumeTerminalSession,
    executeActionBar,
    runLauncherItem,
    handleLauncherKey,
    executeSystemAction,
    cancelSystemAction,
  } = useLauncherActions({
    query,
    resolvedTheme,
    t,
    terminalOpening,
    systemPowerOpening,
    ptyReady,
    mainBrokerSessionIdRef,
    terminalGeneration,
    nextTerminalGeneration,
    sessionClosePromise,
    dimsRef,
    pinStateRef,
    dispatchPinEvent,
    setMainPinnedAway,
    setLauncherFeedback,
    setTerminalFeedback,
    showLauncherFeedback,
    setTerminalMounted,
    setMode,
    setQuery,
    setHistoryIndex,
    setSelectedResultIndex,
    setSelectedActionBar,
    ensureTerminalSession,
    openInTerminal,
    focusTerminalView,
    focusCollapsedInput,
    scheduleCollapsedFocusBeats,
    rememberCommand,
    recordLaunch,
    refreshTerminalSessions,
    openPluginPage,
    isComposing,
    actionBar,
    shortcuts,
    launcherResults,
    resultShortcutSlots,
    runnableResultFlags,
    selectedResultIndex,
    selectedActionBar,
    history,
    historyIndex,
    draftBeforeHistory,
    collapsedCardRef,
    pendingSystemAction,
    setPendingSystemAction,
  });

  /** Sidebar buttons by page, so ↑/↓ can move focus with the selection. */
  const settingsSidebarButtons = useRef(new Map<SettingsPage, HTMLButtonElement>());
  /**
   * The settings surface's keyboard home: the sidebar button for the page
   * currently shown. Plain DOM focus — deliberately not a native
   * make-key/reveal command, so the macOS first-responder chain is untouched
   * (see the entry policy in `surface-policy.ts`).
   */
  const focusCurrentSettingsSidebar = () =>
    focusSettingsSidebar(settingsSidebarButtons.current, settingsPage);

  /**
   * The app's focus entry points, handed to the surface policy. Every "this
   * surface takes the keyboard on entry" path — the mode effect and the
   * reveal listener — runs through these same three seams, so the policy is
   * the only place that decides who owns the keyboard.
   */
  const focusSeams = {
    focusCollapsedInput,
    focusTerminalView,
    focusSettingsSidebar: focusCurrentSettingsSidebar,
  };
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

  useAppKeyboard({
    mode,
    shortcuts,
    recordingAction,
    launcherResults,
    query,
    inputRef,
    selectionRef,
    dimsRef,
    surfaceReady,
    terminalTextInputRef,
    terminalInputTarget,
    activeRenderer,
    activeSurfaceRef,
    setActiveSurface,
    focusCollapsedInput,
    returnToInputMode,
    openInTerminal,
    togglePinnedTerminal,
    copySelection,
    pasteClipboard,
    closeSettings,
    openSettings,
    settingsPage,
    changeSettingsPage,
    settingsSidebarButtons,
    refreshTerminalSessions,
    closePluginPage,
    runLauncherItem,
    handleLauncherKey,
    resultShortcutSlots,
    setQuery,
    setHistory,
    showLauncherFeedback,
    setHistoryIndex,
    collapsedCardRef,
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

  // The material step is an attribute rather than a custom property because it
  // swaps a *set* of tokens (`[data-glass]` in base.css) and because the
  // attribute is what the a11y override blocks key off. It is deliberately not
  // part of the opacity effect above: the two controls are orthogonal, and one
  // effect per axis keeps that visible in the code.
  useEffect(() => {
    document.documentElement.setAttribute("data-glass", settings.glass_step);
    const renderer = rendererRef.current;
    if (renderer) {
      renderer.updateTheme();
      render();
    }
  }, [settings.glass_step]);

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

  // The action bar is a secondary control for a visible result list. Do not
  // expand the result area for an unmatched query just because the generic
  // shell fallback exists; feedback rows remain independently visible below.
  const visibleActionBar = launcherResults.length > 0 ? actionBar : null;

  // The launcher window is exactly as tall as the rows inside it, measured
  // rather than predicted. Extracted into useLauncherHeight hook.
  useLauncherHeight(mode, collapsedCardRef, [
    visibleActionBar,
    launcherFeedback,
    launcherResults.length,
  ]);

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

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  // Show the first-run onboarding tip in the launcher the first time the user
  // opens it; persist dismissal as `seen_tip` so it never returns.
  useEffect(() => {
    if (settingsLoading) return;
    setShowOnboardingTip(!settings.seen_tip && mode === "collapsed");
  }, [settingsLoading, settings.seen_tip, mode]);

  const dismissOnboardingTip = useCallback(() => {
    setShowOnboardingTip(false);
    if (!settings.seen_tip) {
      changeGeneralSetting("seen_tip", true);
    }
  }, [settings.seen_tip, changeGeneralSetting]);

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
  // keyboard lives in `useShortcutCapture`. Leaving it set would mute every
  // key handler in the app, so it is cleared on the way out of settings —
  // the panel can be left by the close button, the global toggle or the
  // tray, and each of those would otherwise need its own reset.
  useEffect(() => {
    if (mode === "settings") return;
    resetRecording();
  }, [mode, resetRecording]);

  useEffect(() => {
    const isRestoring = restoringMode.current === mode;
    suppressBlurUntil.current = Date.now() + 400;

    // The entry focus policy for every surface, declared once in
    // `surface-policy.ts`. Each branch below asks the table who owns the
    // keyboard instead of spelling out its own beat list.
    if (mode === "settings") {
      // Opened from the collapsed card: the window is already visible and keeps
      // its top edge, so only the panel height changes. The keyboard lands on
      // the sidebar item for the current page, which is what makes ↑/↓ and Tab
      // work the moment the panel appears instead of starting on `<body>`.
      applySurfaceFocusOnEntry("settings", focusSeams);
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
            // `syncLauncherHeight` re-runs the focus collector once its native
            // resize settles (see `collapsed-focus.ts`); the beats below cover
            // the commit instant and the reveal race in front of it.
            syncLauncherHeight(collapsedCardRef);
          })
          .catch(() => undefined);
      }
      scheduleCollapsedFocusBeats();
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
    // Windows shows and focuses the window around the time that first attempt
    // lands, and a canvas that missed the keyboard swallows the first key
    // pressed into it. Chased a second time through the same table entry.
    applySurfaceFocusOnEntry("terminal", focusSeams);
    const timer = window.setTimeout(() => {
      if (restoringMode.current === "terminal") {
        restoringMode.current = null;
      }
    }, 160);
    return () => window.clearTimeout(timer);
  }, [mode]);

  // Search results keep the keyboard on the combobox, while Tab may move into
  // the session and settings controls. The collector reclaims focus only when
  // it leaves the card entirely, which covers a click on the window chrome, a
  // kept-alive plugin iframe taking the keyboard, or an element unmounting
  // under the caret — without trapping keyboard users in the input. It also
  // reclaims when the window itself regains OS focus (a native reveal or a WM
  // re-activation), the moment a WebView is most likely to have dropped the
  // input. The controller gates every reclaim on `modeRef`, so the watchers
  // are installed once for the app's lifetime.
  useEffect(() => collapsedFocus.attach(), [collapsedFocus]);

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
      if (modeRef.current === "settings") {
        // The native toggle only remembers terminal/launcher geometry. Keep
        // the mounted editor and its draft, then restore the settings height.
        const height = Math.min(SETTINGS_WINDOW_HEIGHT, Math.max(240, window.screen.availHeight - 24));
        void getCurrentWindow().setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height)).catch(() => undefined);
        const dialog = document.querySelector<HTMLElement>('[aria-modal="true"]:not([inert])');
        if (dialog && !dialog.contains(document.activeElement)) dialog.focus({ preventScroll: true });
        return;
      }
      if (modeRef.current === "plugin") {
        void invoke("show_plugin_page").catch(() => undefined);
        return;
      }
      if (event.payload === "terminal") {
        restoringMode.current = "terminal";
        setTerminalMounted(true);
        setMode("terminal");
        // The same declared policy the mode effect runs: focusTerminalView(80),
        // chased once more on Windows because the reveal that brought the
        // window back is still settling there.
        applySurfaceFocusOnEntry("terminal", focusSeams);
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
      scheduleCollapsedFocusBeats();
      // A hidden launcher can retain the height of its previous result list;
      // when it is revealed the mode may already be "collapsed", so the
      // mode effect will not run again. Measure after the state commit to
      // restore the compact window before it is painted with stale space.
      window.requestAnimationFrame(() => {
        if (modeRef.current === "collapsed") {
          syncLauncherHeight(collapsedCardRef);
        }
      });
      // A webview resuming from a hidden state can run that rAF against a
      // layout that has not been recomputed since it was frozen; one delayed
      // sync catches the settled measurement. Gated on restoringMode so a
      // mode switch that happened in between is never clobbered.
      window.setTimeout(() => {
        if (
          modeRef.current === "collapsed" &&
          restoringMode.current === "collapsed"
        ) {
          syncLauncherHeight(collapsedCardRef);
        }
      }, 120);
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

  // Tray menu hooks: "Settings" and "Reload" surface here so the existing
  // IPC commands (`hide_window`/settings flow / `list_applications`) can do the
  // work — the tray is just a trigger, the frontend is what owns the modes.
  useEffect(() => {
    const unlistenSettingsPromise = listen("floter://open-settings", () => {
      openSettings();
    });
    const unlistenReloadPromise = listen("floter://reload-apps", () => {
      scanApplications(true);
    });
    return () => {
      unlistenSettingsPromise.then((unlisten) => unlisten());
      unlistenReloadPromise.then((unlisten) => unlisten());
    };
  }, []);

  // The `floter://` scheme's two outcomes. The backend owns the allow-list and
  // the validation; here a *validated* connect request becomes the review
  // dialog, and a refusal becomes one toast. Neither path installs anything:
  // the dialog's Connect button runs the ordinary `extensions_install`, so a
  // link can never approve or enable on the user's behalf.
  useEffect(() => {
    const openReview = (request: DeepLinkConnectRequest) => {
      setPendingDeepLink(request);
      openSettings("integrations");
    };
    const unlistenConnectPromise = listen<DeepLinkConnectRequest>(
      DEEP_LINK_CONNECT_EVENT,
      (event) => openReview(event.payload),
    );
    // A refusal is externally triggered and may repeat (a page retrying a
    // broken link, a shell loop), so it rides the app's existing 30s failure
    // deduper — the same gate the clipboard page's background poll uses.
    const unlistenRejectPromise = listen<string>(DEEP_LINK_REJECT_EVENT, (event) => {
      const key = event.payload;
      // The backend sends a dictionary key, never a sentence: unknown keys are
      // dropped rather than painted, exactly like the plugin-page bridge.
      if (!isMessageKey(key)) return;
      if (!deepLinkRejectGate.allow(key)) return;
      notify("error", tRef.current(key));
    });
    // A cold start (`floter connect …` with nothing listening) dispatches
    // before this listener exists, so the backend stores the request and it is
    // consumed once, here.
    invoke<DeepLinkConnectRequest | null>("take_pending_deep_link")
      .then((pending) => {
        if (pending) openReview(pending);
      })
      .catch(() => undefined);
    return () => {
      unlistenConnectPromise.then((unlisten) => unlisten());
      unlistenRejectPromise.then((unlisten) => unlisten());
    };
    // `openSettings` and `notify` are stable app-lifetime callbacks; the
    // translator is read through a ref so a language change never has to
    // re-subscribe the listeners (a re-subscription window is a dropped link).
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
          scheduleCollapsedFocusBeats();
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


  useEffect(() => {
    check().then((update) => {
      if (update?.available) {
        setUpdateInfo({ version: update.version });
      }
    }).catch(() => undefined);
  }, []);

  const downloadAndInstallUpdate = async () => {
    if (updateBusy.current) return;
    updateBusy.current = true;
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
    } finally {
      updateBusy.current = false;
    }
  };

  const quitApp = async () => {
    if (appQuitting.current) return;
    appQuitting.current = true;
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
      appQuitting.current = false;
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

  // The plugin page host lives OUTSIDE the four mode branches, rendered as the
  // first child of every branch's tree. Because React reconciles siblings by
  // position and type, this is the *same* element instance in all four modes —
  // switching modes never unmounts it, so the sandboxed iframe is created once
  // and reused. That is what makes opening the page instant (no re-fetch of the
  // descriptor, page scripts or clipboard entries) and preserves its filter
  // text, selection and scroll position across toggles. The layer is only
  // painted in plugin mode (`data-active`); in the other modes it stays mounted
  // but `display: none`.
  const pluginLayer = (
    <div
      className="plugin-layer"
      data-active={mode === "plugin" && pluginPageId ? "true" : undefined}
    >
      <PluginPageHost
        pluginId={pluginPageId}
        language={language}
        theme={resolvedTheme}
        mainOpacity={normalizeOpacity(settings.main_opacity) / 100}
        terminalOpacity={normalizeOpacity(settings.terminal_opacity) / 100}
        glassStep={settings.glass_step}
        onClose={closePluginPage}
        onDragStart={startDrag}
        onNotify={notify}
      />
    </div>
  );

  // The toast host is rendered once, as a stable sibling of the mode shell (and
  // of `pluginLayer`), so the stack survives mode switches without unmounting:
  // a toast raised in the integrations panel stays visible when the window
  // flips to the launcher or a plugin page, and its dismiss timer is never cut
  // short. Its positioning is surface-specific, so the host carries the current
  // `data-surface` for `#floter-app-toasts` to key off (see extensions.css) —
  // the containing block is the viewport, whose height varies per surface.
  const toastHost = (
    <ToastHost
      toasts={toasts}
      t={t}
      dataSurface={mode}
      onDismiss={dismissToast}
    />
  );

  if (mode === "settings") {
    return (
      <>
        {pluginLayer}
        {toastHost}
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
                    tabIndex={settingsSidebarTabIndex(page, settingsPage)}
                    onClick={() => {
                      changeSettingsPage(page);
                      if (page === "sessions") scheduleSessionRefresh();
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
                  <button type="button" disabled={settingsLoading} onClick={() => void loadSettings()}>{t("settings.retry")}</button>
                </div>
              )}
              {settingsSaveFailed && (
                <div className="settings-save-alert settings-save-alert--toast" role="alert">
                  <AlertCircle size={16} strokeWidth={2} aria-hidden="true" />
                  <span>{t("settings.saveFailed")}</span>
                  <button
                    type="button"
                    aria-label={t("settings.extensions.dismissNotice")}
                    onClick={dismissSaveError}
                  >
                    <X size={14} strokeWidth={2} aria-hidden="true" />
                  </button>
                </div>
              )}
              {settingsPage === "general" && (
              <GeneralPage
                busy={settingsSaving || settingsLoading}
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
                onChangeGlassStep={changeGlassStep}
              />
              )}

              {settingsPage === "shortcuts" && (
              <ShortcutsPage
                busy={shortcutsSaving || settingsLoading}
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
                settingsBusy={settingsSaving || settingsLoading || autostartUpdating}
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
                onNotify={notify}
                pendingDeepLink={pendingDeepLink}
                onDeepLinkConsumed={() => setPendingDeepLink(null)}
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
                onCopiedLink={(message) => notify("success", message)}
              />
              )}
              </main>
            </div>
          </div>
        </div>
      </>
    );
  }

  if (mode === "plugin" && pluginPageId) {
    // A plugin page IS a terminal page: it renders in the very shell the
    // terminal mode uses — same `.terminal-shell` window padding, same
    // `.terminal-panel` card material, radius and platform shadows — shown
    // underneath the plugin layer while active. The window geometry comes
    // from the backend's `show_plugin_page` (same saved size as terminal
    // mode); the embedded PTY keeps running underneath, untouched. The page
    // itself is whatever HTML the plugin declared, hosted through the generic
    // sandboxed-iframe + bridge pipeline, in the persistent `pluginLayer`
    // above.
    return (
      <>
        {pluginLayer}
        {toastHost}
        <div className="terminal-shell">
          {pinnedCardElement}
          {/* The panel renders only as the rounded backdrop under the plugin
              layer; its body would be entirely covered and stay empty. The
              veil is the tint that gives the frame its glass body while the
              page's own sheet paints over it. */}
          <section className="terminal-panel terminal-panel--entered">
            <div className="terminal-panel__veil" aria-hidden="true" />
          </section>
        </div>
      </>
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
      <>
        {pluginLayer}
        {toastHost}
        <div className="collapsed-shell">
          {pinnedCardElement}
          <div
            ref={collapsedCardRef}
            className={`collapsed-card${hasQuery ? " collapsed-card--filled" : ""}`}
            style={{ "--launcher-results-height": `${Math.max(84, window.screen.availHeight - 220)}px` } as React.CSSProperties}
            onMouseDown={startDrag}
            onClick={(event) => {
              if (!(event.target as HTMLElement).closest("button, input")) focusCollapsedInput();
            }}
          >
            <div className="collapsed-card__input-row">
              <div className="collapsed-card__aura" aria-hidden="true" />
              <input
                ref={inputRef}
                className="collapsed-card__input"
                role="combobox"
                aria-label={t("input.placeholder")}
                aria-autocomplete="list"
                aria-expanded={launcherResults.length > 0}
                aria-controls={launcherResults.length > 0 ? "launcher-options" : undefined}
                aria-activedescendant={
                  selectedActionBar && visibleActionBar
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
            {/* First-run onboarding tip: a small dismissible banner shown above
                the result area the first time the user opens the launcher. */}
            {showOnboardingTip && (
              <div className="launcher-tip" role="status">
                <div className="launcher-tip__body">
                  <span className="launcher-tip__icon" aria-hidden="true">
                    <Info size={14} strokeWidth={1.8} />
                  </span>
                  <div className="launcher-tip__text">
                    <div className="launcher-tip__title">{t("launcher.tipTitle")}</div>
                    <div className="launcher-tip__message">{t("launcher.tipMessage")}</div>
                  </div>
                </div>
                <button
                  type="button"
                  className="launcher-tip__dismiss"
                  onClick={(event) => {
                    event.stopPropagation();
                    dismissOnboardingTip();
                  }}
                >
                  {t("launcher.tipDismiss")}
                </button>
              </div>
            )}
            {/* The clip controls visibility while the native window height follows
                the measured content. Its contents fade in and out without a
                competing CSS height animation. */}
            <div
              className={
                launcherResults.length > 0 || launcherFeedback || appsError
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
                  <div className="launcher-feedback launcher-feedback--warning" role="alert" aria-live="assertive">
                    <AlertCircle className="launcher-feedback__icon" size={15} strokeWidth={1.9} aria-hidden="true" />
                    <span>{t(launcherFeedback)}</span>
                  </div>
                )}
                {pendingSystemAction && (
                  <div className="launcher-system-confirm" role="alert">
                    <AlertCircle className="launcher-system-confirm__icon" size={15} strokeWidth={1.9} aria-hidden="true" />
                    <span className="launcher-system-confirm__message">
                      {t(pendingSystemAction.action === "restart"
                        ? "system.restartConfirm"
                        : "system.shutdownConfirm")}
                    </span>
                    <button
                      type="button"
                      className="launcher-system-confirm__execute"
                      data-destructive-confirm
                      onClick={() => void executeSystemAction()}
                    >
                      {t(pendingSystemAction.action === "restart"
                        ? "system.restart"
                        : "system.shutdown")}
                    </button>
                    <button
                      type="button"
                      className="launcher-system-confirm__cancel"
                      onClick={() => cancelSystemAction()}
                    >
                      {t("settings.extensions.cancel")}
                    </button>
                  </div>
                )}
                <LauncherResults
                  t={t}
                  results={launcherResults}
                  actionBar={visibleActionBar}
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
                    if (visibleActionBar) executeActionBar(visibleActionBar);
                  }}
                />
                {launcherResults.length === 0 &&
                  !actionBar &&
                  !query.trim() &&
                  !settings.show_commands_in_search && (
                    <div className="launcher-hint" role="status">
                      {t("launcher.enableIntegrationsHint")}
                    </div>
                  )}
              </div>
            </div>
          </div>
        </div>
      </>
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
    <>
      {pluginLayer}
      {toastHost}
      <div className="terminal-shell">
        {pinnedCardElement}
        <section className="terminal-panel terminal-panel--entered">
          {/* The glass body sits under the canvas: the renderer paints its own
              pixels at `--terminal-opacity`, and this is the tint that makes
              the frame read as glass rather than as a bare blur. */}
          <div className="terminal-panel__veil" aria-hidden="true" />
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
    </>
  );
}
