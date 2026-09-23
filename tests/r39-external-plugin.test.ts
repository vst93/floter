// R39 · external plugins as launcher modes: a switch per command, a trigger
// word that enters the mode, the field's text as argv, and the command's output
// through the existing dual-form pipeline.
//
// The decisions are pure and live in `plugins/external.ts` and
// `launcher/plugin-mode.ts`, so most of this suite drives them directly. The
// pieces that are wiring rather than decisions (the panel's switch markup, the
// App's mode entry, the catalog hook's emission) are pinned at the source, the
// same way the other plugin suites pin a surface the node runner cannot
// instantiate.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import { externalModeFor, pluginModeEntry, pluginModeExitOnBackspace } from "../src/launcher.ts";
import {
  asPluginRows,
  pluginRowToItem,
  pluginTierFor,
  pluginViewInteractive,
  resolvePluginView,
} from "../src/launcher/plugin-mode.ts";
import {
  enabledExternalCommands,
  externalModeCommand,
  externalPluginModeEntry,
  externalRunText,
  isPluginCommandEnabled,
  pluginHasEnabledCommand,
  splitPluginCommandArgs,
  type ExternalPluginCommand,
} from "../src/plugins/external.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");

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

// ── A · the switch gate: absence means off ────────────────────────────────

test("a command is off until its switch is turned on", () => {
  assert.equal(isPluginCommandEnabled({}, "local.tool", "search"), false);
  assert.equal(isPluginCommandEnabled({ "local.tool": {} }, "local.tool", "search"), false);
  // An explicit `false` and an absent key are the same state: not summonable.
  assert.equal(
    isPluginCommandEnabled({ "local.tool": { search: false } }, "local.tool", "search"),
    false,
  );
  assert.equal(
    isPluginCommandEnabled({ "local.tool": { search: true } }, "local.tool", "search"),
    true,
  );
  // The extension id is part of the key: a command enabled on one plugin must
  // not enable the same-named command on another.
  assert.equal(
    isPluginCommandEnabled({ "local.other": { search: true } }, "local.tool", "search"),
    false,
  );
});

test("only enabled commands reach the launcher, and a plugin with none is absent", () => {
  const commands = [command("search"), command("index"), command("purge")];
  const switches = { "local.tool": { search: true, purge: true } };
  assert.deepEqual(
    enabledExternalCommands(commands, switches).map((entry) => entry.commandId),
    ["search", "purge"],
  );
  assert.equal(pluginHasEnabledCommand(commands, switches, "local.tool"), true);
  assert.equal(pluginHasEnabledCommand(commands, { "local.tool": {} }, "local.tool"), false);
  assert.equal(pluginHasEnabledCommand(commands, switches, "local.absent"), false);
});

// ── B · the trigger word and the argv splitter ────────────────────────────

test("a command word plus a space enters its mode and keeps the needle", () => {
  const commands = [command("search"), command("index", { aliases: ["idx"] })];
  const byId = externalPluginModeEntry("search rust async", commands);
  assert.deepEqual(byId, {
    mode: { scope: "external", extensionId: "local.tool", commandId: "search" },
    needle: "rust async",
  });
  // The alias is a trigger at the same tier as the id.
  assert.equal(
    externalPluginModeEntry("idx now", commands)?.mode.commandId,
    "index",
  );
  // Case-insensitive, like every other trigger word.
  assert.equal(externalPluginModeEntry("SEARCH x", commands)?.mode.commandId, "search");
});

test("the bare word is never an entry — the space is what makes it deliberate", () => {
  const commands = [command("search")];
  assert.equal(externalPluginModeEntry("search", commands), null);
  assert.equal(externalPluginModeEntry("searcher x", commands), null);
  assert.equal(externalPluginModeEntry("nope x", commands), null);
  // A disabled command is simply not in the enabled list, so its word falls
  // through to the ordinary search page.
  assert.equal(externalPluginModeEntry("search x", []), null);
});

test("the field's text splits into argv items, respecting quotes", () => {
  assert.deepEqual(splitPluginCommandArgs(""), []);
  assert.deepEqual(splitPluginCommandArgs("   "), []);
  assert.deepEqual(splitPluginCommandArgs("a b c"), ["a", "b", "c"]);
  assert.deepEqual(splitPluginCommandArgs("  a   b  "), ["a", "b"]);
  assert.deepEqual(splitPluginCommandArgs('--msg "hello world" tail'), [
    "--msg",
    "hello world",
    "tail",
  ]);
  assert.deepEqual(splitPluginCommandArgs("--msg 'hello world'"), ["--msg", "hello world"]);
  // An empty quoted run is a real (empty) argument.
  assert.deepEqual(splitPluginCommandArgs('a "" b'), ["a", "", "b"]);
  // An unterminated quote takes the rest of the line.
  assert.deepEqual(splitPluginCommandArgs('--msg "hello'), ["--msg", "hello"]);
});

test("the active mode resolves to its command and its request", () => {
  const commands = [command("search")];
  const mode = externalPluginModeEntry("search rust", commands)!.mode;
  assert.deepEqual(externalModeFor(mode, "  rust  "), {
    extensionId: "local.tool",
    commandId: "search",
    args: "rust",
  });
  assert.equal(externalModeFor({ scope: "clipboard", filter: "all" }, "x"), null);
  assert.equal(externalModeCommand(mode, commands)?.commandId, "search");
  // A command that left the enabled registry has no mode owner.
  assert.equal(externalModeCommand(mode, []), null);
});

test("an empty-field backspace leaves an external mode like any other", () => {
  const mode = { scope: "external" as const, extensionId: "local.tool", commandId: "search" };
  assert.equal(pluginModeExitOnBackspace(mode, ""), true);
  assert.equal(pluginModeExitOnBackspace(mode, "x"), false);
  assert.equal(pluginModeExitOnBackspace(null, ""), false);
});

// ── C · the output's two forms ────────────────────────────────────────────

test("a conforming list becomes the launcher's list, with the plugin's name", () => {
  const output = JSON.stringify([
    { id: "a", title: "First", subtitle: "one" },
    { id: "b", title: "Second", group: "Group", icon: "star", action: { type: "copy", text: "b" } },
  ]);
  const view = resolvePluginView({ output, sourceName: "Tool" });
  assert.equal(view?.form, "list");
  assert.equal(view?.tier, "interactive", "a row with an action takes the keyboard");
  assert.equal(pluginViewInteractive(view), true);
  if (view?.form !== "list") throw new Error("expected a list");
  assert.deepEqual(
    view.items.map((item) => item.type),
    ["plugin", "plugin"],
  );
  const second = view.items[1];
  assert.equal(second.type === "plugin" && second.sourceName, "Tool");
  assert.equal(second.type === "plugin" && second.group, "Group");
  assert.equal(second.type === "plugin" && second.icon, "star");
});

test("a list with no actions is display-only", () => {
  const rows = asPluginRows([{ id: "a", title: "First" }])!;
  assert.equal(pluginTierFor(rows), "display");
  const view = resolvePluginView({ output: [{ id: "a", title: "First" }] });
  assert.equal(view?.form, "list");
  assert.equal(view?.tier, "display");
  assert.equal(pluginViewInteractive(view), false);
});

test("a plugin may declare its own tier, and the layer honours it", () => {
  const rows = [{ id: "a", title: "First", action: { type: "open", url: "https://x" } }];
  assert.equal(pluginTierFor(asPluginRows(rows)!, "display"), "display");
  assert.equal(resolvePluginView({ output: rows, tier: "display" })?.tier, "display");
});

test("anything that is not a list is text — the command's own words", () => {
  const view = resolvePluginView({ output: "usage: tool [options]\n\n  -h  help" });
  assert.equal(view?.form, "text");
  assert.equal(view?.tier, "display");
  if (view?.form !== "text") throw new Error("expected text");
  assert.match(view.text, /usage: tool/);
  // A malformed row set falls to text *whole*, never half-parsed.
  const broken = resolvePluginView({ output: [{ id: "a", title: "First", action: { type: "x" } }] });
  assert.equal(broken?.form, "text");
});

test("a run's text is its stdout, its stderr as a fallback, or nothing", () => {
  const output = (stdout: string, stderr: string) => ({
    success: true,
    exitCode: 0,
    durationMs: 1,
    stdout,
    stderr,
    truncated: false,
  });
  assert.equal(externalRunText(output("hello\n", "warn\n")), "hello\n");
  assert.equal(externalRunText(output("", "boom\n")), "boom\n");
  assert.equal(externalRunText(output("", "")), null);
  assert.equal(externalRunText(output("  \n", "  \n")), null);
});

test("a status row downgrades to the launcher's note, whatever its family", () => {
  const rows = asPluginRows([{ id: "s", title: "Nothing to show", kind: "status" }])!;
  assert.deepEqual(pluginRowToItem(rows[0]), {
    type: "status",
    id: "s",
    title: "Nothing to show",
  });
});

test("the icon field is a closed vocabulary", () => {
  assert.ok(asPluginRows([{ id: "a", title: "A", icon: "star" }]));
  assert.equal(asPluginRows([{ id: "a", title: "A", icon: "not-an-icon" }]), null);
  // An unknown glyph makes the whole output text rather than a silent fallback.
  assert.equal(resolvePluginView({ output: [{ id: "a", title: "A", icon: "x" }] })?.form, "text");
});

// ── D · the wiring the node runner cannot instantiate ─────────────────────

test("the App owns the registry, the gate and the run", async () => {
  const app = stripJsComments(await read("src/App.tsx"));
  assert.match(
    app,
    /invoke<ExternalPluginCommand\[\]>\("external_plugin_commands"\)/,
    "the App loads the external command registry",
  );
  assert.match(
    app,
    /enabledExternalCommands\(externalCommands, settings\.plugin_command_switches\)/,
    "the enabled subset is the gate",
  );
  assert.match(
    app,
    /invoke<ExternalRunOutput>\("external_plugin_run"/,
    "the run goes through the dedicated command",
  );
  assert.match(
    app,
    /args: splitPluginCommandArgs\(externalMode\.args\)/,
    "the field's text is split into argv before the invoke",
  );
  assert.match(
    app,
    /externalPluginModeEntry\(\s*value,\s*enabledExternalCommandList,?\s*\)/,
    "the input's change handler enters an external mode from the enabled list",
  );
  assert.match(
    app,
    /pluginCommandSwitches=\{settings\.plugin_command_switches\}/,
    "the panel receives the switch map",
  );
  assert.match(
    app,
    /changeGeneralSetting\("plugin_command_switches"/,
    "flipping a switch persists through the ordinary settings path",
  );
});

test("the panel renders one switch per external command", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.match(
    panel,
    /onTogglePluginCommand\(selected\.id, command\.id, !enabled\)/,
    "the row's switch flips exactly this command",
  );
  assert.match(
    panel,
    /isPluginCommandEnabled\(\s*pluginCommandSwitches,\s*selected\.id,\s*command\.id,?\s*\)/,
    "the switch reads the same gate the launcher uses",
  );
  assert.match(
    panel,
    /t\("settings\.extensions\.commandEnable", \{ command: command\.name \}\)/,
    "the switch is labelled with the command",
  );
});

test("the catalog hook builds the external emission and gates the mode", async () => {
  const catalog = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));
  assert.match(catalog, /externalRunText\(externalRun\.output\)/, "the run's text feeds the pipeline");
  assert.match(
    catalog,
    /if \(externalMode\) return resolvePluginView\(windowedEmission\(externalEmission\)\)/,
    "the external view goes through the same resolver as the built-ins",
  );
  assert.match(
    catalog,
    /\(item\.type === "plugin" && \(item\.disabled === true \|\| item\.action === undefined\)\)/,
    "a row without an action is not a runnable result",
  );
});

test("the actions hook runs the command and the row's own action", async () => {
  const actions = stripJsComments(await read("src/hooks/useLauncherActions.ts"));
  assert.match(
    actions,
    /if \(externalScope && externalEnterRunsCommand\) \{\s*runExternalCommand\(\);\s*return;\s*\}/,
    "Enter runs the command when the view has no interactive list",
  );
  assert.match(actions, /if \(item\.type === "plugin"\) \{/, "a plugin row is dispatched");
  assert.match(
    actions,
    /invoke\("clipboard_write_text", \{ text: action\.text \}\)/,
    "the copy action uses the launcher's own clipboard command",
  );
  assert.match(
    actions,
    /void openWithSystem\("open_url", \{ url: action\.url \}\)/,
    "the open action uses the launcher's own system hand-off",
  );
});

test("the mode's four status lines exist in both dictionaries", () => {
  const zh = createTranslator("zh");
  for (const key of [
    "launcher.scopePlugin",
    "launcher.externalIdle",
    "launcher.externalRunning",
    "launcher.externalFailed",
    "launcher.externalEmpty",
    "launcher.error.copy",
    "settings.extensions.commandsHint",
    "settings.extensions.commandEnable",
  ] as const) {
    assert.notEqual(en(key), key, `${key} must exist in the English dictionary`);
    assert.notEqual(zh(key), key, `${key} must exist in the Chinese dictionary`);
  }
  // The idle line is the one the user reads first: it must say what to do.
  assert.match(en("launcher.externalIdle"), /Enter/);
  assert.match(zh("launcher.externalIdle"), /回车/);
});

test("the built-in trigger vocabulary still wins the namespace", () => {
  // An external command named like a built-in trigger is unreachable by its own
  // word: the built-in parser runs first, so `browser x` stays the browser mode.
  const builtin = pluginModeEntry("browser rust");
  assert.equal(builtin?.mode.scope, "browser");
  const external = externalPluginModeEntry("browser rust", [command("browser")]);
  assert.equal(external?.mode.scope, "external");
});
