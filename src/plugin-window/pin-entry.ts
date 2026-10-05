// R90 · which result rows can be pinned as a text window, and what text they
// carry.
//
// R84's pin could only leave from the plugin *page* (an external command's
// output). R90 lets the user pin **one selected result** — the smallest
// honest cut of 「钉住单条内容」: the detached window already knows how to show
// a block of text (`PluginTextView`), so a row only has to hand it a snapshot.
//
// The decision is two pure functions rather than a branch inside the renderer,
// for the reason `launcher/plugin-mode.ts` and `detach.ts` are pure: a review
// could silently widen "which rows are pinnable" and nothing would look
// broken, so the rule lives where a node test imports it without a DOM.
//
// Three row types carry a single piece of text, and only those three:
//
//   · `history`   → the recalled command line itself;
//   · `clipboard` → the entry's `text` (an image entry's caption, a text
//                   entry's body; a files entry has none and is not pinnable);
//   · `plugin`    → the row's own text, resolved through the same
//                   `resolvePluginView` the capability layer uses — a row whose
//                   content does not read as text has nothing to pin.
//
// browser / calculator / app / system / command / file rows carry no single
// text body and are deliberately absent: pinning a URL's label or a power
// action as a text window would be a lie about what the window holds.

import type { LauncherItem } from "../launcher/LauncherResults.tsx";
import { resolvePluginView } from "../launcher/plugin-mode.ts";

/** What a pinnable row hands the text window: the window title, and the body
 *  it shows. Both come from the row as it is drawn, never from a re-run — the
 *  snapshot is what the user saw. */
export type PinTextSnapshot = {
  readonly title: string;
  readonly text: string;
};

/** The snapshot a row would pin, or `null` when the row has no single text
 *  body. `text` may be the empty string (an empty clipboard entry is still an
 *  entry); the row type is what decides, not the content's length. */
export const pinTextFor = (item: LauncherItem): PinTextSnapshot | null => {
  if (item.type === "history") return { title: item.title, text: item.commandLine };
  if (item.type === "clipboard") {
    const text = item.entry?.text;
    // `entry.text` is `string | null | undefined`: null is "this kind has no
    // text" (an image without a caption, a files entry), which is exactly the
    // rows the pin must not offer. An empty string is a real value and passes.
    return typeof text === "string" ? { title: item.title, text } : null;
  }
  if (item.type === "plugin") {
    // The row's own text is the only string it carries, so the same
    // `resolvePluginView` that decided it was a row decides whether it reads
    // as text here. A row with nothing to say resolves to `null` and earns no
    // pin.
    const view = resolvePluginView({ output: item.subtitle });
    if (view === null || view.form !== "text") return null;
    return { title: item.title, text: view.text };
  }
  return null;
};

/** Whether the row earns a pin button at all. The renderer's one question, and
 *  the one the matrix test answers row type by row type. */
export const pinTextApplies = (item: LauncherItem): boolean => pinTextFor(item) !== null;
