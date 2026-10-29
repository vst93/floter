package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
)

// P11-aj · **没有两段字压在同一块像素上，也没有一段字跑出窗口。**
//
// The launcher's frame is built from absolute bands (the field row floats over
// the list, the footer is pinned at the panel's bottom edge) and from text
// whose width is whatever the row had left. Both are the two ways a text
// surface goes wrong without any test noticing: a run that is one row too wide
// runs into the trailing keycap, and a band whose box the layout never asked
// for puts its words outside the window where nobody sees them. Neither is
// visible in the text tree — `Tester.Texts` reports every run the tree holds,
// on screen or not — so both are read here off the boxes the frame registered
// (`Tester.Find`), which is the geometry the framework hit-tests and paints.
//
// What counts as a violation:
//   - two runs whose boxes overlap by more than one DIP in both axes, unless
//     one box contains the other (a button's label inside the button is one
//     run on one face, not two words on top of each other), and
//   - a run whose box leaves the window it was painted into.
//
// The full-window boxes are skipped: the panel's own label spans the card
// (`ResultsLabel`), and so does the list's, so they contain every run by
// construction.
func auditLauncherFrame(t *testing.T, tag string, tt *ui.Tester) {
	t.Helper()
	img := tt.Image()
	if img == nil {
		t.Fatalf("%s: no frame", tag)
	}
	w, h := float32(img.Rect.Dx()), float32(img.Rect.Dy())
	seen := map[string]bool{}
	var boxes []struct {
		s string
		r ui.Rect
	}
	for _, s := range tt.Texts() {
		if seen[s] {
			continue
		}
		seen[s] = true
		r, ok := tt.Find(s)
		if !ok || r.W <= 0 || r.H <= 0 {
			continue
		}
		if r.W >= w-1 { // the panel's/list's own label, not a run
			continue
		}
		boxes = append(boxes, struct {
			s string
			r ui.Rect
		}{s, r})
	}
	for _, b := range boxes {
		if b.r.X < -0.5 || b.r.Y < -0.5 || b.r.X+b.r.W > w+0.5 || b.r.Y+b.r.H > h+0.5 {
			t.Errorf("%s: %q leaves the window: %+v (window %.0fx%.0f)", tag, b.s, b.r, w, h)
		}
	}
	for i := 0; i < len(boxes); i++ {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if encloses(a.r, b.r) || encloses(b.r, a.r) {
				continue
			}
			ox := minF(a.r.X+a.r.W, b.r.X+b.r.W) - maxF(a.r.X, b.r.X)
			oy := minF(a.r.Y+a.r.H, b.r.Y+b.r.H) - maxF(a.r.Y, b.r.Y)
			if ox > 1 && oy > 1 {
				t.Errorf("%s: %q %+v is painted over %q %+v (overlap %.1fx%.1f)", tag, a.s, a.r, b.s, b.r, ox, oy)
			}
		}
	}
}

func encloses(outer, inner ui.Rect) bool {
	return outer.X <= inner.X+0.5 && outer.Y <= inner.Y+0.5 &&
		outer.X+outer.W >= inner.X+inner.W-0.5 && outer.Y+outer.H >= inner.Y+inner.H-0.5
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// The three pages the launcher draws from the same card — the empty page with
// its recents, a query with matches and a trailing badge, and a query that
// matched nothing — in both appearances, and in both shipped languages. A row
// long enough to be ellipsized is in the fixture on purpose: that is where a
// text run and a trailing keycap meet.
//
// P11-ba: the audit used to run in English only. The user reads the Chinese
// surface, and the two languages put different-length words in the same boxes
// (a Chinese group label is four wide glyphs where the English one is two
// words), so the same rule has to hold on both. Measured clean on both before
// this change; the arms below keep it that way.
func TestTheLauncherPaintsNoTextOverText(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			t.Run(lang+" "+appearance, func(t *testing.T) {
				a := testApp()
				if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
					t.Fatal(err)
				}
				a.SetTools([]apps.App{
					{Name: "git", Path: "/usr/bin/git"},
					{Name: "Supercalifragilisticexpialidocious-Application-Launcher-With-A-Very-Long-Name", Path: "/usr/bin/long"},
				}, true)
				tt := render(t, a)
				tt.SetDark(dark)
				tt.Frame()
				auditLauncherFrame(t, appearance+" "+lang+" empty", tt)
				tt.Type("git")
				tt.Frame()
				auditLauncherFrame(t, appearance+" "+lang+" match", tt)
				tt.Type("zzzz")
				tt.Frame()
				auditLauncherFrame(t, appearance+" "+lang+" no results", tt)
			})
		}
	}
}
