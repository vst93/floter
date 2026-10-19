package launcher

import (
	"regexp"
	"strings"

	"floter/internal/inputtext"
)

// Parsing the field's text as a command line, for the shell action: the
// tokens, the leading NAME=value environment assignments (which ride the
// command's environment rather than being re-interpreted by a shell), and
// whether the line carries shell syntax that only a real shell can run.
//
// Backslash is a path separator on Windows, not a general escape: treating it
// like POSIX syntax would turn C:\Users into C:Users before the execution
// reaches the terminal.

var environmentAssignment = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// CommandLine is a parsed field text.
type CommandLine struct {
	// Tokens are the words the line is made of.
	Tokens []string
	// Environment holds the leading NAME=value assignments, decoded.
	Environment map[string]string
	// CommandIndex is where the executable token sits, -1 when the line has
	// only assignments.
	CommandIndex int
	// ShellSyntax marks lines with unquoted operators (| & ; < > ( )), which
	// only a real shell can run.
	ShellSyntax bool
}

// ParseCommandLine tokenizes a command line without asking a shell to
// interpret it.
//
// The line is normalized first (internal/inputtext): every Unicode space
// separator becomes U+0020, because that is the only byte a shell reads as a
// word gap. Without it `go<U+3000>version` — what a Chinese input method's
// space bar types — is one token here and one word to the shell, which answers
// `command not found: go version`. The reference normalized the line at the
// boundary for the same reason; this tokenizer is that boundary for a command
// the app hands to a program as argv.
func ParseCommandLine(value string) CommandLine {
	value = inputtext.NormalizeSpaces(value)
	// `Environment` starts nil and is built on the first leading assignment: a
	// line with none (every ordinary command, and the action bar's own row on
	// every frame — see actionbar.go) then allocates no map at all, and the
	// field's zero value already answers `len(env) == 0` and a missing key.
	parsed := CommandLine{CommandIndex: -1}
	var token strings.Builder
	tokenStarted, escaped, shellSyntax := false, false, false
	quote := byte(0)
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch {
		case escaped:
			token.WriteByte(char)
			escaped = false
		case char == '\\' && quote != '\'':
			// POSIX escape: the next character is literal. Windows keeps the
			// backslash (its paths use it), which is why the escape only
			// applies outside single quotes.
			escaped = true
			if quote == '"' {
				token.WriteByte(char)
				tokenStarted = true
			}
		case char == '\'' || char == '"':
			if !tokenStarted {
				tokenStarted = true
			}
			if quote == 0 {
				quote = char
			} else if quote == char {
				quote = 0
			} else {
				token.WriteByte(char)
			}
		case char == ' ' || char == '\t':
			if quote == 0 {
				// The word ends here. Inlined rather than a closure: the token
				// is finished once per word, and a captured `finish` made the
				// builder escape on every call (one allocation per line, on the
				// frame's path).
				if tokenStarted {
					parsed.Tokens = append(parsed.Tokens, token.String())
					token.Reset()
					tokenStarted = false
				}
			} else {
				token.WriteByte(char)
			}
		case strings.IndexByte("|&;<>()", char) >= 0 && quote == 0:
			shellSyntax = true
			token.WriteByte(char)
			tokenStarted = true
		default:
			token.WriteByte(char)
			tokenStarted = true
		}
	}
	// The last word ends at the end of the line.
	if tokenStarted {
		parsed.Tokens = append(parsed.Tokens, token.String())
		token.Reset()
		tokenStarted = false
	}

	for index, token := range parsed.Tokens {
		// `FindStringSubmatchIndex` and not `FindStringSubmatch`: the two
		// halves are offsets into the token, so a line's words are classified
		// without a []string per word on the frame's path.
		if at := environmentAssignment.FindStringSubmatchIndex(token); at != nil {
			if parsed.Environment == nil {
				parsed.Environment = make(map[string]string, 2)
			}
			parsed.Environment[token[at[2]:at[3]]] = token[at[4]:at[5]]
			continue
		}
		parsed.CommandIndex = index
		break
	}
	parsed.ShellSyntax = shellSyntax
	return parsed
}
