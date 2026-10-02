// R68 · the launcher's install rows: "this tool is not installed, here is the
// command" as a *search result*.
//
// R69 adds the other half of the graft layer's last link: the **invoke row** —
// "this tool is installed and it has something to call out, here is the argv" —
// so discover → install → invoke never leaves the launcher. The two rows are the
// same `command` family and the same builder shape; the tool's own state decides
// which one (or neither) a query earns:
//
//   * not detected, a recipe exists        → install row (`installCommand`);
//   * detected, a `launch` hint exists     → invoke row (`launchArgv`);
//   * detected, no hint / not detected, no recipe → no row.
//
// They are mutually exclusive by construction (one requires `detected`, the
// other its negation), which is what stops one tool from appearing twice for
// one query.
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
 *  `MAX_RESULTS` slice still applies on top of this. R69's invoke rows share the
 *  same budget — a query is either "what is missing" or "what I can call out",
 *  and three is the same answer for both. */
export const TOOL_INSTALL_ROW_LIMIT = 3;

/** The most invoke rows one query may add (see [`TOOL_INSTALL_ROW_LIMIT`]). */
export const TOOL_INVOKE_ROW_LIMIT = TOOL_INSTALL_ROW_LIMIT;

/** One install row, in the shape the launcher list already draws. */
export type ToolInstallRow = Extract<LauncherItem, { type: "command" }>;

/** One invoke row, the same shape as an install row. */
export type ToolInvokeRow = Extract<LauncherItem, { type: "command" }>;

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
 * The invoke rows a query earns, in catalog order — R69's half of the loop.
 *
 * A row exists only for a tool that is **detected** *and* carries a `launch`
 * hint *and* matches the needle through the same [`toolEntryMatches`] predicate
 * the install rows use, so the two surfaces can never disagree about what a
 * query is about. The detected/undetected split is what makes an invoke row and
 * an install row mutually exclusive: a tool the user already has is offered a
 * way to call it out, never a way to install it again.
 *
 * The subtitle is the argv joined by spaces — the very bytes the detached spawn
 * will receive, shown to the user verbatim. Like the install command it is data
 * from the catalog, never an i18n string. The row carries no `execution` (it
 * must never enter the provider run path) and no `installCommand`; its
 * `launchArgv` is what `useLauncherActions` reads to call
 * `system_spawn_detached`.
 *
 * `report` is `null` when the catalog could not be read — a soft landing, the
 * same as the install rows — and the answer is then empty, silently.
 */
export const toolInvokeRows = (
  report: ToolCatalogReport | null,
  needle: string,
): ToolInvokeRow[] => {
  if (!report) return [];
  const rows: ToolInvokeRow[] = [];
  for (const entry of report.tools) {
    // Already here — the install row's opposite, and the reason the two can
    // never both appear for one tool.
    if (!entry.detected) continue;
    // A pure CLI tool has nothing to start detached (see the Rust `LaunchHint`
    // doc): no hint, no row.
    if (!entry.launch) continue;
    if (!toolEntryMatches(entry, needle)) continue;
    const argv = [...entry.launch.argv];
    rows.push({
      type: "command",
      id: `tool-invoke:${entry.id}`,
      title: entry.displayName,
      // The argv the spawn will receive, verbatim and untranslated.
      subtitle: argv.join(" "),
      warnings: [],
      // No extension contributed this row; an empty source prints nothing (see
      // `row-content.ts`), so the row is title + argv alone.
      sourceName: "",
      commandLine: argv.join(" "),
      // No execution plan: this row never enters the provider run path.
      execution: null,
      completion: false,
      launchArgv: argv,
    });
    if (rows.length >= TOOL_INVOKE_ROW_LIMIT) break;
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
