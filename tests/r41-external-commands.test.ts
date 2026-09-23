// R41 · the per-command switch has an effect, and the panel says which words
// summon a command.
//
// The user's report, verbatim: 「现在这个开关打开的时候好像并没有什么效果啊，感觉
// 整体也需要优化逻辑和框架。」
//
// The gate itself was sound (switch -> `plugin_command_switches` ->
// `enabledExternalCommands` -> `externalPluginModeEntry`), but the *vocabulary*
// was not: R39 matched a command's id and aliases while the panel printed, and
// the hint told the user to type, the command's **name**. For `id: "search"` /
// `name: "Search"` case folding hid the gap; for any plugin whose name is not
// its id, turning the switch on changed nothing the user could see. This file
// pins the corrected vocabulary and the end-to-end pure chain, plus the panel
// wiring that projects the same state.
//
// Mutations that must turn this file red:
//   * `pluginCommandTriggers` dropping `command.name` -> the name test fails;
//   * `externalPluginModeEntry` going back to id/aliases only -> the name test
//     and the end-to-end "on" case fail;
//   * the panel dropping the `--off` de-emphasis or the triggers line -> the
//     source assertions fail.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  enabledExternalCommands,
  externalPluginModeEntry,
  isPluginCommandEnabled,
  pluginCommandTriggers,
  type ExternalPluginCommand,
} from "../src/plugins/external.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (src: string) =>
  src.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const command = (
  commandId: string,
  overrides: Partial<ExternalPluginCommand> = {},
): ExternalPluginCommand => ({
  extensionId: "local.tool",
  extensionName: "Tool",
  commandId,
  name: commandId.toUpperCase(),
  description: `${commandId} description`,
  aliases: [],
  runtimeAvailable: true,
  ...overrides,
});

// ── 1 · the trigger vocabulary ────────────────────────────────────────────

test("a command's trigger words are its id, its name and its aliases", () => {
  const triggers = pluginCommandTriggers(
    command("acme.search", {
      name: "Search Acme",
      aliases: ["acme", "find"],
    }),
  );
  // A multi-word name is reduced to the one token the trigger split compares.
  assert.deepEqual(triggers, ["acme.search", "Search", "acme", "find"]);
  // Deduplicated case-insensitively: id and name are often the same word in
  // different case, and the panel's "summon with" line must not repeat itself.
  assert.deepEqual(pluginCommandTriggers(command("search", { name: "SEARCH" })), [
    "search",
  ]);
  // Blank entries (a descriptor with an empty alias) are dropped.
  assert.deepEqual(
    pluginCommandTriggers(command("search", { name: "  ", aliases: ["", "  "] })),
    ["search"],
  );
});

test("a command whose name differs from its id is summonable by its name", () => {
  const commands = [
    command("acme.search", { name: "Search Acme", aliases: ["find"] }),
  ];
  // The R39 bug: typing the printed name did nothing. R41 matches it.
  assert.equal(
    externalPluginModeEntry("Search Acme rust", commands)?.mode.commandId,
    "acme.search",
  );
  assert.equal(
    externalPluginModeEntry("search acme rust", commands)?.mode.commandId,
    "acme.search",
    "case-insensitive, like every other trigger",
  );
  // The id and the alias still work at the same tier.
  assert.equal(
    externalPluginModeEntry("acme.search x", commands)?.mode.commandId,
    "acme.search",
  );
  assert.equal(
    externalPluginModeEntry("find x", commands)?.mode.commandId,
    "acme.search",
  );
});

// ── 2 · the end-to-end pure chain ─────────────────────────────────────────

test("the switch is the only fact: on summons, off refuses", () => {
  const commands = [
    command("acme.search", { name: "Search Acme", aliases: ["find"] }),
    command("acme.index", { name: "Index Acme" }),
  ];

  // Off (absence): nothing is summonable, exactly as the panel shows.
  const off = enabledExternalCommands(commands, {});
  assert.deepEqual(off, []);
  assert.equal(externalPluginModeEntry("Search Acme x", off), null);

  // Turning the switch on is the only write; the launcher re-derives its whole
  // view from the same settings map.
  const switches = { "local.tool": { "acme.search": true } };
  const on = enabledExternalCommands(commands, switches);
  assert.deepEqual(on.map((entry) => entry.commandId), ["acme.search"]);
  assert.equal(
    externalPluginModeEntry("Search Acme x", on)?.mode.commandId,
    "acme.search",
  );
  assert.equal(
    externalPluginModeEntry("Index Acme x", on),
    null,
    "the still-off sibling stays unsummonable",
  );

  // Turning it back off refuses again — no cached "was enabled" state.
  const backOff = enabledExternalCommands(commands, {
    "local.tool": { "acme.search": false },
  });
  assert.deepEqual(backOff, []);
  assert.equal(externalPluginModeEntry("Search Acme x", backOff), null);
});

test("the panel's switch reads the same gate the launcher does", () => {
  const switches = { "local.tool": { "acme.search": true } };
  assert.equal(isPluginCommandEnabled(switches, "local.tool", "acme.search"), true);
  assert.equal(isPluginCommandEnabled(switches, "local.tool", "acme.index"), false);
  assert.equal(isPluginCommandEnabled(switches, "local.other", "acme.search"), false);
});

// ── 3 · the panel wiring ──────────────────────────────────────────────────

test("the command card is two rows: identity + switch, then the alias field", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // The switch lives on the head line, the alias editor on its own line below
  // it — the layout the user asked for (「一个开关加一个输入框……被压缩」).
  const head = panel.indexOf("extension-command-list__head");
  const switchAt = panel.indexOf("settings-switch", head);
  const alias = panel.indexOf('className="extension-command-alias"', head);
  assert.ok(head > -1, "the card head must exist");
  assert.ok(
    switchAt > head && alias > switchAt,
    "the switch must be on the head line and the alias editor below it",
  );
  // A command whose switch is off is de-emphasised as a whole.
  assert.match(
    panel,
    /external && !enabled \? " extension-command-list__row--off" : ""/,
    "an off command's row is visually de-emphasised",
  );
  // The panel prints the same trigger words the launcher matches.
  assert.match(
    panel,
    /pluginCommandTriggers\(externalCommand\)\.join\(\", \"\)/,
    "the panel lists the real summoning words",
  );
  assert.match(panel, /t\("settings\.extensions\.commandTriggers"/);

  const css = await read("src/styles/extensions.css");
  assert.match(css, /\.extension-command-list__head\s*\{[^}]*display:\s*flex/);
  assert.match(css, /\.extension-command-list__row--off\s*\{[^}]*opacity:\s*0?\.\d+/);
});

test("the R41 panel copy exists in both languages", async () => {
  const source = await read("src/i18n.ts");
  const occurrences = source.split('"settings.extensions.commandTriggers"').length - 1;
  assert.equal(occurrences, 2, "settings.extensions.commandTriggers must be in both");
});
