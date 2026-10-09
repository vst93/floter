package launcher

import "strings"

// splitArgs splits a command line into words the way a shell would, minus
// the parts that run code: whitespace separates, single and double quotes
// group, and a backslash escapes the next character outside single quotes.
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
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			current.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
			started = true
		case c == '\\' && i+1 < len(line):
			i++
			current.WriteByte(line[i])
			started = true
		case c == ' ' || c == '\t':
			flush()
		default:
			current.WriteByte(c)
			started = true
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
