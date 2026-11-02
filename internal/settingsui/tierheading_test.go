package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The permission block's two tier labels are **section headings**, and a
// section heading on this page has one spelling: `Caption + 620 +
// TextSecondary` (five call sites: `rows.go`'s `rowTitle` and the four
// sub-headings in `settingsui.go`). The permission block's labels were the
// sixth and the only one spelled `Caption + 650 + TextStrong`, which made a
// *sub*-group inside a card heavier than the card's own title above it
// (`Title + 620 + TextStrong`) — the hierarchy was inverted: the child shouted
// louder than the parent.
//
// P13-eh 重定基线：`650 / TextStrong` → `620 / TextSecondary`。父子关系现在由
// **字号 + 墨**两级承担（Title 13.5 > Caption 9.9；TextStrong > TextSecondary），
// 字重不再逆序。
//
// The pins measure **contrast against the label's own ground**, not an absolute
// luma: the tier block sits on a card face and the reference line sits on the
// canvas, so the same ink reads at two absolute levels. Contrast is what the
// eye resolves and what the ink tokens promise.

const tierProbeLabel = "Floter enforces"

// tierInk draws one frame and returns the contrast (darkest pixel against the
// frame's own plane), the height of the first ink band and the label box's top.
func tierInk(t *testing.T, dark bool, build func(c *ui.Context, tk theme.Tokens)) (contrast, bandH, mass int) {
	t.Helper()
	a := New(newStore(t), Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		tk := a.tokens(c)
		c.SetTheme(tk.Theme)
		ui.Column(c).Fill().Padding(tk.Theme.Space(4)).Children(func() {
			build(c, tk)
		})
	}, 460, 200)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	plane := luma(img.Pix[0], img.Pix[1], img.Pix[2])
	first, last := -1, -1
	for y := 0; y < img.Rect.Dy(); y++ {
		row := 0
		for x := 0; x < img.Rect.Dx(); x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			if d := l - plane; d >= 20 || d <= -20 {
				row++
			}
		}
		if row > 0 {
			if first < 0 {
				first = y
			}
			last = y
		} else if first >= 0 {
			// The first gap ends the first band: the label's own line. The
			// card below the label is a face, not a line of ink.
			break
		}
	}
	if first < 0 {
		t.Fatalf("no ink in the frame (dark=%v)", dark)
	}
	for y := first; y <= last; y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			d := l - plane
			if d < 0 {
				d = -d
			}
			if d > contrast {
				contrast = d
			}
			mass += d
		}
	}
	return contrast, last - first + 1, mass
}

// TestThePermissionTierLabelWearsTheSectionHeadingInk is the pin: the tier
// label must read at exactly the contrast of a section heading (same size,
// same weight, same ink) and strictly lighter than the same sentence in the
// row-label ink it used to wear.
func TestThePermissionTierLabelWearsTheSectionHeadingInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		heading := func(c *ui.Context, tk theme.Tokens) {
			ui.Text(c, tierProbeLabel).FontSize(tk.Caption).FontWeight(620).
				TextColor(tk.TextSecondary).SingleLine()
		}
		strong := func(c *ui.Context, tk theme.Tokens) {
			ui.Text(c, tierProbeLabel).FontSize(tk.Caption).FontWeight(650).
				TextColor(tk.TextStrong).SingleLine()
		}
		tier := func(c *ui.Context, tk theme.Tokens) {
			a := New(newStore(t), Actions{})
			a.permissionTier(c, tierProbeLabel, "", true, "en", []string{"clipboard-write"})
		}
		wantContrast, wantH, wantMass := tierInk(t, dark, heading)
		gotContrast, gotH, gotMass := tierInk(t, dark, tier)
		oldContrast, _, _ := tierInk(t, dark, strong)

		if d := gotContrast - wantContrast; d > 4 || d < -4 {
			t.Errorf("%s: the tier label's contrast is %d, want the section heading's %d (±4)",
				appearance, gotContrast, wantContrast)
		}
		if oldContrast-gotContrast < 20 {
			t.Errorf("%s: the tier label's contrast is %d against the old row-label ink's %d: it is still wearing the stronger ink",
				appearance, gotContrast, oldContrast)
		}
		// The darkest pixel does not measure weight (§6.51 判据 2): 400 and 580
		// share it. The weight is the ink *mass* on the line.
		if d := gotMass - wantMass; d*100 > wantMass*6 || d*100 < -wantMass*6 {
			t.Errorf("%s: the tier label's ink mass is %d, want the section heading's %d (±6%%: same weight)",
				appearance, gotMass, wantMass)
		}
		if gotH != wantH {
			t.Errorf("%s: the tier label's line box is %d tall, want the section heading's %d (no geometry change)",
				appearance, gotH, wantH)
		}
	}
}

// TestDumpP13EHTierHeadings writes the two spellings side by side so the
// change can be looked at, not only measured: opt-in with FLOTER_PNG=1.
func TestDumpP13EHTierHeadings(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: set FLOTER_PNG=1 to dump the tier headings")
	}
	a := New(newStore(t), Actions{})
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, arm := range []struct {
			name string
			ink  bool
		}{{"before", true}, {"after", false}} {
			arm := arm
			tt := ui.NewTester(func(c *ui.Context) {
				tk := a.tokens(c)
				c.SetTheme(tk.Theme)
				ui.Column(c).Fill().Padding(tk.Theme.Space(4)).Gap(tk.Theme.Space(3)).Children(func() {
					ui.Text(c, "Section heading (620 / TextSecondary)").FontSize(tk.Caption).
						FontWeight(620).TextColor(tk.TextSecondary).SingleLine()
					if arm.ink {
						ui.Text(c, tierProbeLabel).FontSize(tk.Caption).FontWeight(650).
							TextColor(tk.TextStrong).SingleLine()
					} else {
						a.permissionTier(c, tierProbeLabel, "", true, "en", []string{"clipboard-write"})
					}
				})
			}, 460, 120)
			tt.SetDark(dark)
			tt.Frame()
			path := "/tmp/floter-P13-eh-tier-" + appearance + "-" + arm.name + ".png"
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatalf("%v", err)
			}
			f.Close()
			t.Log(path)
		}
	}
}
