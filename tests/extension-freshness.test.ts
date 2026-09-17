import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createTranslator } from "../src/i18n.ts";
import {
  commandDelta,
  createReprobeNoticeGate,
  decideDriftNotice,
  freshnessOf,
  relativeTime,
  reprobeNoticeKey,
  unixSeconds,
} from "../src/extensions/freshness.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");

const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^\s*\/\/.*$/gm, "");

/** The body of a declaration, taken by brace matching — the same discipline the
 *  R7-3a tests use. Slicing to "the next declaration we happen to know" silently
 *  changes meaning when a neighbour is renamed. */
const declarationBody = (source: string, signature: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const open = source.indexOf("{", at);
  assert.notEqual(open, -1, `declaration without a body: ${signature}`);
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}") {
      depth -= 1;
      if (depth === 0) return source.slice(open + 1, i);
    }
  }
  assert.fail(`unterminated declaration: ${signature}`);
};

/**
 * The JSX of a function component, from its signature to the end of the
 * component's return block.
 *
 * A component's signature is followed by a destructured-props brace, so brace
 * matching from the signature stops at the props — this takes the *last* `}`
 * before the next top-level declaration instead, anchored on that neighbour
 * being present and after the signature (so a rename fails loudly rather than
 * silently widening the slice).
 */
const componentBody = (source: string, signature: string, next: string) => {
  const at = source.indexOf(signature);
  assert.notEqual(at, -1, `missing declaration: ${signature}`);
  const end = source.indexOf(next, at + signature.length);
  assert.notEqual(end, -1, `missing the declaration after: ${next}`);
  const body = source.slice(at, end);
  assert.ok(body.includes("return ("), `${signature} must contain a JSX return`);
  return body;
};

// ── A. the three freshness facts ───────────────────────────────────────────
//
// The section exists to answer "how fresh is the command list I have" with
// three facts: when it was last probed, what that probe did, and how the command
// count moved. Each of the three has to survive on its own — a row with only two
// of them is a bug, not a partial success.

test("freshness projects the probe timestamp, result, and command delta together", () => {
  const freshness = freshnessOf({
    lastProbeAt: 1_700_000_000,
    healthCheckedAt: null,
    healthStatus: null,
    errorCode: null,
    running: false,
    commandCount: 4,
    previousCommandCount: 2,
  });
  assert.equal(freshness.atSeconds, 1_700_000_000, "the probe timestamp is reported");
  assert.equal(freshness.source, "probe", "the sidecar outranks the health report");
  assert.equal(freshness.result, "success");
  assert.deepEqual(freshness.delta, { kind: "increase", count: 2 });
});

test("the command delta distinguishes growth, shrinkage, no change, and unknown", () => {
  assert.deepEqual(commandDelta(2, 5), { kind: "increase", count: 3 });
  assert.deepEqual(commandDelta(5, 2), { kind: "decrease", count: 3 });
  assert.deepEqual(commandDelta(3, 3), { kind: "unchanged", count: 0 });
  // "No previous probe" and "no current count" are both unknown — never
  // "unchanged", which would claim a comparison that never happened.
  assert.deepEqual(commandDelta(null, 3), { kind: "unknown", count: null });
  assert.deepEqual(commandDelta(3, null), { kind: "unknown", count: null });
  assert.deepEqual(commandDelta(undefined, undefined), { kind: "unknown", count: null });
});

// ── B. graceful absence: unknown is never a zero ───────────────────────────

test("an integration that was never probed reports unknown, not zeros", () => {
  const freshness = freshnessOf({});
  assert.equal(freshness.atSeconds, null, "no timestamp means no timestamp");
  assert.equal(freshness.source, "none");
  assert.equal(freshness.result, "unknown");
  assert.equal(freshness.commandCount, null);
  assert.equal(freshness.previousCommandCount, null);
  assert.equal(freshness.delta.kind, "unknown");
  // The trap this guards: `0` is falsy, so a `|| 0` fallback anywhere in the
  // projection would silently turn "we do not know" into "zero commands".
  assert.notEqual(freshness.commandCount, 0);
});

test("a health report is the fallback source and keeps its own status", () => {
  const degraded = freshnessOf({
    lastProbeAt: null,
    healthCheckedAt: "2024-01-01T00:00:00Z",
    healthStatus: "degraded",
  });
  assert.equal(degraded.source, "health");
  assert.equal(degraded.result, "degraded", "a degraded probe is neither success nor failure");
  assert.equal(degraded.atSeconds, unixSeconds("2024-01-01T00:00:00Z"));

  const failed = freshnessOf({
    lastProbeAt: 1_700_000_000,
    errorCode: "tool-error",
    healthStatus: null,
  });
  assert.equal(failed.result, "failed", "a persisted error code outranks a stale timestamp");

  const unhealthy = freshnessOf({ healthCheckedAt: "2024-01-01T00:00:00Z", healthStatus: "unhealthy" });
  assert.equal(unhealthy.result, "failed");
});

test("an in-flight probe reports running whatever the stored facts say", () => {
  const freshness = freshnessOf({
    lastProbeAt: 1_700_000_000,
    healthStatus: "healthy",
    running: true,
  });
  assert.equal(freshness.result, "running");
});

test("relative time is locale-aware and falls back to just-now under a minute", () => {
  const now = 1_700_000_000;
  assert.equal(relativeTime(now, now, "en", "just now"), "just now");
  assert.equal(relativeTime(now - 10, now, "en", "just now"), "just now");
  const minutesEn = relativeTime(now - 180, now, "en", "just now");
  assert.match(minutesEn, /3/, "three minutes ago must mention three");
  assert.match(minutesEn, /min/, "the unit is minutes");
  const hoursZh = relativeTime(now - 7_200, now, "zh", "刚刚");
  assert.match(hoursZh, /2/, "two hours ago must mention two");
  assert.match(hoursZh, /小时/, "zh uses its own unit word");
  // No hand-rolled plural table: the platform formatter is what produces these.
  assert.notEqual(relativeTime(now - 180, now, "en", "just now"), minutesEn.replace("3", "4"));
});

test("an unparsable health timestamp degrades to no timestamp rather than NaN", () => {
  assert.equal(unixSeconds("not a date"), null);
  assert.equal(unixSeconds(null), null);
  assert.equal(unixSeconds(undefined), null);
  const freshness = freshnessOf({ healthCheckedAt: "not a date" });
  assert.equal(freshness.atSeconds, null);
  assert.equal(freshness.source, "none");
});

// ── C. the once-only drift notice ──────────────────────────────────────────

test("the notice gate allows a change exactly once per session", () => {
  const gate = createReprobeNoticeGate();
  const key = reprobeNoticeKey("local.tool", "tool 2.0.0", { kind: "increase", count: 2 });
  assert.equal(gate.allow(key), true, "the first sighting is announced");
  assert.equal(gate.allow(key), false, "the same change is never announced twice");
  assert.equal(gate.allow(key), false, "and stays silent however often it is re-offered");
  assert.equal(gate.size(), 1, "one change occupies one slot");
});

test("a different change is still announced after an earlier one", () => {
  const gate = createReprobeNoticeGate();
  const first = reprobeNoticeKey("local.tool", "tool 2.0.0", { kind: "increase", count: 2 });
  const second = reprobeNoticeKey("local.tool", "tool 3.0.0", { kind: "increase", count: 1 });
  const otherTool = reprobeNoticeKey("local.other", "tool 2.0.0", { kind: "increase", count: 2 });
  assert.equal(gate.allow(first), true);
  assert.equal(gate.allow(second), true, "a later version is news again");
  assert.equal(gate.allow(otherTool), true, "another integration is its own notice");
  assert.equal(gate.allow(first), false);
});

// ── C2. the decision that turns a frame into at most one toast ─────────────
//
// This is the behaviour the round's "exactly one toast" requirement lives in,
// so it is exercised directly rather than only through the render wiring.

test("a changed command count is announced exactly once per session", () => {
  const gate = createReprobeNoticeGate();
  const frame = { extensionId: "local.tool", toolVersion: "tool 2.0.0", previousCommandCount: 1, commandCount: 3 };
  const first = decideDriftNotice(frame, "Tool", gate);
  assert.deepEqual(first.announce, {
    delta: { kind: "increase", count: 2 },
    key: reprobeNoticeKey("local.tool", "tool 2.0.0", { kind: "increase", count: 2 }),
  });
  // The same frame replayed — as a listing loop would — stays silent.
  const second = decideDriftNotice(frame, "Tool", gate);
  assert.equal(second.announce, null, "the same change is never announced twice");
  const third = decideDriftNotice(frame, "Tool", gate);
  assert.equal(third.announce, null);
  assert.equal(gate.size(), 1, "one change, one slot");
  // The display facts still reach the drawer even when the toast is suppressed.
  assert.equal(second.display.commandCount, 3);
  assert.equal(second.display.previousCommandCount, 1);
});

test("an unchanged command count never announces and never burns the gate", () => {
  const gate = createReprobeNoticeGate();
  const unchanged = { extensionId: "local.tool", toolVersion: "tool 2.0.0", previousCommandCount: 2, commandCount: 2 };
  assert.equal(decideDriftNotice(unchanged, "Tool", gate).announce, null, "no change, no toast");
  assert.equal(gate.size(), 0, "a no-op must not consume the once-only slot");
  // The trap: if the no-op had consumed a slot, the real change right after it
  // would be silenced. It must not be.
  const changed = { extensionId: "local.tool", toolVersion: "tool 2.0.0", previousCommandCount: 2, commandCount: 3 };
  const decision = decideDriftNotice(changed, "Tool", gate);
  assert.equal(decision.announce?.delta.count, 1, "the real change still speaks");
  assert.equal(gate.size(), 1);
});

test("an unknown previous count is never announced as a change", () => {
  const gate = createReprobeNoticeGate();
  // A first probe has nothing to compare against: no toast, and the drawer is
  // told the count without a fabricated delta.
  const first = decideDriftNotice(
    { extensionId: "local.tool", toolVersion: "tool 1.0.0", previousCommandCount: null, commandCount: 3 },
    "Tool",
    gate,
  );
  assert.equal(first.announce, null);
  assert.equal(first.display.commandCount, 3);
  assert.equal(first.display.previousCommandCount, null);
  assert.equal(gate.size(), 0);
  // Same for a frame that carries no counts at all.
  const empty = decideDriftNotice({ extensionId: "local.tool" }, "Tool", gate);
  assert.equal(empty.announce, null);
  assert.equal(empty.display.commandCount, null);
  assert.equal(gate.size(), 0);
});

test("a removal is announced with its own direction and wording", () => {
  const gate = createReprobeNoticeGate();
  const decision = decideDriftNotice(
    { extensionId: "local.tool", toolVersion: "tool 2.0.0", previousCommandCount: 5, commandCount: 3 },
    "Tool",
    gate,
  );
  assert.deepEqual(decision.announce?.delta, { kind: "decrease", count: 2 });
});

test("a frame for a row with no name is not announced and does not burn the gate", () => {
  const gate = createReprobeNoticeGate();
  const frame = { extensionId: "local.tool", toolVersion: "tool 2.0.0", previousCommandCount: 1, commandCount: 4 };
  assert.equal(decideDriftNotice(frame, null, gate).announce, null);
  assert.equal(gate.size(), 0, "an unnameable notice must not consume the slot");
  // Once the row is known, the same change is announced.
  assert.equal(decideDriftNotice(frame, "Tool", gate).announce?.delta.count, 3);
});

test("the notice key separates the direction of the change", () => {
  const grown = reprobeNoticeKey("local.tool", "v", { kind: "increase", count: 1 });
  const shrunk = reprobeNoticeKey("local.tool", "v", { kind: "decrease", count: 1 });
  assert.notEqual(grown, shrunk, "an addition and a removal are different facts");
});

// ── D. the wiring: the drawer renders all three, the panel announces once ──

test("the drawer renders a freshness section with all three facts", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const section = componentBody(panel, "function FreshnessSection(", "function EmptyState(");
  // All three facts are rendered from the projected values, not hardcoded.
  assert.match(section, /t\("settings\.extensions\.freshnessLastProbe"\)/, "the probe time row");
  assert.match(section, /t\("settings\.extensions\.freshnessResult"\)/, "the result row");
  assert.match(section, /t\("settings\.extensions\.freshnessCommands"\)/, "the command count row");
  // The timestamp is painted by the relative-time formatter, behind a guard
  // that keeps a never-probed integration from rendering `Invalid Date`.
  assert.match(
    section,
    /freshness\.atSeconds\s*\n?\s*\?\s*relativeTime\(/,
    "the probe time must be rendered through relativeTime",
  );
  // The delta is keyed off the projected kind, not hardcoded to "no change".
  assert.match(
    section,
    /extension-freshness__delta-text--\$\{freshness\.delta\.kind\}/,
    "the delta text must be driven by the projected kind",
  );
  assert.match(
    section,
    /freshnessOf\(\{/,
    "the section must project through the pure freshness module",
  );
  // Every delta kind must reach its own dictionary entry.
  for (const kind of ["increase", "decrease", "unchanged", "unknown"]) {
    assert.match(
      section,
      new RegExp(`${kind}: "settings\\.extensions\\.freshnessDelta`),
      `the ${kind} delta must map to its own string`,
    );
  }
});

test("the drawer shows an explicit unknown state instead of a blank", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const section = componentBody(panel, "function FreshnessSection(", "function EmptyState(");
  assert.match(
    section,
    /:\s*t\("settings\.extensions\.freshnessNever"\)/,
    "a never-probed integration says so in words",
  );
  assert.match(
    section,
    /t\("settings\.extensions\.freshnessCommandsUnknown"\)/,
    "an unknown command count says so in words",
  );
  // The numeric count is only printed when it is a real number — `0` is a
  // legitimate count, so the guard must be a null check, not a truthiness test.
  assert.match(
    section,
    /freshness\.commandCount !== null[\s\S]{0,200}freshnessCommandsValue/,
    "the numeric count must be gated on a null check",
  );
});

test("the drift notice fires through the app toast stack with a details action", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  assert.ok(
    panel.includes('t("settings.extensions.reprobeNotice"'),
    "the drift notice must use the translated notice string",
  );
  // The panel must delegate the "is this worth announcing?" decision to the pure
  // module (whose ordering is covered above), not re-implement it inline.
  assert.match(
    panel,
    /decideDriftNotice\(/,
    "the panel must route the frame through decideDriftNotice",
  );
  assert.match(
    panel,
    /if \(!decision\.announce \|\| !extension\) return;/,
    "the toast must be raised only for a positive decision",
  );
  assert.match(
    panel,
    /label: t\("settings\.extensions\.viewDetails"\), run: \(\) => setSelectedId\(payload\.extensionId\)/,
    "the toast action opens the integration it is talking about",
  );
});

test("the notice frame is handled outside the per-row progress strip", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  // A background frame parked in `operationProgress` would pin a stale
  // "Complete" line to the row (only `runMutation` clears that map).
  assert.match(
    panel,
    /if \(event\.payload\.notice\) \{\s*handleDriftNoticeRef\.current\(event\.payload\);\s*return;\s*\}/,
    "the notice frame must return before the progress strip is written",
  );
});

// ── E. the red lines: this is freshness, not an update centre ──────────────

test("the freshness section carries no update/version-store affordance", async () => {
  const panel = stripJsComments(await read("src/ExtensionsPanel.tsx"));
  const section = componentBody(panel, "function FreshnessSection(", "function EmptyState(");
  assert.ok(section.length > 0, "the freshness section must be locatable");
  for (const banned of ["update", "upgrade", "latest", "reinstall", "rollback"]) {
    assert.ok(
      !new RegExp(`\\b${banned}\\b`, "i").test(section),
      `the freshness block must not mention "${banned}"`,
    );
  }
  // It renders no control at all: freshness is read-only.
  assert.ok(!section.includes("<button"), "freshness must not offer a button");
  assert.ok(!section.includes("onClick"), "freshness must not wire a click handler");
});

test("the freshness module is free of version/upgrade semantics", async () => {
  const module = stripJsComments(await read("src/extensions/freshness.ts"));
  for (const banned of ["latest", "upgrade", "availableVersion", "newVersion"]) {
    assert.ok(
      !new RegExp(`\\b${banned}\\b`, "i").test(module),
      `the freshness module must not mention "${banned}"`,
    );
  }
});

// ── F. i18n balance ────────────────────────────────────────────────────────

const FRESHNESS_KEYS = [
  "settings.extensions.freshness",
  "settings.extensions.freshnessLastProbe",
  "settings.extensions.freshnessNever",
  "settings.extensions.freshnessJustNow",
  "settings.extensions.freshnessResult",
  "settings.extensions.freshnessResult.running",
  "settings.extensions.freshnessResult.success",
  "settings.extensions.freshnessResult.degraded",
  "settings.extensions.freshnessResult.failed",
  "settings.extensions.freshnessResult.unknown",
  "settings.extensions.freshnessCommands",
  "settings.extensions.freshnessCommandsValue",
  "settings.extensions.freshnessDeltaIncrease",
  "settings.extensions.freshnessDeltaDecrease",
  "settings.extensions.freshnessDeltaUnchanged",
  "settings.extensions.freshnessDeltaUnknown",
  "settings.extensions.freshnessCommandsUnknown",
  "settings.extensions.freshnessHealthSource",
  "settings.extensions.reprobeNotice",
  "settings.extensions.reprobeNoticeDeltaIncrease",
  "settings.extensions.reprobeNoticeDeltaDecrease",
  "settings.extensions.viewDetails",
];

test("every freshness key is declared exactly once per language", async () => {
  const i18n = await read("src/i18n.ts");
  for (const key of FRESHNESS_KEYS) {
    const occurrences = i18n.split(`"${key}":`).length - 1;
    assert.equal(occurrences, 2, `${key} must be declared for en and zh`);
  }
});

test("every freshness key translates to distinct, non-empty text in both languages", () => {
  const en = createTranslator("en");
  const zh = createTranslator("zh");
  for (const key of FRESHNESS_KEYS) {
    const english = en(key as Parameters<typeof en>[0], { name: "X", delta: "D", count: 1 });
    const chinese = zh(key as Parameters<typeof zh>[0], { name: "X", delta: "D", count: 1 });
    assert.ok(english.length > 0, `${key} must have en text`);
    assert.ok(chinese.length > 0, `${key} must have zh text`);
    // A zh entry that is byte-identical to its en twin is an untranslated
    // fallback, not a translation. The intentionally shared strings are exempt:
    // the bare +/- delta tokens and the count template, which are formatting
    // placeholders rather than prose (the surrounding words are translated).
    if (/Delta(Increase|Decrease)$/.test(key) || key.endsWith("CommandsValue")) continue;
    assert.notEqual(chinese, english, `${key} must be a real zh translation`);
  }
  // The placeholders are actually substituted, not left as literal braces.
  assert.ok(
    !en("settings.extensions.reprobeNotice", { name: "V Tools", delta: "+1" }).includes("{"),
    "the notice must interpolate its name and delta",
  );
  assert.ok(
    !zh("settings.extensions.reprobeNotice", { name: "V Tools", delta: "+1" }).includes("{"),
    "the zh notice must interpolate too",
  );
  assert.match(
    zh("settings.extensions.reprobeNotice", { name: "V Tools", delta: "+1" }),
    /[\u4e00-\u9fff]/,
    "the zh notice must be Chinese",
  );
});

test("the i18n dictionaries stay balanced overall", async () => {
  const i18n = await read("src/i18n.ts");
  const count = (source: string) => (source.match(/^ {2}"[^"]+":/gm) ?? []).length;
  const en = i18n.slice(i18n.indexOf("const en = {"), i18n.indexOf("export type MessageKey"));
  const zh = i18n.slice(i18n.indexOf("const zh: Record<MessageKey, string> = {"));
  assert.equal(count(en), count(zh), "en and zh must declare the same number of keys");
});
