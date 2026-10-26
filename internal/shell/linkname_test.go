package shell

import (
	"strings"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// The rule is the reference's `validate_command` (main:src-tauri/src/deep_link.rs:364-390),
// and its own test refuses exactly these spellings (`deep_link.rs:1465-1487`).
// The two lists are one answer: a register link names a bare command, and
// everything that is a path, a traversal or a shell string is not a name.
func TestBareCommandNameRefusesWhatIsNotOne(t *testing.T) {
	refused := []string{
		"",
		"   ",
		"/usr/bin/rg",
		`C:\Windows\notepad.exe`,
		`..\rg`,
		"../rg",
		"..",
		"~/.local/bin/rg",
		"-rg",
		"rg;rm -rf /",
		"rg --exec=ls",
		"rg\nx",
		"rg$(x)",
		"rg`x`",
		"rg|cat",
		"rg*",
		`rg"quoted"`,
		"rg'",
		strings.Repeat("a", maxCommandNameChars+1),
	}
	for _, raw := range refused {
		if got, ok := bareCommandName(raw); ok {
			t.Errorf("bareCommandName(%q) = %q, true; a register link names a bare command, not this", raw, got)
		}
	}

	accepted := []string{
		"rg",
		"my-tool",
		"fd-find",
		"python3.13",
		"notes.txt",
		// The rule is structural, not a lookup: a dotted name passes it and
		// then resolves to nothing (the not-found path). On Windows the name
		// probe's own judgement is what refuses a file that cannot run
		// (`progtype.For`, P10-f), not this rule.
		strings.Repeat("a", maxCommandNameChars),
	}
	for _, raw := range accepted {
		got, ok := bareCommandName(raw)
		if !ok {
			t.Errorf("bareCommandName(%q) refused a bare name", raw)
			continue
		}
		if got != raw {
			t.Errorf("bareCommandName(%q) = %q, want the value itself", raw, got)
		}
	}

	// The value is trimmed, because the reference trims before it judges: a
	// link whose `cmd` carries padding names the same program.
	if got, ok := bareCommandName("  rg  "); !ok || got != "rg" {
		t.Errorf("bareCommandName(%q) = %q, %v; want rg, true", "  rg  ", got, ok)
	}
}

// A register link that carries a path is refused by the router, before the page
// opens, with the router's own sentence — the reference's `CommandNotBasename`
// is not one of the silent refusals (`deep_link.rs:225-228`), and it needs no
// new copy: `settings.deepLinkRejected` is the sentence every refused link
// already gets.
//
// Both spellings are here because both reach the same router: the URL form, and
// the terminal line `floter register /usr/bin/rg` that `LinkFromArguments`
// normalizes into it (`linkargs.go:88-92`).
func TestARegisterLinkRefusesAPath(t *testing.T) {
	want := i18n.For("en").Settings.DeepLinkRejected
	for _, link := range []string{
		"floter://register?cmd=%2Fusr%2Fbin%2Frg",
		"floter://register?cmd=C%3A%5CWindows%5Cnotepad.exe",
		"floter://register?cmd=%20%2Fusr%2Fbin%2Frg",
		"floter://register?tool=..%2Frg",
		"floter://register?program=-rg",
		// The positional spelling rides `parsed.Path`; one leading slash is
		// stripped, so a second one still spells an absolute path.
		"floter://register//usr/bin/rg",
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

	// The terminal spelling is the same link, so it is the same refusal.
	link, ok := LinkFromArguments([]string{"register", "/usr/bin/rg"})
	if !ok {
		t.Fatal("register with a path produced no link: the router would then never see it")
	}
	a := newApp(t)
	surface := a.Surf
	a.HandleURL(link)
	if a.Surf != surface {
		t.Errorf("the register command line switched surface to %v", a.Surf)
	}
	if got := pendingNoticeText(a); got != want {
		t.Errorf("the register command line reads %q, want %q", got, want)
	}
}

// The other side of the same line: a bare name is not refused. It opens the
// integrations page and lets the name probe answer — a name nothing resolves is
// the "not found" sentence, a different answer from a refusal.
func TestARegisterLinkTakesABareName(t *testing.T) {
	a := newApp(t)
	const name = "floter-no-such-tool-p10g"
	a.HandleURL("floter://register?cmd=" + name)

	if a.Surf != SurfaceSettings {
		t.Fatalf("a bare name opened surface %v, want the settings page", a.Surf)
	}
	if a.Settings.Page != settingsui.PageIntegrations {
		t.Errorf("a bare name landed on page %v, want the integrations page", a.Settings.Page)
	}
	if got, want := pendingNoticeText(a), i18n.For("en").Settings.RegisterNotFound(name); got != want {
		t.Errorf("the miss reads %q, want %q", got, want)
	}
}
