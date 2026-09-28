// R65 · the direct output's own text may carry terminal formatting.
//
// The user's ask, verbatim: 「同时这种直出命令需要在非列表模式下（文本模式下）兼容
// 输出文本的一些格式，有些是带格式的终端字符」. A command summoned into the launcher
// prints its stdout into the R28 text surface (`launcher/PluginTextView.tsx`), and
// what a CLI actually writes is not plain prose: `git diff`, `ls --color`,
// progress bars and most modern tools emit ANSI SGR escape sequences for colour,
// weight and underline. Rendered verbatim those bytes are visible garbage
// (`^[[31m`), so the text form has to *read* them as the terminal would.
//
// This module is the reading: a pure, total parser from a string to a list of
// text spans, each carrying the style in force when its characters were written.
// It is deliberately not a terminal emulator — there is no cursor, no alternate
// screen, no scrolling region. The only sequences it interprets are the ones
// that change how a character *looks* (SGR); every other escape the stream may
// contain (cursor moves, erases, OSC titles) is dropped rather than printed, so
// `\x1b[2K` cannot turn into a line of noise.
//
// Pure (no React, no DOM, no Tauri) for the same reason `plugin-text-copy.ts`
// is: the grammar is the part a review can silently break, and the node suite
// can only pin it by importing it without the app runtime.
//
// The 16 named colours are handed to the sheet as CSS variables
// (`--ansi-black`..`--ansi-bright-white`, declared per theme in `base.css`) so
// the launcher's colours match the terminal canvas's own palette instead of
// drifting from it. The 256-colour cube and 24-bit "truecolour" forms cannot be
// named by a finite list, so they are resolved to `rgb(…)` right here.

/** What a run of characters looks like. `null` is "the surface's default". */
export type AnsiStyle = {
  readonly bold: boolean;
  /** SGR 2 — half-intensity ("dim"). */
  readonly dim: boolean;
  readonly italic: boolean;
  readonly underline: boolean;
  readonly strike: boolean;
  /** SGR 7 — foreground and background swapped. */
  readonly inverse: boolean;
  /** A CSS colour (`var(--ansi-…)` for the 16 named ones, `rgb(…)` otherwise),
   *  or `null` for the surface's default foreground. */
  readonly fg: string | null;
  /** Same shape as {@link fg} for the background. */
  readonly bg: string | null;
};

/** The state every stream starts in, and the one `SGR 0` restores. */
export const ANSI_DEFAULT_STYLE: AnsiStyle = Object.freeze({
  bold: false,
  dim: false,
  italic: false,
  underline: false,
  strike: false,
  inverse: false,
  fg: null,
  bg: null,
});

/** One run of text and the style in force across it. */
export type AnsiSpan = {
  readonly text: string;
  readonly style: AnsiStyle;
};

/** The eight base colour names, indexed by their ANSI number (`30` → red). */
const ANSI_NAMES = [
  "black",
  "red",
  "green",
  "yellow",
  "blue",
  "magenta",
  "cyan",
  "white",
] as const;

/** The 6×6×6 cube's per-channel levels (the xterm standard). */
const CUBE_LEVELS = [0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff] as const;

const clampByte = (value: number): number =>
  Math.max(0, Math.min(255, Math.trunc(Number.isFinite(value) ? value : 0)));

/**
 * The CSS value for one of the 16 named colours. It is a *variable reference*,
 * not a literal: the palette is the theme's (`base.css`), so light and dark
 * resolve it themselves and the launcher's output reads in the same colours the
 * terminal panel would paint.
 */
export const namedAnsiColor = (index: number): string => {
  const slot = ((Math.trunc(index) % 16) + 16) % 16;
  const name = ANSI_NAMES[slot % 8];
  return slot < 8 ? `var(--ansi-${name})` : `var(--ansi-bright-${name})`;
};

/**
 * The CSS value for a 256-colour index (`38;5;n` / `48;5;n`), by the xterm
 * standard: 0–15 the named palette, 16–231 a 6×6×6 cube, 232–255 a 24-step
 * grayscale ramp. An out-of-range index is clamped, never thrown at.
 */
export const indexedAnsiColor = (index: number): string => {
  const slot = clampByte(index);
  if (slot < 16) return namedAnsiColor(slot);
  if (slot < 232) {
    const n = slot - 16;
    const r = CUBE_LEVELS[Math.floor(n / 36) % 6];
    const g = CUBE_LEVELS[Math.floor(n / 6) % 6];
    const b = CUBE_LEVELS[n % 6];
    return `rgb(${r}, ${g}, ${b})`;
  }
  const v = 8 + 10 * (slot - 232);
  return `rgb(${v}, ${v}, ${v})`;
};

/** The CSS value for a 24-bit colour (`38;2;r;g;b` / `48;2;r;g;b`). */
export const rgbAnsiColor = (r: number, g: number, b: number): string =>
  `rgb(${clampByte(r)}, ${clampByte(g)}, ${clampByte(b)})`;

/** Parse an SGR parameter list. `\x1b[m` and `\x1b[;m` mean "reset", i.e. `0`. */
const parseSgrParams = (body: string): number[] => {
  if (body.length === 0) return [0];
  return body.split(";").map((part) => {
    if (part.length === 0) return 0;
    const value = Number(part);
    return Number.isFinite(value) ? value : Number.NaN;
  });
};

/** The extended colour of a `38`/`48` sequence starting at `start`, or `null`
 *  when the parameters do not spell one. `start` is the index of the `5`/`2`
 *  selector; the returned `length` counts the consumed parameters. */
const extendedColor = (
  params: readonly number[],
  start: number,
): { color: string; length: number } | null => {
  const mode = params[start];
  if (mode === 5) {
    const index = params[start + 1];
    if (!Number.isInteger(index)) return null;
    return { color: indexedAnsiColor(index), length: 2 };
  }
  if (mode === 2) {
    const [r, g, b] = [params[start + 1], params[start + 2], params[start + 3]];
    if (![r, g, b].every((channel) => Number.isInteger(channel))) return null;
    return { color: rgbAnsiColor(r, g, b), length: 4 };
  }
  return null;
};

/** Apply one SGR parameter list to a style, returning the next style. Unknown
 *  and unsupported codes (blink, conceal, fonts, …) are ignored — a code the
 *  surface cannot paint must not blank the text that carries it. */
export const applySgr = (style: AnsiStyle, params: readonly number[]): AnsiStyle => {
  let next = style;
  const set = (patch: Partial<AnsiStyle>) => {
    next = { ...next, ...patch };
  };
  for (let i = 0; i < params.length; i += 1) {
    const code = params[i];
    // Reset both halves whenever a code is written down; the loop below is a
    // flat dispatch where each branch is one SGR number (or one range).
    if (code === 0) {
      next = ANSI_DEFAULT_STYLE;
    } else if (code === 1) {
      set({ bold: true });
    } else if (code === 2) {
      set({ dim: true });
    } else if (code === 3) {
      set({ italic: true });
    } else if (code === 4) {
      set({ underline: true });
    } else if (code === 7) {
      set({ inverse: true });
    } else if (code === 9) {
      set({ strike: true });
    } else if (code === 22) {
      set({ bold: false, dim: false });
    } else if (code === 23) {
      set({ italic: false });
    } else if (code === 24) {
      set({ underline: false });
    } else if (code === 27) {
      set({ inverse: false });
    } else if (code === 29) {
      set({ strike: false });
    } else if (code >= 30 && code <= 37) {
      set({ fg: namedAnsiColor(code - 30) });
    } else if (code >= 90 && code <= 97) {
      set({ fg: namedAnsiColor(code - 90 + 8) });
    } else if (code === 39) {
      set({ fg: null });
    } else if (code >= 40 && code <= 47) {
      set({ bg: namedAnsiColor(code - 40) });
    } else if (code >= 100 && code <= 107) {
      set({ bg: namedAnsiColor(code - 100 + 8) });
    } else if (code === 49) {
      set({ bg: null });
    } else if (code === 38 || code === 48) {
      const extended = extendedColor(params, i + 1);
      if (extended) {
        set(code === 38 ? { fg: extended.color } : { bg: extended.color });
        i += extended.length;
      }
    }
  }
  return next;
};

/**
 * Split `input` into the text runs the surface should draw, interpreting SGR
 * sequences and **dropping every other escape**.
 *
 * The grammar walked here is the stream's own: an `ESC [` … final-byte CSI, an
 * `ESC ]` … `BEL`/`ST` operating-system command, and any other two-byte escape.
 * A malformed or truncated sequence is dropped as far as it goes and the scan
 * resumes — a half-written escape at the end of a truncated output never eats
 * the text before it.
 *
 * Consecutive characters under the same style are one span, so a plain output
 * (the overwhelmingly common case) is a single span and its render is a single
 * text node. A `style` that is content-equal to the previous one is still
 * merged implicitly by the run buffer.
 */
export const parseAnsi = (input: string): AnsiSpan[] => {
  const spans: AnsiSpan[] = [];
  let style = ANSI_DEFAULT_STYLE;
  let buffer = "";
  const flush = () => {
    if (buffer.length > 0) {
      spans.push({ text: buffer, style });
      buffer = "";
    }
  };

  let i = 0;
  while (i < input.length) {
    const character = input[i];
    if (character !== "\u001b") {
      buffer += character;
      i += 1;
      continue;
    }

    const introducer = input[i + 1];

    // CSI — `ESC [` parameter/intermediate bytes, then a final byte 0x40–0x7E.
    if (introducer === "[") {
      let j = i + 2;
      while (j < input.length) {
        const code = input.charCodeAt(j);
        if (code >= 0x40 && code <= 0x7e) break;
        // Anything outside parameter/intermediate bytes means this is not a CSI
        // after all; drop the `ESC` alone and keep reading as text.
        if (code < 0x20 || code > 0x3f) {
          j = -1;
          break;
        }
        j += 1;
      }
      if (j < 0) {
        i += 1;
        continue;
      }
      if (j >= input.length) {
        // A truncated sequence at the buffer's end — nothing left to draw.
        break;
      }
      if (input[j] === "m") {
        flush();
        style = applySgr(style, parseSgrParams(input.slice(i + 2, j)));
      }
      i = j + 1;
      continue;
    }

    // OSC — `ESC ]` … terminated by BEL or ST (`ESC \`). Its payload (a window
    // title, a hyperlink target) has no place in a static block, so it is eaten
    // whole rather than printed.
    if (introducer === "]") {
      let j = i + 2;
      while (j < input.length) {
        if (input[j] === "\u0007") break;
        if (input[j] === "\u001b" && input[j + 1] === "\\") break;
        j += 1;
      }
      i = input[j] === "\u001b" ? j + 2 : j + 1;
      continue;
    }

    // Any other escape (charset designators, single-character controls). The
    // final byte is all this parser needs to know; two characters are dropped
    // when the introducer is actually there, and a lone trailing `ESC` is one.
    i += i + 1 < input.length ? 2 : 1;
  }

  flush();
  return spans;
};

/** The plain text of a stream with every escape dropped — what a copy of the
 *  rendered block yields. The renderer never needs it (the DOM's own text
 *  content is already this), but a test and a caller that wants the words
 *  without the styling can ask without re-implementing the grammar. */
export const stripAnsi = (input: string): string =>
  parseAnsi(input)
    .map((span) => span.text)
    .join("");
