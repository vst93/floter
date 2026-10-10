package launcher

import "testing"

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
