package settingsui

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"reflect"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func pluginFieldEdgeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KN_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kn-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPluginFieldEdge(t *testing.T, kind plugincfg.Kind, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			sheet := PluginSheet{Schema: longSchema("edge"), Values: plugincfg.Values{}}
			field := &sheet.Schema.Fields[5]
			field.Kind, field.Label = kind, "当前字段"
			field.Min, field.Max, field.Step = 0, 100, 1
			field.Options = []plugincfg.Option{{Value: "north", Label: "北方"}, {Value: "south", Label: "南方"}}
			field.Confirm, field.Cancel = "确认清理", "取消清理"
			var initial any = "stored"
			switch kind {
			case plugincfg.Radio, plugincfg.Select:
				initial = "north"
			case plugincfg.Toggle:
				initial = false
			case plugincfg.Checkboxes:
				initial = []string{}
			case plugincfg.Slider, plugincfg.Number:
				initial = float64(12)
			}
			sheet.Values[field.Key] = initial
			calls := 0
			sheet.Change = func(key string, v any) {
				if key != field.Key {
					t.Errorf("changed sibling %q", key)
				}
				calls++
				sheet.Values[key] = v
			}
			sheet.Arm = func(key string) { calls++; sheet.Armed = key }
			sheet.Run = func(string) { t.Error("focus ran destructive command") }
			sheet.Unarm = func(string) { sheet.Armed = "" }
			var viewport, target ui.Rect
			focused := false
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				a.PluginSheet(c, sheet)
				viewport = c.Resolve(a.PluginScroll.Handle).Bounds()
				e := c.Resolve(*a.pluginEditorRef(field.Key))
				target, focused = e.Bounds(), e.FocusWithin()
				if kind == plugincfg.Text || kind == plugincfg.Number {
					target.Y += (target.H - c.Theme().Space(7)) / 2
					target.H = c.Theme().Space(7)
				}
			}, 560, 260)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			pluginFieldEdgeDump(t, tt, name+"-rest", dark)
			maxY := a.PluginScroll.MaxY
			for i := 0; i < 45 && !focused; i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !focused {
				t.Fatal("cannot reach plugin field")
			}
			bounds := func() ui.Rect {
				if kind == plugincfg.Checkboxes {
					return mustFind(t, tt, "北方")
				}
				return target
			}
			absoluteY := bounds().Y + a.PluginScroll.Y
			for _, bottom := range []bool{false, true} {
				// Leave the whole field, then return through its first control.
				if bottom {
					tt.Key(ui.Shift, ui.KeyTab)
				} else {
					for i := 0; i < 5 && focused; i++ {
						tt.Key(0, ui.KeyTab)
						tt.Frame()
					}
				}
				tt.Frame()
				a.PluginScroll.Y = absoluteY - viewport.Y + 8
				if bottom {
					a.PluginScroll.Y = absoluteY - viewport.Y - viewport.H + 8
				}
				tt.Frame()
				if bottom {
					tt.Key(0, ui.KeyTab)
				} else {
					tt.Key(ui.Shift, ui.KeyTab)
				}
				tt.Frame()
				// A checkboxes return lands on the last option; both share this line.
				if !focused {
					t.Fatal("return lost plugin field focus")
				}
				side := "top"
				if bottom {
					side = "bottom"
				}
				pluginFieldEdgeDump(t, tt, name+"-"+side, dark)
				r := bounds()
				t.Logf("%s %s target=%+v viewport=%+v scroll=%v", name, side, r, viewport, a.PluginScroll.Y)
				if math.Floor(float64(r.Y-3)) < math.Floor(float64(viewport.Y+11)) || math.Ceil(float64(r.Y+r.H+3)) > math.Ceil(float64(viewport.Y+viewport.H-11)) {
					t.Errorf("%s %s active ink intersects sheet fade: %+v", name, side, r)
				}
			}
			if a.PluginScroll.MaxY != maxY {
				t.Errorf("focus changed scroll range: %v -> %v", maxY, a.PluginScroll.MaxY)
			}
			if calls != 0 {
				t.Fatalf("focus wrote %d times", calls)
			}
			before := a.PluginScroll.Y
			tt.Wheel(viewport.X+viewport.W/2, viewport.Y+viewport.H/2, 0, 40)
			tt.Frame()
			if a.PluginScroll.Y == before {
				t.Fatal("focus trapped wheel")
			}
			var want any
			switch kind {
			case plugincfg.Radio:
				tt.Key(0, ui.KeyRight)
				want = "south"
			case plugincfg.Checkboxes:
				tt.Key(0, ui.KeySpace)
				want = []string{"south"}
			case plugincfg.Toggle:
				tt.Key(0, ui.KeySpace)
				want = true
			case plugincfg.Select:
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				want = "south"
			case plugincfg.Slider:
				tt.Key(0, ui.KeyRight)
				want = float64(13)
			case plugincfg.Action:
				tt.Key(0, ui.KeyEnter)
			default:
				tt.Key(ui.Cmd, ui.KeyA)
				if kind == plugincfg.Number {
					tt.Type("24")
					want = float64(24)
				} else {
					tt.Type("continued")
					tt.Key(0, ui.KeyEnter)
					want = "continued"
				}
			}
			tt.Frame()
			if kind == plugincfg.Action {
				if sheet.Armed != field.Key || calls != 1 {
					t.Fatalf("action ownership: armed=%s calls=%d", sheet.Armed, calls)
				}
			} else if !reflect.DeepEqual(sheet.Values[field.Key], want) || calls == 0 {
				t.Fatalf("%s edit ownership: got=%v want=%v calls=%d", name, sheet.Values[field.Key], want, calls)
			}
			// The original hit target is still live; no expanded envelope steals it.
			t.Log(fmt.Sprintf("range=%v calls=%d", maxY, calls))
		})
	}
}

func TestPluginFieldEdgeRadio(t *testing.T) { checkPluginFieldEdge(t, plugincfg.Radio, "radio") }
func TestPluginFieldEdgeCheckboxes(t *testing.T) {
	checkPluginFieldEdge(t, plugincfg.Checkboxes, "checkboxes")
}

func TestPluginFieldEdgeText(t *testing.T)   { checkPluginFieldEdge(t, plugincfg.Text, "text") }
func TestPluginFieldEdgeNumber(t *testing.T) { checkPluginFieldEdge(t, plugincfg.Number, "number") }
