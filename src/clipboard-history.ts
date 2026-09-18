// Pure logic behind the built-in clipboard history panel: entry normalization,
// filtering, previews and compact ages. Kept free of React and IPC so the
// node test suite can exercise it directly (see tests/clipboard-history.test.ts).

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
   * which reads pixels through `clipboard_read_image` by id instead. */
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
 * Case-insensitive substring filter over the panel's search line. Text entries
 * match on their content; files entries on any stored path (a basename is a
 * substring of its own path); image entries match a stored caption (an
 * image+text copy stores one image entry whose `text` is the caption) and a
 * bare image — nothing to say about it but its name — still answers to that
 * name in either UI language. Filtering only decides which rows are kept; an
 * image entry never turns into text because of its caption.
 */
export const filterClipboardEntries = (
  entries: ClipboardEntry[],
  query: string,
): ClipboardEntry[] => {
  const needle = query.trim().toLowerCase();
  if (!needle) return entries.slice();
  return entries.filter((entry) => {
    if (entry.kind === "files") {
      return (entry.paths ?? []).some((path) => path.toLowerCase().includes(needle));
    }
    // Text content, or an image's caption.
    if ((entry.text ?? "").toLowerCase().includes(needle)) return true;
    return entry.kind === "image"
      ? ["image", "img", "图片"].some((word) => word.includes(needle))
      : false;
  });
};

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

/**
 * The host a link points at, for the row's secondary line: `example.com`.
 * `www.` is dropped (it is noise, not identity) and a malformed value returns
 * the empty string rather than a half-parsed host. A scheme-less `host/path`
 * is prefixed with a throwaway scheme before parsing, because `URL` refuses to
 * read a bare host as a URL at all.
 */
export const urlHost = (value: string | null | undefined): string => {
  const text = (value ?? "").trim();
  if (!text) return "";
  try {
    const parsed = new URL(/^[a-z][a-z0-9+.-]*:/i.test(text) ? text : `https://${text}`);
    return parsed.hostname.replace(/^www\./i, "");
  } catch {
    return "";
  }
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

/**
 * Whether an entry belongs to one type tab. `null` (the 全部 tab) keeps
everything. Files entries are matched on their stored kind, so a file list
 * that happens to contain a URL is still a file list.
 */
export const clipboardEntryMatchesType = (
  entry: ClipboardEntry,
  type: ClipboardEntryType | null,
): boolean => type === null || clipboardEntryType(entry) === type;

/**
 * Coerce a stored/typed value into a type filter, or `null` for 全部.
 *
 * Deliberately a *separate* normalizer from [`normalizeClipboardSession`]: that
 * one's shape is pinned by `tests/clipboard-history.test.ts` (it returns exactly
 * the four fields it always did), and the type filter is a new, independent
 * axis. Keeping them apart means adding the filter cannot silently change the
 * meaning of a stored session from an older build, and an unknown value degrades
 * to 全部 rather than to a tab that shows nothing.
 */
export const normalizeClipboardTypeFilter = (value: unknown): ClipboardEntryType | null =>
  typeof value === "string" && (CLIPBOARD_ENTRY_TYPES as readonly string[]).includes(value)
    ? (value as ClipboardEntryType)
    : null;

/**
 * The uppercase extension badge a file row shows (`PNG`, `PDF`). Taken from the
 * last dot of the basename, capped at 5 characters so a pathological name does
 * not blow up the badge; an extension-less name (a directory, usually) returns
 * the empty string and the row falls back to a folder glyph instead.
 */
export const fileExtensionBadge = (path: string | null | undefined): string => {
  if (!path) return "";
  const name = splitFilePath(path).basename;
  const dot = name.lastIndexOf(".");
  if (dot <= 0 || dot + 1 >= name.length) return "";
  const extension = name.slice(dot + 1);
  return extension.length <= 5 ? extension.toUpperCase() : "";
};

// ── GLASS-CLIP-2 · the relative age ───────────────────────────────────────

export type ClipboardAgeUnit = "now" | "minute" | "hour" | "day" | "date";

export type ClipboardAge = {
  unit: ClipboardAgeUnit;
  /** Count for `minute`/`hour`/`day`; 0 for `now` and `date`. */
  value: number;
};

/**
 * The row's age as a *unit and a count*, not a formatted string.
 *
 * [`formatClipboardAge`] above stays exactly as it is — it is the compact
 * terminal vocabulary (`42m`) the row paint key and the existing tests use.
 * The rewritten list wants a human sentence instead (「3 分钟前」/ "3 minutes
 * ago"), and a sentence has to be assembled by the translator, not by a string
 * builder in a pure module. So this returns the structured fact and the page
 * picks the wording: one derivation, both renderings, and the pluralization
 * rules stay in `i18n.ts` where every other sentence lives.
 */
export const clipboardAge = (createdAtMs: number, nowMs: number): ClipboardAge => {
  if (!Number.isFinite(createdAtMs) || !Number.isFinite(nowMs)) return { unit: "date", value: 0 };
  const seconds = Math.max(0, Math.floor((nowMs - createdAtMs) / 1000));
  if (seconds < 60) return { unit: "now", value: 0 };
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return { unit: "minute", value: minutes };
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return { unit: "hour", value: hours };
  const days = Math.floor(hours / 24);
  // Beyond a week the exact count stops being useful and the date starts
  // being; the crossover matches the point where "8 天前" reads worse than a
  // concrete timestamp.
  if (days <= 7) return { unit: "day", value: days };
  return { unit: "date", value: 0 };
};
