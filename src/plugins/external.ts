// R39 · external plugins as launcher modes, one command at a time.
//
// The two built-ins (browser, clipboard) are fused with the search box by hand:
// each owns a trigger word, a scope glyph, chips, and a `PluginEmission` the
// capability layer (`launcher/plugin-mode.ts`) turns into a list or text. The
// user asked for the same treatment for *external* plugins — the integrations
// the ExtensionsPanel installs — with two differences:
//
//   1. **A switch per command.** The user's words: 「对插件的每一个命令分别做一个
//      开启项，打开后就可以允许在搜索框内呼出插件」. Every command the extension's
//      provider descriptor declares gets an on/off switch in the integrations
//      panel; only an enabled command can be summoned as a mode.
//   2. **The field is the argv.** 「进入插件之后，后面输入内容自动为对应的命令行
//      参数，回车进行执行命令」. Once inside the mode, everything typed is the
//      command's arguments and Enter runs it. The output then goes through the
//      *existing* dual-form pipeline: a conforming list becomes the launcher's
//      list, anything else becomes text under the field.
//
// This module is the pure half: the gate, the trigger parser and the argv
// splitter. It is free of React, Tauri and the DOM so the node suite can pin
// them, the same arrangement `launcher/plugin-mode.ts` and `plugins/search.ts`
// use.

import type { ActivePluginMode } from "../launcher.ts";
import { splitTriggerWord } from "./mode-entry.ts";

/**
 * R39 · one command of an external plugin, as the Rust registry
 * (`external_plugin_commands`) reports it. `extensionId` + `commandId` are the
 * pair the switch map is keyed by; `commandId` and `aliases` are the trigger
 * vocabulary the user may type to enter the mode.
 */
export type ExternalPluginCommand = {
  extensionId: string;
  extensionName: string;
  commandId: string;
  name: string;
  description: string;
  aliases: string[];
  runtimeAvailable: boolean;
};

/**
 * R39 · the persisted per-command switches, mirroring the Rust
 * `AppSettings::plugin_command_switches`: `extensionId -> commandId -> enabled`.
 *
 * **Absence means off.** A command with no entry has never been enabled, which
 * is what 「打开后就可以允许」 asks for: the switch is the opt-in. An explicit
 * `false` and an absent key therefore behave the same, and both are distinct
 * from `true`.
 */
export type PluginCommandSwitches = Record<string, Record<string, boolean>>;

/** Whether one command's switch is on. Absence is off (see the type). */
export const isPluginCommandEnabled = (
  switches: PluginCommandSwitches,
  extensionId: string,
  commandId: string,
): boolean => switches[extensionId]?.[commandId] === true;

/**
 * The commands the user has enabled — the launcher's whole view of external
 * plugins. A plugin with no enabled command contributes nothing here, so it is
 * never summonable: 「未开任何命令的插件不出现」.
 */
export const enabledExternalCommands = (
  commands: readonly ExternalPluginCommand[],
  switches: PluginCommandSwitches,
): ExternalPluginCommand[] =>
  commands.filter((command) =>
    isPluginCommandEnabled(switches, command.extensionId, command.commandId),
  );

/** Whether any command of one extension is enabled (the plugin-level gate). */
export const pluginHasEnabledCommand = (
  commands: readonly ExternalPluginCommand[],
  switches: PluginCommandSwitches,
  extensionId: string,
): boolean =>
  commands.some(
    (command) =>
      command.extensionId === extensionId &&
      isPluginCommandEnabled(switches, extensionId, command.commandId),
  );

/**
 * R39 · the transition into an external plugin's mode: a typed value whose first
 * word names an enabled command (its id or one of its aliases) and whose next
 * character is whitespace.
 *
 * The rule is deliberately the *same* rule the built-ins use (`pluginModeEntry`
 * in `launcher.ts`): the trigger word must be followed by a space, so the bare
 * word stays an ordinary query and the mode is a deliberate place. `commands`
 * must already be the **enabled** subset — the gate is the caller's, so a
 * disabled command simply has no entry here and its trigger word falls through
 * to the ordinary search page.
 */
export const externalPluginModeEntry = (
  value: string,
  commands: readonly ExternalPluginCommand[],
): { mode: ActivePluginMode; needle: string } | null => {
  // R40 · the trigger split is the shared rule (`plugins/mode-entry.ts`): the
  // first whitespace-separated word, lowercased, and the rest of the value.
  const split = splitTriggerWord(value);
  if (!split) return null;
  const { word, rest } = split;
  const command = commands.find(
    (candidate) =>
      candidate.commandId.toLowerCase() === word ||
      candidate.aliases.some((alias) => alias.toLowerCase() === word),
  );
  if (!command) return null;
  return {
    mode: { scope: "external", extensionId: command.extensionId, commandId: command.commandId },
    needle: rest.trim(),
  };
};

/** R39 · the command an active external mode stands for, or `null` outside the
 *  mode (and when the command is no longer in the enabled registry — a switch
 *  flipped off while the mode is open). */
export const externalModeCommand = (
  mode: ActivePluginMode | null,
  commands: readonly ExternalPluginCommand[],
): ExternalPluginCommand | null => {
  if (mode?.scope !== "external") return null;
  return (
    commands.find(
      (command) =>
        command.extensionId === mode.extensionId && command.commandId === mode.commandId,
    ) ?? null
  );
};

/**
 * R39 · split the field's text into argv items.
 *
 * Whitespace separates arguments; a single or double quote groups a run that
 * contains spaces (the quote characters themselves are not part of the item).
 * There is no expansion, no globbing and no shell — the split is the whole of
 * the interpretation, and every item is passed to the process verbatim. An
 * unterminated quote takes the rest of the line, which is the least surprising
 * reading of a half-typed argument.
 */
export const splitPluginCommandArgs = (value: string): string[] => {
  const args: string[] = [];
  let current = "";
  let quote: '"' | "'" | null = null;
  let started = false;
  for (const character of value) {
    if (quote) {
      if (character === quote) {
        quote = null;
      } else {
        current += character;
      }
      started = true;
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      started = true;
      continue;
    }
    if (/\s/.test(character)) {
      if (started) {
        args.push(current);
        current = "";
        started = false;
      }
      continue;
    }
    current += character;
    started = true;
  }
  if (started) args.push(current);
  return args;
};

/** R39 · the captured output of one external command run, mirroring the Rust
 *  `PluginCommandOutput`. */
export type ExternalRunOutput = {
  success: boolean;
  exitCode: number | null;
  durationMs: number;
  stdout: string;
  stderr: string;
  truncated: boolean;
};

/** R39 · the mode's run state, held by the catalog hook. */
export type ExternalRunState =
  | { status: "idle" }
  | { status: "running" }
  | { status: "done"; output: ExternalRunOutput }
  | { status: "failed"; message: string };

/**
 * R39 · the text a finished run contributes to the dual-form pipeline, or `null`
 * when it contributed nothing printable.
 *
 * The command's **stdout** is the product. When it is empty the stderr is shown
 * instead — a failed command that printed nothing to stdout still has something
 * to say, and dropping it would leave an empty box after a non-zero exit. When
 * both are empty there is nothing to show and the caller prints a status line.
 */
export const externalRunText = (output: ExternalRunOutput): string | null => {
  if (output.stdout.trim().length > 0) return output.stdout;
  if (output.stderr.trim().length > 0) return output.stderr;
  return null;
};
