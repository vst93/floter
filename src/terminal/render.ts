// Canvas renderer for the alacritty-backed terminal.
//
import { ALT_SCREEN } from "./input";

// Consumes the binary frame produced by the Rust backend (see
// `src-tauri/src/terminal/frame.rs` for the wire format) and paints it onto a
// <canvas>. The frontend is intentionally stateless per-frame: every frame
// fully replaces the previous one. Selection overlay + scrollbar are drawn
// from geometry the renderer exposes to the host component.

// Fallbacks for the theme colours below, matching the dark palette in App.css.
// They are only reached when the stylesheet has not applied yet — a canvas
// painted in a colour nothing else on screen uses would be the more visible
// failure.
const FALLBACK_BG = 0x111214;
const FALLBACK_FG = 0xd7dae0;
const FALLBACK_CURSOR = 0x8bd5ca;
const FALLBACK_SELECTION = "rgba(255, 255, 255, 0.16)";
const FALLBACK_SCROLLBAR = "rgba(255, 255, 255, 0.32)";

// What a cell carrying *no* colour of its own looks like on the wire: the
// backend resolves the default foreground and background to these exact values
// (`DEFAULT_FG` / `DEFAULT_BG` in `src-tauri/src/terminal/color.rs`), so they
// are sentinels to compare against, never colours to paint with. Every cell
// that matches is painted in the current theme instead, which is what lets one
// frame render in either palette — a program that picked its own colours keeps
// them, because those arrive as themselves.
const WIRE_BG = 0x101216;
const WIRE_FG = 0xd7dae0;

// Cell flag bits (must match `frame.rs`).
const FLAG_BOLD = 1 << 0;
const FLAG_ITALIC = 1 << 1;
const FLAG_UNDERLINE = 1 << 2;
const FLAG_STRIKE = 1 << 3;
const FLAG_DIM = 1 << 4;
const FLAG_HIDDEN = 1 << 5;
const FLAG_WIDE = 1 << 6;

const CURSOR_BLOCK = 0;
const CURSOR_UNDERLINE = 1;
const CURSOR_BEAM = 2;
const CURSOR_HOLLOW = 3;
const CURSOR_HIDDEN = 4;

const HEADER_BYTES = 23;
const CELL_BYTES = 13;
const COMBINING_RECORD_HEADER_BYTES = 6;
const SCROLLBAR_WIDTH = 5;
const SCROLLBAR_GAP = 0;
/**
 * How long the scrollbar stays up after the last thing that moved it.
 *
 * It is an overlay — it reserves no columns, so the grid is the same width
 * whether it is there or not — which also means it paints *over* whatever the
 * program below is drawing. A full-width background or a centred box in a
 * program that does not use the alternate screen would wear a permanent notch
 * down its right edge, so the bar is only up while it is telling the user
 * something: during a scroll, a drag, or a hover over its own strip.
 */
const SCROLLBAR_LINGER = 1100;
/** Final stretch of the linger, spent fading rather than simply vanishing. */
const SCROLLBAR_FADE = 260;

export interface RendererOptions {
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  paddingX: number;
  paddingY: number;
}

export interface Layout {
  cols: number;
  rows: number;
}

export interface Selection {
  startCol: number;
  startRow: number;
  endCol: number;
  endRow: number;
}

export interface CellPoint {
  col: number;
  row: number;
}

export interface ScrollbarRect {
  x: number;
  y: number;
  w: number;
  h: number;
  thumbY: number;
  thumbH: number;
}

export interface CursorRect {
  x: number;
  y: number;
  height: number;
}

export class TerminalCanvas {
  private ctx: CanvasRenderingContext2D;
  private dpr = 1;
  private cssWidth = 0;
  private cssHeight = 0;
  cellWidth = 0;
  cellHeight = 0;
  mode = 0;
  cols = 0;
  rows = 0;
  historySize = 0;
  displayOffset = 0;
  private cursorCol = 0;
  private cursorRow = 0;
  private lastBytes: Uint8Array | null = null;
  private combining = new Map<number, string>();
  private lastFont = "";
  private colorCache = new Map<number, string>();

  // The palette in use, refreshed by `updateTheme`. Held here rather than read
  // per frame: `getComputedStyle` forces a style resolution, and a frame arrives
  // for every burst of terminal output.
  private bg = FALLBACK_BG;
  private bgOpacity = 1;
  private fg = FALLBACK_FG;
  private cursor = FALLBACK_CURSOR;
  private selection = FALLBACK_SELECTION;
  private scrollbar = FALLBACK_SCROLLBAR;
  /** Timestamp the scrollbar stays visible until; see `SCROLLBAR_LINGER`. */
  private scrollbarUntil = 0;
  /** Display offset of the previous frame, to notice a scroll without being
   * told about one: any frame that moved it is a frame worth showing the bar
   * for, whoever caused it — wheel, keyboard, or the program itself. */
  private lastDisplayOffset = -1;

  constructor(
    private canvas: HTMLCanvasElement,
    private opts: RendererOptions,
  ) {
    const ctx = canvas.getContext("2d", { alpha: true });
    if (!ctx) throw new Error("2d canvas context unavailable");
    this.ctx = ctx;
    this.updateTheme();
    this.measureCell();
  }

  /**
   * Re-read the palette from the document's CSS custom properties.
   *
   * The variables are declared on `:root` and `[data-theme="light"]`, so the
   * document element is what has to be measured — they are inherited by the
   * canvas but the renderer wants whichever block currently applies, and asking
   * the root is how a `data-theme` switch is observed at all.
   *
   * Public because a theme change repaints in place: rebuilding the renderer
   * would take the scroll position and the last frame with it.
   */
  updateTheme(): void {
    const style = getComputedStyle(document.documentElement);
    this.bg = packedColor(style, "--terminal-bg", FALLBACK_BG);
    this.bgOpacity = cssNumber(style, "--terminal-opacity", 1);
    this.fg = packedColor(style, "--terminal-fg", FALLBACK_FG);
    this.cursor = packedColor(style, "--terminal-cursor", FALLBACK_CURSOR);
    // Kept as CSS strings: both are deliberately translucent, and the packed
    // integers the cell colours use have nowhere to put an alpha channel.
    this.selection = cssColor(style, "--terminal-selection", FALLBACK_SELECTION);
    this.scrollbar = cssColor(style, "--terminal-scrollbar", FALLBACK_SCROLLBAR);
  }

  private fontString(bold: boolean, italic: boolean): string {
    const style = bold ? (italic ? "bold italic" : "bold") : italic ? "italic" : "normal";
    return `${style} ${this.opts.fontSize}px ${this.opts.fontFamily}`;
  }

  private measureCell(): void {
    const ctx = this.ctx;
    ctx.font = this.fontString(true, false);
    const width = ctx.measureText("M").width;
    this.cellWidth = Math.max(1, Math.ceil(width));
    this.cellHeight = Math.max(1, Math.ceil(this.opts.fontSize * this.opts.lineHeight));
  }

  /** (Re)compute the backing store and derived grid dimensions. */
  relayout(cssWidth: number, cssHeight: number): Layout {
    this.dpr = window.devicePixelRatio || 1;
    this.cssWidth = cssWidth;
    this.cssHeight = cssHeight;

    this.canvas.width = Math.floor(cssWidth * this.dpr);
    this.canvas.height = Math.floor(cssHeight * this.dpr);
    this.canvas.style.width = `${cssWidth}px`;
    this.canvas.style.height = `${cssHeight}px`;
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);

    this.measureCell();

    // The scrollbar is an overlay drawn in the right padding gutter, so it
    // does not reserve any columns; text uses the full inner width.
    const innerWidth = cssWidth - this.opts.paddingX * 2;
    const innerHeight = cssHeight - this.opts.paddingY * 2;
    return {
      cols: Math.max(1, Math.floor(innerWidth / this.cellWidth)),
      rows: Math.max(1, Math.floor(innerHeight / this.cellHeight)),
    };
  }

  private color(packed: number, alpha = 1): string {
    if (alpha !== 1) {
      const red = (packed >> 16) & 0xff;
      const green = (packed >> 8) & 0xff;
      const blue = packed & 0xff;
      return `rgb(${red} ${green} ${blue} / ${alpha})`;
    }
    let s = this.colorCache.get(packed);
    if (s) return s;
    s = `#${(packed & 0xffffff).toString(16).padStart(6, "0")}`;
    if (this.colorCache.size > 4096) this.colorCache.clear();
    this.colorCache.set(packed, s);
    return s;
  }

  private setFont(bold: boolean, italic: boolean): void {
    const f = this.fontString(bold, italic);
    if (f !== this.lastFont) {
      this.ctx.font = f;
      this.lastFont = f;
    }
  }

  draw(bytes: Uint8Array, blinkOn: boolean, selection: Selection | null = null): void {
    const ctx = this.ctx;
    const { paddingX, paddingY } = this.opts;
    const themeBg = this.bg;
    const themeFg = this.fg;

    ctx.clearRect(0, 0, this.cssWidth, this.cssHeight);
    ctx.fillStyle = this.color(themeBg, this.bgOpacity);
    ctx.fillRect(0, 0, this.cssWidth, this.cssHeight);

    this.lastBytes = bytes;
    if (bytes.byteLength < HEADER_BYTES) return;

    const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    const cols = dv.getUint16(0, true);
    const rows = dv.getUint16(2, true);
    this.combining = readCombining(bytes, cols * rows);
    const cursorCol = dv.getUint16(4, true);
    const cursorRow = dv.getUint16(6, true);
    this.cursorCol = cursorCol;
    this.cursorRow = cursorRow;
    const cursorShape = dv.getUint8(8);
    const cursorVisible = dv.getUint8(9) === 1;
    const cursorBlinking = dv.getUint8(10) === 1;
    this.mode = dv.getUint32(11, true);
    this.historySize = dv.getUint32(15, true);
    this.displayOffset = dv.getUint32(19, true);
    if (this.displayOffset !== this.lastDisplayOffset) {
      if (this.lastDisplayOffset !== -1) this.showScrollbar();
      this.lastDisplayOffset = this.displayOffset;
    }
    this.cols = cols;
    this.rows = rows;

    ctx.textBaseline = "top";

    const cw = this.cellWidth;
    const ch = this.cellHeight;
    const glyphOffset = Math.round((ch - this.opts.fontSize) / 2);

    let cursorChar = 0x20;
    let cursorCombining = "";
    let cursorFg = WIRE_FG;
    let cursorBg = WIRE_BG;
    let cursorFlags = 0;

    let off = HEADER_BYTES;
    let fontBold = false;
    let fontItalic = false;
    this.setFont(false, false);

    for (let row = 0; row < rows; row++) {
      const y = paddingY + row * ch;
      for (let col = 0; col < cols; col++) {
        const char = dv.getUint32(off, true);
        const fg = dv.getUint32(off + 4, true);
        const bg = dv.getUint32(off + 8, true);
        const flags = dv.getUint8(off + 12);
        off += CELL_BYTES;

        if (row === cursorRow && col === cursorCol) {
          cursorChar = char;
          cursorCombining = this.combining.get(row * cols + col) ?? "";
          cursorFg = fg;
          cursorBg = bg;
          cursorFlags = flags;
        }

        const wide = (flags & FLAG_WIDE) !== 0;
        const x = paddingX + col * cw;
        const w = wide ? cw * 2 : cw;

        // A default background needs no fill at all: the whole canvas was
        // already painted in it above.
        if (bg !== WIRE_BG) {
          ctx.fillStyle = this.color(bg);
          ctx.fillRect(x, y, w, ch);
        }

        // Resolved before the early return below so an underline or a strike on
        // an otherwise blank cell is drawn in the theme's colour too.
        const cellFg = fg === WIRE_FG ? themeFg : fg;

        const hidden = (flags & FLAG_HIDDEN) !== 0;
        if (hidden || char === 0x20) {
          this.drawUnderlineStrike(x, y, w, cellFg, flags);
          continue;
        }

        const bold = (flags & FLAG_BOLD) !== 0;
        const italic = (flags & FLAG_ITALIC) !== 0;
        const dim = (flags & FLAG_DIM) !== 0;
        // Dim halves the distance to whatever is actually behind the glyph, so
        // the theme background is what a default-background cell mixes toward.
        const effectiveFg = dim
          ? mix(cellFg, bg === WIRE_BG ? themeBg : bg, 0.5)
          : cellFg;

        if (bold !== fontBold || italic !== fontItalic) {
          fontBold = bold;
          fontItalic = italic;
          this.setFont(fontBold, fontItalic);
        }

        ctx.fillStyle = this.color(effectiveFg);
        const cellIndex = row * cols + col;
        ctx.fillText(String.fromCodePoint(char) + (this.combining.get(cellIndex) ?? ""), x, y + glyphOffset);

        this.drawUnderlineStrike(x, y, w, cellFg, flags);
      }
    }

    if (selection) {
      this.drawSelection(selection);
    }

    const showCursor =
      cursorVisible &&
      cursorShape !== CURSOR_HIDDEN &&
      (!cursorBlinking || blinkOn) &&
      cursorCol < cols &&
      cursorRow < rows;

    if (showCursor) {
      this.drawCursor(
        cursorCol,
        cursorRow,
        cursorShape,
        cursorChar,
        cursorCombining,
        cursorFg,
        cursorBg,
        cursorFlags,
      );
    }

    this.drawScrollbar();
  }

  private drawSelection(sel: Selection): void {
    const ctx = this.ctx;
    const { paddingX, paddingY } = this.opts;
    const cw = this.cellWidth;
    const ch = this.cellHeight;
    const { startCol, startRow, endCol, endRow } = normalizeSelection(sel, this.cols, this.rows);
    if (endRow < startRow) return;

    ctx.fillStyle = this.selection;
    for (let row = startRow; row <= endRow; row++) {
      const y = paddingY + row * ch;
      const colStart = row === startRow ? startCol : 0;
      const colEnd = row === endRow ? endCol : this.cols - 1;
      const x = paddingX + colStart * cw;
      const w = (colEnd - colStart + 1) * cw;
      ctx.fillRect(x, y, w, ch);
    }
  }

  private drawUnderlineStrike(x: number, y: number, w: number, fg: number, flags: number): void {
    const ctx = this.ctx;
    if (flags & (FLAG_UNDERLINE | FLAG_STRIKE)) {
      ctx.strokeStyle = this.color(fg);
      ctx.lineWidth = 1;
      ctx.beginPath();
      if (flags & FLAG_UNDERLINE) {
        const uy = y + this.cellHeight - 2;
        ctx.moveTo(x + 0.5, uy);
        ctx.lineTo(x + w - 0.5, uy);
      }
      if (flags & FLAG_STRIKE) {
        const sy = y + Math.round(this.cellHeight / 2);
        ctx.moveTo(x + 0.5, sy);
        ctx.lineTo(x + w - 0.5, sy);
      }
      ctx.stroke();
    }
  }

  private drawCursor(
    col: number,
    row: number,
    shape: number,
    cellChar: number,
    combining: string,
    // The foreground and background of the cell the cursor sits on, in the order
    // the call site passes them. Only the foreground is used — a block cursor
    // redraws the glyph over its own fill.
    cellFg: number,
    _cellBg: number,
    cellFlags: number,
  ): void {
    const ctx = this.ctx;
    const { paddingX, paddingY } = this.opts;
    const cw = this.cellWidth;
    const ch = this.cellHeight;
    const x = paddingX + col * cw;
    const y = paddingY + row * ch;
    const glyphOffset = Math.round((ch - this.opts.fontSize) / 2);

    ctx.save();
    switch (shape) {
      case CURSOR_BLOCK: {
        ctx.fillStyle = this.color(this.cursor);
        ctx.fillRect(x, y, cw, ch);
        if (!(cellFlags & FLAG_HIDDEN) && cellChar !== 0x20) {
          ctx.fillStyle = this.color(cellFg === WIRE_FG ? this.fg : cellFg);
          this.setFont(false, false);
          ctx.textBaseline = "top";
          ctx.fillText(String.fromCodePoint(cellChar) + combining, x, y + glyphOffset);
        }
        break;
      }
      case CURSOR_BEAM: {
        ctx.fillStyle = this.color(this.cursor);
        ctx.fillRect(x, y, Math.max(2, Math.round(cw * 0.18)), ch);
        break;
      }
      case CURSOR_UNDERLINE: {
        const h = Math.max(2, Math.round(ch * 0.18));
        ctx.fillStyle = this.color(this.cursor);
        ctx.fillRect(x, y + ch - h, cw, h);
        break;
      }
      case CURSOR_HOLLOW: {
        ctx.strokeStyle = this.color(this.cursor);
        ctx.lineWidth = 1.5;
        ctx.strokeRect(x + 0.75, y + 0.75, cw - 1.5, ch - 1.5);
        break;
      }
      default:
        break;
    }
    ctx.restore();
    // restore() also restores the font. Keep the cache synchronized with the
    // actual context after a block cursor temporarily selected the regular face.
    this.lastFont = ctx.font;
  }

  /** Bring the scrollbar up, or keep it up. */
  showScrollbar(): void {
    this.scrollbarUntil = Date.now() + SCROLLBAR_LINGER;
  }

  /** How solid the bar should be drawn right now, 0 when it is not up. */
  private scrollbarAlpha(): number {
    const remaining = this.scrollbarUntil - Date.now();
    if (remaining <= 0) return 0;
    return remaining >= SCROLLBAR_FADE ? 1 : remaining / SCROLLBAR_FADE;
  }

  /** Whether another frame is owed to finish the fade; see `TerminalCanvas`
   * callers, which keep repainting while this is true. */
  scrollbarFading(): boolean {
    return this.scrollbarAlpha() > 0;
  }

  private drawScrollbar(): void {
    if ((this.mode & ALT_SCREEN) !== 0 || this.historySize <= 0) return;
    const alpha = this.scrollbarAlpha();
    if (alpha <= 0) return;
    const ctx = this.ctx;
    const rect = this.scrollbarRect();
    if (!rect) return;

    ctx.save();
    ctx.globalAlpha = alpha;
    ctx.fillStyle = this.scrollbar;
    const thumbH = Math.max(rect.h * 0.1, rect.thumbH);
    roundRectPath(ctx, rect.x, rect.thumbY, rect.w, thumbH, rect.w / 2);
    ctx.fill();
    ctx.restore();
  }

  /** Map a canvas-space pixel to a cell, or null when outside the text area. */
  pixelToCell(px: number, py: number): CellPoint | null {
    const col = Math.floor((px - this.opts.paddingX) / this.cellWidth);
    const row = Math.floor((py - this.opts.paddingY) / this.cellHeight);
    if (col < 0 || col >= this.cols || row < 0 || row >= this.rows) return null;
    return { col, row };
  }

  /** Position the browser text-input proxy at the visible terminal cursor. */
  cursorRect(): CursorRect {
    return {
      x: this.opts.paddingX + Math.min(this.cursorCol, Math.max(0, this.cols - 1)) * this.cellWidth,
      y: this.opts.paddingY + Math.min(this.cursorRow, Math.max(0, this.rows - 1)) * this.cellHeight,
      height: this.cellHeight,
    };
  }

  /** True if the pixel is within the scrollbar track. */
  hitScrollbar(px: number, _py: number): boolean {
    if ((this.mode & ALT_SCREEN) !== 0 || this.historySize <= 0) return false;
    const rect = this.scrollbarRect();
    if (!rect) return false;
    return px >= rect.x && px <= rect.x + rect.w;
  }

  /** Scrollbar geometry (track + thumb) in canvas space. */
  scrollbarRect(): ScrollbarRect | null {
    if (this.cssHeight === 0) return null;
    const x = this.cssWidth - SCROLLBAR_WIDTH - SCROLLBAR_GAP;
    const y = this.opts.paddingY;
    const h = this.cssHeight - this.opts.paddingY * 2;
    const total = this.historySize + this.rows;
    const topFrac = total > 0 ? (this.historySize - this.displayOffset) / total : 0;
    const heightFrac = total > 0 ? this.rows / total : 1;
    const thumbH = Math.max(h * 0.1, heightFrac * h);
    const thumbY = y + Math.max(0, Math.min(h - thumbH, topFrac * h));
    return { x, y, w: SCROLLBAR_WIDTH, h, thumbY, thumbH };
  }

  /** Target display offset for a scrollbar drag at canvas-space pixel `py`. */
  offsetFromDragY(py: number): number {
    const rect = this.scrollbarRect();
    if (!rect) return 0;
    const total = this.historySize + this.rows;
    const frac = Math.max(0, Math.min(1, (py - rect.y - rect.thumbH / 2) / rect.h));
    const topPos = Math.round(frac * total); // lines from top of history
    const target = this.historySize - topPos;
    return Math.max(0, Math.min(this.historySize, target));
  }

  /** Extract text for a stream selection from the last rendered frame. */
  selectionText(sel: Selection): string {
    if (!this.lastBytes) return "";
    const { startCol, startRow, endCol, endRow } = normalizeSelection(sel, this.cols, this.rows);
    if (endRow < startRow) return "";

    const dv = new DataView(
      this.lastBytes.buffer,
      this.lastBytes.byteOffset,
      this.lastBytes.byteLength,
    );
    const lines: string[] = [];
    for (let row = startRow; row <= endRow; row++) {
      const colStart = row === startRow ? startCol : 0;
      const colEnd = row === endRow ? endCol : this.cols - 1;
      let line = "";
      for (let col = colStart; col <= colEnd; col++) {
        const off = HEADER_BYTES + (row * this.cols + col) * CELL_BYTES;
        if (off + CELL_BYTES > dv.byteLength) break;
        const char = dv.getUint32(off, true);
        const flags = dv.getUint8(off + 12);
        if (char === 0x20) {
          line += " ";
        } else {
          line += String.fromCodePoint(char) + (this.combining.get(row * this.cols + col) ?? "");
          if (flags & FLAG_WIDE) col++; // skip the wide-char spacer
        }
      }
      lines.push(line.replace(/\s+$/g, ""));
    }
    return lines.join("\n");
  }

  /** Expand a selection to the word (non-whitespace run) at the given cell. */
  wordSelection(point: CellPoint): Selection | null {
    if (!this.lastBytes || point.col < 0 || point.col >= this.cols) return null;
    const dv = new DataView(
      this.lastBytes.buffer,
      this.lastBytes.byteOffset,
      this.lastBytes.byteLength,
    );
    const charAt = (col: number): number => {
      const off = HEADER_BYTES + (point.row * this.cols + col) * CELL_BYTES;
      if (off + CELL_BYTES > dv.byteLength) return 0x20;
      return dv.getUint32(off, true);
    };
    let startCol = point.col;
    let endCol = point.col;
    while (startCol > 0 && !isSpace(charAt(startCol - 1))) startCol--;
    while (endCol < this.cols - 1 && !isSpace(charAt(endCol + 1))) endCol++;
    return { startCol, startRow: point.row, endCol, endRow: point.row };
  }

  focus(): void {
    this.canvas.focus?.();
  }
}

function isSpace(char: number): boolean {
  return char === 0x20 || char === 0x09;
}

function readCombining(bytes: Uint8Array, cellCount: number): Map<number, string> {
  const values = new Map<number, string>();
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let offset = HEADER_BYTES + cellCount * CELL_BYTES;
  const decoder = new TextDecoder();
  while (offset + COMBINING_RECORD_HEADER_BYTES <= bytes.byteLength) {
    const cellIndex = dv.getUint32(offset, true);
    const byteLength = dv.getUint16(offset + 4, true);
    offset += COMBINING_RECORD_HEADER_BYTES;
    if (cellIndex >= cellCount || offset + byteLength > bytes.byteLength) break;
    values.set(cellIndex, decoder.decode(bytes.subarray(offset, offset + byteLength)));
    offset += byteLength;
  }
  return values;
}

/**
 * A `--terminal-*` custom property as a packed `0xrrggbb`.
 *
 * Packed rather than kept as a string because these three take part in
 * arithmetic — [`mix`] blends a dim glyph toward its background — and because
 * they are compared against the integers the frame carries.
 *
 * Only the notations App.css actually uses are understood (`#rrggbb`, `#rgb`
 * and `rgb()`/`rgba()`); anything else falls back rather than painting a
 * half-parsed colour. Note that a `getPropertyValue` on a *custom* property
 * returns the declaration verbatim, not a normalized `rgb()` triple, so the hex
 * branch is the one that runs today.
 */
function packedColor(style: CSSStyleDeclaration, name: string, fallback: number): number {
  const value = style.getPropertyValue(name).trim();
  if (!value) return fallback;

  if (value.startsWith("#")) {
    const hex = value.slice(1);
    // `#abc` is shorthand for `#aabbcc`.
    const expanded =
      hex.length === 3
        ? hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2]
        : hex;
    if (expanded.length === 6) {
      const packed = Number.parseInt(expanded, 16);
      if (!Number.isNaN(packed)) return packed;
    }
    return fallback;
  }

  const rgb = value.match(/^rgba?\(\s*(\d+)[\s,]+(\d+)[\s,]+(\d+)/);
  if (rgb) {
    return (Number(rgb[1]) << 16) | (Number(rgb[2]) << 8) | Number(rgb[3]);
  }
  return fallback;
}

/** A `--terminal-*` custom property left as written, for translucent overlays. */
function cssColor(style: CSSStyleDeclaration, name: string, fallback: string): string {
  return style.getPropertyValue(name).trim() || fallback;
}

function cssNumber(style: CSSStyleDeclaration, name: string, fallback: number): number {
  const value = Number.parseFloat(style.getPropertyValue(name));
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : fallback;
}

/// Path for a rounded rectangle (canvas `roundRect` fallback).
function roundRectPath(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
): void {
  const radius = Math.min(r, w / 2, h / 2);
  ctx.beginPath();
  ctx.moveTo(x + radius, y);
  ctx.arcTo(x + w, y, x + w, y + h, radius);
  ctx.arcTo(x + w, y + h, x, y + h, radius);
  ctx.arcTo(x, y + h, x, y, radius);
  ctx.arcTo(x, y, x + w, y, radius);
  ctx.closePath();
}

function normalizeSelection(
  sel: Selection,
  cols: number,
  rows: number,
): Selection {
  const clamp = (v: number, max: number) => Math.max(0, Math.min(max, v));
  let { startCol, startRow, endCol, endRow } = sel;
  startCol = clamp(startCol, cols - 1);
  endCol = clamp(endCol, cols - 1);
  startRow = clamp(startRow, rows - 1);
  endRow = clamp(endRow, rows - 1);
  if (startRow > endRow || (startRow === endRow && startCol > endCol)) {
    [startCol, endCol] = [endCol, startCol];
    [startRow, endRow] = [endRow, startRow];
  }
  return { startCol, startRow, endCol, endRow };
}

function mix(a: number, b: number, t: number): number {
  const ar = (a >> 16) & 0xff, ag = (a >> 8) & 0xff, ab = a & 0xff;
  const br = (b >> 16) & 0xff, bg = (b >> 8) & 0xff, bb = b & 0xff;
  const r = Math.round(ar + (br - ar) * t);
  const g = Math.round(ag + (bg - ag) * t);
  const bl = Math.round(ab + (bb - ab) * t);
  return (r << 16) | (g << 8) | bl;
}

/** Decode a base64 frame into raw bytes. */
export function decodeFrame(b64: string): Uint8Array {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return bytes;
}
