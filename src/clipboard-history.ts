// Pure logic behind the built-in clipboard history panel: entry normalization,
// filtering, previews and compact ages. Kept free of React and IPC so the
// node test suite can exercise it directly (see tests/clipboard-history.test.ts).

import { matchesTokens, searchTokens } from "./plugins/search.ts";

export type ClipboardEntry = {
  id: string;
  /** "text" | "image" | "files" */
  kind: string;
  /** Text content; on an image entry, the caption stored when the copy rode
   * along with non-whitespace text (preview line + filter target). */
  text?: string | null;
  /** Absolute paths referenced by a "files" entry; contents are never stored
   * or shipped over IPC, only the path strings themselves. */
  paths?: string[] | null;
  /** File name inside the backend's history store; opaque to the frontend,
   * which reads a scaled thumbnail through `clipboard_thumbnail` by id. */
  image_file?: string | null;
  width?: number | null;
  height?: number | null;
  hash: string;
  created_at: number;
  favorite: boolean;
};

/** Coerce whatever arrives over IPC into trusted shapes, dropping malformed rows. */
export const normalizeEntries = (rows: unknown): ClipboardEntry[] => {
  if (!Array.isArray(rows)) return [];
  const entries: ClipboardEntry[] = [];
  for (const row of rows) {
    if (typeof row !== "object" || row === null) continue;
    const raw = row as Record<string, unknown>;
    const kind =
      raw.kind === "image"
        ? "image"
        : raw.kind === "text"
          ? "text"
          : raw.kind === "files"
            ? "files"
            : null;
    const id = typeof raw.id === "string" ? raw.id : null;
    const hash = typeof raw.hash === "string" ? raw.hash : null;
    if (!kind || !id || !hash) continue;
    entries.push({
      id,
      kind,
      text: typeof raw.text === "string" ? raw.text : null,
      paths: Array.isArray(raw.paths)
        ? raw.paths.filter((path): path is string => typeof path === "string")
        : null,
      image_file: typeof raw.image_file === "string" ? raw.image_file : null,
      width: typeof raw.width === "number" ? raw.width : null,
      height: typeof raw.height === "number" ? raw.height : null,
      hash,
      created_at: typeof raw.created_at === "number" ? raw.created_at : 0,
      favorite: raw.favorite === true,
    });
  }
  return entries;
};

/**
 * The strings one clipboard entry can be searched through. Text entries match on
 * their content; files entries on any stored path (a basename is a substring of
 * its own path); image entries match a stored caption (an image+text copy stores
 * one image entry whose `text` is the caption) and a bare image — nothing to say
 * about it but its name — still answers to that name in either UI language.
 * Filtering only decides which rows are kept; an image entry never turns into
 * text because of its caption.
 */
export const clipboardEntrySearchFields = (entry: ClipboardEntry): string[] => {
  const fields: string[] = [];
  if (entry.kind === "files") {
    fields.push(...(entry.paths ?? []));
  } else if (entry.text) {
    fields.push(entry.text);
  }
  if (entry.kind === "image") fields.push("image", "img", "图片");
  return fields;
};

/**
 * R32 · case-insensitive AND filter over the panel's search line.
 *
 * The line is split on whitespace and **every** token must be found in one of
 * the entry's own fields (see {@link clipboardEntrySearchFields}); `hello world`
 * now finds an entry that contains both words rather than only the literal
 * phrase. An empty line is no filter at all. The rule itself lives in
 * `plugins/search.ts`, so the launcher's clipboard mode and this panel filter
 * identically.
 */
export const filterClipboardEntries = (
  entries: ClipboardEntry[],
  query: string,
): ClipboardEntry[] => {
  const tokens = searchTokens(query);
  if (!tokens.length) return entries.slice();
  return entries.filter((entry) =>
    matchesTokens(tokens, clipboardEntrySearchFields(entry)),
  );
};

/**
 * R89 · whether the launcher's clipboard mode should ask the backend to search
 * the **full** history for this query.
 *
 * The list IPC carries only a prefix of each text entry
 * (`LIST_TEXT_PREFIX_BYTES` in Rust), so the in-memory rule above can miss a
 * match that lives past an entry's prefix. When the query is non-empty and that
 * in-memory filter found nothing, the backend is asked to run the same token
 * rule over the untruncated text; its answer stands in for the empty result.
 *
 * The zero-result gate is the whole point: a query the memory filter already
 * answered costs no IPC, so typing inside the mode stays local (and the list
 * stays on the in-memory path that has no round-trip latency). The known
 * bounded gap this leaves is registered in the R89 report: a query with *some*
 * local hits never reaches a second entry whose only match is past its prefix.
 */
export const shouldSearchFullText = (entries: ClipboardEntry[], query: string): boolean =>
  searchTokens(query).length > 0 && filterClipboardEntries(entries, query).length === 0;

const IMAGE_PREVIEW_MAX = 60;

/** Split a stored path into directory prefix and final segment, aware of both
 * POSIX and Windows separators (and trailing separators). */
export const splitFilePath = (path: string): { basename: string; dirname: string } => {
  const trimmed = path.replace(/[\\/]+$/, "");
  const index = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"));
  if (index < 0) return { basename: trimmed || path, dirname: "" };
  return {
    basename: trimmed.slice(index + 1) || trimmed,
    dirname: trimmed.slice(0, index),
  };
};

const IMAGE_FILE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "webp", "bmp"]);

/** Whether a stored path names a raster-image file the webview can render,
 * judged by extension alone and shared with the backend's preview gating. */
export const isImageFilePath = (path: string): boolean => {
  const name = splitFilePath(path).basename.toLowerCase();
  const dot = name.lastIndexOf(".");
  if (dot <= 0 || dot + 1 >= name.length) return false;
  return IMAGE_FILE_EXTENSIONS.has(name.slice(dot + 1));
};

/** Whether a files entry carries exactly one image-extension path and so can
 * show real pixels in its marker slot. Mirrors the backend's eligibility
 * check; size is gated at read time over there. */
export const isFilesPreviewCandidate = (paths: string[] | null | undefined): boolean =>
  paths?.length === 1 && isImageFilePath(paths[0]);

/** MIME type for an image-extension path, for typing preview blobs. */
export const imageFileMime = (path: string): string => {
  const name = splitFilePath(path).basename.toLowerCase();
  const extension = name.slice(name.lastIndexOf(".") + 1);
  switch (extension) {
    case "png":
      return "image/png";
    case "jpg":
    case "jpeg":
      return "image/jpeg";
    case "gif":
      return "image/gif";
    case "webp":
      return "image/webp";
    case "bmp":
      return "image/bmp";
    default:
      return "application/octet-stream";
  }
};

/** Best-effort guess whether a stored path names a directory: trailing
 * separators say yes, an extension-less final segment probably. Only drives
 * which marker glyph a row gets — never correctness. */
export const looksLikeDirectoryPath = (path: string): boolean => {
  if (/[\\/]$/.test(path)) return true;
  return !splitFilePath(path).basename.includes(".");
};

export type FilesPreview = {
  /** Final segment of the first path, shown in the primary color. */
  basename: string;
  /** Everything before it on the first path, separator included, muted;
   * empty for bare names. */
  dirname: string;
  /** How many items beyond the first the entry holds (N>1 → "+N"). */
  extra: number;
};

/** Row preview data for a files entry: first basename prominent, its
 * directory muted, remaining item count as a suffix. Language-neutral by
 * design — both UI languages render the same shape. */
export const formatFilesPreview = (paths: string[] | null | undefined): FilesPreview => {
  const list = paths ?? [];
  if (list.length === 0) return { basename: "", dirname: "", extra: 0 };
  const first = list[0];
  const { basename } = splitFilePath(first);
  // Slice the real prefix off the original string so Windows-style paths keep
  // their own separators.
  const cut = first.length >= basename.length ? first.length - basename.length : 0;
  return {
    basename,
    dirname: basename ? first.slice(0, cut) : "",
    extra: Math.max(0, list.length - 1),
  };
};

/** POSIX single-quote escaping for pasting stored paths into the embedded
 * shell: wrap in '…', splicing embedded quotes as '\''. */
export const shellQuotePath = (path: string): string => `'${path.split("'").join(`'\\''`)}'`;

/**
 * The one-line preview shown on each row: the first line of a text entry,
 * trimmed and capped. An image entry shows its caption's first line when a
 * simultaneous text copy rode along with the pixels, and falls back to an
 * `[image WxH]` label when there is none.
 */
export const clipboardPreview = (entry: ClipboardEntry, maxLength = 120): string => {
  // Captions are stored trimmed, so the first line is the real one; skipping
  // blank lines here costs nothing and covers anything older that was not.
  const text = entry.text ?? "";
  const source = entry.kind === "text" ? text : text.trimStart();
  const end = source.indexOf("\n");
  const firstLine = source.slice(0, end < 0 ? source.length : end).trim();
  if (entry.kind !== "text") {
    if (!firstLine) {
      const width = Number.isFinite(entry.width) ? entry.width : "?";
      const height = Number.isFinite(entry.height) ? entry.height : "?";
      return `[image ${width}x${height}]`;
    }
    return firstLine.length > IMAGE_PREVIEW_MAX
      ? `${firstLine.slice(0, IMAGE_PREVIEW_MAX)}…`
      : firstLine;
  }
  const cap = Math.max(1, Math.min(maxLength, IMAGE_PREVIEW_MAX * 2));
  return firstLine.length > cap ? `${firstLine.slice(0, cap)}…` : firstLine;
};

/** Content is immutable for an id/hash. Avoid serializing payloads on every poll. */
export const sameClipboardSnapshot = (left: ClipboardEntry[], right: ClipboardEntry[]): boolean =>
  left.length === right.length && left.every((entry, index) => {
    const next = right[index];
    return entry.id === next.id && entry.hash === next.hash && entry.kind === next.kind
      && entry.created_at === next.created_at && entry.favorite === next.favorite;
  });

/**
 * R93 · what an optimistic clipboard delete leaves behind when its write is
 * refused.
 *
 * The old rule put a whole *snapshot* of the list back, which resurrected the
 * entire table if the mode had been left in the meantime (the mode's exit
 * effect clears the list). The corrected rule is deliberately narrow:
 *
 *   * `active` is read at failure time, not at call time — a delete that fails
 *     after the user left the mode restores nothing, because there is no list
 *     to restore into;
 *   * only the one deleted entry comes back, at the index it left, so a list
 *     that changed while the write was in flight keeps those changes;
 *   * an entry already present (a reload landed it) is left alone — the list
 *     is the authority and must not gain a duplicate.
 *
 * Pure and React-free so the node suite drives the exact table the hook uses;
 * the hook only supplies `active`, the removed entry and its index.
 */
export const restoreDeletedEntry = (
  entries: ClipboardEntry[],
  failure: { active: boolean; removed: ClipboardEntry | null; index: number },
): ClipboardEntry[] => {
  const { active, removed, index } = failure;
  if (!active || removed === null) return entries;
  if (entries.some((entry) => entry.id === removed.id)) return entries;
  const next = entries.slice();
  next.splice(Math.min(Math.max(index, 0), next.length), 0, removed);
  return next;
};

export type ClipboardSession = {
  filterText: string;
  view: "all" | "favorites";
  selectedId: string | null;
  scrollTop: number;
};

export function normalizeClipboardSession(raw: unknown): ClipboardSession {
  const value = raw && typeof raw === "object" ? raw as Record<string, unknown> : {};
  return {
    filterText: typeof value.filterText === "string" ? value.filterText.slice(0, 512) : "",
    view: value.view === "favorites" ? "favorites" : "all",
    selectedId: typeof value.selectedId === "string" ? value.selectedId : null,
    scrollTop: typeof value.scrollTop === "number" && Number.isFinite(value.scrollTop) ? Math.max(0, value.scrollTop) : 0,
  };
}

export type ClipboardKeyContext = "search" | "row" | "control";

/** Keys that belong to an IME or a focused button must not paste a row. */
export function shouldActivateClipboardEntry(event: { key: string; isComposing?: boolean; keyCode?: number; repeat?: boolean }, context: ClipboardKeyContext): boolean {
  return event.key === "Enter" && !event.isComposing && event.keyCode !== 229 && !event.repeat && context !== "control";
}

/**
 * Compact relative age in the terminal's own vocabulary: `42m`, `5h`, and for
 * anything older a concrete local datetime via [`formatClipboardDateTime`].
 * Seconds never appear — they churn too fast to read.
 */
export const formatClipboardAge = (createdAtMs: number, nowMs: number): string => {
  if (!Number.isFinite(createdAtMs) || !Number.isFinite(nowMs)) return "";
  const seconds = Math.max(0, Math.floor((nowMs - createdAtMs) / 1000));
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return formatClipboardDateTime(createdAtMs);
};

/**
 * Concrete local datetime, `YYYY-MM-DD HH:mm`, zero-padded. Local time only —
 * no locale formatting, so both UI languages render the identical shape.
 */
export const formatClipboardDateTime = (ms: number): string => {
  if (!Number.isFinite(ms)) return "";
  const date = new Date(ms);
  const pad = (value: number): string => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} `
    + `${pad(date.getHours())}:${pad(date.getMinutes())}`;
};

// ── GLASS-CLIP-2 · the row's *type* ───────────────────────────────────────
//
// The panel used to have exactly one visual register for five different kinds
// of capture: text, a URL, a colour, an image, a file. The type is derived here,
// not in the page, so the five-way split is a pure function the node suite can
// exercise directly (the page owns a DOM and imports CSS, so it cannot be
// instantiated under `node --test`).
//
// The derivation is deliberately *content-first* for text entries: a text entry
// whose content is a URL is a link, and one whose content is a colour literal
// is a colour — the stored `kind` stays `text` (the backend never lies about
// what it captured), and only the *presentation* is refined. That keeps the
// refinement reversible: a later round can drop this function and nothing in
// the storage layer or the bridge changes.

export type ClipboardEntryType = "text" | "link" | "color" | "image" | "files";

/** In display order: the type tabs the page's filter bar shows. */
export const CLIPBOARD_ENTRY_TYPES: readonly ClipboardEntryType[] = [
  "text",
  "link",
  "color",
  "image",
  "files",
] as const;

/** A URL with an explicit scheme. Anchored and scheme-restricted so a sentence
 * containing "foo://" mid-word is not mistaken for a link. */
const SCHEME_URL = /^(?:https?|ftp|file|ssh|git|mailto):\/\/\S+$/i;
/** A bare `www.` host — the one scheme-less form people actually copy. */
const BARE_WWW = /^www\.[^\s/]+\.[^\s]+$/i;
/** A host-and-path with no scheme and no spaces, e.g. `github.com/foo/bar`.
 * Requires a real-looking TLD of 2+ letters and at least one dot. */
const BARE_HOST = /^[a-z0-9][a-z0-9-]*(?:\.[a-z0-9-]+)*\.[a-z]{2,}(?:[/:?#]\S*)?$/i;

/**
 * Whether a whole string is a single URL. Whitespace anywhere disqualifies it —
 * a multi-line paragraph that happens to start with a link is prose, not a
 * link, and must keep its text row.
 */
export const looksLikeUrl = (value: string | null | undefined): boolean => {
  const text = (value ?? "").trim();
  if (!text || /\s/.test(text)) return false;
  return SCHEME_URL.test(text) || BARE_WWW.test(text) || BARE_HOST.test(text);
};

/** A 3/4/6/8-digit hex colour literal. */
const HEX_COLOR = /^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;
/** `rgb()` / `rgba()` / `hsl()` / `hsla()` with only numbers, commas, spaces,
 * percent signs, dots and slashes inside — no `var()`, no `calc()`. */
const FUNCTIONAL_COLOR = /^(?:rgba?|hsla?)\(\s*[-\d.,%\s/]+\s*\)$/i;

/**
 * Whether a whole string is a single colour literal the page can paint as a
 * swatch. Only self-contained forms are accepted: a hex or a literal
 * `rgb()`/`hsl()`. A named colour (`red`) is deliberately *not* accepted — it
 * is far more likely to be an ordinary copied word than a colour someone meant
 * to reuse, and painting a swatch for it would be a confident lie.
 */
export const looksLikeColor = (value: string | null | undefined): boolean => {
  const text = (value ?? "").trim();
  if (!text) return false;
  // A hex literal is one token — any whitespace means it is not one.
  if (HEX_COLOR.test(text)) return true;
  // A functional form legitimately contains spaces (`rgb(1, 2, 3)`), so it is
  // matched by its own anchored pattern rather than by a whitespace guard.
  return FUNCTIONAL_COLOR.test(text);
};

/**
 * The five-way presentation type of an entry. Images and file lists keep their
 * stored kind; a text entry is refined into a link or a colour when its whole
 * content is one. Precedence matters only for the impossible overlap (a hex
 * colour is never a URL, so the order is not load-bearing) and is written as
 * link → colour → text so the more specific claim wins.
 */
export const clipboardEntryType = (entry: ClipboardEntry): ClipboardEntryType => {
  if (entry.kind === "image") return "image";
  if (entry.kind === "files") return "files";
  const text = entry.text ?? "";
  if (looksLikeUrl(text)) return "link";
  if (looksLikeColor(text)) return "color";
  return "text";
};

// ── R27 · the plugin's own settings ───────────────────────────────────────
//
// The clipboard plugin grew a settings card of its own (the browser plugin has
// had one since R26-B), and the one control on it is the capacity: how many
// non-favorite entries the history keeps. The shape and the clamp live here
// rather than in the page for the same reason the entry normalizers do — the
// page owns a DOM and imports CSS, so a node test can only pin a decision that
// lives outside it.

/** The plugin's settings block, mirroring `ClipboardPluginSettings` in Rust. */
export type ClipboardPluginSettings = {
  /** How many non-favorite entries the history keeps. Favorites are exempt. */
  max_items: number;
};

/** The shipped capacity. Mirrors `DEFAULT_CLIPBOARD_MAX_ITEMS` in Rust, and the
 *  constant the store itself shipped with before it was configurable. */
export const DEFAULT_CLIPBOARD_MAX_ITEMS = 300;
/** The floor: below this the control stops being a capacity and starts being an
 *  off switch (that is `clipboard_history_enabled`). */
export const MIN_CLIPBOARD_MAX_ITEMS = 10;
/** The ceiling: high enough to be a preference, low enough that a hand-edited
 *  file cannot ask the monitor to hold an unbounded index. */
export const MAX_CLIPBOARD_MAX_ITEMS = 500;

/** Clamp a capacity to the range the backend honours. A non-finite value — an
 *  empty input box, `NaN` from a bad parse — is the shipped default, not zero:
 *  a typo must not read as "keep nothing". */
export const clampClipboardMaxItems = (value: number): number => {
  if (!Number.isFinite(value)) return DEFAULT_CLIPBOARD_MAX_ITEMS;
  return Math.min(MAX_CLIPBOARD_MAX_ITEMS, Math.max(MIN_CLIPBOARD_MAX_ITEMS, Math.trunc(value)));
};

/** Coerce whatever the bridge returned into the trusted shape. The backend's
 *  `normalize_settings` is the authority; this is the same rule on the page
 *  side, so the card shows the value that was actually stored. */
export const normalizeClipboardSettings = (value: unknown): ClipboardPluginSettings => {
  const record =
    typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
  const raw = typeof record.max_items === "number" ? record.max_items : DEFAULT_CLIPBOARD_MAX_ITEMS;
  return { max_items: clampClipboardMaxItems(raw) };
};
