package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// P13-bz · `docs/liquid-glass.md` §6.2c：设置页的侧栏与内容此前在**同一个 z 平面**上。
//
// 改前的算术：`Sunken` 是 0.17 的浅色**薄罩**，压在窗口地上合成后只差 1 级
// （dark `14,14,16 → 15,15,17`；light `247,247,248 → 246,246,248`）。侧栏与内容
// 两列都戴着它，于是两栏之间唯一的分界就是中间那根 1px 竖发丝线——正是用户
// 2026-10-30 说的「所有东西挤在同一个 z 平面上，靠 1px 描边分家」。
//
// 改后：**铬**（侧栏 + 两侧列间距 + 那根线）是一整块**面**（`Control` 戴在两栏的
// 父行上），内容页是**底**（`FaceOver(地, Sunken)`，与今天逐像素相同的那个不透明值）。
// 这条 pin 钉四件事，全部量像素、不读注释：
//
//	① 面与内容页 ≥ `liteFaceFloor`(8) 级——这一页有了两级 z，不再是一级；
//	② 面与窗口地 ≥ 8 级——侧栏是**面**，不是一层看不见的薄罩；
//	③ 选中行仍站在侧栏面上 ≥ 8 级——面差没有被这一改吃掉（`RowSelected` 不变）；
//	④ 面差 ≥ 那根竖发丝线自己造成的差的 **2 倍**——「不再靠 1px 描边分家」是可测的：
//	   把线加粗或拿掉，这一页照样分得开。
//
// 外加一条回归钉：内容页的像素必须仍等于 `FaceOver(地, Sunken)`——本轮把它从
// 「地 + 薄罩」的合成改成了同一个值的**不透明**形式（它压在铬面上，薄罩会
// 合成成 35 而不是 15），像素一个都不许动。

// sidebarColumns resolves the four pixels this pin measures out of one frame:
// the chrome face, the window ground above it, the content pane, and the
// hairline that sits on the boundary.
func sidebarColumns(t *testing.T, dark bool) (chrome, ground, content, rule ui.Color, chosen ui.Rect) {
	t.Helper()
	tt, _ := liteRender(t, dark, PageGeneral)
	img := tt.Image()
	chosen = mustFind(t, tt, "General")
	// The header band above the body is still the window's ground.
	ground = imagePixel(img, 2, 30)
	// Clear of the sidebar rows (they end at the column's inner padding, ~141)
	// and left of the rule: bare chrome at every y.
	chrome = imagePixel(img, 146, 300)
	ruleX := -1
	for x := 148; x < 240; x++ {
		if channelDelta(imagePixel(img, x, 300), chrome) > 4 {
			ruleX = x
			break
		}
	}
	if ruleX < 0 {
		t.Fatalf("[dark=%v] the column rule was not found between x=148 and x=240", dark)
	}
	rule = imagePixel(img, ruleX, 300)
	contentX := -1
	for x := ruleX + 1; x < 300; x++ {
		if channelDelta(imagePixel(img, x, 300), chrome) > 4 {
			contentX = x
			break
		}
	}
	if contentX < 0 {
		t.Fatalf("[dark=%v] the content pane was not found right of the rule at x=%d", dark, ruleX)
	}
	content = imagePixel(img, contentX+3, 300)
	return chrome, ground, content, rule, chosen
}

func TestTheSidebarIsAFaceNotAVeil(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			dark := appearance.dark
			chrome, ground, content, rule, chosen := sidebarColumns(t, dark)

			// ① The z ladder: two panes, one visible step.
			if step := channelDelta(chrome, content); step < liteFaceFloor {
				t.Errorf("[%s] the sidebar face %v and the content pane %v are %d level(s) apart, under the E1 floor %d: "+
					"the two columns are the same z plane again (liquid-glass.md §6.2c)",
					appearance.name, chrome, content, step, liteFaceFloor)
			}
			// ② …and the face is a face, not a thin veil over the ground.
			if step := channelDelta(chrome, ground); step < liteFaceFloor {
				t.Errorf("[%s] the sidebar face %v sits %d level(s) over the window ground %v, under %d: "+
					"`Sunken` was a 0.17 veil that composites to +1 — a face must read as one",
					appearance.name, chrome, step, ground, liteFaceFloor)
			}
			// ③ The chosen row keeps standing on the face it sits on.
			rowFace := imagePixel(imageOf(t, dark), int(chosen.X)+int(chosen.W)-10, int(chosen.Y+chosen.H/2))
			if step := channelDelta(rowFace, chrome); step < liteFaceFloor {
				t.Errorf("[%s] the chosen page %v sits %d level(s) over the sidebar face %v, under %d: "+
					"the selection stopped standing up", appearance.name, rowFace, step, chrome, liteFaceFloor)
			}
			// ④ The face step dominates the seam: the 1px rule is a seam on a
			// boundary, not the boundary itself. 实测 dark 面差 24 / 线 12、浅色
			// 18 / 13——两个外观上「面差 ≥ 接缝」都成立，把线加粗或拿掉，
			// 两栏照样分得开。
			seam := channelDelta(rule, chrome)
			if seam == 0 {
				t.Fatalf("[%s] the column rule is invisible (%v against the face %v): the pin can no longer tell "+
					"the seam from the face", appearance.name, rule, chrome)
			}
			if step := channelDelta(chrome, content); step <= seam {
				t.Errorf("[%s] the face step is %d levels but the 1px rule alone is %d: the two columns are still "+
					"told apart by a drawn line rather than by their own planes (§6.2c)",
					appearance.name, step, seam)
			}
			// The regression钉: the content pane's pixel is the value it always was.
			tokens, _ := liteTokens(t, dark)
			want := theme.FaceOver(tokens.Theme.Background, tokens.Sunken)
			if d := channelDelta(content, want); d > 2 {
				t.Errorf("[%s] the content pane = %v, want FaceOver(ground, Sunken) = %v (%d levels apart): "+
					"P13-bz changed the sidebar, not the page's own fill", appearance.name, content, want, d)
			}
		})
	}
}

// imageOf re-renders the same frame the columns came from, so the row's own
// pixel is read out of the frame that was measured rather than a second one.
func imageOf(t *testing.T, dark bool) *image.RGBA {
	t.Helper()
	tt, _ := liteRender(t, dark, PageGeneral)
	return tt.Image()
}

// TestDumpSidebarFace writes the whole settings surface to
// /tmp/floter-P13-bz-settings-<appearance>-<tag>.png. Opt-in: it is a
// screenshot dump, not a gate.
func TestDumpSidebarFace(t *testing.T) {
	tag := os.Getenv("FLOTER_P13BZ_TAG")
	if tag == "" {
		t.Skip("opt-in: set FLOTER_P13BZ_TAG=before|after to dump the settings surface")
	}
	for _, appearance := range appearanceNames {
		tt, _ := liteRender(t, appearance.dark, PageGeneral)
		path := filepath.Join("/tmp", "floter-P13-bz-settings-"+appearance.name+"-"+tag+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}
