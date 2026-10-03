// R80 · the platform-extraction round's three pure primitives: the command-row
// builder (`launcher/command-row.ts`), the wrap-cycle primitive
// (`wrap-index.ts`) and the plugin-config field builders
// (`plugins/config-schema.ts`).
//
// All three are pure, so this suite drives them directly — no DOM, no Tauri. It
// pins the *contract* the four row producers, six wrap cycles and two plugin
// pairs now delegate to; the zero-behaviour-change half is proved by the
// existing suites, which pass unmodified.

import assert from "node:assert/strict";
import test from "node:test";
import { commandRow, type CommandRowFields } from "../src/launcher/command-row.ts";
import { wrapIndex } from "../src/wrap-index.ts";
import { clampActionIndex, nextFileActionIndex } from "../src/launcher/file-drops.ts";
import { cyclePluginFilter, pluginFilterAxis } from "../src/plugins/filter-axis.ts";
import {
  CALCULATOR_CONFIG_SCHEMA,
  CLIPBOARD_CONFIG_SCHEMA,
  browserConfigSchema,
} from "../src/plugins/config-schema.ts";

test("wrapIndex wraps at both ends and normalizes a raw index", () => {
  // The cycle: one step forward off the last lands on the first, and one step
  // back off the first lands on the last. A single `%` would return `-1` for
  // the backward step, so this is the double modulo.
  assert.equal(wrapIndex(2, 1, 3), 0, "past the last wraps to the first");
  assert.equal(wrapIndex(0, -1, 3), 2, "before the first wraps to the last");
  assert.equal(wrapIndex(1, 1, 3), 2, "an interior step stays interior");
  assert.equal(wrapIndex(1, -1, 3), 0, "an interior step stays interior backward");
  // A raw index is folded into `[0, length)` before the step — the file-drop
  // switcher's reference behaviour.
  assert.equal(wrapIndex(-1, 0, 3), 2, "a negative index normalizes");
  assert.equal(wrapIndex(7, 0, 3), 1, "a past-the-end index normalizes");
  assert.equal(wrapIndex(Number.NaN, 0, 3), 0, "a non-finite index is zero");
  assert.equal(wrapIndex(1.9, 0, 3), 1, "a fractional index truncates");
});

test("wrapIndex is the rule the file-drop switcher and the filter axis follow", () => {
  // The file-drop switcher is the reference: clampActionIndex is a step of 0
  // and nextFileActionIndex is a step of ±1.
  for (const index of [-2, -1, 0, 1, 2, 3, 4, 7, Number.NaN]) {
    assert.equal(clampActionIndex(index), wrapIndex(index, 0, 3), `clamp ${index}`);
    assert.equal(nextFileActionIndex(index, 1), wrapIndex(index, 1, 3), `next +1 ${index}`);
    assert.equal(nextFileActionIndex(index, -1), wrapIndex(index, -1, 3), `next -1 ${index}`);
  }
  // The filter axis is the same cycle over its own values.
  const axis = pluginFilterAxis(["a", "b", "c"] as const, {
    a: "plugins.config.enabled",
    b: "plugins.config.enabled",
    c: "plugins.config.enabled",
  });
  for (let index = 0; index < axis.values.length; index += 1) {
    const current = axis.values[index];
    assert.equal(cyclePluginFilter(axis, current, 1), axis.values[wrapIndex(index, 1, 3)]);
    assert.equal(cyclePluginFilter(axis, current, -1), axis.values[wrapIndex(index, -1, 3)]);
  }
});

test("commandRow carries the nine fields every command row shares", () => {
  const fields: CommandRowFields = {
    id: "row-1",
    title: "title",
    subtitle: "subtitle",
    warnings: ["unavailable"],
    sourceName: "source",
    commandLine: "cmd",
    execution: null,
    completion: false,
  };
  const row = commandRow(fields);
  assert.equal(row.type, "command");
  assert.deepEqual(Object.keys(row), [
    "type",
    "id",
    "title",
    "subtitle",
    "warnings",
    "sourceName",
    "commandLine",
    "execution",
    "completion",
  ]);
  assert.deepEqual(row, { type: "command", ...fields });
});

test("the config field builders are the shapes the two plugins share", () => {
  const clipboard = CLIPBOARD_CONFIG_SCHEMA.fields;
  const calculator = CALCULATOR_CONFIG_SCHEMA.fields;
  const browser = browserConfigSchema().fields;

  // `enabled`: the browser names a section, the clipboard's default group does not.
  const clipEnabled = clipboard.find((field) => field.key === "enabled");
  const browserEnabled = browser.find((field) => field.key === "enabled");
  assert.equal(clipEnabled?.type, "toggle");
  assert.equal(browserEnabled?.type, "toggle");
  assert.equal(clipEnabled?.sectionKey, undefined);
  assert.equal(browserEnabled?.sectionKey, "plugins.config.sectionGeneral");
  assert.equal(clipEnabled?.labelKey, browserEnabled?.labelKey);

  // `max_items`: same control, step and unit; the range and words are the plugin's.
  const clipMax = clipboard.find((field) => field.key === "max_items");
  const calcMax = calculator.find((field) => field.key === "max_items");
  assert.equal(clipMax?.type, "slider");
  assert.equal(calcMax?.type, "slider");
  if (clipMax?.type === "slider" && calcMax?.type === "slider") {
    assert.equal(clipMax.step, 10, "the capacity slider steps by ten");
    assert.equal(clipMax.unitKey, "plugins.config.unitItems", "the capacity slider's unit");
    assert.equal(calcMax.step, clipMax.step, "both sliders step by the same amount");
    assert.equal(calcMax.unitKey, clipMax.unitKey, "both sliders share the unit word");
    assert.notEqual(clipMax.labelKey, calcMax.labelKey, "the words are the plugin's own");
  }

  // `clear_history`: same label / confirm / cancel, different failure and command.
  const clipClear = clipboard.find((field) => field.key === "clear_history");
  const calcClear = calculator.find((field) => field.key === "clear_history");
  assert.equal(clipClear?.type, "action");
  assert.equal(calcClear?.type, "action");
  if (clipClear?.type === "action" && calcClear?.type === "action") {
    assert.equal(clipClear.labelKey, calcClear.labelKey);
    assert.equal(clipClear.confirmKey, calcClear.confirmKey);
    assert.equal(clipClear.cancelKey, calcClear.cancelKey);
    assert.notEqual(clipClear.failedKey, calcClear.failedKey);
    assert.notEqual(clipClear.command, calcClear.command);
  }
});
