//! Color resolution for terminal cells.
//!
//! Maps a `vte::ansi::Color` (Named / Spec / Indexed) to a concrete `Rgb`
//! using the runtime palette stored in `alacritty_terminal::term::color::Colors`
//! (set via OSC sequences) with a built-in default palette when unset.

use alacritty_terminal::term::color::Colors;
use alacritty_terminal::vte::ansi::{Color, NamedColor, Rgb};

/// Default palette, indexed exactly like `alacritty_terminal::term::color::Colors`:
/// 0..16 named ANSI colors, 16..232 color cube, 232..256 grayscale ramp,
/// 256 foreground, 257 background, 258 cursor, 259..267 dim, 267 bright fg, 268 dim bg.
const DEFAULT_PALETTE: [Rgb; 269] = build_default_palette();

/// Default background / foreground / cursor matching the previous xterm.js theme,
/// so the migration is visually lossless.
pub const DEFAULT_BG: Rgb = Rgb {
    r: 0x10,
    g: 0x12,
    b: 0x16,
};
pub const DEFAULT_FG: Rgb = Rgb {
    r: 0xd7,
    g: 0xda,
    b: 0xe0,
};
pub const DEFAULT_CURSOR: Rgb = Rgb {
    r: 0x8b,
    g: 0xd5,
    b: 0xca,
};

/// Resolve a terminal [`Color`] to a concrete [`Rgb`].
///
/// `colors` is the live palette (updated by OSC 4/10/11/12 sequences);
/// entries that are `None` fall back to [`DEFAULT_PALETTE`].
pub fn resolve(color: &Color, colors: &Colors) -> Rgb {
    match *color {
        Color::Spec(rgb) => rgb,
        Color::Named(named) => {
            let idx = named as usize;
            colors[idx].unwrap_or(DEFAULT_PALETTE[idx])
        }
        Color::Indexed(idx) => {
            let idx = idx as usize;
            // Indexed 0..255 aligns with the palette array directly.
            colors[idx].unwrap_or_else(|| default_indexed(idx))
        }
    }
}

/// Resolve only the background color of a cell, honoring the `INVERSE` flag
/// by swapping foreground/background.
pub fn resolve_cell(fg: &Color, bg: &Color, inverse: bool, colors: &Colors) -> (Rgb, Rgb) {
    let fg_rgb = resolve(fg, colors);
    let bg_rgb = resolve(bg, colors);
    if inverse {
        (bg_rgb, fg_rgb)
    } else {
        (fg_rgb, bg_rgb)
    }
}

fn default_indexed(idx: usize) -> Rgb {
    if idx < 16 {
        DEFAULT_PALETTE[idx]
    } else if idx < 232 {
        // 6x6x6 color cube.
        let idx = idx - 16;
        let r = (idx / 36) % 6;
        let g = (idx / 6) % 6;
        let b = idx % 6;
        Rgb {
            r: LEVELS[r],
            g: LEVELS[g],
            b: LEVELS[b],
        }
    } else if idx < 256 {
        // 24-step grayscale ramp.
        let v = (8 + 10 * (idx - 232)) as u8;
        Rgb { r: v, g: v, b: v }
    } else {
        DEFAULT_PALETTE[idx.min(DEFAULT_PALETTE.len() - 1)]
    }
}

const LEVELS: [u8; 6] = [0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff];

const fn build_default_palette() -> [Rgb; 269] {
    // Start from all-black; the relevant slots are overwritten below.
    let mut palette = [Rgb { r: 0, g: 0, b: 0 }; 269];

    // 0..16: named ANSI colors (Catppuccin-derived, matches prior xterm theme).
    palette[NamedColor::Black as usize] = rgb(0x4b, 0x55, 0x63);
    palette[NamedColor::Red as usize] = rgb(0xf3, 0x8b, 0xa8);
    palette[NamedColor::Green as usize] = rgb(0xa6, 0xe3, 0xa1);
    palette[NamedColor::Yellow as usize] = rgb(0xf9, 0xe2, 0xaf);
    palette[NamedColor::Blue as usize] = rgb(0x89, 0xb4, 0xfa);
    palette[NamedColor::Magenta as usize] = rgb(0xf5, 0xc2, 0xe7);
    palette[NamedColor::Cyan as usize] = rgb(0x94, 0xe2, 0xd5);
    palette[NamedColor::White as usize] = rgb(0xcd, 0xd6, 0xf4);
    palette[NamedColor::BrightBlack as usize] = rgb(0x6b, 0x72, 0x80);
    palette[NamedColor::BrightRed as usize] = rgb(0xf3, 0x8b, 0xa8);
    palette[NamedColor::BrightGreen as usize] = rgb(0xa6, 0xe3, 0xa1);
    palette[NamedColor::BrightYellow as usize] = rgb(0xf9, 0xe2, 0xaf);
    palette[NamedColor::BrightBlue as usize] = rgb(0x89, 0xb4, 0xfa);
    palette[NamedColor::BrightMagenta as usize] = rgb(0xf5, 0xc2, 0xe7);
    palette[NamedColor::BrightCyan as usize] = rgb(0x94, 0xe2, 0xd5);
    palette[NamedColor::BrightWhite as usize] = rgb(0xee, 0xf2, 0xff);

    // 256..259: foreground, background, cursor.
    palette[NamedColor::Foreground as usize] = DEFAULT_FG;
    palette[NamedColor::Background as usize] = DEFAULT_BG;
    palette[NamedColor::Cursor as usize] = DEFAULT_CURSOR;

    // 259..267: dim colors (reuse the normal variants).
    palette[NamedColor::DimBlack as usize] = palette[NamedColor::Black as usize];
    palette[NamedColor::DimRed as usize] = palette[NamedColor::Red as usize];
    palette[NamedColor::DimGreen as usize] = palette[NamedColor::Green as usize];
    palette[NamedColor::DimYellow as usize] = palette[NamedColor::Yellow as usize];
    palette[NamedColor::DimBlue as usize] = palette[NamedColor::Blue as usize];
    palette[NamedColor::DimMagenta as usize] = palette[NamedColor::Magenta as usize];
    palette[NamedColor::DimCyan as usize] = palette[NamedColor::Cyan as usize];
    palette[NamedColor::DimWhite as usize] = palette[NamedColor::White as usize];

    // 267: bright foreground, 268: dim background.
    palette[NamedColor::BrightForeground as usize] = palette[NamedColor::White as usize];
    palette[NamedColor::DimForeground as usize] = DEFAULT_FG;

    palette
}

const fn rgb(r: u8, g: u8, b: u8) -> Rgb {
    Rgb { r, g, b }
}
