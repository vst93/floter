package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"reflect"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func pluginViewportDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KP_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kp-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPluginViewport(t *testing.T, kind plugincfg.Kind, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			sheet := PluginSheet{Schema: longSchema("viewport"), Values: plugincfg.Values{}}
			field := &sheet.Schema.Fields[5]
			field.Kind, field.Label = kind, "当前字段"
			field.Min, field.Max, field.Step = 0, 100, 1
			field.Options = []plugincfg.Option{{Value: "north", Label: "北方"}, {Value: "south", Label: "南方"}}
			field.Confirm, field.Cancel = "确认清理", "取消清理"
			var initial any = "stored"
			switch kind {
			case plugincfg.Radio, plugincfg.Select:
				initial = "north"
			case plugincfg.Checkboxes:
				initial = []string{}
			case plugincfg.Toggle:
				initial = false
			case plugincfg.Number, plugincfg.Slider:
				initial = float64(12)
			}
			sheet.Values[field.Key] = initial
			calls := 0
			sheet.Change = func(key string, v any) {
				if key != field.Key {
					t.Errorf("resize wrote sibling %q", key)
				}
				calls++
				sheet.Values[key] = v
			}
			sheet.Arm = func(key string) { calls++; sheet.Armed = key }
			sheet.Run = func(string) { t.Error("resize ran action") }
			sheet.Unarm = func(string) { sheet.Armed = "" }
			width, height := float32(560), float32(400)
			var target, viewport ui.Rect
			focused := false
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Width(width).Height(height).Children(func() { a.PluginSheet(c, sheet) })
				viewport = c.Resolve(a.PluginScroll.Handle).Bounds()
				e := c.Resolve(*a.pluginEditorRef(field.Key))
				target, focused = e.Bounds(), e.FocusWithin()
				if kind == plugincfg.Text || kind == plugincfg.Number {
					target.Y += (target.H - c.Theme().Space(7)) / 2
					target.H = c.Theme().Space(7)
				}
			}, 560, 400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			pluginViewportDump(t, tt, name+"-rest", dark)
			for i := 0; i < 45 && !focused; i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !focused {
				t.Fatal("cannot reach resize target")
			}
			if kind == plugincfg.Text || kind == plugincfg.Number {
				tt.Key(ui.Cmd, ui.KeyA)
				if kind == plugincfg.Text {
					tt.Type("continued")
				} else {
					tt.Type("1e")
				}
				tt.Frame()
			}
			for _, size := range [][2]float32{{560, 220}, {320, 180}, {560, 400}} {
				width, height = size[0], size[1]
				tt.Frame()
				tt.Frame()
				tt.Frame()
				pluginViewportDump(t, tt, fmt.Sprintf("%s-%dx%d", name, int(width), int(height)), dark)
				t.Logf("%s size=%v target=%+v viewport=%+v scroll=%v", name, size, target, viewport, a.PluginScroll.Y)
				if !focused {
					t.Error("resize lost current field")
				}
				if target.Y-3 < viewport.Y+11 || target.Y+target.H+3 > viewport.Y+viewport.H-11 {
					t.Errorf("%s resize hides active ink: target=%+v viewport=%+v", name, target, viewport)
				}
			}
			if calls != 0 {
				t.Fatalf("resize wrote %d times", calls)
			}
			if kind == plugincfg.Text && a.pluginTextDrafts["viewport\x00"+field.Key] != "continued" {
				t.Error("resize discarded draft")
			}
			if kind == plugincfg.Number && a.pluginNumberDrafts["viewport\x00"+field.Key] != "1e" {
				t.Error("resize discarded partial number")
			}
			tt.Wheel(160, 90, 0, 70)
			tt.Frame()
			manual := a.PluginScroll.Y
			sheet.Values["fielda"] = "host value"
			tt.Frame()
			tt.Frame()
			if a.PluginScroll.Y != manual {
				t.Fatal("unchanged size stole wheel position")
			}
			var want any
			switch kind {
			case plugincfg.Radio:
				tt.Key(0, ui.KeyRight)
				want = "south"
			case plugincfg.Checkboxes:
				tt.Key(0, ui.KeySpace)
				want = []string{"north"}
			case plugincfg.Toggle:
				tt.Key(0, ui.KeySpace)
				want = true
			case plugincfg.Select:
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				want = "south"
			case plugincfg.Text:
				tt.Key(0, ui.KeyEnter)
				want = "continued"
			case plugincfg.Number:
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				want = float64(24)
			case plugincfg.Slider:
				tt.Key(0, ui.KeyRight)
				want = float64(13)
			case plugincfg.Action:
				tt.Key(0, ui.KeyEnter)
			}
			tt.Frame()
			if kind == plugincfg.Action {
				if calls != 1 || sheet.Armed != field.Key {
					t.Error("resized action cannot arm")
				}
			} else if calls != 1 || !reflect.DeepEqual(sheet.Values[field.Key], want) {
				t.Errorf("resized edit got=%v want=%v calls=%d", sheet.Values[field.Key], want, calls)
			}
		})
	}
}

func TestPluginViewportRadio(t *testing.T) { checkPluginViewport(t, plugincfg.Radio, "radio") }
func TestPluginViewportCheckboxes(t *testing.T) {
	checkPluginViewport(t, plugincfg.Checkboxes, "checkboxes")
}
