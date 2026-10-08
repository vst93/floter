// R69 · the graft layer's last link: the launcher's **invoke row**.
//
// R68 shipped the install row ("you do not have this tool, here is the
// command"); R69 ships its other half ("you have this tool, here is how to call
// it out"). Both are the same `command` family, and a tool's own state decides
// which one — or neither — a query earns:
//
//   1. `tool-rows.ts` — the pure invoke-row builder: a truth table over
//      detected / not detected, hint / no hint, match / no match, plus the row
//      shape (`execution: null`, `launchArgv` set, no `installCommand`).
//   2. the wiring — source assertions that `useLauncherActions` reads
//      `launchArgv` into `system_spawn_detached` (bare program + args, no
//      shell), that the catalog appends the rows after the install rows and
//      marks them runnable, and that the renderer does not dim them.
//   3. the Rust half — the launch data is the GUI/TUI subset, and the new IPC
//      command is a single detached spawn with no shell anywhere.
//   4. the i18n — `launcher.invokeFailed` exists in both dictionaries and is not
//      byte-identical.
//
// The pure module needs no DOM, no Tauri host and no network.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  type InstallPlatform,
  type ToolCatalogEntry,
  type ToolCatalogReport,
  type ToolRecipe,
} from "../src/extensions/tool-install.ts";
import {
  TOOL_INVOKE_ROW_LIMIT,
  toolInstallRows,
  toolInvokeRows,
} from "../src/extensions/tool-rows.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
/** Strip Rust comments so a prose mention cannot satisfy or defeat a code check. */
const stripRustComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const tool = (
  id: string,
  displayName: string,
  keywords: string[],
  detected: boolean,
  launch: { argv: string[]; needsTerminal: boolean } | null,
): ToolCatalogEntry => ({
  id,
  displayName,
  keywords,
  recipes: {
    macos: [],
    linux: [{ manager: "apt", package: id } as ToolRecipe],
    windows: [],
  },
  launch,
  detected,
});

/** A detected GUI tool: the only shape that earns an invoke row. */
const flame = (detected = true) =>
  tool("flameshot", "Flameshot", ["截图", "screenshot", "jietu"], detected, {
    argv: ["flameshot", "gui"],
    needsTerminal: false,
  });

/** A detected pure CLI tool: no hint, so no invoke row. */
const jq = (detected = true) =>
  tool("jq", "jq", ["json", "解析"], detected, null);

const report = (
  tools: ToolCatalogEntry[],
  platform: InstallPlatform = "linux",
): ToolCatalogReport => ({
  platform,
  managers: [{ id: "apt", displayName: "apt", detected: true }],
  tools,
});

// ── 1 · the invoke-row truth table ────────────────────────────────────────

test("a detected tool with a launch hint and a matching query earns one row", () => {
  const rows = toolInvokeRows(report([flame()]), "flameshot");
  assert.equal(rows.length, 1);
  const row = rows[0];
  assert.equal(row.type, "command");
  assert.equal(row.id, "tool-invoke:flameshot");
  assert.equal(row.title, "Flameshot");
  assert.equal(row.subtitle, "flameshot gui", "the subtitle is the argv, joined");
  assert.equal(row.commandLine, "flameshot gui");
  assert.deepEqual(row.launchArgv, ["flameshot", "gui"]);
  assert.equal(row.execution, null, "never enters the provider run path");
  assert.equal(row.completion, false);
  assert.deepEqual(row.warnings, []);
  // The two hand-off fields are mutually exclusive on the row itself.
  assert.equal(row.installCommand, undefined);
});

test("the same match predicate drives both rows: a keyword reaches the invoke row", () => {
  // 截图 and the pinyin initials match through `toolEntryMatches`, exactly as
  // they reach the install row.
  for (const needle of ["截图", "FLAMESHOT", "jietu", "screenshot"]) {
    assert.equal(toolInvokeRows(report([flame()]), needle).length, 1, needle);
  }
  assert.deepEqual(toolInvokeRows(report([flame()]), "vscode"), []);
  assert.deepEqual(toolInvokeRows(report([flame()]), ""), [], "an empty query earns nothing");
});

test("an undetected tool earns the install row, never the invoke row", () => {
  // Mutual exclusion, asserted through the two builders on one report: exactly
  // one row total, and it is the install row.
  const missing = report([flame(false)]);
  const install = toolInstallRows(missing, "flameshot", "linux");
  const invoke = toolInvokeRows(missing, "flameshot");
  assert.equal(install.length, 1, "the missing tool is an install row");
  assert.equal(invoke.length, 0, "and never an invoke row");
  assert.equal(install.length + invoke.length, 1, "one tool, one row");
});

test("a detected tool with no launch hint earns no invoke row", () => {
  // The pure CLI case: `jq` is installed, but it reads standard input and has
  // nothing to start detached.
  assert.deepEqual(toolInvokeRows(report([jq()]), "jq"), []);
  // And it is not an install row either — it is already here.
  assert.deepEqual(toolInstallRows(report([jq()]), "jq", "linux"), []);
});

test("a null report is a soft landing, not an error", () => {
  assert.deepEqual(toolInvokeRows(null, "flameshot"), []);
});

test("rows follow catalog order and share the documented budget", () => {
  const many = report([
    tool("a1", "Alpha", ["needle"], true, { argv: ["a1"], needsTerminal: false }),
    tool("a2", "Alpha Two", ["needle"], true, { argv: ["a2"], needsTerminal: false }),
    tool("a3", "Alpha Three", ["needle"], true, { argv: ["a3"], needsTerminal: false }),
    tool("a4", "Alpha Four", ["needle"], true, { argv: ["a4"], needsTerminal: false }),
  ]);
  const rows = toolInvokeRows(many, "needle");
  assert.equal(rows.length, TOOL_INVOKE_ROW_LIMIT);
  assert.equal(TOOL_INVOKE_ROW_LIMIT, 3);
  assert.deepEqual(
    rows.map((row) => row.id),
    ["tool-invoke:a1", "tool-invoke:a2", "tool-invoke:a3"],
    "the first three in catalog order",
  );
});

test("the row copies the argv rather than aliasing the catalog payload", () => {
  const entry = flame();
  const rows = toolInvokeRows(report([entry]), "flameshot");
  assert.notEqual(rows[0].launchArgv, entry.launch!.argv, "a fresh array per row");
  assert.deepEqual(rows[0].launchArgv, entry.launch!.argv);
});

// ── 2 · the wiring ────────────────────────────────────────────────────────

test("the launcher's invoke branch sits beside the install branch and before the provider path", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  const installBranch = source.indexOf("!item.execution && item.installCommand");
  const invokeBranch = source.indexOf("!item.execution && item.launchArgv");
  const executionBranch = source.indexOf("runCommand(item.execution, item.commandLine)");
  assert.notEqual(installBranch, -1, "the install branch must exist");
  assert.notEqual(invokeBranch, -1, "the invoke branch must exist");
  assert.notEqual(executionBranch, -1, "the execution branch must still exist");
  assert.ok(installBranch < invokeBranch, "the invoke branch follows the install branch");
  assert.ok(
    invokeBranch < executionBranch,
    "and both sit in front of the provider run path",
  );
  assert.match(source, /void invokeToolLaunch\(item\.launchArgv\)/);
});

test("the invoke hands the argv over as bare program + args, with no shell", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  // The argv is split, never joined: program is the first element, args the
  // rest, and they reach the Rust command as separate fields.
  assert.match(source, /const \[program, \.\.\.args\] = argv/);
  assert.match(source, /invoke\("system_spawn_detached", \{ program, args \}\)/);
  assert.match(source, /showLauncherFeedback\("launcher\.invokeFailed"\)/);
  // No terminal page and no R62 flag on this path: it spawns the program
  // itself, not a shell.
  const start = source.indexOf("const invokeToolLaunch");
  const body = source.slice(start, source.indexOf("\n  };", start));
  assert.ok(!/ensureTerminalSession|openInstallSession|openTerminalSession/.test(body));
  assert.ok(!/join\(|shell/i.test(body), "no shell string is ever built");
});

test("the catalog appends the invoke rows after the install rows and marks them runnable", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(catalog, /toolInvokeRows\(toolCatalog, needle\)/);
  assert.match(
    catalog,
    /\[\.\.\.commandItems, \.\.\.rankedMatches, \.\.\.installRows, \.\.\.invokeRows\]\.slice\(0, MAX_RESULTS\)/,
    "invoke rows come after the install rows and the ordinary matches",
  );
  // R88 · the runnable rule moved to `launcher/result-budget.ts`; the catalog
  // hook delegates to `resultRunnableFlags`, so the clause is asserted there.
  const budget = stripJsComments(await read("src/launcher/result-budget.ts"));
  assert.match(
    budget,
    /Boolean\(item\.execution\) \|\| Boolean\(item\.installCommand\) \|\| Boolean\(item\.launchArgv\)/,
    "an invoke row is runnable",
  );
  assert.match(
    catalog,
    /const runnableResultFlags = resultRunnableFlags\(launcherResults\);/,
    "the catalog asks the shared rule",
  );
});

test("the renderer does not dim an invoke row", async () => {
  const renderer = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(
    renderer,
    /!item\.execution && !item\.installCommand && !item\.launchArgv/,
    "the runtime-unavailable predicate excludes invoke rows",
  );
  assert.match(renderer, /launchArgv\?: string\[\]/, "the row family carries the field");
});

// ── 3 · the Rust half ─────────────────────────────────────────────────────

test("the new IPC command is a single detached spawn of the bare argv", async () => {
  const source = stripRustComments(await read("src-tauri/src/commands/actions.rs"));
  const start = source.indexOf("pub fn system_spawn_detached");
  assert.notEqual(start, -1, "the command must exist");
  const body = source.slice(start, source.indexOf("\n}", start));
  assert.match(
    body,
    /spawn_application\(program, &args\)/,
    "the one launch entry point, with the argv passed through untouched",
  );
  // Zero shell: no interpreter, no `-c`, no string re-parsing.
  assert.ok(!/"-c"/.test(body), "no shell -c");
  assert.ok(!/Command::new/.test(body), "the command does not build its own Command");
  assert.ok(!/sh|bash|cmd\.exe/.test(body), "no shell is named");

  // Registered, or the frontend cannot reach it.
  const lib = stripJsComments(await read("src-tauri/src/lib.rs"));
  const handler = lib.slice(lib.indexOf("tauri::generate_handler!["));
  assert.match(handler, /system_spawn_detached,/);
});

test("the Rust launch data is the GUI/TUI subset with the exact argv", async () => {
  const rust = await read("src-tauri/src/extensions/tool_catalog.rs");
  const table = rust.slice(
    rust.indexOf("pub const TOOL_CATALOG"),
    rust.indexOf("fn find_first"),
  );
  const blocks = table.split("ToolCatalogEntry {").slice(1);
  const launched: Record<string, string[] | null> = {};
  for (const block of blocks) {
    const id = /id: "([^"]+)"/.exec(block)?.[1];
    assert.ok(id, "every entry declares an id");
    const none = /launch:\s*None/.test(block);
    const some = /launch:\s*Some\(launch\(&\[([^\]]*)\]/.exec(block);
    assert.ok(none || some, `${id} must declare its launch field`);
    launched[id!] = none
      ? null
      : [...some![1].matchAll(/"([^"]*)"/g)].map((match) => match[1]);
  }
  // Only the two GUI/TUI tools carry a hint; a pure CLI filter stays None.
  assert.deepEqual(
    Object.entries(launched)
      .filter(([, argv]) => argv !== null)
      .map(([id]) => id),
    ["flameshot", "lazygit"],
  );
  assert.deepEqual(launched.flameshot, ["flameshot", "gui"]);
  assert.deepEqual(launched.lazygit, ["lazygit"]);
  for (const cli of ["jq", "fd", "ripgrep", "fzf", "bat", "eza", "tldr", "httpie", "gh", "yt-dlp"]) {
    assert.equal(launched[cli], null, `${cli} is a CLI tool and must carry no hint`);
  }
});

// ── 4 · i18n ──────────────────────────────────────────────────────────────

test("launcher.invokeFailed is paired and translated, not byte-identical", async () => {
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
  const enValue = value(en, "launcher.invokeFailed");
  const zhValue = value(zh, "launcher.invokeFailed");
  assert.notEqual(enValue, zhValue, "the key must be translated");
  assert.ok(enValue.length > 0 && zhValue.length > 0);
});
