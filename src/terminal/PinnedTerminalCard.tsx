// The pinnable floating terminal card.
//
// LIVE RENDERING CHOICE (preferred option 1 of the feature contract):
// `TerminalCanvas` in `render.ts` is instantiable per surface — its constructor
// takes any <canvas> and holds no global state — so this card creates a SECOND
// renderer instance subscribed to the pinned session's frame stream
// (`term://frame` events with id "pinned") instead of reparenting the main
// view's canvas. That gives the card its own cell metrics, scrollbar and blink
// phase at card size, with zero risk to the main view's layout. Nothing is
// dropped on either side: each Rust-side session owns a full emulator grid and
// every frame is a complete repaint of it, so the two streams are independent
// and a frame nobody drew is simply superseded by the next one.

import {
  useCallback,
  useEffect,
  useRef,
  type MutableRefObject,
  type PointerEvent as ReactPointerEvent,
} from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { TerminalCanvas, decodeFrame } from "./render";
import { PINNED_SESSION_ID, type CardGeometry, type PinnedSession } from "./pinState";
import type { Translate } from "../i18n";

const LINE_HEIGHT = 1.4;
const PADDING_X = 3;
const PADDING_Y = 3;
const RESIZE_HANDLE_SIZE = 14;

type FramePayload = { id: string; generation: number; frame: string };
type ExitPayload = { id: string; generation: number; code: number | null };

export interface PinnedTerminalCardProps {
  session: PinnedSession;
  fontFamily: string;
  fontSize: number;
  /** Current resolved theme ("dark" | "light"); a change repaints in place. */
  theme: string;
  geometry: CardGeometry;
  onGeometryChange: (geometry: CardGeometry) => void;
  focused: boolean;
  /** Rendered but invisible while the launcher/settings own the window. */
  hidden: boolean;
  onClose: () => void;
  /** Body click: make the pinned session the active input target. */
  onFocusRequest: () => void;
  /** Underlying PTY exited; the card must go away gracefully. */
  onSessionExit: () => void;
  /** Out-parameter: the card's renderer, for mode-aware key encoding. */
  rendererRef: MutableRefObject<TerminalCanvas | null>;
  t: Translate;
}

export function PinnedTerminalCard({
  session,
  fontFamily,
  fontSize,
  theme,
  geometry,
  onGeometryChange,
  focused,
  hidden,
  onClose,
  onFocusRequest,
  onSessionExit,
  rendererRef,
  t,
}: PinnedTerminalCardProps) {
  const mountRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const frameRef = useRef<Uint8Array | null>(null);
  const blinkRef = useRef(true);
  const geometryRef = useRef(geometry);
  const sessionRef = useRef(session);
  geometryRef.current = geometry;
  sessionRef.current = session;

  const draw = useCallback(() => {
    const renderer = rendererRef.current;
    const frame = frameRef.current;
    if (renderer && frame) renderer.draw(frame, blinkRef.current, null);
  }, [rendererRef]);

  // Own renderer instance for this surface (see the choice note above).
  // Rebuilt only when the font changes — the same trade-off the main view
  // makes. While `hidden` (launcher owns the window) rendering continues:
  // frames keep landing so nothing shown after re-expanding is stale.
  useEffect(() => {
    const canvas = canvasRef.current;
    const mount = mountRef.current;
    if (!canvas || !mount) return;

    const renderer = new TerminalCanvas(canvas, {
      fontFamily,
      fontSize,
      lineHeight: LINE_HEIGHT,
      paddingX: PADDING_X,
      paddingY: PADDING_Y,
    });
    rendererRef.current = renderer;

    // Keep the PTY grid matched to the card. A resize while pinned resizes
    // *this* session's PTY only — the main view always runs its own session.
    const relayout = () => {
      const rect = mount.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) return;
      const layout = renderer.relayout(rect.width, rect.height);
      invoke("term_resize", {
        id: PINNED_SESSION_ID,
        cols: layout.cols,
        rows: layout.rows,
      }).catch(() => undefined);
      draw();
    };
    relayout();
    const observer = new ResizeObserver(relayout);
    observer.observe(mount);

    const blink = window.setInterval(() => {
      blinkRef.current = !blinkRef.current;
      draw();
    }, 530);

    return () => {
      window.clearInterval(blink);
      observer.disconnect();
      rendererRef.current = null;
    };
  }, [draw, fontFamily, fontSize, rendererRef]);

  // Theme switches repaint the existing frames instead of rebuilding.
  useEffect(() => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    renderer.updateTheme();
    draw();
  }, [draw, rendererRef, theme]);

  // Frame + exit streams for THIS session view only. `onSessionExit` is read
  // through a ref so a new closure on every parent render cannot resubscribe
  // the listeners.
  const onSessionExitRef = useRef(onSessionExit);
  onSessionExitRef.current = onSessionExit;
  useEffect(() => {
    let alive = true;
    let unlistenFrame: (() => void) | null = null;
    let unlistenExit: (() => void) | null = null;

    const generation = session.generation;
    const framePromise = listen<FramePayload>("term://frame", (event) => {
      if (event.payload.id !== PINNED_SESSION_ID || event.payload.generation !== generation) return;
      frameRef.current = decodeFrame(event.payload.frame);
      blinkRef.current = true;
      draw();
    }).then((unlisten) => {
      if (alive) unlistenFrame = unlisten;
      else unlisten();
    });
    const exitPromise = listen<ExitPayload>("term://exit", (event) => {
      if (event.payload.id !== PINNED_SESSION_ID || event.payload.generation !== generation) return;
      onSessionExitRef.current();
    }).then((unlisten) => {
      if (alive) unlistenExit = unlisten;
      else unlisten();
    });

    return () => {
      alive = false;
      void framePromise.then(() => {
        unlistenFrame?.();
      });
      void exitPromise.then(() => {
        unlistenExit?.();
      });
    };
  }, [draw, session.generation]);

  // ---- drag (header) + resize (corner handle) ------------------------------

  const beginGesture = (
    event: ReactPointerEvent<HTMLElement>,
    apply: (start: CardGeometry, dx: number, dy: number) => CardGeometry,
  ) => {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    const target = event.currentTarget;
    const startX = event.clientX;
    const startY = event.clientY;
    const startGeometry = geometryRef.current;
    const onMove = (move: PointerEvent) => {
      onGeometryChange(apply(startGeometry, move.clientX - startX, move.clientY - startY));
    };
    const onUp = () => {
      target.removeEventListener("pointermove", onMove);
      target.removeEventListener("pointerup", onUp);
      target.removeEventListener("pointercancel", onUp);
    };
    target.setPointerCapture(event.pointerId);
    target.addEventListener("pointermove", onMove);
    target.addEventListener("pointerup", onUp);
    target.addEventListener("pointercancel", onUp);
  };

  const onHeaderPointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    if ((event.target as HTMLElement).closest("button")) return;
    onFocusRequest();
    beginGesture(event, (start, dx, dy) => ({
      ...start,
      x: start.x + dx,
      y: start.y + dy,
    }));
  };

  const onResizeHandlePointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    beginGesture(event, (start, dx, dy) => ({
      ...start,
      width: start.width + dx,
      height: start.height + dy,
    }));
  };

  // Wheel scrolls the pinned session's history / reports to mouse-mode programs.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const onWheel = (event: WheelEvent) => {
      const renderer = rendererRef.current;
      if (!renderer || event.deltaY === 0) return;
      event.preventDefault();
      const unit = Math.max(24, renderer.cellHeight * 1.5);
      const steps = Math.trunc(event.deltaY / unit);
      if (steps === 0) return;
      invoke("term_wheel", {
        id: PINNED_SESSION_ID,
        delta: Math.max(-8, Math.min(8, -steps)),
        column: 0,
        row: 0,
        modifiers: 0,
      }).catch(() => undefined);
    };
    canvas.addEventListener("wheel", onWheel, { passive: false });
    return () => canvas.removeEventListener("wheel", onWheel);
  }, [rendererRef]);

  const title =
    session.label ||
    t("terminal.sessionTitle", { id: session.brokerSessionId.slice(0, 8) });

  return (
    <section
      className={`pinned-card${focused ? " pinned-card--focused" : ""}${hidden ? " pinned-card--hidden" : ""}`}
      style={{
        left: geometry.x,
        top: geometry.y,
        width: geometry.width,
        height: geometry.height,
      }}
      role="dialog"
      aria-label={title}
      data-pinned-card
    >
      <header className="pinned-card__header" onPointerDown={onHeaderPointerDown}>
        <span className="pinned-card__dot" aria-hidden="true" />
        <span className="pinned-card__title">{title}</span>
        <button
          type="button"
          className="pinned-card__close"
          aria-label={t("terminal.pinnedClose")}
          title={t("terminal.pinnedClose")}
          onClick={(event) => {
            event.stopPropagation();
            onClose();
          }}
          onPointerDown={(event) => event.stopPropagation()}
        >
          ×
        </button>
      </header>
      <div ref={mountRef} className="pinned-card__body" onPointerDown={onFocusRequest}>
        <canvas ref={canvasRef} className="pinned-card__canvas" />
      </div>
      <div
        className="pinned-card__resize-handle"
        aria-label={t("terminal.pinnedResize")}
        onPointerDown={onResizeHandlePointerDown}
        style={{ width: RESIZE_HANDLE_SIZE, height: RESIZE_HANDLE_SIZE }}
      />
    </section>
  );
}
