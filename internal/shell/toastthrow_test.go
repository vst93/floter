package shell

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// toastArm is one of the two placements the stack has: the full-height
// surfaces hang the pane from the top right (`--elev-3`), the collapsed
// launcher pins it to the bottom-left 20u band (`--elev-3-compact`).
type toastArm struct {
	name     string
	collapse bool
	open     func(*App)
}

var toastArms = []toastArm{
	{"full", false, func(a *App) { a.Open(SurfaceSettings) }},
	{"collapsed", true, func(a *App) { a.Surf = SurfaceLauncher }},
}

// toastShot renders one settled toast on the given arm at the given frame
// transparency: the frame the pane appeared over, the frame it sits on, and the
// dismiss control's box (the pane's own geometry is read off it).
func toastShot(t *testing.T, arm toastArm, opacity uint8, dark bool) (before, after *image.RGBA, box ui.Rect) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.MainOpacity = opacity }); err != nil {
		t.Fatal(err)
	}
	a := New(Options{Store: store})
	now := time.Unix(0, 0)
	a.toastClock = func() time.Time { return now }
	arm.open(a)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.SetDark(dark)
	tt.Frame()
	// A copy, not the tester's buffer: the frame is reused by the next render,
	// so the comparison point has to be frozen out of it.
	before = shellFrameCopy(tt.Image())
	a.warnNotice("Where am I")
	tt.Frame()
	// The frame it joined the stack started its entrance; step past `--dur-4`
	// and let it land before measuring what it throws.
	now = now.Add(toastEnter)
	tt.Frame()
	dismiss, ok := tt.Find(i18n.For("en").Settings.DismissNotice)
	if !ok {
		t.Fatalf("%s: the toast has no dismiss control: %q", arm.name, tt.Texts())
	}
	return before, shellFrameCopy(tt.Image()), dismiss
}

// paneTop is the pane's first row, read off the dismiss control: 2u of padding
// and a 1px border on the full-height arm, neither on the collapsed strip (a
// fixed 5u box with centred children).
func paneTop(arm toastArm, dismiss ui.Rect) int {
	inset := 9
	if arm.collapse {
		inset = 1
	}
	return int(dismiss.Y) - inset
}

// paneBottom is the pane's last row: the same inset under the dismiss control.
func paneBottom(arm toastArm, dismiss ui.Rect) int {
	inset := 9
	if arm.collapse {
		inset = 1
	}
	return int(dismiss.Y+dismiss.H) + inset
}

// TestTheToastThrowFollowsTheFrameTransparency pins the arithmetic P11-an
// landed for the four older floating layers (`--elev-shadow-scale`) onto the
// fifth one, the toast. It used to borrow the window's own tokens
// (`ShadowAmbient` 0.62 / `ShadowContact` 0.44), so the transparency slider
// moved the fill and left the throw at the same weight: a nearly transparent
// frame wearing a black halo.
//
// The reading is relative — how much of the page's own light the throw takes
// away — because the two frames differ in tint as well as in shade: an opaque
// frame and a 10% one are not the same page, only the same geometry.
func TestTheToastThrowFollowsTheFrameTransparency(t *testing.T) {
	depth := func(opacity uint8) float32 {
		before, after, dismiss := toastShot(t, toastArms[0], opacity, false)
		bottom := paneBottom(toastArms[0], dismiss)
		x := int(dismiss.X + dismiss.W/2)
		deepest := float32(0)
		for y := bottom; y <= bottom+40 && y < after.Rect.Dy(); y++ {
			base := shellPixelAt(before, x, y)
			if base.R < 16 { // a black page tells no ratio
				continue
			}
			if got := (float32(base.R) - float32(shellPixelAt(after, x, y).R)) / float32(base.R); got > deepest {
				deepest = got
			}
		}
		return deepest
	}
	opaque, thin := depth(settings.MaxWindowOpacity), depth(settings.MinWindowOpacity)
	t.Logf("deepest relative darkening: %d%% = %.3f, %d%% = %.3f",
		settings.MaxWindowOpacity, opaque, settings.MinWindowOpacity, thin)
	if opaque <= 0 {
		t.Fatalf("the toast casts no shadow at an opaque frame")
	}
	if thin <= opaque {
		t.Errorf("the throw does not follow the frame: %.3f at %d%%, %.3f at %d%% "+
			"(`--elev-shadow-scale` is 0.45 at an opaque frame and 1.26 at the thinnest stop)",
			opaque, settings.MaxWindowOpacity, thin, settings.MinWindowOpacity)
	}
}

// TestBothToastArmsWearTheFloatingRim pins §2's judgement 3 / §3's table onto
// both arms: an E2 pane wears `EdgeHighlight` on its top row. The collapsed
// strip had no rim at all — the reference's compact derivation drops it, and
// this build inherited the 20u band where the line was never drawn.
func TestBothToastArmsWearTheFloatingRim(t *testing.T) {
	for _, arm := range toastArms {
		for _, appearance := range []struct {
			name string
			dark bool
		}{{"dark", true}, {"light", false}} {
			t.Run(arm.name+"/"+appearance.name, func(t *testing.T) {
				_, after, dismiss := toastShot(t, arm, settings.DefaultMainOpacity, appearance.dark)
				top := paneTop(arm, dismiss)
				x := int(dismiss.X + dismiss.W/2)
				rim, face := 0, luma(shellPixelAt(after, x, top+6))
				// The rim is drawn inside the 1px border, and the pane's own row
				// is fractional in the layout: scan the rows the line can land on.
				for y := top - 1; y <= top+4; y++ {
					if v := luma(shellPixelAt(after, x, y)); v > rim {
						rim = v
					}
				}
				t.Logf("%s/%s: rim %d vs face %d at x=%d top=%d", arm.name, appearance.name, rim, face, x, top)
				if rim <= face {
					t.Errorf("%s/%s: the pane's top row (%d) is not brighter than its own face (%d): "+
						"E2's rim is `EdgeHighlight`, one solid 1px line", arm.name, appearance.name, rim, face)
				}
			})
		}
	}
}

// TestTheLauncherToastKeepsItsContactLayer pins §2's judgement 3 on the one arm
// where it cannot be measured as a cast shadow: the 20u strip hangs 1.5u above
// the window's bottom edge, so the throw below it lands in the window's own
// shadow, where black stays black. What is left to read is how fast the added
// alpha falls away: the contact layer (`0 1px 2px`, the tight one) makes the
// first row under the pane about twice as heavy as the fifth, while the ambient
// alone — the reference's compact derivation — leaves a nearly flat run
// (measured 1.44× without it, 1.96× with it).
func TestTheLauncherToastKeepsItsContactLayer(t *testing.T) {
	arm := toastArms[1]
	before, after, dismiss := toastShot(t, arm, settings.DefaultMainOpacity, false)
	bottom := paneBottom(arm, dismiss)
	x := int(dismiss.X + dismiss.W/2)
	added := func(y int) int {
		return int(shellPixelAt(after, x, y).A) - int(shellPixelAt(before, x, y).A)
	}
	near, far := added(bottom+1), added(bottom+5)
	t.Logf("alpha added under the strip: +1 row = %d, +5 rows = %d", near, far)
	if near <= 0 || far <= 0 {
		t.Fatalf("the strip casts no shadow at all: %d then %d", near, far)
	}
	if float32(near) < 1.7*float32(far) {
		t.Errorf("the strip has no contact layer: the throw is %d deep one row under the pane "+
			"and %d deep five rows under it (a floating pane's tight layer makes the first row "+
			"about twice the fifth; ambient alone leaves a flat run)", near, far)
	}
}
