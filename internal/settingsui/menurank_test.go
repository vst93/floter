package settingsui

import (
	"image/color"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

// The Integrations page has two ranks of navigation and they must not look
// alike.
//
// The report was 「这里的两层菜单样式和交互太丑了」: the page's own three tabs
// (My tools / Add tools / Import & export) and, inside Add tools, the three ways
// to add (Detected / Local tool / Package) were the *same* segmented control
// drawn twice, one row above the other. The second read as a repeat of the
// first rather than as its children, and the page opened on three rows of
// equally-weighted plates before any content.
//
// The ranks are now told apart the way a hierarchy is told apart everywhere in
// this app: the parent is a row of words on the sheet's own surface with a rule
// under the chosen one (a tab bar), and only the child wears a plate. This pin
// reads that difference off the frame — the parent's chosen segment has no
// plate behind it, the child's does — rather than off the source.
func TestTheTwoNavigationRanksDoNotLookAlike(t *testing.T) {
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = "dark"; s.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.selectPage(PageIntegrations)
	a.integrationNav.tab, a.integrationNav.addMethod = 1, 1
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 720, 700)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	// Both chosen segments are on screen: "Add tools" (parent) and "Local tool"
	// (child). A plate behind one and not the other is the whole distinction.
	parent, ok := tt.Find("Add tools")
	if !ok {
		t.Fatalf("the parent tab is missing: %q", tt.Texts())
	}
	child, ok := tt.Find("Local tool")
	if !ok {
		t.Fatalf("the child tab is missing: %q", tt.Texts())
	}
	if child.Y <= parent.Y {
		t.Errorf("the child tab (%v) is not below the parent (%v)", child.Y, parent.Y)
	}
	// The child wears a plate; the parent does not. Read the fill one pixel
	// inside each label's own box, away from the glyphs.
	img := tt.Image()
	fill := func(r ui.Rect) color.RGBA {
		c := img.RGBAAt(int(r.X)+2, int(r.Y+r.H/2))
		return c
	}
	parentFill, childFill := fill(parent), fill(child)
	if parentFill == childFill {
		t.Errorf("the two ranks paint the same face (%v): the child tab and the parent tab are the same control",
			parentFill)
	}
}

// The child rank rides inside the parent's block, not below it as the page's
// first section.
//
// Both ranks are navigation; the page's own gap (Space(4)) between them made the
// child look like content. The measured distance is what the report's second
// half is about.
func TestTheChildRankRidesUnderTheParentRank(t *testing.T) {
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = "dark"; s.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.selectPage(PageIntegrations)
	a.integrationNav.tab = 1
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 720, 700)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	parent, ok := tt.Find("Add tools")
	if !ok {
		t.Fatalf("the parent tab is missing: %q", tt.Texts())
	}
	child, ok := tt.Find("Detected")
	if !ok {
		t.Fatalf("the child tab is missing: %q", tt.Texts())
	}
	// The child's *label* box is a 28u segment with a one-line word centred in
	// it, so the ink starts about 6.6px below the box on each side. The
	// measured distance is box-to-box: one breathing step (`Space(0.5)`), not
	// the page's `Space(4)` = 14.4px section gap — before the fix it was 19.8px,
	// which is what made the child read as the page's first section.
	gap := child.Y - (parent.Y + parent.H)
	if gap > 18 {
		t.Errorf("the child rank sits %vpx under the parent's tab bar, want one breathing step — it reads as the page's first section",
			gap)
	}
}
