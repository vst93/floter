package shell

import (
	"net/url"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// query parses a raw query string the way the router does, so a case reads like
// the link it stands for.
func query(t *testing.T, raw string) url.Values {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return values
}

// The table is the rule: a routed action accepts only the keys its row names,
// each value-bearing slot at most once (`deep_link.rs:467-492`). The hosts this
// build adds are not reference actions, so the table has no opinion about their
// query — that is the documented divergence, not an oversight
// (`docs/mygo-rewrite-plan.md:226`).
func TestLinkQueryFits(t *testing.T) {
	cases := []struct {
		action string
		query  string
		want   bool
		why    string
	}{
		// connect: one key, one slot.
		{"connect", "manifest=/opt/tool.json", true, "the only key connect names"},
		{"connect", "manifest=%2Fhome%2Fu%2Ftool.json", true, "escaped is the same key"},
		{"connect", "", true, "an empty query is missing-manifest, a different refusal"},
		{"connect", "manifest=/a.json&cmd=rg", false, "cmd is a register key, not a connect key"},
		{"connect", "manifest=/a.json&q=x", false, "the launcher's key is not connect's"},
		{"connect", "manifest=/a.json&manifest=/b.json", false, "one manifest, one value"},

		// register: three name spellings share one slot; args and confirm are
		// accepted without a reader.
		{"register", "cmd=rg", true, "the documented spelling"},
		{"register", "tool=rg", true, "the old build's second spelling"},
		{"register", "program=rg", true, "the old build's third spelling"},
		{"register", "cmd=rg&args=src", true, "args is in the table (README:97-104)"},
		{"register", "cmd=rg&confirm=1", true, "confirm is in the table (deep_link.rs:487)"},
		{"register", "confirm=1&confirm=1", true, "a flag carries no value, so a repeat is inert"},
		{"register", "cmd=rg&manifest=/a/tool.json", false, "the reference's own refusal case (:1515)"},
		{"register", "cmd=rg&q=x", false, "the launcher's key is not register's"},
		{"register", "cmd=rg&cmd=fd", false, "the name slot filled twice"},
		{"register", "cmd=rg&tool=fd", false, "two spellings of one slot"},
		{"register", "tool=rg&program=fd", false, "two spellings of one slot"},
		{"register", "cmd=rg&args=a&args=b", false, "the args slot filled twice"},
		{"register", "CMD=rg", false, "keys are exact; the reference's are lowercase"},

		// Not reference actions: no row, no opinion.
		{"settings", "anything=1", true, "this build's own host"},
		{"terminal", "foo=1", true, "this build's own host"},
		{"search", "q=x", true, "this build's documented prefill"},
		{"", "q=x", true, "floter://?q=x, the bare launcher"},
		{"open", "q=x", true, "open is ours; the reference's open takes nothing"},
	}
	for _, tc := range cases {
		if got := linkQueryFits(tc.action, query(t, tc.query)); got != tc.want {
			t.Errorf("linkQueryFits(%q, %q) = %v, want %v (%s)", tc.action, tc.query, got, tc.want, tc.why)
		}
	}
}

// The command line cannot produce a link the table refuses: the normalizer
// spells one key per action (`linkargs.go`), and the rest of a `register` line
// is dropped rather than folded into an `args` the router would then have to
// judge. One pin for the whole normalizer, because a second spelling of the
// query is exactly the drift the table exists to prevent.
func TestTheCommandLineNeverSpellsAnUnknownParameter(t *testing.T) {
	lines := [][]string{
		{"connect", "/opt/tool.json"},
		{"connect"},
		{"register", "rg"},
		{"register"},
		{"register", "rg", "--hidden", "src"},
		{"--yes", "register", "rg"},
		{"open"},
	}
	for _, line := range lines {
		link, ok := LinkFromArguments(line)
		if !ok {
			t.Errorf("%v produced no link", line)
			continue
		}
		parsed, err := url.Parse(link)
		if err != nil {
			t.Errorf("%v produced %q, which does not parse: %v", line, link, err)
			continue
		}
		if !linkQueryFits(parsed.Host, parsed.Query()) {
			t.Errorf("%v produced %q, which the router refuses", line, link)
		}
	}
}

// A register link that carries a key its row does not name is refused before
// anything opens or connects, with the router's own sentence: the reference's
// `UnknownParameter` is not one of the silent refusals (`deep_link.rs:225-228`),
// and it needs no new copy.
func TestARegisterLinkRefusesAParameterItDoesNotName(t *testing.T) {
	want := i18n.For("en").Settings.DeepLinkRejected
	for _, link := range []string{
		// The reference's own case (`deep_link.rs:1515-1528`).
		"floter://register?cmd=rg&manifest=/a/tool.json",
		"floter://register?tool=rg&manifest=/a/tool.json",
		"floter://register?cmd=rg&hidden=1",
		"floter://register?cmd=rg&q=x",
		"floter://register?CMD=rg",
	} {
		a := newApp(t)
		surface, page := a.Surf, a.Settings.Page
		a.HandleURL(link)
		if a.Surf != surface {
			t.Errorf("%s switched surface to %v", link, a.Surf)
		}
		if a.Settings.Page != page {
			t.Errorf("%s landed on page %v, want the page it started on", link, a.Settings.Page)
		}
		if a.registerTarget != "" {
			t.Errorf("%s marked %q", link, a.registerTarget)
		}
		if got := pendingNoticeText(a); got != want {
			t.Errorf("%s reads %q, want %q", link, got, want)
		}
	}
}

// The same rule on the other routed action, and its accepted side: a connect
// link that carries only its manifest still opens the integrations page, so the
// table is a filter on the shape of the query, not a wall in front of connect.
func TestAConnectLinkRefusesAParameterItDoesNotName(t *testing.T) {
	want := i18n.For("en").Settings.DeepLinkRejected
	for _, link := range []string{
		"floter://connect?manifest=/a/tool.json&cmd=rg",
		"floter://connect?manifest=/a/tool.json&q=x",
		"floter://connect?cmd=rg",
	} {
		a := newApp(t)
		surface, page := a.Surf, a.Settings.Page
		a.HandleURL(link)
		if a.Surf != surface {
			t.Errorf("%s switched surface to %v", link, a.Surf)
		}
		if a.Settings.Page != page {
			t.Errorf("%s landed on page %v, want the page it started on", link, a.Settings.Page)
		}
		if got := pendingNoticeText(a); got != want {
			t.Errorf("%s reads %q, want %q", link, got, want)
		}
	}

	// The accepted shape: the page opens synchronously, before the manifest is
	// even read — a refusal never switches a surface.
	a := newApp(t)
	a.HandleURL("floter://connect?manifest=" + t.TempDir() + "/tool.json")
	if a.Surf != SurfaceSettings || a.Settings.Page != settingsui.PageIntegrations {
		t.Errorf("an accepted connect landed on surface %v page %v", a.Surf, a.Settings.Page)
	}
}

// Two keys in one slot are the same value spelled twice, and the reference
// refuses the second occurrence exactly like an unknown key (`deep_link.rs:472`
// `command.is_none()`). The other side of the line: `args` and `confirm` are in
// the table, so a link carrying them still reaches the name probe — they are
// accepted without a reader, not refused.
func TestALinkTakesOneValuePerSlot(t *testing.T) {
	want := i18n.For("en").Settings.DeepLinkRejected
	for _, link := range []string{
		"floter://register?cmd=rg&cmd=fd",
		"floter://register?cmd=rg&tool=fd",
		"floter://register?tool=rg&program=fd",
		"floter://register?cmd=rg&args=a&args=b",
		"floter://connect?manifest=/a.json&manifest=/b.json",
	} {
		a := newApp(t)
		surface := a.Surf
		a.HandleURL(link)
		if a.Surf != surface {
			t.Errorf("%s switched surface to %v", link, a.Surf)
		}
		if got := pendingNoticeText(a); got != want {
			t.Errorf("%s reads %q, want %q", link, got, want)
		}
	}

	// The table's readerless keys: the link routes, the name is what answers.
	// P10-i 重定基线：`args=--hidden` → `args=src`。前导 `-` 是值判据的事
	// （`linkArgsFit`，linkargshint.go），不是表的事；这里钉的是「表内的键
	// 不被拒」，所以值取一个合法 token。
	const name = "floter-no-such-tool-p10h"
	for _, link := range []string{
		"floter://register?cmd=" + name + "&args=src",
		"floter://register?cmd=" + name + "&confirm=1",
		"floter://register?cmd=" + name + "&confirm=1&confirm=1",
	} {
		a := newApp(t)
		a.HandleURL(link)
		if a.Surf != SurfaceSettings || a.Settings.Page != settingsui.PageIntegrations {
			t.Errorf("%s landed on surface %v page %v", link, a.Surf, a.Settings.Page)
		}
		if got, want := pendingNoticeText(a), i18n.For("en").Settings.RegisterNotFound(name); got != want {
			t.Errorf("%s reads %q, want the miss %q", link, got, want)
		}
	}
}
