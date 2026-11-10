package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-ew · 一个滑块行里的三对，以及它下面那行读数贴在卡沿上
// （`docs/liquid-glass.md` §6.118）。
//
// 滑块行（`settingsui.go` 的 `a.slider`）此前对五件东西之间只问一个数：
// `Gap(t.Space(1))`。于是「标题 → 提示」「提示 → 轨道」「轨道 → 它两端的轴词」
// 「轴词 → 禁用原因」读同一条缝——其中**轴词离自己的轨道（13/14px）比离下面那句
// 原因（10px）还远**，两个词成了下一段的开头，而不是轨道的两端。同一个页面上，
// trailing 行与 `stackedRow`（§6.117）早已按对分缝。
//
// 这两条 pin 量的是**读出来的**距离（`inkBands`），不是请求的距离。
//
// P13-ew 重定基线：内列 `Gap(Space(1))` 3.6 → 标题/提示与轨道/轴词各自
// `rowLineGap()`（1px）；外列 `Space(1)` → `Space(2)`（7.2px）。
// 合成卡实测：标题→提示 8 → **6**，提示→轨道 12 → **15**，轨道→轴词 13 → **11**，
// 轴词→原因 10 → **13**——轴词第一次成为这一行里离轨道最近的一件。

// sliderCardOptions says which of a slider row's five parts the probe draws.
type sliderCardOptions struct {
	hint   bool // the row's grey second line
	reason bool // the row's disabled statement
}

// sliderCardParts is what one probe frame resolved to.
type sliderCardParts struct {
	tt                        *ui.Tester
	face, cardTop, cardBottom int
	label, hint, axis         ui.Rect
	reason                    ui.Rect
}

// sliderCard renders one slider row
// on the settings page's own ground, inside a real card, and reads back the
// card's own top and bottom edges off the card's left margin column.
func sliderCard(t *testing.T, dark bool, opts sliderCardOptions) sliderCardParts {
	t.Helper()
	en := i18n.For("en").Settings
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	row := sliderRow{
		label: en.MainOpacity, low: en.TransparencyScaleLow, high: en.TransparencyScaleHigh,
		value: 47, lo: 0, hi: 100,
		format: func(v float64) string { return en.Percent(uint8(v)) },
		apply:  func(float64) {},
	}
	if opts.hint {
		row.description = en.TransparencyHint
	}
	if opts.reason {
		row.disabled = en.TransparencyUnavailable
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(560).Children(func() {
			a.card(c, func() {
				a.slider(c, row)
			})
		})
	}, 640, 420)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	out := sliderCardParts{tt: tt}
	out.label = mustFind(t, tt, en.MainOpacity)
	out.axis = mustFind(t, tt, en.TransparencyScaleLow)
	if opts.hint {
		out.hint = mustFind(t, tt, en.TransparencyHint)
	}
	if opts.reason {
		out.reason = mustFind(t, tt, en.TransparencyUnavailable)
	}
	// The card's own face, read on the card's left margin (between its edge and
	// the row's inset) — never the ground behind it.
	out.face = pageLuma(tt, out.label.X-4, out.label.Y+out.label.H/2)
	out.cardTop, out.cardBottom = -1, -1
	for y := 0; y < 420; y++ {
		if d := pageLuma(tt, out.label.X-4, float32(y)) - out.face; d >= -3 && d <= 3 {
			if out.cardTop < 0 {
				out.cardTop = y
			}
			out.cardBottom = y
		}
	}
	if out.cardTop < 0 {
		t.Fatalf("%s: the probe could not find the card's own face", appearanceOf(dark))
	}
	return out
}

func appearanceOf(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// sliderBands is the row's ink bands, read in one column window: the label's
// own x range, from above the title to below the last part drawn.
func sliderBands(p sliderCardParts, bottom float32) [][2]int {
	last := p.label.Y + p.label.H
	if p.axis.H > 0 {
		last = p.axis.Y + p.axis.H
	}
	if p.hint.H > 0 && p.hint.Y+p.hint.H > last {
		last = p.hint.Y + p.hint.H
	}
	if p.reason.H > 0 && p.reason.Y+p.reason.H > last {
		last = p.reason.Y + p.reason.H
	}
	return inkBands(p.tt, int(p.label.X), int(p.label.X+p.label.W), int(p.label.Y)-4, int(last)+4)
}

// bandAt is the ink band that starts inside box: the band a rect's own glyphs
// paint. It lets a probe name a part instead of counting bands from an end —
// a wrapped sentence is several bands, so an index from the end is not a part.
func bandAt(t *testing.T, bands [][2]int, box ui.Rect, what string) int {
	t.Helper()
	for i, b := range bands {
		if float32(b[0]) >= box.Y-1 && float32(b[0]) <= box.Y+box.H {
			return i
		}
	}
	t.Fatalf("no ink band starts inside the %s's box %v (bands %v)", what, box, bands)
	return -1
}

// TestASlidersAxisWordsHugTheirTrack is the pair's own half. The two words that
// name a track's ends are the track's own scale — one object with it — so they
// must read closer to the track than to anything else in the row: closer than
// the hint above the track is, and closer than the row's own statement below
// them is.
func TestASlidersAxisWordsHugTheirTrack(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := appearanceOf(dark)
		p := sliderCard(t, dark, sliderCardOptions{hint: true, reason: true})
		bands := sliderBands(p, 0)
		if len(bands) < 6 {
			t.Fatalf("%s: expected the row's five parts (six ink bands with a wrapped hint), got %v", appearance, bands)
		}
		axis := bandAt(t, bands, p.axis, "axis words")
		if axis < 2 || axis+1 >= len(bands) {
			t.Fatalf("%s: the axis words are not between the track and the row's statement (band %d of %v)",
				appearance, axis, bands)
		}
		trackAxis := inkGap(bands[axis-1], bands[axis])
		hintTrack := inkGap(bands[axis-2], bands[axis-1])
		axisReason := inkGap(bands[axis], bands[axis+1])
		if d := hintTrack - trackAxis; d < 2 {
			t.Errorf("%s: the axis words read %dpx under the track, the track %dpx under the hint (%+d) — "+
				"the words name the track's ends and must sit nearer to it than the hint above does",
				appearance, trackAxis, hintTrack, d)
		}
		if d := axisReason - trackAxis; d < 2 {
			t.Errorf("%s: the axis words read %dpx under their track and %dpx above the row's own statement (%+d) — "+
				"the words belong to the track, not to the sentence below them",
				appearance, trackAxis, axisReason, d)
		}
	}
}

// TestASlidersTitleAndHintShareTheRowsOwnSeam is the cross-family half: the
// title and its grey line are one sentence, and every other row on this page
// already draws that seam at `rowLineGap()` (§6.117). The slider must ask for
// the same number, so the pair is measured as a *box* gap — the two lines' own
// boxes, which do not move with the glyphs' descenders.
func TestASlidersTitleAndHintShareTheRowsOwnSeam(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		appearance := appearanceOf(dark)
		p := sliderCard(t, dark, sliderCardOptions{hint: true})
		got := p.hint.Y - (p.label.Y + p.label.H)
		// The real page's font-size row: the shipped row that carries a hint.
		store := settings.NewStore(settings.Default())
		a := New(store, Actions{})
		a.selectPage(PageGeneral)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 720, 2600)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		label, hint := mustFind(t, tt, en.FontSize), mustFind(t, tt, en.TerminalHint)
		real := hint.Y - (label.Y + label.H)
		want := a.rowLineGap()
		for _, probe := range []struct {
			where string
			got   float32
		}{{"synthetic card", got}, {"General page font-size row", real}} {
			if d := probe.got - want; d < -0.5 || d > 0.5 {
				t.Errorf("%s: %s draws title→hint at %.1fpx, the page's rows draw it at %.1fpx (rowLineGap) — "+
					"the same two lines of the same kind of row must not read two distances",
					appearance, probe.where, probe.got, want)
			}
		}
	}
}

// TestDumpP13EWSliderPNGs is the round's visual evidence (opt-in:
// `FLOTER_P13EW_DIR`). It writes the General page's transparency group per
// appearance. Run it once on the round's code (`FLOTER_P13EW_TAG=after`) and
// once with `a.slider`'s seams reverted (`FLOTER_P13EW_TAG=before`).
func TestDumpP13EWSliderPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13EW_DIR")
	if dir == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13EW_DIR=<dir>")
	}
	tag := os.Getenv("FLOTER_P13EW_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		appearance := appearanceOf(dark)
		store := settings.NewStore(settings.Default())
		a := New(store, Actions{})
		a.selectPage(PageGeneral)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 860, 1800)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		// The transparency group's card, plus the terminal-appearance sliders.
		crop := image.Rect(170, 990, 860, 1660)
		file, err := os.Create(filepath.Join(out, fmt.Sprintf("floter-P13-ew-slider-%s.png", appearance)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, img.SubImage(crop)); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
