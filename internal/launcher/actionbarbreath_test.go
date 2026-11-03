package launcher

import (
	"image/png"
	"math"
	"os"
	"testing"
	"time"

	"floter/internal/apps"

	"github.com/egoist/mygo/ui"
)

// P13-et · The action bar's band is **three** segments, and the row has no floor
// of its own.
//
// `rowBody` used to say, in prose, that the bar row carries its own 42u floor
// (`Space(10.5)`) plus the 3u breath above it, and the window charges the two
// together as the old build's 45u band. The assignment was real, the effect was
// not: it landed *after* `MinHeight(floor)` had already taken `Space(rowUnits)`,
// so the row was 36 px and the band's third segment — the card's own 2u tail
// (`TailUnits`, which the shell sets as the launcher's `padBottom`) — was what
// filled the rest of the band.
//
// That made the dead line a trap rather than clutter: both accounts sum to 45u
// (3 + 42 vs. 3 + 40 + 2), so the *total* is identical and nothing looks wrong —
// but "fixing" the floor into a real one grows the row to 37.8 px, whose bottom
// edge lands on the window's bottom edge and drops the selected capsule's last
// breath (see `TestTheActionBarKeepsTheCardsTailUnderIt`).
func TestTheActionBarBandIsBreathRowAndTail(t *testing.T) {
	a := testApp()
	// The step is the theme's, and the theme arrives with a frame (P13-ec: a
	// fixture read before its frame measures the framework's stock ruler).
	frame := render(t, a)
	frame.Frame()
	sp := float64(a.Spacing)
	if sp <= 0 {
		t.Fatalf("the fixture never took the shipped step: Spacing %.2f", sp)
	}
	breath := sp * actionBarBreathUnits
	row := sp * rowUnits
	tail := sp * TailUnits
	band := sp * actionBarUnits

	if breath <= 0 || row <= 0 || tail <= 0 {
		t.Fatalf("one of the band's three segments is gone: breath %.2f + row %.2f + tail %.2f", breath, row, tail)
	}
	if got := breath + row + tail; math.Abs(got-band) > 0.02 {
		t.Errorf("the action bar's band is %.2f px but its parts add to %.2f px: breath %.2f + the row's own %.2f + the card's tail %.2f — the band is three segments, not a 42u floor plus a breath (P13-et)", band, got, breath, row, tail)
	}
}

// barBreathApps is a list long enough to overflow the window, so the bar is a
// pinned footer over a scroller rather than the only row.
func barBreathApps(app *App) {
	var list []apps.App
	for _, name := range []string{
		"app-a", "app-b", "app-c", "app-d", "app-e", "app-f",
		"app-g", "app-h", "app-i", "app-j", "app-k", "app-l",
	} {
		list = append(list, apps.App{Name: name, Path: "/usr/bin/" + name})
	}
	app.SetApps(list)
}

// TestTheActionBarKeepsTheCardsTailUnderIt pins the row's *box*, which is what
// the dead floor would have changed: the bar row is the same 40u box a result
// row is, it starts one breath below the band's top edge, and its bottom edge
// stops one `TailUnits` above the window's bottom edge — the card's own tail,
// the breath the selected capsule would lose if it were charged 42u.
func TestTheActionBarKeepsTheCardsTailUnderIt(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		// The window the app asks for, measured off a frame that has room: the
		// test frame is taller than the charged window on purpose, so the
		// charged height has to be read from the app, not from the tester.
		charged := 0
		a := testApp()
		barBreathApps(a)
		a.Actions.ResizeTo = func(h int) { charged = h }
		ta := render(t, a)
		ta.Type("app")
		ta.Frame()
		if charged <= 0 {
			t.Fatal("the app asked for no window height")
		}

		b := testApp()
		barBreathApps(b)
		b.Query = "app"
		frame := ui.NewTester(themedView(b), InputWindowWidth, charged)
		if dark {
			frame.SetDark(true)
			frame.Advance(500 * time.Millisecond)
		}
		frame.Frame()
		// `rowRects` is the previous frame's layout (see `rowBody`), so the
		// second frame is the one whose boxes are the ones just drawn.
		frame.Frame()

		results := b.Results()
		bar := len(results) - 1
		if bar < 0 || actionBarIndex(results) != bar {
			t.Fatalf("the bar is not the list's last row: %v", ids(results))
		}
		box, ok := b.rowRects[bar]
		if !ok || box.H == 0 {
			t.Fatalf("the bar's row box was never laid out: %v", b.rowRects[bar])
		}

		sp := float64(b.Spacing)
		if got, want := float64(box.H), sp*rowUnits; math.Abs(got-want) > 0.02 {
			t.Errorf("the action bar's row is %.2f px tall, not the result row's %.2f px: a bar charged a floor of its own would be %.2f and would eat the card's tail (P13-et)", got, want, sp*10.5)
		}
		bandTop := float64(charged) - sp*actionBarUnits
		if got, want := float64(box.Y), bandTop+sp*actionBarBreathUnits; math.Abs(got-want) > 0.02 {
			t.Errorf("the action bar starts at %.2f, not one breath (%.2f) below its band's top edge (%.2f): got %.2f px of air above it (P13-et)", got, sp*actionBarBreathUnits, bandTop, float64(box.Y)-bandTop)
		}
		if got, want := float64(charged)-float64(box.Y+box.H), sp*TailUnits; math.Abs(got-want) > 0.02 {
			t.Errorf("the action bar's row ends %.2f px above the window's bottom edge, not the card's tail of %.2f px: the selected capsule has lost its last breath and sits on the rim (P13-et)", got, want)
		}
	})
}

// TestDumpP13ETBarPNGs writes the frame for a human eye
// (opt-in: `FLOTER_DUMP_P13ET=1`).
func TestDumpP13ETBarPNGs(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_P13ET") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_DUMP_P13ET=1")
	}
	for _, dark := range []bool{false, true} {
		charged := 0
		a := testApp()
		barBreathApps(a)
		a.Actions.ResizeTo = func(h int) { charged = h }
		ta := render(t, a)
		ta.Type("app")
		ta.Frame()

		b := testApp()
		barBreathApps(b)
		b.Query = "app"
		frame := ui.NewTester(themedView(b), InputWindowWidth, charged)
		if dark {
			frame.SetDark(true)
			frame.Advance(500 * time.Millisecond)
		}
		frame.Frame()
		frame.Frame()

		name := "light"
		if dark {
			name = "dark"
		}
		f, err := os.Create("/tmp/floter-P13-et-bar-" + name + ".png")
		if err != nil {
			t.Fatalf("cannot write the frame: %v", err)
		}
		if err := png.Encode(f, frame.Image()); err != nil {
			t.Fatalf("cannot encode the frame: %v", err)
		}
		f.Close()
	}
}
