package extensions

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

// A keyed run failure round-trips through its wire shape: the key survives and
// the payload's facts come back intact.
func TestRunFailureRoundTrips(t *testing.T) {
	failure := NewRunFailure(RunErrorInterpreterMissing, map[string]any{
		"language": "python",
		"names":    []string{"python3", "python"},
		"searched": []string{"/usr/bin", "/usr/local/bin"},
	})
	parsed, ok := ParseRunFailure(failure.Error())
	if !ok {
		t.Fatalf("the message did not parse: %q", failure.Error())
	}
	if parsed.Key != RunErrorInterpreterMissing {
		t.Errorf("key = %q", parsed.Key)
	}
	if parsed.String("language") != "python" {
		t.Errorf("language = %q", parsed.String("language"))
	}
	if got := strings.Join(parsed.Strings("names"), " / "); got != "python3 / python" {
		t.Errorf("names = %q", got)
	}
	if got := strings.Join(parsed.Strings("searched"), "\n"); got != "/usr/bin\n/usr/local/bin" {
		t.Errorf("searched = %q", got)
	}
}

// The parser is deliberately conservative, exactly as the old frontend's was:
// a known `run_` key with a JSON object payload, and nothing else.
func TestParseRunFailureIsConservative(t *testing.T) {
	cases := []struct {
		name    string
		message string
		ok      bool
		key     RunErrorKey
	}{
		{"program missing", `run_program_missing:{"path":"/opt/bin/node"}`, true, RunErrorProgramMissing},
		{"no body", `run_program_missing`, false, ""},
		{"key plus id is not claimed", `run_param_required:note`, false, ""},
		{"unknown run key", `run_something_new:{"path":"/x"}`, false, ""},
		{"prose", `extensions: the command has no program to run`, false, ""},
		{"body is not an object", `run_timeout:30`, false, ""},
		{"body is broken json", `run_timeout:{"seconds":`, false, ""},
		{"array is not an object", `run_spawn_failed:[1,2]`, false, ""},
		{"blank", ``, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, ok := ParseRunFailure(tc.message)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (%+v)", ok, tc.ok, parsed)
			}
			if ok && parsed.Key != tc.key {
				t.Errorf("key = %q, want %q", parsed.Key, tc.key)
			}
		})
	}
}

// A timeout carries its whole seconds, and an absent payload field reads as
// zero rather than fabricating one.
func TestRunFailurePayloadAccessors(t *testing.T) {
	failure := RunTimeoutFailure(30)
	if failure.String("seconds") != "" || failure.Int("seconds") != 30 {
		t.Errorf("seconds = %q / %d", failure.String("seconds"), failure.Int("seconds"))
	}
	if failure.String("path") != "" || failure.Int("path") != 0 {
		t.Errorf("an absent field invented a value: %q / %d", failure.String("path"), failure.Int("path"))
	}
	if got := len(failure.Strings("names")); got != 0 {
		t.Errorf("an absent list has %d items", got)
	}
}

// A spawn failure is mapped to the key whose remedy differs: a missing file,
// a permission refusal, and everything else (which keeps the OS's own text).
func TestRunSpawnFailureMapsTheOsError(t *testing.T) {
	missing := RunSpawnFailure("/opt/bin/node", fs.ErrNotExist)
	if missing.Key != RunErrorProgramMissing || missing.String("path") != "/opt/bin/node" {
		t.Errorf("missing = %+v", missing)
	}
	forbidden := RunSpawnFailure("/opt/bin/node", fs.ErrPermission)
	if forbidden.Key != RunErrorProgramNotExecutable || forbidden.String("path") != "/opt/bin/node" {
		t.Errorf("forbidden = %+v", forbidden)
	}
	other := RunSpawnFailure("/opt/bin/node", errors.New("bad image"))
	if other.Key != RunErrorSpawnFailed {
		t.Errorf("other key = %q", other.Key)
	}
	if other.String("detail") != "bad image" || other.String("path") != "/opt/bin/node" {
		t.Errorf("other = %+v", other)
	}
}

// A run whose program does not exist answers with the key the surface can
// localise, not an OS prose sentence.
func TestRunCapturedNamesAMissingProgram(t *testing.T) {
	entry := CommandEntry{Program: "/nonexistent/floter-does-not-exist", Args: []string{"--help"}}
	run, err := RunCaptured(context.Background(), entry, nil)
	if err == nil {
		t.Fatal("a missing program ran anyway")
	}
	parsed, ok := ParseRunFailure(err.Error())
	if !ok {
		t.Fatalf("the failure is not keyed: %q", err.Error())
	}
	if parsed.Key != RunErrorProgramMissing || parsed.String("path") != entry.Program {
		t.Errorf("failure = %+v", parsed)
	}
	if run.Success || run.ExitCode != -1 {
		t.Errorf("run = %+v", run)
	}
}
