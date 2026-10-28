// Package textlines is the one reader of "a text blob, as lines".
//
// Every text file the app parses and every child process's stdout it reads is
// a blob of lines, and the framing of a line differs per platform: a Windows
// tool ends every line with "\r\n", and a file saved by a Windows editor may
// start with a UTF-8 BOM (U+FEFF). Both are easy to produce and invisible on
// screen, and a reader that only splits on "\n" carries them into the last
// token of every line — where they are not whitespace to any of the tokenizers
// downstream:
//
//   - `strings.TrimSpace` does strip a trailing "\r", but a tokenizer that
//     splits on spaces and then classifies each token sees "-n\r" and "<NUM>\r"
//     as different tokens than "-n" and "<NUM>". On a Windows host that turned
//     `--help` options into flags, replaced their descriptions with the value
//     placeholder, and dropped every option that had no description at all.
//   - `U+FEFF` is not whitespace (it is Cf, not Zs), so a BOM'd `.desktop`
//     file's first line is "\ufeff[Desktop Entry]" — which is not the section
//     header, so the whole entry read as empty.
//
// So the framing is removed at one place, before any parser sees a line.
// Scope is deliberate:
//
//   - Only "\n" ends a line. A lone "\r" (classic Mac OS) does not: no tool the
//     app talks to produces it, and splitting on it would break a line that
//     legitimately carries "\r" (a progress bar's redraw).
//   - Exactly one trailing "\r" is removed, and exactly one leading BOM from
//     the first line. A second "\r" or a BOM mid-file is content, not framing.
//   - An empty blob is one empty line, which is what `strings.Split` says and
//     what every caller already handled.
package textlines

import "strings"

// bom is the UTF-8 byte order mark, as a string so it can be trimmed.
const bom = "\ufeff"

// Split cuts a text blob into lines with the platform framing removed: each
// line loses one trailing carriage return (the second half of a CRLF), and the
// first line loses one leading byte order mark.
//
// The returned slice always has at least one element; a blob that ends with a
// newline ends with an empty line, which callers skip as they already skip
// blank lines.
func Split(text string) []string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	if len(lines) > 0 {
		lines[0] = strings.TrimPrefix(lines[0], bom)
	}
	return lines
}
