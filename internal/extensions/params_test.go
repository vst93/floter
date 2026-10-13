package extensions

import (
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

// The form's own validation: the missing required values, the numbers that are
// not finite decimals, the choices off the list, and the booleans that are not
// true or false — each naming the row it belongs to and the label to print.
func TestParamIssues(t *testing.T) {
	params := []ParamDefinition{
		{ID: "target", Label: "Target", Kind: ParamText, Required: true},
		{ID: "count", Kind: ParamNumber},
		{ID: "mode", Kind: ParamSelect, Options: []string{"fast", "slow"}},
		{ID: "flag", Kind: ParamBoolean},
		{ID: "file", Kind: ParamPath},
	}
	// Everything good: a filled required value, a finite number, a listed
	// choice, a boolean, and an empty optional.
	good := ParamValues{"target": "x", "count": "2", "mode": "fast", "flag": "true"}
	if issues := ParamIssues(params, good); len(issues) != 0 {
		t.Errorf("a good set has issues: %v", issues)
	}
	// A missing required value, a number that is not one, a choice off the
	// list, a boolean that is neither word, and an optional empty value that
	// is fine.
	issues := ParamIssues(params, ParamValues{"count": "two", "mode": "medium", "flag": "maybe"})
	if len(issues) != 4 {
		t.Fatalf("issues = %v", issues)
	}
	want := []ParamValueIssue{
		{Index: 0, Key: ParamIssueRequiredMissing, Label: "Target"},
		{Index: 1, Key: ParamIssueInvalidValue, Label: "count"},
		{Index: 2, Key: ParamIssueInvalidValue, Label: "mode"},
		{Index: 3, Key: ParamIssueInvalidValue, Label: "flag"},
	}
	for i, expected := range want {
		if issues[i] != expected {
			t.Errorf("issue %d = %+v, want %+v", i, issues[i], expected)
		}
	}
	// A label-less parameter falls back to its id, and a blank optional never
	// earns an issue.
	issues = ParamIssues(params, ParamValues{})
	if len(issues) != 1 || issues[0].Index != 0 || issues[0].Label != "Target" {
		t.Fatalf("issues = %v", issues)
	}
}

// The number grammar is the backend's finite-decimal pattern, not
// `strconv.ParseFloat`: the non-finite and radix-prefixed spellings the host
// refuses are refused here too, so the form never lets through what the
// authority would reject.
func TestParamIssuesRefusesWhatTheBackendRefuses(t *testing.T) {
	params := []ParamDefinition{{ID: "count", Kind: ParamNumber}}
	for _, bad := range []string{"inf", "NaN", "Infinity", "0x1p-2", "1_000"} {
		if issues := ParamIssues(params, ParamValues{"count": bad}); len(issues) != 1 {
			t.Errorf("ParamIssues(%q) = %v, want one issue", bad, issues)
		}
	}
	for _, good := range []string{"2", "-1.5", ".5", "1e3", "  4  "} {
		if issues := ParamIssues(params, ParamValues{"count": good}); len(issues) != 0 {
			t.Errorf("ParamIssues(%q) = %v, want none", good, issues)
		}
	}
}

// A required parameter with a default is not missing: the default is the value
// the backend would run, so the form must not refuse it.
func TestARequiredDefaultIsNotMissing(t *testing.T) {
	params := []ParamDefinition{{ID: "target", Kind: ParamText, Required: true, Default: "here"}}
	if issues := ParamIssues(params, ParamValues{}); len(issues) != 0 {
		t.Errorf("a required-with-default was flagged: %v", issues)
	}
	if issues := ParamIssues(params, ParamValues{"target": ""}); len(issues) != 0 {
		t.Errorf("a blank with a default was flagged: %v", issues)
	}
	// A required parameter with no default still is.
	params[0].Default = ""
	if issues := ParamIssues(params, ParamValues{}); len(issues) != 1 {
		t.Errorf("a required without a default was not flagged: %v", issues)
	}
}

// The declared-input editor's own validation: the same rules the backend
// enforces, answered by row so the view can print each reason under the row
// that caused it — and a row with no id earns nothing, because it is not a
// declaration yet.
func TestParamRowIssues(t *testing.T) {
	good := []ParamDefinition{
		{ID: "target", Kind: ParamText, Flag: "--target"},
		{ID: "verbose", Kind: ParamBoolean, Flag: "--verbose"},
		{ID: "mode", Kind: ParamSelect, Options: []string{"fast"}},
	}
	if issues := ParamRowIssues(good); len(issues) != 0 {
		t.Errorf("a good declaration has issues: %v", issues)
	}

	cases := []struct {
		name  string
		param ParamDefinition
		want  ParamRowIssue
	}{
		{"an id with a space", ParamDefinition{ID: "tar get", Kind: ParamText}, ParamIssueID},
		{"a select with no options", ParamDefinition{ID: "mode", Kind: ParamSelect}, ParamIssueOptions},
		{"a boolean with no flag", ParamDefinition{ID: "verbose", Kind: ParamBoolean}, ParamIssueFlag},
		{"a flag with a space", ParamDefinition{ID: "target", Kind: ParamText, Flag: "--tar get"}, ParamIssueBadFlag},
	}
	for _, tc := range cases {
		issues := ParamRowIssues([]ParamDefinition{tc.param})
		if issues[0] != tc.want {
			t.Errorf("%s: issue = %q, want %q", tc.name, issues[0], tc.want)
		}
	}

	// The later of two rows sharing an id is the one named; the first is left
	// to its own rules.
	issues := ParamRowIssues([]ParamDefinition{
		{ID: "dup", Kind: ParamText},
		{ID: "dup", Kind: ParamText},
	})
	if _, ok := issues[0]; ok {
		t.Errorf("the first row was blamed for the duplicate: %v", issues)
	}
	if issues[1] != ParamIssueDuplicate {
		t.Errorf("the duplicate row = %q, want %q", issues[1], ParamIssueDuplicate)
	}

	// A row still being typed is not a declaration, so it earns no issue even
	// when its kind would otherwise need one.
	if issues := ParamRowIssues([]ParamDefinition{{Kind: ParamSelect}}); len(issues) != 0 {
		t.Errorf("an unnamed row earned an issue: %v", issues)
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
