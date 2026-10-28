package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// P11-ah · A wheel notch scrolls the result list its whole distance.
//
// The list's rows are ~40u; a notch is 120u, so four notches are about twelve
// rows. The port's own reading was four: the pointer resting on the list took
// the selection each time the content slid under it, and the launcher answered
// with `List.ScrollIntoView` for the newly-chosen row. The framework reads a
// ScrollIntoView as "the app has taken the offset over" and drops the notch
// already on its way (`mygo`'s `glideWheels`, `s.list.scrolled`), so every
// notch stopped after its first row. The reveal is the *keyboard's* — a row
// the pointer entered is under the pointer, hence in view — so a pointer
// choice no longer asks for it (`App.pointerChose`).
//
// The selection still follows the pointer's row: R21's entry rule is untouched,
// and a wheel moves the content under a resting pointer row by row.
func TestTheWheelScrollsTheListUnderThePointer(t *testing.T) {
	a := testApp()
	items := make([]apps.App, 30)
	for i := range items {
		items[i] = apps.App{Name: "App" + string(rune('A'+i%26)) + string(rune('0'+i/26)), Path: "/app/i"}
	}
	a.SetApps(items)
	a.Query = "App"
	tt := render(t, a)
	tt.Frame()
	if len(a.Results()) <= maxResultSlots {
		t.Fatalf("the list holds %d results and the window %d rows, so there is nothing to scroll",
			len(a.Results()), maxResultSlots)
	}
	bounds := tt.Image().Bounds()
	x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
	tt.Move(x, y)
	tt.Frame()
	atRest := a.hoveredRow
	if atRest < 0 {
		t.Fatalf("the pointer rests over no row, so the wheel has no row to test: %v", vis2(a))
	}

	const notches = 4
	const notch = 120 // DIPs, the framework's per-notch distance
	first, _ := a.List.Visible()
	for i := 0; i < notches; i++ {
		tt.Wheel(x, y, 0, notch)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
	}
	scrolled, _ := a.List.Visible()
	// Twelve rows is the distance the notches ask for; ten is what the port's
	// own row height spends it on. The old reading was four.
	if moved := scrolled - first; moved < 8 {
		t.Errorf("four notches of %du moved the list %d rows (visible %d..%d), want about %d: "+
			"a notch is being cut short", notch, moved, scrolled, first, notches*notch/40)
	}
	// The wheel moves the content, not the choice: the selection is whatever
	// row the pointer is over, as R21 says it is on entry.
	if a.Selected != a.hoveredRow {
		t.Errorf("after scrolling the choice is row %d while the pointer is over row %d: "+
			"the wheel left the two apart", a.Selected, a.hoveredRow)
	}
	if a.hoveredRow <= atRest {
		t.Errorf("the row under the pointer did not move with the content: %d then %d", atRest, a.hoveredRow)
	}

	// And back: the notches up return the list to its start.
	for i := 0; i < notches; i++ {
		tt.Wheel(x, y, 0, -notch)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
	}
	if back, _ := a.List.Visible(); back != first {
		t.Errorf("four notches back left the list at row %d, want the start at %d", back, first)
	}
}

// P11-ah · The keyboard's own step still scrolls its row into view.
//
// This is the half `pointerChose` must not swallow. A row the pointer entered
// is in view by construction, so the pointer's reveal is skipped — but the
// keyboard walks the whole list, so the row it lands on can be outside the box
// and the list must follow it. The pointer is parked over the *field* here (a
// band with no rows), having first entered the list and left it: that is the
// case where a flag that is set and never cleared would go on suppressing the
// reveal for the rest of the session.
func TestTheKeyboardStillScrollsItsRowIntoView(t *testing.T) {
	a := testApp()
	items := make([]apps.App, 30)
	for i := range items {
		items[i] = apps.App{Name: "App" + string(rune('A'+i%26)) + string(rune('0'+i/26)), Path: "/app/i"}
	}
	a.SetApps(items)
	a.Query = "App"
	tt := render(t, a)
	tt.Frame()
	bounds := tt.Image().Bounds()

	// The pointer enters the list, taking the choice, and then leaves it for
	// the field's own band, which holds no row.
	x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
	tt.Move(x, y)
	tt.Frame()
	if a.hoveredRow < 0 {
		t.Fatalf("the pointer entered no row: %v", vis2(a))
	}
	tt.Move(x, 20)
	tt.Frame()
	if a.hoveredRow != -1 {
		t.Fatalf("the pointer left the list but still rests on row %d", a.hoveredRow)
	}
	entered := a.Selected

	// Walk down past the bottom of the box: the list has to follow.
	for i := 0; i < 15; i++ {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	if a.Selected <= entered {
		t.Fatalf("the arrow keys did not move the choice: %d then %d", entered, a.Selected)
	}
	first, last := a.List.Visible()
	if a.Selected < first || a.Selected > last {
		t.Errorf("the choice is row %d while the list shows %d..%d: the keyboard's row was not "+
			"scrolled into view", a.Selected, first, last)
	}
}

// vis2 is Visible in a form a log line can print.
func vis2(a *App) [2]int {
	first, last := a.List.Visible()
	return [2]int{first, last}
}

// P11-ah visual evidence: the list before and after the notches, in both
// appearances. Opt-in (`FLOTER_PNG=1`) like the other dumps, so a test run does
// not litter /tmp.
func TestDumpP11AHWheelPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		items := make([]apps.App, 30)
		for i := range items {
			items[i] = apps.App{Name: "App" + string(rune('A'+i%26)) + string(rune('0'+i/26)), Path: "/app/i"}
		}
		a.SetApps(items)
		a.Query = "App"
		tt := render(t, a)
		tt.SetDark(dark)
		dump := func(tag string) {
			path := filepath.Join("/tmp", "floter-P11-ah-wheel-"+tag+"-"+appearance+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s", path)
		}
		tt.Frame()
		dump("rest")
		bounds := tt.Image().Bounds()
		x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
		tt.Move(x, y)
		tt.Frame()
		for i := 0; i < 2; i++ {
			tt.Wheel(x, y, 0, 120)
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
		}
		dump("scrolled")
	}
}
