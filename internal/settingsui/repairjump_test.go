package settingsui

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

// P13-cl · 一个键按下之后，它自己不该搬家。
//
// 集成页的重连 / 重新检查键是本仓唯一带「在飞」面的行内控制：静息时是一行字，
// 在飞时是环 + 一行字。此前那枚环是**加**上去的（10px + 5px 间距），于是按下的一
// 瞬间词盒 x 走 15px、胶囊也宽出 15px。参考那边不会动，因为环是**顶替**已有的
// RefreshCw（`ExtensionRow.tsx:305,325,364`）——1:1 换，槽本来就留着。
//
// 这一族判据（`docs/liquid-glass.md` §6.55）：
//
//	① 两个状态（静息 / 在飞）的**词盒 x 逐像素相同**（±1px：布局算术，不是 0）；
//	② 静息臂的 glyph 槽**不是空的**——词左边那一格里真的有墨，且与词同档深，
//	   否则「槽留好了」只是盒子变宽，读起来是一枚键左边莫名多了 15px 空白；
//	③ 在飞臂的那一格也是墨（环），不是面。
//
// 反面臂（量过、否掉）：把静息臂的 glyph 换成「只加左 padding」——词盒 x 与胶囊宽
// 都能对上，但 15px 的空白让胶囊的左内边距是右内边距的三倍，读成排版错误。
func TestTheRepairPillDoesNotMoveItsWordWhenTheWorkStarts(t *testing.T) {
	en := i18n.For("en").Settings

	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			a := newTestApp()
			a.Actions = Actions{
				RepairIntegration: func(string, func(error)) {},
				UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {
				},
			}
			a.Integrations = func() []Integration {
				return []Integration{{ID: "broken", Name: "Broken Tool", Enabled: true, Broken: true,
					RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing}}
			}
			tt := render(t, a, 760, 1200)
			tt.SetDark(appearance.dark)
			tt.Frame()
			if err := tt.Click("Integrations"); err != nil {
				t.Fatalf("click: %v (texts %v)", err, tt.Texts())
			}
			tt.Frame()

			rest := mustFind(t, tt, en.IntegrationsRecheck)
			markRest := markDelta(tt, rest)
			restLeft := markLeft(tt, rest)
			t.Logf("rest: word x=%.1f mark left=%d mark delta=%d", rest.X, restLeft, markRest)
			if markRest < 40 {
				t.Errorf("the resting pill has no mark left of its word: the darkest pixel in the glyph slot is %d, "+
					"the pill face beside it — the slot the in-flight ring swaps into is empty", markRest)
			}

			a.repairBusy = map[string]bool{"broken": true}
			tt.Frame()
			busy := mustFind(t, tt, en.IntegrationsRecheck)
			markBusy := markDelta(tt, busy)
			busyLeft := markLeft(tt, busy)
			t.Logf("busy: word x=%.1f mark left=%d mark delta=%d", busy.X, busyLeft, markBusy)

			if d := busy.X - rest.X; d < -1 || d > 1 {
				t.Errorf("the word moved %.1fpx when the work started (rest x=%.1f, busy x=%.1f): "+
					"the two arms of one pill must draw their word in the same place",
					d, rest.X, busy.X)
			}
			// The pill is the row's trailing control: its right edge is pinned, so
			// every pixel the pill grows shows up on its **left**. The mark sits at
			// the pill's own leading padding, so the mark's leftmost ink column is
			// that outline's proxy — and it is where a reader sees "the button grew
			// when I pressed it".
			if restLeft < 0 || busyLeft < 0 {
				t.Fatalf("no mark ink on the pill's leading edge (rest %d, busy %d)", restLeft, busyLeft)
			}
			if d := busyLeft - restLeft; d < -1 || d > 1 {
				t.Errorf("the pill's leading edge moved %dpx when the work started (rest %d, busy %d): "+
					"one button must not re-lay itself out under the pointer", d, restLeft, busyLeft)
			}
			if markBusy < 40 {
				t.Errorf("the in-flight arm has no ring left of its word: the slot is %d levels off the pill face",
					markBusy)
			}
		})
	}
}

// markSlot is the band the pill's leading mark occupies: one 14px glyph plus the
// 5px gap, immediately left of the word's own box.
func markSlot(word ui.Rect) ui.Rect {
	return ui.Rect{X: word.X - 19, Y: word.Y, W: 13, H: word.H}
}

// markLeft is the leftmost column of the pill's leading mark on the word's band:
// the first column going left from the word that carries ink, not face. The mark
// sits at the pill's own leading padding, so this column is where the pill's
// left outline would be if the padding were zero — a proxy that survives both
// arms being faded differently.
func markLeft(tt *ui.Tester, word ui.Rect) int {
	mid := word.Y + word.H/2
	face := sample(tt, word.X-3, mid)
	for x := int(word.X) - 6; x > int(word.X)-40; x-- {
		if channelDelta(sample(tt, float32(x), mid), face) > 40 {
			return x
		}
	}
	return -1
}

// markDelta is how far the darkest-or-brightest pixel in the mark's slot sits
// from the pill's own face. It is a **distance**, not a darkness: on the dark
// appearance the glyph is lighter than its face and on the light one darker, so
// an absolute ink threshold would only describe one of the two arms (and the
// in-flight pill is disabled, which fades its whole subtree a second way).
func markDelta(tt *ui.Tester, word ui.Rect) int {
	face := sample(tt, word.X-3, word.Y+word.H/2)
	most := 0
	slot := markSlot(word)
	for y := int(slot.Y); y < int(slot.Y+slot.H); y++ {
		for x := int(slot.X); x < int(slot.X+slot.W); x++ {
			if d := channelDelta(sample(tt, float32(x), float32(y)), face); d > most {
				most = d
			}
		}
	}
	return most
}

// The two arms of one pill, side by side, for the eye: opt-in because a PNG is
// not an assertion. `FLOTER_P13CL_DUMP=1` writes
// `/tmp/floter-P13-cl-repairpill-<tag>-<light|dark>.png`.
func TestDumpP13CLRepairPill(t *testing.T) {
	if os.Getenv("FLOTER_P13CL_DUMP") == "" {
		t.Skip("opt-in: set FLOTER_P13CL_DUMP=1 to dump the pill's two arms")
	}
	en := i18n.For("en").Settings
	_ = en
	tag := os.Getenv("FLOTER_P13CL_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range appearanceNames {
		a := newTestApp()
		a.Actions = Actions{
			RepairIntegration: func(string, func(error)) {},
			UninstallComponents: func(string, string, string, UninstallComponents, func(error)) {
			},
		}
		a.Integrations = func() []Integration {
			return []Integration{{ID: "broken", Name: "Broken Tool", Enabled: true, Broken: true,
				RuntimeOwnership: "system", FailureCode: extensions.BindingCodeMissing}}
		}
		tt := render(t, a, 760, 1200)
		tt.SetDark(appearance.dark)
		tt.Frame()
		if err := tt.Click("Integrations"); err != nil {
			t.Fatalf("click: %v", err)
		}
		tt.Frame()
		writePNG(t, "/tmp/floter-P13-cl-repairpill-rest-"+tag+"-"+appearance.name+".png", tt)
		a.repairBusy = map[string]bool{"broken": true}
		tt.Frame()
		writePNG(t, "/tmp/floter-P13-cl-repairpill-busy-"+tag+"-"+appearance.name+".png", tt)
	}
}
