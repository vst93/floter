package shell

import (
	"net/url"
	"testing"
)

// The prefill is this build's own contract, and it names exactly two spellings
// (`docs/mygo-rewrite-plan.md:226`): `floter://search?q=…` and `floter://?q=…`.
// Everything else answers "" — the value `q` first, `query` second, an empty
// value treated as absent (`launcherPrefill`, linkhost.go).
func TestLauncherPrefillOnlyAnswersForItsOwnSpellings(t *testing.T) {
	cases := []struct {
		link string
		want string
		why  string
	}{
		{"floter://search?q=json+viewer", "json viewer", "the documented prefill host"},
		{"floter://search?query=notes", "notes", "the alias the router has always read"},
		{"floter://search?q=a&query=b", "a", "q wins when both are there"},
		{"floter://search?q=&query=b", "b", "an empty value is absent, not a request to clear"},
		{"floter://search", "", "nothing asked for"},
		{"floter://?q=notes", "notes", "the bare launcher"},
		{"floter:///?q=notes", "notes", "the same, with the slash a shell adds"},
		{"FLOTER://SEARCH?q=x", "x", "the host folds case like the action does"},
		{"FLOTER://?q=x", "x", "the bare launcher folds case too"},
		{"floter://bogus?q=hello", "", "an action this router does not know carries nothing"},
		{"floter://launcher?q=hello", "", "there is no `launcher` host; the bare form is the empty one"},
		{"floter:///search?q=hello", "", "a path is not the host spelling"},
		{"floter:connect?q=hello", "", "no authority at all is the reference's MalformedUrl"},
	}
	for _, tc := range cases {
		parsed, err := url.Parse(tc.link)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", tc.link, err)
		}
		if got := launcherPrefill(parsed); got != tc.want {
			t.Errorf("launcherPrefill(%q) = %q, want %q (%s)", tc.link, got, tc.want, tc.why)
		}
	}
	if got := launcherPrefill(nil); got != "" {
		t.Errorf("launcherPrefill(nil) = %q, want \"\"", got)
	}
}

// The same rule at the router: a link the switch falls through opens the
// launcher and leaves the search box alone unless it is one of the two launcher
// spellings. Before P10-m the fallback prefilled for **any** host, so a typo'd
// or stranger's link (`floter://bogus?q=hello`) typed into the user's box —
// the opposite of what the build documents (`mygo-rewrite-plan.md:226`).
func TestTheRouterPrefillsOnlyTheLauncherSpellings(t *testing.T) {
	cases := []struct {
		link string
		want string
		why  string
	}{
		{"floter://search?q=json+viewer", "json viewer", "the documented prefill host"},
		{"floter://?q=notes", "notes", "the bare launcher"},
		{"FLOTER://SEARCH?q=x", "x", "case does not decide the host"},
		{"floter://bogus?q=hello", "keep", "an unknown action opens the launcher and touches nothing"},
		{"floter:connect?q=hello", "keep", "a link with no authority is not the bare launcher"},
		{"floter:///search?q=hello", "keep", "a path is not the host spelling"},
		{"floter://search?q=", "keep", "an empty value asks for nothing"},
	}
	for _, tc := range cases {
		a := newApp(t)
		a.Launcher.SetQuery("keep")
		a.HandleURL(tc.link)
		if a.Surf != SurfaceLauncher {
			t.Errorf("%q moved to surface %v, want the launcher (%s)", tc.link, a.Surf, tc.why)
		}
		if a.Launcher.Query != tc.want {
			t.Errorf("%q left the query at %q, want %q (%s)", tc.link, a.Launcher.Query, tc.want, tc.why)
		}
	}
}
