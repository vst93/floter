package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// The empty region's fourth part: its way out.
//
// `docs/liquid-glass.md` §4.3 fixed the first three parts (mark, sentence, why)
// and left the last pair — reason → action — as "8u + 2u margin, unchanged,
// not measured". Measured, the sessions page's way out is a **plate**: a 30u
// `Control` face with its `ControlEdge` pair, its lit rim and its own shadow.
// It sat **14px** below the reason's ink while the reason sits **12px** below
// the sentence: two pixels of difference between "the next line" and "a thing
// you press". The reference's 2px is written for the *other* arm, where the way
// out is a `+` and a line of accent text (the custom-shortcuts group) — a line
// belongs to the paragraph it follows.
//
// P13-ap 重定基线：板那一档 2u → 8u（盒子 10u → 16u，墨上 14 → 23px）；文字那一档
// 仍是 2u（墨上 10px）。These pins hold the *resolved* distance and the *kind*
// of the thing that distance is above, in both appearances.

// pageSessionsEmpty draws the sessions page — the one empty region whose way
// out is a plate — at a size where its body is in view.
func pageSessionsEmpty(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	a := New(settings.NewStore(settings.Default()), Actions{NewSession: func() {}})
	a.selectPage(PageSessions)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 720, 720)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt
}

// regionPlane is the luma this window resolves as its own ground: the median
// pixel of the whole window, so a region on a card face and one on a sunken
// plane measure the same.
func regionPlane(tt *ui.Tester, x0, x1, y0, y1 int) int {
	img := tt.Image()
	lumas := make([]int, 0, (x1-x0)*(y1-y0))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			lumas = append(lumas, luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]))
		}
	}
	sort.Ints(lumas)
	return lumas[len(lumas)/2]
}

// firstRule finds the first row in [y0,y1) that holds at least `width` pixels
// further than `level` luma steps from the plane: the top edge of a plate, a
// keyline — the first thing after a paragraph that is not a glyph.
func firstRule(tt *ui.Tester, x0, x1, y0, y1, width, level int) int {
	img := tt.Image()
	base := regionPlane(tt, x0, x1, y0, y1)
	for y := y0; y < y1; y++ {
		n := 0
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			d := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) - base
			if d > level || d < -level {
				n++
			}
		}
		if n >= width {
			return y
		}
	}
	return -1
}

// lastInkRow is the bottom row of a text box's glyphs: the last row inside the
// box that carries at least three pixels off the plane.
func lastInkRow(tt *ui.Tester, x0, x1, y0, y1 int) int {
	img := tt.Image()
	base := regionPlane(tt, x0, x1, y0, y1)
	last := -1
	for y := y0; y < y1; y++ {
		n := 0
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			d := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) - base
			if d > 12 || d < -12 {
				n++
			}
		}
		if n >= 3 {
			last = y
		}
	}
	return last
}

// TestTheEmptyRegionsWayOutStandsBelowItsSentence pins the last pair of the
// empty region: the plate the user presses stands further off the paragraph
// than one line of that paragraph stands from the next.
func TestTheEmptyRegionsWayOutStandsBelowItsSentence(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := pageSessionsEmpty(t, dark)
		// The three text parts, on the ink, with the same reader §4.3's pins use.
		why, ok := tt.Find(en.SessionsEmptyHint)
		if !ok {
			t.Fatalf("%s: the empty region's why is missing: %q", appearance, tt.Texts())
		}
		label, ok := tt.Find(en.SessionsNew)
		if !ok {
			t.Fatalf("%s: the empty region's way out is missing: %q", appearance, tt.Texts())
		}
		// The plate's own top edge: the first row under the reason's ink that is
		// a rule rather than a glyph (12+ pixels off the plane).
		inkBottom := lastInkRow(tt, emptyLeft, emptyRight, int(why.Y), int(why.Y+why.H)+1)
		if inkBottom < 0 {
			t.Fatalf("%s: the reason drew no ink in its own box (%v..%v)", appearance, why.Y, why.Y+why.H)
		}
		edge := firstRule(tt, emptyLeft, emptyRight, inkBottom+2, 700, 12, 12)
		if edge < 0 {
			t.Fatalf("%s: no plate edge resolved under the reason (reason ink ends at %d)",
				appearance, inkBottom)
		}
		air := float32(edge-int(inkBottom)) - 1
		if air < 20 {
			t.Errorf("%s: the way out sits %.0fpx under the reason's ink — the plate reads as a "+
				"fourth line of the paragraph instead of the thing the paragraph is for (§4.3)",
				appearance, air)
		}
		t.Logf("%s: reason ink ends %d, plate edge %d, air %.0fpx (label y=%.1f)",
			appearance, inkBottom, edge, air, label.Y)
		if air > 34 {
			t.Errorf("%s: the way out drifted %.0fpx off its paragraph — the region now reads as "+
				"two unrelated blocks", appearance, air)
		}
		// It is still a plate: the label sits inside it, not beside it.
		if label.Y < float32(edge) || label.Y > float32(edge)+26 {
			t.Errorf("%s: the label sits at y=%.0f, outside the plate whose top edge is %d — "+
				"the air above the plate moved, the plate did not", appearance, label.Y, edge)
		}
		if d := label.Y - (why.Y + why.H); d < 22 {
			t.Errorf("%s: label box sits %.0fpx under the reason's box — want the 20u pair (≥22px), "+
				"got the paragraph's own rhythm", appearance, d)
		}
	}
}

// TestTheTextWayOutStaysOnTheParagraphsRhythm is the other arm: the custom
// shortcuts group's way out is a glyph and a line of accent text, so it belongs
// to the paragraph above it and keeps the reference's 2u.
func TestTheTextWayOutStaysOnTheParagraphsRhythm(t *testing.T) {
	en := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	draw := func(kind bool) *ui.Tester {
		var ref ui.Handle
		tt := ui.NewTester(func(c *ui.Context) {
			a.emptyState(c, "keyboard", en.ShortcutsNone, en.ShortcutsEmptyHint, 0, 0, kind,
				func() {
					if kind {
						a.pageButton(c, en.SessionsNew)
						return
					}
					a.actionWithGlyph(c, "plus", en.ShortcutsAddFirst, func() {})
				}).Bind(&ref)
			_ = ref
		}, 720, 400)
		return tt
	}
	get := func(kind bool, want string) (float32, float32) {
		tt := draw(kind)
		tt.Frame()
		tt.Frame()
		hint, ok := tt.Find(en.ShortcutsEmptyHint)
		if !ok {
			t.Fatalf("the why is missing: %q", tt.Texts())
		}
		label, ok := tt.Find(want)
		if !ok {
			t.Fatalf("%q is missing: %q", want, tt.Texts())
		}
		return label.Y, label.Y - (hint.Y + hint.H)
	}
	plateLabel, plateGap := get(true, en.SessionsNew)
	textLabel, textGap := get(false, en.ShortcutsAddFirst)
	if plateGap-textGap < 6 {
		t.Errorf("the plate sits %.0fpx under the why and the text line %.0fpx — the two "+
			"kinds of way out must not resolve to the same distance (§4.3)", plateGap, textGap)
	}
	if textGap > 16 {
		t.Errorf("the text way out sits %.0fpx under the why, want the reference's 2u "+
			"(≈10px): a line of text belongs to the paragraph it follows", textGap)
	}
	t.Logf("plate label y=%.0f (+%.0f under the why) | text label y=%.0f (+%.0f)",
		plateLabel, plateGap, textLabel, textGap)
}

// TestDumpEmptyActionInk writes the frames this round measured. Opt-in.
func TestDumpEmptyActionInk(t *testing.T) {
	if os.Getenv("FLOTER_P13AP_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13AP_DUMP=1 to write /tmp/floter-P13-ap-*.png")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := pageSessionsEmpty(t, dark)
		en := i18n.For("en").Settings
		why, _ := tt.Find(en.SessionsEmptyHint)
		inkBottom := lastInkRow(tt, emptyLeft, emptyRight, int(why.Y), int(why.Y+why.H)+1)
		edge := firstRule(tt, emptyLeft, emptyRight, inkBottom+2, 700, 12, 12)
		fmt.Printf("%s: reason ink ends %d, plate edge %d, air %dpx\n",
			appearance, inkBottom, edge, edge-inkBottom-1)
		writePNG(t, filepath.Join("/tmp", "floter-P13-ap-sessions-empty-"+appearance+".png"), tt)
	}
}
