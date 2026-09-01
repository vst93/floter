// Launcher execution cluster: running a command line, resuming a broker
// session, launching applications, handing URLs/paths to the system, running
// system power actions, dispatching launcher items, and everything the
// launcher does with a key press.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref,
// setter and callback it touches, so the behaviour is unchanged.

import { invoke } from "@tauri-apps/api/core";
import type {
  Dispatch,
  RefObject,
  SetStateAction,
} from "react";
import { nextLauncherSelection, type ExecutionPlan } from "../launcher";
import {
  matchesResultShortcut,
  matchesShortcut,
  matchesShortcutModifiers,
  type ShortcutMap,
} from "../shortcuts";
import { PINNED_SESSION_ID, type PinEvent, type PinState } from "../terminal/pinState";
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
  rememberCommand: (command: string) => void;
  recordLaunch: (path: string) => void;
  refreshTerminalSessions: () => Promise<void>;
  openPluginPage: (pluginId: string) => void;
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
  pendingSystemAction: Extract<LauncherItem, { type: "system" }> | null;
  setPendingSystemAction: Dispatch<SetStateAction<Extract<LauncherItem, { type: "system" }> | null>>;
}) {
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
  } = options;

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
    // Power and clipboard kinds are claimed by `classifyActionBar` so the
    // user reaches them through the action bar — the result list can also
    // show them. Hand off to the same handler `runLauncherItem` uses for
    // the matching system row, with a synthesized item carrying the same
    // shape `useLauncherCatalog` produces.
    if (
      action.type === "restart" ||
      action.type === "shutdown" ||
      action.type === "clipboard"
    ) {
      const systemAction = action.type;
      const titleKey =
        systemAction === "restart"
          ? "system.restart"
          : systemAction === "shutdown"
            ? "system.shutdown"
            : "system.clipboardHistory";
      const subtitleKey =
        systemAction === "restart"
          ? "system.restartSubtitle"
          : systemAction === "shutdown"
            ? "system.shutdownSubtitle"
            : "system.clipboardHistorySubtitle";
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
   *  confirmation banner is confirmed (Enter). */
  const executeSystemAction = async () => {
    const item = pendingSystemAction;
    if (!item) return;

    setPendingSystemAction(null);

    if (item.action === "clipboard") return;

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
      focusCollapsedInput(50);
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

    // A pending system action (restart/shutdown) is armed: Enter executes it,
    // Esc cancels. Nothing else is reachable until the user decides.
    if (pendingSystemAction) {
      if (event.key === "Enter") {
        event.preventDefault();
        void executeSystemAction();
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
      event.preventDefault();
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
