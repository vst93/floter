package extensions

import (
	"runtime"
	"strings"
	"testing"
)

// param is a minimal declaration, the shape most cases start from.
func param(id, kind string) ParamDefinition {
	return ParamDefinition{ID: id, Kind: kind}
}

// argsFor runs one declaration against a set of answers.
func argsFor(t *testing.T, declaration ParamDefinition, values ParamValues, cmdScript bool) []string {
	t.Helper()
	args, err := ParamArguments([]ParamDefinition{declaration}, values, cmdScript)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	return args
}

// refusal runs one declaration and returns the key, failing when it was
// accepted.
func refusal(t *testing.T, declaration ParamDefinition, values ParamValues, cmdScript bool) string {
	t.Helper()
	_, err := ParamArguments([]ParamDefinition{declaration}, values, cmdScript)
	if err == nil {
		t.Fatalf("accepted %v", values)
	}
	return err.Error()
}

func equalArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argv = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q (argv = %q)", i, got[i], want[i], got)
		}
	}
}

// Declaration order is argv order, and a flag becomes its OWN item — never
// `--flag value` joined into one. Mutation: join flag and value and this
// collapses.
func TestParamArgumentsFollowTheDeclaredOrder(t *testing.T) {
	flagged := param("target", ParamText)
	flagged.Flag = "--target"
	positional := param("rest", ParamText)
	args, err := ParamArguments([]ParamDefinition{flagged, positional}, ParamValues{
		"target": "example.com",
		"rest":   "extra",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	equalArgs(t, args, []string{"--target", "example.com", "extra"})

	// A value with spaces stays exactly one item: nothing is re-split, and
	// nothing is quoted into a command string.
	spaced := param("target", ParamText)
	spaced.Flag = "--target"
	equalArgs(t, argsFor(t, spaced, ParamValues{"target": "two words"}, false),
		[]string{"--target", "two words"})
}

// A default seeds an absent optional; a required parameter with no value and no
// default is refused, and an empty string counts as "not supplied".
func TestParamArgumentsSkipOptionalsAndRefuseMissingRequired(t *testing.T) {
	optional := param("note", ParamText)
	equalArgs(t, argsFor(t, optional, ParamValues{}, false), nil)

	defaulted := optional
	defaulted.Default = "hi"
	equalArgs(t, argsFor(t, defaulted, ParamValues{}, false), []string{"hi"})

	required := optional
	required.Required = true
	if key := refusal(t, required, ParamValues{}, false); key != "run_param_required:note" {
		t.Fatalf("key = %q", key)
	}
	equalArgs(t, argsFor(t, required, ParamValues{"note": "x"}, false), []string{"x"})
	if key := refusal(t, required, ParamValues{"note": ""}, false); key != "run_param_required:note" {
		t.Fatalf("key = %q", key)
	}
	// A supplied empty value still falls back to the default.
	withDefault := required
	withDefault.Default = "fallback"
	equalArgs(t, argsFor(t, withDefault, ParamValues{"note": ""}, false), []string{"fallback"})
}

// Values are converted per kind: number trimmed and finite, select on the list,
// boolean only true/false, text and path untouched.
func TestParamArgumentsConvertByKind(t *testing.T) {
	count := param("count", ParamNumber)
	equalArgs(t, argsFor(t, count, ParamValues{"count": "12"}, false), []string{"12"})
	equalArgs(t, argsFor(t, count, ParamValues{"count": " -3.5 "}, false), []string{"-3.5"})
	if key := refusal(t, count, ParamValues{"count": "three"}, false); key != "run_param_invalid:count" {
		t.Fatalf("key = %q", key)
	}

	mode := param("mode", ParamSelect)
	mode.Options = []string{"fast", "slow"}
	equalArgs(t, argsFor(t, mode, ParamValues{"mode": "fast"}, false), []string{"fast"})
	if key := refusal(t, mode, ParamValues{"mode": "turbo"}, false); key != "run_param_invalid:mode" {
		t.Fatalf("key = %q", key)
	}

	force := param("force", ParamBoolean)
	force.Flag = "--force"
	equalArgs(t, argsFor(t, force, ParamValues{"force": "true"}, false), []string{"--force"})
	equalArgs(t, argsFor(t, force, ParamValues{"force": "false"}, false), nil)
	equalArgs(t, argsFor(t, force, ParamValues{}, false), nil)
	// Only the two spellings the switch produces; "yes"/"1" are not booleans.
	if key := refusal(t, force, ParamValues{"force": "yes"}, false); key != "run_param_invalid:force" {
		t.Fatalf("key = %q", key)
	}

	path := param("dest", ParamPath)
	equalArgs(t, argsFor(t, path, ParamValues{"dest": "$HOME/x ~/y"}, false), []string{"$HOME/x ~/y"})
}

// The number kind is a finite decimal: the non-finite and radix-prefixed
// spellings are refused, the plain decimal spellings pass through trimmed.
// Mutation: replace IsFiniteDecimal with strconv.ParseFloat and the inf/NaN
// cases go green.
func TestParamArgumentsRefuseANonFiniteNumber(t *testing.T) {
	count := param("count", ParamNumber)
	for _, value := range []string{
		"inf", "-inf", "NaN", "nan", "infinity", "0x10", "0x1p-2", "0b101", "0o17",
		"1e", "1.2.3", "1_000",
	} {
		if key := refusal(t, count, ParamValues{"count": value}, false); key != "run_param_invalid:count" {
			t.Errorf("%q: key = %q", value, key)
		}
	}
	for _, value := range []string{"1e5", "-2.5", ".5", "1.", "+3", "0", " -2.5 "} {
		equalArgs(t, argsFor(t, count, ParamValues{"count": value}, false), []string{strings.TrimSpace(value)})
	}
}

// An answer keyed by an id no parameter declares is refused: silently dropping
// it would hide a caller bug, treating it as free-form would be an injection
// path. Mutation: drop the unknown-key loop and this passes.
func TestParamArgumentsRefuseAnUndeclaredKey(t *testing.T) {
	_, err := ParamArguments([]ParamDefinition{param("known", ParamText)}, ParamValues{
		"known":  "ok",
		"sneaky": "--rm",
	}, false)
	if err == nil || err.Error() != "run_param_unknown:sneaky" {
		t.Fatalf("err = %v", err)
	}
}

// A hand-edited manifest cannot smuggle a space (two argv items) through the
// run path: the assembler re-runs the flag validator.
func TestParamArgumentsRefuseAMalformedFlag(t *testing.T) {
	bad := param("target", ParamText)
	bad.Flag = "--target value"
	if key := refusal(t, bad, ParamValues{"target": "x"}, false); key != "run_param_invalid:target" {
		t.Fatalf("key = %q", key)
	}
}

// The Windows `.cmd`/`.bat` hole: a metacharacter anywhere refuses the run, and
// the SAME value is fine for a non-cmd program.
func TestParamArgumentsRefuseCmdMetacharactersForACmdScript(t *testing.T) {
	text := param("target", ParamText)
	for _, value := range []string{"a&b", "a|b", "a<b", "a>b", "a^b", "a%b"} {
		if key := refusal(t, text, ParamValues{"target": value}, true); key != "run_param_windows_unsafe:target" {
			t.Errorf("%q: key = %q", value, key)
		}
		equalArgs(t, argsFor(t, text, ParamValues{"target": value}, false), []string{value})
	}
}

// The keyed refusal round-trips, and the parser is conservative: it claims
// only the four `run_param_*` keys with a non-empty id.
func TestParamFailureRoundTrip(t *testing.T) {
	for _, key := range []RunErrorKey{
		RunErrorParamUnknown, RunErrorParamRequired, RunErrorParamInvalid, RunErrorParamWindowsUnsafe,
	} {
		failure := &ParamFailure{Key: key, Param: "note"}
		if failure.Error() != string(key)+":note" {
			t.Errorf("Error() = %q", failure.Error())
		}
		parsed, ok := ParseParamFailure(failure.Error())
		if !ok || parsed.Key != key || parsed.Param != "note" {
			t.Errorf("ParseParamFailure(%q) = %+v, %v", failure.Error(), parsed, ok)
		}
	}
	for _, message := range []string{
		"run_param_required:", "run_param_bogus:note", "run_timeout:{\"seconds\":3}",
		"plain prose", "run_param_required", "",
	} {
		if _, ok := ParseParamFailure(message); ok {
			t.Errorf("%q was claimed", message)
		}
	}
	// The run-failure parser does not claim the id-shaped refusals.
	if _, ok := ParseRunFailure("run_param_required:note"); ok {
		t.Error("ParseRunFailure claimed a param refusal")
	}
}

// The finite-decimal grammar matches the old frontend's regexp, not Go's
// ParseFloat: hex floats, underscores and the non-finite spellings are out.
func TestIsFiniteDecimal(t *testing.T) {
	good := []string{"0", "12", "-2.5", "+3", ".5", "1.", "1e5", "1E-5", " 2 ", "1.5e+3"}
	for _, value := range good {
		if !IsFiniteDecimal(value) {
			t.Errorf("%q was refused", value)
		}
	}
	bad := []string{"", " ", "inf", "NaN", "infinity", "0x1p-2", "0b101", "1_000", "1e", "1.2.3", "e5", "--1"}
	for _, value := range bad {
		if IsFiniteDecimal(value) {
			t.Errorf("%q was accepted", value)
		}
	}
}

// The cmd.exe unsafe set is exactly `& | < > ^ %`.
func TestWindowsCmdUnsafeChar(t *testing.T) {
	for _, value := range []string{"a&b", "a|b", "a<b", "a>b", "a^b", "a%b"} {
		if _, unsafe := WindowsCmdUnsafeChar(value); !unsafe {
			t.Errorf("%q was called safe", value)
		}
	}
	for _, value := range []string{"plain", "a-b", "a b", "a$b", "a;b", ""} {
		if char, unsafe := WindowsCmdUnsafeChar(value); unsafe {
			t.Errorf("%q was called unsafe (%q)", value, char)
		}
	}
}

// The cmd-script spelling check is pure; the platform gate is separate so the
// extension rule is testable everywhere.
func TestCmdScriptNames(t *testing.T) {
	for _, program := range []string{"runner.cmd", "runner.CMD", "C:\\tools\\build.bat", "a/B.BaT"} {
		if !IsCmdScriptName(program) {
			t.Errorf("%q was not a cmd script", program)
		}
	}
	for _, program := range []string{"runner.exe", "runner.sh", "runner", "runner.cmd.txt", "cmdx"} {
		if IsCmdScriptName(program) {
			t.Errorf("%q was a cmd script", program)
		}
	}
	// Off Windows the wrap never happens, so the platform gate is always
	// false, whatever the spelling.
	if runtime.GOOS == "windows" {
		if !ProgramIsCmdScript("runner.cmd") {
			t.Error("ProgramIsCmdScript missed a cmd script on Windows")
		}
	} else if ProgramIsCmdScript("runner.cmd") {
		t.Error("ProgramIsCmdScript claimed a cmd script off Windows")
	}
}
