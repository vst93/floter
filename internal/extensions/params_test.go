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
