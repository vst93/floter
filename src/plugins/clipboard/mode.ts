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
  type ClipboardEntryType,
} from "../../clipboard-history.ts";
import type { MessageKey, Translate } from "../../i18n.ts";
import type { PluginRow } from "../../launcher/plugin-mode.ts";
import { MAX_RESULTS } from "../../launcher/result-budget.ts";

/** How many clipboard rows the mode shows. The budget is the same ten rows
 *  (`MAX_RESULTS`), and the mode's list has no fixed tail, so nine is the
 *  ceiling the launcher can number without scrolling. */
export const CLIPBOARD_FETCH_LIMIT = MAX_RESULTS - 1;

/** The type word each clipboard entry kind prints on its row. The five keys are
 *  the clipboard panel's own type labels, so the launcher and the page name the
 *  same thing the same way. */
const CLIPBOARD_TYPE_KEYS: Record<ClipboardEntryType, MessageKey> = {
  text: "clipboard.typeText",
  link: "clipboard.typeLink",
  color: "clipboard.typeColor",
  image: "clipboard.typeImage",
  files: "clipboard.typeFiles",
};

/** A status line for the clipboard mode: nothing copied yet, or the plugin
 *  switched off. Information, not a door — `kind` tells the capability layer to
 *  draw it as the launcher's muted note (R30) and `disabled` makes a list of
 *  only these display-only. */
export const clipboardStatusRow = (id: string, key: MessageKey, t: Translate): PluginRow => ({
  family: "clipboard",
  id,
  title: t(key),
  subtitle: "",
  disabled: true,
  kind: "status",
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
  const kindKey = CLIPBOARD_TYPE_KEYS[clipboardEntryType(entry)];
  const age = formatClipboardAge(entry.created_at, now);
  return {
    family: "clipboard",
    id: entry.id,
    title: preview || t(kindKey),
    subtitle: age ? `${t(kindKey)} · ${age}` : t(kindKey),
    entry,
  };
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
  needle: string,
  t: Translate,
  now: number,
  /** R29 · the ceiling on the filtered rows. Defaults to the nine-row viewport
   *  budget; the launcher's inline mode raises it and pages the held rows
   *  client-side (`paginatePluginRows`). */
  limit: number = CLIPBOARD_FETCH_LIMIT,
): PluginRow[] => {
  const matches = filterClipboardEntries([...entries], needle).slice(0, limit);
  if (matches.length) return matches.map((entry) => clipboardRow(entry, t, now));
  return [
    clipboardStatusRow(
      "clipboard-empty",
      needle ? "clipboard.emptyFilter" : "clipboard.empty",
      t,
    ),
  ];
};
