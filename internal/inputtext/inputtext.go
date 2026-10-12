// Package inputtext is the injection boundary for every string that becomes
// terminal input bytes.
//
// A shell tokenizes on U+0020 only. Every other Unicode space separator (the
// Zs category) — U+00A0 NO-BREAK SPACE, U+2000..U+200A, U+202F, U+205F and
// U+3000 IDEOGRAPHIC SPACE — glues two words into ONE shell word:
// `go<U+3000>version` is a single token to zsh, which then answers
// `zsh: command not found: go version`. The byte is easy to produce and hard
// to see: a Chinese input method types U+3000 for the space bar, and a copied
// line keeps whatever the page it came from used.
//
// The reference fixed this at one place — the boundary where text becomes PTY
// input (`src/terminal/inputNormalize.ts`) — rather than at each source, and
// this package is that place: the two pure functions every string seam calls.
// Scope is deliberate:
//
//   - U+0020 passes through untouched (and a clean string is returned as the
//     same string: the hot path must not allocate).
//   - Whitespace that is not a Zs separator — tab, newline, CR, vertical tab,
//     form feed, U+0085, U+200B ZERO WIDTH SPACE, U+2028/U+2029 — passes
//     through: tabs and newlines are legitimate terminal input, and U+200B is
//     not a space at all.
//   - Structured argv never flows through here: a per-token argv built from
//     ParseCommandLine lands in the program's own argv, which no shell
//     re-tokenizes.
package inputtext

import (
	"strings"
	"unicode"
)

// isSeparator reports whether r is a space separator that must become U+0020.
func isSeparator(r rune) bool {
	return r != ' ' && unicode.Is(unicode.Zs, r)
}

// NormalizeSpaces replaces every Unicode space separator (except the ASCII
// space) with U+0020. A string with none is returned unchanged, without a
// copy: the boundary runs on every paste and every assembled command line.
func NormalizeSpaces(text string) string {
	if strings.IndexFunc(text, isSeparator) < 0 {
		return text
	}
	return strings.Map(func(r rune) rune {
		if isSeparator(r) {
			return ' '
		}
		return r
	}, text)
}

// StripPasteNewline removes exactly one trailing line break, "\n" or "\r\n".
//
// A copied command almost always carries the newline that ran it in the
// terminal it came from, so pasting it here would execute before the user had
// read the line. Removing one trailing break leaves every interior newline
// intact — a multi-line paste is still multi-line — and a string with no
// trailing break is returned as is.
func StripPasteNewline(text string) string {
	if strings.HasSuffix(text, "\r\n") {
		return text[:len(text)-2]
	}
	if strings.HasSuffix(text, "\n") {
		return text[:len(text)-1]
	}
	return text
}
