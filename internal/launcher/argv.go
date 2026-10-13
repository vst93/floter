package launcher

import (
	"strings"
	"unicode/utf8"
)

// splitArgs splits a command line into words the way a shell would, minus
// the parts that run code: whitespace separates, single and double quotes
// group, and a backslash escapes the next character outside single quotes.
//
// "Whitespace" is the launcher's own word-gap set (JavaScript's `\s`, see
// wordgap.go), not the shell's U+0020: a full-width space from a Chinese input
// method separates words here exactly as it separates trigger words, and a
// multi-byte gap is decoded as one rune rather than three bytes.
//
// It is what turns the command mode's field into the argv handed to the
// terminal. An unterminated quote takes the rest of the line, so a
// half-typed argument never drops out from under the user.
func splitArgs(line string) []string {
	var (
		out     []string
		current strings.Builder
		started bool
		quote   byte
	)
	flush := func() {
		if started {
			out = append(out, current.String())
			current.Reset()
			started = false
		}
	}
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case quote != 0:
			i++
			if c == quote {
				quote = 0
				continue
			}
			current.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
			started = true
			i++
		case c == '\\' && i+1 < len(line):
			i++
			current.WriteByte(line[i])
			started = true
			i++
		default:
			r, size := utf8.DecodeRuneInString(line[i:])
			if isWordGap(r) {
				flush()
			} else {
				current.WriteString(line[i : i+size])
				started = true
			}
			i += size
		}
	}
	flush()
	return out
}

// firstWord is the first word of a line, empty when there is none.
func firstWord(line string) string {
	words := splitArgs(line)
	if len(words) == 0 {
		return ""
	}
	return words[0]
}
