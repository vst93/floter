// Launcher execution cluster: running a command line, resuming a broker
// session, launching applications, handing URLs/paths to the system, running
// system power actions, dispatching launcher items, and everything the
// launcher does with a key press.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref,
// setter and callback it touches, so the behaviour is unchanged.

import { invoke } from "@tauri-apps/api/core";
import { useRef } from "react";
import type {
  Dispatch,
  RefObject,
  SetStateAction,
} from "react";
import { nextLauncherSelection, type ActivePluginMode, type ExecutionPlan } from "../launcher";
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
import { PINNED_SESSION_ID, type PinEvent, type PinState } from "../terminal/pinState";
import { resultIndexForSlot } from "../launcher/result-budget";
import { CLIPBOARD_PLUGIN_ID } from "../plugin-pages";
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
  pinStateRef: RefObject<PinState>;
  dispatchPinEvent: Dispatch<PinEvent>;
  setMainPinnedAway: (value: boolean) => void;
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
  openInTerminal: () => Promise<unknown>;
  focusTerminalView: (delay?: number) => void;
  focusCollapsedInput: (delay?: number) => void;
  /** The shared collapsed-focus beat pattern (see `collapsed-focus.ts`). */
  scheduleCollapsedFocusBeats: () => void;
  rememberCommand: (command: string) => void;
  recordLaunch: (path: string) => void;
  refreshTerminalSessions: () => Promise<void>;
  openPluginPage: (pluginId: string) => void;
  /** R31 · enter a plugin mode deliberately (the browser system row's Enter).
   *  The mode is App state now, so the row hands it over rather than rewriting
   *  the query to a trigger word the hook would have to parse back. */
  enterPluginMode: (mode: ActivePluginMode) => void;
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
    enterPluginMode,
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

    // The clipboard page is a plain view flip — no confirmation, no window
    // hiding, just the same open path the global hotkey and `floter clip`
    // take.
    if (item.action === "clipboard") {
      setQuery("");
      setHistoryIndex(-1);
      openPluginPage(CLIPBOARD_PLUGIN_ID);
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

    if (item.action === "clipboard" || item.action === "browser") return;

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
    // badges, so `⌘9` runs the fixed clipboard row like a click on it would.
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

  return {
    runCommand,
    resumeTerminalSession,
    launchApplication,
    openWithSystem,
    executeActionBar,
    runSystemAction,
    runLauncherItem,
    handleLauncherKey,
    pendingSystemAction,
    executeSystemAction,
    cancelSystemAction,
  };
}
