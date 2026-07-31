//! Serialization of the terminal's visible grid into a compact binary frame.
//!
//! Wire format (all little-endian):
//!
//! ```text
//! u16 cols
//! u16 rows
//! u16 cursor_col
//! u16 cursor_row
//! u8  cursor_shape     // 0 block, 1 underline, 2 beam, 3 hollow, 4 hidden
//! u8  cursor_visible   // 0/1
//! u8  cursor_blinking  // 0/1
//! u32 mode             // active TermMode bits (APP_CURSOR etc.)
//! u32 history_size     // scrollback lines available above the viewport
//! u32 display_offset   // 0 at the bottom (prompt); increases into history
//! // followed by cols*rows cells, each:
//! u32 char             // unicode codepoint (space == 0x20)
//! u32 fg               // 0x00RRGGBB
//! u32 bg               // 0x00RRGGBB
//! u8  flags            // bit0 bold, 1 italic, 2 underline, 3 strike,
//!                      //     4 dim, 5 hidden, 6 wide
//! ```

use alacritty_terminal::grid::Dimensions;
use alacritty_terminal::index::Line;
use alacritty_terminal::term::cell::{Cell, Flags};
use alacritty_terminal::term::color::Colors;
use alacritty_terminal::term::{Term, TermMode};
use alacritty_terminal::vte::ansi::{CursorShape, Rgb};

use super::color;

/// Bit flags packed into the per-cell `flags` byte.
const FLAG_BOLD: u8 = 1 << 0;
const FLAG_ITALIC: u8 = 1 << 1;
const FLAG_UNDERLINE: u8 = 1 << 2;
const FLAG_STRIKE: u8 = 1 << 3;
const FLAG_DIM: u8 = 1 << 4;
const FLAG_HIDDEN: u8 = 1 << 5;
const FLAG_WIDE: u8 = 1 << 6;

/// Serialize the visible portion of `term` into a binary frame.
pub fn serialize<T>(term: &Term<T>) -> Vec<u8> {
    let grid = term.grid();
    let colors = term.colors();

    let cols = grid.columns();
    let rows = grid.screen_lines();
    let cell_count = cols.checked_mul(rows).unwrap_or(0);

    // Header is 23 bytes; each cell is 13 bytes.
    let mut out = Vec::with_capacity(23 + cell_count * 13);
    out.extend_from_slice(&(cols as u16).to_le_bytes());
    out.extend_from_slice(&(rows as u16).to_le_bytes());

    let (cursor_col, cursor_row, cursor_shape, cursor_visible, cursor_blinking) = cursor(term);
    out.extend_from_slice(&cursor_col.to_le_bytes());
    out.extend_from_slice(&cursor_row.to_le_bytes());
    out.push(cursor_shape);
    out.push(cursor_visible);
    out.push(cursor_blinking);
    out.extend_from_slice(&(term.mode().bits()).to_le_bytes());
    out.extend_from_slice(&(grid.history_size() as u32).to_le_bytes());
    out.extend_from_slice(&(grid.display_offset() as u32).to_le_bytes());

    for indexed in grid.display_iter() {
        write_cell(&mut out, indexed.cell, colors);
    }

    out
}

fn write_cell(out: &mut Vec<u8>, cell: &Cell, colors: &Colors) {
    let inverse = cell.flags.contains(Flags::INVERSE);
    let (fg, bg) = color::resolve_cell(&cell.fg, &cell.bg, inverse, colors);

    let codepoint = if cell.c.is_control() || cell.flags.contains(Flags::WIDE_CHAR_SPACER) {
        // Wide-char spacers and control chars render as blank.
        ' ' as u32
    } else {
        cell.c as u32
    };

    out.extend_from_slice(&codepoint.to_le_bytes());
    out.extend_from_slice(&pack_rgb(fg).to_le_bytes());
    out.extend_from_slice(&pack_rgb(bg).to_le_bytes());
    out.push(pack_flags(&cell.flags));
}

fn cursor<T>(term: &Term<T>) -> (u16, u16, u8, u8, u8) {
    let grid = term.grid();
    let point = grid.cursor.point;
    let display_offset = grid.display_offset();

    let style = term.cursor_style();
    let shape_byte = match style.shape {
        CursorShape::Block => 0,
        CursorShape::Underline => 1,
        CursorShape::Beam => 2,
        CursorShape::HollowBlock => 3,
        CursorShape::Hidden => 4,
    };

    let visible = term.mode().contains(TermMode::SHOW_CURSOR)
        && style.shape != CursorShape::Hidden
        && display_offset == 0
        && point.line >= Line(0)
        && point.column <= grid.last_column();

    let row = if point.line < Line(0) {
        0
    } else {
        point.line.0 as u16
    };
    let col = point.column.0 as u16;

    (col, row, shape_byte, visible as u8, style.blinking as u8)
}

#[inline]
fn pack_rgb(rgb: Rgb) -> u32 {
    ((rgb.r as u32) << 16) | ((rgb.g as u32) << 8) | (rgb.b as u32)
}

fn pack_flags(flags: &Flags) -> u8 {
    let mut bits = 0u8;
    if flags.contains(Flags::BOLD) {
        bits |= FLAG_BOLD;
    }
    if flags.contains(Flags::ITALIC) {
        bits |= FLAG_ITALIC;
    }
    if flags.intersects(Flags::ALL_UNDERLINES) {
        bits |= FLAG_UNDERLINE;
    }
    if flags.contains(Flags::STRIKEOUT) {
        bits |= FLAG_STRIKE;
    }
    if flags.contains(Flags::DIM) {
        bits |= FLAG_DIM;
    }
    if flags.contains(Flags::HIDDEN) {
        bits |= FLAG_HIDDEN;
    }
    if flags.contains(Flags::WIDE_CHAR) {
        bits |= FLAG_WIDE;
    }
    bits
}

#[cfg(test)]
mod tests {
    use super::*;
    use alacritty_terminal::event::VoidListener;
    use alacritty_terminal::grid::Dimensions;
    use alacritty_terminal::term::{Config, TermMode};
    use alacritty_terminal::vte::ansi::Processor;

    struct Size(usize, usize);
    impl Dimensions for Size {
        fn total_lines(&self) -> usize {
            self.1
        }
        fn screen_lines(&self) -> usize {
            self.1
        }
        fn columns(&self) -> usize {
            self.0
        }
    }

    #[test]
    fn serializes_text_and_cursor() {
        let size = Size(10, 4);
        let mut term = Term::new(Config::default(), &size, VoidListener);
        let mut processor: Processor = Processor::new();
        processor.advance(&mut term, b"Hi");

        let frame = serialize(&term);

        // Header (23) + 10*4 cells * 13 bytes.
        assert_eq!(frame.len(), 23 + 10 * 4 * 13);

        let cols = u16::from_le_bytes([frame[0], frame[1]]);
        let rows = u16::from_le_bytes([frame[2], frame[3]]);
        assert_eq!(cols, 10);
        assert_eq!(rows, 4);

        let cursor_col = u16::from_le_bytes([frame[4], frame[5]]);
        let cursor_row = u16::from_le_bytes([frame[6], frame[7]]);
        assert_eq!(cursor_col, 2);
        assert_eq!(cursor_row, 0);

        // First cell char should be 'H', second 'i'.
        let char_at = |i: usize| {
            let off = 23 + i * 13;
            u32::from_le_bytes([frame[off], frame[off + 1], frame[off + 2], frame[off + 3]])
        };
        assert_eq!(char_at(0), b'H' as u32);
        assert_eq!(char_at(1), b'i' as u32);

        // Mode word is present and readable.
        let _mode = u32::from_le_bytes([frame[11], frame[12], frame[13], frame[14]]);
    }

    #[test]
    fn carries_alternate_screen_mode_to_the_frontend() {
        let size = Size(10, 4);
        let mut term = Term::new(Config::default(), &size, VoidListener);
        let mut processor: Processor = Processor::new();
        processor.advance(&mut term, b"\x1b[?1049h");

        let frame = serialize(&term);
        let mode = u32::from_le_bytes([frame[11], frame[12], frame[13], frame[14]]);
        assert_ne!(mode & TermMode::ALT_SCREEN.bits(), 0);
    }
}
