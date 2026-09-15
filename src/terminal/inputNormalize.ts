// Injection-boundary normalization for every string that becomes terminal
// input-line bytes.
//
// Why this exists: a shell tokenizes on U+0020 only. Every other Unicode
// space separator (Zs) — U+00A0 (NO-BREAK SPACE), U+2000..U+200A (EN QUAD..
// HAIR SPACE), U+202F (NARROW NO-BREAK SPACE), U+205F (MEDIUM MATHEMATICAL
// SPACE), U+3000 (IDEOGRAPHIC SPACE) — glues two words into ONE shell word.
// `go<U+3000>version` is a single fused token to zsh, which then reports
// `zsh: command not found: go version` — precisely the byte-level failure the
// terminal rounds have been hunting. Any string source (a descriptor field, a
// clipboard paste, an IME commit, a history entry) can carry such a byte, so
// the fix is at the boundary where text becomes PTY input, not at each source.
//
// Scope is deliberate:
//   - U+0020 (the ASCII space) passes through untouched.
//   - Other Unicode whitespace that is NOT a Zs separator (tab, newline, CR,
//     vertical tab, form feed, U+0085, U+200B ZERO WIDTH SPACE, line
//     separators U+2028/U+2029) passes through untouched: tabs and newlines
//     are legitimate terminal input (completion, submit), and U+200B is not a
//     space at all. Only Zs separators masquerading as word gaps are mapped.
//   - The structured execution path (argv arrays) never flows through here:
//     per-token argv is built from `parseCommandLine` output and lands in the
//     PTY via `initial_command_payload` / `SpawnCommand` verbatim contracts.
//     This module guards the *string* seams — typed text, IME commits,
//     pastes, initial command lines assembled for the interactive shell.

/**
 * Every Unicode `Zs` (Space_Separator) code point except U+0020.
 *
 * U+180E MONGOLIAN VOWEL SEPARATOR is intentionally absent: it lost its
 * White_Space/Zs status in Unicode 6.3 and is a format character (Cf) now.
 */
const UNICODE_SPACE_SEPARATORS =
  "\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u202f\u205f\u3000";

const SPACE_SEPARATOR_SOURCE = `[${UNICODE_SPACE_SEPARATORS}]`;

/** Global form drives the replacement pass. */
const SPACE_SEPARATOR_REPLACE = new RegExp(SPACE_SEPARATOR_SOURCE, "gu");
/** Stateless probe: `RegExp.prototype.test` is position-sticky on `/g` regexes,
 * so the fast path uses a non-global twin instead of sharing the replacer. */
const SPACE_SEPARATOR_PROBE = new RegExp(SPACE_SEPARATOR_SOURCE, "u");

/**
 * Replace every Unicode space separator (except the ASCII space) with U+0020.
 *
 * The probe runs first so the hot path (every keystroke flush, every paste)
 * pays one failed regex test and returns the original string untouched — no
 * replacement pass, no copy, no allocation.
 */
export function normalizeTerminalInputSpaces(text: string): string {
  return SPACE_SEPARATOR_PROBE.test(text)
    ? text.replace(SPACE_SEPARATOR_REPLACE, " ")
    : text;
}
