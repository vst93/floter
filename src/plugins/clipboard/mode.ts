// R28 · the clipboard plugin's *output* — and nothing else.
//
// R27 built the clipboard mode as the browser mode's twin, each of them mapping
// its own data to `LauncherItem` rows inside the catalog hook. R28 splits that:
// this module turns clipboard entries into standard `PluginRow`s and the
// capability layer (`launcher/plugin-mode.ts`) decides the form, the tier and
// the height.
//
// What stays here is the clipboard's own vocabulary — the type words its panel
// already prints, the one-line preview, the compact age, the in-memory filter
// over the whole history — because those are facts about clipboard entries, not
// about launchers.

import {
  clipboardEntryType,
  clipboardPreview,
  filterClipboardEntries,
  formatClipboardAge,
  formatFilesPreview,
  type ClipboardEntry,
} from "../../clipboard-history.ts";
import type { MessageKey, Translate } from "../../i18n.ts";
import type { ClipboardMode, ClipboardModeFilter } from "../../launcher.ts";
import type { PluginRow } from "../../launcher/plugin-mode.ts";
import { MAX_RESULTS } from "../../launcher/result-budget.ts";
import { statusRowBase } from "../status.ts";

/** How many clipboard rows the mode shows. The budget is the same ten rows
 *  (`MAX_RESULTS`). R36 derived nine from the launcher's fixed tail — the mode
 *  had no tail, so its ninth row was the highest the launcher could number.
 *  R37 retires the tail (the clipboard is an ordinary contributor), so the mode
 *  spends the whole budget too: its tenth row is reachable by `⌘0`, exactly as
 *  a ten-row plugin page is (see `resultShortcutSlots`). */
export const CLIPBOARD_FETCH_LIMIT = MAX_RESULTS;

/** The type word each clipboard entry prints on its row. The keys are the
 *  clipboard panel's own type labels, and the *four-way split is the chips'
 *  one* (`clipboardKindChip`): a colour literal is a `text` entry and its row
 *  says 文字, so the row's word, its icon and the chip that reveals it always
 *  agree. `clipboard.typeColor` remains the retired page's own label. */
const CLIPBOARD_KIND_KEYS: Record<ClipboardKindChip, MessageKey> = {
  text: "clipboard.typeText",
  link: "clipboard.typeLink",
  image: "clipboard.typeImage",
  files: "clipboard.typeFiles",
};

/** A status line for the clipboard mode: nothing copied yet, or the plugin
 *  switched off. Information, not a door — `kind` tells the capability layer to
 *  draw it as the launcher's muted note (R30) and `disabled` makes a list of
 *  only these display-only. */
export const clipboardStatusRow = (id: string, key: MessageKey, t: Translate): PluginRow => ({
  family: "clipboard",
  ...statusRowBase(id, t(key)),
});

/** One clipboard history row. The title is the entry's one-line preview (a file
 *  entry shows its basename, an image its caption or size); the subtitle is the
 *  type word plus the compact age, the same two facts the panel's own rows
 *  carry. */
export const clipboardRow = (entry: ClipboardEntry, t: Translate, now: number): PluginRow => {
  const files = entry.kind === "files" ? formatFilesPreview(entry.paths) : null;
  const preview = files
    ? `${files.basename}${files.extra > 0 ? ` +${files.extra}` : ""}`
    : clipboardPreview(entry, 96);
  const kindKey = CLIPBOARD_KIND_KEYS[clipboardKindChip(entry)];
  const age = formatClipboardAge(entry.created_at, now);
  return {
    family: "clipboard",
    id: entry.id,
    title: preview || t(kindKey),
    subtitle: age ? `${t(kindKey)} · ${age}` : t(kindKey),
    entry,
  };
};

/** R38 · which of the four *kind* chips an entry belongs to. Deliberately
 *  coarser than `clipboardEntryType`: the chips are 文字 / 图片 / 链接 / 文件
 *  with no colour chip, so a colour literal (a `text` entry) stays 文字. Images
 *  and file lists keep their stored kind; a text entry whose whole content is a
 *  URL is a link, and text and link are mutually exclusive with the link
 *  winning — one entry, one chip. */
export type ClipboardKindChip = "text" | "image" | "link" | "files";

export const clipboardKindChip = (entry: ClipboardEntry): ClipboardKindChip => {
  const type = clipboardEntryType(entry);
  if (type === "image") return "image";
  if (type === "files") return "files";
  if (type === "link") return "link";
  // `text` and `color` both land here: the chips have no colour face.
  return "text";
};

/**
 * R38 · whether an entry survives one chip. `all` keeps everything; `favorites`
 * is the orthogonal retention axis (any kind may be a favorite); the other four
 * are {@link clipboardKindChip}. Every entry matches exactly one kind chip, so
 * the four kind counts add up to the unfiltered total.
 */
export const clipboardEntryMatchesFilter = (
  entry: ClipboardEntry,
  filter: ClipboardModeFilter,
): boolean => {
  if (filter === "all") return true;
  if (filter === "favorites") return entry.favorite;
  return clipboardKindChip(entry) === filter;
};

/**
 * The mode's rows for one query: the history filtered in memory by the needle,
 * capped at the budget. An empty result is a status line, and the two empty
 * states are deliberately distinct — an empty history is not a search that
 * found nothing, and telling a user to shorten a query they never typed would
 * be a lie.
 */
export const clipboardModeRows = (
  entries: readonly ClipboardEntry[],
  mode: ClipboardMode,
  t: Translate,
  now: number,
  /** R29 · the ceiling on the filtered rows. Defaults to the nine-row viewport
   *  budget; the launcher's inline mode raises it and pages the held rows
   *  client-side (`paginatePluginRows`). */
  limit: number = CLIPBOARD_FETCH_LIMIT,
): PluginRow[] => {
  const matched = filterClipboardEntries([...entries], mode.needle).filter((entry) =>
    clipboardEntryMatchesFilter(entry, mode.filter),
  );
  const matches = matched.slice(0, limit);
  if (matches.length) return matches.map((entry) => clipboardRow(entry, t, now));
  return [
    clipboardStatusRow(
      "clipboard-empty",
      mode.needle
        ? "clipboard.emptyFilter"
        : mode.filter === "favorites"
          ? "clipboard.emptyFavorites"
          : "clipboard.empty",
      t,
    ),
  ];
};
