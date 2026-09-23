import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { check } from "@tauri-apps/plugin-updater";
import { relaunch } from "@tauri-apps/plugin-process";
import {
  AlertCircle,
  Blocks,
  Clipboard as ClipboardIcon,
  Globe as GlobeIcon,
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
import { residencySurface } from "./surface-residency";
import { useSurfaceResidency } from "./hooks/useSurfaceResidency";
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
  type MessageKey,
} from "./i18n";
import {
  DEEP_LINK_CONNECT_EVENT,
  DEEP_LINK_REGISTER_EVENT,
  DEEP_LINK_REJECT_EVENT,
  deepLinkRejectGate,
  type DeepLinkConnectRequest,
  type DeepLinkRegisterRequest,
} from "./deep-link";
import { ExtensionsPanel, type ExtensionExecutionPlan } from "./ExtensionsPanel";
import { BUILTIN_BASE_PLUGINS, BROWSER_PLUGIN_ID, CLIPBOARD_PLUGIN_ID } from "./plugin-pages";
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
import { GeneralPage, normalizeFontSize } from "./settings/GeneralPage";
import { clampWindowOpacity } from "./glass-material";
import { ShortcutsPage } from "./settings/ShortcutsPage";
import { SessionsPage } from "./settings/SessionsPage";
import { AboutPage } from "./settings/AboutPage";
import {
  LauncherResults,
  type ActionBar,
  type LauncherItem,
} from "./launcher/LauncherResults";
import { PluginTextView } from "./launcher/PluginTextView";
import { PluginConfigOverlay } from "./plugins/PluginConfigOverlay";
import { pluginConfigSchema } from "./plugins/config-schema";
import { pluginViewInteractive, pluginViewPage, pluginViewRows } from "./launcher/plugin-mode";
import { useFileDrops } from "./hooks/useFileDrops";
import { fileDropActionBar, fileDropRows, selectedDroppedFile as droppedFileAt } from "./launcher/file-drops";
import {
  launcherRowHeight,
  launcherWindowHeight,
  MAX_RESULTS,
  resolveLauncherRows,
  RESULTS_VIEWPORT_CHROME,
  shortcutSlotsWithFixedTail,
  type VisibleRowRange,
  withClipboardResultRow,
} from "./launcher/result-budget";
import type { CommandAliases } from "./command-aliases";
import {
  BROWSER_FILTERS,
  browserModeFor,
  clipboardModeFor,
  cycleBrowserFilter as nextBrowserFilter,
  pluginModeEntry,
  pluginModeExitOnBackspace,
  type ActivePluginMode,
  type BrowserMode,
} from "./launcher";
import { INPUT_WINDOW_WIDTH } from "./window-contract";
import type { BrowserSearchField } from "./browser-page";
import { applyUiScale, uiScaleFactor, type UiScale } from "./ui-scale";
import "./styles/launcher.css";
import "./styles/plugin-config.css";
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
export type ViewMode = "collapsed" | "terminal" | "settings";
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
  /** R35 · how many seconds a surface (a plugin mode, its configuration
   *  overlay, the settings panel, the terminal view) survives after the panel
   *  is dismissed, before a summon returns to the search box. `0` disables the
   *  window and restores the pre-R35 behaviour. Normalized by
   *  `normalizeResidencySeconds` on read and by the backend on save. */
  surface_residency_seconds: number;
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
  /** R27 · how many non-favorite clipboard entries the history keeps (10–500,
   * default 300). The clipboard plugin's own settings page writes it through
   * `clipboard_set_settings`; the frontend carries the value so a whole-app
   * settings save cannot silently reset it (the backend re-reads the stored one
   * in `merge_frontend_settings`). */
  clipboard_history_max_items: number;
  /** Application path -> launch count, ranking the empty-query recent list. */
  launch_counts: Record<string, number>;
  /** Settings page that was open last, restored on the next launch. */
  last_settings_page: SettingsPage;
  /** Whether the first-run onboarding tip has been dismissed. */
  seen_tip: boolean;
  /** R7-10c: whether the macOS menu bar status item / Windows+Linux tray icon
   * is shown (default on). Off hides the icon only — the global hotkey and the
   * settings page stay reachable, so no summon path is lost. */
  show_menubar_icon: boolean;
  /** R7-11: user-defined per-command aliases for launcher search, keyed by the
   * catalog command name (`"git"` -> `"gfm"`). The raw map is kept as typed;
   * conflicts between two commands sharing one alias are resolved at search
   * time (see `resolveCommandAliases`). */
  command_aliases: CommandAliases;
  /** R7-13c: the interface-size step (`"default"` / `"large"` / `"larger"`).
   * The stored string is the vocabulary; the step's `--ui-scale` multiplier
   * lives in `ui-scale.ts` and is written onto the document root by a layout
   * effect in this file (before `useLauncherHeight` measures the card). */
  ui_scale: UiScale;
  /** R26-A: the built-in browser plugin's own settings. `target` is `"auto"`
   * or a browser id from `browser_discover`; `custom_base_dir` adds a
   * non-standard profile directory; `history_days` bounds history search
   * (`0` disables the filter). R26-B adds the DevTools debug-port pair the
   * plugin's own page edits. R26-D adds the plugin's on/off switch, which the
   * settings panel's base-plugins row flips. The backend normalizes all six on
   * save. */
  browser_plugin: BrowserPluginSettings;
}

/** R26-A/R26-B: the browser plugin's settings block, mirroring the Rust
 *  `BrowserPluginSettings`. Every field has to be here: a whole-app settings
 *  save submits this object, so a missing member would be read back as its
 *  default and silently wipe the plugin page's own choices. */
export type BrowserPluginSettings = {
  /** R26-D · the plugin's persisted on/off switch (the settings row's toggle). */
  enabled: boolean;
  target: string;
  custom_base_dir: string | null;
  history_days: number;
  cdp_enabled: boolean;
  cdp_port: number;
  /** R27 · how bookmark and history results are ordered: `"relevance"`,
   * `"recent"`, `"alphabetical"` or `"visits"`. Written by the plugin's own
   * settings page; the backend applies it in the search commands. */
  sort_order: string;
  /** R32 · which fields the launcher's browser search matches against. One of
   *  `"all"` / `"title"` / `"url"`; the launcher applies it in memory. */
  search_fields: BrowserSearchField;
};

/** R32 · the label each browser range filter prints. Reuses the plugin's own
 *  scope words (`launcher.browserBookmarks`/`History`) so the chips and the
 *  result grouping name the same things the same way. */
const BROWSER_FILTER_KEYS: Record<BrowserMode["kind"], MessageKey> = {
  all: "launcher.browserAll",
  bookmarks: "launcher.browserBookmarks",
  history: "launcher.browserHistory",
  tabs: "launcher.browserTabs",
};

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
  /** R29 · whether the launcher's generic plugin-configuration overlay is
   *  open. One at a time, over whatever the collapsed surface was showing. */
  const [pluginConfigOpen, setPluginConfigOpen] = useState(false);
  /** R33 · ref mirror so the once-registered plugin-request listener can tell
   *  a toggle-close from a fresh open without being rebuilt. */
  const pluginConfigOpenRef = useRef(false);
  pluginConfigOpenRef.current = pluginConfigOpen;
  const [query, setQuery] = useState("");
  /** R31 · the plugin mode the launcher is *in*, or `null` for the ordinary
   *  search page. Held as explicit state, not read back out of the query: in a
   *  mode the field shows only the needle the user typed, and the mode word is
   *  gone from the text (see `ActivePluginMode` in `launcher.ts`). The three
   *  things that write it are the input's own change handler (a trigger word +
   *  space enters), the browser system row's Enter, and Esc / Cmd+W (leave). */
  const [pluginMode, setPluginMode] = useState<ActivePluginMode | null>(null);
  // Ref mirror so the query setter below can decide whether a programmatic
  // write is an *entry* (only outside a mode) without being rebuilt per render.
  const pluginModeRef = useRef(pluginMode);
  pluginModeRef.current = pluginMode;

  /**
   * R31 · the query setter the *hooks* receive.
   *
   * A programmatic query write is one of three things, and the mode has to react
   * to two of them:
   *   · `""` — the launcher is being reset (a reveal, a return to the input, a
   *     run that closes the window). Leave the plugin mode, or a fresh summon
   *     would land back inside the browser.
   *   · a string that enters a mode — a recalled history entry like
   *     `browser rust`. Enter the mode and keep only the needle.
   *   · anything else — plain text. Write it through.
   *
   * Function updaters (the keyboard fallback's backspace/typing) are always
   * plain text: backspacing the needle to empty must NOT leave the mode
   * (「退格删空即停」), so the entry branch is deliberately skipped for them.
   * The input's own change handler does not use this setter — it runs the same
   * entry test but never clears the mode on an empty field (see the field's
   * `onChange` below).
   */
  const setQueryExitingPlugin = useCallback<React.Dispatch<React.SetStateAction<string>>>(
    (action) => {
      if (typeof action === "string") {
        if (action === "") {
          setPluginMode(null);
          setQuery("");
          return;
        }
        if (pluginModeRef.current === null) {
          const entry = pluginModeEntry(action);
          if (entry) {
            setPluginMode(entry.mode);
            setQuery(entry.needle);
            return;
          }
        }
      }
      setQuery(action);
    },
    [],
  );
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
  /** A validated `floter://register` request, waiting for the integrations
   *  panel to highlight the discovered tool. Same one-shot hand-off as the
   *  connect request above — and the same rule: highlighting is all it does. */
  const [pendingDeepLinkRegister, setPendingDeepLinkRegister] = useState<DeepLinkRegisterRequest | null>(null);
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
    changeGlassIntensity,
    changeOpacity,
    changeFontSize,
    changeUiScale,
    changeGeneralSetting,
    changeCommandAlias,
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
    setQueryExitingPlugin("");
    setTerminalMounted(false);
    setMode("collapsed");
    try {
      await invoke("show_input");
      syncLauncherHeight(collapsedCardRef, launcherHeightRef.current);
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


  // R26-D · the browser plugin's switch, readable from the once-registered
  // listeners (the hotkey / `floter://plugin-config` path) without closing over
  // the settings object that happened to be current when they were installed.
  const browserPluginEnabledRef = useRef(settings.browser_plugin.enabled);
  browserPluginEnabledRef.current = settings.browser_plugin.enabled;


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
    repaintTerminalSoon,
    terminalInputTarget,
    activeRenderer,
    surfaceReady,
    focusTerminalView,
    closeTerminalSession,
    ensureTerminalSession,
    describeMainSession,
    resetTerminalFrontendState,
    terminalResident,
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
    setQuery: setQueryExitingPlugin,
    setMode,
    showTerminalFeedback,
    t,
  });

  // R31 · the plugin requests the catalog hook fetches for, memoized by
  // (mode, query). The hook's fetch effect keys on the request's identity, so
  // an inline object literal would restart its debounce on every render and
  // starve the fetch.
  const browserMode = useMemo(() => browserModeFor(pluginMode, query), [pluginMode, query]);
  const clipboardMode = useMemo(() => clipboardModeFor(pluginMode, query), [pluginMode, query]);

  // R35 · the page-residency clock. It is the one place that answers "may the
  // app throw this surface away on its own?". Entering a surface (a plugin
  // mode, its configuration overlay, the settings panel, the terminal) starts
  // it; the reveal handler consults `holds()` before it would reset the
  // collapsed surface. `residencySurfaceNow` is the value the clock is keyed
  // on, so a move from the plugin mode to its overlay (or back) restarts the
  // window while a filter keystroke inside one mode does not. The callbacks
  // are destructured because they are the stable identities the effect keys
  // on (the hook's return object is rebuilt every render).
  const residency = useSurfaceResidency(settings.surface_residency_seconds);
  const noteResidency = residency.note;
  const residencySurfaceNow = residencySurface({ mode, pluginMode, pluginConfigOpen });
  useEffect(() => {
    noteResidency(residencySurfaceNow);
  }, [residencySurfaceNow, noteResidency, settings.surface_residency_seconds]);

  const {
    applications,
    appsLoading,
    appsError,
    appIconUrls,
    launcherResults,
    pluginView,
    pluginLoadingMore,
    loadMorePluginPage,
    actionBar,
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
    // R31 · the plugin requests the hook fetches for. In a mode the query is
    // the field's own needle; outside one both are null. Resolved above and
    // memoized so the hook never re-parses a mode word out of the text and
    // never restarts its fetch debounce on an unrelated render.
    browserMode,
    clipboardMode,
    launchCounts: settings.launch_counts,
    showCommandsInSearch: settings.show_commands_in_search,
    showRecentInLauncher: settings.show_recent_in_launcher,
    commandAliases: settings.command_aliases,
    browserEnabled: settings.browser_plugin.enabled,
    browserSearchField: settings.browser_plugin.search_fields,
    clipboardEnabled: settings.clipboard_history_enabled,
    t,
    settingsRef,
    settingsHydration,
    setSettings,
    persistSettings,
  });

  /**
   * R31 · enter a plugin mode deliberately: the browser system row's Enter, and
   * the trigger-word path in the field's own change handler below. The field is
   * emptied (its old text belonged to the ordinary search page, not to the
   * plugin), the mode becomes the scope glyph's owner, and the selection returns
   * to the top of the plugin's own list.
   */
  const enterPluginMode = useCallback((mode: ActivePluginMode) => {
    setPluginMode(mode);
    setQuery("");
    setSelectedActionBar(false);
    setSelectedResultIndex(0);
    setHistoryIndex(-1);
  }, []);

  /**
   * R33 · the unified configuration entry.
   *
   * The settings panel's Configure button and the external triggers (`floter
   * clip`, the clipboard hotkey, a cold-start request) all land here now. One
   * path, one surface: the app leaves whatever mode it was in — settings
   * included — for the launcher's collapsed state, enters the plugin's own
   * mode, and opens the generic configuration overlay on top of it. The plugin
   * page (a second, unrelated settings UI) is gone; what the user sees from
   * settings is exactly what the gear inside the mode shows.
   *
   * The three state writes are batched into one React commit, so the collapsed
   * branch renders with the mode already set and the overlay already open —
   * the overlay's host DOM exists in that same commit. No next-frame dance is
   * needed (and an effect would only add a paint of the bare mode list before
   * the overlay appears). Leaving settings for collapsed is exactly the
   * transition `closeSettings` already owns, so the window resizes back to the
   * launcher through the ordinary `show_input` path.
   */
  const openPluginConfig = useCallback((pluginId: string) => {
    // The browser plugin's configuration sits behind the plugin's own switch.
    // Refusing here — with a reason — is what keeps every trigger honest: the
    // settings row's Configure button, the global hotkey and a cold-start
    // request all funnel through this one path. The clipboard is unaffected.
    if (pluginId === BROWSER_PLUGIN_ID && !browserPluginEnabledRef.current) {
      notify("error", tRef.current("settings.browserDisabled"));
      return;
    }
    if (pluginId !== BROWSER_PLUGIN_ID && pluginId !== CLIPBOARD_PLUGIN_ID) return;
    suppressBlurUntil.current = Date.now() + 400;
    modeRef.current = "collapsed";
    setMode("collapsed");
    enterPluginMode(
      pluginId === BROWSER_PLUGIN_ID ? { scope: "browser", kind: "all" } : { scope: "clipboard" },
    );
    setPluginConfigOpen(true);
  }, [enterPluginMode]);

  /**
   * R31 · leave a plugin mode, returning to the ordinary search page.
   *
   * The field's text is *kept*: the needle the user typed becomes an ordinary
   * query, so `rust` searched inside the browser mode is still `rust` after
   * Esc — the user's own words are never thrown away by a dismissal. The mode
   * itself is what leaves, and with it the scope glyph and the plugin list.
   */
  const exitPluginMode = useCallback(() => {
    setPluginMode(null);
    setHistoryIndex(-1);
  }, []);

  /**
   * R32 · switch the browser mode's range filter. The chips and the Tab key are
   * the two callers; both end on the same state write. The mode's `kind` **is**
   * the filter, so there is no second piece of state to keep in step, and
   * leaving the mode forgets the choice exactly as it forgets the needle.
   */
  const setBrowserFilter = useCallback((kind: BrowserMode["kind"]) => {
    setPluginMode((current) =>
      current?.scope === "browser" ? { scope: "browser", kind } : current,
    );
    setSelectedActionBar(false);
    setSelectedResultIndex(0);
    setHistoryIndex(-1);
  }, []);

  /** R32 · Tab / Shift+Tab through {@link BROWSER_FILTERS}. Wraps at both ends,
   *  so the field's keyboard never falls out of the plugin on a stray Tab. */
  const cycleBrowserFilter = useCallback((direction: 1 | -1) => {
    setPluginMode((current) =>
      current?.scope === "browser"
        ? { scope: "browser", kind: nextBrowserFilter(current.kind, direction) }
        : current,
    );
    setSelectedActionBar(false);
    setSelectedResultIndex(0);
    setHistoryIndex(-1);
  }, []);

  /**
   * R32 · the empty-word way out of a plugin mode.
   *
   * R31 ruled that backspacing the needle to empty *stops* in the mode
   * (「退格删空即停」); this is the user's next step — with the field already
   * empty, another Backspace has nothing to delete and leaves the plugin. It is
   * resolved here, beside Esc / Cmd+W, so the input's own handler and the
   * window-level fallback agree. Returns whether the press was consumed.
   */
  const onPluginModeBackspace = useCallback(
    (event: KeyboardEvent): boolean => {
      if (event.key !== "Backspace" || event.metaKey || event.ctrlKey || event.altKey) {
        return false;
      }
      if (isComposing.current) return false;
      if (!pluginModeExitOnBackspace(pluginModeRef.current, query)) return false;
      event.preventDefault();
      exitPluginMode();
      return true;
    },
    [query, exitPluginMode],
  );

  /**
   * R31 · Esc / Cmd+W on the collapsed surface, resolved in one place so the
   * input's own handler and the window-level fallback cannot disagree.
   *
   * The order is the user's three-level rule:
   *   1. the plugin configuration overlay is open → close it (Esc first);
   *   2. a plugin mode owns the field → leave it, keeping the needle as an
   *      ordinary query;
   *   3. otherwise → the window (Esc hides it, as it always has; Cmd+W is the
   *      new launcher-local binding and hides it too, `preventDefault`-ing so a
   *      webview close-chord cannot fire first).
   *
   * Returns whether the press was consumed. Esc on the ordinary search page is
   * deliberately left to the existing path (`handleLauncherKey` / the dismiss
   * table), so its “clear the dropped rows, then hide” behaviour is unchanged.
   */
  const onLauncherDismiss = useCallback(
    (event: KeyboardEvent): boolean => {
      const escape = event.key === "Escape";
      const modW = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "w";
      if (!escape && !modW) return false;
      if (pluginConfigOpen) {
        event.preventDefault();
        setPluginConfigOpen(false);
        return true;
      }
      if (pluginModeRef.current) {
        event.preventDefault();
        exitPluginMode();
        return true;
      }
      if (modW) {
        event.preventDefault();
        invoke("hide_window");
        return true;
      }
      return false;
    },
    [pluginConfigOpen, exitPluginMode],
  );

  // R7-10a: the file drop listener. It owns the dropped-file state and the
  // `tauri://drag-drop` subscription; the rows and their three actions are
  // composed from that state further below. The drop itself runs nothing —
  // see `useFileDrops`, and red line 1 in `file-drops.ts`.
  const {
    droppedFiles,
    dropsExpanded,
    fileActionIndex,
    clearDrops,
    expandDrops,
    cycleFileAction,
  } = useFileDrops({
    modeRef,
    setSelectedResultIndex,
    setSelectedActionBar,
  });

  // R7-10a: the dropped files are prepended to the result list as a group of
  // their own, with no query required — dropping a file on the launcher is
  // itself the question. The rows are pure data (see `fileDropRows`); the three
  // actions live on the action bar below.
  const fileRows = useMemo(
    () => fileDropRows(droppedFiles, dropsExpanded, t),
    [droppedFiles, dropsExpanded, t],
  );
  // R10-A/R19/R36: the tail row. Nine matched results are followed by one fixed
  // row that opens the clipboard history, in every query state — empty,
  // matching, and (especially) matching nothing, which is when the clipboard is
  // the useful thing left to offer. Ten rows in all, which is the whole budget.
  // `withClipboardResultRow` keeps a query that
  // already matched the clipboard command from growing a duplicate.
  //
  // R27 · inside a plugin scope (`browser ` / `clip `) the tail is **not**
  // appended: the list is the plugin's own content, and a "Clipboard History"
  // row under a list of clipboard entries would be the panel's door inside the
  // panel. The row is a launcher-wide affordance and stays in every other
  // state, including the query that matched nothing.
  // R31 · the scope comes from the explicit mode state, not from re-parsing the
  // query. The variable keeps its name and its meaning: it is the plugin that
  // owns the field right now, or `null` on the ordinary search page.
  const launcherScope = pluginMode?.scope ?? null;
  // R29 · the plugin the current scope belongs to, for the config overlay.
  const launcherPluginId =
    launcherScope === "clipboard"
      ? CLIPBOARD_PLUGIN_ID
      : launcherScope === "browser"
        ? BROWSER_PLUGIN_ID
        : null;
  // R33 · ref mirror for the once-registered plugin-request listener: the
  // hotkey's toggle has to know whether the overlay is already open for this
  // very plugin before deciding to close it instead of opening it again.
  const launcherPluginIdRef = useRef<string | null>(null);
  launcherPluginIdRef.current = launcherPluginId;
  // Leaving the plugin scope (or the collapsed surface) closes the overlay: it
  // belongs to that plugin's field, and a stale overlay over the app list would
  // be a settings page with no owner.
  useEffect(() => {
    if (!launcherPluginId) setPluginConfigOpen(false);
  }, [launcherPluginId]);
  useEffect(() => {
    if (mode !== "collapsed") setPluginConfigOpen(false);
  }, [mode]);
  // R28 · the capability layer's view for the plugin mode the query is in, if
  // any. The text form is drawn by its own block under the field; the list form
  // is the ordinary numbered list, with the tier deciding whether its rows take
  // the keyboard.
  const pluginText = pluginView !== null && pluginView.form === "text" ? pluginView : null;
  const pluginInteractive = pluginView === null || pluginViewInteractive(pluginView);
  // R29 · the plugin list's pagination block, or null for every other list.
  const pluginPage = pluginViewPage(pluginView);
  const displayedResults = useMemo(
    () =>
      launcherScope
        ? [...launcherResults]
        : withClipboardResultRow(
            fileRows.length ? [...fileRows, ...launcherResults] : launcherResults,
            t,
            // R36 · a switched-off clipboard keeps its fixed tail row (and its
            // `⌘0`) but the row is a note, not a door.
            settings.clipboard_history_enabled,
          ),
    [fileRows, launcherResults, t, launcherScope, settings.clipboard_history_enabled],
  );

  // While the selection is on a file row the action bar describes that file's
  // three actions rather than the generic shell fallback. A file row's bar is
  // shown even with an empty query, because the drop is what put the row there.
  const selectedDroppedFile = droppedFileAt(displayedResults, selectedResultIndex);

  // The action bar is a secondary control for a visible result list. Do not
  // expand the result area for an unmatched query just because the generic
  // shell fallback exists; feedback rows remain independently visible below.
  //
  // R10-A: the gate reads the *matched* rows, not the composed list. The fixed
  // clipboard row makes `displayedResults` non-empty in every state, so asking
  // it here would silently start showing the shell fallback for a query that
  // matched nothing — the exact thing the sentence above forbids. A drop still
  // brings its own bar through the `selectedDroppedFile` branch.
  const visibleActionBar: ActionBar | null =
    selectedDroppedFile
      ? fileDropActionBar(selectedDroppedFile, fileActionIndex, t)
      : launcherResults.length > 0 || fileRows.length > 0
        ? actionBar
        : null;

  // Numbered slots and the arrow-key loop follow the composed list, not the
  // query list: a file row is a runnable result like any other.
  //
  // R30 · a plugin status line is not: it is information about the list, drawn
  // as a note rather than a row (see `launcher/plugin-mode.ts`), so the arrows
  // step over it and it never takes a number.
  const displayedRunnableFlags = useMemo(
    () =>
      displayedResults.map(
        (item) => item.type !== "status" && (item.type !== "command" || Boolean(item.execution)),
      ),
    [displayedResults],
  );
  // R34 · the scroll viewport, reported by `LauncherResults` (see the
  // `onVisibleRowsChange` prop). The numbered slots follow it, so scrolling
  // renumbers the list to what is on screen. `[0, MAX_RESULTS]` until the first
  // report: the whole list, which is exactly what a list that fits shows.
  const [visibleResultRange, setVisibleResultRange] = useState<VisibleRowRange>({
    start: 0,
    end: MAX_RESULTS,
  });
  // R10-A/R19/R36: the fixed clipboard row is the tenth and last row, and the
  // shortcut family is 1-9 plus 0, so it carries a real `⌘0` badge — the slot
  // map lives in `shortcutSlotsWithFixedTail` and nowhere else (the key handler
  // asks the same map through `resultIndexForSlot`).
  //
  // R34 · `1`-`9` now number the first nine runnable rows *inside the scroll
  // viewport*; the fixed clipboard row keeps `⌘0` outside that numbering, so
  // scrolling never moves the bottom fixed item.
  //
  // R28 · the capability layer's display tier takes every number away: a list
  // that is there to be read, not run, has no `⌘N` to offer.
  const displayedShortcutSlots = useMemo(
    () => pluginInteractive
      ? shortcutSlotsWithFixedTail(displayedResults, displayedRunnableFlags, visibleResultRange)
      : displayedResults.map(() => null),
    [displayedResults, displayedRunnableFlags, pluginInteractive, visibleResultRange],
  );

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
    setQuery: setQueryExitingPlugin,
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
    enterPluginMode,
    browserScope: launcherScope === "browser",
    cycleBrowserFilter,
    isComposing,
    actionBar,
    shortcuts,
    launcherResults: displayedResults,
    resultShortcutSlots: displayedShortcutSlots,
    runnableResultFlags: displayedRunnableFlags,
    selectedResultIndex,
    selectedActionBar,
    history,
    historyIndex,
    draftBeforeHistory,
    collapsedCardRef,
    selectedDroppedFile,
    expandDroppedFiles: expandDrops,
    cycleFileAction,
    clearDrops,
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
    launcherResults: displayedResults,
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
    runLauncherItem,
    handleLauncherKey,
    onLauncherDismiss,
    onPluginModeBackspace,
    resultShortcutSlots: displayedShortcutSlots,
    setQuery: setQueryExitingPlugin,
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
    root.setProperty("--main-opacity", String(clampWindowOpacity(settings.main_opacity) / 100));
    root.setProperty("--terminal-opacity", String(clampWindowOpacity(settings.terminal_opacity) / 100));
    // GLASS-CLIP: the CSS variables above are the slider's visual feedback and
    // stay on the per-tick path — they are what makes the glass follow the
    // thumb. The canvas repaint is deliberately NOT here. It used to be, and a
    // drag fired `updateTheme()` (a `getComputedStyle` over the whole root)
    // plus a full terminal redraw on every +1 tick, which starved the range
    // input's event handling and made the drag stutter and drop
    // (「透明度的滑杆拖动会中断」). The repaint does not depend on the slider
    // anyway: the canvas paints its background at the frame alpha and the
    // window alpha is the compositor's job, so a 47%→48% nudge changes no
    // canvas pixel. It is coalesced to one trailing call instead (see
    // `repaintTerminalSoon`), and landed immediately whenever the surface is
    // about to go away.
    repaintTerminalSoon();
  }, [settings.main_opacity, settings.terminal_opacity]);

  // GLASS-CLIP: the *flush* half of the split — landing a pending repaint
  // before the terminal surface stops being painted — deliberately lives in
  // `useTerminalView`, not here. It is the hook's renderer-lifecycle effect
  // cleanup that funnels every exit from the terminal surface (settings
  // unmounts the canvas, the launcher and plugin pages hide it, the window
  // closes), and it is the only place that can still run while `rendererRef`
  // is live: a flush scheduled from this component's `mode` effect would
  // already see a null renderer. The window-hidden paths (blur, document
  // hidden) flush through the same scheduler's listeners.

  // R7-13c: the interface-size step, written as `--ui-scale` on the document
  // root — the one knob every box dimension and type step is drawn from. It is
  // a *custom property* rather than an attribute because it is a single scalar
  // the stylesheet multiplies (`calc(var(--u) * N)`); there is nothing to key a
  // rule set off, so no `[data-scale]` attribute is needed. The value comes from
  // `uiScaleFactor` (see `ui-scale.ts`), so this effect never spells the step's
  // number.
  //
  // A *layout* effect, declared before `useLauncherHeight` below, and that
  // ordering is the contract: React runs layout effects in hook order, so this
  // write lands before the measurement reads `offsetTop`/`offsetHeight`. With
  // this as the passive `useEffect` it was, the measurement (also a layout
  // effect) would run on a step change *before* the knob moved, measure the old
  // step's pixels, and the window would sit at the previous height until some
  // other dependency happened to change.
  useLayoutEffect(() => {
    applyUiScale(document.documentElement, settings.ui_scale);
  }, [settings.ui_scale]);

  // The glass *effect* step is an attribute rather than a custom property
  // because it swaps a *set* of tokens (`[data-glass]` in base.css: the blur,
  // the saturation and the control lens scale) and because the attribute is
  // what the a11y override blocks key off. GLASS-REAXIS restored the split the
  // user asked for: this effect step and the two background-transparency
  // sliders are independent axes, so this effect never writes an opacity and
  // the opacity effect below never writes the step.
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
  //
  // Two rules keep a drop out of this effect's way:
  //   * while the query is still the empty string the drop arrived on, the drop
  //     owns the selection (it picks its first file row) and this effect leaves
  //     it alone;
  //   * `firstRunnableResultIndex` indexes the *query* list and the drop's rows
  //     are prepended to the composed list, so the default is shifted past them:
  //     typing after a drop lands on the matched result, exactly as it would
  //     with no drop at all. With no drop the shift is zero, and this is the
  //     expression it always was.
  useEffect(() => {
    const dropped = fileRows.length;
    if (!query.trim() && dropped) return;
    setSelectedResultIndex(
      firstRunnableResultIndex < 0 ? 0 : dropped + firstRunnableResultIndex,
    );
    setSelectedActionBar(defaultsToActionBar);
    // R31 · `pluginMode` is a dependency so that leaving a mode re-evaluates
    // the default selection: the ordinary results (and possibly the action bar)
    // are a different list from the plugin's, and the selection has to land on
    // the first runnable one of the new list rather than stay on an index that
    // belonged to the plugin's.
  }, [defaultsToActionBar, fileRows.length, firstRunnableResultIndex, query, pluginMode]);

  useEffect(() => {
    setSelectedResultIndex((index) => {
      if (!displayedResults.length) return 0;
      return Math.min(index, displayedResults.length - 1);
    });
  }, [displayedResults.length]);

  // R25/R34 · the launcher window is a **slab whose height is the row count**:
  // the ten-row budget is the tallest, and shorter content is exactly as tall
  // as its rows (see `resolveLauncherRows`). The step is read once here (the
  // only place that knows the interface step) and clamped to the display.
  // Nothing a keystroke does may resize it within a row count — that is the
  // whole fix for 「输入进行过滤时页面整体有抖动」 — so the value is computed from
  // the budget and the row count, and handed to the hook and to every
  // imperative sync, never measured.
  //
  // The step multiplies the *budget*, not a measurement: the card is drawn from
  // scaled CSS, so a measurement already carries the step and multiplying it
  // again would scale twice (see `useLauncherHeight`).
  const launcherScale = uiScaleFactor(settings.ui_scale);
  const launcherMaxHeight = Math.min(
    launcherWindowHeight(launcherScale),
    Math.max(240, window.screen.availHeight - 24),
  );
  // R26-D · how many rows the window has to hold right now. The composed list
  // (the fixed clipboard tail included) is the row count; the first-run tip and
  // the feedback/error rows are content too, so each is charged as one row —
  // they are at most a row tall, and counting them keeps a band from landing one
  // row short.
  // R28 · in the text form the numbered list is empty but the block still
  // occupies band rows: the capability layer reports how many
  // (`pluginViewRows`), so a long output lands in the same discrete band a list
  // of that height would.
  const launcherRows = Math.max(
    1,
    // R29 · while the plugin config overlay is open, its own row budget is the
    // window's: a header plus one row per declared field. The overlay scrolls
    // past the ten-row ceiling like every other list.
    (pluginConfigOpen && launcherPluginId
      ? 1 + (pluginConfigSchema(launcherPluginId)?.fields.length ?? 0)
      : 0) ||
      (pluginView ? pluginViewRows(pluginView) : displayedResults.length) +
        (showOnboardingTip && !launcherScope ? 1 : 0) +
        (launcherFeedback || (appsError && !launcherScope) || pendingSystemAction ? 1 : 0),
  );
  // R34 · the row count is sticky: growing is immediate (a height one row short
  // would clip the row), and shrinking waits for the count to fall a row below
  // the held count, so a query oscillating across a boundary does not resize the
  // window. See `resolveLauncherRows`.
  const launcherRowsRef = useRef(1);
  const launcherHeldRows = resolveLauncherRows(launcherRowsRef.current, launcherRows);
  launcherRowsRef.current = launcherHeldRows;
  // R27 · the height charges the action bar only when the bar is drawn, and the
  // empty-query heading only when that heading is drawn. Both are visible in the
  // state the user reported: a query that matched nothing has no action bar (the
  // shell fallback is gated on a *matched* row), so a height that always
  // reserved it left 45u of glass under the one clipboard row.
  const launcherHasBar = visibleActionBar !== null;
  // R32 · the heading is the ordinary search page's, and only its: in a plugin
  // scope the list is the plugin's own and the "Recently launched" label sat
  // over bookmark rows (the leak the user reported).
  const launcherSectionTitle = !launcherScope && !query.trim() && !fileRows.length;
  const launcherHeight = launcherRowHeight(
    launcherHeldRows,
    launcherScale,
    launcherMaxHeight,
    launcherHasBar,
    launcherSectionTitle,
    // R32 · the browser scope's range-filter subline is fixed chrome; charging
    // it here keeps the window from moving as the plugin's rows filter.
    launcherScope === "browser",
  );
  // The same number, readable by the listeners registered once for the app's
  // lifetime (the reveal path): they must not close over the step that happened
  // to be current when they were installed.
  const launcherHeightRef = useRef(launcherHeight);
  launcherHeightRef.current = launcherHeight;

  // The launcher window is the band's height now; the hook holds it there and
  // only raises it for a card that genuinely outgrew the window. Extracted into
  // useLauncherHeight hook.
  //
  // R7-13c: `settings.ui_scale` is in the dependency list on purpose. The budget
  // is expressed in scaled units, so the window height moves with the step —
  // recomputed, never multiplied onto a measurement. The knob stays out of the
  // hook itself; the trigger is here.
  useLauncherHeight(mode, collapsedCardRef, launcherHeight, [
    visibleActionBar,
    launcherFeedback,
    displayedResults.length,
    settings.ui_scale,
    // R29 · the config overlay is its own row budget; the window must resize
    // onto it in the same commit the overlay opens.
    pluginConfigOpen,
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

  // A cold start with `floter clip` records a pending plugin request in the
  // backend during setup; consume it once this component's listeners are up.
  // R33 · it lands on the plugin's configuration overlay like every other
  // external trigger.
  useEffect(() => {
    invoke<string | null>("take_pending_plugin_page")
      .then((pending) => {
        if (pending) openPluginConfig(pending);
      })
      .catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Settings is a compact work panel, not a document. Its header stays fixed
  // while the body scrolls; smaller displays get a proportional cap.
  //
  // R7-13c: the panel's *base* height scales with the interface-size step, so
  // the window grows with its type instead of showing the same column with more
  // scrolling. The screen-derived caps do not scale — they are viewport limits,
  // not layout. Re-runs when the step changes while the panel is open.
  useEffect(() => {
    if (mode !== "settings") return;
    const available = window.screen.availHeight;
    const height = Math.min(
      SETTINGS_WINDOW_HEIGHT * uiScaleFactor(settings.ui_scale),
      Math.max(SETTINGS_MIN_HEIGHT, Math.floor(available * 0.72)),
      Math.max(240, available - 24),
    );
    getCurrentWindow()
      .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, height))
      .catch(() => undefined);
  }, [mode, settings.ui_scale]);

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
            syncLauncherHeight(collapsedCardRef, launcherHeightRef.current);
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
        setQueryExitingPlugin("");
        clearDrops();
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

      // R35 · the residency window. The surface the user was on — a plugin
      // mode or its configuration overlay; settings and terminal already
      // survived above — outlives the summon for as long as the clock holds.
      // Only the *automatic* reset stands down: Esc / Cmd+W, the close button
      // and running a result each take their own path and leave the surface
      // immediately, exactly as the report requires. The clock is refreshed
      // here, because a summon back into the surface is the user confirming
      // they are still in it. A lapsed clock falls through to the ordinary
      // reset below.
      // A clock left over from a surface that has just been left (settings or
      // terminal, in the one render before the note effect clears it) must not
      // claim this branch: only the two collapsed-surface clocks may.
      const heldSurface = residency.current();
      if (
        modeRef.current === "collapsed" &&
        (heldSurface === "plugin-mode" || heldSurface === "plugin-config") &&
        residency.holds()
      ) {
        refreshApplicationsIfStale();
        restoringMode.current = "collapsed";
        setLauncherFeedback(null);
        // The drop belonged to the previous interaction; the plugin's own list
        // is what this summon restores, so the rows go with it.
        clearDrops();
        residency.refresh();
        scheduleCollapsedFocusBeats();
        window.requestAnimationFrame(() => {
          if (modeRef.current === "collapsed") {
            syncLauncherHeight(collapsedCardRef, launcherHeightRef.current);
          }
        });
        window.setTimeout(() => {
          if (
            modeRef.current === "collapsed" &&
            restoringMode.current === "collapsed"
          ) {
            syncLauncherHeight(collapsedCardRef, launcherHeightRef.current);
          }
        }, 120);
        window.setTimeout(() => {
          if (restoringMode.current === "collapsed") {
            restoringMode.current = null;
          }
        }, 160);
        return;
      }

      refreshApplicationsIfStale();

      restoringMode.current = "collapsed";
      setLauncherFeedback(null);
      setQueryExitingPlugin("");
      // A summon is a fresh launcher. A drop belonged to the previous
      // interaction, so its rows go with the reveal that starts the next one —
      // the same place (and the same reason) the query is emptied.
      clearDrops();
      setTerminalMounted(false);
      setMode("collapsed");
      scheduleCollapsedFocusBeats();
      // A hidden launcher can retain the height of its previous result list;
      // when it is revealed the mode may already be "collapsed", so the
      // mode effect will not run again. Measure after the state commit to
      // restore the compact window before it is painted with stale space.
      window.requestAnimationFrame(() => {
        if (modeRef.current === "collapsed") {
          syncLauncherHeight(collapsedCardRef, launcherHeightRef.current);
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
          syncLauncherHeight(collapsedCardRef, launcherHeightRef.current);
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

  // A plugin request. One internal path serves every trigger: the global
  // hotkey, `floter clip` (running instance or cold start) and the settings
  // panel's Configure button. `toggle` says the window was already visible when
  // the hotkey went down: only then does pressing it again mean "close" — and
  // only for the very overlay that is already open; any other request opens.
  //
  // R33 · the toggle used to hide the window when the plugin *page* was up. The
  // page is gone, so its open/close meaning now maps onto the configuration
  // overlay: a second press closes the overlay (the plugin mode underneath
  // stays), it does not hide the whole panel.
  useEffect(() => {
    const unlistenConfigPromise = listen<{ id: string; toggle: boolean }>("floter://plugin-config", (event) => {
      const { id, toggle } = event.payload;
      if (toggle && pluginConfigOpenRef.current && launcherPluginIdRef.current === id) {
        setPluginConfigOpen(false);
        return;
      }
      openPluginConfig(id);
    });

    return () => {
      unlistenConfigPromise.then((unlisten) => unlisten());
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

  // The `floter://` scheme's outcomes. The backend owns the allow-list and
  // the validation; here a *validated* connect request becomes the review
  // dialog, a *validated* register request highlights the discovered tool on
  // the same review surface, and a refusal becomes one toast. None of the
  // paths installs or binds anything: the dialog's Connect button runs the
  // ordinary `extensions_install` / `extensions_connect_tool`, so a link can
  // never approve, enable or bind on the user's behalf.
  useEffect(() => {
    const openReview = (request: DeepLinkConnectRequest) => {
      setPendingDeepLink(request);
      openSettings("integrations");
    };
    const unlistenConnectPromise = listen<DeepLinkConnectRequest>(
      DEEP_LINK_CONNECT_EVENT,
      (event) => openReview(event.payload),
    );
    // A register request lands on the same page but on the Detected surface:
    // the tool is only highlighted, and the user's Connect press is still the
    // one action that binds it.
    const highlightRegistered = (request: DeepLinkRegisterRequest) => {
      setPendingDeepLinkRegister(request);
      openSettings("integrations");
    };
    const unlistenRegisterPromise = listen<DeepLinkRegisterRequest>(
      DEEP_LINK_REGISTER_EVENT,
      (event) => highlightRegistered(event.payload),
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
    // The same once-only consumption for a cold-start `floter register …`.
    invoke<DeepLinkRegisterRequest | null>("take_pending_deep_link_register")
      .then((pending) => {
        if (pending) highlightRegistered(pending);
      })
      .catch(() => undefined);
    return () => {
      unlistenConnectPromise.then((unlisten) => unlisten());
      unlistenRegisterPromise.then((unlisten) => unlisten());
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

  /**
   * The platform drag itself, with no opinion about *what* asked for it. Every
   * entry point funnels here — a mousedown on a shell's chrome, and a drag
   * message from a plugin page (whose iframe cannot start a native drag and
   * reports the intent over the bridge instead). Keeping the body in one place
   * is what keeps the Windows blur-grace applied to both: the modal move loop
   * leaves the webview unfocused and would otherwise read as the user leaving.
   */
  const beginDrag = useCallback(() => {
    if (!IS_WINDOWS) {
      void invoke("start_drag");
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
  }, []);

  const startDrag = (event: React.MouseEvent) => {
    if ((event.target as HTMLElement).closest("button, input, select, textarea, a, summary, [role='dialog'], [data-no-drag]")) {
      return;
    }
    event.preventDefault();
    beginDrag();
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

  const onInputKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    // R31 · Esc / Cmd+W are resolved above `handleLauncherKey`, because the
    // mode-aware levels (close the config overlay, leave the plugin mode) are
    // the collapsed surface's business rather than the field's key grammar.
    if (onLauncherDismiss(event.nativeEvent)) return;
    // R32 · an already-empty field's Backspace leaves the plugin mode; a
    // Backspace that still has a character to delete is left to the field.
    if (onPluginModeBackspace(event.nativeEvent)) return;
    handleLauncherKey(event.nativeEvent);
  };

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
      // R10-B: only the collapsed launcher tags the card "launcher" — its
      // header becomes the implicit, hover-revealed drag band there. The
      // terminal page keeps the always-visible header it has today.
      variant={mode === "collapsed" ? "launcher" : "terminal"}
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

  // The toast host is rendered once, as a stable sibling of the mode shell, so
  // the stack survives mode switches without unmounting: a toast raised in the
  // integrations panel stays visible when the window flips to the launcher or
  // the terminal, and its dismiss timer is never cut
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
                onChangeUiScale={changeUiScale}
                onChangeOpacity={changeOpacity}
                onChangeGlassIntensity={changeGlassIntensity}
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
                commandAliases={settings.command_aliases}
                onChangeCommandAlias={changeCommandAlias}
                basePlugins={BUILTIN_BASE_PLUGINS.map((plugin) => ({
                  ...plugin,
                  // Each base plugin's switch reads its own persisted field: the
                  // clipboard's long-standing `clipboard_history_enabled` and
                  // the browser's `browser_plugin.enabled`. A plugin without a
                  // switch (`toggleable: false`) is reported available.
                  enabled: plugin.toggleable
                    ? plugin.id === BROWSER_PLUGIN_ID
                      ? settings.browser_plugin.enabled
                      : settings.clipboard_history_enabled
                    : true,
                }))}
                onToggleBasePlugin={(id, enabled) => {
                  if (id === CLIPBOARD_PLUGIN_ID) {
                    changeGeneralSetting("clipboard_history_enabled", enabled);
                    return;
                  }
                  if (id === BROWSER_PLUGIN_ID) {
                    // The switch is one field of the plugin's own block, so the
                    // write carries the rest of the block forward unchanged.
                    changeGeneralSetting("browser_plugin", {
                      ...settings.browser_plugin,
                      enabled,
                    });
                  }
                }}
                onOpenPluginConfig={(id) => openPluginConfig(id)}
                onNotify={notify}
                pendingDeepLink={pendingDeepLink}
                onDeepLinkConsumed={() => setPendingDeepLink(null)}
                pendingDeepLinkRegister={pendingDeepLinkRegister}
                onDeepLinkRegisterConsumed={() => setPendingDeepLinkRegister(null)}
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

  if (mode === "collapsed") {
    const hasQuery = query.trim().length > 0;
    // The first scan runs before there is anything to search, so the input says
    // so rather than inviting a query that would match nothing.
    const placeholder = appsError
      ? t("input.scanFailed")
      : launcherScope === "clipboard"
        ? t("input.placeholderClipboard")
        : launcherScope === "browser"
          ? t("input.placeholderBrowser")
          : appsLoading && !applications.length
            ? t("input.scanning")
            : t("input.placeholder");

    return (
      <>
        {toastHost}
        <div className="collapsed-shell">
          {pinnedCardElement}
          <div
            ref={collapsedCardRef}
            /* R18: the seam's gate class is gone with the seam. It existed only
               to light the divider between the field and the list, and nothing
               may take its place: gating a *layout* metric (the search
               surface's breath) on the result count is exactly the 4px jump
               R15 removed from the action bar. */
            className={`collapsed-card${hasQuery ? " collapsed-card--filled" : ""}`}
            style={{ "--launcher-results-height": `${Math.max(84, window.screen.availHeight - RESULTS_VIEWPORT_CHROME)}px` } as React.CSSProperties}
            onClick={(event) => {
              if (!(event.target as HTMLElement).closest("button, input, select, .plugin-config")) focusCollapsedInput();
            }}
          >
            {/* R10-B: the launcher's implicit drag handle. The card itself is
                no longer a drag surface — a press on a result row or a hint
                must never move the window — so the one place that does is an
                invisible 28px band across the card's top, the same implicit
                handle the clipboard page uses. It is the input row's first
                child and paints under the field and the buttons (both already
                `z-index: 1`), so the field keeps every one of its clicks. */}
            <div className="collapsed-card__input-row">
              <div
                className="collapsed-card__drag-zone"
                aria-hidden="true"
                onMouseDown={startDrag}
              />
              {/* R15: the focus aura that used to sit here — a full-width
                  accent wash over the input row — is retired. Focus is the
                  caret and the accent seam on the row's floor (see
                  `styles/launcher.css`); a second accent field under the same
                  row was the shadow the user kept asking to be lighter. */}
              {/* R27 · the plugin scope glyph. In a plugin mode the field is
                  the plugin's search box, and the glyph is what says so — the
                  same "one small icon at the left of the field" Raycast and
                  tinycast use to mark the current scope. It is furniture, not
                  a control: no listener, `aria-hidden`, and the placeholder
                  and `aria-label` carry the meaning. Outside a mode it is not
                  rendered at all, so the field's own left edge is unchanged. */}
              {launcherScope && (
                <span className="collapsed-card__scope" aria-hidden="true">
                  {launcherScope === "clipboard" ? (
                    <ClipboardIcon size={16} strokeWidth={1.8} />
                  ) : (
                    <GlobeIcon size={16} strokeWidth={1.8} />
                  )}
                  {/* R29 · the scope's *name*, not just its glyph. The icon
                      says a plugin owns the field; the muted word says which
                      one, which is what a launcher with more than one plugin
                      needs and what the placeholder alone could not carry. */}
                  <span className="collapsed-card__scope-name">
                    {t(
                      launcherScope === "clipboard"
                        ? "launcher.scopeClipboard"
                        : "launcher.scopeBrowser",
                    )}
                  </span>
                </span>
              )}
              <input
                ref={inputRef}
                className="collapsed-card__input"
                role="combobox"
                aria-label={placeholder}
                aria-autocomplete="list"
                aria-expanded={displayedResults.length > 0}
                aria-controls={displayedResults.length > 0 ? "launcher-options" : undefined}
                aria-activedescendant={
                  selectedActionBar && visibleActionBar
                    ? "launcher-option-action"
                    : displayedResults[selectedResultIndex]
                      ? `launcher-option-${selectedResultIndex}`
                      : undefined
                }
                value={query}
                onChange={(event) => {
                  setLauncherFeedback(null);
                  const value = event.target.value;
                  // R31 · entering a mode is a *transition out of the ordinary
                  // search page*: the trigger word plus a space strips the word
                  // and keeps the needle, and the mode becomes state. It only
                  // runs while no mode is active — once inside one, the field's
                  // text is the plugin's needle and a word that happens to look
                  // like a trigger is just a query. An empty field does NOT
                  // leave the mode (「退格删空即停」); Esc / Cmd+W is the way out.
                  if (pluginModeRef.current === null) {
                    const entry = pluginModeEntry(value);
                    if (entry) {
                      setPluginMode(entry.mode);
                      setQuery(entry.needle);
                      setHistoryIndex(-1);
                      return;
                    }
                  }
                  setQuery(value);
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
              {launcherScope ? (
                // R29 · in a plugin mode the field is the plugin's search box,
                // so the row's trailing control is the *plugin's*: one gear
                // that opens its generic configuration overlay. The sessions
                // entry (the terminal list) is not a plugin action and is gone
                // here; outside a mode the two buttons are exactly as before.
                <button
                  type="button"
                  className="collapsed-card__settings collapsed-card__settings--plugin"
                  aria-label={t("plugins.config.open")}
                  title={t("plugins.config.openHint")}
                  aria-expanded={pluginConfigOpen}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                  }}
                  onClick={(event) => {
                    event.stopPropagation();
                    setPluginConfigOpen((open) => !open);
                  }}
                >
                  <SlidersHorizontal size={16} strokeWidth={1.8} aria-hidden="true" />
                </button>
              ) : (
                <>
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
                </>
              )}
            </div>
            {/* R32 · the browser mode's range filter. A compact subline under
                the field, present for the whole browser scope (including the
                debounce before the first rows arrive) so the chips never
                flicker in and out. It is chrome, not a result: no `⌘N` slot,
                no arrow-key stop. Tab cycles it (see `handleLauncherKey`),
                and a click sets it — `tabIndex={-1}` keeps the field the one
                keyboard owner. Muted text, the active chip underlined and
                heavier, so the row spends no accent budget. */}
            {launcherScope === "browser" && (
              <div className="launcher-filter">
                <div
                  className="launcher-filter__chips"
                  role="tablist"
                  aria-label={t("launcher.browserFilter")}
                >
                  {BROWSER_FILTERS.map((kind) => (
                    <button
                      key={kind}
                      type="button"
                      role="tab"
                      tabIndex={-1}
                      aria-selected={browserMode?.kind === kind}
                      className={
                        browserMode?.kind === kind
                          ? "launcher-filter__chip launcher-filter__chip--active"
                          : "launcher-filter__chip"
                      }
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={(event) => {
                        event.stopPropagation();
                        setBrowserFilter(kind);
                      }}
                    >
                      {t(BROWSER_FILTER_KEYS[kind])}
                    </button>
                  ))}
                </div>
                {/* R35 · the quiet affordance that names the key which cycles
                    the filter. Tab is invisible: the chips are not in the tab
                    order (the field owns the keyboard), so nothing else tells
                    the user the row is reachable without a mouse. Caption-size
                    `--text-tertiary` keeps it below even an inactive chip, so
                    it adds no emphasis and spends no accent. */}
                <span className="launcher-filter__hint">
                  {t("launcher.browserFilterHint")}
                </span>
              </div>
            )}
            {/* First-run onboarding tip: a small dismissible banner shown above
                the result area the first time the user opens the launcher. */}
            {showOnboardingTip && !launcherScope && (
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
            {pluginConfigOpen && launcherPluginId ? (
              // R29 · the plugin's configuration, rendered from its declarative
              // schema by the generic controls. It takes the result area's place
              // rather than opening a document: the launcher stays the surface,
              // and the field above stays the plugin's own search box.
              <PluginConfigOverlay
                pluginId={launcherPluginId}
                t={t}
                clipboardEnabled={settings.clipboard_history_enabled}
                onChangeGeneralSetting={(key, value) => changeGeneralSetting(key, value)}
                onBrowserSettingsChange={(block) => changeGeneralSetting("browser_plugin", block)}
                onClose={() => setPluginConfigOpen(false)}
              />
            ) : (
            <div
              className={
                displayedResults.length > 0 || pluginText !== null || launcherFeedback || appsError
                  ? "launcher-bottom-clip launcher-bottom-clip--open"
                  : "launcher-bottom-clip"
              }
            >
              <div className="launcher-bottom">
                {appsError && !launcherScope && (
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
                {pluginText ? (
                  // R28 · the text form: the plugin's output printed under the
                  // field, in place of the numbered list.
                  <PluginTextView t={t} text={pluginText.text} metrics={pluginText.metrics} />
                ) : (
                  <LauncherResults
                    t={t}
                    results={displayedResults}
                    actionBar={visibleActionBar}
                    appIconUrls={appIconUrls}
                    selectedResultIndex={selectedResultIndex}
                    selectedActionBar={selectedActionBar}
                    resultShortcutSlots={displayedShortcutSlots}
                    actionBarShortcut={actionBarShortcut}
                    selectResultShortcut={shortcuts.select_result}
                    // R28 · the capability layer's list tier. A display-only list
                    // keeps its rows but none of the keyboard: no highlight, no
                    // pointer selection, no Enter.
                    interactive={pluginInteractive}
                    // R29 · the plugin list's pagination. `pluginViewPage` is
                    // null for every non-plugin list, so this changes nothing
                    // outside a plugin mode; inside one it turns the list into
                    // a scroller that asks for the next page at the bottom.
                    pluginPage={pluginPage}
                    pluginLoadingMore={pluginLoadingMore}
                    onLoadMore={loadMorePluginPage}
                    // The drop group brings its own heading (emitted above the
                    // first file row), and it sits *above* the recent apps. The
                    // "Recently launched" heading renders before the whole list,
                    // so leaving it on would put a label over the wrong rows.
                    showRecentTitle={launcherSectionTitle}
                    onSelectResult={(index) => {
                      setSelectedActionBar(false);
                      setSelectedResultIndex(index);
                    }}
                    onSelectActionBar={() => setSelectedActionBar(true)}
                    // R34 · the scroller tells App which rows are on screen so
                    // the `⌘N` slots can follow it; the setter is stable, so
                    // the report only fires when the range really changes.
                    onVisibleRowsChange={setVisibleResultRange}
                    onRunResult={runLauncherItem}
                    onRunActionBar={() => {
                      if (visibleActionBar) executeActionBar(visibleActionBar);
                    }}
                  />
                )}
                {launcherResults.length === 0 &&
                  fileRows.length === 0 &&
                  !actionBar &&
                  !query.trim() &&
                  !launcherScope &&
                  !settings.show_commands_in_search && (
                    <div className="launcher-hint" role="status">
                      {t("launcher.enableIntegrationsHint")}
                    </div>
                  )}
              </div>
            </div>
            )}
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
            {/* R9-2 slice 5 · the PTY child exited and the page is being
                *held* rather than collapsed: the final frame stays painted so
                the output is readable, and this line names the exit code and
                the way out. A non-zero code reads in the warning colour; a
                clean exit in the ordinary secondary text. Closing is the
                user's decision, made with the header × or the new-command
                shortcut. */}
            {terminalResident && (
              <div
                className={`terminal-resident${terminalResident.code !== null && terminalResident.code !== 0 ? " terminal-resident--warning" : ""}`}
                role="status"
                aria-live="polite"
              >
                <span className="terminal-resident__title">
                  {t("terminal.processExited", { code: terminalResident.code ?? 0 })}
                </span>
                <span className="terminal-resident__hint">{t("terminal.processExitedHint")}</span>
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
