// R68 · the install graft's UI round: the launcher install row, the settings
// install button, and the i18n that names them.
//
// Three layers are pinned here:
//
//   1. `tool-rows.ts` — the pure row builder. A truth table over match / no
//      match / detected / no-recipe / manager priority, plus the row shape the
//      `command` family requires (`execution: null`, `installCommand` set).
//   2. the wiring — source assertions that the launcher's Enter branch reads
//      the row's `installCommand`, that `openTerminalSession` carries it into
//      R67's bare-session hand-off, and that the settings button prefers it
//      with the homepage as the fallback.
//   3. the i18n — the new keys exist in both dictionaries and are not
//      byte-identical.
//
// The pure module needs no DOM, no Tauri host and no network.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  installCommand,
  type InstallPlatform,
  type ToolCatalogEntry,
  type ToolCatalogReport,
  type ToolRecipe,
} from "../src/extensions/tool-install.ts";
import {
  installCommandForId,
  TOOL_INSTALL_ROW_LIMIT,
  toolEntryMatches,
  toolInstallRows,
} from "../src/extensions/tool-rows.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const tool = (
  id: string,
  displayName: string,
  keywords: string[],
  detected: boolean,
  recipes: Partial<Record<InstallPlatform, ToolRecipe[]>>,
): ToolCatalogEntry => ({
  id,
  displayName,
  keywords,
  homepage: `https://example.test/${id}`,
  probeCandidates: { macos: [], linux: [], windows: [] },
  recipes: {
    macos: recipes.macos ?? [],
    linux: recipes.linux ?? [],
    windows: recipes.windows ?? [],
  },
  launch: null,
  detected,
});

const flame = () =>
  tool("flameshot", "Flameshot", ["截图", "screenshot", "screen", "jietu"], false, {
    macos: [{ manager: "brew", package: "flameshot" }],
    linux: [
      { manager: "pacman", package: "flameshot" },
      { manager: "apt", package: "flameshot" },
    ],
    windows: [{ manager: "winget", package: "Flameshot.Flameshot" }],
  });

const report = (
  tools: ToolCatalogEntry[],
  managers: ToolCatalogReport["managers"] = [
    { id: "brew", displayName: "Homebrew", detected: false },
    { id: "pacman", displayName: "pacman", detected: true },
    { id: "apt", displayName: "apt", detected: false },
  ],
  platform: InstallPlatform = "linux",
): ToolCatalogReport => ({ platform, managers, tools });

// ── 1 · the row builder truth table ───────────────────────────────────────

test("a needle matches the id, the display name or a keyword, case-insensitively", () => {
  const entry = flame();
  assert.ok(toolEntryMatches(entry, "flameshot"), "id");
  assert.ok(toolEntryMatches(entry, "FLAMESHOT"), "id, upper case");
  assert.ok(toolEntryMatches(entry, "Flameshot"), "display name");
  assert.ok(toolEntryMatches(entry, "截图"), "CJK keyword");
  assert.ok(toolEntryMatches(entry, "SCREENSHOT"), "Latin keyword, upper case");
  assert.ok(toolEntryMatches(entry, "jie"), "pinyin initials are substrings too");
  assert.ok(!toolEntryMatches(entry, "vscode"), "an unrelated word does not match");
  assert.ok(!toolEntryMatches(entry, "   "), "a blank needle matches nothing");
});

test("an install row is built only for an undetected tool with a recipe", () => {
  const present = report([flame()]);
  const rows = toolInstallRows(present, "flameshot", "linux");
  assert.equal(rows.length, 1);
  assert.equal(rows[0].type, "command");
  assert.equal(rows[0].id, "tool-install:flameshot");
  assert.equal(rows[0].title, "Flameshot");
  assert.equal(rows[0].subtitle, "sudo pacman -S flameshot");
  assert.equal(rows[0].commandLine, "sudo pacman -S flameshot");
  assert.equal(rows[0].installCommand, "sudo pacman -S flameshot");
  assert.equal(rows[0].execution, null, "never enters the provider run path");
  assert.equal(rows[0].completion, false);
  assert.equal(rows[0].sourceName, "pacman", "the right slot names the manager");
  assert.deepEqual(rows[0].warnings, []);
});

test("a detected tool is skipped: it is not an install, it is already here", () => {
  const detected = { ...flame(), detected: true };
  assert.deepEqual(toolInstallRows(report([detected]), "flameshot", "linux"), []);
});

test("a platform with no recipe yields no dead row", () => {
  const macOnly = tool("flameshot", "Flameshot", ["截图"], false, {
    macos: [{ manager: "brew", package: "flameshot" }],
  });
  assert.deepEqual(toolInstallRows(report([macOnly]), "flameshot", "linux"), []);
  assert.equal(toolInstallRows(report([macOnly]), "flameshot", "macos").length, 1);
});

test("a no-match query and a null report both yield nothing", () => {
  assert.deepEqual(toolInstallRows(report([flame()]), "vscode", "linux"), []);
  assert.deepEqual(toolInstallRows(report([flame()]), "", "linux"), []);
  assert.deepEqual(toolInstallRows(null, "flameshot", "linux"), []);
});

test("the detected manager wins, and the row carries the very same command", () => {
  const present = report(
    [flame()],
    [
      { id: "brew", displayName: "Homebrew", detected: false },
      { id: "pacman", displayName: "pacman", detected: false },
      { id: "apt", displayName: "apt", detected: true },
    ],
  );
  const entry = present.tools[0];
  const rows = toolInstallRows(present, "flameshot", "linux");
  assert.equal(rows[0].subtitle, "sudo apt install flameshot");
  assert.equal(
    rows[0].subtitle,
    installCommand(entry, ["apt"], "linux"),
    "the row is a pass-through of R67's own selection",
  );
  assert.equal(rows[0].sourceName, "apt");
});

test("the row budget is capped, and the cap is the documented constant", () => {
  const many = report([
    tool("a1", "Alpha", ["needle"], false, { linux: [{ manager: "apt", package: "a1" }] }),
    tool("a2", "Alpha Two", ["needle"], false, { linux: [{ manager: "apt", package: "a2" }] }),
    tool("a3", "Alpha Three", ["needle"], false, { linux: [{ manager: "apt", package: "a3" }] }),
    tool("a4", "Alpha Four", ["needle"], false, { linux: [{ manager: "apt", package: "a4" }] }),
  ]);
  const rows = toolInstallRows(many, "needle", "linux");
  assert.equal(rows.length, TOOL_INSTALL_ROW_LIMIT);
  assert.equal(TOOL_INSTALL_ROW_LIMIT, 3);
});

// ── 2 · the panel's lookup ────────────────────────────────────────────────

test("installCommandForId is the same lookup the rows use", () => {
  const present = report([flame()]);
  assert.equal(installCommandForId(present, "flameshot", "linux"), "sudo pacman -S flameshot");
  assert.equal(installCommandForId(present, "gh", "linux"), null, "not in the catalog");
  assert.equal(installCommandForId(null, "flameshot", "linux"), null);
  const macOnly = report(
    [tool("flameshot", "Flameshot", ["截图"], false, { macos: [{ manager: "brew", package: "flameshot" }] })],
    [{ id: "brew", displayName: "Homebrew", detected: true }],
    "macos",
  );
  assert.equal(installCommandForId(macOnly, "flameshot", "macos"), "brew install flameshot");
  assert.equal(installCommandForId(macOnly, "flameshot", "linux"), null);
});

// ── 3 · the wiring ────────────────────────────────────────────────────────

test("the launcher's install branch runs before the provider execution branch", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  const installBranch = source.indexOf('!item.execution && item.installCommand');
  const executionBranch = source.indexOf("runCommand(item.execution, item.commandLine)");
  assert.notEqual(installBranch, -1, "the install branch must exist");
  assert.notEqual(executionBranch, -1, "the execution branch must still exist");
  assert.ok(
    installBranch < executionBranch,
    "the install branch must sit in front of the provider path",
  );
  assert.match(source, /void openTerminalSession\(item\.installCommand\)/);
});

test("openTerminalSession carries the install command into the one R60 open path", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(source, /const openTerminalSession = async \(installCommand\?: string\): Promise<boolean>/);
  assert.match(source, /openInstallSession\(installCommand,/);
  assert.match(source, /showLauncherFeedback\(installCommand \? "launcher\.installFailed"/);
  // The bare session is still the R60 call — no command handed to the spawn.
  assert.match(source, /await ensureTerminalSession\(null\)/);
});

test("the row is runnable and not dimmed, without touching the command geometry", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(catalog, /Boolean\(item\.execution\) \|\| Boolean\(item\.installCommand\)/);
  assert.match(catalog, /toolInstallRows\(toolCatalog, needle, toolCatalog\.platform\)/);
  const renderer = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(
    renderer,
    /item\.type === "command" && !item\.execution && !item\.installCommand/,
    "an install row is not the runtime-unavailable kind",
  );
});

test("the store is memoized, reveal-refreshed and timer-free", async () => {
  const store = stripJsComments(await read("src/extensions/tool-catalog-store.ts"));
  assert.match(store, /let report: ToolCatalogReport \| null = null/);
  assert.match(store, /export const refreshToolCatalog/);
  assert.ok(!/setInterval|setTimeout/.test(store), "no polling, no timer");
  const app = await read("src/App.tsx");
  const reveal = app.indexOf('listen<string>("floter://revealed"');
  const refresh = app.indexOf("refreshToolCatalog()", reveal);
  assert.notEqual(reveal, -1, "the reveal listener exists");
  assert.notEqual(refresh, -1, "and refreshes the catalog once per reveal");
});

test("the panel prefers the terminal hand-off and keeps the homepage fallback", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /const installLine = toolCatalog/);
  assert.match(panel, /installCommandForId\(toolCatalog, extension\.id, toolCatalog\.platform\)/);
  // The branch condition itself: only a non-null catalog command takes the
  // terminal path; everything else falls through to the homepage open below.
  assert.match(panel, /if \(installLine\) \{/);
  assert.match(panel, /onInstallInTerminal\(installLine\)/);
  assert.match(panel, /invoke\("open_url", \{ url: extension\.homepage \}\)/);
  assert.match(panel, /installInTerminal=\{Boolean\(installLine\)\}/);

  const row = stripJsComments(await read("src/extensions/ExtensionRow.tsx"));
  // The label's own branch: a terminal install is named as such, the fallback
  // keeps the long-standing wording.
  assert.match(
    row,
    /installInTerminal \? "settings\.extensions\.installInTerminal" : "settings\.extensions\.installTool"/,
  );
  assert.match(row, /!extension\.runtimeAvailable && !extension\.homepage && !installInTerminal/);
});

// ── 4 · i18n ──────────────────────────────────────────────────────────────

test("the new keys are paired and translated, not byte-identical", async () => {
  const source = await read("src/i18n.ts");
  const en = source.slice(source.indexOf("const en = {"), source.indexOf("export type MessageKey"));
  const zh = source.slice(
    source.indexOf("const zh: Record<MessageKey, string> = {"),
    source.indexOf("const messages: Record<Language"),
  );
  const value = (body: string, key: string) => {
    const match = new RegExp(`^ {2}"${key.replace(/\./g, "\\.")}":\\s*"((?:[^"\\\\]|\\\\.)*)"`, "m").exec(body);
    assert.ok(match, `${key} must be declared`);
    return match![1];
  };
  for (const key of [
    "launcher.installFailed",
    "settings.extensions.installInTerminal",
    "settings.extensions.installInTerminalFailed",
  ]) {
    assert.notEqual(value(en, key), value(zh, key), `${key} must be translated`);
    assert.ok(value(en, key).length > 0 && value(zh, key).length > 0);
  }
});

// ── 5 · the Rust keywords table ───────────────────────────────────────────

test("every Rust catalog entry carries two to five keywords", async () => {
  const rust = await read("src-tauri/src/extensions/tool_catalog.rs");
  const table = rust.slice(
    rust.indexOf("pub const TOOL_CATALOG"),
    rust.indexOf("fn find_first"),
  );
  const entries = [...table.matchAll(/\bid: "([^"]+)"/g)].map((match) => match[1]);
  const keywords = [...table.matchAll(/keywords: &\[([^\]]*)\]/g)].map((match) => match[1]);
  assert.equal(keywords.length, entries.length, "every entry declares keywords");
  for (const list of keywords) {
    const items = [...list.matchAll(/"([^"]*)"/g)].map((match) => match[1]);
    assert.ok(items.length >= 2 && items.length <= 5, `keyword count ${items.length}`);
    assert.equal(new Set(items).size, items.length, "no repeated keyword");
  }
});
