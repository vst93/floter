// React binding for the main terminal view: renderer lifecycle, frame/exit
// events, mouse/selection/wheel handling and the IME proxy textarea.
//
// Extracted verbatim from `App.tsx`; the hook receives every App-owned ref and
// setter it touches, so the behaviour is unchanged.
//
// Note the deliberate scope: the broker-side session bookkeeping refs
// (`ptyReady`, `terminalGeneration`, ...) stay in `App.tsx` because the pin
// coordinator and the session resume path share them.

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
import { TerminalCanvas, decodeFrame, type Selection } from "../terminal/render";
import { MOUSE_MOTION, usesMouseReporting } from "../terminal/keys";
import { PINNED_SESSION_ID } from "../terminal/pinState";
import { normalizeFontSize } from "../settings/GeneralPage";
import { IS_MAC } from "../shortcuts";
import type { ExecutionPlan } from "../launcher";
import type { BrokerSessionInfo, MainSessionIdentity, ViewMode } from "../App";
import type { MessageKey, Translate } from "../i18n";

// 'DejaVu Sans Mono' and 'Liberation Mono' are the monospace faces actually
// present on Linux desktops; without them the stack falls through to a generic
// `monospace` whose fontconfig match is frequently not a terminal face at all.
const FALLBACK_FONT_FAMILY =
  "'SF Mono','Menlo','Monaco','Consolas','JetBrains Mono','DejaVu Sans Mono','Liberation Mono',monospace";
const LINE_HEIGHT = 1.4;
const PADDING_X = 3;
const PADDING_Y = 3;
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
  resolvedTheme: "dark" | "light";
  ptyReady: RefObject<boolean>;
  terminalGeneration: RefObject<number | null>;
  nextTerminalGeneration: RefObject<number>;
  mainBrokerSessionIdRef: RefObject<string | null>;
  pinnedRendererRef: RefObject<TerminalCanvas | null>;
  activeSurfaceRef: RefObject<"main" | "pinned">;
  setActiveSurface: (surface: "main" | "pinned") => void;
  sessionClosePromise: RefObject<Promise<unknown> | null>;
  restoringMode: RefObject<ViewMode | null>;
  setMainSessionIdentity: Dispatch<SetStateAction<MainSessionIdentity | null>>;
  setMainPinnedAway: (value: boolean) => void;
  setTerminalFeedback: Dispatch<SetStateAction<MessageKey | null>>;
  setQuery: Dispatch<SetStateAction<string>>;
  setMode: (mode: ViewMode) => void;
  focusCollapsedInput: (delay?: number) => void;
  showTerminalFeedback: (key: MessageKey) => void;
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
    resolvedTheme,
    ptyReady,
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
    focusCollapsedInput,
    showTerminalFeedback,
    t,
  } = options;

  const [terminalMounted, setTerminalMounted] = useState(false);
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

  /** The frontend id keystrokes must reach right now: the main view, or the
   * pinned card after its body was clicked. */
  const terminalInputTarget = (): string =>
    activeSurfaceRef.current === "pinned" ? PINNED_SESSION_ID : "main";

  /** The renderer whose emulator mode governs key encoding for the active
   * surface — the two views can run programs with different modes. */
  const activeRenderer = (): TerminalCanvas | null =>
    activeSurfaceRef.current === "pinned" ? pinnedRendererRef.current : rendererRef.current;

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
      // term_spawn hands back the daemon-side session id (see the Rust
      // command), remembered so pinning can re-attach this PTY to the card.
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
        setMainPinnedAway(false);
        void describeMainSession(brokerSessionId, initialCommand);
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

  // The renderer is bound to the canvas element of the mode that mounted it.
  // Keyed on `mode` too, so leaving the terminal page (for the clipboard page
  // or settings) tears the renderer down with its canvas and re-entering
  // builds a fresh one against the newly mounted node. Frames fully replace
  // each other and `frameRef` survives the flip, so the switch is lossless:
  // the last frame repaints immediately and the embedded PTY never stopped
  // running underneath.
  useEffect(() => {
    if (!terminalMounted || mode === "plugin" || mode === "settings") {
      termOpened.current = false;
      rendererRef.current = null;
      return;
    }
    if (mode !== "terminal") return;
    if (!canvasRef.current || !mountRef.current || termOpened.current) return;

    const renderer = new TerminalCanvas(canvasRef.current, {
      fontFamily: terminalFontFamily(fontFamily),
      fontSize: normalizeFontSize(fontSize),
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
  }, [fontFamily, fontSize, terminalMounted, mode]);

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
    // Clicking the main area always reclaims the keyboard from the card.
    setActiveSurface("main");
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
    if (text) {
      try {
        await writeSystemClipboard(text);
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

  const sendTerminalText = (text: string, bracketed = false) => {
    if (!text || !ptyReady.current) return;
    const payload = bracketed ? `\x1b[200~${text}\x1b[201~` : text;
    void invoke("term_input", {
      id: terminalInputTarget(),
      data: Array.from(new TextEncoder().encode(payload)),
    });
  };

  const flushTerminalTextInput = (bracketed = false) => {
    const input = terminalTextInputRef.current;
    if (!input || terminalComposing.current || !input.value) return;
    const text = input.value;
    input.value = "";
    sendTerminalText(text, bracketed);
  };

  const onTerminalTextInput = (event: React.FormEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as InputEvent;
    if (nativeEvent.isComposing || terminalComposing.current) return;
    const bracketedPaste =
      nativeEvent.inputType === "insertFromPaste" &&
      Boolean((activeRenderer()?.mode ?? 0) & BRACKETED_PASTE);
    flushTerminalTextInput(bracketedPaste);
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
    sendTerminalText(text, (renderer.mode & BRACKETED_PASTE) !== 0);
  };

  return {
    terminalMounted,
    setTerminalMounted,
    rendererRef,
    dimsRef,
    selectionRef,
    render,
    terminalInputTarget,
    activeRenderer,
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
