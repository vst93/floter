package settingsui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// metaFixture is one installed integration whose meta line carries all three
// slots the reference fills — identity, kind, runtime source — plus the state
// word, so the line has to lay out four nodes in a 187u column at 460u.
func metaFixture() Integration {
	return Integration{
		ID: "dev.floter.tool", Name: "V Tools", Publisher: "vst", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		RuntimeSource: "managed",
	}
}

// The `·` between two meta facts is part of the *fact it introduces*, not a flex
// child of its own.
//
// The reference writes it as the following span's `::before` (`main:src/styles/
// extensions.css:979-984`: `content: "·"`, `margin-right: 6px`, `opacity: .7`),
// so it can only ever appear at the start of the fact it belongs to. This build
// drew it as a separate child of the wrapping row, which is a different thing
// the moment the row wraps: measured at 460u — where the meta column is 187u
// and every fact takes a line of its own — the dot stayed on the line it still
// fitted on, so all three lines ended in an orphan `·` and began with a bare
// fact. A dot that closes a line introduces nothing.
func TestAMetaSeparatorTravelsWithItsFact(t *testing.T) {
	integration := metaFixture()
	en := i18n.For("en").Settings
	facts := []string{
		integration.identity(),
		integration.integrationKind(en),
		integration.runtimeSourceLabel(en),
	}
	for _, width := range []int{860, 460} {
		width := width
		tt := integrationPage(t, oneIntegrationApp(integration, Actions{}), width, 900)
		for i, fact := range facts {
			if fact == "" {
				t.Fatalf("%du: fixture slot %d is empty, the line is one node short", width, i)
			}
			want := "· " + fact
			if i == 0 {
				want = fact // the first fact takes no separator
			}
			if _, ok := tt.Find(want); !ok {
				t.Errorf("%du: %q is not one node — the dot is not inside the fact's own text "+
					"(drawn: %q)", width, want, tt.Texts())
			}
		}
		if _, ok := tt.Find("·"); ok {
			t.Errorf("%du: a bare `·` is drawn as a node of its own, so it can be left "+
				"behind at a line end", width)
		}
	}
}

// The state word gets the same separator, because it is the line's last fact —
// unless it is a *boxed* chip.
//
// `extensions.css:986-995` keeps the dot off `--recommended` / `--broken` with
// two `:not()` selectors: inside a chip's keyline the dot would sit in its
// padding instead of beside it. Every transparent state keeps one.
func TestTheMetaLineDotsTheStateWordUnlessItIsABoxedChip(t *testing.T) {
	en := i18n.For("en").Settings

	for _, tc := range []struct {
		name  string
		edit  func(*Integration)
		state string
		boxed bool
	}{
		{"running", func(i *Integration) {}, en.IntegrationsRunning, false},
		{"off", func(i *Integration) { i.Enabled, i.Running = false, false }, en.IntegrationsOff, false},
		{"broken", func(i *Integration) { i.Broken, i.RuntimeAvailable = true, false },
			en.IntegrationsBroken, true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			integration := metaFixture()
			tc.edit(&integration)
			tt := integrationPage(t, oneIntegrationApp(integration, Actions{}), 860, 900)
			if !tt.HasText(tc.state) {
				t.Fatalf("the state word %q is not drawn: %q", tc.state, tt.Texts())
			}
			if n := bareDots(tt); n != 0 {
				t.Errorf("%d bare `·` nodes are drawn; the separator has to ride inside "+
					"the node it introduces, or a wrap can leave it behind", n)
			}
			_, dotted := tt.Find("· " + tc.state)
			if tc.boxed && dotted {
				t.Errorf("the boxed chip %q carries a `·` inside its keyline", tc.state)
			}
			if !tc.boxed && !dotted {
				t.Errorf("the state word %q is not introduced by a `·`", tc.state)
			}
		})
	}

	// The same rule in the other language: the dot is punctuation this build
	// writes, not part of either word, so it does not move when the words do.
	zh := i18n.For("zh").Settings
	zhApp := oneIntegrationApp(metaFixture(), Actions{})
	zhApp.set(func(s *settings.Settings) { s.Language = "zh" })
	zhApp.selectPage(PageIntegrations)
	tt := render(t, zhApp, 860, 900)
	if _, ok := tt.Find("· " + zh.IntegrationsRunning); !ok {
		t.Errorf("the zh state word is not introduced by its own `·`: %q", tt.Texts())
	}
	if _, ok := tt.Find("· " + en.IntegrationsRunning); ok {
		t.Errorf("the en state word is still drawn beside the zh one: the switch half-applied")
	}
}

// The state word sits a half unit past the row's own 8u gap, the way the
// reference's `.extension-status { margin-left: 2px }` sets it off from the last
// fact — 10u in total. Without it the word reads as one more fact of the same
// list rather than as the line's verdict.
func TestTheStateWordSitsHalfAUnitPastTheRowGap(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	a := oneIntegrationApp(metaFixture(), Actions{})
	var gap, half float32
	tt := ui.NewTester(func(c *ui.Context) {
		gap, half = c.Theme().Space(2), c.Theme().Space(0.5)
		a.View(c)
	}, 860, 900)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("open Integrations: %v", err)
	}
	tt.Frame()
	last := mustFind(t, tt, "· Managed") // the third fact, the last before the state
	state := mustFind(t, tt, "· Running")
	if state.X <= last.X {
		t.Fatalf("the state word wrapped to its own line (x=%.1f vs %.1f); this pin is "+
			"measured on one line", state.X, last.X)
	}
	if got := state.X - (last.X + last.W); got != gap+half {
		t.Errorf("the state word starts %.1fu after the last fact, want %.1fu "+
			"(%.1fu row gap + the reference's 2px margin-left)", got, gap+half, gap)
	}
}

// bareDots counts the text nodes that are nothing but a separator: the shape
// this build used to draw, and the one a wrap can strand at a line end.
func bareDots(tt *ui.Tester) int {
	n := 0
	for _, text := range tt.Texts() {
		if strings.TrimSpace(text) == "·" {
			n++
		}
	}
	return n
}
