package terminalui

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/frameaudit"
	"floter/internal/settings"
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
		for _, width := range []int{settings.MinTerminalWidth, 720, 900, 1100} {
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
//
// P11-by：规则本身在 `internal/frameaudit`（三处共用一份）。这里只留「这个表面
// 拥有哪一段」的过滤：条带以上的浮条与它的 toast 不归这把尺子管。**整宽箱子不再
// 跳过** —— 条带自己的 label 是一个整宽祖先，`Encloses` 本来就会豁免它。
func auditDrawerBandFrom(t *testing.T, tag string, tt *ui.Tester, label string) {
	t.Helper()
	stripTop, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%s: the drawer's strip is not on the frame: %q", tag, tt.Texts())
	}
	frameaudit.Audit(t, tag, tt, func(run frameaudit.Run) bool {
		return run.R.Y+run.R.H > stripTop.Y
	})
}

// labelOfDrawer is the strip's own accessible name.
func labelOfDrawer() string { return "Terminal appearance" }

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
