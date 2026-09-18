// GLASS-CLIP-2 · the clipboard list's *model*, extracted so the rewritten
// UI's behavior is testable.
//
// The page (`src/plugins/clipboard/main.ts`) runs in a sandboxed iframe and
// owns a real DOM, which `node --test` cannot instantiate (there is no DOM
// library in this repo's dependency set, by design — see the note repeated
// across the other plugin-page suites). Everything in the page that is a
// *decision* rather than a *DOM write* therefore lives here as a pure
// function, and the page is left with the plumbing: read state, call one of
// these, paint the answer.
//
// This is the same split `clipboard-history.ts` already uses for the
// derivation layer (type, age, URL host). The difference is that this module
// knows about the *page's* vocabulary — the five type tabs, the row action
// trio, the chip faces — and not about the storage record. That separation is
// deliberate: an entry's type is a fact about the record, while which glyph a
// row paints for it is a presentation choice, and a later round can change the
// glyph without touching the record's meaning.

import { fileExtensionBadge, isFilesPreviewCandidate, looksLikeDirectoryPath, clipboardEntryType, shouldActivateClipboardEntry, type ClipboardEntry, type ClipboardEntryType } from "./clipboard-history.ts";
import type { ClipboardIconName } from "./clipboard-icons.ts";

// ── The two filter axes ───────────────────────────────────────────────────

/** The scope tabs, in the order the bar shows them. `all` is the 0th stop. */
export const CLIPBOARD_VIEWS = ["all", "favorites"] as const;
export type ClipboardView = (typeof CLIPBOARD_VIEWS)[number];

/** The type-tab order, `null` (全部) first. Arrow keys and the 3–7 number row
 * both walk this one list, so the key map and the painted bar can never
 * disagree about what "next type" means. */
export const CLIPBOARD_TYPE_FILTER_ORDER: readonly (ClipboardEntryType | null)[] = [
  null,
  "text",
  "link",
  "color",
  "image",
  "files",
];

/** The i18n key for a type tab's label. One table, so a sixth type could not
 * be added to the enum without a label (the type enforces it). */
export const CLIPBOARD_TYPE_LABEL: Record<ClipboardEntryType | "all", string> = {
  all: "clipboard.typeAll",
  text: "clipboard.typeText",
  link: "clipboard.typeLink",
  color: "clipboard.typeColor",
  image: "clipboard.typeImage",
  files: "clipboard.typeFiles",
};

/** The Lucide glyph each type's chip and tab carry. Kept beside the label so
 * the two vocabularies are read as a pair. */
export const CLIPBOARD_TYPE_ICON: Record<ClipboardEntryType, ClipboardIconName> = {
  text: "type",
  link: "link",
  color: "palette",
  image: "image",
  files: "file",
};

/**
 * Move a selection index by `delta` with wrap-around, the list-nav rule every
 * command palette in the app uses. An empty list has no selection to move, so
 * it stays at 0 rather than going negative or `NaN`.
 */
export const moveClipboardSelection = (index: number, delta: number, length: number): number => {
  if (length <= 0) return 0;
  const base = Number.isFinite(index) ? Math.trunc(index) : 0;
  return ((base + delta) % length + length) % length;
};

/**
 * Step the type filter by `step` (an ArrowLeft/ArrowRight is ±1), wrapping
 * through the whole six-stop bar. `step` 0 is the identity, so a caller that
 * reads a non-arrow key cannot accidentally jump the filter.
 */
export const cycleClipboardTypeFilter = (
  current: ClipboardEntryType | null,
  step: number,
): ClipboardEntryType | null => {
  const length = CLIPBOARD_TYPE_FILTER_ORDER.length;
  const at = CLIPBOARD_TYPE_FILTER_ORDER.indexOf(current);
  const from = at < 0 ? 0 : at;
  return CLIPBOARD_TYPE_FILTER_ORDER[moveClipboardSelection(from, step, length)];
};

// ── The row's action trio ─────────────────────────────────────────────────

export type ClipboardRowAction = "copy" | "pin" | "delete";

/** The three actions, in the order the row paints them. */
export const CLIPBOARD_ROW_ACTIONS: readonly ClipboardRowAction[] = ["copy", "pin", "delete"];

/**
 * The bridge command each action runs, and the argument name its id rides in.
 *
 * Every value here is an **existing** command from the frozen allowlist in
 * `src-tauri/src/plugin_pages.rs` — the rewrite is presentation-only, so the
 * action trio reuses the same copy/pin/delete the row always used. Mapping it
 * as data (rather than three inline `invokeCommand("…")` calls) is what lets
 * the node suite assert that "pin" still persists through the *existing*
 * favorite command and that no action quietly grew a new write path.
 *
 * `pin` really is `clipboard_set_favorite`: the storage record has always
 * called this flag `favorite`, and GLASS-CLIP-2 only renamed it in the UI
 * (置顶 / Pin) because that is what a pin means to a user. The command name,
 * its argument and the stored field are untouched, so an older build reads
 * every record this one writes.
 */
export const CLIPBOARD_ROW_ACTION_COMMAND: Record<ClipboardRowAction, string> = {
  copy: "clipboard_copy_entry",
  pin: "clipboard_set_favorite",
  delete: "clipboard_delete",
};

/** The icon each of the row's actions carries by default (the copy action
 * swaps to `check` while its confirmation shows). */
export const CLIPBOARD_ROW_ACTION_ICON: Record<ClipboardRowAction, ClipboardIconName> = {
  copy: "copy",
  pin: "pin",
  delete: "trash",
};

// ── The type chip's five faces ────────────────────────────────────────────

/**
 * What a row's 32×32 type chip paints. A discriminated union rather than the
 * page's inline `if (type === …)` ladder, because the choice is a *model*
 * question — "this row is a thumbnail / a swatch / an extension badge / a
 * glyph" — and the page should only have to *draw* the answer.
 *
 * Every face carries its own `signature`: the identity the page compares to
 * skip an unchanged rebuild. Folding it into each variant (rather than an
 * intersection with a shared `{ signature }`) keeps the union narrowable — an
 * intersection would make `face.signature` unreadable on the un-narrowed
 * value, which is exactly the read the paint path does.
 */
export type ClipboardChipFace =
  | { kind: "thumbnail"; signature: string }
  | { kind: "swatch"; color: string; signature: string }
  | { kind: "badge"; text: string; signature: string }
  | { kind: "icon"; icon: ClipboardIconName; signature: string };

/**
 * Decide a row's chip face. `hasThumbnail` is passed in rather than read from
 * the page's blob registry so this stays pure: the caller knows whether the
 * bytes have arrived, this function only decides precedence.
 *
 * Precedence is the page's long-standing one: a real thumbnail beats a glyph
 * (it is strictly more information), a colour paints itself, a file shows its
 * extension, and everything else falls back to its type glyph. A directory or
 * an extension-less file gets the folder/file glyph instead of an empty badge.
 */
export const clipboardChipFace = (
  entry: ClipboardEntry,
  hasThumbnail: boolean,
): ClipboardChipFace => {
  const type = clipboardEntryType(entry);
  const wantsThumbnail =
    hasThumbnail
    && (entry.kind === "image" || isFilesPreviewCandidate(entry.paths));

  if (wantsThumbnail) {
    // The URL is not part of the *model* signature (it is a page-side blob
    // handle), so the face carries the intent and the page appends the handle
    // to its own comparison key. `thumbnail` is still its own signature so a
    // face that is *not* a thumbnail can never collide with one that is.
    return { kind: "thumbnail", signature: "thumbnail" };
  }

  if (type === "color") {
    const color = (entry.text ?? "").trim().toLowerCase();
    return { kind: "swatch", color, signature: `color:${color}` };
  }

  if (type === "files") {
    const first = entry.paths?.[0];
    const badge = fileExtensionBadge(first);
    if (badge) return { kind: "badge", text: badge, signature: `files:${badge}` };
    const icon: ClipboardIconName =
      first && looksLikeDirectoryPath(first) ? "folder" : "file";
    return { kind: "icon", icon, signature: `files:${icon}` };
  }

  const icon = CLIPBOARD_TYPE_ICON[type];
  return { kind: "icon", icon, signature: `icon:${icon}` };
};

// ── Applying the two filter axes ──────────────────────────────────────────

/**
 * The list after both of the page's filters: scope first (the cheaper cut, and
 * the one the tab counts are reported against), then the type, then the text
 * query. Extracted from the page so the *order* of the cuts — which decides
 * what each tab's count means — is pinned by a test rather than by reading
 * the page's source.
 *
 * `matchText` is injected because the text matcher lives in
 * `clipboard-history.ts` and taking it as a parameter keeps this module from
 * re-implementing it (and from importing the whole search layer for one call).
 */
export const applyClipboardFilters = (
  entries: readonly ClipboardEntry[],
  view: ClipboardView,
  type: ClipboardEntryType | null,
  filterText: string,
  matchText: (entries: ClipboardEntry[], query: string) => ClipboardEntry[],
): ClipboardEntry[] => {
  const scoped = view === "favorites" ? entries.filter((entry) => entry.favorite) : [...entries];
  const typed = type === null
    ? scoped
    : scoped.filter((entry) => clipboardEntryType(entry) === type);
  return matchText(typed, filterText);
};

// ── The keyboard model ────────────────────────────────────────────────────
//
// The page's whole key handling is a pure decision here, and `main.ts` only
// *executes* the answer. That split is what makes the rewritten keyboard
// surface testable: the node suite drives this function directly with a fake
// key + a fake state bag, and asserts the resulting action, without needing a
// DOM, a focus model or an event loop.
//
// It mirrors the page's precedence exactly, because the precedence is the
// design: the filter field is the page's keyboard home, so while it holds
// focus a printable key is text and `Backspace` is a character deletion — the
// row commands only become reachable once the user deliberately steps into the
// list. Getting that order wrong is the classic way a palette starts eating
// the query the user is typing.

/** Where the keyboard currently is, in the page's own three-way vocabulary:
 * the filter field, a row/row-action, or another control (a tab, the clear
 * button). `control` is the only one that swallows Enter, because a focused
 * button's Enter is its own activation. */
export type ClipboardKeyFocus = "search" | "row" | "control";

/** The input to [`resolveClipboardKey`]: the raw event facts the page cares
 * about, plus the page state the answer depends on. Deliberately structural
 * (not a DOM `KeyboardEvent`) so a test can build one literally. */
export type ClipboardKeyInput = {
  key: string;
  metaKey?: boolean;
  ctrlKey?: boolean;
  altKey?: boolean;
  repeat?: boolean;
  isComposing?: boolean;
  keyCode?: number;
  /** Which of the page's three focus worlds holds the keyboard. */
  focus: ClipboardKeyFocus;
  /** True when the focused control is the page's clear-history button, whose
   * Enter is the one control key the page preventDefaults (so no form submits).
   * A focused button's Enter/Space is otherwise left to activate the button. */
  clearFocused?: boolean;
  /** Whether the clear-history confirmation is armed (Escape disarms it
   * instead of closing the page while it is). */
  clearArmed: boolean;
};

/**
 * What one keypress means. A discriminated union rather than a side effect, so
 * the caller keeps the DOM and the state writes and this stays a table.
 *
 * `preventDefault` is part of the answer (not assumed) because the page
 * *does* let a few keys through untouched — platform copy/paste while typing,
 * and any unhandled key. The page honors the flag and nothing else.
 */
export type ClipboardKeyAction =
  | { kind: "ignore"; preventDefault: boolean }
  | { kind: "close" }
  | { kind: "disarm-clear" }
  | { kind: "toggle-view" }
  | { kind: "cycle-type"; step: number }
  | { kind: "set-type"; type: ClipboardEntryType | null }
  | { kind: "set-view"; view: ClipboardView }
  | { kind: "move"; delta: number }
  | { kind: "activate" }
  | { kind: "toggle-pin" }
  | { kind: "delete" }
  | { kind: "append-char"; char: string };

/** Keys whose *repeat* is suppressed in the list: holding Enter must not copy
 * the same row over and over, and holding a single-key command must not repeat
 * a write. */
const REPEAT_SUPPRESSED = ["Enter", "Delete", "Backspace", "d", "D", "f", "F", "p", "P", "*"];

/** The pin/favorite keys. `p` is the rewritten UI's advertised key; `f` and
 * `*` are the pre-GLASS-CLIP-2 spellings and stay accepted so an old habit is
 * not punished. All three mean the same action. */
const PIN_KEYS = ["f", "F", "*", "p", "P"];

/**
 * Resolve one keypress against the page's state. The branch order is the
 * page's precedence, and it is load-bearing: see the module note above.
 */
export const resolveClipboardKey = (input: ClipboardKeyInput): ClipboardKeyAction => {
  const {
    key,
    metaKey = false,
    ctrlKey = false,
    altKey = false,
    repeat = false,
    isComposing = false,
    keyCode = 0,
    focus,
    clearFocused = false,
    clearArmed,
  } = input;
  const modifier = metaKey || ctrlKey;

  // 1 · An IME composition owns the key entirely — never a command.
  if (isComposing || keyCode === 229) return { kind: "ignore", preventDefault: false };

  // 2 · Escape first disarms an armed clear, which is the more local intent.
  if (key === "Escape" && clearArmed) return { kind: "disarm-clear" };

  // 3 · Suppress a *held* action key. While the filter holds focus the guard
  //     only bites for the modifier/Enter forms, so holding a plain letter
  //     keeps typing normally.
  if (
    repeat
    && REPEAT_SUPPRESSED.includes(key)
    && (focus !== "search" || modifier || key === "Enter")
  ) {
    return { kind: "ignore", preventDefault: true };
  }

  // 4 · A focused button's Enter/Space is its own activation; the page must
  //     not also run a row command. The clear button's Enter is the one case
  //     the page claims (a raw form submit would otherwise reload the frame).
  if (focus === "control" && (key === "Enter" || key === " ")) {
    return { kind: "ignore", preventDefault: key === "Enter" && clearFocused };
  }

  // 5 · Cmd/Ctrl+W dismisses the page, the app-wide overlay convention.
  if (modifier && key.toLowerCase() === "w") return { kind: "close" };

  // 6 · Cmd/Ctrl+Backspace deletes the selected row from ANY focus — the
  //     collision-free escape hatch that survives even while typing.
  if (modifier && key === "Backspace") return { kind: "delete" };

  if (key === "Escape") return { kind: "close" };

  // 7 · Tab toggles the scope (全部/收藏) rather than walking focus away.
  if (key === "Tab") return { kind: "toggle-view" };

  // 8 · ←/→ walk the *type* bar — but inside the filter they are ordinary
  //     caret moves, so the page stays out of the way there.
  if (key === "ArrowLeft" || key === "ArrowRight") {
    if (focus === "search") return { kind: "ignore", preventDefault: false };
    return { kind: "cycle-type", step: key === "ArrowRight" ? 1 : -1 };
  }

  if (key === "ArrowDown") return { kind: "move", delta: 1 };
  if (key === "ArrowUp") return { kind: "move", delta: -1 };

  // 9 · Enter (the fast copy path) — never for an IME confirmation, a repeat,
  //     or a focused control (handled above). This is the action the
  //     rewritten keyboard surface advertises with "Enter 复制".
  if (shouldActivateClipboardEntry({ key, isComposing, keyCode, repeat }, focus === "search" ? "search" : "row")) {
    return { kind: "activate" };
  }

  // 10 · From here the filter owns the keyboard: printable keys and Backspace
  //      are text editing and must pass through untouched.
  if (focus === "search") return { kind: "ignore", preventDefault: false };

  // 11 · Platform copy/paste/cut keep working in the list too.
  if (modifier) return { kind: "ignore", preventDefault: false };
  // 12 · Alt+letter belongs to a global shortcut; never a row command.
  if (altKey && key.length === 1) return { kind: "ignore", preventDefault: false };

  // 13 · Single-key row commands, list-focus only.
  if (PIN_KEYS.includes(key)) return { kind: "toggle-pin" };
  if (key === "d" || key === "D" || key === "Delete") return { kind: "delete" };
  if (key === "1") return { kind: "set-view", view: "all" };
  if (key === "2") return { kind: "set-view", view: "favorites" };
  // 3–7 are the five type tabs, in the order the bar shows them.
  if (key >= "3" && key <= "7") return { kind: "set-type", type: CLIPBOARD_TYPE_FILTER_ORDER[Number(key) - 2] ?? null };
  // ⌫ on the list deletes the selected row — what the row's ⌫ hint promises.
  // (It used to route a character deletion into the filter; that is gone.)
  if (key === "Backspace") return { kind: "delete" };
  // Any other printable character lands in the filter.
  if (key.length === 1) return { kind: "append-char", char: key };
  return { kind: "ignore", preventDefault: false };
};

/** How many entries each type tab should show, computed from the *scoped* set
 * so the counts agree with what the tab would actually reveal. */
export const clipboardTypeCounts = (
  entries: readonly ClipboardEntry[],
  view: ClipboardView,
): Record<ClipboardEntryType, number> => {
  const counts: Record<ClipboardEntryType, number> = { text: 0, link: 0, color: 0, image: 0, files: 0 };
  for (const entry of entries) {
    if (view === "favorites" && !entry.favorite) continue;
    counts[clipboardEntryType(entry)] += 1;
  }
  return counts;
};
