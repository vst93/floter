// R89 · the clipboard list's token search, pinned in **both** languages.
//
// The list IPC now ships only a prefix of each text entry (see
// `LIST_TEXT_PREFIX_BYTES` in `src-tauri/src/clipboard_history/mod.rs`), so the
// launcher can fall back to a backend search over the full text. That fallback
// is only honest if the backend runs the *same* rule the frontend runs in
// memory: whitespace-split AND tokens, each hitting one of the entry's search
// fields, the fields being exactly `clipboardEntrySearchFields`.
//
// The two rules live in different languages and cannot import each other, so the
// fixture below is hard-coded on both sides — the same vectors in
// `clipboard_history/search.rs`. The last test reads the Rust fixture back and
// fails if the two lists ever drift, which is the only thing that keeps "the
// same rule" from quietly becoming "a similar rule".
//
// Mutation that must turn this red: reverting the Rust side to a single
// substring `contains` (the multi-token-different-fields vector, and the
// one-missing-token vector, both flip).

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { filterClipboardEntries, type ClipboardEntry } from "../src/clipboard-history.ts";

const root = new URL("../", import.meta.url);

type SearchCase = {
  name: string;
  kind: string;
  text: string | null;
  paths: string[] | null;
  query: string;
  match: boolean;
};

/**
 * The shared fixture. Byte-for-byte the vectors in
 * `src-tauri/src/clipboard_history/search.rs`; keep the two lists identical and
 * in the same order.
 */
const SEARCH_CASES: SearchCase[] = [
  { name: "empty query matches everything", kind: "text", text: "hello", paths: null, query: "", match: true },
  { name: "whitespace-only query matches everything", kind: "text", text: "hello", paths: null, query: "   ", match: true },
  { name: "single token hits the text field", kind: "text", text: "hello world", paths: null, query: "hello", match: true },
  { name: "AND: two tokens in one text field", kind: "text", text: "hello world", paths: null, query: "world hello", match: true },
  { name: "AND: one missing token rejects", kind: "text", text: "hello world", paths: null, query: "hello missing", match: false },
  { name: "case-insensitive hit", kind: "text", text: "Rust Async", paths: null, query: "rUsT aSYNC", match: true },
  { name: "CJK token", kind: "text", text: "剪贴板历史", paths: null, query: "剪贴板", match: true },
  { name: "CJK two tokens AND", kind: "text", text: "剪贴板历史", paths: null, query: "历史 剪贴板", match: true },
  { name: "files entry matches a path token", kind: "files", text: null, paths: ["/tmp/report.pdf"], query: "report", match: true },
  { name: "files entry matches a full path token", kind: "files", text: null, paths: ["/tmp/report.pdf"], query: "/tmp/report.pdf", match: true },
  { name: "files entry ignores a stray text field", kind: "files", text: "hidden", paths: ["/tmp/a.txt"], query: "hidden", match: false },
  { name: "files entry misses an absent path token", kind: "files", text: null, paths: ["/tmp/a.txt"], query: "zzz", match: false },
  { name: "bare image answers to its name", kind: "image", text: null, paths: null, query: "图片", match: true },
  { name: "bare image answers to img", kind: "image", text: null, paths: null, query: "IMG", match: true },
  { name: "captioned image matches its caption", kind: "image", text: "diagram", paths: null, query: "diagram", match: true },
  { name: "captioned image: tokens may hit different fields", kind: "image", text: "diagram", paths: null, query: "diagram 图片", match: true },
  { name: "text entry does not answer to the image word", kind: "text", text: "diagram", paths: null, query: "diagram 图片", match: false },
  { name: "non-files text entry matches its text", kind: "text", text: "hidden", paths: null, query: "hidden", match: true },
];

const entryFor = (vector: SearchCase): ClipboardEntry => ({
  id: "id",
  kind: vector.kind,
  text: vector.text,
  paths: vector.paths,
  image_file: null,
  width: null,
  height: null,
  hash: "h",
  created_at: 0,
  favorite: false,
});

test("the shared contract vectors hold on the frontend rule", () => {
  for (const vector of SEARCH_CASES) {
    const kept = filterClipboardEntries([entryFor(vector)], vector.query);
    assert.equal(kept.length === 1, vector.match, `vector drifted: ${vector.name}`);
  }
});

test("the Rust fixture still carries the same vectors, in the same order", async () => {
  const rust = await readFile(
    new URL("src-tauri/src/clipboard_history/search.rs", root),
    "utf8",
  );
  const names = [...rust.matchAll(/case!\(\s*"([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(
    names,
    SEARCH_CASES.map((vector) => vector.name),
    "the Rust and node contract fixtures have drifted apart",
  );
});
