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

function withAltPrefix(sequence: Uint8Array, e: KeyboardEvent): Uint8Array {
  return e.altKey
    ? new Uint8Array([27, ...sequence])
    : sequence;
}

/**
 * Text-producing keys must be left to the browser's native text input path.
 * That path is what turns IME compositions and dead keys into their final
 * Unicode text. Control and Meta combinations still belong to `encodeKey`.
 */
export function shouldUseTerminalTextInput(e: KeyboardEvent): boolean {
  const altGraph = e.getModifierState?.("AltGraph") ?? false;
  if (e.metaKey || (e.ctrlKey && !altGraph) || (e.altKey && !altGraph)) return false;
  return e.key.length === 1 || e.key === "Dead" || e.key === "Process" || e.key === "Unidentified";
}

/** WebKit can clear `isComposing` before the key that confirms an IME choice. */
export function isTerminalCompositionKey(e: KeyboardEvent): boolean {
  return e.isComposing || e.keyCode === 229;
}

/**
 * Encode a key event, or return `null` when the event should not be forwarded
 * to the terminal (e.g. Cmd-based window shortcuts handled by the app).
 */
export function encodeKey(e: KeyboardEvent, mode: number): Uint8Array | null {
  if (e.metaKey) return null;

  const key = e.key;
  // On international layouts AltGr is commonly exposed as Ctrl+Alt. Treating
  // it as either modifier corrupts characters such as @, €, and braces before
  // they reach the PTY. `key` already contains the composed character.
  const altGraph = e.getModifierState?.("AltGraph") ?? false;

  // Ctrl sequences -> C0 control characters.
  if (e.ctrlKey && !altGraph && key.length === 1) {
    const code = key.toUpperCase().charCodeAt(0);
    if (code >= 0x41 && code <= 0x5a) {
      return withAltPrefix(new Uint8Array([code - 0x40]), e);
    }
    let control: number | null = null;
    switch (key) {
      case "[": control = 27; break;
      case "\\": control = 28; break;
      case "]": control = 29; break;
      case "^": control = 30; break;
      case "_": control = 31; break;
      case " ": control = 0; break;
      case "?": control = 127; break;
      case "@": control = 0; break;
      default: return null;
    }
    return withAltPrefix(new Uint8Array([control]), e);
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
    case "F1": return csiKey("P", e, "OP");
    case "F2": return csiKey("Q", e, "OQ");
    case "F3": return csiKey("R", e, "OR");
    case "F4": return csiKey("S", e, "OS");
    case "F5": return tildeKey(15, e);
    case "F6": return tildeKey(17, e);
    case "F7": return tildeKey(18, e);
    case "F8": return tildeKey(19, e);
    case "F9": return tildeKey(20, e);
    case "F10": return tildeKey(21, e);
    case "F11": return tildeKey(23, e);
    case "F12": return tildeKey(24, e);
  }

  if (key.length !== 1) return null;

  // Alt acts as Meta: send ESC + char (readline-style shortcuts).
  if (e.altKey && !altGraph) {
    return new Uint8Array([27, ...encoder.encode(key)]);
  }

  return encoder.encode(key);
}
