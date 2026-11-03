package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
)

// P13-ep · 浮层动作被拒之后那句话是**插件数据**，而它和一枚 25.2px 的胶囊并排。
//
// 本轮是**先量**的一轮，量出来的是一条真的病，但它的根不在墨里，所以按纪律没有
// 擅自改（见 `docs/rounds/P13-ep-report.md` 与 `.pending-bugs.md` 的「需拍板」）。
//
// 量到的事实（深浅同值，720 宽与 420 宽同值）：
//
//	浮层内容高度 `PluginScroll.MaxY`     短句 486.2 / 长句 666.2   → **+180px**
//	同一帧里的横向范围 `MaxX`            0 / 0                    → 不是横向溢出
//	把长句换成 360 个字符的**单个词**      666.2                    → 与折行无关
//	给那句话加 `SingleLine()` / `NoWrap()` / `Grow(1)` / `Shrink(1)` → 全是 666.2
//
// 180px ≈ 13 行 `Caption` 行盒，正好是行标签 "Clear history" 的**字符数**——也就是
// 那句话的最小宽度（一个不可断的长词）把 `a.row` 的标签列压到了「一个字符一行」。
// 根因是**行内 flex 的宽度争夺**，不是那句话折了几行：任何只作用在那句话上的
// 排版约束都改不动它，能改的只有「这句话该占多宽」或「它该不该在这一行里」——
// 两条都是几何/布局决定，属「需拍板」。
//
// 这条探针把上面那张表重新算一遍并出图，不设判据（本轮没有产品改动可钉）。

// sheetFailureSchema is a schema tall enough to overflow a short viewport, with
// the action field last — so the sentence it draws is the last thing on the sheet
// and any growth shows up in `MaxY` in full.
func sheetFailureSchema(failed string) *plugincfg.Schema {
	fields := make([]plugincfg.Field, 0, 8)
	for i := 0; i < 7; i++ {
		fields = append(fields, plugincfg.Field{
			Key:     "field" + string(rune('a'+i)),
			Section: "General",
			Label:   "Retention",
			Help:    "how long the history is kept",
			Kind:    plugincfg.Text,
		})
	}
	fields = append(fields, plugincfg.Field{
		Key:     "clear",
		Section: "General",
		Label:   "Clear history",
		Help:    "wipes every entry",
		Kind:    plugincfg.Action,
		Confirm: "Clear",
		Cancel:  "Cancel",
		Failed:  failed,
	})
	return &plugincfg.Schema{Title: "V Tools", Fields: fields}
}

// sheetFailureFrame draws the real `PluginSheet` at one width with one failure
// sentence, in the launcher's own chain (panel, then the band the sheet is given).
func sheetFailureFrame(t *testing.T, dark, narrow bool, failed string) (*ui.Tester, *App) {
	t.Helper()
	a := New(newStore(t), Actions{})
	sheet := PluginSheet{
		Schema:       sheetFailureSchema(failed),
		Values:       plugincfg.Values{},
		ActionFailed: "clear",
	}
	build := func(c *ui.Context) {
		tokens := a.tokens(c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(tokens.Theme.Background)
		ui.Box(c).Fill().Background(tokens.Sunken).Children(func() {
			ui.Box(c).Fill().Padding(56, 4, 0, 4).Children(func() {
				a.PluginSheet(c, sheet)
			})
		})
	}
	var tt *ui.Tester
	if narrow {
		tt = ui.NewTester(build, 420, 240)
	} else {
		tt = ui.NewTester(build, 720, 300)
	}
	tt.Frame()
	if dark {
		tt.SetDark(true)
		tt.Frame()
	}
	// Two frames: the scroller's range is a layout result, and the first frame
	// only establishes it.
	tt.Frame()
	return tt, a
}

// TestProbeP13EpTheActionFailureSentenceSqueezesTheRow is the opt-in measurement
// that produced the table above: it prints the four numbers and writes the
// frames, and asserts nothing (there is no product change this round to pin).
//
// FLOTER_SHEET_FAILURE_PROBE=1 runs it; without the variable it skips, because a
// probe that only logs is not a gate.
func TestProbeP13EpTheActionFailureSentenceSqueezesTheRow(t *testing.T) {
	if os.Getenv("FLOTER_SHEET_FAILURE_PROBE") == "" {
		t.Skip("set FLOTER_SHEET_FAILURE_PROBE=1 to re-measure the action row's width fight")
	}
	short := "refused"
	long := strings.Repeat("could not clear the history: permission denied on ~/.cache/floter ", 6)
	for _, dark := range []bool{false, true} {
		for _, narrow := range []bool{false, true} {
			name := appearanceName(dark) + "/" + map[bool]string{false: "wide", true: "narrow"}[narrow]
			t.Run(name, func(t *testing.T) {
				noneTT, none := sheetFailureFrame(t, dark, narrow, "")
				shortTT, base := sheetFailureFrame(t, dark, narrow, short)
				longTT, withLong := sheetFailureFrame(t, dark, narrow, long)
				_ = noneTT
				_ = shortTT
				t.Logf("MaxY: no sentence %.1f, short %.1f, long %.1f (+%.1f) | MaxX short %.1f, long %.1f",
					none.PluginScroll.MaxY, base.PluginScroll.MaxY, withLong.PluginScroll.MaxY,
					withLong.PluginScroll.MaxY-base.PluginScroll.MaxY,
					base.PluginScroll.MaxX, withLong.PluginScroll.MaxX)
				if os.Getenv("FLOTER_PNG") == "" {
					return
				}
				for _, pair := range []struct {
					tag string
					tt  *ui.Tester
				}{{"none", noneTT}, {"short", shortTT}, {"long", longTT}} {
					file, err := os.Create(filepath.Join("/tmp",
						"floter-P13-ep-"+pair.tag+"-"+map[bool]string{false: "light", true: "dark"}[dark]+"-"+
							map[bool]string{false: "wide", true: "narrow"}[narrow]+".png"))
					if err != nil {
						t.Fatal(err)
					}
					if err := png.Encode(file, pair.tt.Image()); err != nil {
						t.Fatal(err)
					}
					file.Close()
				}
			})
		}
	}
}
