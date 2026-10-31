package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-ah · 受光边讲「级数」，不是讲 alpha（`docs/liquid-glass.md` §1 的 0-11 行）。
//
// 这条 pin 钉的是**绘制出来的级数**，不是 token 的 alpha：同一个 0.09 在两套外观上
// 差 6 倍（浅色 +3 级 / 深色 +19 级），所以 alpha 本身不能当断言。量法沿用
// `litProbe`（真页面地面 + 产品自己的画法），读冠线行与面行：
//
//   - 浅色档：冠线比面亮 **≥ 24**（三通道和 = 8 级，§1 的 8 级下沿）。旧值 0.09 是
//     +9（+3 级）—— 1x 屏上画不出的名义像素，这条 pin 就是它变红的样子。
//   - 深色档：**一个字节都不许动**（+56）。深色的 0.09 本来就够，把它一起抬才是错。
func TestTheLitEdgeSpeaksInLevelsNotAlpha(t *testing.T) {
	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		_, crown, _, face := litProbe(t, dark, true, func(a *App, c *ui.Context) ui.Element {
			return boundSquare(c, func() {
				a.sessionIconButton(c, "play", en.SessionsResume, false, true, false, func() {})
			})
		})
		step := litSum(crown) - litSum(face)
		if dark {
			if step != 56 {
				t.Errorf("the dark lit edge stepped %d levels over its face (%v vs %v), want the pinned 56 (+19/channel): "+
					"the dark appearance's 0.09 already reads and must not ride the light appearance's change",
					step, crown, face)
			}
			continue
		}
		if step < 24 {
			t.Errorf("the light lit edge stepped only %d levels over its face (%v vs %v), under the 8-level floor "+
				"(24 in the three-channel sum): the crown is a nominal pixel, not a lit line", step, crown, face)
		}
	}
	// 第二块面：胶囊的面比图标键亮（230 对 221），头顶空间就只有 25 级 —— 同一条 0.24
	// 落成 **+6 级/通道**（和 18）。它比按钮小哥一档是应该的（面越白越没地方亮），
	// 所以它的地板是 18，不是 24：这块面要证的是「它也跟着亮起来了」，不是「它和按钮一样响」。
	_, crown, _, face := litProbe(t, false, false, func(a *App, c *ui.Context) ui.Element {
		return a.sheetActionButton(c, "Clear history", false, func() {})
	})
	if step := litSum(crown) - litSum(face); step < 18 {
		t.Errorf("the light action pill stepped only %d levels over its face (%v vs %v), under 18: "+
			"a pill on a 230 face has 25 levels of headroom and must take some of it", step, crown, face)
	} else {
		t.Logf("light action pill: crown %v face %v = +%d (%d/channel)", crown, face, step, step/3)
	}
}
