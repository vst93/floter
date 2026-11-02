package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// P13-en · 浮层的分组换行比它自己组内的两行还挤。
//
// §6.105（P13-ej）抬的是「分组标题 → 它自己的第一张卡」那一对（`sheetSection` 的
// `Space(2)` → `Space(4)`）。同一张浮层上**另一对**——「上一组最后一张卡 → 下一组
// 标题」——此前是外层 `Column` 的 `Gap(t.Space(3))`，比它自己组内的 `Space(4)`
// **低一档**。量的是**盒对盒**（同一种东西；墨对盒会把行盒上身算进空气）：
//
//	对                              改前     改后（第二组自带 Space(4) 上内边距）
//	第一张卡底 → 第二组标题盒顶      12.2px   26.6px
//	第二组标题盒底 → 它自己那张卡顶   14.8px   14.4px
//	浮层名字盒底 → 第一组标题盒顶     14.9px   14.9px（**不动**）
//
// 改前**换一组比组内还紧 2.6px** → 第二组标题读成第一张卡的最后一行小字。
// 改后 26.6px 落在 §4.3 那条「一个可以按的新东西」的 20px 档上（与 §6.69 会话页
// 「注底 → 页脚胶囊 22px」同一读数），组内 14.4px 留在「同一段的下一行」档——
// 两条不同的东西拿到两条不同的档，这才是 §4.3 的规矩。
//
// 为什么空气加在**组身上**而不是外层 `Column` 的 `Gap`：那一列的孩子是
// 「页头 / 第一组 / 第二组」，`Gap` 是**三对共用**的一个数——抬它会把
// 「页头 → 第一组」一起抬（实测 14.9 → 25.7px），而那一对不是「换一组」。
// 这是 §6.106 已经立过的教训：「空气属于那一对，就加在那一对的元素上」。
//
// 为什么不是 `Space(5)`（+2u，与 §6.105 / §6.106 同一步）：`Space(5)` 的盒间是
// 19.2px，与组内的 14.4px 只拉开 4.8px，够不到「一整档」的下限 6px（§6.105 的 pin 判）。
//
// 三条判据：① 组边界 ≥ 组内 + 6px（一整档的下限，改前差 **−2.6**）；
// ② 组边界 ≤ 组内的两倍（否则「换一组」变成「换一章」，另一个角色）；
// ③ 浮层名字 → 第一组 ≥ 组内（名字与分组标题是同一个角色，这一轮不许把它压到组内之下）。
//
// 窗口全部来自夹具自己的 `sheetBand()` 与 `sheetInkBands()`，不写死像素（§6.102 的教训）。

func TestThePluginSheetOpensANewGroupWiderThanANewRow(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			img := tt.Image()
			p := img.Pix
			band := sheetBand(tt)
			// 卡的面是这一列上连续的「非地面」段：`x = 300` 在卡内、在字的右边，
			// 所以它只读得到卡的填充与它的一圈边，读不到任何墨。
			const col = 300
			gr := int(p[(band.Min.Y-1)*img.Stride+col*4])
			gg := int(p[(band.Min.Y-1)*img.Stride+col*4+1])
			gb := int(p[(band.Min.Y-1)*img.Stride+col*4+2])
			blank := func(y int) bool {
				i := y*img.Stride + col*4
				return int(p[i]) == gr && int(p[i+1]) == gg && int(p[i+2]) == gb
			}
			var cards [][2]float32
			in, start := false, 0
			for y := band.Min.Y; y < band.Max.Y; y++ {
				if !blank(y) && !in {
					start, in = y, true
				}
				if blank(y) && in {
					cards = append(cards, [2]float32{float32(start), float32(y - 1)})
					in = false
				}
			}
			if in {
				cards = append(cards, [2]float32{float32(start), float32(band.Max.Y - 1)})
			}
			// 深色档的顶部 / 底部 ScrollEdge 各 5 行也会读成「非地面」；按高度筛掉
			// （真卡至少 30 行，边只有 5 行）。
			tall := cards[:0]
			for _, c := range cards {
				if c[1]-c[0] > 20 {
					tall = append(tall, c)
				}
			}
			cards = tall
			if len(cards) < 2 {
				t.Fatalf("found %d card faces in the sheet (%v): the pin reads the last card of one "+
					"group and the first of the next", len(cards), cards)
			}
			name, ok := tt.Find("Probe plugin")
			if !ok {
				t.Fatalf("the sheet's name was not drawn: %q", tt.Texts())
			}
			first, ok := tt.Find("First section")
			if !ok {
				t.Fatalf("the first group's heading was not drawn: %q", tt.Texts())
			}
			second, ok := tt.Find("Second section")
			if !ok {
				t.Fatalf("the second group's heading was not drawn: %q", tt.Texts())
			}
			cardToHeading := second.Y - cards[0][1]
			headingToCard := cards[1][0] - (second.Y + second.H)
			nameToFirst := first.Y - (name.Y + name.H)
			t.Logf("card → next heading %.1fpx, heading → its card %.1fpx, name → first heading "+
				"%.1fpx (MaxY %v)", cardToHeading, headingToCard, nameToFirst, a.PluginScroll.MaxY)

			if gap := cardToHeading - headingToCard; gap < 6 {
				t.Errorf("the air from a group's last card to the next group's heading is %.1fpx while "+
					"the air from that heading to its own card is %.1fpx (a %.1fpx difference): opening "+
					"a new group is tighter than moving one row inside it, so the heading reads as the "+
					"previous card's last line — a new group must stand off by a whole rung (want ≥6)",
					cardToHeading, headingToCard, gap)
			}
			if cardToHeading > 2*headingToCard {
				t.Errorf("the air between two groups is %.1fpx against %.1fpx inside one (%.0f%%): a "+
					"group break has become a chapter break, which is a different role (§4.3)",
					cardToHeading, headingToCard, 100*cardToHeading/headingToCard)
			}
			if nameToFirst < headingToCard {
				t.Errorf("the sheet's name stands off its first group by %.1fpx while a heading stands "+
					"off its card by %.1fpx: the two share one gap, so this round must not flatten the "+
					"ladder", nameToFirst, headingToCard)
			}
			// P13-en 重定基线：旧值 **330.8** → 新值 **345.2**。只有**第二组**自带
			// 一条 `Space(4)` 的上内边距（第一组不动），所以 `MaxY` 由内容自然长
			// **+14.4**。判据本身（「边是画出来的、不是几何」）不动，基线跟着内容
			// 高度走。
			if got := a.PluginScroll.MaxY; got < 345.15 || got > 345.25 {
				t.Errorf("the sheet's scroll range moved: MaxY %v, want 345.2 (one group's own "+
					"Space(4) over the P13-ej baseline 330.8)", got)
			}
			if tag := os.Getenv("FLOTER_P13EN_TAG"); tag != "" {
				name := filepath.Join("/tmp",
					fmt.Sprintf("floter-P13-en-groups-%s-%s.png", tag, appearanceName(dark)))
				f, err := os.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s", name)
			}
		})
	}
}
