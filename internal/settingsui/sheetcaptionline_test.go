package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/theme"

	"github.com/egoist/mygo/ui"
)

// P13-db · **插件配置弹层里那句「这条没写进去」，挂在字段那一列。**
//
// 这块浮层上有一条关于**字段**的句子（`Settings.SaveFailed`）和一条关于**动作**的
// 句子（动作自己那一行的 `field.Failed`）。两条说的是同一件事——一次写被插件拒了
// ——却挂在不同列上：动作那条坐在行里（行标签从 `28.0` 起），写失败那条挂在浮层的
// 框上（scroller 的 4u，实测墨左沿 `14.4`），是这张表上唯一一条比它讲的那批字段
// （右端 `692`）还宽的句子。
//
// 这一条钉两件事：写失败线现在与它下一行的分组标题**同一条左沿**、与字段标签
// **同一条右沿**（也就是「和它介绍的东西同一列」，`liquid-glass.md` §4.3 的规则用在
// 一块浮层上）；而浮层自己的名字仍然挂在浮层的列上，不跟着缩进——它是这块面的
// 名字，不是某张卡的话。
func TestTheSheetsWriteFailureHangsOnTheFieldsColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			store := pluginSheetStore(t, "en")
			copy := i18n.For("en").Settings
			schema := &plugincfg.Schema{
				Plugin: "probe",
				Title:  "Probe plugin",
				Fields: []plugincfg.Field{{
					Key:     "k",
					Kind:    plugincfg.Toggle,
					Label:   "Probe field",
					Help:    "Probe help",
					Section: "Probe section",
				}},
			}
			tt := renderPluginSheet(t, store, PluginSheet{
				Schema: schema, Values: plugincfg.Values{}, Failed: true,
			}, 720, 900, dark)

			failure, ok := tt.Find(copy.SaveFailed)
			if !ok {
				t.Fatalf("the sheet does not draw its write-failure line; it drew %q", tt.Texts())
			}
			label, ok := tt.Find("Probe field")
			if !ok {
				t.Fatalf("the sheet does not draw its field label; it drew %q", tt.Texts())
			}
			title, ok := tt.Find("Probe plugin")
			if !ok {
				t.Fatalf("the sheet does not draw its title; it drew %q", tt.Texts())
			}

			// The one sentence about a refused write hangs on the column of the
			// thing it reports — the fields — and not on the sheet's own frame.
			// A glyph's left bearing is not zero, so the question is which column
			// it is *nearer*, not which pixel it starts on.
			failureInk, titleInk, labelInk := inkLeft(tt, failure), inkLeft(tt, title), inkLeft(tt, label)
			if failureInk < 0 || titleInk < 0 || labelInk < 0 {
				t.Fatalf("%s: no ink inside one of the boxes (failure %.1f, title %.1f, label %.1f)",
					appearance, failureInk, titleInk, labelInk)
			}
			toTitle, toLabel := abs(float64(failureInk-titleInk)), abs(float64(failureInk-labelInk))
			if toLabel >= toTitle {
				t.Errorf("the write-failure line's ink starts at x=%.1f — %.1f from the fields' column (x=%.1f) "+
					"and %.1f from the sheet's own column (x=%.1f): it reports a write the fields refused, "+
					"so it hangs where they do, not on the sheet's frame",
					failureInk, toLabel, labelInk, toTitle, titleInk)
			}
			// The sheet's own name keeps the sheet's column: the scroller's own 4u
			// padding, the column a settings page's header hangs on too.
			if got, want := pixel(title.X), pixel(theme.For(settings.Default(), dark).Theme.Space(4)); got != want {
				t.Errorf("the sheet's title starts at x=%s, want the sheet's own column x=%s", got, want)
			}
		})
	}
}

// pixel renders a coordinate at the resolution the layout paints: one decimal.
func pixel(v float32) string { return fmt.Sprintf("%.1f", v) }

// inkLeft is the first column inside r whose pixel leaves the plane the box
// sits on. The plane is sampled at the box's own right edge, the way inkRows
// samples it: a Text box that carries padding ends in padding, and one that
// does not ends where its last glyph's bearing ends.
func inkLeft(tt *ui.Tester, r ui.Rect) float32 {
	img := tt.Image()
	y := int(r.Y) + int(r.H)/2
	px := y*img.Stride + (int(r.X+r.W)-1)*4
	plane := luma(img.Pix[px], img.Pix[px+1], img.Pix[px+2])
	for x := int(r.X); x < int(r.X+r.W) && x < img.Bounds().Dx(); x++ {
		i := y*img.Stride + x*4
		if d := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) - plane; d > 8 || d < -8 {
			return float32(x)
		}
	}
	return -1
}

// The opt-in dump: the plugin configuration sheet with both of its failure
// lines showing, both appearances. Run with FLOTER_P13DB_TAG=before while the
// failure line's padding is re-injected to get the comparison pair.
func TestDumpP13DBSheetCaptionLine(t *testing.T) {
	if os.Getenv("FLOTER_P13DB_DUMP") == "" {
		t.Skip("set FLOTER_P13DB_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13DB_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		store := pluginSheetStore(t, "en")
		copy := i18n.For("en").Settings
		schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
		tt := renderPluginSheet(t, store, PluginSheet{
			Schema: schema, Values: values, Failed: true, ActionFailed: "clear_history",
		}, 720, 900, dark)
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		name := filepath.Join(os.TempDir(), fmt.Sprintf("floter-P13-db-pluginsheet-%s-%s.png", appearance, tag))
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
