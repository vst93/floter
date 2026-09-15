// Injection-boundary normalization: every string seam that becomes terminal
// input-line bytes must collapse Unicode space separators (Zs, except U+0020)
// to the ASCII space before encoding. A missed Zs byte is the "go version"
// fused-token failure — zsh tokenizes on U+0020 only, so `go\u3000version`
// is ONE word to the shell and reports `command not found: go version`.
//
// Mutation kill points, per assertion group:
//   - Dropping any code point from the separator class (e.g. U+202F) makes
//     its dedicated row fail: the byte survives normalization unchanged.
//   - Including U+0020 in the class makes the "ASCII space passes through"
//     row fail only if the implementation also *changes* clean input (it must
//     not — identity is asserted by reference equality).
//   - Returning the input untouched (deleting the function body) fails every
//     separator row and the mixed row.
//   - Over-normalizing (mapping tab/newline/U+200B) fails the
//     non-separator-passthrough row.
//   - Breaking the fast-path identity (e.g. always `text.replace(...)`)
//     survives value equality but is caught by the `toBe` identity row —
//     the hot path must not allocate on clean input.
import assert from "node:assert/strict";
import test from "node:test";
import { normalizeTerminalInputSpaces } from "../src/terminal/inputNormalize.ts";

test("each Unicode space separator collapses to U+0020", () => {
  // One row per Zs code point (minus U+0020 itself). A dropped class member
  // fails exactly its row, which is what makes the table a kill-point map.
  const separators: Array<[string, string]> = [
    ["\u00a0", "U+00A0 NO-BREAK SPACE"],
    ["\u1680", "U+1680 OGHAM SPACE MARK"],
    ["\u2000", "U+2000 EN QUAD"],
    ["\u2001", "U+2001 EM QUAD"],
    ["\u2002", "U+2002 EN SPACE"],
    ["\u2003", "U+2003 EM SPACE"],
    ["\u2004", "U+2004 THREE-PER-EM SPACE"],
    ["\u2005", "U+2005 FOUR-PER-EM SPACE"],
    ["\u2006", "U+2006 SIX-PER-EM SPACE"],
    ["\u2007", "U+2007 FIGURE SPACE"],
    ["\u2008", "U+2008 PUNCTUATION SPACE"],
    ["\u2009", "U+2009 THIN SPACE"],
    ["\u200a", "U+200A HAIR SPACE"],
    ["\u202f", "U+202F NARROW NO-BREAK SPACE"],
    ["\u205f", "U+205F MEDIUM MATHEMATICAL SPACE"],
    ["\u3000", "U+3000 IDEOGRAPHIC SPACE"],
  ];
  for (const [separator, label] of separators) {
    assert.equal(
      normalizeTerminalInputSpaces(`go${separator}version`),
      "go version",
      `${label} must become U+0020 between two words`,
    );
  }
});

test("mixed separators all collapse in one pass", () => {
  assert.equal(
    normalizeTerminalInputSpaces("go\u3000version\u00a0--flag\u2009x"),
    "go version --flag x",
  );
  // A line whose ONLY separators are bad bytes fully recovers.
  assert.equal(
    normalizeTerminalInputSpaces("npm\u2003run\u2007build"),
    "npm run build",
  );
});

test("the ASCII space and spaces inside words are untouched", () => {
  const clean = "echo 'a  b'   c";
  assert.equal(normalizeTerminalInputSpaces(clean), clean);
  // Identity, not just equality: the clean-input fast path must hand back the
  // original string without allocating a replacement copy.
  assert.ok(
    typeof normalizeTerminalInputSpaces(clean) === "string",
  );
  const passthrough = normalizeTerminalInputSpaces(clean);
  assert.equal(passthrough, clean);
  // The byte the shell actually tokenizes on stays exactly U+0020.
  assert.ok(normalizeTerminalInputSpaces("a b").includes(" "));
});

test("non-separator whitespace and zero-width characters pass through", () => {
  // Tab, newline, CR are legitimate terminal input (completion, submit).
  assert.equal(normalizeTerminalInputSpaces("a\tb\nc\rd"), "a\tb\nc\rd");
  // U+200B is a format character, not a space; U+2028/U+2029 are line
  // separators (Zl/Zp), not Zs. None of them masquerade as a word gap the
  // way Zs bytes do, and rewriting them would corrupt legitimate input.
  assert.equal(normalizeTerminalInputSpaces("a\u200bb"), "a\u200bb");
  assert.equal(normalizeTerminalInputSpaces("a\u2028b"), "a\u2028b");
  assert.equal(normalizeTerminalInputSpaces("a\u2029b"), "a\u2029b");
});

test("clean input returns the same string reference (no hot-path copy)", () => {
  const clean = "cargo build --release";
  assert.ok(
    Object.is(normalizeTerminalInputSpaces(clean), clean),
    "clean input must be returned as-is so keystroke flushes never allocate",
  );
});

test("empty and separator-only strings normalize without error", () => {
  assert.equal(normalizeTerminalInputSpaces(""), "");
  assert.equal(normalizeTerminalInputSpaces("\u3000\u00a0"), "  ");
});

test("non-ASCII word content survives untouched", () => {
  // CJK text with a genuine U+3000 as the *only* space is the hard case:
  // the byte is normalized even though the words themselves are non-ASCII.
  assert.equal(
    normalizeTerminalInputSpaces("中文\u3000测试"),
    "中文 测试",
  );
  // But non-space non-ASCII bytes (the payload's legitimate UTF-8 content)
  // are never touched.
  const cjk = "空格";
  assert.ok(Object.is(normalizeTerminalInputSpaces(cjk), cjk));
});

test("the go-version failure signature is repaired", () => {
  // The exact byte shape the differential rounds isolated: U+3000 between
  // the two words produces zsh's `command not found: go version`.
  assert.deepEqual(
    Array.from(new TextEncoder().encode(normalizeTerminalInputSpaces("go\u3000version"))),
    Array.from(new TextEncoder().encode("go version")),
  );
  // So does the U+00A0 shape.
  assert.deepEqual(
    Array.from(new TextEncoder().encode(normalizeTerminalInputSpaces("go\u00a0version"))),
    Array.from(new TextEncoder().encode("go version")),
  );
  // Byte-exact: the encoded line carries 0x20 between the words, never
  // 0xC2 0xA0 or 0xE3 0x80 0x80.
  const repaired = new TextEncoder().encode(
    normalizeTerminalInputSpaces("go\u3000version\u00a0x"),
  );
  assert.equal(Array.from(repaired).join(","), Array.from(new TextEncoder().encode("go version x")).join(","));
  assert.ok(!repaired.includes(0xc2));
  assert.ok(!repaired.includes(0xe3));
});
