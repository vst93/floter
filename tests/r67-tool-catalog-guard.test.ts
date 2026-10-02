// R67 · the Rust↔TS guard for the install catalog.
//
// The install data lives once, in `src-tauri/src/extensions/tool_catalog.rs`
// (a tool that is not installed cannot produce a descriptor, so there is no
// runtime source for it). The frontend holds the *syntax* — one command
// template per manager — and a mirror of the tool ids. Two languages therefore
// describe the same vocabulary, which is exactly the kind of pair that drifts
// silently.
//
// This guard compares both directions with `deepEqual`, so neither a Rust entry
// without a frontend id nor a frontend id without a Rust entry can survive:
//
//   * the tool id set (Rust `TOOL_CATALOG` vs `TOOL_CATALOG_IDS`),
//   * the recipe platform key set (Rust `RecipeTable` fields vs
//     `RECIPE_PLATFORMS`), and
//   * the package-manager id set (Rust `PACKAGE_MANAGERS` vs
//     `MANAGER_INSTALL_COMMANDS`).
//
// The remaining cases pin the architecture red lines: detection is a `stat` of
// the host search path (never the bare process `PATH`, never a spawned
// process), and the terminal hand-off is wired to the launcher row and the
// settings install button (R68).

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  MANAGER_INSTALL_COMMANDS,
  RECIPE_PLATFORMS,
  TOOL_CATALOG_IDS,
} from "../src/extensions/tool-install.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

/** Strip Rust comments so a prose mention cannot satisfy or defeat a code check. */
const stripRustComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The text between `start` and the first `end` after it. */
const sliceBetween = (source: string, start: string, end: string): string => {
  const from = source.indexOf(start);
  assert.notEqual(from, -1, `missing marker: ${start}`);
  const to = source.indexOf(end, from);
  assert.notEqual(to, -1, `missing end marker: ${end}`);
  return source.slice(from, to);
};

const catalog = () => read("src-tauri/src/extensions/tool_catalog.rs");

// ── 1 · the id set, both directions ───────────────────────────────────────

test("the frontend's tool ids are exactly the Rust catalog's ids", async () => {
  const rust = await catalog();
  const table = sliceBetween(rust, "pub const TOOL_CATALOG", "fn find_first");
  const rustIds = [...table.matchAll(/\bid: "([^"]+)"/g)].map((match) => match[1]);
  assert.ok(rustIds.length > 0, "the Rust catalog must declare ids");
  assert.deepEqual(
    [...rustIds].sort(),
    [...TOOL_CATALOG_IDS].sort(),
    "a tool added on one side only is a drift the guard must catch",
  );
});

// ── 2 · the platform key set, both directions ─────────────────────────────

test("the frontend's platform keys are exactly the Rust recipe-table keys", async () => {
  const rust = await catalog();
  const recipeTable = sliceBetween(rust, "pub struct RecipeTable {", "}");
  const rustKeys = [...recipeTable.matchAll(/pub (\w+):/g)].map((match) => match[1]);
  assert.deepEqual(
    rustKeys,
    [...RECIPE_PLATFORMS],
    "RecipeTable's fields and RECIPE_PLATFORMS must name the same platforms",
  );

  // The declared vocabulary constant agrees too, so a third source cannot
  // disagree with either.
  const keys = /pub const PLATFORM_KEYS: &\[&str\] = &\[([^\]]+)\];/.exec(rust);
  assert.ok(keys, "PLATFORM_KEYS must be declared");
  const declared = [...keys![1].matchAll(/"([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(declared, [...RECIPE_PLATFORMS]);
});

// ── 3 · the manager id set, both directions ───────────────────────────────

test("the frontend's command table covers exactly the Rust package managers", async () => {
  const rust = await catalog();
  const managers = sliceBetween(
    rust,
    "pub const PACKAGE_MANAGERS",
    "/// The tool table",
  );
  const rustIds = [...managers.matchAll(/\bid: "([^"]+)"/g)].map((match) => match[1]);
  assert.ok(rustIds.length > 0, "the Rust manager table must declare ids");
  assert.deepEqual(
    [...rustIds].sort(),
    Object.keys(MANAGER_INSTALL_COMMANDS).sort(),
    "every manager Rust can name must have a command template",
  );
});

// ── 4 · detection is a stat of the one search path ────────────────────────

test("the catalog module spawns nothing and reads no environment PATH", async () => {
  const rust = stripRustComments(await catalog());
  for (const forbidden of [
    "Command::new",
    "std::process",
    "reqwest",
    "std::env::var",
    "env::var_os",
  ]) {
    assert.ok(
      !rust.includes(forbidden),
      `detection must not use ${forbidden}: it is a stat, never a process`,
    );
  }
  // It resolves presence through the same executable predicate the rest of the
  // extension runtime uses.
  assert.match(rust, /install::is_linked_executable_public/);
});

test("the IPC command scans the shared search path", async () => {
  const commands = stripJsComments(await read("src-tauri/src/commands/extensions.rs"));
  const start = commands.indexOf("pub fn extensions_tool_catalog");
  assert.notEqual(start, -1, "the command must exist");
  const body = commands.slice(start, commands.indexOf("\n}", start));
  assert.match(
    body,
    /runtime_path::search_directories\(\)/,
    "the one path source, not the bare process PATH",
  );
  assert.match(body, /tool_catalog::build_report\(/);

  // Registered with the Tauri handler, or the frontend cannot reach it.
  const lib = stripJsComments(await read("src-tauri/src/lib.rs"));
  const handler = lib.slice(lib.indexOf("tauri::generate_handler!["));
  assert.match(handler, /extensions_tool_catalog,/);
});

// ── 5 · the hand-off is wired this round (R68) ─────────────────────────────

test("openInstallSession is wired to the launcher row and the panel button", async () => {
  // R67 shipped the mechanism and pinned it *unwired*; R68 is the round that
  // turns it on. The launcher's install branch reads the row's `installCommand`
  // and the settings panel routes its install button through
  // `onInstallInTerminal` (which `App.tsx` wires to the R60 open path). The
  // mechanism itself is unchanged: a bare session, then the keystrokes.
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(actions, /openInstallSession\(/, "the launcher reaches the hand-off");
  assert.match(actions, /item\.installCommand/, "it keys off the row's own field");

  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(panel, /installCommandForId\(/, "the button asks the catalog");
  assert.match(panel, /onInstallInTerminal\(/, "and hands the command to the app");

  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /onInstallInTerminal=\{\(commandLine: string\) => openTerminalSession\(commandLine\)\}/,
    "the one R60 open path carries the install command",
  );

  // The mechanism is still exported from its own module, still bare-spawn.
  const module = await read("src/extensions/tool-install.ts");
  assert.match(module, /export const openInstallSession = async/);
  assert.match(module, /deps\.ensureTerminalSession\(null\)/);
});

test("the install button prefers the terminal hand-off and falls back to the homepage", async () => {
  // R68 flips the R67 boundary: the homepage open is still present — it is the
  // fallback when the catalog has no recipe — but the terminal hand-off is the
  // preferred branch, and the row's label changes with it.
  const panel = await read("src/ExtensionsPanel.tsx");
  assert.match(
    panel,
    /invoke\("open_url", \{ url: extension\.homepage \}\)/,
    "the homepage remains the fallback",
  );
  assert.match(panel, /const installLine = toolCatalog/);
  assert.match(panel, /onInstallInTerminal\(installLine\)/);
  const row = await read("src/extensions/ExtensionRow.tsx");
  assert.match(row, /installInTerminal/, "the row carries the new behavior");
  assert.match(row, /settings\.extensions\.installInTerminal/);
});
