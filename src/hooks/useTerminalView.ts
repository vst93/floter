// React binding for the main terminal view: renderer lifecycle, frame/exit
// events, mouse/selection/wheel handling and the IME proxy textarea.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref and
// setter it touches, so the behaviour is unchanged.
//
// Note the deliberate scope: the broker-side session bookkeeping refs
// (`ptyReady`, `terminalGeneration`, ...) stay in `App.tsx` because the
// session resume path shares them.

import {
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import {
  TerminalCanvas,
  decodeFrame,
  wheelScrollSteps,
  type Selection,
} from "../terminal/render";
import { MOUSE_MOTION, usesMouseReporting } from "../terminal/keys";
import { normalizeTerminalInputSpaces, stripPasteNewline } from "../terminal/inputNormalize";
import { normalizeFontSize } from "../settings/GeneralPage";
import { normalizeLineHeight, type BoldMode, type TerminalTheme } from "../terminal/terminal-appearance";
import { createDeferredRepaint, type DeferredRepaint } from "../deferred-repaint";
import { IS_MAC } from "../shortcuts";
import type { ExecutionPlan } from "../launcher";
import type { BrokerSessionInfo, MainSessionIdentity, ViewMode } from "../App";
import type { MessageKey, Translate } from "../i18n";

// 'DejaVu Sans Mono' and 'Liberation Mono' are the monospace faces actually
// present on Linux desktops; without them the stack falls through to a generic
// `monospace` whose fontconfig match is frequently not a terminal face at all.
const FALLBACK_FONT_FAMILY =
  "'SF Mono','Menlo','Monaco','Consolas','JetBrains Mono','DejaVu Sans Mono','Liberation Mono',monospace";
const TERMINAL_SIZE_SAVE_DELAY = 280;
const BRACKETED_PASTE = 1 << 4;

export const terminalFontFamily = (value: string): string => {
  const family = value.trim();
  if (!family || family === "monospace") return FALLBACK_FONT_FAMILY;
  const escaped = family.replace(/[\\']/g, "\\$&");
  return `'${escaped}',${FALLBACK_FONT_FAMILY}`;
};

type ModifierEvent = {
  shiftKey: boolean;
  altKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
};

function terminalMouseModifiers(event: ModifierEvent): number {
  return (event.shiftKey ? 4 : 0) | (event.altKey || event.metaKey ? 8 : 0) | (event.ctrlKey ? 16 : 0);
}

type FramePayload = { id: string; generation: number; frame: string };
type ExitPayload = { id: string; generation: number; code: number | null };

type DragState =
  | { mode: "none" | "select" | "scroll" }
  | { mode: "mouse"; button: number };

/** Answer to `open_in_default_terminal` in the backend. */
type ExternalTerminalOutcome = { session_handed_off: boolean };

export function useTerminalView(options: {
  canvasRef: RefObject<HTMLCanvasElement | null>;
  mountRef: RefObject<HTMLDivElement | null>;
  terminalTextInputRef: RefObject<HTMLTextAreaElement | null>;
  terminalComposing: RefObject<boolean>;
  mode: ViewMode;
  fontFamily: string;
  fontSize: number;
  /** R42 · the rest of the terminal's appearance, all user-owned. */
  lineHeight: number;
  padding: number;
  cursorBlink: boolean;
  showScrollbar: boolean;
  terminalTheme: TerminalTheme;
  /** R43 · the interaction axes: wheel-scroll line count, bold rendering, copy
   *  on selection, and stripping a paste's trailing newline. */
  wheelLines: number;
  boldMode: BoldMode;
  selectCopy: boolean;
  pasteSafe: boolean;
  resolvedTheme: "dark" | "light";
  ptyReady: RefObject<boolean>;
  terminalGeneration: RefObject<number | null>;
  nextTerminalGeneration: RefObject<number>;
  mainBrokerSessionIdRef: RefObject<string | null>;
  sessionClosePromise: RefObject<Promise<unknown> | null>;
  restoringMode: RefObject<ViewMode | null>;
  setMainSessionIdentity: Dispatch<SetStateAction<MainSessionIdentity | null>>;
  setTerminalFeedback: Dispatch<SetStateAction<MessageKey | null>>;
  setQuery: Dispatch<SetStateAction<string>>;
  setMode: (mode: ViewMode) => void;
  showTerminalFeedback: (key: MessageKey) => void;
  /** R44 · report a copy's outcome in the terminal's own status row. Both the
   *  copy-on-select path below and the explicit copy shortcut (which calls the
   *  same `copySelection`) go through here, so every path gets one notice. */
  showCopyNotice: (key: MessageKey) => void;
  t: Translate;
}) {
  const {
    canvasRef,
    mountRef,
    terminalTextInputRef,
    terminalComposing,
    mode,
    fontFamily,
    fontSize,
    lineHeight,
    padding,
    cursorBlink,
    showScrollbar,
    terminalTheme,
    wheelLines,
    boldMode,
    selectCopy,
    pasteSafe,
    resolvedTheme,
    ptyReady,
    terminalGeneration,
    nextTerminalGeneration,
    mainBrokerSessionIdRef,
    sessionClosePromise,
    restoringMode,
    setMainSessionIdentity,
    setTerminalFeedback,
    setQuery,
    setMode,
    showTerminalFeedback,
    showCopyNotice,
    t,
  } = options;

  const [terminalMounted, setTerminalMounted] = useState(false);
  /** R9-2 slice 5 · the terminal page's residency after the PTY child exits.
   *
   *  A completed run (or an exited interactive shell) must not tear the page
   *  down: the last frame stays painted and this notice states the exit code,
   *  so the user reads the output and closes the page themselves. `null` means
   *  no resident state — the normal live view. */
  const [terminalResident, setTerminalResident] = useState<{ code: number | null } | null>(null);
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
  const termOpened = useRef(false);
  const externalTerminalOpening = useRef(false);
  const clickSeq = useRef({ count: 0, time: 0, col: -1, row: -1 });

  const positionTerminalTextInput = () => {
    const renderer = rendererRef.current;
    const input = terminalTextInputRef.current;
    if (!renderer || !input) return;
    const cursor = renderer.cursorRect();
    input.style.transform = `translate(${cursor.x}px, ${cursor.y}px)`;
    input.style.height = `${cursor.height}px`;
  };

  const render = () => {
    const renderer = rendererRef.current;
    const frame = frameRef.current;
    if (renderer && frame) {
      renderer.draw(frame, blinkRef.current, selectionRef.current);
      positionTerminalTextInput();
    }
  };

  /**
   * Re-read the palette and repaint, on the spot. This is the coalesced unit
   * the transparency sliders schedule (see `repaintTerminalSoon`); every other
   * caller of `render()` — a frame arriving, a selection change, a resize —
   * stays synchronous, because those *do* change canvas pixels and are not
   * per-tick event storms.
   *
   * `updateTheme` before `draw`, never after: the renderer resolves the
   * terminal's own `--terminal-*` colours at `updateTheme` time, so a draw
   * first would paint the previous palette once.
   */
  const repaintTerminal = () => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    renderer.updateTheme();
    render();
  };

  /**
   * GLASS-CLIP: coalesce the palette repaint that a *continuous* control (the
   * window-transparency sliders) would otherwise run once per tick. A drag
   * used to call `updateTheme()` + `render()` on every +1, and the resulting
   * `getComputedStyle` + full canvas redraw starved the range input's own
   * event handling — the drag stuttered and dropped
   * (「透明度的滑杆拖动会中断」). The scheduler is trailing-edge and shared by
   * every slider move, so N ticks inside one window paint once.
   */
  const repaintScheduler = useRef<DeferredRepaint | null>(null);
  const repaintTerminalSoon = () => {
    repaintScheduler.current ??= createDeferredRepaint(repaintTerminal);
    repaintScheduler.current.schedule();
  };

  // `repaintTerminal` closes over `render`, which closes over `frameRef`,
  // `blinkRef` and `selectionRef` — all refs, so the closure is effectively
  // stable — but it is re-created on every render, so a consumer that depends
  // on it would re-run every render. This ref is the stable handle for the
  // *flush* consumers below, which must fire on a surface change and on
  // nothing else. (A flush on every render would not be wrong, only wasteful:
  // it would defeat the coalescing the scheduler exists for.)
  const repaintFlushRef = useRef<() => void>(() => {});
  repaintFlushRef.current = () => repaintScheduler.current?.flush();

  // A pending coalesced repaint is a *deferred* one, so it has to be landed
  // before the surface stops being painted. Three ways that can happen:
  //   * the window is hidden (blur → `hide_window`) — flush, so the reveal
  //     never shows the previous palette's frame for a beat;
  //   * the document is hidden — same;
  //   * the renderer is torn down (the effect below's cleanup, which every
  //     exit from the terminal surface funnels through) — flush *before*
  //     `rendererRef` is nulled, then cancel the timer so a later mode flip
  //     cannot run a stale repaint against a different renderer.
  useEffect(() => {
    const onVisibilityChange = () => {
      if (document.hidden) repaintFlushRef.current();
    };
    const onBlur = () => repaintFlushRef.current();
    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener("blur", onBlur);
    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener("blur", onBlur);
    };
  }, []);

  /** The frontend id keystrokes must reach: the main view's own session. */
  const terminalInputTarget = (): string => "main";

  /** The renderer whose emulator mode governs key encoding. */
  const activeRenderer = (): TerminalCanvas | null => rendererRef.current;

  /** Whether the session is able to receive input. */
  const surfaceReady = (): boolean => ptyReady.current;

  const focusTerminalView = (delay = 0) => {
    window.setTimeout(() => {
      relayoutAndResize();
      terminalTextInputRef.current?.focus({ preventScroll: true });
    }, delay);
  };

  const relayoutAndResize = () => {
    const renderer = rendererRef.current;
    const mount = mountRef.current;
    if (!renderer || !mount) return;
    const rect = mount.getBoundingClientRect();
    const layout = renderer.relayout(rect.width, rect.height);
    dimsRef.current = layout;
    positionTerminalTextInput();
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
    setTerminalResident(null);
    // R60 · the session is gone, so the bar's identity goes with it, and the
    // page is back to its empty state. The next spawn (or attach) describes
    // itself again. Nothing else reads this: `describeMainSession` is the only
    // writer on the way in, and the exit listener is the only other one.
    setMainSessionIdentity(null);
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
    // A fresh session replaces any resident (exited) view; the exit notice
    // belongs to the session that produced it.
    setTerminalResident(null);
    const { cols, rows } = dimsRef.current;
    const generation = ++nextTerminalGeneration.current;
    terminalGeneration.current = generation;
    try {
      // term_spawn hands back the daemon-side session id (see the Rust
      // command), remembered so a later resume can re-attach this PTY.
      const brokerSessionId = await invoke<string>("term_spawn", {
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
        mainBrokerSessionIdRef.current = brokerSessionId;
        void describeMainSession(brokerSessionId, initialCommand);
      }
    } catch (error) {
      if (terminalGeneration.current === generation) {
        terminalGeneration.current = null;
      }
      throw error;
    }
  };

  /**
   * The PTY child exited. R9-2 slice 5: this **holds** the page instead of
   * collapsing it — the final frame stays painted, input is gated off
   * (`ptyReady` false), and `terminalResident` drives a notice that names the
   * exit code. The user closes the page with the header's × or the configured
   * new-command shortcut (`returnToInputMode`), which is the only thing that
   * tears the view down.
   *
   * Mutation: call `closeTerminalSession()` + `setMode("collapsed")` here
   * again and the residency test (`terminal-command-execution`) goes red: the
   * page would vanish before the output could be read.
   */
  const handleTerminalExit = (code: number | null) => {
    // Release the input slot but keep the rendered frame and the mounted page.
    // Deliberately NOT `closeTerminalSession`: that resets `frameRef`, which
    // would erase the very output the user is meant to read.
    ptyReady.current = false;
    terminalGeneration.current = null;
    setTerminalResident({ code });
  };

  /** Fill the terminal bar's identity zone for `brokerSessionId`: the command
   * the session was launched with when present, else the broker's session
   * name, else the generic session title. The session list also reports the
   * exit state, so an attach of an already-dead session shows that instead of
   * a live dot. */
  const describeMainSession = async (brokerSessionId: string, initialCommand: string | null) => {
    const fallbackTitle = t("terminal.sessionTitle", { id: brokerSessionId.slice(0, 8) });
    try {
      const sessions = await invoke<BrokerSessionInfo[]>("term_list_sessions");
      const info = sessions.find((entry) => entry.sessionId === brokerSessionId);
      setMainSessionIdentity({
        title: initialCommand || info?.name || fallbackTitle,
        exited: info?.exited ?? false,
        exitCode: info?.exited ? info.exitCode : null,
      });
    } catch {
      setMainSessionIdentity({ title: initialCommand || fallbackTitle, exited: false, exitCode: null });
    }
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
      // Record the exit before the collapse: the identity zone carries the
      // running→exited transition, and any later describe of this session
      // would otherwise show a live dot for a dead PTY.
      setMainSessionIdentity((current) =>
        current ? { ...current, exited: true, exitCode: event.payload.code } : current,
      );
      handleTerminalExit(event.payload.code);
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

  // The renderer is bound to the canvas element of the mode that mounted it.
  // Keyed on `mode` too, so leaving the terminal page (for the clipboard page
  // or settings) tears the renderer down with its canvas and re-entering
  // builds a fresh one against the newly mounted node. Frames fully replace
  // each other and `frameRef` survives the flip, so the switch is lossless:
  // the last frame repaints immediately and the embedded PTY never stopped
  // running underneath.
  // Renderer lifecycle: creation, wheel listener, resize observer, blink
  // interval. Keyed only on `mode` and `terminalMounted` so font changes do
  // not tear down the renderer and rebind listeners — font changes are
  // handled by the effect below, which just re-measures cells and relayouts.
  useEffect(() => {
    if (!terminalMounted || mode === "settings") {
      termOpened.current = false;
      rendererRef.current = null;
      return;
    }
    if (mode !== "terminal") return;
    if (!canvasRef.current || !mountRef.current || termOpened.current) return;

    const renderer = new TerminalCanvas(canvasRef.current, {
      fontFamily: terminalFontFamily(fontFamily),
      fontSize: normalizeFontSize(fontSize),
      lineHeight: normalizeLineHeight(lineHeight),
      paddingX: padding,
      paddingY: padding,
      cursorBlink,
      showScrollbar,
      theme: terminalTheme,
      wheelLines,
      boldMode,
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

      const delta = wheelScrollSteps(event, renderer, wheelRemainder);
      if (delta === 0) return;

      const point = renderer.pixelToCell(event.offsetX, event.offsetY) ?? {
        col: Math.max(0, Math.min(renderer.cols - 1, Math.floor(event.offsetX / renderer.cellWidth))),
        row: Math.max(0, Math.min(renderer.rows - 1, Math.floor(event.offsetY / renderer.cellHeight))),
      };
      invoke("term_wheel", {
        id: "main",
        delta,
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
      // GLASS-CLIP: land a coalesced palette repaint before the renderer is
      // dropped. This cleanup is the single funnel every exit from the terminal
      // surface goes through — the settings panel unmounts the canvas, the
      // launcher and plugin pages hide it, the window closes — so the pending
      // repaint is landed here while `rendererRef` is still live, rather than
      // after it has been nulled (when it would be a no-op) or never (when the
      // timer would fire against a dead renderer). The cancel afterwards drops
      // the now-meaningless timer so a later mode flip cannot run a stale
      // repaint against whatever renderer exists then.
      repaintFlushRef.current();
      repaintScheduler.current?.cancel();
      termOpened.current = false;
      rendererRef.current = null;
    };
  }, [terminalMounted, mode]);

  // Font/geometry change: re-measure cells and relayout without rebuilding
  // the renderer or rebinding listeners. The renderer's relayout() already
  // calls measureCell() internally, so cell dimensions stay correct. R42 adds
  // line height and padding to the same axis: all four move the cell grid, so
  // all four go through `setOptions` + `relayout`. The frame, the selection and
  // the PTY's scrollback are untouched — nothing here closes the session.
  useEffect(() => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    renderer.setOptions({
      fontFamily: terminalFontFamily(fontFamily),
      fontSize: normalizeFontSize(fontSize),
      lineHeight: normalizeLineHeight(lineHeight),
      paddingX: padding,
      paddingY: padding,
    });
    relayoutAndResize();
  }, [fontFamily, fontSize, lineHeight, padding]);

  // R42 · the options that change pixels but not the grid: a cursor-blink veto,
  // the scrollbar switch and the canvas palette. These repaint in place and
  // deliberately do NOT relayout — a theme change must not resize the PTY.
  useEffect(() => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    renderer.setOptions({ cursorBlink, showScrollbar, theme: terminalTheme, wheelLines, boldMode });
    renderer.updateTheme();
    render();
  }, [cursorBlink, showScrollbar, terminalTheme, wheelLines, boldMode]);

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

  // ---- selection / scroll helpers ---------------------------------------

  const clampCell = (px: number, py: number): { col: number; row: number } | null => {
    const renderer = rendererRef.current;
    if (!renderer) return null;
    let col = Math.floor((px - renderer.paddingX) / renderer.cellWidth);
    let row = Math.floor((py - renderer.paddingY) / renderer.cellHeight);
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
    // R43 · copy on select: the drag that just ended was a text selection and
    // the user asked for it to land on the clipboard without a second gesture.
    // The selection itself stays highlighted (the renderer keeps painting it),
    // so this is additive to the explicit copy shortcut rather than a swap.
    const finishedSelection = drag.mode === "select" && selectionRef.current !== null;
    dragRef.current = { mode: "none" };
    window.removeEventListener("mousemove", onWindowMouseMove);
    window.removeEventListener("mouseup", onWindowMouseUp);
    if (finishedSelection && selectCopy) void copySelection();
  };

  const beginDrag = () => {
    window.addEventListener("mousemove", onWindowMouseMove);
    window.addEventListener("mouseup", onWindowMouseUp);
  };

  const onCanvasMouseDown = (e: React.MouseEvent) => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    terminalTextInputRef.current?.focus({ preventScroll: true });
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

  /** Read the system clipboard. The webview's own Clipboard API cannot be
   * relied on here — WebKitGTK ships without `navigator.clipboard`, and
   * WKWebView rejects programmatic reads outside its strict gesture policy —
   * so both directions go through the arboard-backed backend commands and
   * fall back to the JS API only where that exists (browser dev builds). */
  const readSystemClipboard = (): Promise<string> =>
    invoke<string>("clipboard_read_text").catch(() => navigator.clipboard.readText());

  /** Write the system clipboard; see `readSystemClipboard` for why this takes
   * the Rust path first. */
  const writeSystemClipboard = (text: string): Promise<void> =>
    invoke("clipboard_write_text", { text })
      .then(() => undefined)
      .catch(() => navigator.clipboard.writeText(text));

  const copySelection = async () => {
    const renderer = rendererRef.current;
    const sel = selectionRef.current;
    if (!renderer || !sel) return;
    const text = renderer.selectionText(sel);
    if (!text) {
      // A selection of nothing but blank cells (a drag across empty screen) has
      // nothing to put on the clipboard: that is not a failure, so it stays
      // silent rather than reporting a copy that never happened.
      return;
    }
    try {
      await writeSystemClipboard(text);
    } catch {
      // Clipboard unavailable; the selection remains highlighted. R44 · the
      // failed write is reported in the terminal's own status row — silence
      // here was the gap the user reported.
      showCopyNotice("terminal.copyNotice.failed");
      return;
    }
    // R44 · the success notice fires only after the clipboard write resolved,
    // so "Copied" can never be shown for a write that did not land.
    showCopyNotice("terminal.copyNotice.copied");
    // Where the copy shortcut is Ctrl-based it is also the shell's interrupt,
    // so the highlight is dropped after a copy: the next press then reaches
    // the shell instead of copying the same text again. macOS copies with Cmd
    // and keeps its selection.
    if (!IS_MAC) {
      selectionRef.current = null;
      render();
    }
  };

  const sendTerminalText = (text: string, bracketed = false, isPaste = false) => {
    // Gated on the *target* surface's readiness, matching `terminalInputTarget`
    // below: typed text, IME commits and pastes all go to the card while it owns
    // the keyboard, and the main slot is empty precisely then.
    if (!text || !surfaceReady()) return;
    // R43 · safe paste: one trailing break is the newline a copied command
    // carried, and leaving it in would run the command before it was read. The
    // trim happens before the space normalization so both boundaries share one
    // entry point.
    const pasted = isPaste && pasteSafe ? stripPasteNewline(text) : text;
    // Injection boundary: whatever produced this string (hidden-textarea
    // commit on macOS WebKit, an IME composition, the system clipboard), a
    // Unicode space separator here would fuse two shell words into one —
    // `go\u3000version` reads to zsh as a single token named "go version".
    // Every Unicode Zs except the ASCII space becomes U+0020 before encode.
    const normalized = normalizeTerminalInputSpaces(pasted);
    const payload = bracketed ? `\x1b[200~${normalized}\x1b[201~` : normalized;
    void invoke("term_input", {
      id: terminalInputTarget(),
      data: Array.from(new TextEncoder().encode(payload)),
    });
  };

  const flushTerminalTextInput = (bracketed = false, isPaste = false) => {
    const input = terminalTextInputRef.current;
    if (!input || terminalComposing.current || !input.value) return;
    const text = input.value;
    input.value = "";
    sendTerminalText(text, bracketed, isPaste);
  };

  const onTerminalTextInput = (event: React.FormEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as InputEvent;
    if (nativeEvent.isComposing || terminalComposing.current) return;
    const isPaste = nativeEvent.inputType === "insertFromPaste";
    const bracketedPaste =
      isPaste && Boolean((activeRenderer()?.mode ?? 0) & BRACKETED_PASTE);
    flushTerminalTextInput(bracketedPaste, isPaste);
  };

  const pasteClipboard = async () => {
    const renderer = activeRenderer();
    if (!renderer) return;
    let text = "";
    try {
      text = await readSystemClipboard();
    } catch {
      return;
    }
    if (!text) return;
    sendTerminalText(text, (renderer.mode & BRACKETED_PASTE) !== 0, true);
  };

  return {
    terminalMounted,
    setTerminalMounted,
    terminalResident,
    rendererRef,
    dimsRef,
    selectionRef,
    render,
    repaintTerminal,
    repaintTerminalSoon,
    repaintFlushRef,
    terminalInputTarget,
    activeRenderer,
    surfaceReady,
    focusTerminalView,
    relayoutAndResize,
    resetTerminalFrontendState,
    closeTerminalSession,
    ensureTerminalSession,
    describeMainSession,
    openInTerminal,
    copySelection,
    pasteClipboard,
    onCanvasMouseDown,
    onCanvasMouseMove,
    onTerminalTextInput,
    flushTerminalTextInput,
  };
}
