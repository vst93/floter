package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/drops"
	"floter/internal/settings"
	"floter/internal/theme"

	"github.com/egoist/mygo/ui"
)

// P13-f · `docs/liquid-glass.md` §6.5 文件页：这一页的 E1 终于有一条像素证据，
// 它的图标列也终于不再说「最近」。
//
// 文件页的行不是另一套绘制——`files.go:144` 给每条行的是 `drop:<action>:<path>`，
// 没有 `bar:` 前缀，于是它们走的就是 `rowBody` 的普通结果行：选中块在 P12-b 之后
// **已经**是 E1（`tokens.LitFace`）。问题是这一页从来没有量过它，「继承到了」与
// 「量到了」是两件事；而这一页每行第一眼落在那块 28u 的图标板上，板上画的是
// `rowGlyph` 尾巴的时钟——十五个「最近用过」，把三个动作说成同一件事。

// filesPinApp is the files page over one small drop, in one appearance. Three
// files is three actions each and no expander: the page's ordinary shape.
func filesPinApp(t *testing.T, appearance string) *App {
	t.Helper()
	files := []drops.File{
		{Path: "/tmp/report.pdf", Name: "report.pdf", Directory: "/tmp"},
		{Path: "/tmp/notes.md", Name: "notes.md", Directory: "/tmp"},
		{Path: "/tmp/budget.xlsx", Name: "budget.xlsx", Directory: "/tmp"},
	}
	a := filesPageApp(t, "en", files, false)
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	return a
}

// TestTheFilesPageLeadsWithItsOwnActionMarks pins §6.5 row 5-2 (and 5-3): the
// three actions a dropped file offers are the three the path action bar already
// wears, so they wear the same marks — folder / terminal / file. The page used
// to lead every one of those rows with the recents' clock: fifteen history
// marks over five files, the one glyph none of those rows is.
func TestTheFilesPageLeadsWithItsOwnActionMarks(t *testing.T) {
	a := filesPinApp(t, "light")
	want := map[string]string{
		"drop:open:": "folder",   // bar:path-open
		"drop:cd:":   "terminal", // bar:path-cd
		"drop:copy:": "file",     // bar:path-copy
	}
	seen := map[string]bool{}
	for _, item := range a.filesItems() {
		if item.heading {
			continue
		}
		var prefix string
		for p := range want {
			if len(item.ID) >= len(p) && item.ID[:len(p)] == p {
				prefix = p
			}
		}
		if prefix == "" {
			continue // the expander, pinned by the arm below
		}
		seen[prefix] = true
		g, ok := rowGlyph(item)
		if !ok {
			t.Errorf("the %q row has no glyph", item.ID)
			continue
		}
		if got := glyphName(g); got != want[prefix] {
			t.Errorf("the %q row leads with %s, want %s (the files page's three actions are the action bar's "+
				"three actions, so they wear its three marks)", item.ID, got, want[prefix])
		}
	}
	for prefix := range want {
		if !seen[prefix] {
			t.Errorf("no %q row was drawn, so its mark was never checked: %v", prefix, ids(a.filesItems()))
		}
	}

	// 5-3: the tail row that collapses the rest of the drop *reveals* rows more —
	// it is not a recent launch, so it is not the clock either.
	expander := Item{ID: "drop:more", Title: "…and 2 more files"}
	if g, ok := rowGlyph(expander); !ok || glyphName(g) != "chevron" {
		t.Errorf("the expander row leads with %s (ok=%v), want chevron: it opens the list, it does not "+
			"list a launch", glyphName(g), ok)
	}
}

// TestTheFilesPageChosenRowStandsOnOneLitEdge pins §6.5 row 5-1 — the row's own
// claim, 「与 1-2 同一条规则」: the chosen row on *this* page is the same E1 block
// the launcher list draws (face step, one lit top edge, no cast of its own).
//
// It is measured here rather than inherited on paper because the page's rows are
// three per file with a group heading above them and a path in every subtitle —
// the one list in the app where the chosen block is not adjacent to another
// block of the same kind.
func TestTheFilesPageChosenRowStandsOnOneLitEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			a := filesPinApp(t, appearance)
			tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), dark)
			// The rows enter over 120ms (rowTransition); a frame read at opacity
			// 0 is the panel's own fill.
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			img := tt.Image()
			tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})

			row, ok := tt.Find("Open  report.pdf") // the drop's first row is chosen
			if !ok {
				t.Fatalf("the files page's first row was not drawn: %q", tt.Texts())
			}
			// A column inside the chosen block and clear of both the 28u plate
			// and the title's ink: two pixels left of where the title starts.
			x := int(row.X) - 2
			panel := img.RGBAAt(1, int(row.Y+row.H/2))
			block := blockAround(img, x, int(row.Y), int(row.Y+row.H))
			face := faceOf(img, block)

			// ① The face rung: the block is visibly a 面 over the ground.
			if step := channelGap(face, panel); step < litedgeFaceFloor {
				t.Errorf("the chosen file row's face is %v against the panel %v — a %d-level step, under the E1 "+
					"floor %d (liquid-glass.md §1)", face, panel, step, litedgeFaceFloor)
			}
			// ② …plus the lit top edge, which is `LensRim` and not the floating
			// `EdgeHighlight` (§2): E1 and E2 are two edges, not one.
			crown, ok := crownOf(img, block, tokens.LensRim)
			if !ok {
				t.Fatalf("the chosen file row is a flat block (%v, face %v): none of its top four rows is LensRim "+
					"%v over that face — the files page's chosen row is not E1 "+
					"(liquid-glass.md §6.5 第 5-1 行)", block, face, tokens.LensRim)
			}
			if !brighter(crown, face) {
				t.Errorf("the chosen file row's top edge %v is not brighter than its own face %v", crown, face)
			}
			// ③ E1 casts nothing: one pixel outside the block's left gutter, at
			// the block's own height, is still the panel.
			gutter := int(ui.LightTheme().Space(2))
			if side := img.RGBAAt(gutter-2, int(row.Y+row.H/2)); side != panel {
				t.Errorf("the chosen file row casts a shadow beside itself (%v against the panel %v): an E1 block "+
					"lies on the ground (liquid-glass.md §2 判据 2)", side, panel)
			}
		})
	}
}

// TestDumpFilesPagePNGs is the round's visual evidence: the files page, both
// appearances, written to /tmp. Opt-in, as every other dump in this package is;
// `FLOTER_PNG_SUFFIX` keeps the round's before/after frames apart.
func TestDumpFilesPagePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		files := []drops.File{
			{Path: "/tmp/report.pdf", Name: "report.pdf", Directory: "/tmp"},
			{Path: "/tmp/notes.md", Name: "notes.md", Directory: "/tmp"},
			{Path: "/tmp/screenshots/png/shot.png", Name: "shot.png", Directory: "/tmp/screenshots/png"},
			{Path: "/tmp/budget.xlsx", Name: "budget.xlsx", Directory: "/tmp"},
			{Path: "/tmp/invoice-2026-10.pdf", Name: "invoice-2026-10.pdf", Directory: "/tmp/Documents"},
			{Path: "/tmp/photo.jpg", Name: "photo.jpg", Directory: "/tmp"},
			{Path: "/tmp/gone.txt", Name: "gone.txt", Directory: "/tmp"},
		}
		a := filesPageApp(t, "en", files, false)
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), dark)
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-f-files-"+appearance+suffix+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		img := tt.Image()
		t.Logf("%s (%dx%d)", path, img.Rect.Dx(), img.Rect.Dy())
	}
}
