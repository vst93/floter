// R55 · the independent pinned-terminal window.
//
// The user asked for the pin to be its own window (「独立出来并固定住，不是只能在
// 终端页面内小窗」). Rust opens a second native window labelled
// `pinned-terminal` and loads this document with `?pinned=<brokerSessionId>`;
// `main.tsx` renders this component instead of `<App />` for that window.
//
// The window owns no PTY. It attaches a *view* to the same broker session under
// the frontend id `pinned` (`term_attach_existing`) — exactly what the retired
// in-window card used to do — so the session, its scrollback and its running
// process all survive the move. Frames arrive over the manager's global
// `term://frame` broadcast; this view filters them by id + generation and draws
// them with its own renderer instance. Closing the window detaches the view (the
// PTY keeps running) and Rust tells the main window to take the session back.

import { useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { TerminalCanvas, decodeFrame, wheelScrollSteps } from "./terminal/render";
import { PINNED_SESSION_ID, clampCardGeometry, loadPinnedGeometry } from "./terminal/pinState";
import { encodeKey, shouldUseTerminalTextInput } from "./terminal/keys";
import { normalizeTerminalInputSpaces } from "./terminal/inputNormalize";
import {
  normalizeBoldMode,
  normalizeCursorBlink,
  normalizeFontSize,
  normalizeLineHeight,
  normalizeScrollbar,
  normalizeTerminalTheme,
  normalizeWheelLines,
  terminalPaddingPx,
} from "./terminal/terminal-appearance";
import { createTranslator, normalizeLanguage } from "./i18n";
import type { AppSettings } from "./App";
import "./styles/launcher.css";
import "./styles/terminal.css";
import "./styles/base.css";

type FramePayload = { id: string; generation: number; frame: string };
type ExitPayload = { id: string; generation: number; code: number | null };

/** Which session this window was opened for, from its own URL, or `null` when
 *  the document was loaded with no `?pinned=`. */
export const pinnedSessionFromLocation = (search: string): string | null => {
  const value = new URLSearchParams(search).get("pinned");
  return value && value.trim() ? value.trim() : null;
};

export function isPinnedWindowLocation(search: string): boolean {
  return pinnedSessionFromLocation(search) !== null;
}

export function PinnedTerminalWindowApp({ brokerSessionId }: { brokerSessionId: string }) {
  const mountRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const rendererRef = useRef<TerminalCanvas | null>(null);
  const frameRef = useRef<Uint8Array | null>(null);
  const blinkRef = useRef(true);
  const wheelRemainder = useRef(0);
  const textInputRef = useRef<HTMLTextAreaElement>(null);
  const composing = useRef(false);
  const attached = useRef(false);

  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [systemDark, setSystemDark] = useState(
    () => typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: dark)").matches,
  );
  const [generation, setGeneration] = useState<number | null>(null);
  const [label, setLabel] = useState<string | null>(null);

  usePinnedWindowGeometryPersistence();

  // Restore the user's last window geometry (screen coordinates) on mount.
  useEffect(() => {
    const saved = loadPinnedGeometry(window.localStorage);
    if (!saved) return;
    const bounds = { width: window.screen.availWidth, height: window.screen.availHeight };
    const clamped = clampCardGeometry(saved, bounds.width, bounds.height);
    void invoke("set_pinned_terminal_geometry", clamped as unknown as Record<string, number>).catch(() => undefined);
  }, []);

  useEffect(() => {
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setSystemDark(query.matches);
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);

  const resolvedTheme: "dark" | "light" =
    settings?.theme === "dark" || settings?.theme === "light"
      ? settings.theme
      : systemDark
        ? "dark"
        : "light";

  const t = useMemo(
    () => createTranslator(normalizeLanguage(settings?.language ?? "en")),
    [settings?.language],
  );

  // The palette overrides in base.css are attribute selectors on `html`, so the
  // resolved theme has to be written where the main window writes it.
  useEffect(() => {
    document.documentElement.dataset.theme = resolvedTheme;
  }, [resolvedTheme]);

  // Attach the view to the broker session and load appearance settings.
  useEffect(() => {
    let alive = true;
    void (async () => {
      let loaded: AppSettings | null = null;
      try {
        loaded = await invoke<AppSettings>("get_settings");
      } catch {
        loaded = null;
      }
      if (!alive) return;
      setSettings(loaded);
      const theme =
        loaded?.theme === "dark" || loaded?.theme === "light"
          ? loaded.theme
          : window.matchMedia("(prefers-color-scheme: dark)").matches
            ? "dark"
            : "light";

      const nextGeneration = Date.now();
      try {
        await invoke("term_attach_existing", {
          request: {
            id: PINNED_SESSION_ID,
            generation: nextGeneration,
            brokerSessionId,
            theme,
            cols: 80,
            rows: 24,
          },
        });
      } catch {
        // The session is gone; keep the window (the close path still returns it).
        return;
      }
      if (!alive) return;
      attached.current = true;
      setGeneration(nextGeneration);
      try {
        const sessions = await invoke<{ sessionId: string; name: string }[]>("term_list_sessions");
        if (alive) {
          setLabel(sessions.find((entry) => entry.sessionId === brokerSessionId)?.name ?? null);
        }
      } catch {
        // A missing label only costs the header its name.
      }
    })();
    return () => {
      alive = false;
    };
  }, [brokerSessionId]);

  const draw = () => {
    const renderer = rendererRef.current;
    const frame = frameRef.current;
    if (renderer && frame) renderer.draw(frame, blinkRef.current, null);
  };

  // Own renderer, re-created only when the font/appearance axes change.
  useEffect(() => {
    const canvas = canvasRef.current;
    const mount = mountRef.current;
    if (!canvas || !mount || !settings || generation === null) return;

    const renderer = new TerminalCanvas(canvas, {
      fontFamily: settings.font_family,
      fontSize: normalizeFontSize(settings.font_size),
      lineHeight: normalizeLineHeight(settings.terminal_line_height),
      paddingX: terminalPaddingPx(settings.terminal_padding),
      paddingY: terminalPaddingPx(settings.terminal_padding),
      cursorBlink: normalizeCursorBlink(settings.terminal_cursor_blink),
      showScrollbar: normalizeScrollbar(settings.terminal_scrollbar),
      theme: normalizeTerminalTheme(settings.terminal_theme),
      wheelLines: normalizeWheelLines(settings.terminal_wheel_lines),
      boldMode: normalizeBoldMode(settings.terminal_bold),
    });
    rendererRef.current = renderer;

    const relayout = () => {
      const rect = mount.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) return;
      const layout = renderer.relayout(rect.width, rect.height);
      void invoke("term_resize", {
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
  }, [settings, generation]);

  // Frame + exit streams for this view only.
  useEffect(() => {
    if (generation === null) return;
    let alive = true;
    let unlistenFrame: (() => void) | null = null;
    let unlistenExit: (() => void) | null = null;
    const generationId = generation;
    const framePromise = listen<FramePayload>("term://frame", (event) => {
      if (event.payload.id !== PINNED_SESSION_ID || event.payload.generation !== generationId) return;
      frameRef.current = decodeFrame(event.payload.frame);
      blinkRef.current = true;
      draw();
    }).then((unlisten) => {
      if (alive) unlistenFrame = unlisten;
      else unlisten();
    });
    const exitPromise = listen<ExitPayload>("term://exit", (event) => {
      if (event.payload.id !== PINNED_SESSION_ID || event.payload.generation !== generationId) return;
      void invoke("close_pinned_terminal").catch(() => undefined);
    }).then((unlisten) => {
      if (alive) unlistenExit = unlisten;
      else unlisten();
    });
    return () => {
      alive = false;
      void framePromise.then(() => unlistenFrame?.());
      void exitPromise.then(() => unlistenExit?.());
    };
  }, [generation]);

  // Detach the view (the PTY keeps running), then close the native window. Rust
  // tells the main window so it can take the session back.
  const closeWindow = () => {
    if (attached.current) {
      void invoke("term_close", { id: PINNED_SESSION_ID }).catch(() => undefined);
      attached.current = false;
    }
    void invoke("close_pinned_terminal").catch(() => undefined);
  };

  // Keyboard input. `encodeKey` is the main view's encoder; printable/IME keys
  // go through a hidden textarea so dead keys and composition work.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.isComposing || event.keyCode === 229) return;
      if (shouldUseTerminalTextInput(event)) {
        textInputRef.current?.focus();
        return;
      }
      const bytes = encodeKey(event, rendererRef.current?.mode ?? 0);
      if (!bytes) return;
      event.preventDefault();
      event.stopPropagation();
      void invoke("term_input", { id: PINNED_SESSION_ID, data: Array.from(bytes) }).catch(
        () => undefined,
      );
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, []);

  const flushTextInput = () => {
    const input = textInputRef.current;
    if (!input || composing.current || !input.value) return;
    const text = input.value;
    input.value = "";
    void invoke("term_input", {
      id: PINNED_SESSION_ID,
      data: Array.from(new TextEncoder().encode(normalizeTerminalInputSpaces(text))),
    }).catch(() => undefined);
  };

  // Wheel scrolls this session's history / reports to mouse-mode programs.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    wheelRemainder.current = 0;
    const onWheel = (event: WheelEvent) => {
      const renderer = rendererRef.current;
      if (!renderer || event.deltaY === 0) return;
      event.preventDefault();
      const delta = wheelScrollSteps(event, renderer, wheelRemainder);
      if (delta === 0) return;
      void invoke("term_wheel", {
        id: PINNED_SESSION_ID,
        delta,
        column: 0,
        row: 0,
        modifiers: 0,
      }).catch(() => undefined);
    };
    canvas.addEventListener("wheel", onWheel, { passive: false });
    return () => canvas.removeEventListener("wheel", onWheel);
  }, [generation]);

  if (!settings || generation === null) {
    return (
      <div className="pinned-window" data-pinned-window data-theme={resolvedTheme}>
        <div className="pinned-window__placeholder">{t("terminal.pinnedLoading")}</div>
      </div>
    );
  }

  const title =
    label || t("terminal.sessionTitle", { id: brokerSessionId.slice(0, 8) });

  return (
    <div
      className="pinned-window"
      data-pinned-window
      data-theme={resolvedTheme}
      onPointerDown={() => textInputRef.current?.focus()}
    >
      <header className="pinned-window__bar" data-tauri-drag-region>
        <span className="pinned-window__dot" aria-hidden="true" />
        <span className="pinned-window__title" data-tauri-drag-region>{title}</span>
        <button
          type="button"
          className="pinned-window__close"
          aria-label={t("terminal.pinnedClose")}
          title={t("terminal.pinnedClose")}
          onClick={closeWindow}
        >
          ×
        </button>
      </header>
      <div ref={mountRef} className="pinned-window__mount">
        <canvas ref={canvasRef} className="pinned-window__canvas" />
      </div>
      <textarea
        ref={textInputRef}
        className="terminal-text-input pinned-window__text-input"
        aria-label={t("terminal.input")}
        rows={1}
        spellCheck={false}
        autoCapitalize="off"
        autoCorrect="off"
        onInput={() => flushTextInput()}
        onCompositionStart={() => {
          composing.current = true;
        }}
        onCompositionEnd={() => {
          composing.current = false;
          queueMicrotask(() => flushTextInput());
        }}
      />
    </div>
  );
}

/** Persist the native window's own move/resize so it reopens where it was. */
export function usePinnedWindowGeometryPersistence() {
  useEffect(() => {
    const win = getCurrentWindow();
    const save = async () => {
      try {
        const [position, size] = await Promise.all([win.outerPosition(), win.outerSize()]);
        const scale = await win.scaleFactor();
        window.localStorage.setItem(
          "floter.pinned-terminal.geometry",
          JSON.stringify({
            x: position.x / scale,
            y: position.y / scale,
            width: size.width / scale,
            height: size.height / scale,
          }),
        );
      } catch {
        // An unsaved position only costs the user their last drag.
      }
    };
    const unlistenMoved = win.onMoved(() => void save());
    const unlistenResized = win.onResized(() => void save());
    return () => {
      void unlistenMoved.then((fn) => fn());
      void unlistenResized.then((fn) => fn());
    };
  }, []);
}
