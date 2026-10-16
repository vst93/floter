package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The row's trailing key slot is the reference's `.launcher-result__action`
// (`main:src/styles/launcher.css:1361-1366`): a *fixed* 30u-minimum box at the
// row's right edge, drawn in the body step at weight 620 in `--text-tertiary`
// (lifted to `--text-primary` on the selected row). The port had drawn the slot
// content-sized in the body-minus-one step at `--text-muted`, so a column of
// `⌘N` badges did not line up and read one register too quiet.
//
// The numbered family (`⌘1`–`⌘0`) and a command row's `Tab` affordance both go
// through `rowActionText`, so the two cannot drift apart.
func TestTheRowTrailingBadgeIsTheReferenceActionSlot(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Frame()

	// The slot's own 30u minimum, resolved at the same step the row draws it:
	// `MinWidth(Space(7.5))` is the reference's `min-width: calc(var(--u) * 30)`.
	slot := theme.ForPreferences(settings.Normalize(a.settings()), false, ui.Preferences{}).Theme.Space(7.5)

	selected := a.resultNumber(1)
	selRect, ok := tt.Find(selected)
	if !ok {
		t.Fatalf("the selected row's badge %q was not drawn: %q", selected, tt.Texts())
	}
	if selRect.W < slot {
		t.Errorf("the badge box is %.0f wide, want at least the action slot's %.0f (30u)", selRect.W, slot)
	}
	// Right-aligned at the row's inner right edge: the card is 720 wide with the
	// panel's own 4u inset and the row's 12u padding.
	if selRect.X+selRect.W > InputWindowWidth-14 {
		t.Errorf("the badge's right edge is %.0f, past the row's inner edge (%d)",
			selRect.X+selRect.W, InputWindowWidth-16)
	}

	// The selected row's badge wears the lifted register and an unselected
	// row's badge the tertiary one, so the two inks differ.
	unselRect, ok := tt.Find(a.resultNumber(2))
	if !ok {
		t.Fatalf("the second row's badge was not drawn: %q", tt.Texts())
	}
	selInk := strongestInk(tt, selRect)
	unselInk := strongestInk(tt, unselRect)
	if channelDiff(selInk, unselInk) < 20 {
		t.Errorf("the selected and unselected badges wear the same ink (%v vs %v); the "+
			"reference lifts the chosen one to --text-primary", selInk, unselInk)
	}
}

// A command row's `Tab` affordance goes through the same row slot as the
// numbered family, while the action bar's own `⌘↩` keeps the quieter
// `.launcher-action-bar__hint` register — the two are not the same text object.
func TestTheCommandRowTabHintUsesTheRowSlot(t *testing.T) {
	a := commandApp()
	tt := render(t, a)
	tt.Frame()

	if !tt.HasText("Tab") {
		t.Fatalf("the command row's Tab affordance is missing: %q", tt.Texts())
	}
	tabRect, ok := tt.Find("Tab")
	if !ok {
		t.Fatalf("the Tab hint was not drawn: %q", tt.Texts())
	}
	if tabRect.W < 30 {
		t.Errorf("the Tab slot is %.0f wide, want the action slot's 30u minimum", tabRect.W)
	}
	// The footer's own hint is the quieter caption step: the row slot must be
	// taller than it.
	barRect, ok := tt.Find(a.enterBadge())
	if !ok {
		t.Fatalf("the action bar's own hint was not drawn: %q", tt.Texts())
	}
	if tabRect.H <= barRect.H {
		t.Errorf("the Tab slot (h=%.0f) is not taller than the footer hint (h=%.0f); "+
			"the row slot is the body step, the footer hint the caption step",
			tabRect.H, barRect.H)
	}
}

// strongestInk is the pixel farthest (in summed channels) from the light panel
// surface inside a rect — the glyph's own colour, since text is darker than the
// plane it is drawn on.
func strongestInk(tt *ui.Tester, r ui.Rect) [3]int {
	img := tt.Image()
	var best [3]int
	bestSum := 1 << 30
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			sum := int(c.R) + int(c.G) + int(c.B)
			if sum < bestSum {
				bestSum = sum
				best = [3]int{int(c.R), int(c.G), int(c.B)}
			}
		}
	}
	return best
}

func channelDiff(a, b [3]int) int {
	d := 0
	for i := 0; i < 3; i++ {
		v := a[i] - b[i]
		if v < 0 {
			v = -v
		}
		if v > d {
			d = v
		}
	}
	return d
}
