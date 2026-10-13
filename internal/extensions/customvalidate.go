package extensions

import (
	"strings"
	"unicode/utf8"
)

// The shape rules a connection request has to satisfy before any package is
// written, named so both sides of the wire can speak about the same rule: the
// backend refuses the request by name, and the settings form prints its own
// localized sentence under the row that broke it. They are the old host's
// `settings.extensions.form.*` checks (`create_custom_integration_locked`),
// plus the version rule whose sentence the old host built ad hoc.

// CustomField names the form row an issue belongs to: the backend never draws
// anything, but it names the field so the surface can put the sentence where
// the mistake is instead of in a banner.
type CustomField string

const (
	CustomFieldName      CustomField = "name"
	CustomFieldCommand   CustomField = "command"
	CustomFieldVersion   CustomField = "version"
	CustomFieldPlatforms CustomField = "platforms"
	CustomFieldMode      CustomField = "mode"
	CustomFieldScript    CustomField = "script"
	CustomFieldLanguage  CustomField = "scriptLanguage"
)

// CustomIssueKey names which rule a request broke. It is the stable half of
// the old host's localized form keys (`settings.extensions.form.nameLength`,
// …): the i18n copy owns the words, this owns the identity.
type CustomIssueKey string

const (
	// CustomIssueNameLength is a name that is blank or longer than 80
	// characters, counted in runes (the old host counted `chars`).
	CustomIssueNameLength CustomIssueKey = "nameLength"
	// CustomIssueCommandShape is a command that is not
	// `[a-z0-9][a-z0-9_-]{0,63}`: the word that summons the tool is a
	// lowercase slug, so it never collides with its own argv.
	CustomIssueCommandShape CustomIssueKey = "commandShape"
	// CustomIssueVersionShape is a version that is not a semantic version.
	CustomIssueVersionShape CustomIssueKey = "versionShape"
	// CustomIssuePlatformRequired is a request that supports no platform at
	// all: an empty list would mean "everywhere", which only the codec says,
	// never the user.
	CustomIssuePlatformRequired CustomIssueKey = "platformRequired"
	// CustomIssueModeInvalid is a mode that is neither executable nor script.
	CustomIssueModeInvalid CustomIssueKey = "modeInvalid"
	// CustomIssueScriptEmpty is a script integration whose script is blank.
	CustomIssueScriptEmpty CustomIssueKey = "scriptEmpty"
	// CustomIssueScriptLanguageInvalid is a script language on an executable
	// integration: the interpreter belongs to the script runtime alone.
	CustomIssueScriptLanguageInvalid CustomIssueKey = "scriptLanguageInvalid"
)

// CustomIssue is one rule a request broke, named by the field it belongs to.
type CustomIssue struct {
	Field CustomField
	Key   CustomIssueKey
}

// CustomIssues is what a connection request still needs, in the order the form
// lists its rows: name, command, version, platforms, mode, script, language.
// It is the form's own pre-check and the backend's own gate, one function so
// the two cannot drift.
//
// It reads the request the way the form sends it: an empty command or version
// is a default the host fills (the program's own name, `0.0.1`), not an issue,
// and a blank mode is the executable default. `ScriptContent` is checked only
// for a script, and `ScriptLanguage` is refused only on a non-script: a script
// with no language is a separate rule the interpreter probe answers.
func CustomIssues(request CustomRequest) []CustomIssue {
	var issues []CustomIssue

	name := strings.TrimSpace(request.Name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		issues = append(issues, CustomIssue{CustomFieldName, CustomIssueNameLength})
	}

	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	switch mode {
	case "", "executable", "script":
	default:
		issues = append(issues, CustomIssue{CustomFieldMode, CustomIssueModeInvalid})
	}

	command := strings.ToLower(strings.TrimSpace(request.Command))
	if command != "" && !validCustomCommand(command) {
		issues = append(issues, CustomIssue{CustomFieldCommand, CustomIssueCommandShape})
	}

	if version := strings.TrimSpace(request.Version); version != "" {
		if _, err := ParseSemver(version); err != nil {
			issues = append(issues, CustomIssue{CustomFieldVersion, CustomIssueVersionShape})
		}
	}

	if len(request.Platforms) == 0 {
		issues = append(issues, CustomIssue{CustomFieldPlatforms, CustomIssuePlatformRequired})
	}

	if mode == "script" && strings.TrimSpace(request.ScriptContent) == "" {
		issues = append(issues, CustomIssue{CustomFieldScript, CustomIssueScriptEmpty})
	}
	if mode != "script" && strings.TrimSpace(request.ScriptLanguage) != "" {
		issues = append(issues, CustomIssue{CustomFieldLanguage, CustomIssueScriptLanguageInvalid})
	}

	return issues
}

// validCustomCommand is the command grammar the old host pinned:
// `[a-z0-9][a-z0-9_-]{0,63}` — one lowercase letter or digit, then up to 63
// more of those plus hyphen and underscore. The caller has already lowercased
// and trimmed.
func validCustomCommand(command string) bool {
	if len(command) == 0 || len(command) > 64 {
		return false
	}
	for i := 0; i < len(command); i++ {
		letter := command[i]
		switch {
		case letter >= 'a' && letter <= 'z', letter >= '0' && letter <= '9':
		case (letter == '-' || letter == '_') && i > 0:
		default:
			return false
		}
	}
	return true
}
