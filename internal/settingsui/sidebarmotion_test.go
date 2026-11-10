package settingsui

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

func TestSidebarSelectionMovesAndRetargetsWithoutDelayingPage(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		a := newTestApp()
		var ctx *ui.Context
		tt := ui.NewTester(func(c *ui.Context) { ctx = c; c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 580)
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduced})
		tt.Frame()
		start := a.sidebarMarker.Bounds(ctx)
		if start.H < 39 || start.W < 100 {
			t.Fatalf("missing marker: %+v", start)
		}
		about := mustFind(t, tt, "About")
		x, y := int(start.X+start.W-12), int(about.Y+about.H/2+10)
		pane := tt.Image().RGBAAt(x, y)
		tt.Move(about.X+about.W/2, about.Y+about.H/2)
		tt.Advance(theme.ColorStep)
		tt.Frame()
		if err := tt.Click("About"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.Page != PageAbout {
			t.Fatal("page waits for animation")
		}
		target := mustFind(t, tt, "About")
		tt.Advance(45 * time.Millisecond)
		tt.Frame() // Commit the exact clock time before comparing a retarget.
		mid := a.sidebarMarker.Bounds(ctx)
		endY := target.Y + target.H/2 - start.H/2
		if reduced {
			if math.Abs(float64(mid.Y-endY)) > 1 {
				t.Fatalf("reduced motion still moves: %+v -> %+v", mid, target)
			}
		} else if mid.Y <= start.Y || mid.Y >= endY {
			t.Fatalf("no intermediate movement: start=%+v mid=%+v target=%+v", start, mid, target)
		}
		if !reduced && tt.Image().RGBAAt(x, y) != pane {
			t.Fatal("selected row kept a second stationary hover background during movement")
		}
		// Reverse before it completes; navigation and hit regions stay in place.
		if err := tt.Click("General"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.Page != PageGeneral {
			t.Fatal("rapid reverse did not switch page")
		}
		if reverse := a.sidebarMarker.Bounds(ctx); !reduced && math.Abs(float64(reverse.Y-mid.Y)) > 1 {
			t.Fatalf("reverse jumped instead of retargeting: %+v -> %+v", mid, reverse)
		}
		tt.Advance(250 * time.Millisecond)
		if got := a.sidebarMarker.Bounds(ctx); math.Abs(float64(got.Y-start.Y)) > 1 {
			t.Fatalf("marker did not settle: %+v", got)
		}
		// Keyboard navigation must stay immediate.
		tt.Key(0, ui.KeyDown)
		tt.Frame()
		if a.Page != PageSessions {
			t.Fatalf("keyboard page=%d", a.Page)
		}
		got := a.sidebarMarker.Bounds(ctx)
		label := mustFind(t, tt, "Sessions")
		if math.Abs(float64(got.Y+got.H/2-label.Y-label.H/2)) > 1 {
			t.Fatalf("keyboard marker animated: %+v %+v", got, label)
		}
	}
}

func TestDumpP15SettingsInteractions(t *testing.T) {
	dir := os.Getenv("FLOTER_P15_DUMP")
	if dir == "" {
		t.Skip("optional visual evidence")
	}
	for _, dark := range []bool{false, true} {
		a := newTestApp()
		tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(a.tokens(c).Theme); a.View(c) }, 720, 580)
		tt.SetDark(dark)
		tt.Frame()
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		save := func(state string) {
			t.Helper()
			f, err := os.Create(filepath.Join(dir, "settings-"+appearance+"-"+state+".png"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
		}
		save("rest")
		language := mustFind(t, tt, "Language")
		tt.Move(language.X+5, language.Y+5)
		tt.Frame()
		save("row-hover")
		tt.Press(language.X+5, language.Y+5)
		tt.Frame()
		save("row-press")
		tt.CancelPress()
		light := mustFind(t, tt, "Light")
		tt.Move(light.X+light.W/2, light.Y+light.H/2)
		tt.Frame()
		save("segment-hover")
		b := mustFind(t, tt, "About")
		tt.Move(b.X+b.W/2, b.Y+b.H/2)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
		save("hover")
		if err := tt.Click("About"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		tt.Advance(45 * time.Millisecond)
		save("moving")
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
		save("selected")
	}
}
