package launcher

import (
	"strings"
	"unicode/utf8"
)

// The launcher's word gaps are JavaScript's `\s`, not the shell's U+0020 and
// not Go's unicode.IsSpace.
//
// The old frontend split trigger words with regular expressions — `/^(\S+)\s+
// ([\s\S]*)$/` to enter a mode (`src/plugins/mode-entry.ts`) and
// `split(/\s+/)[0]` to reduce a command's name to its first word
// (`src/plugins/external.ts`). JavaScript's `\s` is exactly the twenty-five
// code points in jsWhitespace below: the six ASCII blanks, the Zs separators
// U+00A0 and U+1680..U+3000, the two line separators U+2028/U+2029 and the
// byte-order mark U+FEFF. It deliberately excludes U+0085 (NEL) and U+180E
// (Mongolian vowel separator), which Go's unicode.IsSpace and the Zs category
// disagree about.
//
// The consequence the old build was built on: a Chinese input method types
// U+3000 IDEOGRAPHIC SPACE for its space bar, so `clip\u3000foo` must enter the
// clipboard mode exactly as `clip foo` does. A word gap here is a *parsing*
// rule; text on its way to a PTY is a different boundary (internal/inputtext),
// whose scope is the Zs category alone — a newline there is legitimate terminal
// input, not a word gap.

// jsWhitespace is every code point JavaScript's `\s` matches, and the
// reference the switch in isWordGap is checked against (see wordgap_test.go).
const jsWhitespace = "\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003\u2004" +
	"\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

// isWordGap reports whether r separates words in the launcher's query: the
// JavaScript `\s` set, so the trigger-word rules match the old frontend's.
func isWordGap(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return '\u2000' <= r && r <= '\u200a'
}

// indexWordGap is the byte index of the first word gap in s, or -1 when the
// line has none. It is modeWord's half of the trigger split.
func indexWordGap(s string) int {
	for i, r := range s {
		if isWordGap(r) {
			return i
		}
	}
	return -1
}

// lastWordGapEnd is the byte offset just past the last word gap in s, or -1
// when the line has none. It is replaceLastWord's half of the split, and the
// offset (rather than the gap's own index) keeps a multi-byte gap whole: a
// full-width space is three bytes, and slicing at its first byte would cut the
// rune in half.
func lastWordGapEnd(s string) int {
	end := -1
	for i, r := range s {
		if isWordGap(r) {
			end = i + utf8.RuneLen(r)
		}
	}
	return end
}

// trimTrailingWordGaps drops every trailing word gap, so "does this line end
// in a gap?" has one answer on every path (completion tokens, the current
// word, and appending a completed word).
func trimTrailingWordGaps(s string) string {
	return strings.TrimRightFunc(s, isWordGap)
}
