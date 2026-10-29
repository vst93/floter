package launcher

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/valuedomain"
)

// P11-bw · **四个界面档位上，行盒是它自己的 40u（或两行字的盒），报出的窗口装得下十行。**
//
// The interface step (`settings.UIScale` → `theme.ForPreferences`) is the one
// setting that changes *every* number the launcher draws with, and until this
// pin no launcher test ever drew with it: every frame in this package is laid
// out on the framework's stock theme (`Spacing = 4`, `FontSize = 12`), because
// a tester only gets the app's theme when the view installs it, and the
// launcher's view — unlike the settings pages' — reads its geometry from
// `c.Theme()` while the shell is the one that installs `theme.ForPreferences`
// on the real window. So the audit rounds that ran "at four interface steps"
// (P11-bv §6 ①) drew the same frame four times, and a font scaled by the step
// *twice* stayed green.
//
// The two facts pinned here are the two the drawn frame and the charged window
// have to agree on at every step:
//
//   - a result row's box is `rowUnits` (40u) of the theme's own spacing, or the
//     two line boxes it holds when those are taller — the rule
//     `Geometry.Height` charges and `rowBody` draws (`launcher.go`'s `floor`,
//     `height.go`'s "a row is its content or its own box, whichever is taller");
//   - the height the launcher asks the shell for (`Actions.ResizeTo`, fed by
//     `Geometry.Height`) holds every row it draws — the window tracks the
//     content exactly, so the ten-row budget has to fit in the ten-row window.
//
// Both are measured at the *shipped* step, on the app's own theme, and both are
// expressed as equalities against the frame's own numbers (the spacing the
// context carries, the line boxes the app measured) rather than as literals at
// one step, so a future round may re-lay the rows and both sides move together.
type stepFrame struct {
	step     string
	spacing  float32 // the theme's `--u` unit (four of them are one `Space(1)`)
	font     float32 // the theme's body size
	title    float32 // the row title's line box, as the app measured it
	subtitle float32 // the row subtitle's line box
	pitch    float32 // the drawn distance from one row's title to the next
	charged  int     // the window height the launcher asked the shell for
	drawn    int     // rows of the ten-row page whose title box is wholly inside that window
}

// stepApp is a launcher at one interface step with a full ten-row page of
// recents. Each frame needs its own App: one App handed to a second
// `ui.Tester` panics in `FocusSearch` (a handle bound to two windows).
func stepApp(t *testing.T, step string) *App {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
		t.Fatal(err)
	}
	recentPage(a, maxResultSlots)
	return a
}

// measureStep reads one interface step off two frames: a tall one, where the
// launcher states the window it wants, and the shipped one, drawn at exactly
// that height, where the rows and the spacing are read.
func measureStep(t *testing.T, step string) stepFrame {
	t.Helper()
	tall := stepApp(t, step)
	charged := 0
	tall.Actions.ResizeTo = func(h int) { charged = h }
	ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tall.tokens(c).Theme)
		tall.View(c)
	}, InputWindowWidth, 1400).Frame()
	if charged <= 0 {
		t.Fatalf("[%s] the launcher never stated a window height", step)
	}

	a := stepApp(t, step)
	f := stepFrame{step: step, charged: charged, title: float32(tall.TitleLine), subtitle: float32(tall.SubtitleLine)}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		f.spacing, f.font = c.Theme().Spacing, c.Theme().FontSize
		a.View(c)
	}, InputWindowWidth, charged)
	tt.Frame()

	first, ok := tt.Find("Recent 00")
	if !ok {
		t.Fatalf("[%s] the first row is not drawn: %q", step, tt.Texts())
	}
	second, ok := tt.Find("Recent 01")
	if !ok {
		t.Fatalf("[%s] the second row is not drawn: %q", step, tt.Texts())
	}
	f.pitch = second.Y - first.Y

	img := tt.Image()
	if img == nil {
		t.Fatalf("[%s] no frame", step)
	}
	height := float32(img.Rect.Dy())
	for i := 0; i < maxResultSlots; i++ {
		label := fmt.Sprintf("Recent %02d", i)
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("[%s] %q is not drawn: the charged window holds %d rows, not the budget's %d (%q)",
				step, label, f.drawn, maxResultSlots, tt.Texts())
		}
		if r.Y >= 0 && r.Y+r.H <= height {
			f.drawn++
		}
	}
	return f
}

func stepAbs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestTheRowBoxIsTheSameUnitsAtEveryInterfaceStep is the first pin in this
// package that draws at an interface step other than the framework's stock one.
//
// Measured (Linux, 720 wide, ten recents, P11-bw):
//
//	step     spacing  font   pitch  40u  title+subtitle
//	tiny     3.2      9.6    33     32   33
//	small    3.6      10.8   36     36   36
//	default  4.0      12.0   40     40   40
//	large    4.4      13.2   44     44   43
//
// The two branches of "its own box or its content" are both real: at `tiny`
// the line boxes are taller than 40u (33 against 32), at `large` the box is
// (44 against 43). A row that scales the step twice — `Space(rowUnits) * scale`
// — leaves `large` at 48.4 and pushes the pitch/type ratio 9% off the other
// three steps; a row pinned to a literal 40px holds `tiny` at 40 and `large` at
// 40. Both are red here.
func TestTheRowBoxIsTheSameUnitsAtEveryInterfaceStep(t *testing.T) {
	frames := make([]stepFrame, 0, len(valuedomain.Scales))
	for _, step := range valuedomain.Scales {
		frames = append(frames, measureStep(t, step))
	}

	for _, f := range frames {
		want := float32(rowUnits) * f.spacing
		rule := "its own 40u"
		if content := f.title + f.subtitle; content > want {
			want, rule = content, "its two line boxes"
		}
		if stepAbs(f.pitch-want) > 1 {
			t.Errorf("[%s] the rows are %.1fpx apart, want %s: %.1fpx = %d × the theme's %.3f spacing",
				f.step, f.pitch, rule, want, rowUnits, f.spacing)
		}
	}

	// The type and the box travel together: the pitch/type ratio is one number
	// at every step (the tiny step's 3% is the row's line boxes rasterizing to
	// whole pixels — 18/9.6 against 22/12 — not a different ratio).
	low, high := frames[0].pitch/frames[0].font, frames[0].pitch/frames[0].font
	for _, f := range frames[1:] {
		ratio := f.pitch / f.font
		low, high = float32(math.Min(float64(low), float64(ratio))), float32(math.Max(float64(high), float64(ratio)))
	}
	if (high-low)/high > 0.05 {
		var ratios []string
		for _, f := range frames {
			ratios = append(ratios, fmt.Sprintf("%s %.3f (%.1fpx rows on %.1fpx type)", f.step, f.pitch/f.font, f.pitch, f.font))
		}
		t.Errorf("the row box and the type do not scale together: %v", ratios)
	}

	// And it does follow the step: a row that ignores it keeps one pitch.
	for i := 1; i < len(frames); i++ {
		if frames[i].pitch <= frames[i-1].pitch {
			t.Errorf("%s and %s draw their rows %.1fpx and %.1fpx apart: a bigger interface size is a bigger row",
				frames[i-1].step, frames[i].step, frames[i-1].pitch, frames[i].pitch)
		}
		if frames[i].charged <= frames[i-1].charged {
			t.Errorf("%s and %s ask for %dpx and %dpx of window: a bigger interface size is a taller window",
				frames[i-1].step, frames[i].step, frames[i-1].charged, frames[i].charged)
		}
	}
}

// TestTheChargedWindowHoldsTheTenRowsAtEveryInterfaceStep reads the other end of
// the same chain: the height the launcher hands `Actions.ResizeTo` is the
// height the shell gives the window, and the window tracks the content exactly
// (no one-row margin — R43/R66), so the ten-row budget has to fit in the
// ten-row window at every step.
//
// The frame is drawn at exactly the charged height, with that step's own theme,
// so a window the launcher under-charges stops laying out the ten rows it drew:
// the row that no longer fits is gone from the frame, not merely clipped by a
// test that never looked. That is the arm this pin's two mutations (a row box
// scaled twice, a row box pinned to 40px) turn red.
//
// Its measured reach, so nobody over-reads it: the framework clamps a clipped
// run to its clip rect, so a *small* shortfall — dropping the 4u breath under
// the field still leaves all ten rows *drawn*, with the tenth one's text sitting
// 16px below the window — does not fail this pin (measured, P11-bw). What it
// catches is a window that loses a row, which is the shape the height chain
// actually broke in before (the action bar, the eleventh unpaid band).
//
// And what it deliberately does not assert: the tenth row's *plate*. Measured
// at all four steps, that plate hangs 14px past the charged window at `default`
// (y=448..488 in a 474px window) because a section heading is charged inside
// the ten-row budget and so buys one of the ten result rows. Growing the window
// by the heading's band is a visible change to a behaviour the codebase already
// treats as a design decision (P11-bb's `last` ruling describes the same
// half-visible tenth row), so it belongs in `.pending-bugs.md`'s decision
// table, not in this round.
func TestTheChargedWindowHoldsTheTenRowsAtEveryInterfaceStep(t *testing.T) {
	for _, step := range valuedomain.Scales {
		f := measureStep(t, step)
		if f.drawn != maxResultSlots {
			t.Errorf("[%s] the window the launcher asked for (%dpx) holds %d of the budget's %d rows",
				step, f.charged, f.drawn, maxResultSlots)
		}
	}
}
