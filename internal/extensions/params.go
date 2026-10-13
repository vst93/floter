package extensions

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Declared inputs: a manifest may name the values a run needs, and the host
// renders a form for them and turns the answers into argv. The declaration
// travels with the manifest — the digest, the export, the approval — so
// editing it invalidates an approval on purpose: the user asked for the input,
// so the review has to be repeated.
//
// The argv half is the security boundary: every flag becomes exactly one
// element of the argv, and a flag is validated to be one token by
// construction, because a malformed flag could split into two.

// ParamDefinition is one declared input.
type ParamDefinition struct {
	// ID is the stable key the values are stored and deduped by. It is never
	// handed to the tool.
	ID string `json:"id"`
	// Label is the display name; empty falls back to the id.
	Label string `json:"label"`
	// Kind is text, number, boolean, select or path.
	Kind string `json:"kind"`
	// Default pre-fills the input. It is not an exemption: a required
	// parameter with a default still counts as required.
	Default string `json:"default,omitempty"`
	// Required says the run may not proceed without a value.
	Required bool `json:"required,omitempty"`
	// Placeholder is the field's own hint.
	Placeholder string `json:"placeholder,omitempty"`
	// Options are the choices for a select; empty for every other kind.
	Options []string `json:"options,omitempty"`
	// Flag is the argv prefix (`--target`). Empty means a positional value.
	Flag string `json:"flag,omitempty"`
}

// The parameter kinds.
const (
	ParamText    = "text"
	ParamNumber  = "number"
	ParamBoolean = "boolean"
	ParamSelect  = "select"
	ParamPath    = "path"
)

// ErrBadParam is what a declaration's validator reports.
var ErrBadParam = errors.New("extensions: the parameter declaration is not usable")

// ValidateParams is the one validator for a `params` array: a manifest is
// checked wherever it is installed or generated, so a hand-written one and one
// the connect form produced meet the same rule.
func ValidateParams(params []ParamDefinition) error {
	seen := map[string]bool{}
	for _, param := range params {
		if param.ID == "" || !isParamID(param.ID) {
			return fmt.Errorf("%w: bad id %q", ErrBadParam, param.ID)
		}
		if seen[param.ID] {
			return fmt.Errorf("%w: duplicate id %q", ErrBadParam, param.ID)
		}
		seen[param.ID] = true
		switch param.Kind {
		case ParamText, ParamNumber, ParamBoolean, ParamPath:
		case ParamSelect:
			if len(param.Options) == 0 {
				return fmt.Errorf("%w: %s is a select with no choices", ErrBadParam, param.ID)
			}
		default:
			return fmt.Errorf("%w: %s has kind %q", ErrBadParam, param.ID, param.Kind)
		}
		if param.Kind == ParamBoolean && param.Flag == "" {
			// A boolean's value *is* its flag; a positional boolean would have
			// to be "true"/"false" in argv, which is a text parameter.
			return fmt.Errorf("%w: %s is a boolean with no flag", ErrBadParam, param.ID)
		}
		if param.Flag != "" {
			if err := validateParamFlag(param.Flag); err != nil {
				return err
			}
		}
	}
	return nil
}

// isParamID is the id charset: letters, digits, dot, underscore, dash.
func isParamID(id string) bool {
	for _, char := range id {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '.', char == '_', char == '-':
		default:
			return false
		}
	}
	return true
}

// validateParamFlag checks a flag is one argv token: a leading `-` followed by
// characters that cannot open a shell metacharacter, a space or a quote. It is
// the foundation of the injection defence — the runtime appends each flag as
// its own argv element, and this is what guarantees that element is still
// exactly one flag.
func validateParamFlag(flag string) error {
	if !strings.HasPrefix(flag, "-") || len(flag) < 2 {
		return fmt.Errorf("%w: %q is not a flag", ErrBadParam, flag)
	}
	for _, char := range flag[1:] {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '-', char == '_', char == '.':
		default:
			return fmt.Errorf("%w: %q carries %q", ErrBadParam, flag, string(char))
		}
	}
	return nil
}

// ParamValues are the answers a run was given, by parameter id.
type ParamValues map[string]string

// ParamIssues is what is wrong with a set of answers: the required values that
// are missing, the numbers that are not numbers, the choices off the list. It
// is the form's own validation, so a Run button can refuse before the tool
// does.
func ParamIssues(params []ParamDefinition, values ParamValues) []string {
	var issues []string
	for _, param := range params {
		value := strings.TrimSpace(values[param.ID])
		switch {
		case value == "" && param.Required:
			issues = append(issues, param.ID)
		case value == "":
			continue
		case param.Kind == ParamNumber:
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				issues = append(issues, param.ID)
			}
		case param.Kind == ParamSelect:
			found := false
			for _, option := range param.Options {
				if option == value {
					found = true
					break
				}
			}
			if !found {
				issues = append(issues, param.ID)
			}
		}
	}
	return issues
}

// ParamDefaults seeds a form: each parameter's own default, or the first
// choice for a select with none (a select cannot be blank and valid).
func ParamDefaults(params []ParamDefinition) ParamValues {
	values := ParamValues{}
	for _, param := range params {
		if param.Default != "" {
			values[param.ID] = param.Default
			continue
		}
		if param.Kind == ParamSelect && len(param.Options) > 0 {
			values[param.ID] = param.Options[0]
		}
	}
	return values
}

// ParamRowIssue is the one thing a row of the declared-input editor still
// needs: the backend's own reason, named so the view can print it under the
// row that caused it.
type ParamRowIssue string

const (
	// ParamIssueID is a non-empty id that is not a legal key.
	ParamIssueID ParamRowIssue = "id"
	// ParamIssueDuplicate is an id another declared row already uses.
	ParamIssueDuplicate ParamRowIssue = "duplicate"
	// ParamIssueOptions is a select that declares no choices.
	ParamIssueOptions ParamRowIssue = "options"
	// ParamIssueFlag is a boolean with no flag — its value *is* its flag.
	ParamIssueFlag ParamRowIssue = "flag"
	// ParamIssueBadFlag is a flag that is not one argv token.
	ParamIssueBadFlag ParamRowIssue = "badflag"
)

// ParamRowIssues is what a parameter row still needs, by its position in the
// editor: the same rules the backend enforces on save, answered before the
// save is asked for — the old build's `paramIssues`, whose errors rendered
// under the row that caused them so a save never had to bounce back from the
// server to explain a typo.
//
// A row with no id is not a declaration (the editor drops it from the
// request), so it earns no issue of its own; a *declared* row that cannot be
// saved does. One row carries at most one reason, the first one the backend
// would report.
func ParamRowIssues(params []ParamDefinition) map[int]ParamRowIssue {
	issues := map[int]ParamRowIssue{}
	seen := map[string]int{}
	for i, param := range params {
		id := strings.TrimSpace(param.ID)
		if id == "" {
			continue
		}
		if !isParamID(id) {
			issues[i] = ParamIssueID
			continue
		}
		if _, dup := seen[id]; dup {
			issues[i] = ParamIssueDuplicate
			continue
		}
		seen[id] = i
		flag := strings.TrimSpace(param.Flag)
		switch {
		case param.Kind == ParamSelect && len(param.Options) == 0:
			issues[i] = ParamIssueOptions
		case param.Kind == ParamBoolean && flag == "":
			issues[i] = ParamIssueFlag
		case flag != "" && validateParamFlag(flag) != nil:
			issues[i] = ParamIssueBadFlag
		}
	}
	return issues
}
