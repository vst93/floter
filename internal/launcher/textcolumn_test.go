package launcher

import (
	"image/color"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// P11-ad · 同一个窗口里的两种列表只有一条文字列。
//
// The launcher's result list and its own captured-output panel are two lists in
// **one card**: the same window, the same 8u card inset, the same 28u icon
// column, the same 8u row padding. The only横向 term that differed was the
// row's own gap — 10u in the list, 8u in the panel — so a row's title started
// at 54u in one and 52u in the other, and a user switching between the list and
// the output saw the text column shift by a hair for no reason.
//
// The assertion is the two measured offsets being equal, not a literal 54: a
// future round that re-lays either surface's rows moves both, and this pin is
// about the two surfaces agreeing rather than about the number they agree on.
// P11-ac's pin (`TestTheGroupHeadingSitsOnTheRowsTextColumn`) keeps each
// heading on its own surface's column; this one keeps the two surfaces' columns
// on each other.
func TestTheTwoListsShareOneTextColumn(t *testing.T) {
	// §1 the launcher list: a recent app's row.
	a := testApp()
	a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
	a.SetRecent([]string{"/Applications/Editor.app"}, true)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) },
		InputWindowWidth, int(WindowHeight("small"))+200)
	tt.Frame()
	listTitle := mustFind(t, tt, "Editor")

	// §2 the captured-output panel: the same window, its own list.
	b := testApp()
	rows := `[{"id":"r0","title":"Alpha","group":"Group A"},` +
		`{"id":"r1","title":"Beta","group":"Group A"},` +
		`{"id":"r2","title":"Gamma","group":"Group B"}]`
	ot := runListOutput(t, b, rows)
	outputTitle := mustFind(t, ot, "Alpha")

	if !sameColumn(listTitle.X, outputTitle.X) {
		t.Errorf("the launcher list's title starts at x=%.1f, the output panel's at x=%.1f: "+
			"two lists in one card share one text column", listTitle.X, outputTitle.X)
	}
	// The headings follow their own surface's column (P11-ac's pin), so a
	// heading that stayed behind while the rows moved is the same failure seen
	// from the other end.
	listHeading := mustFind(t, tt, "Recently launched")
	outputHeading := mustFind(t, ot, "Group A")
	if !sameColumn(listHeading.X, outputHeading.X) {
		t.Errorf("the launcher's heading starts at x=%.1f, the output panel's at x=%.1f",
			listHeading.X, outputHeading.X)
	}
	t.Logf("text column: list title %.1f / heading %.1f, output title %.1f / heading %.1f",
		listTitle.X, listHeading.X, outputTitle.X, outputHeading.X)
}

// The P11-ad visual evidence: both lists of the one card, both appearances, so
// the report can point at a file and a pixel column rather than a description.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11ADOutputPanelPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		backdrop := color.RGBA{R: 0xDF, G: 0xDF, B: 0xE4, A: 0xFF}
		if dark {
			appearance = "dark"
			backdrop = color.RGBA{R: 0x1C, G: 0x1C, B: 0x20, A: 0xFF}
		}

		a := testApp()
		a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
		a.SetRecent([]string{"/Applications/Editor.app"}, true)
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) },
			InputWindowWidth, int(WindowHeight("small"))+200)
		tt.Frame()
		writeDump(t, "floter-P11-ad-list-"+appearance+".png", tt, backdrop)

		b := testApp()
		if err := b.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		rows := `[{"id":"r0","title":"Alpha","group":"Group A"},` +
			`{"id":"r1","title":"Beta","group":"Group A"},` +
			`{"id":"r2","title":"Gamma","group":"Group B"}]`
		ot := runListOutput(t, b, rows)
		writeDump(t, "floter-P11-ad-output-"+appearance+".png", ot, backdrop)
	}
}
