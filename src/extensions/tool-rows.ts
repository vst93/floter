// R68 · the launcher's install rows: "this tool is not installed, here is the
// command" as a *search result*.
//
// The row is deliberately the ordinary `command` family — same shape, same
// geometry, same `⌘N` slotting, same Enter path — with one difference: it
// carries `execution: null` (so it can never enter the provider run path) and
// an `installCommand` string instead. `useLauncherActions` reads that string
// and hands it to R67's `openInstallSession`; everything else about the row is
// the existing machinery.
//
// This module is pure over its three inputs (`report`, `needle`, `platform`)
// so the node suite can pin the truth table without a DOM, a Tauri host or a
// live machine. The fetch that produces `report` lives in
// `tool-catalog-store.ts`; the wiring lives in the launcher hook and the
// extensions panel.
//
// Two rules keep the rows out of the way of an ordinary search:
//
//   * a row exists only when the tool is **not** detected *and* this platform
//     has a recipe — a dead row (no command to type) is never rendered;
//   * the row matches only a real needle against the id, the display name or
//     the catalog's keywords, so an empty query and an unrelated query are
//     untouched.

import { normalizeSearch } from "../launcher.ts";
import type { LauncherItem } from "../launcher/LauncherResults";
import {
  chosenRecipe,
  installCommand,
  type InstallPlatform,
  type ToolCatalogEntry,
  type ToolCatalogReport,
} from "./tool-install.ts";

/** The most install rows one query may add. A discovery row is not the main
 *  match, and a single loose keyword ("git", "search") can hit several tools at
 *  once; three keeps the list a launcher and not a package index. The global
 *  `MAX_RESULTS` slice still applies on top of this. */
export const TOOL_INSTALL_ROW_LIMIT = 3;

/** One install row, in the shape the launcher list already draws. */
export type ToolInstallRow = Extract<LauncherItem, { type: "command" }>;

/** The ids of the package managers this machine actually has, from the same
 *  payload `installCommand` reads. */
export const detectedManagerIds = (report: ToolCatalogReport): string[] =>
  report.managers.filter((manager) => manager.detected).map((manager) => manager.id);

/** Whether `needle` is a case-insensitive substring of the entry's id, display
 *  name or any keyword. `needle` is normalized by the caller in the launcher,
 *  but this normalizes both sides so the pure function is honest on its own
 *  (and so "截图" and "SCREENSHOT" both work in a test without a hook). */
export const toolEntryMatches = (entry: ToolCatalogEntry, needle: string): boolean => {
  const query = normalizeSearch(needle);
  if (!query) return false;
  return [entry.id, entry.displayName, ...entry.keywords]
    .map((candidate) => normalizeSearch(candidate))
    .some((candidate) => candidate.includes(query));
};

/**
 * The install rows a query earns, in catalog order.
 *
 * `report` is `null` when the catalog could not be read (a soft landing: the
 * rows are discovery, not a feature the user asked for) and the answer is then
 * empty, silently. `platform` is the report's own platform; it is a parameter
 * so the builder stays a pure function of its inputs.
 */
export const toolInstallRows = (
  report: ToolCatalogReport | null,
  needle: string,
  platform: InstallPlatform,
): ToolInstallRow[] => {
  if (!report) return [];
  const managers = detectedManagerIds(report);
  const rows: ToolInstallRow[] = [];
  for (const entry of report.tools) {
    if (entry.detected) continue;
    if (!toolEntryMatches(entry, needle)) continue;
    const command = installCommand(entry, managers, platform);
    if (command === null) continue;
    // The right-hand source slot names the manager the command will use — the
    // same "where did this row come from" answer a catalog command gives with
    // its extension name. It is a proper noun from the payload, not i18n.
    const manager = chosenRecipe(entry, managers, platform)?.manager;
    const sourceName =
      report.managers.find((candidate) => candidate.id === manager)?.displayName ?? "";
    rows.push({
      type: "command",
      id: `tool-install:${entry.id}`,
      title: entry.displayName,
      // The command string itself, verbatim: the user reads what their shell
      // will receive. Deliberately not translated.
      subtitle: command,
      warnings: [],
      sourceName,
      commandLine: command,
      // No execution plan: this row must never enter the provider run path.
      execution: null,
      completion: false,
      installCommand: command,
    });
    if (rows.length >= TOOL_INSTALL_ROW_LIMIT) break;
  }
  return rows;
};

/**
 * The install command for one catalog id, or `null`. The extensions panel's
 * install button uses this to decide whether its click is a terminal hand-off
 * or the old homepage fallback; it is the same lookup the launcher rows are
 * built from, so the two surfaces can never disagree about which tools are
 * installable.
 */
export const installCommandForId = (
  report: ToolCatalogReport | null,
  id: string,
  platform: InstallPlatform,
): string | null => {
  if (!report) return null;
  const entry = report.tools.find((tool) => tool.id === id);
  if (!entry) return null;
  return installCommand(entry, detectedManagerIds(report), platform);
};
