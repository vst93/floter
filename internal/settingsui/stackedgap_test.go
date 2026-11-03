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

// P13-ev · 一个堆叠行的三对，此前只有一条缝（`docs/liquid-glass.md` §6.117）。
//
// `stackedRow`（设置页里控件要占整行的那种行：外观/语言/界面尺寸/材质强度、
// 以及插件浮层的下拉）此前对列里的**三对**都问一个数——`Gap(t.Space(1))`：
// 「标题 → 它的解释」与「解释 → 控件」读同一条缝。同一个页面上，trailing 行
// （控件在标题右边）的同一对是 1px（`rowLineGap()`）。
//
// 这两条 pin 量的是**读出来的**距离，不是请求的距离：`inkBands` 找的是墨带，
// 所以它同时守住两件事——标题和它的灰线是一句（同一条缝，与 trailing 行同值），
// 控件是一件可以按的面（比那句话更远一档）。
//
// P13-ev 重定基线：stacked 行的「标题 → 解释」盒缝 `Space(1)`（3.6px，墨上 10px）
// → `rowLineGap()`（1px，墨上 6px，与 trailing 行逐像素相同）；「解释 → 控件」
// `Space(1)` → `Space(2)`（墨上 9px）。

// stackedRowBands renders the General page and returns the ink bands of the two
// rows the pin compares: the stacked Language row (title, hint, control) and the
// trailing Launch-at-startup row (title, hint).
func stackedRowBands(t *testing.T, dark bool) (stacked, trailing [][2]int) {
	t.Helper()
	copy := i18n.For("en").Settings
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	a.selectPage(PageGeneral)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 860, 1200)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Advance(500 * time.Millisecond)
	tt.Frame()

	label := mustFind(t, tt, copy.Language)
	ctl := mustFind(t, tt, "English")
	stacked = inkBands(tt, int(label.X), int(label.X+label.W), int(label.Y)-4, int(ctl.Y+ctl.H)+4)

	tl := mustFind(t, tt, copy.LaunchAtStartup)
	th := mustFind(t, tt, copy.LaunchAtStartupHint)
	trailing = inkBands(tt, int(tl.X), int(tl.X+tl.W), int(tl.Y)-4, int(th.Y+th.H)+4)
	return stacked, trailing
}

// inkGap is the number of empty rows between two ink bands.
func inkGap(a, b [2]int) int { return b[0] - a[1] - 1 }

// TestAStackedRowSpacesItsTitleLikeAnyOtherRow is the cross-family half: the
// pair「标题 → 它的解释」must read the same distance whether the control sits
// under the row or beside it — they are the same two lines of the same kind of
// row, and only one of them used to ask a different number.
func TestAStackedRowSpacesItsTitleLikeAnyOtherRow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		stacked, trailing := stackedRowBands(t, dark)
		if len(stacked) < 3 || len(trailing) < 2 {
			t.Fatalf("%s: expected three ink bands on the stacked row and two on the trailing row, got %v and %v",
				appearance, stacked, trailing)
		}
		got, want := inkGap(stacked[0], stacked[1]), inkGap(trailing[0], trailing[1])
		if d := got - want; d < -1 || d > 1 {
			t.Errorf("%s: the stacked row's title→hint gap reads %dpx, the trailing row's reads %dpx (%+d) — "+
				"the same two lines of the same kind of row must not read two distances",
				appearance, got, want, d)
		}
		if want < 4 || want > 9 {
			t.Errorf("%s: the trailing row's title→hint gap reads %dpx, outside the pair's own step (4..9) — "+
				"the probe is not measuring the pair it thinks it is", appearance, want)
		}
	}
}

// TestAStackedRowsControlStandsApartFromItsSentence is the pair's own half: the
// control is a face that can be pressed, so it must not sit in the same seam as
// the title and its hint (§4.3: only the pair whose second half is a pressable
// face gets the full step).
func TestAStackedRowsControlStandsApartFromItsSentence(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		stacked, _ := stackedRowBands(t, dark)
		if len(stacked) < 3 {
			t.Fatalf("%s: expected three ink bands on the stacked row, got %v", appearance, stacked)
		}
		sentence, control := inkGap(stacked[0], stacked[1]), inkGap(stacked[1], stacked[2])
		if control-sentence < 2 {
			t.Errorf("%s: the control's ink sits %dpx under the hint and the hint %dpx under the title — "+
				"the pressable face reads as one more line of the sentence (want ≥2px more)",
				appearance, control, sentence)
		}
	}
}

// TestDumpP13EVStackedGapPNGs is the round's visual evidence (opt-in:
// `FLOTER_P13EV_DIR`). It writes the General page's first two cards per
// appearance. Run it once on the round's code (`FLOTER_P13EV_TAG=after`) and
// once with `stackedRow` reverted (`FLOTER_P13EV_TAG=before`).
func TestDumpP13EVStackedGapPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13EV_DIR")
	if dir == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13EV_DIR=<dir>")
	}
	tag := os.Getenv("FLOTER_P13EV_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		a := New(store, Actions{})
		a.selectPage(PageGeneral)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 860, 760)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		crop := image.Rect(170, 110, 860, 560)
		file, err := os.Create(filepath.Join(out, fmt.Sprintf("floter-P13-ev-stacked-%s.png", appearance)))
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
