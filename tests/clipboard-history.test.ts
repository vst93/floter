import assert from "node:assert/strict";
import test from "node:test";
import {
  clipboardPreview,
  filterClipboardEntries,
  formatClipboardAge,
  formatFilesPreview,
  imageFileMime,
  isFilesPreviewCandidate,
  isImageFilePath,
  looksLikeDirectoryPath,
  normalizeEntries,
  shellQuotePath,
  splitFilePath,
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

test("normalizeEntries accepts files rows and keeps their paths", () => {
  const rows = [
    {
      id: "f",
      kind: "files",
      paths: ["/home/u/report.pdf", "/home/u/notes.txt", 42, null],
      hash: "hf",
      created_at: 9,
      favorite: false,
    },
    { id: "g", kind: "files", hash: "hg", created_at: 10, favorite: false },
    { id: "h", kind: "files", paths: "not-an-array", hash: "hh", created_at: 11, favorite: false },
  ];

  const entries = normalizeEntries(rows);

  assert.equal(entries.length, 3);
  // Non-string items are dropped from the list but the entry survives.
  assert.deepEqual(entries[0].paths, ["/home/u/report.pdf", "/home/u/notes.txt"]);
  assert.deepEqual(entries[1].paths, null);
  assert.deepEqual(entries[2].paths, null);
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

test("filter matches files entries against full paths and basenames", () => {
  const entries = [
    entry({ id: "f", kind: "files", text: null, paths: ["/home/u/report.pdf"] }),
    entry({ id: "g", kind: "files", text: null, paths: null }),
  ];

  // Full path, basename, case-folded.
  assert.deepEqual(
    filterClipboardEntries(entries, "/home/u").map((item) => item.id),
    ["f"],
  );
  assert.deepEqual(
    filterClipboardEntries(entries, "REPORT").map((item) => item.id),
    ["f"],
  );
  assert.deepEqual(
    filterClipboardEntries(entries, "pdf").map((item) => item.id),
    ["f"],
  );
  // No paths at all matches nothing.
  assert.equal(filterClipboardEntries(entries, "anything").length, 0);
});

test("splitFilePath handles POSIX and Windows separators", () => {
  assert.deepEqual(splitFilePath("/home/u/report.pdf"), {
    basename: "report.pdf",
    dirname: "/home/u",
  });
  assert.deepEqual(splitFilePath("C:\\Users\\u\\notes.txt"), {
    basename: "notes.txt",
    dirname: "C:\\Users\\u",
  });
  assert.deepEqual(splitFilePath("bare.txt"), { basename: "bare.txt", dirname: "" });
  // Trailing separators belong to the directory itself, not the basename.
  assert.deepEqual(splitFilePath("/home/u/dir/"), { basename: "dir", dirname: "/home/u" });
});

test("isImageFilePath judges by extension on both path styles", () => {
  assert.equal(isImageFilePath("/a/b/pic.png"), true);
  assert.equal(isImageFilePath("C:\\a\\b\\PIC.JPG"), true);
  assert.equal(isImageFilePath("/a/b.d/c.webp"), true);
  assert.equal(isImageFilePath("/a/b/c.txt"), false);
  assert.equal(isImageFilePath("/a/b/c"), false);
  assert.equal(isImageFilePath(""), false);
});

test("isFilesPreviewCandidate requires exactly one image path", () => {
  assert.equal(isFilesPreviewCandidate(["/x/a.png"]), true);
  assert.equal(isFilesPreviewCandidate(["/x/a.png", "/y/b.jpg"]), false);
  assert.equal(isFilesPreviewCandidate(["/x/a.txt"]), false);
  assert.equal(isFilesPreviewCandidate([]), false);
  assert.equal(isFilesPreviewCandidate(null), false);
  assert.equal(isFilesPreviewCandidate(undefined), false);
});

test("imageFileMime maps extensions to blob types", () => {
  assert.equal(imageFileMime("/x/a.png"), "image/png");
  assert.equal(imageFileMime("/x/a.jpeg"), "image/jpeg");
  assert.equal(imageFileMime("C:\\x\\b.GIF"), "image/gif");
  assert.equal(imageFileMime("/x/c.txt"), "application/octet-stream");
});

test("looksLikeDirectoryPath guesses from trailing separator or extension", () => {
  assert.equal(looksLikeDirectoryPath("/home/u/dir/"), true);
  assert.equal(looksLikeDirectoryPath("C:\\dir\\"), true);
  assert.equal(looksLikeDirectoryPath("/home/u/build"), true);
  assert.equal(looksLikeDirectoryPath("/home/u/report.pdf"), false);
});

test("formatFilesPreview splits basename, muted prefix and count suffix", () => {
  const one = formatFilesPreview(["/home/u/report.pdf"]);
  assert.equal(one.basename, "report.pdf");
  assert.equal(one.dirname, "/home/u/");
  assert.equal(one.extra, 0);

  // Windows-style prefix keeps its own separators.
  const win = formatFilesPreview(["C:\\Users\\u\\notes.txt"]);
  assert.equal(win.basename, "notes.txt");
  assert.equal(win.dirname, "C:\\Users\\u\\");

  // Several items at once: +N suffix data.
  const many = formatFilesPreview(["/a/b.txt", "/c/d.txt", "/e/f.txt"]);
  assert.equal(many.basename, "b.txt");
  assert.equal(many.dirname, "/a/");
  assert.equal(many.extra, 2);

  // Degenerate inputs stay empty rather than crashing.
  assert.deepEqual(formatFilesPreview([]), { basename: "", dirname: "", extra: 0 });
  assert.deepEqual(formatFilesPreview(null), { basename: "", dirname: "", extra: 0 });
});

test("shellQuotePath escapes embedded single quotes POSIX-style", () => {
  assert.equal(shellQuotePath("/home/u/my file.pdf"), "'/home/u/my file.pdf'");
  assert.equal(shellQuotePath("/home/u/it's.pdf"), "'/home/u/it'\\''s.pdf'");
});
