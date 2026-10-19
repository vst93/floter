package shell

import (
	"runtime"
	"testing"

	"floter/internal/settings"
)

// The card takes the whole window where the window cannot show a material
// behind it: the band the design leaves around the card is transparent glass,
// and a platform that draws its own frame color there turns that band into a
// ring of background (「窗口周边外圈还有一圈额外的底色」) and, on a wider
// client than the 720 contract, into the two blank blocks the user reads as
// 「搜索页面左右两边各有一大块空白」. The insets are the whole of that band,
// so zeroing them is the fix; the card's radius and the face's alpha follow
// (see View).
func TestTheCardFillsTheWindowWhereNoMaterialShows(t *testing.T) {
	a := newChromeTestApp(t)
	top, right, bottom, left := a.insetFor(false)
	if top != 0 || right != 0 || bottom != 0 || left != 0 {
		t.Fatalf("no material: insets = %v,%v,%v,%v, want zero", top, right, bottom, left)
	}
	// With a material the platform's own band stands, so the desktop shows
	// through the glass at the card's edges.
	gotTop, gotRight, gotBottom, gotLeft := a.insetFor(true)
	wantTop, wantRight, wantBottom, wantLeft := a.inset()
	if gotTop != wantTop || gotRight != wantRight || gotBottom != wantBottom || gotLeft != wantLeft {
		t.Fatalf("with material: insets = %v,%v,%v,%v, want %v,%v,%v,%v",
			gotTop, gotRight, gotBottom, gotLeft, wantTop, wantRight, wantBottom, wantLeft)
	}
}

// A window that shows no material is only possible on Windows (macOS and
// Linux composite the transparent margin themselves), so the fallback must not
// reach the other two platforms' frames — their cards keep the design's
// transparent band and rounded corners.
func TestOnlyAWindowsWindowCanLoseItsGlass(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fallback is the Windows path")
	}
	a := newChromeTestApp(t)
	if !a.windowShowsMaterial(nil) {
		t.Fatalf("%s: a headless frame reports no material", runtime.GOOS)
	}
}

// newChromeTestApp is an app with a store and no window: the headless shell the
// geometry questions are asked of.
func newChromeTestApp(t *testing.T) *App {
	t.Helper()
	return New(Options{Store: settings.NewStore(settings.Default())})
}

// The three platforms' bands, read on whatever host runs the suite.
//
// The band is the whole of the card's distance to the window edge, so a wrong
// number there *is* the 「窗口周边外圈还有一圈额外的底色」 ring and, on the
// horizontal pair, 「搜索页面左右两边各有一大块空白」 — and the port's own
// history has one such slip (10px written where the design draws 10u). The
// values are `base.css`'s platform rules (`:1503,1591-1607`): `.platform-linux
// .collapsed-shell { padding: calc(var(--u) * 10) }`, Windows' `4u 10u 12u 4u`
// for the launcher's shadow room and `10u` for the panels, macOS none. They are
// pinned per platform because only one of them is ever *drawn* by the host.
func TestThePlatformBandsAreTheReferenceUnits(t *testing.T) {
	cases := []struct {
		goos string
		surf Surface
		want [4]float32 // top, right, bottom, left
	}{
		{"linux", SurfaceLauncher, [4]float32{2.5, 2.5, 2.5, 2.5}},
		{"linux", SurfaceSettings, [4]float32{2.5, 2.5, 2.5, 2.5}},
		{"linux", SurfaceTerminal, [4]float32{2.5, 2.5, 2.5, 2.5}},
		{"windows", SurfaceLauncher, [4]float32{1, 2.5, 3, 1}},
		{"windows", SurfaceSettings, [4]float32{2.5, 2.5, 2.5, 2.5}},
		{"windows", SurfaceTerminal, [4]float32{2.5, 2.5, 2.5, 2.5}},
		{"darwin", SurfaceLauncher, [4]float32{0, 0, 0, 0}},
		{"darwin", SurfaceSettings, [4]float32{0, 0, 0, 0}},
		{"darwin", SurfaceTerminal, [4]float32{0, 0, 0, 0}},
	}
	for _, c := range cases {
		top, right, bottom, left := insetForOS(c.goos, c.surf)
		got := [4]float32{top, right, bottom, left}
		if got != c.want {
			t.Errorf("%s/%v: band = %v, want %v (units, `--u` × 4 = one Space step)",
				c.goos, c.surf, got, c.want)
		}
	}
}

// The drawn card is the window less that band, on every platform: the window's
// own width and the card's are one number, which is what keeps the two bands
// beside the search page from existing at all.
func TestTheCardIsTheWindowLessTheBand(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		_, right, _, left := insetForOS(goos, SurfaceLauncher)
		if got, want := CardWidth(SurfaceLauncher, 720, left, right), 720-left-right; got != want {
			t.Errorf("%s: card width = %v, want the window's 720 less the band %v", goos, got, want)
		}
	}
}
