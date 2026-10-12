package launcher

import (
	"testing"

	"floter/internal/inputtext"
)

func TestParseCommandLine(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		tokens    []string
		env       map[string]string
		cmdIndex  int
		shellExec bool
	}{
		{"plain", "git status", []string{"git", "status"}, map[string]string{}, 0, false},
		{"extra spaces", "  git   status  ", []string{"git", "status"}, map[string]string{}, 0, false},
		{"single quotes", `sh -c 'echo hi'`, []string{"sh", "-c", "echo hi"}, map[string]string{}, 0, false},
		{"double quotes", `echo "a b"`, []string{"echo", "a b"}, map[string]string{}, 0, false},
		{"escaped space", `/opt/my\ app/run`, []string{"/opt/my app/run"}, map[string]string{}, 0, false},
		// Under the POSIX syntax a backslash escapes the next character, so a
		// Windows path loses its separators: the old build ran a separate
		// "windows" syntax on Windows, whose paths use the backslash.
		{"windows path under posix", `C:\\Users\\me`, []string{`C:\Users\me`}, map[string]string{}, 0, false},
		{"double-quoted backslash", `"C:\\Users\\me"`, []string{`C:\\Users\\me`}, map[string]string{}, 0, false},
		{"env prefix", "FOO=bar git status", []string{"FOO=bar", "git", "status"}, map[string]string{"FOO": "bar"}, 1, false},
		{"two env prefixes", "A=1 B=2 git", []string{"A=1", "B=2", "git"}, map[string]string{"A": "1", "B": "2"}, 2, false},
		{"env with quote", `FOO='a b' git`, []string{"FOO=a b", "git"}, map[string]string{"FOO": "a b"}, 1, false},
		{"pipe", "a | b", []string{"a", "|", "b"}, map[string]string{}, 0, true},
		{"redirect", "a > out", []string{"a", ">", "out"}, map[string]string{}, 0, true},
		{"quote keeps pipe", `sh -c 'a | b'`, []string{"sh", "-c", "a | b"}, map[string]string{}, 0, false},
		{"assignments only", "FOO=bar", []string{"FOO=bar"}, map[string]string{"FOO": "bar"}, -1, false},
		{"quoted pipe is not syntax", `"a|b"`, []string{"a|b"}, map[string]string{}, 0, false},
	}
	for _, tc := range cases {
		parsed := ParseCommandLine(tc.line)
		if len(parsed.Tokens) != len(tc.tokens) {
			t.Fatalf("%s: tokens = %v, want %v", tc.name, parsed.Tokens, tc.tokens)
		}
		for i, token := range tc.tokens {
			if parsed.Tokens[i] != token {
				t.Fatalf("%s: tokens = %v, want %v", tc.name, parsed.Tokens, tc.tokens)
			}
		}
		if len(parsed.Environment) != len(tc.env) {
			t.Errorf("%s: environment = %v, want %v", tc.name, parsed.Environment, tc.env)
		}
		for key, value := range tc.env {
			if parsed.Environment[key] != value {
				t.Errorf("%s: environment[%q] = %q", tc.name, key, parsed.Environment[key])
			}
		}
		if parsed.CommandIndex != tc.cmdIndex {
			t.Errorf("%s: command index = %d, want %d", tc.name, parsed.CommandIndex, tc.cmdIndex)
		}
		if parsed.ShellSyntax != tc.shellExec {
			t.Errorf("%s: shell syntax = %v, want %v", tc.name, parsed.ShellSyntax, tc.shellExec)
		}
	}
}

// TestUnicodeSeparatorsAreWordGaps: a shell tokenizes on U+0020 only, so a
// query typed with a full-width space (what a Chinese input method's space bar
// produces) must be two tokens here — one fused token reaches the program as
// the argv[0] "go version", which no machine has.
func TestUnicodeSeparatorsAreWordGaps(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		tokens []string
		index  int
	}{
		{"ideographic space", "go\u3000version", []string{"go", "version"}, 0},
		{"no-break space", "npm\u00a0install\u00a0left-pad", []string{"npm", "install", "left-pad"}, 0},
		{"thin space", "git\u2009status", []string{"git", "status"}, 0},
		{"mixed", "git\u3000status --short\u2009-u", []string{"git", "status", "--short", "-u"}, 0},
		{"inside quotes", "sh -c \"echo\u3000hi\"", []string{"sh", "-c", "echo hi"}, 0},
		{"env prefix", "FOO=bar\u3000git", []string{"FOO=bar", "git"}, 1},
		{"separator around a pipe", "a\u3000|\u3000b", []string{"a", "|", "b"}, 0},
	}
	for _, tc := range cases {
		parsed := ParseCommandLine(tc.line)
		if len(parsed.Tokens) != len(tc.tokens) {
			t.Fatalf("%s: tokens = %q, want %q", tc.name, parsed.Tokens, tc.tokens)
		}
		for i, token := range tc.tokens {
			if parsed.Tokens[i] != token {
				t.Errorf("%s: tokens = %q, want %q", tc.name, parsed.Tokens, tc.tokens)
				break
			}
		}
		if parsed.CommandIndex != tc.index {
			t.Errorf("%s: command index = %d, want %d", tc.name, parsed.CommandIndex, tc.index)
		}
	}
	// The separator becomes the word gap and nothing else: a pipe between two
	// words is still shell syntax, and tabs and newlines keep their own
	// meaning.
	if parsed := ParseCommandLine("a\u3000|\u3000b"); !parsed.ShellSyntax {
		t.Error("a separator must not hide the shell syntax beside it")
	}
	if parsed := ParseCommandLine("a\tb"); len(parsed.Tokens) != 2 {
		t.Errorf("a tab is still a gap: %q", parsed.Tokens)
	}
}

// TestTheTokenizerNormalizesThroughOneFunction pins the boundary: the rule
// lives in internal/inputtext and the parser calls it, so the parser cannot
// grow a second table of separators that drifts from the paste path's.
func TestTheTokenizerNormalizesThroughOneFunction(t *testing.T) {
	line := "go\u3000version\u00a0--flag"
	parsed := ParseCommandLine(line)
	normalized := ParseCommandLine(inputtext.NormalizeSpaces(line))
	if len(parsed.Tokens) != len(normalized.Tokens) {
		t.Fatalf("normalized twice: %q vs %q", parsed.Tokens, normalized.Tokens)
	}
	for i := range parsed.Tokens {
		if parsed.Tokens[i] != normalized.Tokens[i] {
			t.Errorf("token %d: %q vs %q", i, parsed.Tokens[i], normalized.Tokens[i])
		}
	}
}
