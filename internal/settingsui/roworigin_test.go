package settingsui

import (
	"image/png"
	"math"
	"os"
	"testing"

	"floter/internal/i18n"
)

// The P13-bs account: where a row's first line starts inside its row.
//
// A trailing row (`rowNote`, and so every switch, select and action row) is a
// two-column line: the text column on the left, the control slot on the right.
// The row's own height is the taller of the two, and until P13-bs the *text
// column* was centred across it — which is right for a row that carries a
// second grey line (the block is then the tall one and centring is a no-op) and
// wrong for a row that does not: a single 18px label centred against a 33px
// select starts 6px lower than the same label in the row above it.
//
// That is what the General page showed: the Terminal card's first row (the
// font-family select, the page's one trailing row with no hint) put its label
// **22.2px** under the group heading while Window / Menu bar / Material put
// theirs at **16.3px**, in both appearances. The rows under it in the same card
// — the two sliders, which are stacked rows — started at 16.3px, so one card
// carried two text origins.
//
// The pin is the reading, not the source: every group's first row label must
// start at the same distance under its own heading. Before P13-bs the spread
// was 5.9px; the tolerance is the renderer's own sub-pixel snapping (7.2px
// gaps draw at 6.7 or 7.7).

// firstRowLabels names, for each of the General page's groups, the text of the
// first row's own label inside the group's first card. Appearance's first row
// label is the heading's own text ("Appearance"), so it is left out: the other
// four groups are enough to hold the one origin, and the fifth is a segmented
// row whose label already sits at the top of its column by construction.
func firstRowLabels(copy i18n.Settings) []struct{ name, title, label string } {
	return []struct{ name, title, label string }{
		{"window", copy.GroupWindow, copy.LaunchAtStartup},
		{"menubar", copy.GroupMenuBar, copy.ShowMenubarIcon},
		{"terminal", copy.GroupTerminal, copy.FontFamily},
		{"material", copy.Material, copy.Glass},
	}
}

// TestTheFirstRowStartsWhereEveryOtherGroupDoes pins the one text origin: the
// room from a group's heading to the first line of its first card's first row
// is one number across the page, so no card's first line hangs below the
// others'.
func TestTheFirstRowStartsWhereEveryOtherGroupDoes(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, _ := renderIn(t, a, 720, 2000, appearance.dark)
		lo, hi := math.Inf(1), math.Inf(-1)
		readings := map[string]float64{}
		for _, g := range firstRowLabels(copy) {
			heading := mustFind(t, tt, g.title)
			label := mustFind(t, tt, g.label)
			gap := float64(label.Y) - float64(heading.Y+heading.H)
			readings[g.name] = gap
			lo, hi = math.Min(lo, gap), math.Max(hi, gap)
		}
		t.Logf("%s: heading→first row label %v", appearance.name, readings)
		if hi-lo > 1.5 {
			t.Fatalf("[%v] the groups' first rows do not share one text origin: %v — "+
				"spread %.1fpx, want ≤ 1.5px (P13-bs: the text column starts at the row's "+
				"content top, so a hint-less row cannot drop below its neighbours)",
				appearance.name, readings, hi-lo)
		}
	}
}

// TestDumpGeneralForP13bs writes the General page at both appearances to
// /tmp, for the round's own before/after comparison. Opt-in: `FLOTER_DUMP=1`.
func TestDumpGeneralForP13bs(t *testing.T) {
	if os.Getenv("FLOTER_DUMP") == "" {
		t.Skip("opt-in: set FLOTER_DUMP=1 to write /tmp/floter-P13-bs-general-*.png")
	}
	for _, name := range []string{"dark", "light"} {
		dark := name == "dark"
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, _ := renderIn(t, a, 720, 2000, dark)
		path := "/tmp/floter-P13-bs-general-" + name + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
