package shell

import (
	"strings"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// The `args` hint is accepted without a reader (`linkparams.go:22-26`), but
// "accepted" is not "anything goes": the reference splits the value on ASCII
// spaces and refuses the whole link when a token is not shell-inert
// (`validate_arguments`, `deep_link.rs:402-438`). Its refusal matrix is a unit
// test there (`:1498-1513`); this is the same matrix, plus the two bounds the
// reference only states in prose.
func TestTheArgumentsHintHasTheReferencesTokenRules(t *testing.T) {
	long := strings.Repeat("a", 65)
	longest := strings.Repeat("a", 64)
	// 16 tokens of one character fit; the seventeenth does not.
	sixteen := strings.TrimSpace(strings.Repeat("a ", 16))
	seventeen := strings.TrimSpace(strings.Repeat("a ", 17))
	// The total-length bound is on the whole value: five 51-character tokens are
	// 259 characters (five short of every other bound), so only the total catches
	// this one.
	wide := strings.TrimSpace(strings.Repeat(strings.Repeat("a", 51)+" ", 5))
	for _, tc := range []struct {
		value string
		want  bool
		why   string
	}{
		// Accepted: an optional key, and a token of shell-inert punctuation.
		{"", true, "an absent hint is not a refusal (Ok(None))"},
		{"   ", true, "whitespace is the same as absent"},
		{"src", true, "the plainest token"},
		{"src main", true, "two tokens"},
		{"a  b", true, "a run of spaces is skipped, not an empty token"},
		{"a=b,c:d@e%f+g.h_i", true, "every inert punctuation character"},
		{longest, true, "exactly the per-token bound"},
		{sixteen, true, "exactly the token bound"},
		{strings.Repeat("日", 60), false, "the character class is ASCII-only, however few runes"},

		// Refused: the reference's own six metacharacter cases (:1498-1513).
		{"src;rm", false, "a semicolon ends the token for a shell"},
		{"|cat", false, "a pipe"},
		{"$HOME", false, "a variable expansion"},
		{"*", false, "a glob"},
		{"~/etc", false, "a tilde"},
		{`"quoted"`, false, "a quote"},
		{"a&b", false, "a background job"},
		{"a`id`", false, "a command substitution"},
		{"a<b", false, "a redirect"},
		{"a(b)", false, "a subshell"},
		{"a{b}", false, "a brace expansion"},
		{"a[b]", false, "a bracket glob"},
		{"a?b", false, "a single-character glob"},
		{"a!b", false, "a history expansion"},
		{"a\\b", false, "an escape"},
		{"src/main.go", false, "a slash is a path, not an argument"},
		{"a\tb", false, "a tab is whitespace, and whitespace is not inert"},
		{"a\nb", false, "a newline is a control character"},
		{"-hidden", false, "a leading dash"},
		{"--version=1.2.3", false, "a leading dash, even when the rest is inert"},
		{long, false, "one character over the per-token bound"},
		{seventeen, false, "one token over the count bound"},
		{wide, false, "over the total bound"},
	} {
		if got := linkArgsFit(tc.value); got != tc.want {
			t.Errorf("linkArgsFit(%q) = %v, want %v (%s)", tc.value, got, tc.want, tc.why)
		}
	}
}

// The judgement is on the live path, not only in the pure function: a register
// link whose hint is not inert is refused the way every other bad link is —
// one sentence, no trip to the integrations page (`refuseLink`). The reference's
// `InvalidArguments` is not one of its silent refusals either (`:225-228`).
func TestTheRegisterLinkRefusesAnArgumentsHintThatIsNotInert(t *testing.T) {
	want := i18n.For("en").Settings.DeepLinkRejected
	for _, hint := range []string{
		"src%3Brm",
		"%7Ccat",
		"%24HOME",
		"*",
		"~%2Fetc",
		"%22quoted%22",
		"--hidden",
		strings.Repeat("a", 65),
	} {
		a := newApp(t)
		surface := a.Surf
		a.HandleURL("floter://register?cmd=rg&args=" + hint)
		if a.Surf != surface {
			t.Errorf("args=%s switched surface to %v", hint, a.Surf)
		}
		if got := pendingNoticeText(a); got != want {
			t.Errorf("args=%s reads %q, want the refusal %q", hint, got, want)
		}
	}
}

// The other side of the line: an inert hint changes nothing at all. The link
// still reaches the name probe and still answers with the miss — `args` is
// accepted and dropped, so it cannot turn a link that routes into a refusal or
// a link that fails into a success.
func TestAnInertArgumentsHintChangesNothing(t *testing.T) {
	const name = "floter-no-such-tool-p10i"
	for _, hint := range []string{"src", "src%20main", "a=b,c:d@e%25f+g.h_i", ""} {
		a := newApp(t)
		link := "floter://register?cmd=" + name
		if hint != "" {
			link += "&args=" + hint
		}
		a.HandleURL(link)
		if a.Surf != SurfaceSettings || a.Settings.Page != settingsui.PageIntegrations {
			t.Errorf("%s landed on surface %v page %v", link, a.Surf, a.Settings.Page)
			continue
		}
		if got, want := pendingNoticeText(a), i18n.For("en").Settings.RegisterNotFound(name); got != want {
			t.Errorf("%s reads %q, want the miss %q", link, got, want)
		}
	}
}
