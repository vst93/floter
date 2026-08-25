// Keyboard shortcuts shared between the settings panel and the key handlers.
//
// A shortcut is stored as the string Tauri's global-shortcut plugin parses
// ("Ctrl+Space", "Cmd+W"), so the same value works for the bindings the
// OS owns and for the ones the webview handles.

export type ShortcutAction =
  | "toggle_window"
  | "new_command"
  | "open_external_terminal"
  | "copy_selection"
  | "paste"
  | "open_settings"
  | "select_result"
  | "pin_terminal";

export type ShortcutMap = Record<ShortcutAction, string>;

export const IS_MAC =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);
export const IS_WINDOWS =
  typeof navigator !== "undefined" && /Windows/.test(navigator.userAgent);
export const IS_LINUX =
  typeof navigator !== "undefined" && /Linux/.test(navigator.userAgent);

/** Cmd on macOS, Ctrl everywhere else — the modifier apps use for their own commands. */
const APP_MODIFIER = IS_MAC ? "Cmd" : "Ctrl";

/** Mirrors `default_shortcuts()` in `commands/config.rs`. */
export const DEFAULT_SHORTCUTS: ShortcutMap = {
  toggle_window: "Ctrl+Space",
  new_command: `${APP_MODIFIER}+W`,
  open_external_terminal: `${APP_MODIFIER}+N`,
  copy_selection: IS_MAC ? "Cmd+C" : "Ctrl+Shift+C",
  paste: IS_MAC ? "Cmd+V" : "Ctrl+Shift+V",
  open_settings: `${APP_MODIFIER}+Comma`,
  select_result: `${APP_MODIFIER}+1`,
  // Free against every other default on all platforms (the closest neighbours
  // are Ctrl+Shift+C/V for copy/paste); verified against DEFAULT_SHORTCUTS and
  // the select_result modifier family.
  pin_terminal: IS_MAC ? "Cmd+Shift+P" : "Ctrl+Shift+P",
};

/** Display order of the shortcuts section. */
export const SHORTCUT_ACTIONS: ShortcutAction[] = [
  "toggle_window",
  "new_command",
  "open_external_terminal",
  "copy_selection",
  "paste",
  "open_settings",
  "select_result",
  "pin_terminal",
];

type Modifiers = { ctrl: boolean; alt: boolean; shift: boolean; meta: boolean };
type ParsedShortcut = Modifiers & { key: string };

/** Punctuation accepted in both spellings, normalized to the named form. */
const KEY_ALIASES: Record<string, string> = {
  ",": "Comma",
  ".": "Period",
  "/": "Slash",
  "\\": "Backslash",
  ";": "Semicolon",
  "'": "Quote",
  "-": "Minus",
  "=": "Equal",
  "[": "BracketLeft",
  "]": "BracketRight",
  "`": "Backquote",
  " ": "Space",
  esc: "Escape",
  return: "Enter",
  del: "Delete",
  up: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  right: "ArrowRight",
};

/** How a key is drawn: symbols on macOS, spelled out elsewhere. */
const KEY_SYMBOLS: Record<string, string> = {
  Comma: ",",
  Period: ".",
  Slash: "/",
  Backslash: "\\",
  Semicolon: ";",
  Quote: "'",
  Minus: "-",
  Equal: "=",
  BracketLeft: "[",
  BracketRight: "]",
  Backquote: "`",
  ArrowUp: "↑",
  ArrowDown: "↓",
  ArrowLeft: "←",
  ArrowRight: "→",
};

const MODIFIER_KEYS = new Set(["Control", "Shift", "Alt", "Meta", "CapsLock"]);

const normalizeKey = (key: string) => {
  const alias = KEY_ALIASES[key] ?? KEY_ALIASES[key.toLowerCase()];
  if (alias) return alias;
  return key.length === 1 ? key.toUpperCase() : key;
};

/**
 * The key token of an event, in the same vocabulary the stored strings use.
 * `code` is preferred over `key` so a combination keeps working regardless of
 * which modifiers are held (Cmd+Shift+1 still reports `Digit1`).
 */
export const keyTokenFromEvent = (event: KeyboardEvent): string | null => {
  if (MODIFIER_KEYS.has(event.key)) return null;

  const code = event.code;
  if (code.startsWith("Key") && code.length === 4) return code.slice(3);
  if (code.startsWith("Digit")) return code.slice(5);
  if (code.startsWith("Numpad") && code.length > 6) return code;
  if (/^F\d{1,2}$/.test(code)) return code;
  if (
    /^(Space|Enter|Tab|Escape|Backspace|Delete|Insert|Home|End|PageUp|PageDown|Arrow(Up|Down|Left|Right)|Comma|Period|Slash|Backslash|Semicolon|Quote|Minus|Equal|Bracket(Left|Right)|Backquote)$/.test(
      code,
    )
  ) {
    return code;
  }

  // Layouts `code` does not describe (or events without one): use the printed key.
  if (event.key) return normalizeKey(event.key);
  return null;
};

/**
 * Every token an event may be recognised by: the physical key, and the
 * character it prints. Keyboard layouts move punctuation around (`,` sits on
 * the physical M key on AZERTY), so a binding matches either reading.
 */
const keyTokensFromEvent = (event: KeyboardEvent): string[] => {
  const tokens: string[] = [];
  const token = keyTokenFromEvent(event);
  if (token) tokens.push(token.toLowerCase());
  if (event.key && !MODIFIER_KEYS.has(event.key)) {
    const printed = normalizeKey(event.key).toLowerCase();
    if (!tokens.includes(printed)) tokens.push(printed);
  }
  return tokens;
};

export const parseShortcut = (value: string): ParsedShortcut | null => {
  const parsed: ParsedShortcut = {
    ctrl: false,
    alt: false,
    shift: false,
    meta: false,
    key: "",
  };

  for (const raw of value.split("+")) {
    const part = raw.trim();
    if (!part) continue;
    switch (part.toLowerCase()) {
      case "ctrl":
      case "control":
        parsed.ctrl = true;
        break;
      case "alt":
      case "option":
        parsed.alt = true;
        break;
      case "shift":
        parsed.shift = true;
        break;
      case "cmd":
      case "command":
      case "meta":
      case "super":
        parsed.meta = true;
        break;
      case "commandorcontrol":
      case "cmdorctrl":
        if (IS_MAC) parsed.meta = true;
        else parsed.ctrl = true;
        break;
      default:
        parsed.key = normalizeKey(part);
    }
  }

  return parsed.key ? parsed : null;
};

const modifiersMatch = (event: KeyboardEvent, shortcut: Modifiers) =>
  event.ctrlKey === shortcut.ctrl &&
  event.altKey === shortcut.alt &&
  event.shiftKey === shortcut.shift &&
  event.metaKey === shortcut.meta;

const hasModifier = (shortcut: Modifiers) =>
  shortcut.ctrl || shortcut.alt || shortcut.shift || shortcut.meta;

const shortcutWithKey = (shortcut: Modifiers, key: string): string =>
  [
    shortcut.ctrl && "Ctrl",
    shortcut.alt && "Alt",
    shortcut.shift && "Shift",
    shortcut.meta && (IS_MAC ? "Cmd" : "Super"),
    key,
  ]
    .filter(Boolean)
    .join("+");

/**
 * Result selection is a shortcut family, not one literal key: the recording
 * stores its modifiers and the first digit. Reject a modifier-less binding so
 * normal number entry and Enter can never launch something unexpectedly.
 */
export const normalizeResultShortcut = (value: string | undefined): string | null => {
  if (!value) return null;
  const shortcut = parseShortcut(value);
  if (!shortcut || !hasModifier(shortcut)) return null;
  return shortcutWithKey(shortcut, "1");
};

/** Whether `event` is exactly the combination `value` describes. */
export const matchesShortcutModifiers = (
  event: KeyboardEvent,
  value: string | undefined,
): boolean => {
  if (!value) return false;
  const shortcut = parseShortcut(value);
  return Boolean(shortcut && modifiersMatch(event, shortcut));
};

export const matchesShortcut = (event: KeyboardEvent, value: string | undefined): boolean => {
  if (!value || !matchesShortcutModifiers(event, value)) return false;
  const shortcut = parseShortcut(value);
  return Boolean(shortcut && keyTokensFromEvent(event).includes(shortcut.key.toLowerCase()));
};

/**
 * The 1-9 variant used by the launcher: `value` binds the first result, and the
 * remaining digits reuse its modifiers.
 */
export const matchesResultShortcut = (
  event: KeyboardEvent,
  value: string | undefined,
): number | null => {
  if (!value) return null;
  const normalized = normalizeResultShortcut(value);
  const shortcut = normalized ? parseShortcut(normalized) : null;
  if (!shortcut || !modifiersMatch(event, shortcut)) return null;
  const digit = keyTokensFromEvent(event).find((token) => /^[1-9]$/.test(token));
  return digit ? Number(digit) : null;
};

/** Build the stored representation of the combination the user just pressed. */
export const shortcutFromEvent = (event: KeyboardEvent): string | null => {
  const key = keyTokenFromEvent(event);
  if (!key) return null;

  const parts: string[] = [];
  if (event.ctrlKey) parts.push("Ctrl");
  if (event.altKey) parts.push("Alt");
  if (event.shiftKey) parts.push("Shift");
  // `metaKey` is the Windows key on Windows and the Super key on Linux.
  // Tauri's global-shortcut plugin expects "Super" (not "Win") for this
  // modifier on non-macOS platforms, and "Cmd" on macOS.
  if (event.metaKey) parts.push(IS_MAC ? "Cmd" : "Super");
  parts.push(key);
  return parts.join("+");
};

const formatKey = (key: string) => KEY_SYMBOLS[key] ?? key;

/** Human-readable form: `⌘,` on macOS, `Ctrl+,` elsewhere. */
export const formatShortcut = (value: string | undefined): string => {
  if (!value) return "";
  const shortcut = parseShortcut(value);
  if (!shortcut) return value;

  if (IS_MAC) {
    let output = "";
    if (shortcut.ctrl) output += "⌃";
    if (shortcut.alt) output += "⌥";
    if (shortcut.shift) output += "⇧";
    if (shortcut.meta) output += "⌘";
    return `${output}${formatKey(shortcut.key)}`;
  }

  const parts: string[] = [];
  if (shortcut.ctrl) parts.push("Ctrl");
  if (shortcut.alt) parts.push("Alt");
  if (shortcut.shift) parts.push("Shift");
  if (shortcut.meta) parts.push("Win");
  parts.push(formatKey(shortcut.key));
  return parts.join("+");
};

/** The launcher badge: the result shortcut with its digit swapped in. */
export const formatResultShortcut = (
  value: string | undefined,
  key: number | "Enter",
): string => {
  const normalized = normalizeResultShortcut(value ?? DEFAULT_SHORTCUTS.select_result);
  const shortcut = normalized ? parseShortcut(normalized) : null;
  if (!shortcut) return String(key);
  const formatted = formatShortcut(shortcutWithKey(shortcut, String(key)));
  return key === "Enter" ? formatted.replace(/Enter$/, "↩") : formatted;
};

/** Fill in the platform defaults for anything the settings file omits. */
export const withShortcutDefaults = (
  shortcuts: Partial<Record<string, string>> | undefined,
): ShortcutMap => {
  const resolved = { ...DEFAULT_SHORTCUTS };
  if (!shortcuts) return resolved;
  for (const action of SHORTCUT_ACTIONS) {
    const value = shortcuts[action];
    if (!value || !value.trim()) continue;
    if (action === "select_result") {
      resolved[action] = normalizeResultShortcut(value) ?? DEFAULT_SHORTCUTS.select_result;
    } else {
      resolved[action] = value.trim();
    }
  }
  return resolved;
};
