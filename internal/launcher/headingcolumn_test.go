package launcher

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
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
	tt := ui.NewTester(themedView(a),
		InputWindowWidth, int(WindowHeight("small"))+200)
	tt.Frame()
	heading := mustFind(t, tt, "Recently launched")
	title := mustFind(t, tt, "Editor")
	if !sameColumn(heading.X, title.X) {
		t.Errorf("launcher: the section heading starts at x=%.1f, its row's title at x=%.1f: "+
			"a heading sits on the column of the rows it heads", heading.X, title.X)
	}
	// …and it is the same column a *typed* frame's built-in row uses, so the
	// list has one text column rather than a per-block one. P11-bk: the empty
	// page no longer carries the built-in rows (it is the recents' own block),
	// so the second frame asks for one by name.
	typed := testApp()
	typed.SetQuery("shutdown")
	bt := ui.NewTester(themedView(typed),
		InputWindowWidth, int(WindowHeight("small"))+200)
	bt.Frame()
	builtin := mustFind(t, bt, "Shut down the computer")
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
//
// P13-dw: the comparator used to be a ±0.5 band, which is a tolerance by
// another name — it would call 48.6 and 49.0 the same column. It asks the
// question at the painter's own resolution instead (`tenths`, §6.77/§6.84):
// one column means one drawn position to a tenth of a pixel, no band.
func sameColumn(a, b float32) bool { return tenths(float64(a)) == tenths(float64(b)) }

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
		tt := ui.NewTester(themedView(a),
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

// TestTheSectionHeadingStartsAtTheShippedTextColumn is the pin the fixture swap
// needed: every other assertion in this file compares two columns **to each
// other** (`sameColumn`), so a fixture that draws the framework's stock ruler
// and one that draws the app's own ruler both pass — the whole family was blind
// to which ruler it stood on (P13-eg).
//
// Measured, not assumed (P13-eg, `InputWindowWidth` frame, 720px):
//
//	stock ruler   heading X=54   Y=66   H=22   title X=54   Y=100  H=24
//	shipped ruler heading X=48.6 Y=59.4 H=17   title X=48.6 Y=88.5 H=19
//
// 54u × 0.9 = 48.6 is the reference's text column, and 17 is the title line box
// (`designMetrics`). The second arm keeps the stock numbers in the file: if a
// fixture drifts back to a bare `ui.NewTester`, the shipped arm reads 54/22 and
// says so.
func TestTheSectionHeadingStartsAtTheShippedTextColumn(t *testing.T) {
	build := func(shipped bool) *ui.Tester {
		t.Helper()
		a := testApp()
		a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
		a.SetRecent([]string{"/Applications/Editor.app"}, true)
		draw := themedView(a)
		if !shipped {
			draw = func(c *ui.Context) { a.View(c) }
		}
		tt := ui.NewTester(draw, InputWindowWidth, int(WindowHeight("small"))+200)
		tt.Frame()
		return tt
	}
	_, _, _, captionLine := designMetrics()

	shipped := build(true)
	heading := mustFind(t, shipped, "Recently launched")
	title := mustFind(t, shipped, "Editor")
	if got, want := tenths(float64(heading.X)), tenths(54*designUnit()); got != want {
		t.Errorf("the section heading starts at x=%v on the shipped ruler, want %v (54u): "+
			"a fixture that draws the stock ruler reads 54", got, want)
	}
	if got, want := tenths(float64(title.Y)), tenths(88.5); got != want {
		t.Errorf("the first row's title starts at y=%v, want %v: the row above it is the shipped 17px line, "+
			"not the stock 22px one", got, want)
	}
	if got, want := tenths(float64(heading.H)), tenths(captionLine); got != want {
		t.Errorf("the heading's line box is %v tall, want the shipped caption line %v (the stock ruler draws 22)",
			got, want)
	}
	if !sameColumn(heading.X, title.X) {
		t.Errorf("the heading is at x=%.1f and its row's title at x=%.1f", heading.X, title.X)
	}

	// The other ruler, so the two numbers above cannot both be satisfied by one
	// fixture: this is what the pin reads when the theme is not installed.
	stock := build(false)
	stockHeading := mustFind(t, stock, "Recently launched")
	if got, want := tenths(float64(stockHeading.X)), tenths(54); got != want {
		t.Errorf("the stock ruler's heading starts at x=%v, want the framework's own 54 — the two rulers are "+
			"no longer told apart, so the assertion above proves nothing", got)
	}
	if got, want := tenths(float64(stockHeading.H)), tenths(22); got != want {
		t.Errorf("the stock ruler's heading line box is %v, want 22", got)
	}
}
