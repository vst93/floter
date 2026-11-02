package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/i18n"
)

// The About card's fourth row (the `Project` link) is the one row §6.11 did not
// reach: the three facts above it became "name is the caption, value is the
// content" (`keyValue`, `settingsui.go:4313`), while this one kept the plain
// settings row (`a.row`) whose name is hard-wired to the emphasis step by
// `rowTitle` (`rows.go:1166`: `tokens.Emphasis` / 580 / `TextStrong`).
//
// On screen that was a label column that changes rung once: 11px / 500 /
// `TextSecondary` three times, then 15px / 580 / the card's heaviest ink
// (measured 202 / 198 light and dark, against the three names' 122 / 115). The
// card's only pressable row therefore read as its heading — and emphasis on a
// link is a *colour* question (`ui.Link` paints `Theme.Accent`), never a type
// ladder one (§6.10), so the label was what had to come back down.
//
// These pins hold the resolved ink and boxes of all four rows in one frame: four
// names on one rung (one line box, one column, one optical line with their
// values), four values on one rung too — the link's at `tokens.Emphasis` and in
// the accent's own hue.

// aboutRows are the four rows of the identity card, in draw order: three
// read-only facts and the one link.
func aboutRows() [][2]string {
	en := i18n.For("en").Settings
	return append(aboutFacts(), [2]string{en.AboutProject, aboutFixture().RepoURL})
}

// TestTheAboutCardsFourthRowKeepsTheCardsOwnLabelRung is the round's pin.
func TestTheAboutCardsFourthRowKeepsTheCardsOwnLabelRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := appearanceName(dark)
		tt := aboutPage(t, dark)
		rows := aboutRows()
		var plain, plainValue, linkedName int
		var plainH, plainValueH, colX float32
		for i, row := range rows {
			name, value := mustFind(t, tt, row[0]), mustFind(t, tt, row[1])
			nameInk, valueInk := inkSpan(tt, name), inkSpan(tt, value)
			t.Logf("%s row %d %q ink %d (box %v×%v x=%v y=%v) | %q ink %d (box %v×%v)",
				appearance, i, row[0], nameInk, name.W, name.H, name.X, name.Y, row[1], valueInk,
				value.W, value.H)

			// The name of a fact is the caption over its value, on every row of
			// this card — including the one that happens to be pressable.
			if nameInk > 150 {
				t.Errorf("%s: row %d's name %q resolves %d levels — on this card the name is the "+
					"caption over its value, not a setting's title (§6.11). The link's emphasis "+
					"belongs to the link's own ink, not to its label (§6.10)",
					appearance, i, row[0], nameInk)
			}
			// The row's content still carries it: the value is above its name.
			// The link cannot reach the facts' values in luma — `ui.Link` paints
			// `Theme.Accent`, and a hue's ceiling is its own brightness — so the
			// claim here is the direction, not the size of the step.
			if valueInk <= nameInk {
				t.Errorf("%s: row %d's value %q resolves %d against its name's %d — on this card "+
					"the value is the content and it may not sink under its own caption",
					appearance, i, row[1], valueInk, nameInk)
			}
			// One row, one optical line: the 11px caption and the 15px value have
			// to share a band, or the row reads as two lines.
			nameTop, nameBottom := inkRows(tt, name)
			valueTop, valueBottom := inkRows(tt, value)
			shorter := min(nameBottom-nameTop, valueBottom-valueTop)
			if overlap := min(nameBottom, valueBottom) - max(nameTop, valueTop); overlap < 0.6*shorter {
				t.Errorf("%s: row %d's %q ink spans rows %v-%v and its value rows %v-%v — they "+
					"overlap by %.0f of the shorter band, so the pair reads as two lines",
					appearance, i, row[0], nameTop, nameBottom, valueTop, valueBottom, overlap)
			}
			// One rung means one line box: the four captions are all 11px, so
			// their boxes are the same height; the four values are all the
			// emphasis step, so theirs are too (the link was the odd one out at
			// the body step: a 17px box against three 18px ones).
			if i == 0 {
				plainH, plainValueH, colX = name.H, value.H, name.X
				plain, plainValue = nameInk, valueInk
			} else {
				if d := name.H - plainH; d > 0.5 || d < -0.5 {
					t.Errorf("%s: row %d's name %q is drawn in a %vpx-tall box, the card's first "+
						"name in %vpx — the four names are one rung of the type ladder",
						appearance, i, row[0], name.H, plainH)
				}
				if d := value.H - plainValueH; d > 0.5 || d < -0.5 {
					t.Errorf("%s: row %d's value %q is drawn in a %vpx-tall box, the card's first "+
						"value in %vpx — the four values are one rung of the type ladder",
						appearance, i, row[1], value.H, plainValueH)
				}
			}
			// One column: the four names start at the same x.
			if d := name.X - colX; d > 0.5 || d < -0.5 {
				t.Errorf("%s: row %d's name %q starts at x=%v, the card's first name at x=%v — "+
					"the four names are one column", appearance, i, row[0], name.X, colX)
			}
			if i == len(rows)-1 {
				linkedName = nameInk
				// The link is the card's only accent-coloured string: its ink is
				// a hue, and the three facts' values are the strong rung.
				if valueInk == plainValue {
					t.Errorf("%s: the link resolves %d levels, the same as the three facts' values "+
						"(%d) — the link's identity is its hue, not the strong ink", appearance,
						valueInk, plainValue)
				}
			} else if i == 0 {
				plainValue = valueInk
			}
		}
		// The rung itself: the linked row's name is not a step heavier than its
		// three neighbours in the same column. Ink is a three-channel sum here,
		// so one brightness level is three units.
		const level = 3
		if plain == 0 {
			t.Fatalf("%s: the fact rows reported no name ink", appearance)
		}
		if d := linkedName - plain; d > level || d < -level {
			t.Errorf("%s: the link row's name resolves %d and the fact rows' names %d (%+d) — the "+
				"fourth row of this card wore the rung §6.11 replaced, so the label column changed "+
				"size, weight and ink halfway down the card", appearance, linkedName, plain, d)
		}
	}
}

// TestTheAboutLinksValueStillFitsANarrowWindow guards the width the round's
// rung cost: the link grew from the body step to the emphasis step (14 → 15px,
// `https://github.com/vst93/floter` 159 → 179px wide), and §6.11's 14-8 is the
// open item about this card's label column at 640. At the window's narrow end
// the value is still inside the card and the caption still has room for its
// own word — a rung may not buy hierarchy with a truncation.
func TestTheAboutLinksValueStillFitsANarrowWindow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := appearanceName(dark)
		a := New(newStore(t), Actions{})
		a.About = aboutFixture()
		tt, _ := renderIn(t, a, 640, 1200, dark)
		if err := tt.Click("About"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		rows := aboutRows()
		name := mustFind(t, tt, rows[3][0])
		value := mustFind(t, tt, rows[3][1])
		// The card's own right edge: the widest fact's value ends inside it.
		cardRight := float32(0)
		for _, row := range rows[:3] {
			if v := mustFind(t, tt, row[1]); v.X+v.W > cardRight {
				cardRight = v.X + v.W
			}
		}
		t.Logf("%s 640px: link %v×%v at x=%v (ends %v), facts end at %v, caption %v×%v at x=%v",
			appearance, value.W, value.H, value.X, value.X+value.W, cardRight, name.W, name.H, name.X)
		if value.X+value.W > cardRight {
			t.Errorf("%s: at 640 the link ends at x=%v, past the card's own content edge %v — the "+
				"emphasis step (14 → 15px) pushed the card's widest string out of the card",
				appearance, value.X+value.W, cardRight)
		}
		if name.W < 30 {
			t.Errorf("%s: at 640 the caption's box is %v wide — the label column no longer has "+
				"room for a word", appearance, name.W)
		}
	}
}

// TestDumpP13DJAboutProject writes the round's visual evidence: the About page
// in both appearances, tagged by the caller so a before/after pair comes out of
// the same build.
func TestDumpP13DJAboutProject(t *testing.T) {
	if os.Getenv("FLOTER_P13DJ_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13DJ_DUMP=1 to write /tmp/floter-P13-dj-*.png")
	}
	skipWithoutTheReferenceFont(t)
	tag := os.Getenv("FLOTER_P13DJ_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		tt := aboutPage(t, dark)
		for i, row := range aboutRows() {
			name, value := mustFind(t, tt, row[0]), mustFind(t, tt, row[1])
			t.Logf("%s row %d %q ink %d box %v×%v | %q ink %d box %v×%v",
				appearanceName(dark), i, row[0], inkSpan(tt, name), name.W, name.H,
				row[1], inkSpan(tt, value), value.W, value.H)
		}
		writePNG(t, filepath.Join("/tmp", fmt.Sprintf("floter-P13-dj-about-%s-%s.png",
			appearanceName(dark), tag)), tt)
	}
}
