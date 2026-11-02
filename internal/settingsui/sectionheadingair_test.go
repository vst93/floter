package settingsui

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-ek：设置页的分组标题离它自己第一张卡，比同一组里两张卡之间**还紧**。
//
// 改前实测（`PageGeneral`，720×2000，深浅同值）：标题墨的底 → 第一张卡的上边
// **6.7~6.9px**（框架的 `Space(2)` = 7.2），而同一组里两张卡之间是 `Space(2.5)` =
// **9px**——标题比自己下面那些卡贴得更近，读起来是「又一张卡」，不是「新的一组」。
// 墨上同一句话：`Window` 组标题墨 451..461 → 第一行标签墨 486..498 是 **24px** 空气，
// 而组里上一行 help 墨 505..516 → 下一行标签墨 538..548 是 **21px**：差 3px。
//
// §4.3 的规矩是「间距属于成对的两件东西」+「一个新的东西抬开一整档」，所以抬的是
// 标题的**下边**一整档 8u：标题 → 卡 = `Space(4)`（改后 14.1~14.7px，墨上 32px 对
// 21px，差 11px），而卡 → 组注仍是 7.2px（那一对是「同一块」，§4.3 的相邻一档不许动）。
// 同一个格子在浮层里是 §6.105，逐值相同。

// sectionCardGaps reads every General-page group's heading → first-card gap off the
// picture (`cardTopGap`) and returns them in page order.
func sectionCardGaps(t *testing.T, tt *ui.Tester, copy i18n.Settings) ([]string, []float64) {
	t.Helper()
	img := tt.Image()
	names := make([]string, 0, 5)
	gaps := make([]float64, 0, 5)
	for _, g := range generalSectionGroups(copy) {
		box := mustFind(t, tt, sectionLabel(g.title))
		heading := mustFind(t, tt, g.title)
		names = append(names, g.name)
		gaps = append(gaps, cardTopGap(t, img, box, heading))
	}
	return names, gaps
}

func TestTheSectionHeadingStandsOffItsFirstCard(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, tokens := renderIn(t, a, 720, 2000, appearance.dark)

		// ① 每一组的标题 → 第一张卡都是一个数，而且是一整档：`Space(4)` 的 14.4px
		// （渲染器把它按组的 y 吸到 14.1~14.7）。改前是 `Space(2)` 的 6.7~6.9。
		names, gaps := sectionCardGaps(t, tt, copy)
		want := float64(tokens.Theme.Space(4))
		for i, gap := range gaps {
			if gap < want-1.5 || gap > want+1.5 {
				t.Errorf("[%v] %s's heading sits %.1fpx above its first card, want %.1fpx "+
					"(`Space(4)`, a whole rung — the card's own gap is %.1fpx): a heading that hugs "+
					"its card tighter than the cards hug each other reads as one more card",
					appearance.name, names[i], gap, want, float64(tokens.Theme.Space(2.5)))
			}
		}

		// ② 那一格必须比同一组里两张卡之间更松，且松一整档——否则「标题 → 卡」这一对
		// 仍然读成「卡 → 卡」那一对。
		cardGap := float64(tokens.Theme.Space(2.5))
		if spread := gaps[0] - cardGap; spread < 4 {
			t.Errorf("[%v] the heading→card gap is %.1fpx and the cards' own gap is %.1fpx "+
				"(a %.1fpx difference): the group's label must stand off its cards by a whole "+
				"rung, not by two pixels (want ≥4)", appearance.name, gaps[0], cardGap, spread)
		}

		// ③ 墨上同一个断言：`Window` 组的卡是一叠「标签 + help」，没有宽控件（`Appearance`
		// 的卡里有主题色块，`Menu bar` / `Terminal` 有它们自己的宽控件），所以它是唯一
		// 一组能整列读的。标题墨 → 第一行标签墨 必须比「上一行 help → 下一行标签」多
		// 一整档的下限 6px。
		g := generalSectionGroups(copy)[1]
		box := mustFind(t, tt, sectionLabel(g.title))
		heading := mustFind(t, tt, g.title)
		band := image.Rect(int(box.X)+8, int(heading.Y)-6, int(box.X+box.W*0.55), int(heading.Y)+230)
		bands := sheetInkBands(tt, band, band.Min.Y, band.Max.Y, 40)
		labels := make([]sheetBandInk, 0, 4)
		for _, b := range bands {
			if b.peak >= 150 {
				labels = append(labels, b)
			}
		}
		if len(labels) < 3 {
			t.Fatalf("[%v] found %d text lines in the Window group (%v): the pin reads the "+
				"heading and two row labels", appearance.name, len(labels), bands)
		}
		var help sheetBandInk
		for _, b := range bands {
			if b.from > labels[1].to && b.to < labels[2].from && b.peak > help.peak {
				help = b
			}
		}
		if help.to == 0 {
			t.Fatalf("[%v] no help line between the first two row labels (%v): every settings row "+
				"draws one, and this pin measures the air under it", appearance.name, bands)
		}
		headingAir := labels[1].from - labels[0].to - 1
		rowAir := labels[2].from - help.to - 1
		t.Logf("[%v] heading %d..%d, first label %d..%d, help %d..%d, next label %d..%d "+
			"(heading air %d, row air %d, card gaps %v)", appearance.name,
			labels[0].from, labels[0].to, labels[1].from, labels[1].to,
			help.from, help.to, labels[2].from, labels[2].to, headingAir, rowAir, gaps)
		if gap := headingAir - rowAir; gap < 6 {
			t.Errorf("[%v] the air under the section heading is %dpx while the air between two "+
				"rows is %dpx (a %dpx difference): a new group must stand off its first row by a "+
				"whole rung (want ≥6)", appearance.name, headingAir, rowAir, gap)
		}

		// ④ 只抬标题**下面**那一格：第一组的标题仍在它原来的 y 上，整页第一个像素没动。
		// 130 是改前实测的行（小数部分是布局的 0.9 步，不参与判据）。
		if got := mustFind(t, tt, generalSectionGroups(copy)[0].title).Y; math.Abs(float64(got)-130) > 1 {
			t.Errorf("[%v] the first group's heading moved to y=%.1f, want 130: this round's air "+
				"belongs below the heading, not above it", appearance.name, got)
		}
	}
}

// P13-ek 出图：`FLOTER_PNG=1` 才跑；`FLOTER_P13EK_TAG` 选改前/改后两趟。
func TestDumpP13EKGeneralSections(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: set FLOTER_PNG=1 to dump the General page's section headings")
	}
	tag := os.Getenv("FLOTER_P13EK_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		a.selectPage(PageGeneral)
		tt, _ := renderIn(t, a, 720, 2000, appearance.dark)
		img := tt.Image()
		// 两个相邻组之间的那一段：第一组的标题 + 卡顶 + 第二组的标题。
		crop := image.Rect(0, 120, 520, 500)
		out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
		for y := crop.Min.Y; y < crop.Max.Y; y++ {
			for x := crop.Min.X; x < crop.Max.X; x++ {
				out.Set(x-crop.Min.X, y-crop.Min.Y, img.At(x, y))
			}
		}
		path := "/tmp/floter-P13-ek-sections-" + tag + "-" + appearance.name + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, out); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s wrote %s (%dx%d)", appearance.name, path, out.Rect.Dx(), out.Rect.Dy())
	}
}
