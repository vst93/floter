// R70 · the invoke row learns the difference between a GUI and a TUI.
//
// R69 shipped one call-out path for every hint: split the argv and hand it to
// `system_spawn_detached`, which gives the child null stdio. That is exactly
// right for a GUI program (`flameshot` owns a window and does not care about a
// terminal) and exactly wrong for a full-screen TUI (`lazygit` needs a real PTY
// or it exits the instant it starts — the user sees "nothing happened").
//
// R70 adds one data bit, `needsTerminal`, to the Rust `LaunchHint`, mirrors it
// in the TS catalog type, copies it onto the invoke row, and makes the row's
// branch route on it:
//
//   * `needsTerminal: true`  → `openTerminalSession(argv.join(" "))` — R68's
//     bare-session hand-off that types the argv into the user's own shell;
//   * `needsTerminal: false` → `system_spawn_detached`, exactly as R69.
//
// The bit is catalog data, never a program-name test: no branch inspects the
// string "lazygit". These tests pin the data, the mirror and the routing.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  type ToolCatalogEntry,
  type ToolCatalogReport,
} from "../src/extensions/tool-install.ts";
import { toolInvokeRows } from "../src/extensions/tool-rows.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
const stripRustComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

const tool = (
  id: string,
  displayName: string,
  detected: boolean,
  launch: {
    argv: string[];
    description: string;
    needsTerminal: boolean;
  } | null,
): ToolCatalogEntry => ({
  id,
  displayName,
  keywords: [],
  homepage: `https://example.test/${id}`,
  probeCandidates: { macos: [], linux: [], windows: [] },
  recipes: { macos: [], linux: [], windows: [] },
  launch,
  detected,
});

const report = (tools: ToolCatalogEntry[]): ToolCatalogReport => ({
  platform: "linux",
  managers: [{ id: "apt", displayName: "apt", detected: true }],
  tools,
});

const gui = () =>
  tool("flameshot", "Flameshot", true, {
    argv: ["flameshot", "gui"],
    description: "Take a screenshot",
    needsTerminal: false,
  });

const tui = () =>
  tool("lazygit", "lazygit", true, {
    argv: ["lazygit"],
    description: "Open the lazygit TUI",
    needsTerminal: true,
  });

// ── 1 · the row carries the bit, verbatim and untranslated ────────────────

test("an invoke row copies needsTerminal off the catalog entry", () => {
  assert.equal(toolInvokeRows(report([gui()]), "flameshot")[0].launchNeedsTerminal, false);
  assert.equal(toolInvokeRows(report([tui()]), "lazygit")[0].launchNeedsTerminal, true);
});

test("the bit never changes the row's shape, subtitle or budget", () => {
  // R70 is a routing change only: the row the user sees is identical for a GUI
  // and a TUI, save for the invisible data bit.
  const guiRow = toolInvokeRows(report([gui()]), "flameshot")[0];
  const tuiRow = toolInvokeRows(report([tui()]), "lazygit")[0];
  assert.equal(guiRow.type, "command");
  assert.equal(tuiRow.type, "command");
  assert.equal(guiRow.subtitle, "flameshot gui");
  assert.equal(tuiRow.subtitle, "lazygit", "the subtitle stays the argv, joined");
  assert.equal(guiRow.execution, null);
  assert.equal(tuiRow.execution, null);
  assert.equal(guiRow.installCommand, undefined);
  assert.equal(tuiRow.installCommand, undefined);
});

// ── 2 · the routing in the launcher branch ────────────────────────────────

test("the invoke branch routes a TUI to the terminal and a GUI to the detached spawn", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  // Find the invoke branch and read its body up to the provider path.
  const start = source.indexOf("!item.execution && item.launchArgv");
  assert.notEqual(start, -1, "the invoke branch must exist");
  const branch = source.slice(start, source.indexOf("runCommand(item.execution", start));
  // The predicate is the data bit, not the program name.
  assert.match(
    branch,
    /if \(item\.launchNeedsTerminal\) \{\s*void openTerminalSession\(item\.launchArgv\.join\(" "\)\);/,
    "a TUI opens a terminal session and types the argv in",
  );
  assert.match(branch, /void invokeToolLaunch\(item\.launchArgv\)/, "a GUI keeps R69's path");
  // No tool name is ever tested on this path.
  assert.ok(!/lazygit|flameshot/.test(branch), "no program name is hard-coded in the branch");
});

test("the terminal hand-off is R68's bare-session path, not a second one", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  // `openTerminalSession` with a command still opens the session bare and types
  // through `openInstallSession` — R62's predicate must stay false for the
  // user's own shell to outlive the call-out. That is the one path R68 built.
  const start = source.indexOf("const openTerminalSession");
  const body = source.slice(start, source.indexOf("\n  };", start));
  assert.match(body, /await ensureTerminalSession\(null\)/, "the session opens bare");
  assert.match(body, /openInstallSession\(installCommand/, "and R67 types the command in");
  assert.match(
    body,
    /showLauncherFeedback\(installCommand \? "launcher\.installFailed" : "launcher\.error\.command"\)/,
    "a refused hand-off reuses the existing install-failed key",
  );
});

test("the TUI path never reaches the detached spawn or the provider run path", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  // `invokeToolLaunch` is only reached when the bit is false — assert that the
  // TUI branch returns before it and that no shell string is joined anywhere
  // beyond the argv the terminal hand-off deliberately types.
  const branch = source.slice(
    source.indexOf("!item.execution && item.launchArgv"),
    source.indexOf("runCommand(item.execution"),
  );
  const tuiIndex = branch.indexOf("openTerminalSession(item.launchArgv.join(\" \"))");
  const guiIndex = branch.indexOf("invokeToolLaunch(item.launchArgv)");
  assert.ok(tuiIndex !== -1 && guiIndex !== -1);
  assert.ok(tuiIndex < guiIndex, "the terminal branch is decided first");
});

// ── 3 · the TS type mirror ────────────────────────────────────────────────

test("the TS catalog type mirrors the Rust field as needsTerminal", async () => {
  const source = await read("src/extensions/tool-install.ts");
  assert.match(
    source,
    /launch: \{ argv: string\[\]; description: string; needsTerminal: boolean \} \| null;/,
    "the payload type carries the bit",
  );
});

test("the row family declares the bit as optional data", async () => {
  const renderer = stripJsComments(await read("src/launcher/LauncherResults.tsx"));
  assert.match(renderer, /launchNeedsTerminal\?: boolean;/, "the row family carries the bit");
  // The bit must not become a second availability signal: the row is still
  // runnable purely on `launchArgv`.
  assert.match(
    renderer,
    /!item\.execution && !item\.installCommand && !item\.launchArgv/,
    "availability still keys off launchArgv alone",
  );
});

// ── 4 · the Rust data pin ─────────────────────────────────────────────────

test("the Rust LaunchHint declares needs_terminal and serializes it as camelCase", async () => {
  const rust = await read("src-tauri/src/extensions/tool_catalog.rs");
  assert.match(
    rust,
    /pub struct LaunchHint \{\s*pub argv: &'static \[&'static str\],\s*pub description: &'static str,\s*pub needs_terminal: bool,\s*\}/,
    "the struct has the field",
  );
  // The serde rename is on the struct, so the field crosses the wire camelCased.
  assert.match(
    rust,
    /#\[serde\(rename_all = "camelCase"\)\]\s*pub struct LaunchHint/,
    "the payload is camelCase",
  );
  // The two live hints carry the exact values the routing depends on.
  assert.match(
    rust,
    /launch: Some\(launch\(&\["flameshot", "gui"\], "Take a screenshot", false\)\)/,
    "flameshot is a GUI: detached",
  );
  assert.match(
    rust,
    /launch: Some\(launch\(&\["lazygit"\], "Open the lazygit TUI", true\)\)/,
    "lazygit is a TUI: terminal",
  );
});

test("the Rust table has no other launch hint that forgot its bit", async () => {
  const rust = stripRustComments(await read("src-tauri/src/extensions/tool_catalog.rs"));
  const table = rust.slice(
    rust.indexOf("pub const TOOL_CATALOG"),
    rust.indexOf("fn find_first"),
  );
  const blocks = table.split("ToolCatalogEntry {").slice(1);
  for (const block of blocks) {
    const id = /id: "([^"]+)"/.exec(block)?.[1];
    assert.ok(id, "every entry declares an id");
    const some = /launch:\s*Some\(launch\(&\[[^\]]*\],\s*"[^"]*",\s*(true|false)\)\)/.exec(block);
    const none = /launch:\s*None/.test(block);
    assert.ok(none || some, `${id} must declare a launch hint with its needs_terminal bit`);
  }
});
