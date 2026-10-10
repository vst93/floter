package extensions

import (
	"strings"
	"testing"
)

// A declaration is validated wherever it is parsed: a bad id, a duplicate, a
// select with no choices, a boolean with no flag, and a flag that could split
// into two argv entries are all refused.
func TestValidateParams(t *testing.T) {
	good := []ParamDefinition{
		{ID: "target", Label: "Target", Kind: ParamText, Flag: "--target"},
		{ID: "count", Kind: ParamNumber},
		{ID: "verbose", Kind: ParamBoolean, Flag: "--verbose"},
		{ID: "mode", Kind: ParamSelect, Options: []string{"fast", "slow"}},
		{ID: "file", Kind: ParamPath, Flag: "--file"},
	}
	if err := ValidateParams(good); err != nil {
		t.Fatalf("a good declaration was refused: %v", err)
	}
	bad := map[string][]ParamDefinition{
		"an empty id":            {{ID: "", Kind: ParamText}},
		"an id with a slash":     {{ID: "a/b", Kind: ParamText}},
		"a duplicate id":         {{ID: "a", Kind: ParamText}, {ID: "a", Kind: ParamText}},
		"an unknown kind":        {{ID: "a", Kind: "colour"}},
		"a select with none":     {{ID: "a", Kind: ParamSelect}},
		"a boolean with no flag": {{ID: "a", Kind: ParamBoolean}},
		"a flag with no dash":    {{ID: "a", Kind: ParamText, Flag: "target"}},
		"a flag with a space":    {{ID: "a", Kind: ParamText, Flag: "--tar get"}},
		"a flag with a dollar":   {{ID: "a", Kind: ParamText, Flag: "--ta$get"}},
	}
	for name, params := range bad {
		if err := ValidateParams(params); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The argv half: each flag its own element, each value its own, a boolean's
// flag present only when it is true, and nothing ever concatenated.
func TestParamArgv(t *testing.T) {
	params := []ParamDefinition{
		{ID: "target", Kind: ParamText, Flag: "--target"},
		{ID: "count", Kind: ParamNumber, Flag: "--count"},
		{ID: "verbose", Kind: ParamBoolean, Flag: "--verbose"},
		{ID: "loose", Kind: ParamText},
	}
	argv := ParamArgv(params, ParamValues{
		"target":  "a b c",
		"count":   "3",
		"verbose": "true",
		"loose":   "value",
	})
	want := []string{"--target", "a b c", "--count", "3", "--verbose", "value"}
	if len(argv) != len(want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	for i, value := range want {
		if argv[i] != value {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], value)
		}
	}
	// A value with a space stays one element: nothing is re-split, and nothing
	// is quoted into a command string.
	if argv[1] != "a b c" {
		t.Errorf("a value with spaces was split: %q", argv[1])
	}
	// An off boolean contributes nothing.
	if argv := ParamArgv(params, ParamValues{"verbose": "false"}); len(argv) != 0 {
		t.Errorf("a false boolean contributed %q", argv)
	}
	// An empty value contributes nothing either.
	if argv := ParamArgv(params, ParamValues{"target": "  "}); len(argv) != 0 {
		t.Errorf("an empty value contributed %q", argv)
	}
}

// The form's own validation: the missing required values, the numbers that are
// not numbers, the choices off the list.
func TestParamIssues(t *testing.T) {
	params := []ParamDefinition{
		{ID: "target", Kind: ParamText, Required: true},
		{ID: "count", Kind: ParamNumber},
		{ID: "mode", Kind: ParamSelect, Options: []string{"fast", "slow"}},
		{ID: "file", Kind: ParamPath},
	}
	// Everything good.
	issues := ParamIssues(params, ParamValues{"target": "x", "count": "2", "mode": "fast"})
	if len(issues) != 0 {
		t.Errorf("a good set has issues: %v", issues)
	}
	// A missing required value, a number that is not one, a choice off the
	// list, and an optional empty value that is fine.
	issues = ParamIssues(params, ParamValues{"count": "two", "mode": "medium"})
	if len(issues) != 3 {
		t.Fatalf("issues = %v", issues)
	}
	joined := strings.Join(issues, ",")
	for _, want := range []string{"target", "count", "mode"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%q is not in %v", want, issues)
		}
	}
}

// The defaults seed a form: a parameter's own, and a select's first choice so
// it cannot start blank and invalid.
func TestParamDefaults(t *testing.T) {
	params := []ParamDefinition{
		{ID: "target", Kind: ParamText, Default: "here"},
		{ID: "count", Kind: ParamNumber},
		{ID: "mode", Kind: ParamSelect, Options: []string{"fast", "slow"}},
	}
	values := ParamDefaults(params)
	if values["target"] != "here" || values["mode"] != "fast" {
		t.Errorf("defaults = %v", values)
	}
	if _, ok := values["count"]; ok {
		t.Errorf("a parameter with no default got one: %v", values)
	}
}
