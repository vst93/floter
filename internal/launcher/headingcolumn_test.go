package launcher

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// P11-ac · 列表的分组头落在它自己的文字列上。
//
// The rule is one sentence: a section heading sits on the column where the
// titles of the rows it heads start. Both surfaces print that element — the
// launcher list's own blocks and the captured-output panel's groups — and
// before P11-ac they answered it twice, differently, and both wrong: the
// launcher's label hung 8u to the *right* of its titles (62u against 54u) and
// the output panel's hung 32u to their *left* (20u against 52u), while each
// site's own comment claimed the alignment.
//
// The assertion is an equality between two measured rects rather than a
// literal: a column that moves on one side and not the other is exactly the
// drift this pin exists to catch, and a future round that re-lays the rows
// moves both rects at once.
func TestTheGroupHeadingSitsOnTheRowsTextColumn(t *testing.T) {
	// §1 the launcher list: the recents' heading over the app it heads.
	a := testApp()
	a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
	a.SetRecent([]string{"/Applications/Editor.app"}, true)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) },
		InputWindowWidth, int(WindowHeight("small"))+200)
	tt.Frame()
	heading := mustFind(t, tt, "Recently launched")
	title := mustFind(t, tt, "Editor")
	if !sameColumn(heading.X, title.X) {
		t.Errorf("launcher: the section heading starts at x=%.1f, its row's title at x=%.1f: "+
			"a heading sits on the column of the rows it heads", heading.X, title.X)
	}
	// …and it is the same column the plain built-in rows above it use, so the
	// list has one text column rather than a per-block one.
	builtin := mustFind(t, tt, "Shut down the computer")
	if !sameColumn(heading.X, builtin.X) {
		t.Errorf("launcher: the section heading starts at x=%.1f, a built-in row's title at x=%.1f",
			heading.X, builtin.X)
	}

	// §2 the captured-output panel: two groups, each heading over its own rows.
	b := testApp()
	rows := `[{"id":"r0","title":"Alpha","group":"Group A"},` +
		`{"id":"r1","title":"Beta","group":"Group A"},` +
		`{"id":"r2","title":"Gamma","group":"Group B"}]`
	ot := runListOutput(t, b, rows)
	for headingLabel, titleLabel := range map[string]string{
		"Group A": "Alpha",
		"Group B": "Gamma",
	} {
		h := mustFind(t, ot, headingLabel)
		r := mustFind(t, ot, titleLabel)
		if !sameColumn(h.X, r.X) {
			t.Errorf("output: the group heading %q starts at x=%.1f, its row's title %q at x=%.1f",
				headingLabel, h.X, titleLabel, r.X)
		}
	}
}

// mustFind is Find with a failure that prints what the frame did draw, so a
// renamed label reads as a missing element rather than as a zero rect.
func mustFind(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	rect, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%q is not in the frame: %q", label, tt.Texts())
	}
	return rect
}

// sameColumn compares two layout offsets: the same sum can arrive as 54 and
// 54.000004 through float32 arithmetic, and that is not a different column.
func sameColumn(a, b float32) bool { return math.Abs(float64(a-b)) < 0.5 }

// The P11-ac visual evidence: the launcher list's section heading and the
// output panel's group heading in both appearances, so the report can point at
// a file and a pixel column rather than at a description.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11ACGroupHeadingPNGs(t *testing.T) {
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
		writeDump(t, "floter-P11-ac-list-"+appearance+".png", tt, backdrop)

		b := testApp()
		if err := b.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		rows := `[{"id":"r0","title":"Alpha","group":"Group A"},` +
			`{"id":"r1","title":"Beta","group":"Group A"},` +
			`{"id":"r2","title":"Gamma","group":"Group B"}]`
		ot := runListOutput(t, b, rows)
		writeDump(t, "floter-P11-ac-output-"+appearance+".png", ot, backdrop)
	}
}

// writeDump composites a translucent tester frame onto a neutral desktop and
// writes it to /tmp, printing the path and size for the report.
func writeDump(t *testing.T, name string, tt *ui.Tester, backdrop color.RGBA) {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, tt.Image().Rect.Dx(), tt.Image().Rect.Dy()))
	draw.Draw(frame, frame.Rect, &image.Uniform{C: backdrop}, image.Point{}, draw.Src)
	draw.Draw(frame, frame.Rect, tt.Image(), image.Point{}, draw.Over)
	path := filepath.Join("/tmp", name)
	f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, frame); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	t.Logf("%s (%dx%d)", path, frame.Rect.Dx(), frame.Rect.Dy())
}
