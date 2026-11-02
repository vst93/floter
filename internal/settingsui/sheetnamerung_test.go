package settingsui

import (
	"image/png"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"
)

// sheetLine is one line of ink inside the sheet: the row its ink starts on and
// the largest luma distance from the plane of its own rows.
type sheetLine struct {
	y, peak int
}

// sheetLines finds the lines of ink in one window of the sheet. Each row is
// measured against its own median luma, so a line on the sheet's sunken ground
// and one on a card face measure the same way; rows two or closer apart belong
// to one line (a descender hangs from the line it belongs to).
func sheetLines(tt *ui.Tester, x0, x1, y0, y1 int) []sheetLine {
	img := tt.Image()
	lines := make([]sheetLine, 0, 8)
	start, peak, blank := -1, 0, 0
	flush := func() {
		if start >= 0 {
			lines = append(lines, sheetLine{y: start, peak: peak})
		}
		start, peak = -1, 0
	}
	for y := y0; y < y1; y++ {
		lumas := make([]int, 0, x1-x0)
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			lumas = append(lumas, luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]))
		}
		sorted := append([]int(nil), lumas...)
		sort.Ints(sorted)
		plane := sorted[len(sorted)/2]
		worst := 0
		for _, l := range lumas {
			if d := l - plane; d > worst {
				worst = d
			} else if -d > worst {
				worst = -d
			}
		}
		if worst > 12 {
			if start < 0 {
				start = y
			}
			if worst > peak {
				peak = worst
			}
			blank = 0
			continue
		}
		if start >= 0 {
			blank++
			if blank > 2 {
				flush()
			}
		}
	}
	flush()
	return lines
}

// P13-ei：浮层的名字不再是自己这块面上最淡的一行。
//
// 改前（`docs/rounds/P13-ei-spec.md` 的实测表）：名字 128 / 分组标题 27 /
// help 57（浅色档最暗像素）。名字比它下面每一条 help 都淡，比它自己带的
// 分组标题淡 101 级，而 `pluginsheet.go` 的注释写着 name > section > field > help。
// 这一轮只换墨与字重（字号不动，见 spec 的 0.2px 顶边余量），所以 pin 也分成两半：
// 墨那半要求名字与分组标题**同一档**，几何那半要求行盒与滚动范围一个字节不动。
func TestThePluginSheetNameIsNotTheQuietestLineOnItsOwnFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			band := sheetBand(tt)
			lines := sheetLines(tt, band.Min.X, band.Max.X, band.Min.Y, band.Min.Y+120)
			if len(lines) < 2 {
				t.Fatalf("found %d ink lines in the sheet's first 120 rows (%v): the name and the "+
					"section heading are what this pin compares", len(lines), lines)
			}
			name, section := lines[0], lines[1]
			t.Logf("name y=%d peak=%d, section y=%d peak=%d, MaxY %v",
				name.y, name.peak, section.y, section.peak, a.PluginScroll.MaxY)
			// ① 墨：名字与它自己带的分组标题同一档（±4）。改前是 118 对 219。
			if diff := section.peak - name.peak; diff > 4 {
				t.Errorf("the sheet's name reads %d levels off its plane while the section heading under "+
					"it reads %d (a %d-level gap): the name is the quietest line on the face it names — "+
					"it must wear the header ink, not the muted one (want ≤4)", name.peak, section.peak, diff)
			}
			// ② 几何：名字的墨仍从视口顶第 11 行起（§6.72 的顶边高就是 11 行），
			// 行盒与滚动范围一个字节不动——只换墨不许改盒。
			if got := name.y - band.Min.Y; got != 11 {
				t.Errorf("the name's ink starts %d rows below the band's top, want 11: raising the ink "+
					"must not move the line (the sheet's top edge has 0.2px of room)", got)
			}
			// P13-ej 重定基线：旧值 **316.4** → 新值 **330.8**（§6.105 的分组间距
			// +7.2 × 两张卡）。这一条判的是「换墨不许动盒」，本轮的盒**是故意动的**
			// ——但动的是分组标题下面那一格，名字与分组标题的 y 一个字节没动（见上面 ②）。
			if got, want := a.PluginScroll.MaxY, float32(330.8); math.Abs(float64(got-want)) > 0.6 {
				t.Errorf("the sheet's scroll range = %v, want %v: the ink change must not move geometry",
					got, want)
			}
		})
	}
}

// P13-ei 出图：`FLOTER_PNG=1` 才跑。
func TestDumpP13EISheetName(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: set FLOTER_PNG=1 to dump the plugin sheet")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, _ := pluginSheetFrame(t, dark)
		tt.Frame()
		path := "/tmp/floter-P13-ei-sheet-" + appearance + ".png"
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
