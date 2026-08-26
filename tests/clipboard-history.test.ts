import assert from "node:assert/strict";
import test from "node:test";
import {
  clipboardPreview,
  filterClipboardEntries,
  formatClipboardAge,
  normalizeEntries,
  type ClipboardEntry,
} from "../src/clipboard-history.ts";

const entry = (overrides: Partial<ClipboardEntry>): ClipboardEntry => ({
  id: "id",
  kind: "text",
  text: "hello",
  image_file: null,
  width: null,
  height: null,
  hash: "h",
  created_at: 0,
  favorite: false,
  ...overrides,
});

test("normalizeEntries keeps well-formed rows and drops malformed ones", () => {
  const rows = [
    { id: "a", kind: "text", text: "hi", hash: "h1", created_at: 1, favorite: false },
    { id: "b", kind: "image", image_file: "b.png", width: 4, height: 6, hash: "h2", created_at: 2, favorite: true },
    { id: "c", kind: "weird", hash: "h3", created_at: 3, favorite: false },
    { id: "", kind: "text", hash: "h4", created_at: 4, favorite: false },
    null,
    "nope",
  ];

  const entries = normalizeEntries(rows);

  assert.equal(entries.length, 2);
  assert.equal(entries[0].kind, "text");
  assert.equal(entries[1].kind, "image");
  assert.equal(entries[1].favorite, true);
  // A non-array payload is simply an empty history.
  assert.deepEqual(normalizeEntries(undefined), []);
});

test("filter matches text content case-insensitively and images by name", () => {
  const entries = [
    entry({ id: "t", text: "Hello World" }),
    entry({ id: "i", kind: "image", text: null }),
    entry({ id: "z", text: "unrelated" }),
  ];

  assert.deepEqual(
    filterClipboardEntries(entries, "hello").map((item) => item.id),
    ["t"],
  );
  // Case-folded needle.
  assert.deepEqual(
    filterClipboardEntries(entries, "WORLD").map((item) => item.id),
    ["t"],
  );
  assert.deepEqual(
    filterClipboardEntries(entries, "img").map((item) => item.id),
    ["i"],
  );
  // The Chinese word for image answers too.
  assert.deepEqual(
    filterClipboardEntries(entries, "图片").map((item) => item.id),
    ["i"],
  );
  // An empty (or whitespace-only) query passes everything through, in order.
  assert.equal(filterClipboardEntries(entries, "  ").length, 3);
});

test("preview shows the first line of text, capped with an ellipsis", () => {
  assert.equal(clipboardPreview(entry({ text: "one\ntwo\nthree" })), "one");
  assert.equal(clipboardPreview(entry({ text: "   padded   " })), "padded");

  const long = "x".repeat(300);
  const preview = clipboardPreview(entry({ text: long }));
  assert.equal(preview.length, 121);
  assert.ok(preview.endsWith("…"));
  assert.ok(preview.startsWith("x".repeat(120)));
});

test("preview labels images with their dimensions", () => {
  assert.equal(clipboardPreview(entry({ kind: "image", width: 320, height: 240 })), "[image 320x240]");
  // Missing dimensions degrade to question marks rather than crashing.
  assert.equal(clipboardPreview(entry({ kind: "image", width: null, height: undefined }) as ClipboardEntry), "[image ?x?]");
});

test("age formats compactly across unit boundaries", () => {
  const now = 10_000_000;
  const at = (secondsBefore: number) => now - secondsBefore * 1000;

  assert.equal(formatClipboardAge(at(42), now), "42s");
  assert.equal(formatClipboardAge(at(59), now), "59s");
  assert.equal(formatClipboardAge(at(60), now), "1m");
  // Minutes roll into hours at the hour mark.
  assert.equal(formatClipboardAge(at(90 * 60), now), "1h");
  assert.equal(formatClipboardAge(at(23 * 60 * 60), now), "23h");
  assert.equal(formatClipboardAge(at(24 * 60 * 60), now), "1d");
  assert.equal(formatClipboardAge(at(45 * 24 * 60 * 60), now), "45d");
  // A future timestamp (clock skew) clamps to zero rather than going negative.
  assert.equal(formatClipboardAge(now + 5000, now), "0s");
});
