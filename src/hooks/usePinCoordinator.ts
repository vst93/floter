// React binding for the pin-card coordination family: pin / detach /
// attachAsPinned / resumeIntoMainView / unpin / toggle.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref and
// setter it touches, so the behaviour is unchanged.

import { useCallback, useRef, type Dispatch, type RefObject, type SetStateAction } from "react";
import { invoke } from "@tauri-apps/api/core";
import { PINNED_SESSION_ID, type PinEvent } from "../terminal/pinState";
import { usePinnedTerminal } from "./usePinnedTerminal";
import type { BrokerSessionInfo, MainSessionIdentity, ViewMode } from "../App";
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
  const {
    pinState,
    dispatchPinEvent: dispatchRawPinEvent,
    geometry: cardGeometry,
    updateGeometry: updateCardGeometry,
  } = usePinnedTerminal();
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
    // The card's view is attached and can take input from here on. Set only on
    // the success path: a throw leaves the flag alone, and the caller's error
    // branch releases the session into the session list instead.
    pinnedReady.current = true;
    return generation;
  };

  /** Release the main view's session without killing its PTY, so the card can
   * take it over. */
  const detachMainView = async () => {
    terminalGeneration.current = null;
    ptyReady.current = false;
    mainBrokerSessionIdRef.current = null;
    setMainSessionIdentity(null);
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
      void describeMainSession(attachedId, null);
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
      // Sampled BEFORE the unpin, because unpinning into an empty main slot
      // fills that slot: `unpinPinnedSession` resumes the session there and sets
      // `ptyReady`. Reading the flag afterwards would therefore see the session
      // just handed back and immediately re-pin it, and the shortcut could never
      // unpin anything. What the re-pin is actually for is the other case — a
      // NEWER session already running in the main area, which the card moves to.
      const mainWasLive = ptyReady.current;
      await unpinPinnedSession();
      if (mainWasLive) {
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
  }, [dispatchPinEvent, setActiveSurface, setMainPinnedAway]);

  return {
    pinState,
    pinStateRef,
    dispatchPinEvent,
    cardGeometry,
    updateCardGeometry,
    togglePinnedTerminal,
    unpinPinnedSession,
    handlePinnedSessionExit,
  };
}
