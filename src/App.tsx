import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
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
  Play,
  RefreshCw,
  SlidersHorizontal,
  SquareTerminal,
  Trash2,
} from "lucide-react";
import { TerminalCanvas, decodeFrame, type CellPoint, type Selection } from "./terminal/render";
import { encodeKey, FOCUS_IN_OUT, MOUSE_MOTION, usesMouseReporting } from "./terminal/input";
import {
  createTranslator,
  normalizeLanguage,
  LANGUAGE_OPTIONS,
  type Language,
  type MessageKey,
  type Translate,
} from "./i18n";
import { ExtensionsPanel, type ExtensionExecutionPlan } from "./ExtensionsPanel";
import {
  classifyActionBar,
  completedCommandLine,
  executionWithCompletion,
  normalizeSearch,
  parseCommandLine,
  scoreApp,
  shouldDefaultToActionBar,
  type ActionBarKind,
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
  shortcutFromEvent,
  withShortcutDefaults,
  type ShortcutAction,
  type ShortcutMap,
} from "./shortcuts";
import "./App.css";

if (IS_WINDOWS) {
  document.documentElement.classList.add("platform-windows");
} else if (IS_MAC) {
  document.documentElement.classList.add("platform-macos");
} else if (IS_LINUX) {
  document.documentElement.classList.add("platform-linux");
}

type ViewMode = "collapsed" | "terminal" | "settings";
type SettingsPage = "general" | "shortcuts" | "sessions" | "integrations" | "about";
type ExternalTerminalOutcome = { session_handed_off: boolean };

type BrokerSessionInfo = {
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

type LocalApplication = {
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

type SystemAction = "restart" | "shutdown";

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

type LauncherItem =
  | { type: "app"; id: string; title: string; subtitle: string; app: LocalApplication }
  | {
      type: "command";
      id: string;
      title: string;
      subtitle: string;
      sourceName: string;
      commandLine: string;
      execution: ExecutionPlan | null;
      completion: boolean;
    }
  | { type: "system"; id: string; title: string; subtitle: string; action: SystemAction };

/**
 * What the query does when it is not the name of anything installed.
 *
 * This is the row below the results, and unlike them there is always exactly one
 * of it for a non-empty query: every string is *something* the shell can be
 * handed, and a few shapes of string are better answered by the browser or the
 * file manager instead.
 */
type ActionBar = { type: ActionBarKind; label: string; value: string };

type AppSettings = {
  hotkey: string;
  hide_on_blur: boolean;
  launch_at_startup: boolean;
  theme: string;
  font_size: number;
  font_family: string;
  cursor_shape: string;
  language: Language;
  main_opacity: number;
  terminal_opacity: number;
  shortcuts: ShortcutMap;
};

/**
 * The appearance choices, in the order they are offered.
 *
 * `auto` first because it is the default and the one most people want. Unlike
 * `LANGUAGE_OPTIONS` these live here rather than in i18n.ts: a language is
 * always named in itself ("English", "中文"), while a theme name is ordinary
 * prose that has to be translated, so there is nothing to keep beside the
 * dictionaries.
 */
const THEME_OPTIONS: { value: string; labelKey: MessageKey }[] = [
  { value: "auto", labelKey: "settings.theme.auto" },
  { value: "dark", labelKey: "settings.theme.dark" },
  { value: "light", labelKey: "settings.theme.light" },
];

const MIN_OPACITY = 10;
const MAX_OPACITY = 100;
const OPACITY_PRESETS = [25, 50, 75, 100];
const OPACITY_SNAP_DISTANCE = 2;

const normalizeOpacity = (value: number): number => {
  const safeValue = Number.isFinite(value) ? value : MAX_OPACITY;
  const clamped = Math.round(Math.min(MAX_OPACITY, Math.max(MIN_OPACITY, safeValue)));
  return OPACITY_PRESETS.find((preset) => Math.abs(preset - clamped) <= OPACITY_SNAP_DISTANCE)
    ?? clamped;
};

const formatBytes = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const value = bytes / 1024 ** index;
  return `${index === 0 ? Math.round(value) : value.toFixed(1)} ${units[index]}`;
};

const FONT_FAMILY =
  "'SF Mono','Menlo','Monaco','Consolas','JetBrains Mono',monospace";
const FONT_SIZE = 13;
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
];

// Where an application came from, read off the shape of its path: `.app`
// bundles on macOS, `.desktop` entries on Linux, Start Menu shortcuts on
// Windows.
const appSubtitleKey = (path: string): MessageKey => {
  if (IS_MAC) {
    if (path.startsWith("/Applications/")) return "launcher.application";
    if (path.startsWith("/System/Applications/")) return "launcher.systemApplication";
    if (path.includes("/Applications/")) return "launcher.userApplication";
    return "launcher.application";
  }
  if (/^([A-Za-z]:)?[\\/]Users[\\/]/.test(path)) return "launcher.userApplication";
  if (path.startsWith("/home/") || path.startsWith("/root/")) return "launcher.userApplication";
  if (/^\/(usr|opt|var)\//.test(path)) return "launcher.systemApplication";
  return "launcher.application";
};

/** Lucide `rotate-cw` for restart, `power` for shutdown. */
const SystemActionIcon = ({ action }: { action: "restart" | "shutdown" }) => (
  <svg
    viewBox="0 0 24 24"
    width="16"
    height="16"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    {action === "restart" ? (
      <>
        <path d="M21 12a9 9 0 1 1-2.64-6.36" />
        <path d="M21 3v6h-6" />
      </>
    ) : (
      <>
        <path d="M12 2v10" />
        <path d="M18.4 6.6a9 9 0 1 1-12.77.04" />
      </>
    )}
  </svg>
);

/**
 * The action bar's icon: Lucide `external-link` for a URL, `folder` for a path,
 * and a shell prompt for everything else.
 *
 * The `$` is a glyph rather than Lucide's `terminal` because it is what the row
 * below it in the terminal will actually say, and it reads as "a command line"
 * to anyone who has ever seen one.
 */
const ActionBarIcon = ({ kind }: { kind: ActionBarKind }) => {
  if (kind === "shell") return <span>$</span>;
  return (
    <svg
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {kind === "url" ? (
        <>
          <path d="M15 3h6v6" />
          <path d="M10 14 21 3" />
          <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        </>
      ) : (
        <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
      )}
    </svg>
  );
};

type OpacityControlProps = {
  label: string;
  value: number;
  onChange: (value: number) => void;
};

function OpacityControl({ label, value, onChange }: OpacityControlProps) {
  return (
    <div className="opacity-control">
      <div className="opacity-control__header">
        <label className="opacity-control__label">{label}</label>
        <output className="opacity-control__value">{value}%</output>
      </div>
      <input
        className="opacity-control__range"
        type="range"
        min={MIN_OPACITY}
        max={MAX_OPACITY}
        step="1"
        value={value}
        aria-label={label}
        onChange={(event) => onChange(normalizeOpacity(Number(event.currentTarget.value)))}
      />
      <div className="opacity-control__presets" aria-label={label}>
        {OPACITY_PRESETS.map((preset) => (
          <button
            key={preset}
            type="button"
            className={`opacity-control__preset${value === preset ? " opacity-control__preset--active" : ""}`}
            aria-pressed={value === preset}
            onClick={() => onChange(preset)}
          >
            {preset}%
          </button>
        ))}
      </div>
    </div>
  );
}

type ShortcutRecorderProps = {
  action: ShortcutAction;
  shortcut: string;
  recording: boolean;
  onToggle: (action: ShortcutAction) => void;
  onCapture: (action: ShortcutAction, shortcut: string) => void;
  onCancel: () => void;
  t: Translate;
};

/**
 * A single rebindable shortcut.
 *
 * While recording it owns the keyboard: the listener runs in the capture phase
 * and stops propagation, so neither the app's own handler nor the browser sees
 * the combination being pressed. A press without any modifier is ignored —
 * binding a bare letter would swallow it everywhere in the app.
 */
function ShortcutRecorder({
  action,
  shortcut,
  recording,
  onToggle,
  onCapture,
  onCancel,
  t,
}: ShortcutRecorderProps) {
  // Windows swallows Alt+Space so its window menu never opens over the panel,
  // which also keeps the combination from ever reaching this recorder. The flag
  // lifts that for as long as one is listening; the cleanup runs on capture, on
  // cancel and on unmount, so it cannot be left raised. Kept in its own effect,
  // keyed on nothing but `recording`, so a re-render of the settings panel does
  // not lower and raise it again mid-recording.
  useEffect(() => {
    if (!IS_WINDOWS || !recording) return;
    invoke("set_recording_flag", { on: true }).catch(() => undefined);
    return () => {
      invoke("set_recording_flag", { on: false }).catch(() => undefined);
    };
  }, [recording]);

  useEffect(() => {
    if (!recording) return;

    const onKeyDown = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();

      const bare = !event.ctrlKey && !event.altKey && !event.shiftKey && !event.metaKey;
      if (event.key === "Escape" && bare) {
        onCancel();
        return;
      }
      const next = shortcutFromEvent(event);
      if (!next) return;
      if (bare && !/^F\d{1,2}$/.test(next)) return;
      onCapture(action, next);
    };

    // If the window loses focus while recording (user clicked away, Cmd-Tab,
    // etc.), cancel immediately and restore shortcuts. Otherwise the global
    // shortcuts stay suspended and the user's hotkey is dead.
    const onBlur = () => onCancel();

    window.addEventListener("keydown", onKeyDown, true);
    window.addEventListener("blur", onBlur);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
      window.removeEventListener("blur", onBlur);
    };
  }, [action, onCancel, onCapture, recording]);

  return (
    <button
      type="button"
      className={`shortcut-recorder${recording ? " shortcut-recorder--recording" : ""}`}
      aria-label={t("settings.shortcut.record")}
      title={t("settings.shortcut.record")}
      onMouseDown={(event) => event.preventDefault()}
      onClick={() => onToggle(action)}
    >
      {recording ? (
        <span className="shortcut-recorder__prompt">{t("settings.shortcut.recording")}</span>
      ) : (
        <span className="shortcut-recorder__keys">{formatShortcut(shortcut)}</span>
      )}
    </button>
  );
}


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
  const opacitySaveTimer = useRef<number | null>(null);
  const clickSeq = useRef({ count: 0, time: 0, col: -1, row: -1 });

  const ptyReady = useRef(false);
  const terminalGeneration = useRef<number | null>(null);
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
  const [settingsPage, setSettingsPage] = useState<SettingsPage>("general");
  const [query, setQuery] = useState("");
  const [terminalMounted, setTerminalMounted] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [applications, setApplications] = useState<LocalApplication[]>([]);
  /** True until the first scan settles, whether it hit the cache or not. */
  const [appsLoading, setAppsLoading] = useState(true);
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
  const [sessionActionId, setSessionActionId] = useState<string | null>(null);
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
  });
  const settingsRef = useRef(settings);
  useEffect(() => { settingsRef.current = settings; }, [settings]);
  const [recordingAction, setRecordingAction] = useState<ShortcutAction | null>(null);
  const [rejectedAction, setRejectedAction] = useState<ShortcutAction | null>(null);
  const [autostartUpdating, setAutostartUpdating] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<{ version: string } | null>(null);
  const [updateDownloading, setUpdateDownloading] = useState(false);
  const [updateProgress, setUpdateProgress] = useState<{ downloaded: number; total: number } | null>(null);
  const [updateFailed, setUpdateFailed] = useState(false);
  const [launcherFeedback, setLauncherFeedback] = useState<MessageKey | null>(null);
  const [terminalFeedback, setTerminalFeedback] = useState<MessageKey | null>(null);
  const isComposing = useRef(false);
  const suppressBlurUntil = useRef(0);

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
  const resolvedTheme = settings.theme === "auto" ? systemTheme : settings.theme;

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
  useEffect(() => {
    const value = query.trim();
    const generation = ++catalogRequestGeneration.current;
    if (!value) {
      setCatalogSuggestions([]);
      return;
    }

    setCatalogSuggestions([]);
    const timer = window.setTimeout(() => {
      const searchTokens = parseCommandLine(query).tokens;
      const completionLine = parseCommandLine(query, true);
      const command = completionLine.tokens[0] ?? "";
      const wantsCompletion = completionLine.tokens.length > 1;
      const search = invoke<CatalogEntry[]>("catalog_search", {
        request: {
          query,
          tokens: searchTokens,
          cwd: null,
          limit: 20,
          includeSystemCommands: true,
        },
      });
      const complete = wantsCompletion
        ? invoke<CatalogCompletionResponse>("catalog_complete", {
            request: {
              command,
              tokens: completionLine.tokens,
              cwd: null,
            },
          }).catch(() => null)
        : Promise.resolve<CatalogCompletionResponse | null>(null);

      Promise.all([search, complete])
        .then(([entries, completion]) => {
          if (catalogRequestGeneration.current !== generation) return;
          const commands = entries.filter((entry) => entry.sourceKind !== "systemApplication");
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
              commandLine: completedCommandLine(query, completionLine.fragmentStart, item),
              execution: executionWithCompletion(exact, completionLine.tokens, item),
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
  }, [query]);

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
    if (!command) return [];

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
        const unavailable = !entry.runtimeAvailable
          ? ` · ${t("extensions.runtimeUnavailable")}`
          : "";
        const conflict = (commandCounts.get(entry.command) ?? 0) > 1
          ? ` · ${t("extensions.conflict")}`
          : "";
        if (suggestion.kind === "completion") {
          const dynamic = suggestion.dynamic
            ? ` · ${t("extensions.dynamicCompletion")}`
            : "";
          return {
            type: "command",
            id: `${entry.id}:completion:${suggestion.completion.value}`,
            title: suggestion.completion.label,
            subtitle: `${suggestion.completion.description}${dynamic}${unavailable}`,
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
          subtitle: `${entry.description}${conflict}${unavailable}`,
          sourceName: entry.sourceName,
          commandLine: parseCommandLine(query).tokens.length > 1 ? query : `${entry.command} `,
          execution: entry.execution,
          completion: false,
        };
      });

    // The action bar occupies the final row. Keep at least one local match when
    // applications or power actions matched alongside catalog commands.
    return [...commandItems, ...rankedMatches].slice(0, MAX_RESULTS - 1);
  }, [catalogSuggestions, query, searchableApps, t]);

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

  const hasCommandResult = launcherResults.some((item) => item.type === "command");

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
      hasCommandResult,
    );
  }, [actionBar, hasCommandResult, launcherResults.length, query]);

  // A full scan walks every application directory, so the two callers below
  // share one: the initial load and a refresh after a summon must never end up
  // running at the same time.
  const scanApplications = (forceRefresh: boolean) => {
    if (appScanning.current) return;
    appScanning.current = true;
    invoke<LocalApplication[]>("list_applications", { forceRefresh })
      .then(setApplications)
      .catch(() => undefined)
      .finally(() => {
        appScanning.current = false;
        setAppsLoading(false);
      });
  };

  const refreshTerminalSessions = () => {
    setSessionsLoading(true);
    return invoke<BrokerSessionInfo[]>("term_list_sessions")
      .then(setTerminalSessions)
      .catch(() => setTerminalSessions([]))
      .finally(() => setSessionsLoading(false));
  };

  useEffect(() => {
    scanApplications(false);
  }, []);

  // A new query starts from its own default: the first result for a name, the
  // action bar for a command line, a URL or a path. See `defaultsToActionBar`.
  useEffect(() => {
    setSelectedResultIndex(0);
    setSelectedActionBar(defaultsToActionBar);
  }, [defaultsToActionBar, query]);

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

  const render = () => {
    const renderer = rendererRef.current;
    const frame = frameRef.current;
    if (renderer && frame) {
      renderer.draw(frame, blinkRef.current, selectionRef.current);
    }
  };

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
      canvasRef.current?.focus();
    }, delay);
  };

  const relayoutAndResize = () => {
    const renderer = rendererRef.current;
    const mount = mountRef.current;
    if (!renderer || !mount) return;
    const rect = mount.getBoundingClientRect();
    const layout = renderer.relayout(rect.width, rect.height);
    dimsRef.current = layout;
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
      await invoke("term_spawn", {
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

  useEffect(() => {
    invoke<AppSettings>("get_settings")
      .then((loaded) =>
        setSettings({
          ...loaded,
          language: normalizeLanguage(loaded.language),
          launch_at_startup: loaded.launch_at_startup ?? false,
          main_opacity: normalizeOpacity(loaded.main_opacity ?? 94),
          terminal_opacity: normalizeOpacity(loaded.terminal_opacity ?? 92),
          shortcuts: withShortcutDefaults(loaded.shortcuts),
        }),
      )
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    invoke<string>("app_version")
      .then(setAppVersion)
      .catch(() => undefined);
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

  useEffect(() => {
    if (!terminalMounted) {
      termOpened.current = false;
      rendererRef.current = null;
      return;
    }
    if (!canvasRef.current || !mountRef.current || termOpened.current) return;

    const renderer = new TerminalCanvas(canvasRef.current, {
      fontFamily: FONT_FAMILY,
      fontSize: FONT_SIZE,
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
  }, [terminalMounted]);

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

    if (mode === "collapsed") {
      if (!isRestoring) {
        // `show_input` resizes to the bare input row, which is the right height
        // for an empty query and a couple of pixels short of one with results
        // (or of a card that draws a border). It lands after the layout effect
        // above, so the measured height is applied again once it has.
        invoke("show_input")
          .then(() => {
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

  // The launcher's keyboard belongs to its input, and nothing else in the card
  // has any use for it: the result rows and the settings button all decline
  // focus on mousedown, so anything that takes it away — a click landing on the
  // card itself, an element unmounting under the caret, a reveal that raced the
  // field into existence — is an accident. Focus is simply taken back.
  //
  // Deferred by a tick because at `focusout` time the incoming element has not
  // been focused yet, and the check has to see where the keyboard ended up.
  useEffect(() => {
    if (mode !== "collapsed") return;
    const onFocusOut = () => {
      window.setTimeout(() => {
        if (document.activeElement === inputRef.current) return;
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

  useEffect(() => {
    if (!settings.hide_on_blur) return;

    const currentWindow = getCurrentWindow();
    let mounted = true;
    let unlisten: (() => void) | undefined;

    currentWindow.onFocusChanged(({ payload: focused }) => {
      if (!mounted) return;
      if (focused) {
        if (mode === "collapsed") {
          focusCollapsedInput(20);
          focusCollapsedInput(80);
        } else if (mode === "terminal") {
          focusTerminalView(40);
          if ((rendererRef.current?.mode ?? 0) & FOCUS_IN_OUT) {
            invoke("term_input", { id: "main", data: [27, 91, 73] });
          }
        }
        return;
      }
      if (mode === "terminal" && (rendererRef.current?.mode ?? 0) & FOCUS_IN_OUT) {
        invoke("term_input", { id: "main", data: [27, 91, 79] });
      }
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
    canvasRef.current?.focus();
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

  const copySelection = async () => {
    const renderer = rendererRef.current;
    const sel = selectionRef.current;
    if (!renderer || !sel) return;
    const text = renderer.selectionText(sel);
    if (text) {
      try {
        await navigator.clipboard.writeText(text);
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

  const pasteClipboard = async () => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    let text = "";
    try {
      text = await navigator.clipboard.readText();
    } catch {
      return;
    }
    if (!text) return;
    const bracketed = (renderer.mode & BRACKETED_PASTE) !== 0;
    const payload = bracketed ? `\x1b[200~${text}\x1b[201~` : text;
    invoke("term_input", {
      id: "main",
      data: Array.from(new TextEncoder().encode(payload)),
    });
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
            id: "main",
            delta: event.key === "PageUp" ? lines : -lines,
          });
          return;
        }
        const renderer = rendererRef.current;
        const encoded = renderer ? encodeKey(event, renderer.mode) : null;
        if (encoded) {
          event.preventDefault();
          invoke("term_input", { id: "main", data: Array.from(encoded) });
        }
        return;
      }

      if (mode === "settings") {
        if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
          event.preventDefault();
          closeSettings();
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
        if (launcherResults[resultNumber - 1]) {
          event.preventDefault();
          runLauncherItem(launcherResults[resultNumber - 1]);
        }
        return;
      }
      if (inputFocused) return;

      // Below here the input does not have the keyboard, which in this mode is
      // never what the user meant: the launcher is one text field and a list
      // that is driven from it. Whatever the key was, it takes the field back —
      // and then does what it would have done had the field never lost it.
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
    const label = session.name || session.sessionId.slice(0, 8);
    if (!window.confirm(t("terminal.sessionKillConfirm", { name: label }))) return;
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

  useEffect(() => () => {
    if (opacitySaveTimer.current !== null) {
      window.clearTimeout(opacitySaveTimer.current);
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
    if (value === settings[field]) return;
    const updated: AppSettings = { ...settings, [field]: value };
    setSettings(updated);
    if (opacitySaveTimer.current !== null) {
      window.clearTimeout(opacitySaveTimer.current);
    }
    opacitySaveTimer.current = window.setTimeout(() => {
      opacitySaveTimer.current = null;
      invoke("save_settings", { settings: settingsRef.current }).catch(() => undefined);
    }, 180);
  };

  const changeTheme = (theme: string) => {
    if (theme === settings.theme) return;
    const updated: AppSettings = { ...settings, theme };
    setSettings(updated);
    suppressBlurUntil.current = Date.now() + 400;
    invoke("save_settings", { settings: updated }).catch(() => undefined);
  };

  const changeLanguage = (next: Language) => {
    if (next === language) return;
    const updated: AppSettings = { ...settings, language: next };
    setSettings(updated);
    suppressBlurUntil.current = Date.now() + 400;
    invoke("save_settings", { settings: updated }).catch(() => undefined);
  };

  const changeLaunchAtStartup = async (enabled: boolean) => {
    if (autostartUpdating || enabled === settings.launch_at_startup) return;
    const previous = settings.launch_at_startup;
    const updated: AppSettings = { ...settings, launch_at_startup: enabled };
    settingsRef.current = updated;
    setSettings(updated);
    setAutostartUpdating(true);
    suppressBlurUntil.current = Date.now() + 400;
    try {
      await invoke("set_launch_at_startup", { enabled });
      const latest = { ...settingsRef.current, launch_at_startup: enabled };
      await invoke("save_settings", { settings: latest });
    } catch {
      await invoke("set_launch_at_startup", { enabled: previous }).catch(() => undefined);
      setSettings((current) =>
        current.launch_at_startup === enabled
          ? { ...current, launch_at_startup: previous }
          : current,
      );
    } finally {
      setAutostartUpdating(false);
    }
  };

  const toggleRecording = (action: ShortcutAction) => {
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
      setSettings((current) => ({
        ...current,
        hotkey: shortcuts.toggle_window,
        shortcuts,
      }));
      setRecordingAction(null);
      setRejectedAction(null);
    } catch {
      // Keep the current shortcuts if the system rejects the default toggle.
    }
  };

  // Store the new binding optimistically; the backend is the authority on
  // whether a system-wide combination can actually be taken.
  const captureShortcut = (action: ShortcutAction, next: string) => {
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
    const previous = shortcuts[action];
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

    setSettings((current) => ({
      ...current,
      shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: next },
    }));
    suppressBlurUntil.current = Date.now() + 400;
    invoke("update_shortcut", { action, shortcut: next }).then(() => {
      invoke("resume_shortcuts").catch(() => undefined);
    }).catch(() => {
      setSettings((current) => ({
        ...current,
        shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: previous },
      }));
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
    setTerminalMounted(true);
    setMode("terminal");
    try {
      if (sessionClosePromise.current) await sessionClosePromise.current;
      const { cols, rows } = dimsRef.current;
      const generation = ++nextTerminalGeneration.current;
      terminalGeneration.current = generation;
      await invoke("term_attach_existing", {
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
      if (launcherResults[resultNumber - 1]) {
        event.preventDefault();
        runLauncherItem(launcherResults[resultNumber - 1]);
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

    if (event.key === "Tab" && !selectedActionBar) {
      const selected = launcherResults[selectedResultIndex];
      if (selected?.type === "command") {
        event.preventDefault();
        setQuery(selected.commandLine);
        setHistoryIndex(-1);
        return;
      }
    }

    // The results and the action bar are navigated as one loop that wraps at
    // both ends. With no query there is neither, and the arrows fall through to
    // the shell history below.
    if (actionBar || launcherResults.length) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        if (selectedActionBar) {
          // Past the action bar is the top of the list again.
          if (launcherResults.length) {
            setSelectedActionBar(false);
            setSelectedResultIndex(0);
          }
        } else if (selectedResultIndex < launcherResults.length - 1) {
          setSelectedResultIndex((index) => index + 1);
        } else {
          setSelectedActionBar(Boolean(actionBar));
        }
        return;
      }

      if (event.key === "ArrowUp") {
        event.preventDefault();
        if (selectedActionBar) {
          if (launcherResults.length) {
            setSelectedActionBar(false);
            setSelectedResultIndex(launcherResults.length - 1);
          }
        } else if (selectedResultIndex > 0) {
          setSelectedResultIndex((index) => index - 1);
        } else {
          setSelectedActionBar(Boolean(actionBar));
        }
        return;
      }

      // Tab is the direct way across, for when the list is long enough that
      // arrowing to the bottom of it is work.
      if (event.key === "Tab") {
        event.preventDefault();
        if (!selectedActionBar) {
          setSelectedActionBar(Boolean(actionBar));
        } else if (launcherResults.length) {
          setSelectedActionBar(false);
          setSelectedResultIndex(0);
        }
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

  if (mode === "settings") {
    const updatePercent =
      updateProgress && updateProgress.total > 0
        ? Math.min(100, (updateProgress.downloaded / updateProgress.total) * 100)
        : 0;
    return (
      <div className="settings-shell">
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
                onClick={() => invoke("quit_app")}
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
            {settingsPage === "general" && <>
            <div className="settings-preferences">
              <section className="settings-section">
                <h2 className="settings-section__label">{t("settings.theme")}</h2>
                <div
                  className="settings-options settings-options--inline"
                  role="radiogroup"
                  aria-label={t("settings.theme")}
                >
                  {THEME_OPTIONS.map((option) => {
                    const active = option.value === settings.theme;
                    return (
                      <button
                        key={option.value}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        className={`settings-option${active ? " settings-option--active" : ""}`}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => changeTheme(option.value)}
                      >
                        <span className="settings-option__label">{t(option.labelKey)}</span>
                      </button>
                    );
                  })}
                </div>
                <p className="settings-section__hint">{t("settings.themeHint")}</p>
              </section>

              <section className="settings-section">
                <h2 className="settings-section__label">{t("settings.language")}</h2>
                <div
                  className="settings-options settings-options--inline"
                  role="radiogroup"
                  aria-label={t("settings.language")}
                >
                  {LANGUAGE_OPTIONS.map((option) => {
                    const active = option.value === language;
                    return (
                      <button
                        key={option.value}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        className={`settings-option${active ? " settings-option--active" : ""}`}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => changeLanguage(option.value)}
                      >
                        <span className="settings-option__label">{option.label}</span>
                      </button>
                    );
                  })}
                </div>
                <p className="settings-section__hint">{t("settings.languageHint")}</p>
              </section>
            </div>

            <section className="settings-section">
              <div className="settings-option settings-option--static">
                <span className="settings-option__main">
                  <span className="settings-option__label">
                    {t("settings.launchAtStartup")}
                  </span>
                  <span className="settings-option__description">
                    {t("settings.launchAtStartupHint")}
                  </span>
                </span>
                <button
                  type="button"
                  className={`settings-switch${settings.launch_at_startup ? " settings-switch--active" : ""}`}
                  role="switch"
                  aria-checked={settings.launch_at_startup}
                  aria-label={t("settings.launchAtStartup")}
                  disabled={autostartUpdating}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => void changeLaunchAtStartup(!settings.launch_at_startup)}
                >
                  <span className="settings-switch__thumb" />
                </button>
              </div>
            </section>

            <section className="settings-section settings-section--material">
              <h2 className="settings-section__label">{t("settings.opacity")}</h2>
              <div className="opacity-controls">
                <OpacityControl
                  label={t("settings.opacity.main")}
                  value={normalizeOpacity(settings.main_opacity)}
                  onChange={(value) => changeOpacity("main_opacity", value)}
                />
                <OpacityControl
                  label={t("settings.opacity.terminal")}
                  value={normalizeOpacity(settings.terminal_opacity)}
                  onChange={(value) => changeOpacity("terminal_opacity", value)}
                />
              </div>
              <p className="settings-section__hint">{t("settings.opacityHint")}</p>
            </section>
            </>}

            {settingsPage === "shortcuts" && (
            <section className="settings-section">
              <div className="settings-section__heading">
                <h2 className="settings-section__label">{t("settings.shortcuts")}</h2>
                <button
                  type="button"
                  className="settings-reset"
                  title={t("settings.shortcutsResetHint")}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => void restoreDefaultShortcuts()}
                >
                  <span className="settings-reset__icon" aria-hidden="true">↺</span>
                  <span>{t("settings.shortcutsReset")}</span>
                </button>
              </div>
              <div className="settings-options">
                {SHORTCUT_ACTIONS.map((action) => {
                  const labelKey: MessageKey = `shortcut.${action}`;
                  const rejected = rejectedAction === action;
                  return (
                    <div key={action} className="settings-option settings-option--static">
                      <span className="settings-option__main">
                        <span className="settings-option__label">{t(labelKey)}</span>
                        {rejected && (
                          <span className="settings-option__description settings-option__description--warning">
                            {t("settings.shortcut.rejected")}
                          </span>
                        )}
                      </span>
                      <ShortcutRecorder
                        action={action}
                        shortcut={shortcuts[action]}
                        recording={recordingAction === action}
                        onToggle={toggleRecording}
                        onCapture={captureShortcut}
                        onCancel={cancelRecording}
                        t={t}
                      />
                    </div>
                  );
                })}
              </div>
              <p className="settings-section__hint">{t("settings.shortcutsHint")}</p>
            </section>
            )}

            {settingsPage === "sessions" && (
            <section className="settings-section session-manager">
              <div className="settings-section__heading">
                <h2 className="settings-section__label">{t("terminal.sessions")}</h2>
                <button
                  type="button"
                  className="session-manager__icon-button"
                  aria-label={t("terminal.sessionsRefresh")}
                  title={t("terminal.sessionsRefresh")}
                  disabled={sessionsLoading}
                  onClick={() => void refreshTerminalSessions()}
                >
                  <RefreshCw size={14} strokeWidth={1.9} aria-hidden="true" />
                </button>
              </div>

              {terminalSessions.length === 0 ? (
                <div className="session-manager__empty">
                  <SquareTerminal size={20} strokeWidth={1.6} aria-hidden="true" />
                  <span>{sessionsLoading ? t("terminal.sessionsLoading") : t("terminal.sessionsEmpty")}</span>
                </div>
              ) : (
                <div className="session-manager__list">
                  {terminalSessions.map((session) => {
                    const busy = sessionActionId === session.sessionId;
                    const state = session.exited
                      ? t("terminal.sessionExited")
                      : session.attached
                        ? t("terminal.sessionAttached")
                        : t("terminal.sessionDetached");
                    const created = new Date(session.createdAt);
                    return (
                      <div key={session.sessionId} className="session-manager__row">
                        <span className="session-manager__marker" aria-hidden="true">
                          <SquareTerminal size={16} strokeWidth={1.8} />
                        </span>
                        <span className="session-manager__main">
                          <span className="session-manager__name">
                            {session.name || t("terminal.sessionTitle", { id: session.sessionId.slice(0, 8) })}
                          </span>
                          <span className="session-manager__cwd">{session.cwd || "~"}</span>
                          <span className="session-manager__meta">
                            <span className={session.exited ? "session-state session-state--exited" : "session-state"}>
                              {state}
                            </span>
                            <span>{session.size || `${session.width}x${session.height}`}</span>
                            {!Number.isNaN(created.getTime()) && <span>{sessionDateFormatter.format(created)}</span>}
                          </span>
                        </span>
                        <span className="session-manager__actions">
                          <button
                            type="button"
                            className="session-manager__icon-button"
                            aria-label={t("terminal.sessionResume")}
                            title={t("terminal.sessionResume")}
                            disabled={session.exited || busy || sessionActionId !== null}
                            onClick={() => void resumeTerminalSession(session)}
                          >
                            <Play size={14} strokeWidth={1.9} aria-hidden="true" />
                          </button>
                          <button
                            type="button"
                            className="session-manager__icon-button session-manager__icon-button--danger"
                            aria-label={t("terminal.sessionKill")}
                            title={t("terminal.sessionKill")}
                            disabled={busy || sessionActionId !== null}
                            onClick={() => void killTerminalSession(session)}
                          >
                            <Trash2 size={14} strokeWidth={1.9} aria-hidden="true" />
                          </button>
                        </span>
                      </div>
                    );
                  })}
                </div>
              )}
            </section>
            )}

            {settingsPage === "integrations" && (
            <ExtensionsPanel
              t={t}
              locale={language}
              onOpenCommand={(plan: ExtensionExecutionPlan, label: string) => runCommand(plan, label)}
            />
            )}

            {settingsPage === "about" && (
            <section className="settings-section">
              <div className="update-banner">
                <div className="update-banner__info">
                  <span className="update-banner__title">
                    {t("settings.currentVersion")}: v{appVersion}
                  </span>
                  <span className="update-banner__desc">
                    {updateFailed
                      ? t("settings.updateFailed")
                      : updateInfo
                        ? `${t("settings.latestVersion")}: v${updateInfo.version}`
                        : t("settings.upToDate")}
                  </span>
                </div>
                {updateProgress ? (
                  <div className="update-banner__progress">
                    <div className="update-banner__progress-track">
                      <div
                        className="update-banner__progress-bar"
                        style={{ width: `${updatePercent}%` }}
                      />
                    </div>
                    <span className="update-banner__progress-label">
                      {updateProgress.total > 0
                        ? `${Math.round(updatePercent)}% · ${formatBytes(updateProgress.downloaded)} / ${formatBytes(updateProgress.total)}`
                        : formatBytes(updateProgress.downloaded)}
                    </span>
                  </div>
                ) : updateDownloading ? (
                  <button type="button" className="update-banner__button" disabled>
                    {t("settings.installing")}
                  </button>
                ) : updateFailed ? (
                  <button
                    type="button"
                    className="update-banner__button"
                    onClick={downloadAndInstallUpdate}
                  >
                    {t("settings.retry")}
                  </button>
                ) : updateInfo ? (
                  <button
                    type="button"
                    className="update-banner__button"
                    onClick={downloadAndInstallUpdate}
                  >
                    {t("settings.downloadUpdate")}
                  </button>
                ) : null}
              </div>
            </section>
            )}
            </main>
          </div>
        </div>
      </div>
    );
  }

  if (mode === "collapsed") {
    const hasQuery = query.trim().length > 0;
    // The first scan runs before there is anything to search, so the input says
    // so rather than inviting a query that would match nothing.
    const placeholder =
      appsLoading && !applications.length ? t("input.scanning") : t("input.placeholder");

    return (
      <div className="collapsed-shell">
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
          {(launcherResults.length > 0 || actionBar || launcherFeedback) && (
            <div className="launcher-bottom">
              {launcherFeedback && (
                <div className="launcher-feedback" role="status" aria-live="polite">
                  <AlertCircle className="launcher-feedback__icon" size={15} strokeWidth={1.9} aria-hidden="true" />
                  <span>{t(launcherFeedback)}</span>
                </div>
              )}
              {launcherResults.length > 0 && (
                <div className="launcher-results" role="listbox" aria-label={t("launcher.results")}>
                  {launcherResults.map((item, index) => {
                    const selected = !selectedActionBar && index === selectedResultIndex;
                    return (
                      <button
                        key={item.id}
                        type="button"
                        className={`launcher-result${selected ? " launcher-result--selected" : ""}`}
                        role="option"
                        aria-selected={selected}
                        onMouseMove={() => {
                          setSelectedActionBar(false);
                          setSelectedResultIndex(index);
                        }}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => runLauncherItem(item)}
                      >
                        <span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
                          {item.type === "app" && appIconUrls[item.app.path] ? (
                            <img src={appIconUrls[item.app.path]} alt="" />
                          ) : item.type === "system" ? (
                            <SystemActionIcon action={item.action} />
                          ) : (
                            // The placeholder for an application whose icon has
                            // not resolved yet: a first letter over a real icon
                            // reads as a different application rather than as a
                            // pending one.
                            <span>$</span>
                          )}
                        </span>
                        <span className="launcher-result__main">
                          <span className="launcher-result__title">{item.title}</span>
                          <span className="launcher-result__subtitle">
                            {item.subtitle}
                            <span className="launcher-result__source">
                              {t("extensions.source", {
                                source: item.type === "command"
                                  ? item.sourceName
                                  : item.type === "app"
                                    ? t(appSubtitleKey(item.app.path))
                                    : t("extensions.builtIn"),
                              })}
                            </span>
                          </span>
                        </span>
                        <span className="launcher-result__action">
                          {formatResultShortcut(shortcuts.select_result, index + 1)}
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
              {actionBar && (
                <button
                  type="button"
                  className={`launcher-action-bar launcher-action-bar--${actionBar.type}${
                    selectedActionBar ? " launcher-action-bar--selected" : ""
                  }`}
                  onMouseMove={() => setSelectedActionBar(true)}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => executeActionBar(actionBar)}
                >
                  <span className="launcher-action-bar__icon">
                    <ActionBarIcon kind={actionBar.type} />
                  </span>
                  <span className="launcher-action-bar__main">
                    <span className="launcher-action-bar__title">{actionBar.value}</span>
                    <span className="launcher-action-bar__subtitle">{actionBar.label}</span>
                  </span>
                  <span className="launcher-action-bar__hint">
                    {actionBarShortcut}
                  </span>
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="terminal-shell">
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
            <canvas ref={canvasRef} className="terminal-canvas" tabIndex={0} />
          </div>
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
