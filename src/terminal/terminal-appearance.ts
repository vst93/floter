// R42 · the terminal's appearance settings, described once.
//
// The terminal page grew its own settings panel (the gear in the terminal bar)
// and the settings page already had a "Terminal appearance" card. Both write
// the *same* `AppSettings` fields through the *same* normalizers and option
// lists, which live here — a React-free module, so the node test runner can
// import the domain directly and the two surfaces cannot drift into two
// truths.
//
// Nothing here reads CSS: the canvas is a bitmap, so every value the renderer
// needs is a number this module hands it. The R7-13b decoupling marker (the
// canvas cell size is `opts.fontSize`, never a `getComputedStyle` font-size)
// stays exactly as it was.

import type { MessageKey } from "../i18n";

// ---- Font size -----------------------------------------------------------
//
// Moved here from `settings/GeneralPage.tsx` (which re-exports it) so the
// settings card and the in-terminal panel clamp with one function. The
// domain is the one the slider has always shipped: 8–48 px, integer.

export const MIN_FONT_SIZE = 8;
export const MAX_FONT_SIZE = 48;
export const DEFAULT_FONT_SIZE = 14;

export const normalizeFontSize = (value: number): number =>
  Math.round(
    Math.min(
      MAX_FONT_SIZE,
      Math.max(MIN_FONT_SIZE, Number.isFinite(value) ? value : DEFAULT_FONT_SIZE),
    ),
  );

// ---- Line height ---------------------------------------------------------
//
// A multiple of the font size, 1.0–2.0 in 0.05 steps. `render.ts` already
// consumed a `lineHeight` option; R42 makes it user-owned. The floor is the
// shipped 1.0 (a face's ink floor still wins inside `measureCell`, so a tight
// multiple can never clip a descender), the ceiling is a genuinely airy 2.0.

export const MIN_LINE_HEIGHT = 1;
export const MAX_LINE_HEIGHT = 2;
export const LINE_HEIGHT_STEP = 0.05;
export const DEFAULT_LINE_HEIGHT = 1.4;

/** Snap to the 0.05 grid by scaling to twentieths, so 1.4 survives as 1.4
 *  rather than as the `1.4000000000000001` a `* 0.05` would produce. */
export const normalizeLineHeight = (value: number): number => {
  const raw = Number.isFinite(value) ? value : DEFAULT_LINE_HEIGHT;
  const clamped = Math.min(MAX_LINE_HEIGHT, Math.max(MIN_LINE_HEIGHT, raw));
  return Math.round(clamped * 20) / 20;
};

// ---- Padding -------------------------------------------------------------
//
// Three named steps rather than a free number: the user asked for a *simple*
// panel, and the difference a millimetre makes is not worth a slider. The
// pixel each step paints is the canvas's inner inset (see `RendererOptions`),
// and `regular` is the 3px every earlier build shipped — so a settings file
// written before this key existed renders pixel-identically.

export const TERMINAL_PADDING_STEPS = {
  compact: 1,
  regular: 3,
  relaxed: 6,
} as const;

export type TerminalPadding = keyof typeof TERMINAL_PADDING_STEPS;

export const TERMINAL_PADDING_ORDER: TerminalPadding[] = ["compact", "regular", "relaxed"];
export const DEFAULT_TERMINAL_PADDING: TerminalPadding = "regular";

export const normalizeTerminalPadding = (value: string): TerminalPadding =>
  value === "compact" || value === "regular" || value === "relaxed"
    ? value
    : DEFAULT_TERMINAL_PADDING;

export const terminalPaddingPx = (value: string): number =>
  TERMINAL_PADDING_STEPS[normalizeTerminalPadding(value)];

// ---- Terminal palette ----------------------------------------------------
//
// The canvas palette is a *separate axis* from the app's dark/light theme:
// `inherit` keeps the shipped behaviour (the renderer reads the `--terminal-*`
// tokens, which base.css maps per app theme), while the two overrides paint a
// fixed palette inside the canvas only. They are a canvas colour mapping — the
// R30 base.css material/glass formulas are not touched, and no global token
// moves.

export const TERMINAL_THEMES = [
  "inherit",
  "contrast",
  "paper",
  "ink",
  "fog",
  "forest",
  "dusk",
  "mist",
  "amber",
] as const;
export type TerminalTheme = (typeof TERMINAL_THEMES)[number];
export const DEFAULT_TERMINAL_THEME: TerminalTheme = "inherit";

export const normalizeTerminalTheme = (value: string): TerminalTheme =>
  (TERMINAL_THEMES as readonly string[]).includes(value)
    ? (value as TerminalTheme)
    : DEFAULT_TERMINAL_THEME;

/** A fixed palette a theme override paints, in the same shape `render.ts`
 *  resolves from CSS. Packed `0xRRGGBB` for the opaque colours; CSS strings
 *  for the two deliberately translucent ones. */
export interface TerminalPalette {
  bg: number;
  fg: number;
  cursor: number;
  selection: string;
  scrollbar: string;
}

/// The eight overrides. `contrast` is a true black/white pair with a saturated
///  cursor; `paper` is a warm, low-glare light palette; R43 adds six more so the
///  picker is a palette rack rather than a pair — `ink` (near-black), `fog`
///  (cool grey), `forest` (deep green), `dusk` (violet night), `mist` (a light
///  blue-grey) and `amber` (warm dark). `inherit` has no entry — it means "read
///  the document's `--terminal-*` tokens".
export const TERMINAL_PALETTES: Record<Exclude<TerminalTheme, "inherit">, TerminalPalette> = {
  contrast: {
    bg: 0x000000,
    fg: 0xffffff,
    cursor: 0x00ff9c,
    selection: "rgba(255, 255, 255, 0.30)",
    scrollbar: "rgba(255, 255, 255, 0.55)",
  },
  paper: {
    bg: 0xf4efe4,
    fg: 0x2a2a28,
    cursor: 0x8a5a00,
    selection: "rgba(0, 0, 0, 0.14)",
    scrollbar: "rgba(0, 0, 0, 0.34)",
  },
  ink: {
    bg: 0x0b0d10,
    fg: 0xe6e9ef,
    cursor: 0x7dd3fc,
    selection: "rgba(255, 255, 255, 0.24)",
    scrollbar: "rgba(255, 255, 255, 0.48)",
  },
  fog: {
    bg: 0x1b1f24,
    fg: 0xc9d1d9,
    cursor: 0x9ece6a,
    selection: "rgba(255, 255, 255, 0.22)",
    scrollbar: "rgba(255, 255, 255, 0.44)",
  },
  forest: {
    bg: 0x0f1a14,
    fg: 0xd7e4d0,
    cursor: 0x8bd450,
    selection: "rgba(255, 255, 255, 0.22)",
    scrollbar: "rgba(255, 255, 255, 0.44)",
  },
  dusk: {
    bg: 0x1a1526,
    fg: 0xe2d9f3,
    cursor: 0xc792ea,
    selection: "rgba(255, 255, 255, 0.24)",
    scrollbar: "rgba(255, 255, 255, 0.46)",
  },
  mist: {
    bg: 0xdfe7ef,
    fg: 0x2b3440,
    cursor: 0x2f6f9f,
    selection: "rgba(0, 0, 0, 0.16)",
    scrollbar: "rgba(0, 0, 0, 0.34)",
  },
  amber: {
    bg: 0x1c140a,
    fg: 0xf0e2c8,
    cursor: 0xffb454,
    selection: "rgba(255, 255, 255, 0.24)",
    scrollbar: "rgba(255, 255, 255, 0.46)",
  },
};

/** `0xRRGGBB` -> `#rrggbb`, for the palette preview's inline styles (the
 *  renderer's own packed integers are its business; the preview needs a CSS
 *  string). Pure and total. */
export const packedHex = (packed: number): string =>
  `#${(packed & 0xffffff).toString(16).padStart(6, "0")}`;

// ---- Cursor --------------------------------------------------------------

export type CursorShape = "beam" | "block" | "underline";
export const DEFAULT_CURSOR_SHAPE: CursorShape = "beam";

export const normalizeCursorShape = (value: string): CursorShape =>
  value === "beam" || value === "block" || value === "underline" ? value : DEFAULT_CURSOR_SHAPE;

export const DEFAULT_CURSOR_BLINK = true;
/** The user's blink preference. The wire carries the *program's* request
 *  (`cursor_blinking` in the frame); this is the user's own veto, applied by
 *  the renderer. */
export const normalizeCursorBlink = (value: unknown): boolean => value !== false;

export const DEFAULT_SCROLLBAR = true;
export const normalizeScrollbar = (value: unknown): boolean => value !== false;

// ---- R43 · the interaction axes ------------------------------------------
//
// Four more canvas/term capabilities the renderer and the input path can
// actually honour. Everything here is applied *live* by the running session:
// nothing needs a new PTY, so a change repaints or re-binds rather than
// resetting. The one capability the brief floated that is **out of reach** is
// the shell integration / prompt marks — the emulator has no OSC 133 handling
// on the wire, so no setting can honestly promise it.

// How many lines one wheel notch scrolls. The renderer's own unit was
// `max(24, cellHeight * 1.5)` — about a line and a half — and this multiplies
// it, so the default 3 matches the travel a user expects from a notch while a
// trackpad's many small deltas still accumulate (see `wheelScrollSteps`).
export const MIN_WHEEL_LINES = 1;
export const MAX_WHEEL_LINES = 8;
export const DEFAULT_WHEEL_LINES = 3;

export const normalizeWheelLines = (value: number): number =>
  Math.round(
    Math.min(
      MAX_WHEEL_LINES,
      Math.max(MIN_WHEEL_LINES, Number.isFinite(value) ? value : DEFAULT_WHEEL_LINES),
    ),
  );

/** How a bold cell is drawn: with the face's bold weight (`font`, the shipped
 *  behaviour) or with a brighter foreground (`bright`, the terminal classic).
 *  Unknown ids fall back to the shipped `font`. */
export const BOLD_MODES = ["font", "bright"] as const;
export type BoldMode = (typeof BOLD_MODES)[number];
export const DEFAULT_BOLD_MODE: BoldMode = "font";

export const normalizeBoldMode = (value: string): BoldMode =>
  value === "bright" ? "bright" : DEFAULT_BOLD_MODE;

/** Whether finishing a drag-copy puts the selection on the system clipboard.
 *
 *  R44 · **on by default**. R43 shipped it off (the explicit copy shortcut was
 *  the only path); the user asked for the selection to land on the clipboard
 *  without a second gesture, so the default flips here and in the Rust
 *  `Default` / serde-default pair. The switch is still a real veto: an explicit
 *  `false` — persisted by anyone who turned it off — wins over the default, and
 *  `normalizeSelectCopy` still refuses anything that is not a boolean `true`.
 *  This is the single source of truth the settings UI and `useSettings` read. */
export const DEFAULT_SELECT_COPY = true;
export const normalizeSelectCopy = (value: unknown): boolean => value === true;

/** Whether a paste has one trailing newline stripped, so pasting a command
 *  does not run it before the user has read it. Off by default — the shipped
 *  behaviour pastes verbatim. */
export const DEFAULT_PASTE_SAFE = false;
export const normalizePasteSafe = (value: unknown): boolean => value === true;

// ---- Option lists --------------------------------------------------------

export type Choice<T extends string> = { value: T; labelKey: MessageKey };

export const CURSOR_SHAPE_OPTIONS: Choice<CursorShape>[] = [
  { value: "beam", labelKey: "settings.cursor.beam" },
  { value: "block", labelKey: "settings.cursor.block" },
  { value: "underline", labelKey: "settings.cursor.underline" },
];

export const TERMINAL_THEME_OPTIONS: Choice<TerminalTheme>[] = TERMINAL_THEMES.map((value) => ({
  value,
  labelKey: `settings.terminalTheme.${value}` as MessageKey,
}));

export const TERMINAL_PADDING_OPTIONS: Choice<TerminalPadding>[] = TERMINAL_PADDING_ORDER.map(
  (value) => ({ value, labelKey: `settings.terminalPadding.${value}` as MessageKey }),
);

export const BOLD_MODE_OPTIONS: Choice<BoldMode>[] = BOLD_MODES.map((value) => ({
  value,
  labelKey: `settings.terminalBold.${value}` as MessageKey,
}));

// ---- Font family ---------------------------------------------------------
//
// The static list is the shipped one (GeneralPage's `FONT_FAMILY_OPTIONS`
// before R42), kept as the *fallback* so a browser that cannot probe fonts —
// or a node test — still offers the same choices. `monospace` is the system
// stack and is always first.

export const SYSTEM_FONT_FAMILY = "monospace";

export interface FontFamilyOption {
  value: string;
  label: string;
}

/** `monospace` resolves through the renderer's own fallback stack
 *  (`terminalFontFamily`), so the label names the stack, not a face. */
export const SYSTEM_FONT_OPTION: FontFamilyOption = { value: SYSTEM_FONT_FAMILY, label: "System Mono" };

/** Candidates probed against the platform. Order is presentation order. */
export const MONO_FONT_CANDIDATES: FontFamilyOption[] = [
  { value: "JetBrains Mono", label: "JetBrains Mono" },
  { value: "Fira Code", label: "Fira Code" },
  { value: "Cascadia Mono", label: "Cascadia Mono" },
  { value: "SF Mono", label: "SF Mono" },
  { value: "Menlo", label: "Menlo" },
  { value: "Monaco", label: "Monaco" },
  { value: "Consolas", label: "Consolas" },
  { value: "Hack", label: "Hack" },
  { value: "Source Code Pro", label: "Source Code Pro" },
  { value: "IBM Plex Mono", label: "IBM Plex Mono" },
  { value: "Roboto Mono", label: "Roboto Mono" },
  { value: "Ubuntu Mono", label: "Ubuntu Mono" },
  { value: "Noto Sans Mono", label: "Noto Sans Mono" },
  { value: "Inconsolata", label: "Inconsolata" },
  { value: "DejaVu Sans Mono", label: "DejaVu Sans Mono" },
  { value: "Liberation Mono", label: "Liberation Mono" },
  { value: "Courier New", label: "Courier New" },
];

/** The fallback list every surface offered before R42, used verbatim when the
 *  platform cannot be probed. */
export const FALLBACK_FONT_FAMILY_OPTIONS: FontFamilyOption[] = [
  SYSTEM_FONT_OPTION,
  ...MONO_FONT_CANDIDATES.filter((option) =>
    [
      "JetBrains Mono",
      "SF Mono",
      "Cascadia Mono",
      "Menlo",
      "Consolas",
      "DejaVu Sans Mono",
      "Liberation Mono",
    ].includes(option.value),
  ),
];

/**
 * The font-family choices to offer, given what the platform reports.
 *
 * `detected` is the output of `detectMonospaceFonts()` (empty when the probe
 * cannot run). The result is the system stack, then the detected candidates in
 * list order; `current` is appended when it is neither — a face the user
 * chose earlier (or hand-edited into the file) must stay selectable rather
 * than silently snap to the first entry.
 *
 * When nothing is detected the shipped static list is returned, so the picker
 * is never reduced to a single option on a platform the probe cannot read.
 */
export function fontFamilyOptions(
  detected: string[],
  current?: string,
): FontFamilyOption[] {
  const base = detected.length > 0
    ? [SYSTEM_FONT_OPTION, ...MONO_FONT_CANDIDATES.filter((option) => detected.includes(option.value))]
    : FALLBACK_FONT_FAMILY_OPTIONS;
  if (current && !base.some((option) => option.value === current)) {
    return [...base, { value: current, label: current }];
  }
  return base;
}
