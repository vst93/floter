// GLASS-CLIP-2 · the clipboard page's inline icon set.
//
// The page runs in a sandboxed iframe served as its own document, so it cannot
// render the React `lucide-react` components the host uses — those are React
// elements, and this document has no React. What it *can* do is carry the same
// geometry: the paths below are the icon nodes from `lucide-react@1.31.0`,
// copied verbatim (same `d`/`cx`/`r`/`width` attributes), rendered as inline
// `<svg>` through `document.createElementNS`.
//
// Copied rather than re-drawn on purpose: a hand-rolled glyph drifts from the
// host's icon language within a round or two, and the whole point of the type
// chips is that they read as the same family as the toolbar and the settings
// rows. `tests/clipboard-ui.test.ts` asserts the path data here still matches
// the installed `lucide-react` package, so an upgrade that moves an icon cannot
// silently leave this file behind.
//
// All icons are the flat linear 24×24 grid with `stroke-width: 2`, `round`
// caps/joins and `currentColor` stroke — i.e. the unmodified Lucide defaults.
// Colour is never baked into a glyph; the caller's `color` decides, so the
// icons stay neutral furniture and the accent budget stays with the controls.

/** One Lucide icon node: `[tag, attributes]`. */
type IconNode = [string, Record<string, string | number>];

export type ClipboardIconName =
  | "type"
  | "link"
  | "palette"
  | "image"
  | "file"
  | "folder"
  | "copy"
  | "pin"
  | "trash"
  | "search"
  | "close"
  | "check";

/** The Lucide nodes, keyed by the name the page asks for. */
export const CLIPBOARD_ICON_NODES: Record<ClipboardIconName, readonly IconNode[]> = {
  // lucide `type` — a text entry's chip.
  type: [
    ["path", { d: "M12 4v16" }],
    ["path", { d: "M4 7V5a1 1 0 0 1 1-1h14a1 1 0 0 1 1 1v2" }],
    ["path", { d: "M9 20h6" }],
  ],
  // lucide `link` — a URL entry's chip.
  link: [
    ["path", { d: "M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" }],
    ["path", { d: "M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" }],
  ],
  // lucide `palette` — a colour entry's chip.
  palette: [
    ["path", { d: "M12 22a1 1 0 0 1 0-20 10 9 0 0 1 10 9 5 5 0 0 1-5 5h-2.25a1.75 1.75 0 0 0-1.4 2.8l.3.4a1.75 1.75 0 0 1-1.4 2.8z" }],
    ["circle", { cx: "13.5", cy: "6.5", r: ".5", fill: "currentColor" }],
    ["circle", { cx: "17.5", cy: "10.5", r: ".5", fill: "currentColor" }],
    ["circle", { cx: "6.5", cy: "12.5", r: ".5", fill: "currentColor" }],
    ["circle", { cx: "8.5", cy: "7.5", r: ".5", fill: "currentColor" }],
  ],
  // lucide `image` — an image entry's chip.
  image: [
    ["rect", { width: "18", height: "18", x: "3", y: "3", rx: "2", ry: "2" }],
    ["circle", { cx: "9", cy: "9", r: "2" }],
    ["path", { d: "m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21" }],
  ],
  // lucide `file` — a file entry's chip.
  file: [
    ["path", { d: "M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z" }],
    ["path", { d: "M14 2v5a1 1 0 0 0 1 1h5" }],
  ],
  // lucide `folder` — a directory entry's chip.
  folder: [
    ["path", { d: "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" }],
  ],
  // lucide `copy` — the row's primary inline action.
  copy: [
    ["rect", { width: "14", height: "14", x: "8", y: "8", rx: "2", ry: "2" }],
    ["path", { d: "M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" }],
  ],
  // lucide `pin` — the row's pin action.
  pin: [
    ["path", { d: "M12 17v5" }],
    ["path", { d: "M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z" }],
  ],
  // lucide `trash-2` — the row's delete action.
  trash: [
    ["path", { d: "M10 11v6" }],
    ["path", { d: "M14 11v6" }],
    ["path", { d: "M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" }],
    ["path", { d: "M3 6h18" }],
    ["path", { d: "M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" }],
  ],
  // lucide `search` — the filter field's leading glyph.
  search: [
    ["path", { d: "m21 21-4.34-4.34" }],
    ["circle", { cx: "11", cy: "11", r: "8" }],
  ],
  // lucide `x` — the filter clear control.
  close: [
    ["path", { d: "M18 6 6 18" }],
    ["path", { d: "m6 6 12 12" }],
  ],
  // lucide `check` — the copied-confirmation mark on a row.
  check: [["path", { d: "M20 6 9 17l-5-5" }]],
};

/**
 * Build one inline icon as an `<svg>` element on the given document.
 *
 * `aria-hidden` is unconditional: every icon in this page is decorative
 * furniture beside a text label or an `aria-label`ed control, and an icon that
 * announced itself would double every row's accessible name. Callers that need
 * the icon to *be* the name put the label on the button, not the glyph.
 */
export const clipboardIcon = (
  document: Document,
  name: ClipboardIconName,
  size = 14,
): SVGSVGElement => {
  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("width", String(size));
  svg.setAttribute("height", String(size));
  svg.setAttribute("fill", "none");
  svg.setAttribute("stroke", "currentColor");
  svg.setAttribute("stroke-width", "2");
  svg.setAttribute("stroke-linecap", "round");
  svg.setAttribute("stroke-linejoin", "round");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  svg.classList.add("clipboard-icon");
  for (const [tag, attributes] of CLIPBOARD_ICON_NODES[name]) {
    const node = document.createElementNS(NS, tag);
    for (const [attribute, value] of Object.entries(attributes)) {
      node.setAttribute(attribute, String(value));
    }
    svg.append(node);
  }
  return svg;
};
