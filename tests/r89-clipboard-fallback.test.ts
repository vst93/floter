// R89 · the launcher's zero-result full-text fallback.
//
// The list IPC ships only a prefix of each text entry, so the clipboard mode's
// in-memory filter can miss a match that lives past that prefix. The mode asks
// the backend to search the full text — but only when the query is non-empty and
// the memory filter found nothing. This suite pins that gate and the two halves
// of the wiring:
//
//   1. the decision is a pure predicate (`shouldSearchFullText`), so the
//      "local hits ⇒ no IPC / zero hits ⇒ ask" rule is a unit, not a comment;
//   2. the catalog's effect is gated on it and the emission trusts the backend's
//      answer as-is (`clipboardSearchRows`) — the in-memory rule is deliberately
//      not re-applied, because the returned rows carry only a prefix and
//      re-filtering them would drop exactly the deep matches the fallback exists
//      to find.
//
// Mutation that must turn this red: flipping the predicate (a local hit calling
// the backend, or a zero-result query never calling it), or removing the
// `clipboardNeedsFullText` gate from the catalog's emission.

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  shouldSearchFullText,
  type ClipboardEntry,
} from "../src/clipboard-history.ts";
import { createTranslator } from "../src/i18n.ts";
import { clipboardModeRows, clipboardSearchRows } from "../src/plugins/clipboard/mode.ts";

const root = new URL("../", import.meta.url);
const read = (path: string) => readFile(new URL(path, root), "utf8");
const stripJsComments = (source: string) =>
  source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const en = createTranslator("en");

const entry = (
  id: string,
  overrides: Partial<ClipboardEntry> = {},
): ClipboardEntry => ({
  id,
  kind: "text",
  text: `note ${id}`,
  paths: null,
  image_file: null,
  width: null,
  height: null,
  hash: `hash-${id}`,
  created_at: 1_700_000_000_000,
  favorite: false,
  ...overrides,
});

// ── A · the predicate ─────────────────────────────────────────────────────

test("the fallback fires exactly on a non-empty query the prefix search missed", () => {
  const local = entry("local", { text: "short note" });
  // An empty query is not a search: the mode shows the whole history.
  assert.equal(shouldSearchFullText([local], ""), false);
  assert.equal(shouldSearchFullText([local], "   "), false);
  // A query the memory filter answered costs no IPC.
  assert.equal(shouldSearchFullText([local], "short"), false);
  assert.equal(shouldSearchFullText([local], "short note"), false);
  // A query it did not answer asks the backend.
  assert.equal(shouldSearchFullText([local], "zzz"), true);
  // The list only carries a prefix, so a match past it looks like "no local
  // hit" from here — which is precisely when the backend is asked.
  assert.equal(shouldSearchFullText([local], "beyond-the-prefix"), true);
  // An empty history is still a zero-result query.
  assert.equal(shouldSearchFullText([], "zzz"), true);
});

// ── B · the backend's answer is trusted, the chip still applies ───────────

test("the backend's full-text answer is not re-filtered in memory", () => {
  // The server returned an entry because the needle matched past the prefix;
  // the truncated text that reached the frontend no longer contains it.
  const deep = entry("deep", { text: "a".repeat(9000) });
  const mode = { needle: "deep-match", filter: "all" as const };

  // The in-memory path can only see the (truncated) text and finds nothing…
  const local = clipboardModeRows([deep], mode, en, 1);
  assert.deepEqual(local.map((row) => row.id), ["clipboard-empty"]);
  // …while the backend's answer renders, because the rule is not run again.
  const trusted = clipboardSearchRows([deep], mode, en, 1);
  assert.deepEqual(trusted.map((row) => row.id), ["deep"]);
});

test("the chip still selects on top of the backend's answer", () => {
  const favorite = entry("fav", { favorite: true });
  const plain = entry("plain", { favorite: false });
  const mode = { needle: "deep-match", filter: "favorites" as const };
  assert.deepEqual(
    clipboardSearchRows([plain, favorite], mode, en, 1).map((row) => row.id),
    ["fav"],
  );
});

test("an empty backend answer is the query's empty note, never the history's", () => {
  const rows = clipboardSearchRows([], { needle: "deep-match", filter: "all" }, en, 1);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].id, "clipboard-empty");
  assert.equal(rows[0].title, en("clipboard.emptyFilter"));
});

// ── C · the catalog wiring ────────────────────────────────────────────────

test("the catalog asks the backend only on the zero-result gate", async () => {
  const source = stripJsComments(await read("src/hooks/useLauncherCatalog.ts"));

  // The gate is the pure predicate, over the fetched entries and the needle.
  assert.match(
    source,
    /clipboardNeedsFullText =\s*clipboardActive && shouldSearchFullText\(clipboardEntries, clipboardNeedle\)/,
    "the fallback gate is the shared predicate",
  );
  // The fallback read passes the needle as `filter` (the full-text search), and
  // its effect only runs when the gate is on.
  assert.match(
    source,
    /if \(!clipboardActive \|\| !clipboardEnabled \|\| !clipboardNeedsFullText\) \{\s*setClipboardFallback\(null\);/,
    "no local hit means no IPC; the fallback clears itself otherwise",
  );
  assert.match(
    source,
    /invoke<unknown\[\]>\("clipboard_get_entries", \{ filter: clipboardNeedle \}\)/,
    "the fallback sends the needle to the backend's full-text search",
  );
  // The whole-history read is unchanged and still the mode's first call.
  assert.match(
    source,
    /invoke<unknown\[\]>\("clipboard_get_entries", \{ filter: null \}\)/,
    "the mode still reads the whole (prefixed) history once",
  );
  // The emission uses the backend's answer only while the gate is on and only
  // for the needle it was fetched for.
  assert.match(
    source,
    /clipboardNeedsFullText && clipboardFallback\?\.needle === clipboardNeedle/,
    "a fallback keyed to a previous needle is never shown",
  );
  assert.match(
    source,
    /clipboardSearchRows\(fallback, clipboardMode, t, Date\.now\(\), MAX_CLIPBOARD_MAX_ITEMS\)/,
    "the backend's answer is trusted as-is",
  );
  assert.match(
    source,
    /clipboardModeRows\(clipboardEntries, clipboardMode, t, Date\.now\(\), MAX_CLIPBOARD_MAX_ITEMS\)/,
    "the in-memory path stays the default",
  );
});
