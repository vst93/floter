package shell

import (
	"net/url"
	"testing"

	"floter/internal/settingsui"
)

// The terminal spelling of `register` is the one the extension guide documents
// next to the URL (docs/extensions/README.zh-CN.md): `floter register rg` is
// `floter://register?cmd=rg`. Flags ride the line without changing it — the old
// build's canonical_argument skipped them too (deep_link.rs:576) — and words
// after the name are not part of the name.
func TestTheTerminalSpellingOfRegisterIsItsLink(t *testing.T) {
	for _, line := range [][]string{
		{"register", "rg"},
		{"--yes", "register", "rg"},
		{"register", "--yes", "rg"},
		{"register", "rg", "--hidden", "src"},
		{"REGISTER", "rg"},
	} {
		link, ok := LinkFromArguments(line)
		if !ok {
			t.Errorf("%q is not a link at all", line)
			continue
		}
		if want := "floter://register?cmd=rg"; link != want {
			t.Errorf("%q = %q, want %q", line, link, want)
		}
	}
}

// A connect line carries its manifest, escaped: a path has slashes and can have
// spaces, and an unescaped value would split the query the router reads.
func TestTheTerminalSpellingOfConnectCarriesTheManifest(t *testing.T) {
	for _, value := range []string{"/tmp/a b/manifest.json", "https://example.com/m.json?v=1", `C:\tools\m.json`} {
		link, ok := LinkFromArguments([]string{"connect", value})
		if !ok {
			t.Fatalf("connect %q is not a link", value)
		}
		parsed, err := url.Parse(link)
		if err != nil {
			t.Fatalf("connect %q produced an unparseable link %q: %v", value, link, err)
		}
		if parsed.Host != "connect" {
			t.Errorf("connect %q landed on host %q", value, parsed.Host)
		}
		if got := parsed.Query().Get("manifest"); got != value {
			t.Errorf("connect %q carried manifest = %q, want the argument back", value, got)
		}
		if want := url.QueryEscape(value); link != "floter://connect?manifest="+want {
			t.Errorf("connect %q = %q, want the escaped form", value, link)
		}
	}
}

// `open` is the action with nothing to carry: it is the summon, and a word
// after it is not a query (the router's own `q` comes from a link's query, and
// inventing one here would prefill the search box on a launch that asked for
// nothing).
func TestTheTerminalSpellingOfOpenCarriesNothing(t *testing.T) {
	for _, line := range [][]string{{"open"}, {"open", "junk"}, {"--background", "open"}} {
		link, ok := LinkFromArguments(line)
		if !ok {
			t.Errorf("%q is not a link at all", line)
			continue
		}
		if want := "floter://open"; link != want {
			t.Errorf("%q = %q, want %q", line, link, want)
		}
	}
}

// A URL among the arguments belongs to the framework: mygo scans the launch's
// arguments (and a second instance's forwarded ones) for URLs of the schemes
// the app registered and fires OnOpenURL with them (deeplink.go:80,185).
// Normalizing the same argument here as well would route one launch twice — two
// refusal toasts, two connect attempts — so the URL form is explicitly not this
// function's.
func TestTheCommandLineLeavesURLsToTheFramework(t *testing.T) {
	for _, line := range [][]string{
		{"floter://register?cmd=rg"},
		{"FLOTER://register?cmd=rg"},
		{"--yes", "floter://connect?manifest=%2Ftmp%2Fm.json"},
		// A line that carries both a URL and words is still the framework's:
		// the URL is delivered, and the words are not a second request. (This
		// is the case that makes the check load-bearing: without it the words
		// alone would build a link and the launch would route twice.)
		{"register", "rg", "floter://open"},
	} {
		if link, ok := LinkFromArguments(line); ok {
			t.Errorf("%q was normalized to %q as well as delivered by the framework: one launch, two routes", line, link)
		}
	}
}

// A word the router does not know is not this app's business, and neither is a
// line that names no action: the old build dropped an unknown action with one
// log line and no UI noise (deep_link.rs:33-34), because it is noise from
// somewhere else rather than something the user asked for.
func TestUnknownWordsStayQuiet(t *testing.T) {
	for _, line := range [][]string{
		{},
		{""},
		{"-h"},
		{"--yes"},
		{"hello"},
		{"openx"},
		{"settings"},
		{"--background", "--yes"},
	} {
		if link, ok := LinkFromArguments(line); ok {
			t.Errorf("%q produced %q, want no link", line, link)
		}
	}
}

// A value-less action is still a link: the router is the one place that decides
// what an action may carry, and it refuses these with the same sentence a
// value-less URL gets (system.go:196-215). Building the link here keeps that
// decision in one place instead of a second, quieter refusal in the parser.
func TestAValueLessActionIsStillTheRouters(t *testing.T) {
	for _, action := range []string{"connect", "register"} {
		link, ok := LinkFromArguments([]string{action})
		if !ok {
			t.Errorf("%q produced no link: the refusal would then live in two places", action)
			continue
		}
		if want := "floter://" + action; link != want {
			t.Errorf("%q = %q, want %q", action, link, want)
		}
	}
}

// The two halves are one path: a terminal line goes through the normalizer and
// then the router that the floter:// links already use, and lands where the URL
// form lands. A name nothing can resolve is the safe way to drive it — the
// router says so in place (registerFromLink's "not found" notice) instead of
// connecting anything.
func TestTheRegisterCommandLineReachesTheIntegrationsPage(t *testing.T) {
	a := newApp(t)
	link, ok := LinkFromArguments([]string{"register", "floter-no-such-tool-p9j"})
	if !ok {
		t.Fatalf("register with a name produced no link")
	}
	a.HandleURL(link)
	if a.Surf != SurfaceSettings {
		t.Fatalf("the register command line opened surface %v, want the settings page", a.Surf)
	}
	if a.Settings.Page != settingsui.PageIntegrations {
		t.Errorf("the register command line landed on page %v, want the integrations page", a.Settings.Page)
	}
}
