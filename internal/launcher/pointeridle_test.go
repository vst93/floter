package launcher

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// walkApps builds n apps whose names all match the query "walk", so a test can
// walk a list longer than the window with one hand and rest the pointer with
// the other.
func walkApps(n int) []apps.App {
	out := make([]apps.App, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, apps.App{
			Name: fmt.Sprintf("Walk %02d", i),
			Path: fmt.Sprintf("/Applications/Walk%02d.app", i),
		})
	}
	return out
}

// rowByName is the centre of a row's own text, as the pointer enters it.
func rowByName(t *testing.T, tt *ui.Tester, name string) (float32, float32) {
	t.Helper()
	rect, ok := tt.Find(name)
	if !ok {
		t.Fatalf("no %q row: %q", name, tt.Texts())
	}
	return rect.X + rect.W/2, rect.Y + rect.H/2
}

// A pointer that does not move does not choose. R21 says the pointer's
// selection is an *entry* edge (the reference's `onPointerEnter`), but the port
// read "entry" as "the row under the pointer changed" — and a row changes
// under a still pointer whenever the list scrolls, which the keyboard makes it
// do on every step past the window's last row. Half the steps were handed back
// to the row that slid under the resting pointer, so fifteen Downs advanced
// eight rows.
func TestTheKeyboardWalksTheListUnderARestingPointer(t *testing.T) {
	a := testApp()
	a.SetApps(walkApps(20))
	tt := render(t, a)
	tt.Type("walk")
	tt.Frame()
	x, y := rowByName(t, tt, "Walk 03")
	tt.Move(x, y)
	tt.Frame()
	if a.Selected != 3 {
		t.Fatalf("the pointer did not enter row 3: selected=%d", a.Selected)
	}
	// The pointer stays where it is; only the keyboard moves.
	for i := 0; i < 15; i++ {
		tt.Key(0, ui.KeyDown)
	}
	if want := 18; a.Selected != want {
		t.Errorf("fifteen arrow steps from row 3 landed on %d, want %d: a resting pointer took the choice back",
			a.Selected, want)
	}
}

// The other half of the same rule: nothing here turns the pointer off. Once
// the pointer goes somewhere, the row it lands on is the choice again — the
// suppression is "a still pointer", not "the keyboard owns the list".
func TestThePointersWordCountsAgainOnceItMovesAfterAScroll(t *testing.T) {
	a := testApp()
	a.SetApps(walkApps(20))
	tt := render(t, a)
	tt.Type("walk")
	tt.Frame()
	x, y := rowByName(t, tt, "Walk 03")
	tt.Move(x, y)
	tt.Frame()
	for i := 0; i < 15; i++ {
		tt.Key(0, ui.KeyDown)
	}
	if a.Selected != 18 {
		t.Fatalf("the keyboard did not walk the list: selected=%d want 18", a.Selected)
	}
	// The pointer goes to a row the walk left on screen, away from the one the
	// scroll carried under it.
	x, y = rowByName(t, tt, "Walk 15")
	tt.Move(x, y)
	tt.Frame()
	if a.Selected != 15 {
		t.Errorf("after a keyboard scroll the pointer no longer chooses: selected=%d want 15", a.Selected)
	}
}

// The same rule on the other hand: a query that reflows the list under an idle
// pointer does not hand the choice to whatever row lands there. A new query
// starts on the first row, and that is where it stays until the user moves the
// pointer or the keyboard.
func TestATypedQueryKeepsTheChoiceOffTheIdlePointersRow(t *testing.T) {
	a := testApp()
	a.SetApps(walkApps(20))
	tt := render(t, a)
	tt.Type("walk")
	tt.Frame()
	x, y := rowByName(t, tt, "Walk 03")
	tt.Move(x, y)
	tt.Frame()
	if a.Selected != 3 {
		t.Fatalf("the pointer did not enter row 3: selected=%d", a.Selected)
	}
	// One more character narrows the list: rows move up under the still
	// pointer.
	tt.Type("0")
	tt.Frame()
	if a.Selected != 0 {
		t.Errorf("typing handed the choice to the idle pointer's row: selected=%d want 0", a.Selected)
	}
}

// P11-ai visual evidence: the choice before and after the walk, with the
// pointer left where it was. Opt-in (`FLOTER_PNG=1`) like the other dumps, so a
// test run does not litter /tmp.
func TestDumpP11AIIdlePointerPNGs(t *testing.T) {
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
		a.SetApps(walkApps(12))
		tt := render(t, a)
		tt.SetDark(dark)
		px, py := float32(0), float32(0)
		dump := func(tag string) {
			tt.Frame()
			path := filepath.Join("/tmp", "floter-P11-ai-idle-"+tag+"-"+appearance+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			under := "nothing"
			if a.hoveredRow >= 0 && a.hoveredRow < len(a.Results()) {
				under = a.Results()[a.hoveredRow].Title
			}
			at := "out of frame"
			if img := tt.Image(); img != nil {
				if c := img.At(int(px), int(py)); c != nil {
					r, g, b, _ := c.RGBA()
					at = fmt.Sprintf("(%d,%d,%d)", r>>8, g>>8, b>>8)
				}
			}
			chosen := "--"
			if a.Selected >= 0 && a.Selected < len(a.Results()) {
				chosen = a.Results()[a.Selected].Title
			}
			t.Logf("%s  %s: chosen=%q under-the-pointer=%q pixel at the pointer (%.0f,%.0f)=%s",
				path, tag, chosen, under, px, py, at)
		}
		tt.Type("walk")
		tt.Frame()
		px, py = rowByName(t, tt, "Walk 03")
		tt.Move(px, py)
		tt.Frame()
		dump("entered")
		for i := 0; i < 8; i++ {
			tt.Key(0, ui.KeyDown)
		}
		dump("walked")
	}
}
