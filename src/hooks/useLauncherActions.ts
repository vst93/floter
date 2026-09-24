// Launcher execution cluster: running a command line, resuming a broker
// session, launching applications, handing URLs/paths to the system, running
// system power actions, dispatching launcher items, and everything the
// launcher does with a key press.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref,
// setter and callback it touches, so the behaviour is unchanged.

import { invoke } from "@tauri-apps/api/core";
import { useEffect, useRef, useState } from "react";
import type {
  Dispatch,
  RefObject,
  SetStateAction,
} from "react";
import { nextLauncherSelection, CALCULATOR_FAVORITE_SHORTCUT, CLIPBOARD_FAVORITE_SHORTCUT, HISTORY_DELETE_SHORTCUT, type ActivePluginMode, type ExecutionPlan } from "../launcher";
import {
  HISTORY_DELETE_CONFIRM_MS,
  historyDeleteCanClaim,
  reduceHistoryDelete,
  selectionAfterRemoval,
  type ArmedHistoryDelete,
} from "../plugins/history-actions";
import { calculatorCopyText, type CalculatorCopyMode, type CalculatorEntry } from "../calculator";
import {
  fileActionKindForBar,
  fileActionRequest,
  isFileActionKind,
  actionValue,
  type DroppedFile,
  type FileActionRequest,
} from "../launcher/file-drops";
import { normalizeTerminalInputSpaces } from "../terminal/inputNormalize";
import {
  matchesResultShortcut,
  matchesShortcut,
  matchesShortcutModifiers,
  IS_WINDOWS,
  type ShortcutMap,
} from "../shortcuts";
import { resultIndexForSlot } from "../launcher/result-budget";
import { isBareTerminalRow } from "../launcher/terminal-row";

import type { BrokerSessionInfo, LocalApplication, ViewMode } from "../App";
import type { MessageKey, Translate } from "../i18n";
import type { ActionBar, LauncherItem } from "../launcher/LauncherResults";

export function useLauncherActions(options: {
  query: string;
  resolvedTheme: "dark" | "light";
  t: Translate;
  terminalOpening: RefObject<boolean>;
  systemPowerOpening: RefObject<boolean>;
  ptyReady: RefObject<boolean>;
  mainBrokerSessionIdRef: RefObject<string | null>;
  terminalGeneration: RefObject<number | null>;
  nextTerminalGeneration: RefObject<number>;
  sessionClosePromise: RefObject<Promise<unknown> | null>;
  dimsRef: RefObject<{ cols: number; rows: number }>;
  setLauncherFeedback: Dispatch<SetStateAction<MessageKey | null>>;
  setTerminalFeedback: Dispatch<SetStateAction<MessageKey | null>>;
  showLauncherFeedback: (key: MessageKey, duration?: number) => void;
  setTerminalMounted: (value: boolean) => void;
  setMode: (mode: ViewMode) => void;
  setQuery: Dispatch<SetStateAction<string>>;
  setHistoryIndex: Dispatch<SetStateAction<number>>;
  setSelectedResultIndex: Dispatch<SetStateAction<number>>;
  setSelectedActionBar: Dispatch<SetStateAction<boolean>>;
  ensureTerminalSession: (
    initialCommand?: string | null,
    execution?: ExecutionPlan | null,
  ) => Promise<void>;
  /** R60 · describe an attached broker session (its command name, its exit
   *  state) in the bar's identity zone. A fresh spawn already does this inside
   *  `ensureTerminalSession`; the attach path above needs the same call, so the
   *  page can tell "no session" from "a session the broker has not named yet". */
  describeMainSession: (brokerSessionId: string, initialCommand: string | null) => Promise<void>;
  openInTerminal: () => Promise<unknown>;
  focusTerminalView: (delay?: number) => void;
  focusCollapsedInput: (delay?: number) => void;
  /** The shared collapsed-focus beat pattern (see `collapsed-focus.ts`). */
  scheduleCollapsedFocusBeats: () => void;
  rememberCommand: (command: string) => void;
  recordLaunch: (path: string) => void;
  refreshTerminalSessions: () => Promise<void>;
  /** R31 · enter a plugin mode deliberately (the browser system row's Enter).
   *  The mode is App state now, so the row hands it over rather than rewriting
   *  the query to a trigger word the hook would have to parse back. */
  enterPluginMode: (mode: ActivePluginMode) => void;
  /** R32 · whether the browser mode owns the field, so Tab cycles its range
   *  filter instead of moving focus into the card. */
  browserScope: boolean;
  /** R32 · step through the browser filter (`1` forward, `-1` back). */
  cycleBrowserFilter: (direction: 1 | -1) => void;
  /** R38 · whether the clipboard mode owns the field. Tab cycles its filter
   *  chips (the browser rule, applied to the clipboard's six) and ⌘D favorites
   *  the selected row. */
  clipboardScope: boolean;
  /** R50/R53 · whether the calculator mode owns the field. Tab cycles its two
   *  chips, ⌘D favorites and ⌃⌫ deletes; Enter evaluates a fresh expression
   *  (`calculatorEnterEvaluates`) or copies the selected row. */
  calculatorScope: boolean;
  /** R50 · whether the field holds an expression that has not been evaluated
   *  yet, so Enter should calculate rather than run the selected history row.
   *  Computed by the App from `calculatorEnterAction`. */
  calculatorEnterEvaluates: boolean;
  /** R50 · evaluate the field's expression and record it. */
  evaluateCalculator: () => void;
  /** R50 · what Enter copies from a history row. */
  calculatorCopyMode: CalculatorCopyMode;
  /** R39 · whether an external plugin's command mode owns the field. */
  externalScope: boolean;
  /** R39 · whether Enter should run the external command with the field's
   *  arguments. True when the mode's view has no interactive list of its own
   *  (text, a status row, or a display-only list); false when its rows take
   *  Enter themselves. */
  externalEnterRunsCommand: boolean;
  /** R39 · run the external command the field is feeding. */
  runExternalCommand: () => void;
  /** R38 · step through the clipboard filter chips (`1` forward, `-1` back). */
  cycleClipboardFilter: (direction: 1 | -1) => void;
  /** R50 · step through the calculator filter chips (`1` forward, `-1` back). */
  cycleCalculatorFilter: (direction: 1 | -1) => void;
  /** R38 · toggle the favorite flag of the clipboard entry with this id. The
   *  write and its optimistic paint live in the catalog hook that owns the
   *  entries; this is only the key's call into it. */
  toggleClipboardFavorite: (id: string) => void;
  /** R50 · the calculator row's favorite toggle, the clipboard's twin. */
  toggleCalculatorFavorite: (id: string) => void;
  /** R50 · delete one clipboard entry (the select-then-⌃⌫ path). */
  deleteClipboardEntry: (id: string) => void;
  /** R50 · delete one calculator entry. */
  deleteCalculatorEntry: (id: string) => void;
  isComposing: RefObject<boolean>;
  actionBar: ActionBar | null;
  shortcuts: ShortcutMap;
  launcherResults: LauncherItem[];
  resultShortcutSlots: Array<number | null>;
  runnableResultFlags: boolean[];
  selectedResultIndex: number;
  selectedActionBar: boolean;
  history: string[];
  historyIndex: number;
  draftBeforeHistory: RefObject<string>;
  collapsedCardRef: RefObject<HTMLDivElement | null>;
  /** The dropped file the selection is on, if any; the file action bar acts on
   *  it rather than on the query. */
  selectedDroppedFile: DroppedFile | null;
  /** Run the "…and N more" row: reveal every dropped file. */
  expandDroppedFiles: () => void;
  /** Move the dropped file's action switcher (←/→ on the bar). */
  cycleFileAction: (direction: -1 | 1) => void;
  /** Drop the dropped rows — the launcher's existing clear path, reused. */
  clearDrops: () => void;
  pendingSystemAction: Extract<LauncherItem, { type: "system" }> | null;
  setPendingSystemAction: Dispatch<SetStateAction<Extract<LauncherItem, { type: "system" }> | null>>;
}) {
  const launcherOpening = useRef(false);
  // R50 · the two-step inline delete: one armed row at a time, cleared by a
  // confirming press, Esc / any other key, a focus loss or the timeout. The ref
  // mirror lets the once-rendered key handler read the current arm without
  // being rebuilt.
  const [armedDelete, setArmedDelete] = useState<ArmedHistoryDelete>(null);
  const armedDeleteRef = useRef<ArmedHistoryDelete>(null);
  armedDeleteRef.current = armedDelete;
  const disarmDelete = () => setArmedDelete(null);
  // The timeout: an armed row disarms itself after a few seconds of stillness.
  const armedId = armedDelete?.id ?? null;
  const armedAt = armedDelete?.armedAt ?? 0;
  useEffect(() => {
    if (armedId === null) return;
    const timer = window.setTimeout(() => {
      setArmedDelete((current) =>
        reduceHistoryDelete(current, { type: "expire", now: Date.now() }).state,
      );
    }, HISTORY_DELETE_CONFIRM_MS + 50);
    return () => window.clearTimeout(timer);
  }, [armedId, armedAt]);
  // A focus loss (the window, or the field losing the keyboard to another
  // surface) cancels a half-finished confirmation.
  useEffect(() => {
    const onBlur = () => setArmedDelete(null);
    window.addEventListener("blur", onBlur);
    return () => window.removeEventListener("blur", onBlur);
  }, []);
  const {
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
    describeMainSession,
    openInTerminal,
    focusTerminalView,
    focusCollapsedInput,
    scheduleCollapsedFocusBeats,
    rememberCommand,
    recordLaunch,
    refreshTerminalSessions,
    enterPluginMode,
    browserScope,
    cycleBrowserFilter,
    clipboardScope,
    cycleClipboardFilter,
    toggleClipboardFavorite,
    calculatorScope,
    calculatorEnterEvaluates,
    evaluateCalculator,
    calculatorCopyMode,
    cycleCalculatorFilter,
    toggleCalculatorFavorite,
    deleteClipboardEntry,
    deleteCalculatorEntry,
    externalScope,
    externalEnterRunsCommand,
    runExternalCommand,
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
    selectedDroppedFile,
    expandDroppedFiles,
    cycleFileAction,
    clearDrops,
    pendingSystemAction,
    setPendingSystemAction,
  } = options;

  const runCommand = async (
    execution: ExecutionPlan | null = null,
    commandLine = query.trim(),
  ) => {
    // Injection boundary for the interactive-shell path: `command` is what
    // gets typed verbatim into the fresh PTY (`initial_command_payload` in the
    // broker appends `\r` and nothing else). If the raw line — or the
    // display/`commandLine` string a catalog row carried in — holds a Unicode
    // space separator between two words, zsh would read them as ONE fused
    // token (`command not found: go version`). Normalize to U+0020 here,
    // upstream of the verbatim contract, so the broker test keeps asserting
    // byte-exactness on bytes that are already clean.
    const command = normalizeTerminalInputSpaces(commandLine.trim());
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
      scheduleCollapsedFocusBeats();
    } finally {
      terminalOpening.current = false;
    }
  };

  /**
   * R60 · open a bare terminal session.
   *
   * The same transition `runCommand` makes — the terminal page is entered, the
   * field is emptied, focus moves to the canvas — with one difference: no
   * command is handed to the PTY. `ensureTerminalSession(null)` passes
   * `initialCommand: null`, which is the interactive shell the Rust side
   * resolves by default, so nothing is typed, nothing is executed and nothing
   * joins the command history. The session itself is an ordinary broker session
   * (recorded, listed by the sessions page, resumable) because it goes through
   * the very same spawn path a typed command does.
   *
   * Reached from the two R60 doors — the terminal system row's Enter and the
   * ⌘-held row's Enter — which share the `action: "terminal"` branch in
   * `runSystemAction`.
   */
  const openTerminalSession = async () => {
    if (terminalOpening.current) return;
    terminalOpening.current = true;
    setLauncherFeedback(null);
    setTerminalFeedback(null);
    setTerminalMounted(true);
    setMode("terminal");
    try {
      await ensureTerminalSession(null);
      setQuery("");
      focusTerminalView();
    } catch {
      showLauncherFeedback("launcher.error.command");
      setTerminalMounted(false);
      setMode("collapsed");
      scheduleCollapsedFocusBeats();
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
        // R60 · fill the bar's identity from the broker, exactly as a fresh
        // spawn does. Until this round an attach left the identity zone showing
        // whatever the *previous* session had said (or nothing at all), and the
        // terminal page's empty state — which is a function of "is any session
        // attached" — would have read an attached session as an absent one.
        void describeMainSession(brokerSessionId, null);
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
      scheduleCollapsedFocusBeats();
    } finally {
      terminalOpening.current = false;
    }
  };

  const launchApplication = async (app: LocalApplication) => {
    if (launcherOpening.current) return;
    launcherOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke("open_application", { path: app.path });
      setQuery("");
      setHistoryIndex(-1);
      invoke("hide_window");
    } catch {
      showLauncherFeedback("launcher.error.application");
      // Keep the launcher open so the user can revise the query.
    } finally {
      launcherOpening.current = false;
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
    if (launcherOpening.current) return;
    launcherOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke(command, args);
    } catch {
      showLauncherFeedback(command === "open_url" ? "launcher.error.url" : "launcher.error.path");
      return;
    } finally {
      launcherOpening.current = false;
    }
    setQuery("");
    setHistoryIndex(-1);
    invoke("hide_window");
  };

  /**
   * R26-A · open a browser row's URL in the browser the row came from.
   *
   * Mirrors `openWithSystem`: the launcher closes only after the backend
   * accepted the URL, so a refusal leaves the row on screen to retry.
   */
  const openBrowserUrl = async (profileKey: string, url: string) => {
    if (launcherOpening.current) return;
    launcherOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke("browser_open_url", { profileKey, url });
    } catch {
      showLauncherFeedback("launcher.error.browser");
      return;
    } finally {
      launcherOpening.current = false;
    }
    setQuery("");
    setHistoryIndex(-1);
    invoke("hide_window");
  };

  /**
   * R26-B · switch to a tab the browser already has open.
   *
   * Distinct from `openBrowserUrl`: the tab exists, so the right action is to
   * focus it, not to open the URL a second time. `url` rides along as the
   * fallback the backend uses when it cannot reach the browser (a closed debug
   * port, a browser that is not running), so the row still does what the user
   * asked. The launcher closes on the same rule as `openBrowserUrl`: only after
   * the backend accepted the call.
   */
  const activateBrowserTab = async (
    tab: { browserId: string; windowIndex: number; tabIndex: number },
    url: string,
  ) => {
    if (launcherOpening.current) return;
    launcherOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke("browser_activate_tab", {
        browserId: tab.browserId,
        windowIndex: tab.windowIndex,
        tabIndex: tab.tabIndex,
        url,
      });
    } catch {
      showLauncherFeedback("launcher.error.browser");
      return;
    } finally {
      launcherOpening.current = false;
    }
    setQuery("");
    setHistoryIndex(-1);
    invoke("hide_window");
  };

  /**
   * R27 · copy a clipboard row's entry back to the system clipboard.
   *
   * The same act the clipboard panel's own row performs
   * (`clipboard_copy_entry`), reached from the search field instead of the
   * panel. The launcher closes only after the backend accepted the copy, so a
   * failure leaves the row on screen with a feedback line rather than a silent
   * no-op.
   */
  const copyClipboardEntry = async (id: string) => {
    if (launcherOpening.current) return;
    launcherOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke("clipboard_copy_entry", { id });
    } catch {
      showLauncherFeedback("clipboard.copyFailed");
      return;
    } finally {
      launcherOpening.current = false;
    }
    setQuery("");
    setHistoryIndex(-1);
    invoke("hide_window");
  };

  /**
   * R50 · copy a calculator history row per the plugin's copy setting — the
   * whole `expression = result` line or the bare result. Mirrors the clipboard
   * copy: the launcher closes only after the write was accepted.
   */
  const copyCalculatorEntry = async (entry: CalculatorEntry) => {
    if (launcherOpening.current) return;
    launcherOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke("clipboard_write_text", {
        text: calculatorCopyText(entry, calculatorCopyMode),
      });
    } catch {
      showLauncherFeedback("launcher.error.copy");
      return;
    } finally {
      launcherOpening.current = false;
    }
    setQuery("");
    setHistoryIndex(-1);
    invoke("hide_window");
  };

  /**
   * Run one of a dropped file's three actions.
   *
   * The whole point of R7-10a: these are the *only* three things a dropped file
   * can be asked to do, and every one of them is reached from an explicit Enter
   * or click on the action bar. Nothing on the drop path calls into here — see
   * the mutation lock in `tests/file-drops.test.ts`, which fails if a dropped
   * row's arrival starts running its default action.
   */
  const runFileAction = async (request: FileActionRequest) => {
    if (request.kind === "open") {
      // Hand the path to the system opener, the same command the path action bar
      // uses: a dropped file opens in whatever owns it, a folder in the file
      // manager. The file itself is never run.
      try {
        await invoke("open_path", { path: request.path });
      } catch {
        // The refusal keeps the rows up, exactly as the path action bar stays
        // up: the drop is still there to try again or copy instead.
        showLauncherFeedback("launcher.error.fileOpen");
        return;
      }
      // Answered. The rows go with the summon that produced them, so the next
      // reveal is a fresh launcher rather than a replay of this drop.
      clearDrops();
      invoke("hide_window");
      return;
    }
    if (request.kind === "cd") {
      // Reuse the existing terminal-session path verbatim: the `cd` is typed
      // into a fresh interactive shell, so no spawn parameter was added and the
      // session semantics are untouched.
      await runCommand(null, request.commandLine);
      clearDrops();
      return;
    }
    try {
      await invoke("clipboard_write_text", { text: request.path });
    } catch {
      showLauncherFeedback("launcher.error.fileCopy");
      return;
    }
    clearDrops();
    invoke("hide_window");
  };

  /** The action bar's Enter for a dropped file: run whatever the switcher is
   *  showing, on the file the selection is actually on. */
  const runFileActionBar = (action: ActionBar) => {
    const file = selectedDroppedFile;
    if (!file) return;
    // The bar's own kind is what decides the action, not the index: the row the
    // user sees and the thing that runs are then the same value.
    const kind = fileActionKindForBar(action.type);
    void runFileAction(fileActionRequest(kind, actionValue(file, kind), IS_WINDOWS));
  };

  const executeActionBar = (action: ActionBar) => {
    // A dropped file's bar is a different control wearing the same row shape:
    // it acts on the file under the selection, not on the query text.
    if (isFileActionKind(action.type)) {
      runFileActionBar(action);
      return;
    }
    if (action.type === "url") {
      void openWithSystem("open_url", { url: action.value });
      return;
    }
    if (action.type === "path") {
      void openWithSystem("open_path", { path: action.value });
      return;
    }
    // Power and clipboard kinds are claimed by `classifyActionBar` so the
    // user reaches them through the action bar — the result list can also
    // show them. Hand off to the same handler `runLauncherItem` uses for
    // the matching system row, with a synthesized item carrying the same
    // shape `useLauncherCatalog` produces.
    if (
      action.type === "restart" ||
      action.type === "shutdown" ||
      action.type === "clipboard" ||
      action.type === "browser"
    ) {
      const systemAction = action.type;
      const titleKey =
        systemAction === "restart"
          ? "system.restart"
          : systemAction === "shutdown"
            ? "system.shutdown"
            : systemAction === "clipboard"
              ? "system.clipboardHistory"
              : "system.browserSearch";
      const subtitleKey =
        systemAction === "restart"
          ? "system.restartSubtitle"
          : systemAction === "shutdown"
            ? "system.shutdownSubtitle"
            : systemAction === "clipboard"
              ? "system.clipboardHistorySubtitle"
              : "system.browserSearchSubtitle";
      void runSystemAction({
        type: "system",
        id: `system-${systemAction}`,
        title: t(titleKey),
        subtitle: t(subtitleKey),
        action: systemAction,
      });
      return;
    }
    void runCommand();
  };

  const runSystemAction = async (item: Extract<LauncherItem, { type: "system" }>) => {
    // R26-A: the browser row is not an action, it is the door into the browser
    // result mode. R31 · it enters the mode directly (`enterPluginMode`) rather
    // than rewriting the query to `browser ` — the mode is explicit state now,
    // and the trigger vocabulary lives in `pluginModeEntry` rather than being
    // reconstructed here. The field is emptied by the entry.
    if (item.action === "browser") {
      // R26-D · a disabled row is a note, not a door: pressing Enter on it must
      // not open the mode the plugin is switched out of.
      if (item.disabled) return;
      enterPluginMode({ scope: "browser", kind: "all" });
      return;
    }

    // R33 · the clipboard row is the browser row's twin: it is the door into
    // the clipboard result mode, not a settings page. Entering the mode gives
    // the plugin's own list (and its gear opens the configuration overlay).
    // R36 · a disabled row is a note, not a door — the same guard the browser
    // branch above has had since R26-D.
    if (item.action === "clipboard") {
      if (item.disabled) return;
      enterPluginMode({ scope: "clipboard", filter: "all" });
      return;
    }

    // R51 · the calculator row is the third door: Enter enters the calculator
    // plugin's own mode (its marker-hinted history list, its chips and its
    // gear). Like R50's every other entry path, this leaves the existing
    // mode/chips/history/favorite/delete/settings behaviour untouched — it is
    // one transition into `{ scope: "calculator" }`, nothing more.
    if (item.action === "calculator") {
      if (item.disabled) return;
      enterPluginMode({ scope: "calculator", filter: "all" });
      return;
    }

    // R60 · the terminal row is the fourth door, and the only one that opens a
    // *session* rather than a mode: Enter spawns a bare PTY (no command) and
    // the surface flips to the terminal page. The ⌘-held row carries the same
    // `action`, so the two entrances share this one branch — and both stop
    // here, before the power confirmation could arm anything (a terminal is
    // not a destructive system action and must never ask "press again").
    if (item.action === "terminal") {
      void openTerminalSession();
      return;
    }

    // Cancel any previously armed confirmation and dismiss this one: selecting
    // a different system action does not transfer the confirmation.
    if (pendingSystemAction) {
      setPendingSystemAction(null);
      return;
    }

    // First click: arm the confirmation and surface an inline banner. Esc or
    // a second click on the same row cancels.
    setPendingSystemAction(item);
  };

  /** Run the previously armed system action. Called when the inline
   *  confirmation control is deliberately activated by click or Space. */
  const executeSystemAction = async () => {
    const item = pendingSystemAction;
    if (!item) return;

    setPendingSystemAction(null);

    if (item.action === "clipboard" || item.action === "browser" || item.action === "calculator" || item.action === "terminal")
      return;

    if (systemPowerOpening.current) return;
    systemPowerOpening.current = true;
    setLauncherFeedback(null);
    try {
      await invoke("hide_window");
      await invoke("system_power", { action: item.action });
      setQuery("");
      setHistoryIndex(-1);
    } catch {
      setMode("collapsed");
      await invoke("show_input").catch(() => undefined);
      showLauncherFeedback(
        item.action === "restart"
          ? "launcher.error.restart"
          : "launcher.error.shutdown",
      );
      scheduleCollapsedFocusBeats();
    } finally {
      systemPowerOpening.current = false;
    }
  };

  const cancelSystemAction = () => {
    setPendingSystemAction(null);
    focusCollapsedInput();
  };

  const runLauncherItem = (item: LauncherItem | undefined) => {
    if (!item) return;
    // R30 · a plugin's status line is information, not an action: Enter and a
    // click must do nothing at all (the renderer already refuses to draw it as
    // a control, and this is the belt to that brace).
    if (item.type === "status") return;
    if (item.type === "file") {
      // A dropped file's *row* is a preview: clicking it shows the three
      // actions and puts the keyboard on the bar. It never runs the file, and
      // it never runs the default action either — that is the red line, and the
      // only way to run anything is the deliberate second gesture on the bar.
      setSelectedActionBar(true);
      return;
    }
    if (item.type === "file-more") {
      expandDroppedFiles();
      return;
    }
    if (item.type === "app") {
      recordLaunch(item.id);
      void launchApplication(item.app);
      return;
    }
    if (item.type === "system") {
      void runSystemAction(item);
      return;
    }
    if (item.type === "browser") {
      // A status row ("no browser found", "no matches") is not runnable.
      if (item.disabled) return;
      // R26-B: a live-tab row switches to the tab; every other browser row
      // opens its URL.
      if (item.tab) {
        void activateBrowserTab(item.tab, item.url);
        return;
      }
      void openBrowserUrl(item.profileKey, item.url);
      return;
    }
    if (item.type === "clipboard") {
      // R27: a status row ("nothing copied yet", "the plugin is off") is not
      // runnable; every other clipboard row copies its entry back.
      if (item.disabled || !item.entry) return;
      void copyClipboardEntry(item.entry.id);
      return;
    }
    if (item.type === "calculator") {
      // R50: a status row is not runnable; every other calculator row copies
      // its entry per the plugin's copy setting.
      if (item.disabled || !item.entry) return;
      void copyCalculatorEntry(item.entry);
      return;
    }
    if (item.type === "plugin") {
      // R39 · an external plugin's list row. A row with no action (or one the
      // plugin marked disabled) is information; the three actions the protocol
      // defines are all performed by the launcher's own existing commands.
      if (item.disabled || !item.action) return;
      const action = item.action;
      if (action.type === "open") {
        void openWithSystem("open_url", { url: action.url });
        return;
      }
      if (action.type === "copy") {
        invoke("clipboard_write_text", { text: action.text }).catch(() => {
          showLauncherFeedback("launcher.error.copy");
        });
        return;
      }
      // `insert`: put the text back in the field without running anything.
      setQuery(action.text);
      setHistoryIndex(-1);
      focusCollapsedInput();
      return;
    }
    if (item.type === "history") {
      // History items recall the command line without executing it.
      setQuery(item.commandLine);
      setHistoryIndex(-1);
      focusCollapsedInput();
      return;
    }
    if (item.type === "command" && item.execution && item.sourceName) {
      void runCommand(item.execution, item.commandLine);
      return;
    }
    // A catalog row with an unavailable runtime remains visible for discovery,
    // but is not silently reinterpreted by the user's shell.
    showLauncherFeedback("extensions.runtimeUnavailable", 9000);
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
    if (event.repeat && event.key === "Enter") {
      event.preventDefault();
      return;
    }

    // R50 · an armed inline delete is cancelled by any key except the delete
    // key itself (which confirms). Esc disarms and stops there — it must not
    // also hide the window on the first press.
    if (armedDeleteRef.current) {
      const deleteKey =
        (clipboardScope || calculatorScope) && matchesShortcut(event, HISTORY_DELETE_SHORTCUT);
      if (!deleteKey) {
        disarmDelete();
        if (event.key === "Escape") {
          event.preventDefault();
          return;
        }
      }
    }

    // A pending power action requires its dedicated confirmation control.
    // Input Enter cannot execute it; Escape cancels and Tab reaches controls.
    if (pendingSystemAction) {
      if (event.key === "Enter") {
        event.preventDefault();
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        cancelSystemAction();
        return;
      }
      // Swallow every other key while the confirmation is armed — the
      // launcher list is still visible underneath, but the action is a
      // deliberate two-step gesture and stray typing must not run anything.
      if (event.key !== "Tab") event.preventDefault();
      return;
    }

    // R32 · in the browser mode Tab is the range filter's cycle key, in both
    // directions. It runs before the command-completion Tab below because a
    // browser row is never a command row; and it runs before the Shift+Tab
    // focus escape so the field keeps the keyboard for the whole cycle.
    if (event.key === "Tab" && browserScope) {
      event.preventDefault();
      cycleBrowserFilter(event.shiftKey ? -1 : 1);
      return;
    }

    // R38 · the clipboard mode's twin: Tab cycles its six filter chips, and
    // ⌘D / Ctrl+D favorites the selected row. Both are mode-local keys (like
    // the browser's Tab) and both run here, before the numbered-result family,
    // so the field keeps the keyboard for the whole gesture. The favorite key
    // is checked against the row the selection is on and only ever acts on a
    // runnable clipboard entry — a status line has nothing to favorite.
    if (clipboardScope && event.key === "Tab") {
      event.preventDefault();
      cycleClipboardFilter(event.shiftKey ? -1 : 1);
      return;
    }
    if (clipboardScope && matchesShortcut(event, CLIPBOARD_FAVORITE_SHORTCUT)) {
      // The key is the mode's whether or not the selection happens to be on a
      // row: consuming it keeps the webview's own bookmark gesture (if any)
      // from firing on a status line.
      event.preventDefault();
      const selected = launcherResults[selectedResultIndex];
      if (selected?.type === "clipboard" && !selected.disabled && selected.entry) {
        toggleClipboardFavorite(selected.entry.id);
      }
      return;
    }

    // R50/R53 · the calculator mode's twin: Tab cycles its two chips, ⌘D
    // favorites the selected row, and ⌃⌫ arms / confirms the inline delete of a
    // runnable history row. The same three-block shape as the clipboard above.
    if (calculatorScope && event.key === "Tab") {
      event.preventDefault();
      cycleCalculatorFilter(event.shiftKey ? -1 : 1);
      return;
    }
    if (calculatorScope && matchesShortcut(event, CALCULATOR_FAVORITE_SHORTCUT)) {
      event.preventDefault();
      const selected = launcherResults[selectedResultIndex];
      if (selected?.type === "calculator" && !selected.disabled && selected.entry) {
        toggleCalculatorFavorite(selected.entry.id);
      }
      return;
    }

    // R50/R53/R55 · delete one history row, two-step, shared by both built-in
    // history modes. The key is `CmdOrCtrl+Backspace` (⌘⌫ / Ctrl+⌫) — the app
    // modifier, on the user's explicit R55 request. The macOS collision that made
    // R53 avoid it (⌘⌫ = delete to the start of the line) is mitigated here
    // rather than dodged: the key is claimed only on a runnable history row
    // *while the field is empty*, so a hand trimming text keeps the editing
    // gesture. `query` holds only the mode's needle, not the trigger word, so
    // `` (browsing the history) is the only state that can arm. The first press
    // arms the row (the renderer shows the muted "press again" note); a second
    // press inside the window confirms; the arm is cancelled by any other key, a
    // focus loss or the timeout.
    if (clipboardScope || calculatorScope) {
      if (historyDeleteCanClaim(true, query) && matchesShortcut(event, HISTORY_DELETE_SHORTCUT)) {
        const selected = launcherResults[selectedResultIndex];
        const entry =
          selected?.type === "clipboard" || selected?.type === "calculator"
            ? selected.entry
            : undefined;
        const runnable =
          selected !== undefined &&
          (selected.type === "clipboard" || selected.type === "calculator") &&
          selected.disabled !== true &&
          entry !== undefined;
        if (selected && entry && runnable) {
          event.preventDefault();
          const outcome = reduceHistoryDelete(armedDeleteRef.current, {
            type: "press",
            id: entry.id,
            now: Date.now(),
          });
          setArmedDelete(outcome.state);
          if (outcome.confirm) {
            const nextIndex = selectionAfterRemoval(selectedResultIndex, launcherResults.length);
            if (selected.type === "clipboard") deleteClipboardEntry(entry.id);
            else deleteCalculatorEntry(entry.id);
            setSelectedActionBar(false);
            setSelectedResultIndex(nextIndex);
          }
          return;
        }
      }
    }

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
    // never run a command by mistake. R19: the slot → row mapping is
    // `result-budget.ts`'s (`resultIndexForSlot`), the same one that prints the
    // badges, so a numbered key runs exactly the row whose badge it matches.
    // R37 · `0` is the family's tenth number now, not a fixed row's: it runs the
    // tenth visible result like a click on it would.
    const resultNumber = matchesResultShortcut(event, shortcuts.select_result);
    if (resultNumber !== null) {
      const resultIndex = resultIndexForSlot(resultShortcutSlots, resultNumber);
      if (resultIndex >= 0) {
        event.preventDefault();
        runLauncherItem(launcherResults[resultIndex]);
      }
      return;
    }

    if (
      actionBar &&
      selectedActionBar &&
      selectedDroppedFile &&
      (event.key === "ArrowLeft" || event.key === "ArrowRight")
    ) {
      // The dropped file's three actions share the action bar's one row. ←/→
      // switch which of them Enter would run — the same "one switcher, not a
      // second control" rule the whole action bar follows. Arrow Up/Down keep
      // their ordinary job of moving through the results and the bar.
      event.preventDefault();
      cycleFileAction(event.key === "ArrowRight" ? 1 : -1);
      return;
    }

    if (event.key === "Escape" || matchesShortcut(event, shortcuts.new_command)) {
      event.preventDefault();
      // Esc is the launcher's dismissal for everything on it, and a dropped
      // file is part of that: the rows are cleared through the same path, so
      // the next reveal starts empty.
      clearDrops();
      invoke("hide_window");
      return;
    }

    if (
      event.key === "Enter" &&
      actionBar &&
      matchesShortcutModifiers(event, shortcuts.select_result) &&
      // R60 · the one exception to "the chord runs the action bar": the held
      // terminal row. ⌘ alone moves the highlight onto the action bar (that is
      // what makes ⌘⏎ the advertised "run in shell"), so the bar is the default
      // selection here — but the same key put the terminal row on screen, and ↑
      // steps off the bar straight onto it. A row the arrows can reach and Enter
      // cannot run would be a dead row, so the chord yields to it. Every other
      // selection — including the paste-then-⌘⏎ flow, where the query change has
      // re-selected a result — keeps the long-standing rule exactly as it was.
      !isBareTerminalRow(launcherResults[selectedResultIndex])
    ) {
      event.preventDefault();
      executeActionBar(actionBar);
      return;
    }

    if (event.key === "Enter") {
      event.preventDefault();
      // R50 · the calculator mode's Enter: a fresh expression evaluates; once
      // it has been evaluated, Enter belongs to the selected history row (the
      // copy). The App decides which with `calculatorEnterAction`.
      if (calculatorScope && calculatorEnterEvaluates) {
        event.preventDefault();
        evaluateCalculator();
        return;
      }
      // R39 · an external plugin mode whose view is not an interactive list has
      // no row to run: Enter is the command's own key, and the field's text is
      // its argv. When the view *is* an interactive list, its rows take Enter
      // (the branch below) and the command is re-run by editing the arguments.
      if (externalScope && externalEnterRunsCommand) {
        runExternalCommand();
        return;
      }
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

  return {
    runCommand,
    resumeTerminalSession,
    openTerminalSession,
    launchApplication,
    openWithSystem,
    executeActionBar,
    runSystemAction,
    runLauncherItem,
    handleLauncherKey,
    armedDeleteId: armedDelete?.id ?? null,
    pendingSystemAction,
    executeSystemAction,
    cancelSystemAction,
  };
}
