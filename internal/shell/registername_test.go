package shell

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// stubNotFinding replaces the register link's name probe with one that records
// every name it is asked for and resolves none. The table below then sees which
// spelling reached `registerFromLink` without depending on the machine's search
// path (the seam is `stubLookTool`, register_test.go). It returns a getter
// because the slice is filled after the stub is installed.
func stubNotFinding(t *testing.T) func() []string {
	t.Helper()
	var asked []string
	stubLookTool(t, func(names ...string) (string, bool) {
		asked = append(asked, names...)
		return "", false
	})
	return func() []string { return asked }
}

// The register link's four name spellings, and which of them the reference
// shares. `cmd` is the reference's only spelling (`deep_link.rs:472`; its CLI
// normalizer appends the same key, `:588`). `tool`, `program` and a positional
// path are this build's aliases: the reference refuses them with
// `UnknownParameter` / `UnknownParameter` / `MissingCommand`. An alias is a
// spelling, not a second feature — the link reaches the same place either way —
// so one table pins both halves: the name the probe is asked for, and the page
// that opens.
func TestTheRegisterLinkTakesTheNameTheReferenceTakes(t *testing.T) {
	cases := []struct {
		link string
		name string
		why  string
	}{
		{"floter://register?cmd=p10k-cmd", "p10k-cmd", "the reference's only spelling"},
		{"floter://register?tool=p10k-tool", "p10k-tool", "this build's alias (reference: UnknownParameter)"},
		{"floter://register?program=p10k-program", "p10k-program", "this build's alias (reference: UnknownParameter)"},
		{"floter://register/p10k-pos", "p10k-pos", "this build's positional alias (reference: MissingCommand)"},
	}
	for _, tc := range cases {
		t.Run(tc.link, func(t *testing.T) {
			a := newApp(t)
			asked := stubNotFinding(t)
			a.HandleURL(tc.link)

			if a.Surf != SurfaceSettings {
				t.Fatalf("%s opened surface %v, want the settings page (%s)", tc.link, a.Surf, tc.why)
			}
			if a.Settings.Page != settingsui.PageIntegrations {
				t.Errorf("%s landed on page %v, want the integrations page (%s)", tc.link, a.Settings.Page, tc.why)
			}
			if got := asked(); len(got) == 0 || got[0] != tc.name {
				t.Errorf("%s asked the probe for %v, want %q (%s)", tc.link, got, tc.name, tc.why)
			}
			if a.registerTarget != "" {
				t.Errorf("%s marked %q, want no target for a name nothing resolves", tc.link, a.registerTarget)
			}
			if got, want := pendingNoticeText(a), i18n.For("en").Settings.RegisterNotFound(tc.name); got != want {
				t.Errorf("%s reads %q, want %q (%s)", tc.link, got, want, tc.why)
			}
		})
	}
}

// A query name outranks the positional one: `firstNonEmpty` reads `cmd`, `tool`,
// `program` and only then the path (system.go:232-234), so a link that carries
// both names the query's. The reference agrees by accident — it never reads the
// path at all — and this pin keeps the agreement if someone later reorders the
// arguments.
func TestThePositionalSpellingNeverOverridesAQueryName(t *testing.T) {
	for _, spelling := range []string{"cmd", "tool", "program"} {
		link := "floter://register/p10k-pos?" + spelling + "=p10k-query"
		a := newApp(t)
		asked := stubNotFinding(t)
		a.HandleURL(link)

		if got := asked(); len(got) == 0 || got[0] != "p10k-query" {
			t.Errorf("%s asked the probe for %v, want p10k-query (the query name outranks the path)", link, got)
		}
		if a.Surf != SurfaceSettings || a.Settings.Page != settingsui.PageIntegrations {
			t.Errorf("%s did not reach the integrations page (surface %v, page %v)", link, a.Surf, a.Settings.Page)
		}
	}
}
