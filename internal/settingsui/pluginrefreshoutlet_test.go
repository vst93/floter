package settingsui

import (
	"image/png"
	"math"
	"os"
	"reflect"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func pluginRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KO_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ko-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkPluginRefreshOutlet(t *testing.T, kind plugincfg.Kind, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			sheet := PluginSheet{Schema: longSchema("refresh"), Values: plugincfg.Values{}}
			field := &sheet.Schema.Fields[5]
			field.Kind, field.Label = kind, "当前字段"
			field.Min, field.Max, field.Step = 0, 100, 1
			field.Options = []plugincfg.Option{{Value: "north", Label: "北方"}, {Value: "south", Label: "南方"}}
			field.Confirm, field.Cancel, field.Failed = "确认清理", "取消清理", "清理失败"
			var initial any = "stored"
			switch kind {
			case plugincfg.Radio, plugincfg.Select:
				initial = "north"
			case plugincfg.Toggle:
				initial = false
			case plugincfg.Checkboxes:
				initial = []string{}
			case plugincfg.Number, plugincfg.Slider:
				initial = float64(12)
			}
			sheet.Values[field.Key] = initial
			calls := 0
			sheet.Change = func(key string, value any) {
				if key != field.Key {
					t.Errorf("refresh wrote sibling %s", key)
				}
				calls++
				sheet.Values[key] = value
			}
			sheet.Arm = func(key string) { calls++; sheet.Armed = key }
			sheet.Run = func(string) { t.Error("refresh ran destructive action") }
			sheet.Unarm = func(string) { sheet.Armed = "" }
			var target, viewport ui.Rect
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
			pluginRefreshDump(t, tt, name+"-rest", dark)
			for i := 0; i < 45 && !focused; i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !focused {
				t.Fatal("cannot reach refresh target")
			}
			bounds := func() ui.Rect {
				// The two choices share one line; use its full layout box,
				// never Tester.Find's clipped text rectangle.
				return target
			}
			check := func(stage string) {
				t.Helper()
				r := bounds()
				pluginRefreshDump(t, tt, name+"-"+stage, dark)
				t.Logf("%s %s target=%+v scroll=%v", name, stage, r, a.PluginScroll.Y)
				if !focused {
					t.Errorf("%s lost current control", stage)
				}
				if math.Floor(float64(r.Y-3)) < math.Floor(float64(viewport.Y+11)) || math.Ceil(float64(r.Y+r.H+3)) > math.Ceil(float64(viewport.Y+viewport.H-11)) {
					t.Errorf("%s %s refresh hides active ink: %+v", name, stage, r)
				}
			}
			// Preserve an unsubmitted text/partial number while the host changes layout.
			if kind == plugincfg.Text || kind == plugincfg.Number {
				tt.Key(ui.Cmd, ui.KeyA)
				if kind == plugincfg.Text {
					tt.Type("continued")
				} else {
					tt.Type("1e")
				}
				tt.Frame()
			}
			sheet.Failed = true
			tt.Frame()
			tt.Frame()
			check("failure")
			// A real host regroup moves the same editor, without a focus transition.
			for i := range sheet.Schema.Fields {
				sheet.Schema.Fields[i].Section = "存储与恢复"
			}
			tt.Frame()
			tt.Frame()
			check("heading")
			// Removing the status at the upper edge must reveal in the other direction.
			a.PluginScroll.Y += bounds().Y - viewport.Y - 16
			tt.Frame()
			tt.Frame()
			sheet.Failed = false
			tt.Frame()
			tt.Frame()
			check("recovery")
			if calls != 0 {
				t.Fatalf("layout refresh wrote %d times", calls)
			}
			if kind == plugincfg.Text && a.pluginTextDrafts["refresh\x00"+field.Key] != "continued" {
				t.Fatal("refresh discarded text draft")
			}
			if kind == plugincfg.Number && a.pluginNumberDrafts["refresh\x00"+field.Key] != "1e" {
				t.Fatal("refresh discarded partial number")
			}
			// Wheel and value-only updates must not turn into a continuous focus follower.
			tt.Wheel(280, 130, 0, 70)
			tt.Frame()
			manual := a.PluginScroll.Y
			sheet.Values["fielda"] = "host refreshed value"
			tt.Frame()
			tt.Frame()
			if a.PluginScroll.Y != manual {
				t.Fatalf("value refresh stole scroll: %v -> %v", manual, a.PluginScroll.Y)
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
			case plugincfg.Slider:
				tt.Key(0, ui.KeyRight)
				want = float64(13)
			case plugincfg.Action:
				tt.Key(0, ui.KeyEnter)
			case plugincfg.Text:
				tt.Key(0, ui.KeyEnter)
				want = "continued"
			case plugincfg.Number:
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				want = float64(24)
			}
			tt.Frame()
			if kind == plugincfg.Action {
				if calls != 1 || sheet.Armed != field.Key {
					t.Fatalf("action ownership: %v / %d", sheet.Armed, calls)
				}
			} else if calls != 1 || !reflect.DeepEqual(sheet.Values[field.Key], want) {
				t.Fatalf("edit ownership: got=%v want=%v calls=%d", sheet.Values[field.Key], want, calls)
			}
		})
	}
}

func TestPluginRefreshOutletRadio(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Radio, "radio")
}
func TestPluginRefreshOutletCheckboxes(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Checkboxes, "checkboxes")
}

func TestPluginRefreshOutletText(t *testing.T) { checkPluginRefreshOutlet(t, plugincfg.Text, "text") }
func TestPluginRefreshOutletNumber(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Number, "number")
}

func TestPluginRefreshOutletToggle(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Toggle, "toggle")
}
func TestPluginRefreshOutletSelect(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Select, "select")
}

func TestPluginRefreshOutletSlider(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Slider, "slider")
}
func TestPluginRefreshOutletAction(t *testing.T) {
	checkPluginRefreshOutlet(t, plugincfg.Action, "action")
}

func TestPluginRefreshWrappedOptionKeepsItsValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			sheet := PluginSheet{Schema: &plugincfg.Schema{Plugin: "export", Title: "导出设置", Fields: []plugincfg.Field{{Key: "formats", Kind: plugincfg.Checkboxes, Label: "导出格式"}}}, Values: plugincfg.Values{}}
			field := &sheet.Schema.Fields[0]
			for _, value := range []string{"csv", "json", "text", "markdown", "html", "xml", "yaml", "tsv", "log"} {
				field.Options = append(field.Options, plugincfg.Option{Value: value, Label: "保留完整时间戳和来源的导出格式 " + value})
			}
			calls := 0
			sheet.Change = func(key string, value any) {
				if key != "formats" {
					t.Errorf("wrong field %q", key)
				}
				calls++
				sheet.Values[key] = value
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				a.PluginSheet(c, sheet)
			}, 320, 180)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			chosen := field.Options[0]
			if !tt.Focused(chosen.Label) {
				t.Fatal("cannot focus first option")
			}
			maxY := a.PluginScroll.MaxY
			// Reorder the very same backing slice, then move the focused option back.
			for _, last := range []bool{true, false} {
				if last {
					copy(field.Options, field.Options[1:])
					field.Options[len(field.Options)-1] = chosen
				} else {
					copy(field.Options[1:], field.Options[:len(field.Options)-1])
					field.Options[0] = chosen
				}
				tt.Frame()
				tt.Frame()
				if !tt.Focused(chosen.Label) {
					t.Fatal("reorder lost option identity")
				}
				r := mustFind(t, tt, chosen.Label)
				if r.H != 17 || r.Y-3 < 11 || r.Y+r.H+3 > 169 {
					t.Errorf("reordered option clipped: %+v", r)
				}
				if a.PluginScroll.MaxY != maxY {
					t.Fatal("reorder changed layout range")
				}
				stage := "wrapped-first"
				if last {
					stage = "wrapped-last"
				}
				pluginRefreshDump(t, tt, stage, dark)
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if !reflect.DeepEqual(sheet.Values[field.Key], []string{chosen.Value}) {
					t.Fatalf("Space wrote wrong choice: %v", sheet.Values)
				}
				if err := tt.Click(chosen.Label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if len(stringsOf(sheet.Values[field.Key])) != 0 {
					t.Fatal("refresh envelope stole pointer")
				}
			}
			if calls != 4 {
				t.Fatalf("unexpected writes %d", calls)
			}
		})
	}
}
