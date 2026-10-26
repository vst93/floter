package shell

import (
	"strings"
	"unicode/utf8"
)

// The reference's `args` hint has a shape even though this build has no reader
// for it (`linkparams.go:22-26`). `validate_arguments`
// (`main:src-tauri/src/deep_link.rs:402-438`) splits the value on ASCII spaces
// and refuses the whole link when any token is not shell-inert; the reason it
// judges a value nobody runs is written down right there (`:398-400`): a hostile
// link must not smuggle something a *later* reader might execute. This build
// drops `args` on the floor today, so the refusal is not observable as data —
// but a link the reference refuses must not be accepted here (P10-g/P10-h), and
// the day a surface shows the hint, the value has already been through a gate.
//
// The numbers are the reference's (`deep_link.rs:116-125`).
const (
	maxArgumentTokens = 16
	maxArgumentChars  = 64
	maxArgumentsChars = 256
)

// linkArgsFit reports whether an `args` value has the shape the reference
// accepts. An empty value fits: the key is optional, and the reference returns
// `Ok(None)` for it (`:404-406`) — `args=` is "no arguments", not a refusal.
//
// The bounds count `chars()` in the reference, so this counts runes rather than
// bytes. That distinction is unobservable on its own: the character class below
// is ASCII-only, so a token that is long in bytes and short in runes (a CJK word)
// is refused by the class anyway. Counting runes is still the faithful reading,
// and it is what a future widening of the class would need.
func linkArgsFit(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return true
	}
	if utf8.RuneCountInString(value) > maxArgumentsChars {
		return false
	}
	tokens := 0
	for _, token := range strings.Split(value, " ") {
		token = strings.TrimSpace(token)
		if token == "" {
			// A run of spaces is not an empty argument: the reference skips
			// these before it counts (`:412-415`).
			continue
		}
		if utf8.RuneCountInString(token) > maxArgumentChars {
			return false
		}
		// A leading dash needs no branch of its own: `-` is not in the class
		// below, so `--hidden` is refused by the character test. The reference
		// has both (`:419-421` and `:422-424`) and its dash branch is
		// unreachable for the same reason — the mutation that deletes it stays
		// green (report §4), so this build keeps only the branch that decides.
		for _, character := range token {
			if !inertArgumentChar(character) {
				return false
			}
		}
		tokens++
	}
	return tokens <= maxArgumentTokens
}

// inertArgumentChar is the one character class an `args` token may use: letters,
// digits and the punctuation that means nothing to a shell, a path resolver or a
// glob (`is_inert_argument_char`, `deep_link.rs:440-443`). Everything else — `;`,
// `|`, `$`, a backtick, a quote, a glob, a tilde, a slash, any whitespace or
// control character — is a refusal.
func inertArgumentChar(character rune) bool {
	if character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' {
		return true
	}
	switch character {
	case '.', '_', '+', '=', ':', ',', '@', '%':
		return true
	}
	return false
}
