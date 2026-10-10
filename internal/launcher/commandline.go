package launcher

import (
	"regexp"
	"strings"
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
func ParseCommandLine(value string) CommandLine {
	parsed := CommandLine{Environment: map[string]string{}, CommandIndex: -1}
	var token strings.Builder
	tokenStarted, escaped, shellSyntax := false, false, false
	quote := byte(0)

	finish := func() {
		if tokenStarted {
			parsed.Tokens = append(parsed.Tokens, token.String())
			token.Reset()
			tokenStarted = false
		}
	}
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
				finish()
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
	finish()

	for index, token := range parsed.Tokens {
		if assignment := environmentAssignment.FindStringSubmatch(token); assignment != nil {
			parsed.Environment[assignment[1]] = assignment[2]
			continue
		}
		parsed.CommandIndex = index
		break
	}
	parsed.ShellSyntax = shellSyntax
	return parsed
}
