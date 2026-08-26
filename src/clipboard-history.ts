// Pure logic behind the built-in clipboard history panel: entry normalization,
// filtering, previews and compact ages. Kept free of React and IPC so the
// node test suite can exercise it directly (see tests/clipboard-history.test.ts).

export type ClipboardEntry = {
  id: string;
  /** "text" | "image" */
  kind: string;
  text?: string | null;
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
    const kind = raw.kind === "image" ? "image" : raw.kind === "text" ? "text" : null;
    const id = typeof raw.id === "string" ? raw.id : null;
    const hash = typeof raw.hash === "string" ? raw.hash : null;
    if (!kind || !id || !hash) continue;
    entries.push({
      id,
      kind,
      text: typeof raw.text === "string" ? raw.text : null,
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
 * match on their content; images answer to their own names in either UI
 * language, since there is nothing else to say about them.
 */
export const filterClipboardEntries = (
  entries: ClipboardEntry[],
  query: string,
): ClipboardEntry[] => {
  const needle = query.trim().toLowerCase();
  if (!needle) return entries.slice();
  return entries.filter((entry) =>
    entry.kind === "text"
      ? (entry.text ?? "").toLowerCase().includes(needle)
      : ["image", "img", "图片"].some((word) => word.includes(needle)),
  );
};

const IMAGE_PREVIEW_MAX = 60;

/**
 * The one-line preview shown on each row: the first line of a text entry,
 * trimmed and capped, or an `[image WxH]` label for images.
 */
export const clipboardPreview = (entry: ClipboardEntry, maxLength = 120): string => {
  if (entry.kind !== "text") {
    const width = Number.isFinite(entry.width) ? entry.width : "?";
    const height = Number.isFinite(entry.height) ? entry.height : "?";
    return `[image ${width}x${height}]`;
  }
  const firstLine = (entry.text ?? "").split("\n")[0].trim();
  const cap = Math.max(1, Math.min(maxLength, IMAGE_PREVIEW_MAX * 2));
  return firstLine.length > cap ? `${firstLine.slice(0, cap)}…` : firstLine;
};

/**
 * Compact relative age in the terminal's own vocabulary: `42s`, `5m`, `3h`,
 * `21d`. Unit glyphs are shared by both UI languages, so no translation table
 * is involved.
 */
export const formatClipboardAge = (createdAtMs: number, nowMs: number): string => {
  if (!Number.isFinite(createdAtMs) || !Number.isFinite(nowMs)) return "";
  const seconds = Math.max(0, Math.floor((nowMs - createdAtMs) / 1000));
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
};
