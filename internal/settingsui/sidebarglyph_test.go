package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
)

// The reference's settings sidebar names one lucide mark per page
// (`main:src/App.tsx:2903-2908`):
//
//	[["general", SlidersHorizontal], ["sessions", SquareTerminal],
//	 ["shortcuts", Keyboard], ["integrations", Blocks], ["about", Info]]
//
// and draws each at `<Icon size={15} strokeWidth={2}/>` (`:2925`). The port
// had drifted onto a different, unrelated five — general→`settings`,
// sessions→`terminal`, shortcuts→`star`, integrations→`file`, about→`alert` —
// so four of the five marks were the wrong shape and the other three names
// (`star`/`file`/`alert`) did not even describe the page they led.
//
// `pageGlyph` resolves through the launcher's shared vocabulary, so the mark
// it returns is the parsed glyph itself; this drives all five pages, pins each
// to the reference's own name, and rejects a page that still leads with the
// drifted mark. Mutation check: restore any old name and that page fails the
// name assertion (and, for the two pairs that share shape families, the
// distinctness arm).
func TestTheSidebarPagesLeadWithTheReferenceMarks(t *testing.T) {
	want := map[int]string{
		PageGeneral:      "sliders-horizontal",
		PageSessions:     "square-terminal",
		PageShortcuts:    "keyboard",
		PageIntegrations: "blocks",
		PageAbout:        "info",
	}
	drifted := map[int]string{
		PageGeneral:      "settings",
		PageSessions:     "terminal",
		PageShortcuts:    "star",
		PageIntegrations: "file",
		PageAbout:        "alert",
	}

	seen := map[*ui.SVG]string{}
	for page, name := range want {
		glyph, ok := pageGlyph(page)
		if !ok || glyph == nil {
			t.Fatalf("settings page %d leads with no mark", page)
		}
		reference, ok := launcher.Glyph(name)
		if !ok {
			t.Fatalf("the launcher vocabulary has no %q mark for page %d", name, page)
		}
		if glyph != reference {
			t.Errorf("settings page %d leads with the wrong mark: want %q", page, name)
		}
		if stale, ok := launcher.Glyph(drifted[page]); ok && glyph == stale {
			t.Errorf("settings page %d still leads with the drifted mark %q", page, drifted[page])
		}
		if prev, dup := seen[glyph]; dup {
			t.Errorf("settings pages %q and %q share one mark; the reference draws five", prev, name)
		}
		seen[glyph] = name
	}
}

// The two marks the port was missing live in the launcher's shared vocabulary
// now, and each is its own shape — not an alias onto a neighbouring icon. A
// missing parse would come back as the zero SVG, which is neither nil nor its
// neighbours.
func TestTheLauncherGainsTheTwoReferenceMarks(t *testing.T) {
	for _, name := range []string{"sliders-horizontal", "blocks"} {
		glyph, ok := launcher.Glyph(name)
		if !ok || glyph == nil {
			t.Fatalf("the launcher vocabulary has no %q mark", name)
		}
		if other, ok := launcher.Glyph("settings"); ok && glyph == other {
			t.Errorf("%q aliases the settings gear", name)
		}
	}
}

// The leading mark is a *literal* 15 DIPs: the reference writes
// `<Icon size={15} strokeWidth={2} />` in the JSX (`main:src/App.tsx:2925`), so
// the five sidebar marks sit at 15px under every interface-size step — unlike
// every other icon in this app, which is sized in `--u`. A `Space(4)` here
// tracked the step and left the mark at 14.4 even at the shipped default.
func TestTheSidebarMarkIsALiteralFifteen(t *testing.T) {
	if sidebarGlyphSize != 15 {
		t.Fatalf("the sidebar's leading mark is %v DIPs, want the reference's literal 15", sidebarGlyphSize)
	}
	// It must not be reached through `Space`: the whole point is that the step
	// does not move it. Under the shipped step the two differ already.
	for _, scale := range []float32{0.9, 1.0, 1.25} {
		var stepped float32
		tt := ui.NewTester(func(c *ui.Context) { stepped = c.Theme().Space(4) }, 100, 100)
		tt.SetScale(scale)
		tt.Frame()
		if stepped == 0 {
			t.Fatal("the frame resolved no theme")
		}
		if stepped == sidebarGlyphSize {
			t.Errorf("at scale %v the step's Space(4) is %v, the same as the mark; the mark is not step-independent",
				scale, stepped)
		}
	}
}
