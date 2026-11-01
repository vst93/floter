package settingsui

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The P13-br account: the General page's five section groups, read as pixels.
//
// Two claims are pinned here, both of them geometry rather than source text —
// the source can be re-argued, the picture cannot:
//
//  1. every group keeps the *same* room between its heading and its first card
//     (the group's one internal tick, `.settings-section { gap: 8u }`), and
//  2. the material group is still the one section that sits inset from its own
//     box (`padding: 10u 0`), while the page's between-section rhythm — the
//     room from one group's box to the next — stays the shared 22u for all of
//     them.
//
// The second is why the first could be unified at all: the exception the
// reference wrote into that one section was two things (`gap: 10u` *and*
// `padding: 10u 0`), and only the padding is large enough to read.

// generalSectionGroups are the General page's groups in page order, with the
// text that names each group's box (`sectionGroup` labels it) and the group's
// own title, which `sectionHeading` draws as the first child inside that box.
func generalSectionGroups(copy i18n.Settings) []struct{ name, title string } {
	return []struct{ name, title string }{
		{"appearance", copy.GroupAppearance},
		{"window", copy.GroupWindow},
		{"menubar", copy.GroupMenuBar},
		{"terminal", copy.GroupTerminal},
		{"material", copy.Material},
	}
}

// cardTopGap is the number of physical pixels between a group's heading text
// and the top edge of the first card under it — the group's own gap, since the
// heading row carries no vertical padding (`sectionHeadingInline`:
// `Padding(0, Space(0.5))`) and the card is the group's next child.
//
// It is read off the picture rather than off the tree: the card's top edge is a
// colour step, and scanning a column inside the card finds it. The column sits
// at 55% of the group's width, right of every row's label and left of every
// control, so nothing but the card plane is on it.
func cardTopGap(t *testing.T, img *image.RGBA, box, heading ui.Rect) float64 {
	t.Helper()
	x := int(box.X + box.W*0.55)
	bg := imagePixel(img, x, int(box.Y)-3)
	bottom := heading.Y + heading.H
	for y := int(bottom); y < int(bottom)+60; y++ {
		if imagePixel(img, x, y) != bg {
			return float64(y) - float64(bottom)
		}
	}
	t.Fatalf("no card edge under the heading at x=%d (heading bottom %.1f): the group has no card, "+
		"or the scan column landed on something else", x, bottom)
	return 0
}

// TestTheSectionGroupsShareOneInternalGap pins the first claim: the room
// between a group's heading and its first card is one number across the page.
//
// The tolerance is not slack for the design, it is the renderer's own snapping:
// 8u is 7.2px, and a group whose box lands on a fractional y draws that edge at
// 6.7 or 7.7 depending on where it fell. The value this pin was written to
// catch — the material group's `gap: 10u`, 9.0px against 7.2px — is outside
// that band in every appearance, which is what makes the band a threshold
// rather than a fudge.
func TestTheSectionGroupsShareOneInternalGap(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, _ := renderIn(t, a, 720, 2000, appearance.dark)
		img := tt.Image()
		lo, hi := math.Inf(1), math.Inf(-1)
		readings := map[string]float64{}
		for _, g := range generalSectionGroups(copy) {
			box := mustFind(t, tt, sectionLabel(g.title))
			heading := mustFind(t, tt, g.title)
			gap := cardTopGap(t, img, box, heading)
			readings[g.name] = gap
			lo, hi = math.Min(lo, gap), math.Max(hi, gap)
		}
		t.Logf("%s: heading→card %v", appearance.name, readings)
		if hi-lo > 1.5 {
			t.Fatalf("[%v] the groups' internal gap is not one number: %v — "+
				"spread %.1fpx, want ≤ 1.5px (8u is 7.2px; the renderer snaps it to 6.7 or 7.7)",
				appearance.name, readings, hi-lo)
		}
	}
}

// TestTheMaterialGroupKeepsItsOwnPadding pins the second claim, and with it the
// reason the first could be unified: the material group's `padding: 10u 0` is
// still there (its heading sits inset from its own box top, every other group's
// sits flush), and that padding is *inside* the box — the room from one group's
// box to the next is the same 22u everywhere, so the material section reads
// looser without the page's rhythm growing a second value.
func TestTheMaterialGroupKeepsItsOwnPadding(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, tokens := renderIn(t, a, 720, 2000, appearance.dark)
		groups := generalSectionGroups(copy)
		boxes := make([]ui.Rect, len(groups))
		for i, g := range groups {
			boxes[i] = mustFind(t, tt, sectionLabel(g.title))
			heading := mustFind(t, tt, g.title)
			inset := float64(heading.Y - boxes[i].Y)
			if g.name == "material" {
				want := float64(tokens.Theme.Space(2.5))
				if math.Abs(inset-want) > 1 {
					t.Fatalf("[%v] the material group's heading sits %.1fpx inside its box, want %.1fpx "+
						"(`padding: 10u 0`, settings.css:767-772) — the section's only remaining identity",
						appearance.name, inset, want)
				}
				continue
			}
			if inset != 0 {
				t.Fatalf("[%v] %s's heading sits %.1fpx inside its box, want 0: only the material group "+
					"carries padding (P13-br)", appearance.name, g.name, inset)
			}
		}
		// The page's between-section rhythm: one value for all four gaps, and
		// the material group's padding is inside its box, not added to this.
		var first, spread float64
		for i := 1; i < len(boxes); i++ {
			gap := float64(boxes[i].Y - (boxes[i-1].Y + boxes[i-1].H))
			if i == 1 {
				first = gap
				continue
			}
			spread = math.Max(spread, math.Abs(gap-first))
		}
		t.Logf("%s: between-group gap %.0fpx (spread %.0f)", appearance.name, first, spread)
		if spread > 1 {
			t.Fatalf("[%v] the between-group gap is not one number: first %.0fpx, spread %.1fpx "+
				"(`pageColumn` keeps 22u for every section)", appearance.name, first, spread)
		}
	}
}

// TestDumpP13BRGeneralSectionRhythm is the round's visual evidence: the General
// page in both appearances, so the report can point at a file instead of at a
// description. Opt-in like every other dump in this package (`FLOTER_PNG=1`).
func TestDumpP13BRGeneralSectionRhythm(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	copy := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, _ := renderIn(t, a, 720, 2000, appearance.dark)
		img := tt.Image()
		path := "/tmp/floter-P13-br-general-" + appearance.name + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		// What the picture shows, in numbers, so the report quotes the frame it
		// is pointing at rather than the layout code that drew it.
		for _, g := range generalSectionGroups(copy) {
			box := mustFind(t, tt, sectionLabel(g.title))
			heading := mustFind(t, tt, g.title)
			t.Logf("%s %-10s box=(y=%.0f h=%.0f) heading-inset=%.0f heading→card=%.1f",
				appearance.name, g.name, box.Y, box.H, heading.Y-box.Y,
				cardTopGap(t, img, box, heading))
		}
		t.Logf("%s wrote %s (%dx%d)", appearance.name, path, img.Rect.Dx(), img.Rect.Dy())
	}
}
