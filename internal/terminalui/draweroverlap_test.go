package terminalui

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// The appearance strip's rows share a wrapping grid whose cells are sized from
// a floor (`drawerCellFloor`, the reference's `minmax(230px, 1fr)`), and a cell
// hands each row **no more than the floor it asked for**. A trailing control
// that is wider than what is left of its cell therefore paints past the cell's
// right edge, into the cell next to it — the two rows' words land on the same
// pixels (measured before P11-as at 900×600: the cursor-shape picker's
// `Underline` and the next row's `Blinking cursor` overlapped 4.7×22u, and the
// palette's preview box 24×22u with `Copy on select`).
//
// So: every word a row draws must stay inside the cell the row was given, which
// is the same invariant P11-aj holds the launcher card to (`text over text`),
// applied to the strip. The audit walks the drawer's own band — the rows under
// the strip's top edge — and leaves the bar's toast above it alone: that toast
// is an opaque floating face painted last, on purpose, over the bar's title.
func TestTheDrawerRowsKeepTheirWordsInsideTheirCells(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, width := range []int{720, 900, 1100} {
			a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
			withSession(a, &terminal.Terminal{})
			a.Drawer = true
			tt := render(t, a, width, 600)
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			auditDrawerBand(t, appearance+" "+strconv.Itoa(width), tt)
		}
	}
}

// auditDrawerBand reports any two text runs of the drawer's band that paint over
// each other, and any run that leaves the window.
func auditDrawerBand(t *testing.T, tag string, tt *ui.Tester) {
	t.Helper()
	auditDrawerBandFrom(t, tag, tt, labelOfDrawer())
}

// auditDrawerBandFrom is auditDrawerBand with the strip's accessible name given
// by the caller, so a language whose label is not the English one can be
// audited by the same rule.
func auditDrawerBandFrom(t *testing.T, tag string, tt *ui.Tester, label string) {
	t.Helper()
	img := tt.Image()
	if img == nil {
		t.Fatalf("%s: no frame", tag)
	}
	w, h := float32(img.Rect.Dx()), float32(img.Rect.Dy())
	stripTop, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%s: the drawer's strip is not on the frame: %q", tag, tt.Texts())
	}
	type run struct {
		s string
		r ui.Rect
	}
	seen := map[string]bool{}
	var runs []run
	for _, s := range tt.Texts() {
		if seen[s] {
			continue
		}
		seen[s] = true
		r, found := tt.Find(s)
		if !found || r.W <= 0 || r.H <= 0 {
			continue
		}
		if r.W >= w-1 { // the strip's own label, the panel's, a whole row's box
			continue
		}
		if r.Y+r.H <= stripTop.Y {
			continue // the bar and its toast, above the strip
		}
		runs = append(runs, run{s, r})
	}
	for _, run := range runs {
		if run.r.X < -0.5 || run.r.Y < -0.5 || run.r.X+run.r.W > w+0.5 || run.r.Y+run.r.H > h+0.5 {
			t.Errorf("%s: %q leaves the window: %+v (window %.0fx%.0f)", tag, run.s, run.r, w, h)
		}
	}
	for i := 0; i < len(runs); i++ {
		for j := i + 1; j < len(runs); j++ {
			a, b := runs[i], runs[j]
			if encloses(a.r, b.r) || encloses(b.r, a.r) {
				continue
			}
			ox := minOf(a.r.X+a.r.W, b.r.X+b.r.W) - maxOf(a.r.X, b.r.X)
			oy := minOf(a.r.Y+a.r.H, b.r.Y+b.r.H) - maxOf(a.r.Y, b.r.Y)
			if ox > 1 && oy > 1 {
				t.Errorf("%s: %q %+v paints over %q %+v (overlap %.1fx%.1f)", tag, a.s, a.r, b.s, b.r, ox, oy)
			}
		}
	}
}

// labelOfDrawer is the strip's own accessible name.
func labelOfDrawer() string { return "Terminal appearance" }

func encloses(outer, inner ui.Rect) bool {
	return outer.X <= inner.X+0.5 && outer.Y <= inner.Y+0.5 &&
		outer.X+outer.W >= inner.X+inner.W-0.5 && outer.Y+outer.H >= inner.Y+inner.H-0.5
}

func minOf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxOf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// TestDumpDrawerPNGs is the round's visual evidence: the strip in both
// appearances with a session behind it, written to /tmp. Opt-in, as the other
// dumps are.
func TestDumpDrawerPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		withTitle(a, "~/workspace/floter")
		a.Drawer = true
		tt := render(t, a, 900, 600)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		img := tt.Image()
		if img == nil {
			t.Fatalf("%s: no frame", appearance)
		}
		path := filepath.Join("/tmp", "floter-P11-as-drawer-"+appearance+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
