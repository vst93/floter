package launcher

import (
	"image/color"
	"math"
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
			ruler := newAccentRuler(tokens.Theme.Accent, rest)
			if ruler.norm == 0 {
				t.Fatalf("the accent token %v is neutral: the ruler cannot answer", tokens.Theme.Accent)
			}
			accent := 0
			for y := top; y <= bottom; y++ {
				for xx := 8; xx < InputWindowWidth-8; xx++ {
					if ruler.carries(img.RGBAAt(xx, y)) {
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

// accentRuler answers 「这个像素带不带 accent」 for whichever hue the accent
// token currently is.
//
// P11-am: the retired ruler was the literal red one — `R−G > 24 && R−B > 24 &&
// |G−B| ≤ 8`, correct while the accent was `#E5484D`. P11-al put the token back
// on the reference's blue (`#064AA6` / `#8FB7FF`, whose `R−B` is −160 / −112), so
// every accent pixel read as "not the accent" and the two pins that used this
// rule could no longer fail. Freezing the hue into the probe is the defect; the
// token is the only source for which hue the accent is.
//
// The ruler compares **hue directions**, not a fixed channel: an accent blended
// into a neutral surface (a quiet tint, a keyline, an antialiased glyph) has the
// accent's chroma vector `(R−B, G−B)` scaled by the blend's alpha, so the
// accent is a ray through the greys rather than one colour. It also separates
// the accent from the warm surfaces that are legitimate colours of their own:
// `Warning`'s near-neutral red and the system-icon amber sit off that ray.
// The ruler is measured **against a neutral surface**, not in absolute colour:
// this design's greys are a level or two of blue themselves (`#F7F7F8`'s own
// chroma is `(−1, −1)`), so "how blue is this pixel" cannot tell a quiet accent
// tint from a cool grey. What a tint *adds* to a surface is the accent's chroma
// times its alpha, and that addition is what the ruler projects.
type accentRuler struct {
	// ax, ay is the accent's own chroma: the direction a blend of it adds.
	ax, ay int
	// bx, by is the neutral surface the addition is measured against.
	bx, by int
	// norm is the accent chroma's squared length.
	norm int
}

// newAccentRuler builds the ruler from the accent token and the surface the
// accent is looked for on. The accent is a token (`ui.Color`); everything the
// ruler reads from a frame is a `color.RGBA`.
func newAccentRuler(accent ui.Color, base color.RGBA) accentRuler {
	ax, ay := int(accent.R)-int(accent.B), int(accent.G)-int(accent.B)
	bx, by := chroma(base)
	return accentRuler{ax: ax, ay: ay, bx: bx, by: by, norm: ax*ax + ay*ay}
}

// chroma is a colour's hue with its lightness taken out: `(R−B, G−B)`.
func chroma(c color.RGBA) (x, y int) {
	return int(c.R) - int(c.B), int(c.G) - int(c.B)
}

// measure reports how much of the accent a pixel has picked up (1 for the accent
// itself, 0.065 for `AccentTintQuiet` over the base, ~0 for a grey) and how many
// degrees its *added* hue sits off the accent's.
func (r accentRuler) measure(c color.RGBA) (share, degrees float64) {
	if r.norm == 0 {
		return 0, 0
	}
	cx, cy := chroma(c)
	cx, cy = cx-r.bx, cy-r.by
	if cx == 0 && cy == 0 {
		return 0, 0
	}
	dot := float64(cx*r.ax + cy*r.ay)
	share = dot / float64(r.norm)
	cosine := dot / (math.Hypot(float64(cx), float64(cy)) * math.Sqrt(float64(r.norm)))
	if cosine > 1 {
		cosine = 1
	} else if cosine < -1 {
		cosine = -1
	}
	return share, math.Acos(cosine) * 180 / math.Pi
}

// carries reports whether the pixel shows the accent at all: at least 5% of the
// accent's chroma (one rung under `AccentTintQuiet`'s 0.065) and within 12° of
// its hue (an 8-bit blend of the accent measures 1–6°; the design's cool greys
// measure ~15° and their projection is ~4.5%, under the share floor as well).
func (r accentRuler) carries(c color.RGBA) bool {
	share, degrees := r.measure(c)
	return share >= 0.05 && degrees <= 12
}
