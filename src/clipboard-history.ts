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
  const firstLine =
    entry.kind === "text"
      ? (entry.text ?? "").split("\n")[0].trim()
      : (entry.text ?? "")
          .split("\n")
          .map((line) => line.trim())
          .find(Boolean) ?? "";
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
