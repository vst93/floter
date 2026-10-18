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
