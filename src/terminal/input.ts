// Translates a browser `KeyboardEvent` into the raw bytes a PTY expects.
//
// Honors the terminal's `APP_CURSOR` mode bit (carried in each rendered frame)
// so that arrow keys behave correctly inside full-screen apps (vim, less, ...).

const APP_CURSOR = 1 << 1;

const encoder = new TextEncoder();

function bytes(s: string): Uint8Array {
  return encoder.encode(s);
}

function esc(s: string): Uint8Array {
  return bytes(`\x1b${s}`);
}

/**
 * Encode a key event, or return `null` when the event should not be forwarded
 * to the terminal (e.g. Cmd-based window shortcuts handled by the app).
 */
export function encodeKey(e: KeyboardEvent, mode: number): Uint8Array | null {
  if (e.metaKey) return null;

  const key = e.key;

  // Ctrl sequences -> C0 control characters.
  if (e.ctrlKey) {
    if (key.length === 1) {
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
      }
    }
    // Unrecognized Ctrl combo: don't send a literal character.
    return null;
  }

  const appCursor = (mode & APP_CURSOR) !== 0;

  switch (key) {
    case "Enter": return new Uint8Array([13]);
    case "Backspace": return new Uint8Array([127]);
    case "Tab": return new Uint8Array([9]);
    case "Escape": return new Uint8Array([27]);
    case "ArrowUp": return esc(appCursor ? "OA" : "[A");
    case "ArrowDown": return esc(appCursor ? "OB" : "[B");
    case "ArrowRight": return esc(appCursor ? "OC" : "[C");
    case "ArrowLeft": return esc(appCursor ? "OD" : "[D");
    case "Home": return esc(appCursor ? "OH" : "[H");
    case "End": return esc(appCursor ? "OF" : "[F");
    case "Insert": return bytes("\x1b[2~");
    case "Delete": return bytes("\x1b[3~");
    case "PageUp": return bytes("\x1b[5~");
    case "PageDown": return bytes("\x1b[6~");
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
