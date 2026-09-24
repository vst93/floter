// R50 · the calculator plugin's own vocabulary: what an expression evaluates
// to, how a result is printed, how a history entry is shaped, and what the
// history's retention policy keeps.
//
// This is the calculator's `clipboard-history.ts`: the model and every pure
// decision the plugin makes, with no React, no Tauri and no DOM, so the node
// suite can pin them directly. The *rows* the launcher draws live in
// `plugins/calculator/mode.ts`, on top of this module, exactly as the
// clipboard's do.
//
// ## Why `expr-eval`
//
// The user asked for a mature open-source evaluation core rather than a
// hand-written parser (「核心逻辑和计算逻辑最好复用成熟的开源包」). `expr-eval`
// is a small, dependency-free, pure-JS expression evaluator: four rules, powers,
// parentheses, factorials and the usual `sqrt`/`sin`/`cos`/`log`/`abs` family,
// with `PI`/`E` constants. It ships no transitive dependencies and no I/O.
//
// Its published advisories concern callers that hand *attacker-controlled*
// scopes or functions to `evaluate` (and the `toJSFunction` code-generator,
// which this module never touches). This module closes that door by
// construction:
//
//   · a **hardened parser instance** with member access and the assignment /
//     function-definition / comparison / logical / concatenation / random
//     operators switched off, so an expression is arithmetic and named built-in
//     functions only — never a variable write or a property read;
//   · **no scope and no custom functions** are ever passed to `evaluate`;
//   · a **length cap** on the expression before it reaches the parser; and
//   · the result must be a **finite number**, so `NaN`, `Infinity` and a
//     string result are refused (a refused result is an error line, never a
//     stored entry).
//
// With those, the untrusted input is a short arithmetic string evaluated by a
// parser that cannot reach the host, which is the whole of what this plugin
// needs.
//
// ## Formatting
//
// Floating-point arithmetic is inexact (`0.1 + 0.2` is `0.30000000000000004`),
// so a raw `String(value)` reads as a bug. Every result is first rounded to
// `CALCULATOR_PRECISION` significant digits and only then printed: integers
// print bare, ordinary decimals print without a trailing `.0`, and values that
// leave the comfortable band (`>= 1e15` or `< 1e-9`) print in scientific
// notation with their trailing zeros trimmed. The precision/band choice is
// shrunk below; the point is that the *policy* is here and testable, not
// scattered through a renderer.

import { Parser } from "expr-eval";
import type { MessageKey } from "./i18n.ts";

// ── the result printing policy ──────────────────────────────────────────────

/** Significant digits a result is rounded to before printing. Twelve is past
 *  the point where IEEE-754 fuzz stops mattering for a hand calculator and
 *  short of the 17 that would print the fuzz. */
export const CALCULATOR_PRECISION = 12;

/** At or above this magnitude a plain decimal stops being readable (and
 *  `String` starts reaching for `e` notation itself, inconsistently). */
export const CALCULATOR_SCIENTIFIC_HIGH = 1e15;

/** Below this magnitude a plain decimal is a run of leading zeros; scientific
 *  notation reads it in one glance. Zero is exempt. */
export const CALCULATOR_SCIENTIFIC_LOW = 1e-9;

/** Trim the trailing zeros of a mantissa, leaving `1e+20` from
 *  `1.00000000000e+20`. Applied only to the output of `toExponential`. */
const trimExponential = (text: string): string =>
  text.replace(/\.?0+e/, "e");

/**
 * The printed form of one numeric result. See the module header for the
 * precision and the two scientific-notation thresholds. A non-finite value has
 * no printed form and returns an empty string — the caller refuses it before
 * this point (see {@link evaluateExpression}).
 */
export const formatCalculatorResult = (value: number): string => {
  if (!Number.isFinite(value)) return "";
  const magnitude = Math.abs(value);
  if (magnitude !== 0 && (magnitude >= CALCULATOR_SCIENTIFIC_HIGH || magnitude < CALCULATOR_SCIENTIFIC_LOW)) {
    return trimExponential(value.toExponential(CALCULATOR_PRECISION - 1));
  }
  const rounded = Number(value.toPrecision(CALCULATOR_PRECISION));
  // `String` already prints an integer-looking float without a fraction
  // (`String(5)` is `"5"`); the branch exists so a rounded value inside the
  // safe-integer band is guaranteed bare rather than whatever `String` decides.
  if (Number.isInteger(rounded) && Math.abs(rounded) <= Number.MAX_SAFE_INTEGER) {
    return String(rounded);
  }
  return String(rounded);
};

// ── evaluation ──────────────────────────────────────────────────────────────

/** The longest expression the parser is handed. A calculator expression is
 *  short; the cap is the cheap denial-of-service guard for a hostile paste. */
export const CALCULATOR_MAX_EXPRESSION_LENGTH = 256;

/**
 * The hardened parser. Built once (parsing options are fixed) and reused: the
 * instance holds no per-expression state, because every writing operator is
 * off and `evaluate` is called with no scope.
 */
const hardenedParser = new Parser({
  allowMemberAccess: false,
  operators: {
    assignment: false,
    fndef: false,
    in: false,
    random: false,
    length: false,
    concatenate: false,
    conditional: false,
    logical: false,
    comparison: false,
  },
});
// `random` is an enabled-by-default *function* as well as an operator; a hand
// calculator has no use for it and its output is not reproducible, so it goes
// too.
delete (hardenedParser.functions as Record<string, unknown>).random;

/** A successful evaluation: the numeric value and its printed form. */
export type CalculatorEvaluation =
  | { readonly ok: true; readonly value: number; readonly formatted: string }
  | { readonly ok: false; readonly errorKey: MessageKey };

/**
 * Evaluate one expression to a finite number.
 *
 * The three refusals are distinct so the status line can say what actually
 * went wrong: an empty field, an expression the parser cannot read, and an
 * expression whose value is not a finite number (`1/0`, `sqrt(-1)`, `0/0`).
 * A refused evaluation stores nothing and never throws.
 */
export const evaluateExpression = (expression: string): CalculatorEvaluation => {
  const trimmed = expression.trim();
  if (!trimmed) return { ok: false, errorKey: "calculator.error.empty" };
  if (trimmed.length > CALCULATOR_MAX_EXPRESSION_LENGTH) {
    return { ok: false, errorKey: "calculator.error.tooLong" };
  }
  let value: unknown;
  try {
    value = hardenedParser.parse(trimmed).evaluate();
  } catch {
    return { ok: false, errorKey: "calculator.error.invalid" };
  }
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return { ok: false, errorKey: "calculator.error.notFinite" };
  }
  return { ok: true, value, formatted: formatCalculatorResult(value) };
};

// ── the history entry ───────────────────────────────────────────────────────

/** One recorded calculation, mirroring `CalculatorEntry` in Rust. */
export type CalculatorEntry = {
  id: string;
  /** The expression exactly as the user typed it (trimmed on entry). */
  expression: string;
  /** The printed result, the string Enter copies in `result` mode. */
  result: string;
  /** Unix timestamp in milliseconds. */
  created_at: number;
  /** Favorites are exempt from both the count cap and the age window. */
  favorite: boolean;
};

/** Coerce one stored row, dropping a row with no identity or expression. A
 *  result is allowed to be empty (an old row from before a format change). */
export const normalizeCalculatorEntry = (raw: unknown): CalculatorEntry | null => {
  if (typeof raw !== "object" || raw === null) return null;
  const record = raw as Record<string, unknown>;
  const id = typeof record.id === "string" ? record.id : "";
  const expression = typeof record.expression === "string" ? record.expression : "";
  if (!id || !expression) return null;
  return {
    id,
    expression,
    result: typeof record.result === "string" ? record.result : "",
    created_at:
      typeof record.created_at === "number" && Number.isFinite(record.created_at)
        ? record.created_at
        : 0,
    favorite: record.favorite === true,
  };
};

export const normalizeCalculatorEntries = (raw: unknown): CalculatorEntry[] =>
  Array.isArray(raw)
    ? raw.map(normalizeCalculatorEntry).filter((entry): entry is CalculatorEntry => entry !== null)
    : [];

// ── the plugin's settings block ─────────────────────────────────────────────

/** The capacity range, matching `MIN_CALCULATOR_MAX_ITEMS` in Rust. */
export const MIN_CALCULATOR_MAX_ITEMS = 10;
export const MAX_CALCULATOR_MAX_ITEMS = 500;
export const DEFAULT_CALCULATOR_MAX_ITEMS = 100;

/** The age window's vocabulary, in the order the settings select lists it.
 *  `0` means "never expire". */
export const CALCULATOR_RETENTION_DAYS: readonly number[] = [0, 1, 7, 30];
export const DEFAULT_CALCULATOR_RETENTION_DAYS = 30;

/** What Enter copies from a history row: the whole `expression = result` line
 *  or the result alone. */
export const CALCULATOR_COPY_MODES = ["full", "result"] as const;
export type CalculatorCopyMode = (typeof CALCULATOR_COPY_MODES)[number];
export const DEFAULT_CALCULATOR_COPY_MODE: CalculatorCopyMode = "full";

/** The plugin's settings block, mirroring `CalculatorPluginSettings` in Rust.
 *  The three fields are independent: the capacity, the age window (a favorite
 *  is exempt from both) and the copy mode. */
export type CalculatorPluginSettings = {
  max_items: number;
  retention_days: number;
  copy_mode: CalculatorCopyMode;
};

/** Snap a capacity into range. A non-finite value falls back to the default
 *  rather than to a bound, matching the Rust normalizer. */
export const clampCalculatorMaxItems = (value: number): number => {
  if (!Number.isFinite(value)) return DEFAULT_CALCULATOR_MAX_ITEMS;
  return Math.min(
    MAX_CALCULATOR_MAX_ITEMS,
    Math.max(MIN_CALCULATOR_MAX_ITEMS, Math.round(value)),
  );
};

/** Accept one of {@link CALCULATOR_RETENTION_DAYS}; anything else is the
 *  default. */
export const normalizeCalculatorRetentionDays = (value: unknown): number => {
  const parsed = typeof value === "number" ? value : Number(value);
  return CALCULATOR_RETENTION_DAYS.includes(parsed) ? parsed : DEFAULT_CALCULATOR_RETENTION_DAYS;
};

export const normalizeCalculatorCopyMode = (value: unknown): CalculatorCopyMode =>
  CALCULATOR_COPY_MODES.includes(value as CalculatorCopyMode)
    ? (value as CalculatorCopyMode)
    : DEFAULT_CALCULATOR_COPY_MODE;

/** Normalize a whole settings block read from the backend (or `{}` before it
 *  answers). Every field is present and legal. */
export const normalizeCalculatorSettings = (raw: unknown): CalculatorPluginSettings => {
  const record = (typeof raw === "object" && raw !== null ? raw : {}) as Record<string, unknown>;
  return {
    max_items: clampCalculatorMaxItems(
      typeof record.max_items === "number" ? record.max_items : DEFAULT_CALCULATOR_MAX_ITEMS,
    ),
    retention_days: normalizeCalculatorRetentionDays(
      record.retention_days ?? DEFAULT_CALCULATOR_RETENTION_DAYS,
    ),
    copy_mode: normalizeCalculatorCopyMode(record.copy_mode ?? DEFAULT_CALCULATOR_COPY_MODE),
  };
};

// ── retention ───────────────────────────────────────────────────────────────

/** The age cutoff for a window of `days` at `now`; `-Infinity` when the window
 *  is off, so nothing is ever older than it. */
export const calculatorRetentionCutoff = (now: number, days: number): number =>
  days <= 0 ? Number.NEGATIVE_INFINITY : now - days * 24 * 60 * 60 * 1000;

/**
 * Apply the retention policy: drop non-favorite entries older than the window,
 * then the oldest non-favorites beyond the capacity.
 *
 * Favorites are exempt from **both** passes — the user's 「收藏的记录不要清理，
 * 不管是超过条数还是时间」 — so a favorited entry survives a full history and an
 * expired one alike. Entries keep their input order (newest first); the cap is
 * applied by age, not by position, so a hand-edited out-of-order index still
 * prunes the truly oldest. Returns `(kept, dropped)` so a caller can report or
 * persist the difference.
 */
export const pruneCalculatorEntries = (
  entries: readonly CalculatorEntry[],
  now: number,
  settings: CalculatorPluginSettings,
): { kept: CalculatorEntry[]; dropped: CalculatorEntry[] } => {
  const cutoff = calculatorRetentionCutoff(now, settings.retention_days);
  const fresh: CalculatorEntry[] = [];
  const dropped: CalculatorEntry[] = [];
  for (const entry of entries) {
    if (!entry.favorite && entry.created_at < cutoff) dropped.push(entry);
    else fresh.push(entry);
  }

  const nonFavoriteIndices = fresh
    .map((entry, index) => ({ entry, index }))
    .filter(({ entry }) => !entry.favorite)
    .sort((left, right) => left.entry.created_at - right.entry.created_at);
  const excess = Math.max(0, nonFavoriteIndices.length - settings.max_items);
  const doomed = new Set(nonFavoriteIndices.slice(0, excess).map(({ index }) => index));

  const kept: CalculatorEntry[] = [];
  fresh.forEach((entry, index) => {
    if (doomed.has(index)) dropped.push(entry);
    else kept.push(entry);
  });
  return { kept, dropped };
};

// ── copy semantics ──────────────────────────────────────────────────────────

/**
 * What Enter copies from a history row.
 *
 * `full` is the record as the user thinks of it — `expression = result` —
 * and `result` is the bare number, for pasting into something that only wants
 * the value. The two are the setting's whole meaning, so they live here.
 */
export const calculatorCopyText = (
  entry: CalculatorEntry,
  mode: CalculatorCopyMode,
): string => (mode === "result" ? entry.result : `${entry.expression} = ${entry.result}`);

// ── the row's time ──────────────────────────────────────────────────────────

/** The row's compact time: a clock for today, the word for yesterday, a date
 *  beyond that. `yesterday` is left for the caller to translate. */
export type CalculatorTime =
  | { readonly kind: "time"; readonly text: string }
  | { readonly kind: "yesterday" }
  | { readonly kind: "date"; readonly text: string };

const pad2 = (value: number): string => String(value).padStart(2, "0");

const sameCalendarDay = (a: Date, b: Date): boolean =>
  a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();

export const calculatorTime = (createdAtMs: number, nowMs: number): CalculatorTime => {
  if (!Number.isFinite(createdAtMs) || !Number.isFinite(nowMs)) {
    return { kind: "date", text: "" };
  }
  const created = new Date(createdAtMs);
  const now = new Date(nowMs);
  if (sameCalendarDay(created, now)) {
    return { kind: "time", text: `${pad2(created.getHours())}:${pad2(created.getMinutes())}` };
  }
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (sameCalendarDay(created, yesterday)) return { kind: "yesterday" };
  const text = created.getFullYear() === now.getFullYear()
    ? `${pad2(created.getMonth() + 1)}-${pad2(created.getDate())}`
    : `${created.getFullYear()}-${pad2(created.getMonth() + 1)}-${pad2(created.getDate())}`;
  return { kind: "date", text };
};

// ── the mode's request ──────────────────────────────────────────────────────

/** The calculator mode's two filter chips: everything, or starred entries. */
export type CalculatorModeFilter = "all" | "favorites";

/**
 * Whether an entry survives one chip. `all` keeps everything; `favorites` is
 * the orthogonal retention axis (any entry may be starred), and a future type
 * axis would sit beside it rather than replacing it — a starred entry would
 * then appear under both. This round ships only the two.
 */
export const calculatorEntryMatchesFilter = (
  entry: CalculatorEntry,
  filter: CalculatorModeFilter,
): boolean => (filter === "favorites" ? entry.favorite : true);

/**
 * Whether Enter in the calculator mode should *evaluate* the field or *run*
 * the selected history row.
 *
 * The field is both the expression and the list's search box, so Enter has to
 * choose. The rule: an expression that has not been evaluated yet is
 * evaluated; once it has, Enter belongs to the selected row (copy). Editing the
 * expression arms evaluation again. This is a pure decision so the node suite
 * can drive the whole matrix; the App holds the one piece of state it reads
 * (`lastEvaluated`).
 */
export const calculatorEnterAction = (
  expression: string,
  lastEvaluated: string | null,
): "evaluate" | "run" => {
  const trimmed = expression.trim();
  return trimmed.length > 0 && trimmed !== lastEvaluated ? "evaluate" : "run";
};
