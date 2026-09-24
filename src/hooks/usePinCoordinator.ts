// React binding for the pinned-native-window coordination family: pin /
// detach / resumeIntoMainView / unpin / toggle.
//
// R55 · pinning opens a second native window (`open_pinned_terminal_window`)
// instead of a card inside this one, so the reducer is used directly here —
// there is no in-window geometry to own (the pinned window persists its own).
//
// Extracted from `App.tsx`; the hook receives every App-owned ref and setter it
// touches, so the behaviour is otherwise unchanged.

import {
  useCallback,
  useReducer,
  useRef,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";
import { invoke } from "@tauri-apps/api/core";
import { PINNED_SESSION_ID, pinReducer, type PinEvent, type PinState } from "../terminal/pinState";
import type { MainSessionIdentity, ViewMode } from "../App";
import type { MessageKey } from "../i18n";

export function usePinCoordinator(options: {
  mode: ViewMode;
  resolvedTheme: "dark" | "light";
  ptyReady: RefObject<boolean>;
  /** Set here, read by the input gates: whether the CARD's view is attached to
   * a live session. Kept apart from `ptyReady`, which only ever describes the
   * main slot — and which pinning empties. */
  pinnedReady: RefObject<boolean>;
  terminalGeneration: RefObject<number | null>;
  nextTerminalGeneration: RefObject<number>;
  mainBrokerSessionIdRef: RefObject<string | null>;
  dimsRef: RefObject<{ cols: number; rows: number }>;
  setActiveSurface: (surface: "main" | "pinned") => void;
  setMainPinnedAway: (value: boolean) => void;
  setMainSessionIdentity: Dispatch<SetStateAction<MainSessionIdentity | null>>;
  describeMainSession: (brokerSessionId: string, initialCommand: string | null) => Promise<void>;
  focusTerminalView: (delay?: number) => void;
  resetTerminalFrontendState: () => void;
  showTerminalFeedback: (key: MessageKey) => void;
  refreshTerminalSessions: () => Promise<void>;
}) {
  const {
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
  } = options;

  const pinBusy = useRef(false);
  const [pinState, dispatchRawPinEvent] = useReducer(pinReducer, { status: "idle" } as PinState);
  const pinStateRef = useRef(pinState);
  pinStateRef.current = pinState;

  /**
   * The reducer dispatch, wrapped so that `pinnedReady` cannot outlive the card.
   *
   * Every path that removes the card goes through an event — this hook's unpin
   * and exit handlers, and the session-resume path in `useLauncherActions`,
   * which takes over the pinned session directly. Clearing here rather than at
   * each call site means a future fourth path cannot leave the flag set, which
   * would let keystrokes be posted to a view that no longer exists.
   */
  const dispatchPinEvent = useCallback(
    (event: PinEvent) => {
      if (event.type === "unpin") {
        pinnedReady.current = false;
      } else if (event.type === "sessionClosed") {
        const pinned = pinStateRef.current;
        if (pinned.status === "pinned" && pinned.session.generation === event.generation) {
          pinnedReady.current = false;
        }
      }
      dispatchRawPinEvent(event);
    },
    [dispatchRawPinEvent, pinnedReady],
  );

  /** Release the main view's session without killing its PTY, so the pinned
   *  window can take it over. */
  const detachMainView = async () => {
    terminalGeneration.current = null;
    ptyReady.current = false;
    mainBrokerSessionIdRef.current = null;
    setMainSessionIdentity(null);
    resetTerminalFrontendState();
    setActiveSurface("main");
  };

  /** Pin (or, when something is already pinned and a new main session is
   * running, replace) — the current main session moves into a second native
   * window (`open_pinned_terminal_window`), which attaches its own view. */
  const pinCurrentMain = async () => {
    const brokerSessionId = mainBrokerSessionIdRef.current;
    const generation = terminalGeneration.current;
    if (!ptyReady.current || !brokerSessionId || generation === null) return;
    pinBusy.current = true;
    try {
      // Detach first, then let the pinned window attach the same PTY under its
      // own view id. If the window never attaches, the session stays alive in
      // the daemon and is resumable from the session list.
      await invoke("term_detach_view", { id: "main", generation });
      await detachMainView();
      setMainPinnedAway(true);
      await invoke("open_pinned_terminal_window", { brokerSessionId });
      dispatchPinEvent({ type: "pin", brokerSessionId, generation: Date.now() });
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
      void describeMainSession(attachedId, null);
      focusTerminalView();
    } catch {
      terminalGeneration.current = null;
      showTerminalFeedback("launcher.error.session");
      refreshTerminalSessions();
    }
  };

  /** Dismiss the pinned window; the session returns to the normal flow — back
   * into the main view when that is free, otherwise left detached in the
   * session list. */
  const unpinPinnedSession = async () => {
    const pinned = pinStateRef.current;
    if (pinned.status !== "pinned" || pinBusy.current) return;
    pinBusy.current = true;
    try {
      // Attached views close by detaching only — the PTY survives. Do it before
      // closing the window so the view is gone whether the close is ours or the
      // OS's.
      await invoke("term_close", { id: PINNED_SESSION_ID }).catch(() => undefined);
      await invoke("close_pinned_terminal").catch(() => undefined);
      const { brokerSessionId } = pinned.session;
      dispatchPinEvent({ type: "unpin" });
      setMainPinnedAway(false);
      setActiveSurface("main");
      if (!ptyReady.current && mode === "terminal") {
        await resumeIntoMainView(brokerSessionId);
      }
    } finally {
      pinBusy.current = false;
    }
  };

  /** Shortcut entry point: pin / unpin / replace, depending on what is live. */
  const togglePinnedTerminal = async () => {    if (pinBusy.current) return;
    const pinned = pinStateRef.current;
    if (pinned.status === "pinned") {
      // Sampled BEFORE the unpin, because unpinning into an empty main slot
      // fills that slot: `unpinPinnedSession` resumes the session there and sets
      // `ptyReady`. Reading the flag afterwards would see the session handed
      // back and immediately re-pin it, and the shortcut could never unpin.
      const mainWasLive = ptyReady.current;
      await unpinPinnedSession();
      if (mainWasLive) {
        await pinCurrentMain();
      }
      return;
    }
    await pinCurrentMain();
  };

  /** The pinned window went away without our unpin path (the OS close button, a
   *  session that exited and closed the window): take the session back. */
  const handlePinnedWindowClosed = useCallback(
    async (brokerSessionId: string) => {
      const pinned = pinStateRef.current;
      if (pinned.status !== "pinned") return;
      dispatchPinEvent({ type: "unpin" });
      setMainPinnedAway(false);
      setActiveSurface("main");
      if (!ptyReady.current && mode === "terminal") {
        await resumeIntoMainView(brokerSessionId);
      }
    },
    [dispatchPinEvent, mode, resumeIntoMainView, setActiveSurface, setMainPinnedAway],
  );

  return {
    pinState,
    pinStateRef,
    dispatchPinEvent,
    togglePinnedTerminal,
    unpinPinnedSession,
    handlePinnedWindowClosed,
  };
}
