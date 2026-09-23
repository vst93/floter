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

export const TERMINAL_THEMES = ["inherit", "contrast", "paper"] as const;
export type TerminalTheme = (typeof TERMINAL_THEMES)[number];
export const DEFAULT_TERMINAL_THEME: TerminalTheme = "inherit";

export const normalizeTerminalTheme = (value: string): TerminalTheme =>
  value === "inherit" || value === "contrast" || value === "paper"
    ? value
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

/** The two overrides. `contrast` is a true black/white pair with a saturated
 *  cursor; `paper` is a warm, low-glare light palette. `inherit` has no entry
 *  — it means "read the document's `--terminal-*` tokens". */
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
};

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

// ---- Option lists --------------------------------------------------------

export type Choice<T extends string> = { value: T; labelKey: MessageKey };

export const CURSOR_SHAPE_OPTIONS: Choice<CursorShape>[] = [
  { value: "beam", labelKey: "settings.cursor.beam" },
  { value: "block", labelKey: "settings.cursor.block" },
  { value: "underline", labelKey: "settings.cursor.underline" },
];

export const TERMINAL_THEME_OPTIONS: Choice<TerminalTheme>[] = [
  { value: "inherit", labelKey: "settings.terminalTheme.inherit" },
  { value: "contrast", labelKey: "settings.terminalTheme.contrast" },
  { value: "paper", labelKey: "settings.terminalTheme.paper" },
];

export const TERMINAL_PADDING_OPTIONS: Choice<TerminalPadding>[] = TERMINAL_PADDING_ORDER.map(
  (value) => ({ value, labelKey: `settings.terminalPadding.${value}` as MessageKey }),
);

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
