// R50 · the built-in calculator plugin.
//
// Three layers are pinned here:
//
//   1. the pure evaluation core (`src/calculator.ts`) — evaluation, result
//      formatting, settings normalization, retention and the Enter rule;
//   2. the mode's rows and the shared two-step delete state machine
//      (`plugins/calculator/mode.ts`, `plugins/history-actions.ts`); and
//   3. the cross-language constants, read out of `commands/config.rs` so the
//      Node defaults and the Rust normalizer cannot drift apart.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  CALCULATOR_COPY_MODES,
  CALCULATOR_MAX_EXPRESSION_LENGTH,
  CALCULATOR_RETENTION_DAYS,
  DEFAULT_CALCULATOR_COPY_MODE,
  DEFAULT_CALCULATOR_MAX_ITEMS,
  DEFAULT_CALCULATOR_RETENTION_DAYS,
  MAX_CALCULATOR_MAX_ITEMS,
  MIN_CALCULATOR_MAX_ITEMS,
  calculatorCopyText,
  calculatorEnterAction,
  calculatorEntryMatchesFilter,
  calculatorTime,
  clampCalculatorMaxItems,
  evaluateExpression,
  formatCalculatorResult,
  normalizeCalculatorCopyMode,
  normalizeCalculatorEntries,
  normalizeCalculatorRetentionDays,
  normalizeCalculatorSettings,
  pruneCalculatorEntries,
  type CalculatorEntry,
  type CalculatorPluginSettings,
} from "../src/calculator.ts";
import {
  CALCULATOR_FILTERS,
  calculatorModeFor,
  cycleCalculatorFilter,
  parseCalculatorMode,
  pluginModeEntry,
} from "../src/launcher.ts";
import { calculatorModeRows } from "../src/plugins/calculator/mode.ts";
import {
  HISTORY_DELETE_CONFIRM_MS,
  reduceHistoryDelete,
  selectionAfterRemoval,
} from "../src/plugins/history-actions.ts";
import { CALCULATOR_CONFIG_SCHEMA, configDefaults, pluginConfigSchema } from "../src/plugins/config-schema.ts";
import { CALCULATOR_PLUGIN_ID } from "../src/plugin-pages.ts";

const t = ((key: string) => key) as unknown as (key: never) => string;
const DAY = 24 * 60 * 60 * 1000;

const entry = (
  id: string,
  expression: string,
  result: string,
  created_at: number,
  favorite = false,
): CalculatorEntry => ({ id, expression, result, created_at, favorite });

// ── 1 · evaluation ─────────────────────────────────────────────────────────

test("the evaluation core is the arithmetic a hand calculator writes", () => {
  const equal = (expression: string, value: number) => {
    const result = evaluateExpression(expression);
    assert.equal(result.ok, true, `${expression} should evaluate`);
    if (result.ok) assert.equal(result.value, value, expression);
  };
  equal("2+3*4", 14);
  equal("(2+3)*4", 20);
  equal("2^10", 1024);
  equal("10/4", 2.5);
  equal("7%3", 1);
  equal("5!", 120);
  equal("sqrt(16)", 4);
  equal("abs(-3)", 3);
  equal("round(2.5)", 3);
  equal("sin(PI/2)", 1);
  equal("cos(0)", 1);
  assert.equal(evaluateExpression("E").ok, true);
});

test("the evaluation core refuses empty, malformed, too-long and non-finite input", () => {
  assert.deepEqual(evaluateExpression("   "), { ok: false, errorKey: "calculator.error.empty" });
  const invalid = evaluateExpression("2+");
  assert.equal(invalid.ok, false);
  if (!invalid.ok) assert.equal(invalid.errorKey, "calculator.error.invalid");
  // Division by zero is Infinity, sqrt(-1) is NaN: neither is a result.
  for (const expression of ["1/0", "0/0", "sqrt(-1)"]) {
    const result = evaluateExpression(expression);
    assert.equal(result.ok, false, expression);
    if (!result.ok) assert.equal(result.errorKey, "calculator.error.notFinite");
  }
  const tooLong = evaluateExpression("1+".repeat(CALCULATOR_MAX_EXPRESSION_LENGTH) + "1");
  assert.equal(tooLong.ok, false);
  if (!tooLong.ok) assert.equal(tooLong.errorKey, "calculator.error.tooLong");
});

test("the hardened parser cannot assign, compare or reach a property", () => {
  // The writing and reaching operators are off, so an expression cannot leave a
  // variable behind or read an object. All of these are parse errors.
  for (const expression of ["x=3", "(1).constructor", "a.b", "1<2", "true?1:2", "random()"]) {
    const result = evaluateExpression(expression);
    assert.equal(result.ok, false, expression);
  }
});

test("results print integers bare, round float fuzz, and leave the band for scientific notation", () => {
  assert.equal(formatCalculatorResult(14), "14");
  assert.equal(formatCalculatorResult(-7), "-7");
  // 0.1 + 0.2 is 0.30000000000000004 before rounding.
  assert.equal(formatCalculatorResult(0.1 + 0.2), "0.3");
  assert.equal(formatCalculatorResult(1 / 3), "0.333333333333");
  assert.equal(formatCalculatorResult(1e15), "1e+15");
  assert.equal(formatCalculatorResult(1.5e20), "1.5e+20");
  assert.equal(formatCalculatorResult(1.2345e-12), "1.2345e-12");
  // Zero is never treated as "below the low band".
  assert.equal(formatCalculatorResult(0), "0");
});

// ── 2 · settings ───────────────────────────────────────────────────────────

test("settings normalize: capacity clamps, window and copy mode snap to their vocabulary", () => {
  assert.deepEqual(normalizeCalculatorSettings({}), {
    max_items: DEFAULT_CALCULATOR_MAX_ITEMS,
    retention_days: DEFAULT_CALCULATOR_RETENTION_DAYS,
    copy_mode: DEFAULT_CALCULATOR_COPY_MODE,
  });
  assert.equal(clampCalculatorMaxItems(1), MIN_CALCULATOR_MAX_ITEMS);
  assert.equal(clampCalculatorMaxItems(9999), MAX_CALCULATOR_MAX_ITEMS);
  assert.equal(clampCalculatorMaxItems(Number.NaN), DEFAULT_CALCULATOR_MAX_ITEMS);
  assert.equal(clampCalculatorMaxItems(100.6), 101);
  assert.equal(normalizeCalculatorRetentionDays(7), 7);
  assert.equal(normalizeCalculatorRetentionDays(5), DEFAULT_CALCULATOR_RETENTION_DAYS);
  assert.equal(normalizeCalculatorCopyMode("result"), "result");
  assert.equal(normalizeCalculatorCopyMode("nonsense"), DEFAULT_CALCULATOR_COPY_MODE);
  assert.deepEqual(normalizeCalculatorSettings({ max_items: 3, retention_days: 1, copy_mode: "result" }), {
    max_items: MIN_CALCULATOR_MAX_ITEMS,
    retention_days: 1,
    copy_mode: "result",
  });
  assert.deepEqual(normalizeCalculatorEntries([{ id: "a", expression: "1+1" }, { nope: true }]), [
    { id: "a", expression: "1+1", result: "", created_at: 0, favorite: false },
  ]);
});

// ── 3 · retention ──────────────────────────────────────────────────────────

test("the age window drops non-favorites and keeps favorites", () => {
  const now = 100 * DAY;
  const settings: CalculatorPluginSettings = { max_items: 100, retention_days: 30, copy_mode: "full" };
  const { kept, dropped } = pruneCalculatorEntries(
    [
      entry("fav-old", "1+1", "2", now - 40 * DAY, true),
      entry("old", "1+2", "3", now - 40 * DAY),
      entry("fresh", "1+3", "4", now - 2 * DAY),
    ],
    now,
    settings,
  );
  assert.deepEqual(kept.map((row) => row.id), ["fav-old", "fresh"]);
  assert.deepEqual(dropped.map((row) => row.id), ["old"]);
});

test("a full and expired history drops the rest and keeps the favorite (the round's red line)", () => {
  const now = 100 * DAY;
  const settings: CalculatorPluginSettings = { max_items: 1, retention_days: 30, copy_mode: "full" };
  const { kept, dropped } = pruneCalculatorEntries(
    [
      entry("fav-old", "1", "1", now - 40 * DAY, true),
      entry("plain-old", "2", "2", now - 40 * DAY),
      entry("plain-a", "3", "3", now - 1000),
      entry("plain-b", "4", "4", now - 900),
      entry("plain-c", "5", "5", now - 800),
    ],
    now,
    settings,
  );
  // Favorites are exempt from BOTH passes: the expired favorite survives and
  // the capacity of one still counts only non-favorites.
  assert.deepEqual(kept.map((row) => row.id), ["fav-old", "plain-c"]);
  assert.equal(dropped.length, 3);
});

test("the window off (0) never expires anything", () => {
  const settings: CalculatorPluginSettings = { max_items: 100, retention_days: 0, copy_mode: "full" };
  const { kept, dropped } = pruneCalculatorEntries([entry("old", "1", "1", 0)], 1000 * DAY, settings);
  assert.equal(kept.length, 1);
  assert.equal(dropped.length, 0);
});

// ── 4 · the row's time and the copy semantics ──────────────────────────────

test("a row's time is a clock today, the word yesterday, a date beyond", () => {
  const now = new Date(2026, 2, 14, 15, 30, 0).getTime();
  assert.deepEqual(calculatorTime(new Date(2026, 2, 14, 9, 5, 0).getTime(), now), {
    kind: "time",
    text: "09:05",
  });
  assert.deepEqual(calculatorTime(new Date(2026, 2, 13, 23, 0, 0).getTime(), now), {
    kind: "yesterday",
  });
  assert.deepEqual(calculatorTime(new Date(2026, 2, 1, 0, 0, 0).getTime(), now), {
    kind: "date",
    text: "03-01",
  });
  assert.deepEqual(calculatorTime(new Date(2025, 11, 31, 0, 0, 0).getTime(), now), {
    kind: "date",
    text: "2025-12-31",
  });
});

test("copy text is the whole line in full mode and the number alone in result mode", () => {
  const row = entry("a", "2+3", "5", 0);
  assert.equal(calculatorCopyText(row, "full"), "2+3 = 5");
  assert.equal(calculatorCopyText(row, "result"), "5");
});

// ── 5 · the Enter rule and the filter axis ─────────────────────────────────

test("Enter evaluates a fresh expression and runs the selected row once it is evaluated", () => {
  assert.equal(calculatorEnterAction("2+3", null), "evaluate");
  assert.equal(calculatorEnterAction("2+3", "2+3"), "run");
  assert.equal(calculatorEnterAction("2+4", "2+3"), "evaluate");
  assert.equal(calculatorEnterAction("   ", "2+3"), "run");
  assert.equal(calculatorEnterAction("", null), "run");
});

test("the calculator mode enters on its trigger word and cycles its two chips", () => {
  assert.deepEqual(parseCalculatorMode("calc 2+3"), { needle: "2+3", filter: "all" });
  assert.deepEqual(parseCalculatorMode("计算器 "), { needle: "", filter: "all" });
  assert.deepEqual(parseCalculatorMode("= 1+1"), { needle: "1+1", filter: "all" });
  // The bare word is not an entry — the space is what makes the mode deliberate.
  assert.equal(parseCalculatorMode("calc"), null);
  assert.deepEqual(pluginModeEntry("calc 2+3"), {
    mode: { scope: "calculator", filter: "all" },
    needle: "2+3",
  });
  assert.deepEqual(calculatorModeFor({ scope: "calculator", filter: "favorites" }, " 1+1 "), {
    needle: "1+1",
    filter: "favorites",
  });
  assert.equal(calculatorModeFor({ scope: "clipboard", filter: "all" }, "x"), null);
  assert.deepEqual([...CALCULATOR_FILTERS], ["all", "favorites"]);
  assert.equal(cycleCalculatorFilter("all", 1), "favorites");
  assert.equal(cycleCalculatorFilter("favorites", 1), "all");
  assert.equal(cycleCalculatorFilter("all", -1), "favorites");
});

test("the two chips keep everything or only favorites", () => {
  const plain = entry("a", "1+1", "2", 0);
  const starred = entry("b", "2+2", "4", 0, true);
  assert.equal(calculatorEntryMatchesFilter(plain, "all"), true);
  assert.equal(calculatorEntryMatchesFilter(plain, "favorites"), false);
  assert.equal(calculatorEntryMatchesFilter(starred, "favorites"), true);
});

test("mode rows filter by chip and the list is never narrowed by the expression", () => {
  const now = Date.now();
  const all = [entry("a", "1+1", "2", now), entry("b", "2+2", "4", now, true)];
  const rows = calculatorModeRows(all, { needle: "", filter: "favorites" }, t, now);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].id, "b");
  assert.equal(rows[0].family, "calculator");
  assert.equal(rows[0].entry?.result, "4");
  // R50 · the field's text is the pending expression, not a search needle: a
  // fresh expression must not hide the history the user is about to add to.
  const whileTyping = calculatorModeRows(all, { needle: "1+", filter: "all" }, t, now);
  assert.deepEqual(whileTyping.map((row) => row.id), ["a", "b"]);
  // Two distinct empty states.
  const empty = calculatorModeRows([], { needle: "", filter: "all" }, t, now);
  assert.equal(empty[0].kind, "status");
  assert.equal(empty[0].id, "calculator-empty");
  assert.equal(empty[0].title, "calculator.empty");
  const noFavorites = calculatorModeRows([entry("a", "1+1", "2", now)], { needle: "", filter: "favorites" }, t, now);
  assert.equal(noFavorites[0].title, "calculator.emptyFavorites");
});

// ── 6 · the shared two-step delete ─────────────────────────────────────────

test("the delete confirm arms on the first press and fires on the second inside the window", () => {
  const armed = reduceHistoryDelete(null, { type: "press", id: "a", now: 1000 });
  assert.equal(armed.confirm, null);
  assert.deepEqual(armed.state, { id: "a", armedAt: 1000 });
  const confirmed = reduceHistoryDelete(armed.state, { type: "press", id: "a", now: 2000 });
  assert.equal(confirmed.confirm, "a");
  assert.equal(confirmed.state, null);
});

test("pressing a different row re-arms it; the confirmation never transfers", () => {
  const armed = reduceHistoryDelete(null, { type: "press", id: "a", now: 1000 });
  const other = reduceHistoryDelete(armed.state, { type: "press", id: "b", now: 1100 });
  assert.equal(other.confirm, null);
  assert.deepEqual(other.state, { id: "b", armedAt: 1100 });
});

test("a stale arm expires and a fresh one is untouched; cancel always clears", () => {
  const armed = reduceHistoryDelete(null, { type: "press", id: "a", now: 1000 });
  const stale = reduceHistoryDelete(armed.state, {
    type: "expire",
    now: 1000 + HISTORY_DELETE_CONFIRM_MS + 1,
  });
  assert.equal(stale.state, null);
  const fresh = reduceHistoryDelete(armed.state, { type: "expire", now: 2000 });
  assert.notEqual(fresh.state, null);
  assert.equal(reduceHistoryDelete(armed.state, { type: "cancel" }).state, null);
});

test("the selection hands over to the neighbour, never to the top", () => {
  assert.equal(selectionAfterRemoval(0, 3), 0);
  assert.equal(selectionAfterRemoval(1, 3), 1);
  assert.equal(selectionAfterRemoval(2, 3), 1);
  assert.equal(selectionAfterRemoval(0, 1), 0);
});

// ── 7 · the schema and the cross-language constants ────────────────────────

test("the calculator schema is registered and holds the four controls", () => {
  assert.equal(pluginConfigSchema(CALCULATOR_PLUGIN_ID), CALCULATOR_CONFIG_SCHEMA);
  assert.deepEqual(
    CALCULATOR_CONFIG_SCHEMA.fields.map((field) => `${field.key}:${field.type}`),
    ["max_items:slider", "retention_days:select", "copy_mode:radio", "clear_history:action"],
  );
  const defaults = configDefaults(CALCULATOR_CONFIG_SCHEMA);
  assert.equal(defaults.max_items, DEFAULT_CALCULATOR_MAX_ITEMS);
  assert.equal(defaults.retention_days, String(DEFAULT_CALCULATOR_RETENTION_DAYS));
  assert.equal(defaults.copy_mode, DEFAULT_CALCULATOR_COPY_MODE);
  const action = CALCULATOR_CONFIG_SCHEMA.fields.find((field) => field.type === "action");
  assert.equal(action && action.type === "action" ? action.command : null, "calculator_clear_history");
});

test("the Node constants match the Rust normalizer's tables", async () => {
  const rust = await readFile("src-tauri/src/commands/config.rs", "utf8");
  assert.deepEqual([...CALCULATOR_RETENTION_DAYS], [0, 1, 7, 30]);
  assert.match(rust, /pub const CALCULATOR_RETENTION_DAYS: \[u32; 4\] = \[0, 1, 7, 30\];/);
  assert.equal(DEFAULT_CALCULATOR_RETENTION_DAYS, 30);
  assert.match(rust, /pub const DEFAULT_CALCULATOR_RETENTION_DAYS: u32 = 30;/);
  assert.deepEqual([...CALCULATOR_COPY_MODES], ["full", "result"]);
  assert.match(rust, /pub const CALCULATOR_COPY_MODES: \[&str; 2\] = \["full", "result"\];/);
  assert.equal(DEFAULT_CALCULATOR_COPY_MODE, "full");
  assert.match(rust, /pub const DEFAULT_CALCULATOR_COPY_MODE: &str = "full";/);
  assert.equal(DEFAULT_CALCULATOR_MAX_ITEMS, 100);
  assert.match(rust, /pub const DEFAULT_CALCULATOR_MAX_ITEMS: u32 = 100;/);
  assert.match(rust, /pub const MIN_CALCULATOR_MAX_ITEMS: u32 = 10;/);
  assert.match(rust, /pub const MAX_CALCULATOR_MAX_ITEMS: u32 = 500;/);
});

test("the calculator's store commands are registered and the settings block round-trips", async () => {
  const lib = await readFile("src-tauri/src/lib.rs", "utf8");
  for (const command of [
    "calculator_get_entries",
    "calculator_add_entry",
    "calculator_set_favorite",
    "calculator_delete",
    "calculator_clear_history",
    "calculator_get_settings",
    "calculator_set_settings",
  ]) {
    assert.match(lib, new RegExp(`calculator_history::${command}`), command);
  }
  const config = await readFile("src-tauri/src/commands/config.rs", "utf8");
  // The settings block is a field of AppSettings, normalized and owned by its
  // own command (a whole-app save must not revert it).
  assert.match(config, /pub calculator_plugin: CalculatorPluginSettings,/);
  assert.match(config, /submitted\.calculator_plugin = stored\.calculator_plugin\.clone\(\);/);
  assert.match(config, /pub fn write_calculator_settings\(/);
});
