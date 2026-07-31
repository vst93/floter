// Translates a browser `KeyboardEvent` into the raw bytes a PTY expects.
//
// Honors the terminal's `APP_CURSOR` mode bit (carried in each rendered frame)
// so that arrow keys behave correctly inside full-screen apps (vim, less, ...).

const APP_CURSOR = 1 << 1;
export const MOUSE_REPORT_CLICK = 1 << 3;
export const MOUSE_MOTION = 1 << 6;
export const FOCUS_IN_OUT = 1 << 11;
export const ALT_SCREEN = 1 << 12;
export const MOUSE_DRAG = 1 << 13;
export const MOUSE_MODE = MOUSE_REPORT_CLICK | MOUSE_MOTION | MOUSE_DRAG;

export function usesMouseReporting(mode: number): boolean {
  return (mode & MOUSE_MODE) !== 0;
}

const encoder = new TextEncoder();

function bytes(s: string): Uint8Array {
  return encoder.encode(s);
}

function esc(s: string): Uint8Array {
  return bytes(`\x1b${s}`);
}

function modifierParameter(e: KeyboardEvent): number {
  return 1 + Number(e.shiftKey) + Number(e.altKey) * 2 + Number(e.ctrlKey) * 4;
}

function csiKey(final: string, e: KeyboardEvent, appSequence: string): Uint8Array {
  const modifier = modifierParameter(e);
  return modifier === 1 ? esc(appSequence) : esc(`[1;${modifier}${final}`);
}

function tildeKey(code: number, e: KeyboardEvent): Uint8Array {
  const modifier = modifierParameter(e);
  return esc(modifier === 1 ? `[${code}~` : `[${code};${modifier}~`);
}

/**
 * Encode a key event, or return `null` when the event should not be forwarded
 * to the terminal (e.g. Cmd-based window shortcuts handled by the app).
 */
export function encodeKey(e: KeyboardEvent, mode: number): Uint8Array | null {
  if (e.metaKey) return null;

  const key = e.key;

  // Ctrl sequences -> C0 control characters.
  if (e.ctrlKey && key.length === 1) {
    const code = key.toUpperCase().charCodeAt(0);
    if (code >= 0x41 && code <= 0x5a) {
      return new Uint8Array([code - 0x40]);
    }
    switch (key) {
      case "[": return new Uint8Array([27]);
      case "\\": return new Uint8Array([28]);
      case "]": return new Uint8Array([29]);
      case "^": return new Uint8Array([30]);
      case "_": return new Uint8Array([31]);
      case " ": return new Uint8Array([0]);
      case "?": return new Uint8Array([127]);
      case "@": return new Uint8Array([0]);
      default: return null;
    }
  }

  const appCursor = (mode & APP_CURSOR) !== 0;

  switch (key) {
    case "Enter": return new Uint8Array([13]);
    case "Backspace": return new Uint8Array([127]);
    case "Tab": return e.shiftKey ? esc("[Z") : new Uint8Array([9]);
    case "Escape": return new Uint8Array([27]);
    case "ArrowUp": return csiKey("A", e, appCursor ? "OA" : "[A");
    case "ArrowDown": return csiKey("B", e, appCursor ? "OB" : "[B");
    case "ArrowRight": return csiKey("C", e, appCursor ? "OC" : "[C");
    case "ArrowLeft": return csiKey("D", e, appCursor ? "OD" : "[D");
    case "Home": return csiKey("H", e, appCursor ? "OH" : "[H");
    case "End": return csiKey("F", e, appCursor ? "OF" : "[F");
    case "Insert": return tildeKey(2, e);
    case "Delete": return tildeKey(3, e);
    case "PageUp": return tildeKey(5, e);
    case "PageDown": return tildeKey(6, e);
    case "F1": return bytes("\x1bOP");
    case "F2": return bytes("\x1bOQ");
    case "F3": return bytes("\x1bOR");
    case "F4": return bytes("\x1bOS");
    case "F5": return bytes("\x1b[15~");
    case "F6": return bytes("\x1b[17~");
    case "F7": return bytes("\x1b[18~");
    case "F8": return bytes("\x1b[19~");
    case "F9": return bytes("\x1b[20~");
    case "F10": return bytes("\x1b[21~");
    case "F11": return bytes("\x1b[23~");
    case "F12": return bytes("\x1b[24~");
  }

  if (key.length !== 1) return null;

  // Alt acts as Meta: send ESC + char (readline-style shortcuts).
  if (e.altKey) {
    return new Uint8Array([27, ...encoder.encode(key)]);
  }

  return encoder.encode(key);
}
