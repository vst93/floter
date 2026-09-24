// R48 · the inline trigger hint's *floor*.
//
// R43 drew the ordinary search page's trigger hint ("space enters this plugin")
// as a subline of its own under the field, sharing the chips row's band. The
// user read that back as a line the window should not be paying for:
// 「还有输入框下方，提示"空格进入某个具体插件命令"的内容，放在输入框下占用了一行，
// 似乎不太合适。能不能在输入框当中做些标识或者提示」. R48 moves the nudge *into*
// the field row, so it costs no band and no `setSize`; the judgement of *what*
// to hint about is untouched (`plugins/external.ts` owns `externalTriggerHint`).
//
// What is left for this module is the one rule the layout cannot express in CSS:
// how little room the nudge may keep before it is better gone than truncated to
// a stub. The field row is a flex line — the input takes the row's width and
// the nudge rides whatever it leaves — so the nudge's own width is decided by
// the row, and CSS `text-overflow` can turn it into an ellipsis but cannot ask
// "is this less than a word?". The App measures the rendered width and asks
// this function; the measured `ch` is the element's own font, so the floor
// follows the interface step with no second number.
//
// Pure (no React, no DOM, no Tauri) so the node suite drives the boundary
// without laying out a window.

/** How many `ch` the inline hint keeps before it is removed entirely. Below
 *  this the ellipsis would leave a two-glyph stub — a mark with no reading —
 *  so the nudge disappears rather than shrinking to noise. Six cells is the
 *  width of the space glyph plus a two-letter word. */
export const TRIGGER_HINT_MIN_CELLS = 6;

/** Whether the hint still has room to be read at `width`, in the pixels a
 *  layout pass handed it, given the advance width of one `ch` in its own font.
 *
 *  A non-positive `cellWidth` means the font could not be measured; the hint is
 *  then kept (it is already ellipsised by CSS and a wrong *hide* loses the
 *  nudge for good, while a wrong *show* costs one line of muted text). */
export const triggerHintFits = (
  width: number,
  cellWidth: number,
  minCells: number = TRIGGER_HINT_MIN_CELLS,
): boolean => !(cellWidth > 0) || width >= cellWidth * minCells;
