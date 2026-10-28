package launcher

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P8-e §2 守卫 3（正面 + 反面）：捕获输出的列表**选中行**是中性面
// `tokens.RowSelected`，行内没有一个 accent 像素。
//
// §5 的禁止清单里「列表选中行」是明写的一条，而 `outputRow` 一直是最后一块
// 例外：它把选中行画成 `AccentTintQuiet`（一块带色相的洗）。P8-e 把它改到与
// 启动器列表（P8-a）同一档，于是同一个应用里「选中」只有一种面。
//
// 正面：选中行在空白的 x 处读到的像素 == `RowSelected` 合成在**这一行的底**
// 上的值（两套外观各自的 rung），并且它的三个通道差不超过 2 级（中性填充
// 不可能带色偏）。
// 反面：它既不等于 accent 的静默淡染（旧语言），行带内也数不出 accent 像素。
func TestTheOutputSelectedRowIsTheNeutralChosenRung(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = tc.name }); err != nil {
				t.Fatal(err)
			}
			tt := runListOutput(t, a, outputRowsWithActions(4))
			img := tt.Image()

			selected, ok := tt.Find("Row 0") // the empty list protocol's first row is chosen
			if !ok {
				t.Fatalf("the selected row was not drawn: %q", tt.Texts())
			}
			plain, ok := tt.Find("Row 2") // an unselected row, one band below
			if !ok {
				t.Fatalf("the third row was not drawn: %q", tt.Texts())
			}
			tokens := theme.ForPreferences(a.settings(), tc.dark, ui.Preferences{})

			// A blank column to the right of both titles: the row's own fill
			// with no glyph or text ink over it.
			const x = 600
			selY, plainY := int(selected.Y+selected.H/2), int(plain.Y+plain.H/2)
			sel := img.RGBAAt(x, selY)
			rest := img.RGBAAt(x, plainY)

			// The two appearances' rungs must differ (the contract splits them).
			if sel == rest {
				t.Fatalf("the chosen row's fill is the plain row's %v: no selection is drawn at all", sel)
			}
			if want := over(tokens.RowSelected, rest); !closeRGB(sel, want) {
				t.Errorf("the chosen row is %v, want %v (the neutral chosen-row rung %v over the row's own %v)",
					sel, want, tokens.RowSelected, rest)
			}
			// A neutral fill cannot shift one channel against another.
			spread := spreadOf(sel)
			if spread > 2 {
				t.Errorf("the chosen row is %v: its channels differ by %d levels, so the fill carries a hue "+
					"(the contract's chosen row is neutral)", sel, spread)
			}
			// The anti-regression half: the old accent wash sat here.
			if wash := over(tokens.AccentTintQuiet, rest); closeRGB(sel, wash) {
				t.Errorf("the chosen row is painted with the accent's quiet tint %v (%v); the contract's row fill is neutral",
					tokens.AccentTintQuiet, wash)
			}
			// …and no pixel of the chosen band is the accent: the row is one
			// neutral plane, glyphs and title included.
			top, bottom := selY, selY
			for y := selY; y >= 0 && img.RGBAAt(x, y) != rest; y-- {
				top = y
			}
			for y := selY; y < img.Bounds().Max.Y && img.RGBAAt(x, y) != rest; y++ {
				bottom = y
			}
			accent := 0
			for y := top; y <= bottom; y++ {
				for xx := 8; xx < InputWindowWidth-8; xx++ {
					if carriesAccent(img.RGBAAt(xx, y)) {
						accent++
					}
				}
			}
			if accent > 0 {
				t.Errorf("%d pixels of the chosen row carry the accent (%v); the row is neutral (§5 forbids the accent on a list's chosen row)",
					accent, tokens.Theme.Accent)
			}
		})
	}
}

// spreadOf is the largest per-channel difference a colour's own bytes show:
// the ruler a "neutral, no hue" claim is read against.
func spreadOf(c color.RGBA) int {
	spread := 0
	for _, d := range []int{
		int(c.R) - int(c.G), int(c.G) - int(c.B), int(c.B) - int(c.R),
	} {
		if d < 0 {
			d = -d
		}
		if d > spread {
			spread = d
		}
	}
	return spread
}

// carriesAccent reports whether a pixel is the accent's red: R well over both
// G and B while G and B stay together — the pair of facts that separates it
// from the warm system-icon amber the power rows' plates legitimately use
// (the same ruler `TestTheLauncherListCarriesNoAccent` reads).
func carriesAccent(c interface {
	RGBA() (uint32, uint32, uint32, uint32)
}) bool {
	r, g, b, _ := c.RGBA()
	ri, gi, bi := int(r>>8), int(g>>8), int(b>>8)
	return ri-gi > 24 && ri-bi > 24 && absInt(gi-bi) <= 8
}
