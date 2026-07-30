import { useEffect, useMemo, useRef, useState } from "react";
import { convertFileSrc, invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow, LogicalSize } from "@tauri-apps/api/window";
import { TerminalCanvas, decodeFrame, type CellPoint, type Selection } from "./terminal/render";
import { encodeKey } from "./terminal/input";
import {
  createTranslator,
  normalizeLanguage,
  LANGUAGE_OPTIONS,
  type Language,
  type MessageKey,
  type Translate,
} from "./i18n";
import {
  DEFAULT_SHORTCUTS,
  formatResultShortcut,
  formatShortcut,
  IS_MAC,
  matchesResultShortcut,
  matchesShortcut,
  SHORTCUT_ACTIONS,
  shortcutFromEvent,
  withShortcutDefaults,
  type ShortcutAction,
  type ShortcutMap,
} from "./shortcuts";
import "./App.css";

type ViewMode = "collapsed" | "terminal" | "settings";

type LocalApplication = {
  name: string;
  localizedName?: string | null;
  path: string;
  iconPath?: string | null;
  comment?: string | null;
  /** Latin search key built by the backend; see `compute_initials` there. */
  initials: string;
};

/** Answer to `check_applications`: whether a rescan would find anything new. */
type ApplicationsStatus = { upToDate: boolean; count: number };

type SystemAction = "restart" | "shutdown";

type LauncherItem =
  | { type: "app"; id: string; title: string; subtitle: string; app: LocalApplication }
  | { type: "command"; id: string; title: string; subtitle: string }
  | { type: "system"; id: string; title: string; subtitle: string; action: SystemAction };

type AppSettings = {
  hotkey: string;
  hide_on_blur: boolean;
  theme: string;
  font_size: number;
  font_family: string;
  cursor_shape: string;
  language: Language;
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

const FONT_FAMILY =
  "'SF Mono','Menlo','Monaco','Consolas','JetBrains Mono',monospace";
const FONT_SIZE = 13;
const LINE_HEIGHT = 1.4;
const PADDING_X = 3;
const PADDING_Y = 3;
const TARGET_ROWS = 24;
const INPUT_WINDOW_WIDTH = 720;
const TERMINAL_WINDOW_WIDTH = 860;
const INPUT_ROW_HEIGHT = 56;
const RESULT_ROW_HEIGHT = 42;
const RESULT_LIST_PADDING = 8;
const RESULT_ROW_GAP = 1;
const MAX_RESULTS = 6;
const WINDOW_FRAME_PADDING = 0;
const BRACKETED_PASTE = 1 << 4;
/** Idle window before an icon is fetched, so the intermediate result lists that
 * flash past while a query is still being typed cost nothing. */
const ICON_LOAD_DELAY = 150;

const normalizeSearch = (value: string) =>
  value.toLowerCase().normalize("NFKC").replace(/[^\p{L}\p{N}]+/gu, " ").trim();

/**
 * Fuzzy match score for a needle and haystack that are *already* normalized.
 *
 * Normalization is the expensive half of a match — NFKC plus a Unicode-property
 * regex — and the launcher runs one query against a few hundred installed
 * applications on every keystroke. So it happens once per query and once per
 * application name, never inside the scoring loop.
 */
const scoreNormalized = (needle: string, haystack: string) => {
  if (!needle || !haystack) return 0;
  // Nothing shorter than the needle can equal it, start with it, contain it, or
  // hold it as a subsequence — so one integer compare rejects most of the list
  // before any of the string scans below run.
  if (haystack.length < needle.length) return 0;
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

/** An application with its searchable names normalized once, up front. */
type SearchableApp = { app: LocalApplication; names: string[]; initials: string };

/**
 * Best score for a needle across an application's names and its initials.
 *
 * The initials are a separate key rather than another entry in `names` because
 * they need their own ceiling. `wyyyy` *is* the whole of "网易云音乐"'s key, so it
 * would otherwise score a perfect 1000 and outrank an application whose actual
 * name the query spells out in full — initials are a shorthand, and a real name
 * match is always the more certain of the two.
 */
const scoreApp = (needle: string, names: string[], initials: string) => {
  let best = 0;
  for (const name of names) {
    const score = scoreNormalized(needle, name);
    if (score > best) best = score;
  }
  if (initials) {
    const score = scoreNormalized(needle, initials);
    // Below an exact name match, above a prefix one: typing an application's
    // initials is deliberate enough to beat a name that merely starts the same.
    const capped = score >= 1000 ? 950 : score;
    if (capped > best) best = capped;
  }
  return best;
};

/**
 * The built-in power actions, searched like applications.
 *
 * `searchNames` carries the wording of *every* language rather than only the
 * current one: the UI language says nothing about the keyboard the query is
 * typed on, so "restart" has to find the entry on a Chinese UI and "关机" on an
 * English one. They are normalized here, once, for the same reason application
 * names are — see [`scoreNormalized`].
 */
const SYSTEM_COMMANDS: {
  action: SystemAction;
  titleKey: MessageKey;
  subtitleKey: MessageKey;
  searchNames: string[];
}[] = [
  {
    action: "restart",
    titleKey: "system.restart",
    subtitleKey: "system.restartSubtitle",
    searchNames: ["restart", "reboot", "重启", "重新启动"].map(normalizeSearch),
  },
  {
    action: "shutdown",
    titleKey: "system.shutdown",
    subtitleKey: "system.shutdownSubtitle",
    searchNames: ["shutdown", "shut down", "power off", "关机", "关闭电脑"].map(normalizeSearch),
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

/** Lucide `keyboard`, sized like the settings gear next to it. */
const KeyboardIcon = () => (
  <svg
    viewBox="0 0 24 24"
    width="14"
    height="14"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    <path d="M10 8h.01" />
    <path d="M12 12h.01" />
    <path d="M14 8h.01" />
    <path d="M16 12h.01" />
    <path d="M18 8h.01" />
    <path d="M6 8h.01" />
    <path d="M7 16h10" />
    <path d="M8 12h.01" />
    <rect width="20" height="16" x="2" y="4" rx="2" />
  </svg>
);

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

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
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
        <>
          <KeyboardIcon />
          <span className="shortcut-recorder__keys">{formatShortcut(shortcut)}</span>
        </>
      )}
    </button>
  );
}


type FramePayload = { id: string; frame: string };
type ExitPayload = { id: string; code: number | null };

type DragMode = "none" | "select" | "scroll";

export default function App() {
  const inputRef = useRef<HTMLInputElement>(null);
  const mountRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const settingsRef = useRef<HTMLDivElement>(null);
  const settingsBodyRef = useRef<HTMLDivElement>(null);
  const rendererRef = useRef<TerminalCanvas | null>(null);
  const frameRef = useRef<Uint8Array | null>(null);
  const blinkRef = useRef(true);
  const dimsRef = useRef<{ cols: number; rows: number }>({ cols: 80, rows: 24 });
  const selectionRef = useRef<Selection | null>(null);
  const dragRef = useRef<{ mode: DragMode }>({ mode: "none" });
  const lastScrollAt = useRef(0);
  const clickSeq = useRef({ count: 0, time: 0, col: -1, row: -1 });

  const ptyReady = useRef(false);
  const sessionClosePromise = useRef<Promise<unknown> | null>(null);
  const termOpened = useRef(false);
  const pendingCommand = useRef<string | null>(null);
  const draftBeforeHistory = useRef("");
  const restoringMode = useRef<ViewMode | null>(null);
  /** Guards against two scans overlapping: a cold cache reads as out of date, so
   * a summon during the very first scan would otherwise start a second one. */
  const appScanning = useRef(false);

  const [mode, setMode] = useState<ViewMode>("collapsed");
  const [query, setQuery] = useState("");
  const [terminalMounted, setTerminalMounted] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [applications, setApplications] = useState<LocalApplication[]>([]);
  /** True until the first scan settles, whether it hit the cache or not. */
  const [appsLoading, setAppsLoading] = useState(true);
  const [appIconUrls, setAppIconUrls] = useState<Record<string, string>>({});
  const [selectedItemIndex, setSelectedItemIndex] = useState(0);
  const [settings, setSettings] = useState<AppSettings>({
    hotkey: "Ctrl+Shift+Space",
    hide_on_blur: true,
    theme: "dark",
    font_size: 14,
    font_family: "monospace",
    cursor_shape: "beam",
    language: "en",
    shortcuts: DEFAULT_SHORTCUTS,
  });
  const [recordingAction, setRecordingAction] = useState<ShortcutAction | null>(null);
  const [rejectedAction, setRejectedAction] = useState<ShortcutAction | null>(null);
  const suppressBlurUntil = useRef(0);

  const language = normalizeLanguage(settings.language);
  const t = useMemo(() => createTranslator(language), [language]);
  const shortcuts = useMemo(
    () => withShortcutDefaults(settings.shortcuts),
    [settings.shortcuts],
  );

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
      })),
    [applications],
  );

  const launcherItems = useMemo<LauncherItem[]>(() => {
    const command = query.trim();
    if (!command) return [];

    const commandItem: LauncherItem = {
      type: "command",
      id: "command",
      title: command,
      subtitle: t("launcher.runInShell"),
    };

    // A query of nothing but punctuation normalizes away entirely; it can only
    // ever be a shell command.
    const needle = normalizeSearch(command);
    if (!needle) return [commandItem];

    // Applications and the power actions are scored the same way and ranked
    // against each other, so "restart" reaches the power action while "restic"
    // still reaches the application.
    const matches: { item: LauncherItem; score: number }[] = [];

    for (const entry of searchableApps) {
      const score = scoreApp(needle, entry.names, entry.initials);
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
      // No initials of their own: `searchNames` already carries every spelling a
      // power action is reached by, in each language.
      const score = scoreApp(needle, [normalizeSearch(title), ...entry.searchNames], "");
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

    const items = matches
      .sort((a, b) => b.score - a.score || a.item.title.localeCompare(b.item.title))
      .slice(0, MAX_RESULTS - 1)
      .map((match) => match.item);

    if (!items.length) return [commandItem];
    // The best match keeps the top slot and the shell command the one below it,
    // so running the query as typed is always exactly one row away.
    return [items[0], commandItem, ...items.slice(1)].slice(0, MAX_RESULTS);
  }, [query, searchableApps, t]);

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

  useEffect(() => {
    scanApplications(false);
  }, []);

  useEffect(() => {
    setSelectedItemIndex(0);
  }, [query]);

  useEffect(() => {
    setSelectedItemIndex((index) => {
      if (!launcherItems.length) return 0;
      return Math.min(index, launcherItems.length - 1);
    });
  }, [launcherItems.length]);

  useEffect(() => {
    if (mode !== "collapsed") return;
    const resultHeight = launcherItems.length
      ? RESULT_LIST_PADDING +
        launcherItems.length * RESULT_ROW_HEIGHT +
        Math.max(0, launcherItems.length - 1) * RESULT_ROW_GAP
      : 0;
    getCurrentWindow()
      .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, INPUT_ROW_HEIGHT + resultHeight))
      .catch(() => undefined);
  }, [launcherItems.length, mode]);

  useEffect(() => {
    const missing = launcherItems
      .filter((item): item is Extract<LauncherItem, { type: "app" }> => item.type === "app")
      .filter((item) => !appIconUrls[item.app.path])
      .slice(0, 6);

    if (!missing.length) return;
    let cancelled = false;

    // Resolving an icon means walking the platform's icon directories, so it
    // waits for the query to settle: every keystroke changes the result list,
    // and the only list worth fetching for is the one the user stops on.
    const timer = window.setTimeout(() => {
      for (const item of missing) {
        invoke<string | null>("application_icon", { path: item.app.path })
          .then((path) => {
            if (!path || cancelled) return;
            setAppIconUrls((current) => ({
              ...current,
              [item.app.path]: convertFileSrc(path),
            }));
          })
          .catch(() => undefined);
      }
    }, ICON_LOAD_DELAY);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [appIconUrls, launcherItems]);

  const render = () => {
    const renderer = rendererRef.current;
    const frame = frameRef.current;
    if (renderer && frame) {
      renderer.draw(frame, blinkRef.current, selectionRef.current);
    }
  };

  const focusCollapsedInput = (delay = 0) => {
    window.setTimeout(() => {
      const input = inputRef.current;
      if (!input) return;
      input.focus();
      const length = input.value.length;
      input.setSelectionRange(length, length);
    }, delay);
  };

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

  // Resize the window so the terminal area holds an integer number of rows,
  // eliminating the bottom gap from row rounding. The drag bar is an overlay
  // (out of flow), so the terminal fills the full window height.
  const fitWindow = async () => {
    const renderer = rendererRef.current;
    if (!renderer || renderer.cellHeight <= 0) return;
    const exactHeight = WINDOW_FRAME_PADDING * 2 + PADDING_Y * 2 + TARGET_ROWS * renderer.cellHeight;
    try {
      await getCurrentWindow().setSize(new LogicalSize(TERMINAL_WINDOW_WIDTH, exactHeight));
      // The summon anchor centers the *expanded* terminal, so the backend needs
      // the height rows actually round to rather than the nominal constant.
      invoke("set_terminal_height", { height: exactHeight });
    } catch {
      // setSize may be unavailable; relayout will still adapt rows to the
      // current window size.
    }
  };

  const flushPendingCommand = (delay = 0) => {
    if (!pendingCommand.current) return;
    const command = pendingCommand.current;
    pendingCommand.current = null;
    window.setTimeout(() => {
      invoke("term_input", {
        id: "main",
        data: Array.from(new TextEncoder().encode(`${command}\n`)),
      });
      focusTerminalView();
    }, delay);
  };

  const resetTerminalFrontendState = () => {
    frameRef.current = null;
    selectionRef.current = null;
    dragRef.current = { mode: "none" };
    clickSeq.current = { count: 0, time: 0, col: -1, row: -1 };
  };

  const closeTerminalSession = () => {
    ptyReady.current = false;
    resetTerminalFrontendState();
    const closing = invoke("term_close", { id: "main" }).catch(() => undefined);
    sessionClosePromise.current = closing;
    closing.finally(() => {
      if (sessionClosePromise.current === closing) {
        sessionClosePromise.current = null;
      }
    });
  };

  const ensureTerminalSession = async () => {
    if (sessionClosePromise.current) {
      await sessionClosePromise.current;
    }
    if (ptyReady.current) return;
    const { cols, rows } = dimsRef.current;
    await invoke("term_spawn", { id: "main", shell: null, cols, rows });
    ptyReady.current = true;
  };

  const handleTerminalExit = () => {
    closeTerminalSession();
    pendingCommand.current = null;
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
          shortcuts: withShortcutDefaults(loaded.shortcuts),
        }),
      )
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    const unlistenFramePromise = listen<FramePayload>("term://frame", (event) => {
      if (event.payload.id !== "main") return;
      frameRef.current = decodeFrame(event.payload.frame);
      blinkRef.current = true;
      render();
    });

    const unlistenExitPromise = listen<ExitPayload>("term://exit", (event) => {
      if (event.payload.id !== "main") return;
      handleTerminalExit();
    });

    return () => {
      unlistenFramePromise.then((unlisten) => unlisten());
      unlistenExitPromise.then((unlisten) => unlisten());
      rendererRef.current = null;
      frameRef.current = null;
      termOpened.current = false;
      pendingCommand.current = null;
      ptyReady.current = false;
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
    void fitWindow();
    flushPendingCommand(40);

    const resizeObserver = new ResizeObserver(() => relayoutAndResize());
    resizeObserver.observe(mountRef.current);

    const onWheelNative = (e: WheelEvent) => {
      const renderer = rendererRef.current;
      if (!renderer || renderer.historySize <= 0) return;
      const delta =
        -Math.sign(e.deltaY) * Math.max(1, Math.round(Math.abs(e.deltaY) / 40));
      if (delta === 0) return;
      e.preventDefault();
      invoke("term_scroll", { id: "main", delta });
    };
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

  // The settings panel drives the window height from its own content, so new
  // rows can be added later without hand-tuning a constant. The body's
  // scrollHeight is measured rather than the card's box: the card is capped to
  // the screen, and measuring the cap would keep the window at its old size.
  useEffect(() => {
    if (mode !== "settings") return;
    const panel = settingsRef.current;
    const body = settingsBodyRef.current;
    if (!panel || !body) return;

    const applyHeight = () => {
      const header = panel.getBoundingClientRect().height - body.getBoundingClientRect().height;
      const height = Math.ceil(header + body.scrollHeight);
      if (height <= 0) return;
      const limit = Math.round(window.screen.availHeight * 0.85);
      getCurrentWindow()
        .setSize(new LogicalSize(INPUT_WINDOW_WIDTH, Math.min(height, limit)))
        .catch(() => undefined);
    };

    applyHeight();
    const observer = new ResizeObserver(applyHeight);
    observer.observe(panel);
    observer.observe(body);
    return () => observer.disconnect();
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
        invoke("show_input");
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
    const timer = window.setTimeout(() => {
      if (restoringMode.current === "terminal") {
        restoringMode.current = null;
      }
    }, 160);
    return () => window.clearTimeout(timer);
  }, [mode]);

  useEffect(() => {
    const unlistenModePromise = listen<string>("floter://mode", (event) => {
      if (event.payload === "collapsed") {
        closeTerminalSession();
        pendingCommand.current = null;
        setQuery("");
        setTerminalMounted(false);
        setMode("collapsed");
      }
    });

    const unlistenRevealPromise = listen<string>("floter://revealed", (event) => {
      // Every summon asks whether the application directories have changed since
      // the last scan. The check only stats them, so a machine where nothing was
      // installed pays nothing and the user never sees a refresh they did not
      // need; a changed one rescans in the background, with the old list still
      // usable until the new one lands.
      invoke<ApplicationsStatus>("check_applications")
        .then((status) => {
          if (!status.upToDate) scanApplications(true);
        })
        .catch(() => undefined);

      if (event.payload === "terminal") {
        restoringMode.current = "terminal";
        setTerminalMounted(true);
        setMode("terminal");
        focusTerminalView(80);
        window.setTimeout(() => {
          if (restoringMode.current === "terminal") {
            restoringMode.current = null;
          }
        }, 160);
        return;
      }

      restoringMode.current = "collapsed";
      pendingCommand.current = null;
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
        }
        return;
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

  const onWindowMouseMove = (e: MouseEvent) => {
    const canvas = canvasRef.current;
    const renderer = rendererRef.current;
    if (!canvas || !renderer) return;
    const rect = canvas.getBoundingClientRect();
    const px = e.clientX - rect.left;
    const py = e.clientY - rect.top;
    const drag = dragRef.current;
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

  const onWindowMouseUp = () => {
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

  // Open the current shell's working directory in the system default terminal.
  const openInTerminal = () => {
    invoke("open_in_default_terminal", { id: "main" });
  };

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      // The recorder listens in the capture phase; this is only a safety net.
      if (recordingAction) return;

      if (mode === "terminal") {
        // App shortcuts first, everything else is forwarded to the shell.
        if (matchesShortcut(event, shortcuts.new_command)) {
          event.preventDefault();
          returnToInputMode();
          return;
        }
        if (matchesShortcut(event, shortcuts.open_external_terminal)) {
          event.preventDefault();
          openInTerminal();
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
        if (launcherItems[resultNumber - 1]) {
          event.preventDefault();
          runLauncherItem(launcherItems[resultNumber - 1]);
        }
        return;
      }
      if (inputFocused) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      if (event.key === "Backspace") {
        event.preventDefault();
        setQuery((current) => current.slice(0, -1));
        focusCollapsedInput();
        return;
      }

      if (event.key.length === 1) {
        event.preventDefault();
        setQuery((current) => `${current}${event.key}`);
        setHistoryIndex(-1);
        focusCollapsedInput();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [launcherItems, mode, query, recordingAction, shortcuts]);

  const startDrag = (event: React.MouseEvent) => {
    if ((event.target as HTMLElement).closest("button") || (event.target as HTMLElement).closest("input")) {
      return;
    }
    event.preventDefault();
    invoke("start_drag");
  };

  const rememberCommand = (command: string) => {
    setHistory((current) => [command, ...current.filter((entry) => entry !== command)].slice(0, 20));
    setHistoryIndex(-1);
    draftBeforeHistory.current = "";
  };

  const returnToInputMode = () => {
    closeTerminalSession();
    pendingCommand.current = null;
    setQuery("");
    setTerminalMounted(false);
    setMode("collapsed");
  };

  const openSettings = () => {
    // Resizing the window steals focus for a moment; don't treat that as a blur.
    suppressBlurUntil.current = Date.now() + 400;
    setMode("settings");
  };

  const closeSettings = () => {
    // The window is already anchored; letting the collapsed layout effect restore
    // the height keeps a pending query's result list intact.
    restoringMode.current = "collapsed";
    setMode("collapsed");
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

  const toggleRecording = (action: ShortcutAction) => {
    setRejectedAction(null);
    setRecordingAction((current) => (current === action ? null : action));
  };

  const cancelRecording = () => setRecordingAction(null);

  // Store the new binding optimistically; the backend is the authority on
  // whether a system-wide combination can actually be taken.
  const captureShortcut = (action: ShortcutAction, next: string) => {
    setRecordingAction(null);
    setRejectedAction(null);
    const previous = shortcuts[action];
    if (next === previous) return;

    setSettings((current) => ({
      ...current,
      shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: next },
    }));
    suppressBlurUntil.current = Date.now() + 400;
    invoke("update_shortcut", { action, shortcut: next }).catch(() => {
      setSettings((current) => ({
        ...current,
        shortcuts: { ...withShortcutDefaults(current.shortcuts), [action]: previous },
      }));
      setRejectedAction(action);
    });
  };

  const runCommand = async () => {
    const command = query.trim();
    if (!command) return;

    rememberCommand(command);
    pendingCommand.current = command;
    try {
      await ensureTerminalSession();
    } catch {
      pendingCommand.current = null;
      return;
    }
    setTerminalMounted(true);
    setQuery("");
    setMode("terminal");
  };

  const launchApplication = async (app: LocalApplication) => {
    try {
      await invoke("open_application", { path: app.path });
      setQuery("");
      setHistoryIndex(-1);
      invoke("hide_window");
    } catch {
      // Keep the launcher open so the user can revise the query.
    }
  };

  const runLauncherItem = (item: LauncherItem | undefined) => {
    if (!item) return;
    if (item.type === "app") {
      void launchApplication(item.app);
      return;
    }
    if (item.type === "system") {
      invoke("system_power", { action: item.action }).catch(() => undefined);
      // Hidden without waiting for the answer: the panel sits at a window level
      // above almost everything, and macOS confirms a restart with a dialog that
      // would otherwise appear behind it.
      setQuery("");
      setHistoryIndex(-1);
      invoke("hide_window");
      return;
    }
    void runCommand();
  };

  const onInputKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    const native = event.nativeEvent;
    const resultNumber = matchesResultShortcut(native, shortcuts.select_result);
    if (resultNumber !== null) {
      if (launcherItems[resultNumber - 1]) {
        event.preventDefault();
        runLauncherItem(launcherItems[resultNumber - 1]);
      }
      return;
    }

    if (event.key === "Escape" || matchesShortcut(native, shortcuts.new_command)) {
      event.preventDefault();
      invoke("hide_window");
      return;
    }

    if (event.key === "Enter") {
      event.preventDefault();
      runLauncherItem(launcherItems[selectedItemIndex]);
      return;
    }

    if (launcherItems.length && event.key === "ArrowUp") {
      event.preventDefault();
      setSelectedItemIndex((index) =>
        index <= 0 ? launcherItems.length - 1 : index - 1,
      );
      return;
    }

    if (launcherItems.length && event.key === "ArrowDown") {
      event.preventDefault();
      setSelectedItemIndex((index) =>
        index >= launcherItems.length - 1 ? 0 : index + 1,
      );
      return;
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

  if (mode === "settings") {
    return (
      <div className="settings-shell">
        <div className="settings-card" ref={settingsRef} onMouseDown={startDrag}>
          <header className="settings-card__header">
            <span className="settings-card__title">{t("settings.title")}</span>
            <button
              type="button"
              className="toolbar-button toolbar-button--close"
              aria-label={t("settings.close")}
              title={t("settings.closeHint")}
              onClick={closeSettings}
            >
              ×
            </button>
          </header>

          <div className="settings-card__body" ref={settingsBodyRef}>
            <section className="settings-section">
              <h2 className="settings-section__label">{t("settings.theme")}</h2>
              <div
                className="settings-options"
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
                      <span className="settings-option__main">
                        <span className="settings-option__label">{t(option.labelKey)}</span>
                      </span>
                      <span className="settings-option__check" aria-hidden="true">
                        {active ? "✓" : ""}
                      </span>
                    </button>
                  );
                })}
              </div>
              <p className="settings-section__hint">{t("settings.themeHint")}</p>
            </section>

            <section className="settings-section">
              <h2 className="settings-section__label">{t("settings.language")}</h2>
              <div
                className="settings-options"
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
                      <span className="settings-option__main">
                        <span className="settings-option__label">{option.label}</span>
                        <span className="settings-option__description">
                          {t(option.descriptionKey)}
                        </span>
                      </span>
                      <span className="settings-option__check" aria-hidden="true">
                        {active ? "✓" : ""}
                      </span>
                    </button>
                  );
                })}
              </div>
              <p className="settings-section__hint">{t("settings.languageHint")}</p>
            </section>

            <section className="settings-section">
              <h2 className="settings-section__label">{t("settings.shortcuts")}</h2>
              <div className="settings-options">
                {SHORTCUT_ACTIONS.map((action) => {
                  const labelKey: MessageKey = `shortcut.${action}`;
                  const descriptionKey: MessageKey = `shortcut.${action}.description`;
                  const rejected = rejectedAction === action;
                  return (
                    <div key={action} className="settings-option settings-option--static">
                      <span className="settings-option__main">
                        <span className="settings-option__label">{t(labelKey)}</span>
                        <span
                          className={`settings-option__description${
                            rejected ? " settings-option__description--warning" : ""
                          }`}
                        >
                          {rejected ? t("settings.shortcut.rejected") : t(descriptionKey)}
                        </span>
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
                setQuery(event.target.value);
                setHistoryIndex(-1);
              }}
              onKeyDown={onInputKeyDown}
              placeholder={placeholder}
              autoFocus
              spellCheck={false}
              autoCapitalize="off"
              autoCorrect="off"
            />
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
          {launcherItems.length > 0 && (
            <div className="launcher-results" role="listbox" aria-label="Launcher results">
              {launcherItems.map((item, index) => (
                <button
                  key={item.id}
                  type="button"
                  className={`launcher-result${index === selectedItemIndex ? " launcher-result--selected" : ""}`}
                  role="option"
                  aria-selected={index === selectedItemIndex}
                  onMouseEnter={() => setSelectedItemIndex(index)}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => runLauncherItem(item)}
                >
                  <span className={`launcher-result__icon launcher-result__icon--${item.type}`}>
                    {item.type === "app" && appIconUrls[item.app.path] ? (
                      <img src={appIconUrls[item.app.path]} alt="" />
                    ) : item.type === "system" ? (
                      <SystemActionIcon action={item.action} />
                    ) : (
                      // An application whose icon has not resolved yet and the
                      // shell command share the same placeholder: both are "run
                      // this", and a first letter over a real icon reads as a
                      // different application rather than as a pending one.
                      <span>$</span>
                    )}
                  </span>
                  <span className="launcher-result__main">
                    <span className="launcher-result__title">{item.title}</span>
                    <span className="launcher-result__subtitle">{item.subtitle}</span>
                  </span>
                  <span className="launcher-result__action">
                    {formatResultShortcut(shortcuts.select_result, index + 1)}
                  </span>
                </button>
              ))}
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
              onClick={openInTerminal}
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
          >
            <canvas ref={canvasRef} className="terminal-canvas" tabIndex={0} />
          </div>
        </div>
      </section>
    </div>
  );
}
