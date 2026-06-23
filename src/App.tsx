import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "@xterm/xterm/css/xterm.css";
import "./App.css";

type ViewMode = "collapsed" | "terminal";

type AppSettings = {
  hotkey: string;
  hide_on_blur: boolean;
  theme: string;
  font_size: number;
  font_family: string;
};

const THEME = {
  background: "#101216",
  foreground: "#d7dae0",
  cursor: "#8bd5ca",
  cursorAccent: "#101216",
  selectionBackground: "#2c3140",
  black: "#4b5563",
  red: "#f38ba8",
  green: "#a6e3a1",
  yellow: "#f9e2af",
  blue: "#89b4fa",
  magenta: "#f5c2e7",
  cyan: "#94e2d5",
  white: "#cdd6f4",
  brightBlack: "#6b7280",
  brightRed: "#f38ba8",
  brightGreen: "#a6e3a1",
  brightYellow: "#f9e2af",
  brightBlue: "#89b4fa",
  brightMagenta: "#f5c2e7",
  brightCyan: "#94e2d5",
  brightWhite: "#eef2ff",
};

export default function App() {
  const inputRef = useRef<HTMLInputElement>(null);
  const termRef = useRef<HTMLDivElement>(null);
  const xterm = useRef<Terminal | null>(null);
  const fit = useRef<FitAddon | null>(null);
  const ptyReady = useRef(false);
  const termOpened = useRef(false);
  const pendingCommand = useRef<string | null>(null);
  const draftBeforeHistory = useRef("");
  const restoringMode = useRef<ViewMode | null>(null);

  const [mode, setMode] = useState<ViewMode>("collapsed");
  const [query, setQuery] = useState("");
  const [terminalMounted, setTerminalMounted] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [settings, setSettings] = useState<AppSettings>({
    hotkey: "Ctrl+Shift+Space",
    hide_on_blur: true,
    theme: "dark",
    font_size: 14,
    font_family: "monospace",
  });
  const suppressBlurUntil = useRef(0);

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
      fit.current?.fit();
      const dimensions = fit.current?.proposeDimensions();
      if (dimensions) {
        invoke("pty_resize", { id: "main", rows: dimensions.rows, cols: dimensions.cols });
      }
      xterm.current?.focus();
    }, delay);
  };

  const flushPendingCommand = (delay = 0) => {
    if (!pendingCommand.current) return;

    const command = pendingCommand.current;
    pendingCommand.current = null;

    window.setTimeout(() => {
      invoke("pty_write", {
        id: "main",
        data: Array.from(new TextEncoder().encode(`${command}\n`)),
      });
      focusTerminalView();
    }, delay);
  };

  const createTerminal = () => {
    xterm.current?.dispose();

    const terminal = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "'SF Mono','Menlo','Monaco','Consolas',monospace",
      theme: THEME,
      lineHeight: 1.35,
      scrollback: 10000,
      allowTransparency: false,
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.loadAddon(new WebLinksAddon());

    terminal.onData((data) => {
      invoke("pty_write", {
        id: "main",
        data: Array.from(new TextEncoder().encode(data)),
      });
    });

    xterm.current = terminal;
    fit.current = fitAddon;
    return { terminal, fitAddon };
  };

  useEffect(() => {
    invoke<AppSettings>("get_settings")
      .then(setSettings)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    if (ptyReady.current) return;
    ptyReady.current = true;

    invoke("pty_spawn", { id: "main", shell: null });

    const unlistenPromise = listen<[string, number[]]>("pty-output", (event) => {
      if (event.payload[0] === "main") {
        xterm.current?.write(new TextDecoder().decode(new Uint8Array(event.payload[1])));
      }
    });

    return () => {
      unlistenPromise.then((unlisten) => unlisten());
      xterm.current?.dispose();
      xterm.current = null;
      fit.current = null;
      termOpened.current = false;
      pendingCommand.current = null;
    };
  }, []);

  useEffect(() => {
    if (!terminalMounted) {
      termOpened.current = false;
      xterm.current?.dispose();
      xterm.current = null;
      fit.current = null;
      return;
    }
    if (!termRef.current || termOpened.current) return;

    const terminalElement = termRef.current;
    const { terminal, fitAddon } = createTerminal();

    terminal.open(terminalElement);
    termOpened.current = true;

    const syncSize = () => {
      fitAddon.fit();
      const dimensions = fitAddon.proposeDimensions();
      if (dimensions) {
        invoke("pty_resize", { id: "main", rows: dimensions.rows, cols: dimensions.cols });
      }
    };

    const resizeObserver = new ResizeObserver(syncSize);
    resizeObserver.observe(terminalElement);
    syncSize();
    flushPendingCommand(40);

    return () => {
      resizeObserver.disconnect();
      termOpened.current = false;
      terminal.dispose();
      if (xterm.current === terminal) {
        xterm.current = null;
        fit.current = null;
      }
    };
  }, [terminalMounted]);

  useEffect(() => {
    const isRestoring = restoringMode.current === mode;
    suppressBlurUntil.current = Date.now() + 400;

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
        pendingCommand.current = null;
        setQuery("");
        setTerminalMounted(false);
        setMode("collapsed");
      }
    });

    const unlistenRevealPromise = listen<string>("floter://revealed", (event) => {
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
        } else {
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

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const key = event.key.toLowerCase();
      const hasCommandModifier = event.metaKey || event.ctrlKey;

      if (mode === "terminal" && hasCommandModifier && !event.altKey && (key === "w" || key === "n")) {
        event.preventDefault();
        returnToInputMode();
        return;
      }

      if (mode !== "collapsed") return;

      if (event.key === "Escape") {
        event.preventDefault();
        if (query.trim()) {
          setQuery("");
          setHistoryIndex(-1);
          return;
        }
        invoke("hide_window");
        return;
      }

      const inputFocused = document.activeElement === inputRef.current;
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
  }, [mode, query]);

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
    pendingCommand.current = null;
    setQuery("");
    setTerminalMounted(false);
    setMode("collapsed");
  };

  const runCommand = () => {
    const command = query.trim();
    if (!command) return;

    rememberCommand(command);
    pendingCommand.current = command;
    setTerminalMounted(true);
    setQuery("");
    setMode("terminal");
  };

  const onInputKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      event.preventDefault();
      runCommand();
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

  if (mode === "collapsed") {
    return (
      <div className="collapsed-shell">
        <div
          className="collapsed-card"
          onMouseDown={startDrag}
          onClick={() => focusCollapsedInput()}
        >
          <div className="collapsed-card__prompt">›</div>
          <input
            ref={inputRef}
            className="collapsed-card__input"
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setHistoryIndex(-1);
            }}
            onKeyDown={onInputKeyDown}
            placeholder={history[0] ? `Run a shell command · ↑ ${history[0]}` : "Run a shell command"}
            autoFocus
            spellCheck={false}
            autoCapitalize="off"
            autoCorrect="off"
          />
        </div>
      </div>
    );
  }

  return (
    <div className="terminal-shell">
      <section className="terminal-panel terminal-panel--entered">
        <header className="terminal-panel__header" onMouseDown={startDrag}>
          <div className="terminal-panel__actions">
            <button className="toolbar-button toolbar-button--close" aria-label="New command" onClick={returnToInputMode}>×</button>
          </div>
        </header>

        <div className="terminal-panel__body">
          <div ref={termRef} className="terminal-panel__mount" onMouseDown={() => xterm.current?.focus()} />
        </div>
      </section>
    </div>
  );
}
