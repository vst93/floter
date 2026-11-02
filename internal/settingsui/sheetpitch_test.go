package settingsui

import (
	"image"
	"image/png"
	"os"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"
)

// sheetBand is one run of rows carrying text ink: the first and last row whose
// worst pixel is at least `threshold` away from that row's own plane.
type sheetBandInk struct{ from, to, peak int }

// sheetInkBands finds the rows of the sheet that carry ink, above a threshold
// high enough to skip the faces themselves: a card's own edge reads 17 levels
// off the ground it sits on, while a label reads 202 and a help line 57. The
// window is the band the sheet was given (from the fixture's own geometry), not
// a hand-picked pixel box (§6.102).
func sheetInkBands(tt *ui.Tester, band image.Rectangle, y0, y1, threshold int) []sheetBandInk {
	img := tt.Image()
	bands := make([]sheetBandInk, 0, 8)
	for y := y0; y < y1; y++ {
		// The window is the sheet's **text column**: the left three quarters of
		// the band. The control slot is right-aligned at the band's far edge
		// (`sheetFieldSlot`, `MinWidth(Space(12))` = 43px), so a checkbox never
		// enters the probe — and a checkbox's solid run would otherwise merge a
		// field's help line into its label's band.
		x0, x1 := band.Min.X, band.Min.X+band.Dx()*3/4
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
		if worst < threshold {
			continue
		}
		if n := len(bands); n > 0 && y-bands[n-1].to <= 2 {
			bands[n-1].to = y
			if worst > bands[n-1].peak {
				bands[n-1].peak = worst
			}
			continue
		}
		bands = append(bands, sheetBandInk{from: y, to: y, peak: worst})
	}
	return bands
}

// P13-ej：分组标题离它自己的第一行只比两行之间多 2px——分组读不出来。
//
// 改前实测（`pluginSheetFrame`，深浅同值）：分组标题的墨 100..109，第一行标签的墨
// 135..143 → 空气 **25px**；而同一张卡里上一行的 help 墨 144..164 到下一行标签
// 188..197 的空气是 **23px**。两个数只差 2px，读者看到的是「又一行」，不是「新的一组」。
// §4.3 的规矩是「一个新的东西抬开一整档」，所以这一格走 `Space(2)` → `Space(4)`
// （+7.2px = 8u）：改后 25 → 32px，与 23px 拉开 9px。
//
// 判据两条：① 分组标题的空气比字段之间的空气至少多 6px（一整档的下限，改前差 2）；
// ② 名字仍在视口顶第 11 行、分组标题仍在第 33 行（这一轮只动标题与卡之间的那一格，
// 不许把整块面往上或往下挪）。
func TestThePluginSheetSectionHeadingStandsOffItsFirstField(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			band := sheetBand(tt)
			bands := sheetInkBands(tt, band, band.Min.Y, band.Min.Y+240, 40)
			t.Logf("bands %v", bands)
			// A label reads ≥190 levels off its plane, a help line 94-101, and a
			// row's hairline 43 (only in the dark frame) — so the pin picks the
			// **labels** by their ink and reads the help line as the strongest
			// band between two of them. Ordering alone would mistake a hairline
			// or a card's top edge for a label in the dark frame.
			labels := make([]sheetBandInk, 0, 3)
			for _, b := range bands {
				if b.peak >= 150 {
					labels = append(labels, b)
				}
			}
			if len(labels) < 3 {
				t.Fatalf("found %d text lines in the sheet's first 240 rows (%v): the pin reads the "+
					"heading and two field labels", len(labels), bands)
			}
			name, heading, label, next := labels[0], labels[1], labels[2], labels[3]
			var help sheetBandInk
			for _, b := range bands {
				if b.from > label.to && b.to < next.from && b.peak > help.peak {
					help = b
				}
			}
			if help.to == 0 {
				t.Fatalf("no help line between the first two field labels (%v): the sheet draws one "+
					"per field, and this pin measures the air under it", bands)
			}
			headingAir := label.from - heading.to - 1
			fieldAir := next.from - help.to - 1
			t.Logf("name %d..%d, heading %d..%d, label %d..%d, help %d..%d, next label %d..%d "+
				"(heading air %d, field air %d, MaxY %v)",
				name.from, name.to, heading.from, heading.to, label.from, label.to,
				help.from, help.to, next.from, next.to, headingAir, fieldAir, a.PluginScroll.MaxY)
			if gap := headingAir - fieldAir; gap < 6 {
				t.Errorf("the gap under a section heading is %dpx while the gap between two fields "+
					"is %dpx (a %dpx difference): a new group must stand off its first row by a whole "+
					"rung, not by two pixels — the heading reads as one more field (want ≥6)",
					headingAir, fieldAir, gap)
			}
			if got := name.from - band.Min.Y; got != 11 {
				t.Errorf("the name's ink starts %d rows below the band's top, want 11: the gap under a "+
					"heading must not move the block above it", got)
			}
			if got := heading.from - band.Min.Y; got != 44 {
				t.Errorf("the first section heading's ink starts %d rows below the band's top, want 44 "+
					"(it was 44 before this round): the new gap belongs below the heading, not above it",
					got)
			}
		})
	}
}

// P13-ej 出图：`FLOTER_PNG=1` 才跑。
func TestDumpP13EJSheetSections(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: set FLOTER_PNG=1 to dump the plugin sheet's sections")
	}
	tag := os.Getenv("FLOTER_P13EJ_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, _ := pluginSheetFrame(t, dark)
		img := tt.Image()
		crop := image.Rect(0, 56, 500, 260)
		out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
		for y := crop.Min.Y; y < crop.Max.Y; y++ {
			for x := crop.Min.X; x < crop.Max.X; x++ {
				out.Set(x-crop.Min.X, y-crop.Min.Y, img.At(x, y))
			}
		}
		path := "/tmp/floter-P13-ej-sections-" + tag + "-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, out); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}
