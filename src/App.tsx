import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "@xterm/xterm/css/xterm.css";

const THEME = {
  background: "#1e1e2e",
  foreground: "#cdd6f4",
  cursor: "#f5e0dc",
  cursorAccent: "#1e1e2e",
  selectionBackground: "#585b70",
  black: "#45475a",
  red: "#f38ba8",
  green: "#a6e3a1",
  yellow: "#f9e2af",
  blue: "#89b4fa",
  magenta: "#f5c2e7",
  cyan: "#94e2d5",
  white: "#bac2de",
  brightBlack: "#585b70",
  brightRed: "#f38ba8",
  brightGreen: "#a6e3a1",
  brightYellow: "#f9e2af",
  brightBlue: "#89b4fa",
  brightMagenta: "#f5c2e7",
  brightCyan: "#94e2d5",
  brightWhite: "#a6adc8",
};

export default function App() {
  const inputRef = useRef<HTMLInputElement>(null);
  const termRef = useRef<HTMLDivElement>(null);
  const xterm = useRef<Terminal | null>(null);
  const fit = useRef<FitAddon | null>(null);
  const ptyReady = useRef(false);

  const [mode, setMode] = useState<"input" | "terminal">("input");
  const [cmd, setCmd] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [histIdx, setHistIdx] = useState(-1);

  // === 一次性初始化：PTY + xterm + listener ===
  useEffect(() => {
    if (ptyReady.current) return;
    ptyReady.current = true;

    invoke("pty_spawn", { id: "main", shell: null });

    const t = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "'SF Mono','Menlo','Monaco','Consolas',monospace",
      theme: THEME,
      lineHeight: 1.3,
      scrollback: 10000,
    });
    const f = new FitAddon();
    t.loadAddon(f);
    t.loadAddon(new WebLinksAddon());
    t.open(termRef.current!);
    xterm.current = t;
    fit.current = f;

    listen<[string, number[]]>("pty-output", (ev) => {
      if (ev.payload[0] === "main") {
        t.write(new TextDecoder().decode(new Uint8Array(ev.payload[1])));
      }
    });

    new ResizeObserver(() => {
      f.fit();
      const d = f.proposeDimensions();
      if (d) invoke("pty_resize", { id: "main", rows: d.rows, cols: d.cols });
    }).observe(termRef.current!);
  }, []);

  // === 模式切换：调整窗口大小 ===
  useEffect(() => {
    if (mode === "input") {
      invoke("show_input");
      setTimeout(() => inputRef.current?.focus(), 100);
    } else {
      invoke("show_terminal");
      setTimeout(() => { fit.current?.fit(); xterm.current?.focus(); }, 100);
    }
  }, [mode]);

  // === ESC ===
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      if (mode === "terminal") setMode("input");
      else invoke("hide_window");
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [mode]);

  useEffect(() => { inputRef.current?.focus(); }, []);

  // === 输入框回车 → 发命令 → 进入终端 ===
  const submit = () => {
    const c = cmd.trim();
    if (!c) return;
    setHistory((p) => [...p, c]);
    setHistIdx(-1);
    setCmd("");
    setMode("terminal");
    setTimeout(() => {
      invoke("pty_write", { id: "main", data: Array.from(new TextEncoder().encode(c + "\n")) });
      fit.current?.fit();
      xterm.current?.focus();
    }, 150);
  };

  const onKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") { e.preventDefault(); submit(); }
    else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (!history.length) return;
      const i = Math.min(histIdx + 1, history.length - 1);
      setHistIdx(i);
      setCmd(history[history.length - 1 - i]);
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      if (histIdx <= 0) { setHistIdx(-1); setCmd(""); }
      else { setHistIdx(histIdx - 1); setCmd(history[history.length - 2 - histIdx]); }
    } else if (e.key === "Escape") {
      invoke("hide_window");
    }
  };

  const startDrag = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest("input") || (e.target as HTMLElement).closest("button")) return;
    e.preventDefault();
    invoke("start_drag");
  };

  // === 输入模式 ===
  if (mode === "input") {
    return (
      <div
        style={{ width: "100vw", height: "100vh", display: "flex", alignItems: "flex-start", justifyContent: "center", paddingTop: "20vh", background: "transparent" }}
        onClick={(e) => { if (e.target === e.currentTarget) invoke("hide_window"); }}
      >
        <div
          onMouseDown={startDrag}
          style={{
            width: 560, height: 48, background: "rgba(24,24,27,0.95)", borderRadius: 12,
            boxShadow: "0 25px 50px -12px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.05)",
            backdropFilter: "blur(20px)", display: "flex", alignItems: "center", padding: "0 14px", gap: 10, cursor: "grab",
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#a78bfa" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="4 17 10 11 4 5" />
            <line x1="12" y1="19" x2="20" y2="19" />
          </svg>
          <input
            ref={inputRef}
            type="text"
            value={cmd}
            onChange={(e) => { setCmd(e.target.value); setHistIdx(-1); }}
            onKeyDown={onKey}
            placeholder="输入命令..."
            autoFocus
            style={{ flex: 1, background: "transparent", border: "none", outline: "none", color: "#e4e4e7", fontSize: 15, fontFamily: "'SF Mono','Menlo','Monaco','Consolas',monospace" }}
          />
          {cmd && <span style={{ fontSize: 12, color: "#52525b" }}>↵</span>}
        </div>
      </div>
    );
  }

  // === 终端模式 — 只有终端，没有输入框 ===
  return (
    <div
      style={{ width: "100vw", height: "100vh", background: "transparent" }}
      onClick={(e) => { if (e.target === e.currentTarget) setMode("input"); }}
    >
      <div
        style={{
          position: "absolute", top: "10vh", left: "50%", transform: "translateX(-50%)",
          width: 720, height: 480, background: "rgba(24,24,27,0.95)", borderRadius: 12,
          boxShadow: "0 25px 50px -12px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.05)",
          backdropFilter: "blur(20px)", overflow: "hidden", display: "flex", flexDirection: "column",
        }}
      >
        <div
          onMouseDown={startDrag}
          style={{ height: 32, display: "flex", alignItems: "center", justifyContent: "space-between", padding: "0 12px", background: "rgba(30,30,46,0.98)", borderBottom: "1px solid rgba(69,71,90,0.5)", cursor: "grab", flexShrink: 0 }}
        >
          <span style={{ color: "#6c7086", fontSize: 12 }}>floter</span>
          <button
            onClick={(e) => { e.stopPropagation(); setMode("input"); }}
            style={{ background: "transparent", border: "none", color: "#6c7086", cursor: "pointer", fontSize: 12, padding: "2px 8px", borderRadius: 4 }}
            onMouseEnter={(e) => { (e.target as HTMLElement).style.color = "#cdd6f4"; }}
            onMouseLeave={(e) => { (e.target as HTMLElement).style.color = "#6c7086"; }}
          >
            ESC
          </button>
        </div>
        <div
          ref={termRef}
          style={{ flex: 1, background: "#1e1e2e" }}
          onMouseDown={() => xterm.current?.focus()}
        />
      </div>
    </div>
  );
}
