package settingsui

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func runFieldEdgeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KM_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-km-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRunFieldEdge(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			field := &in.Params[4]
			field.Kind = kind
			field.Options = []string{"North", "South"}
			label := field.Label
			if kind == extensions.ParamBoolean {
				field.Default = "false"
			}
			if kind == extensions.ParamSelect {
				field.Default, label = "North", "North"
			}
			var viewport, control ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				viewport = c.Resolve(a.Body.Handle).Bounds()
				form := c.Resolve(a.noticeFocus(c, in.ID, "run").panel)
				if form.Valid() {
					focus := ui.Local(form, "focused-parameter", func() ui.Handle { return ui.Handle{} })
					control = c.Resolve(*focus).Bounds()
				}
			})
			runFieldEdgeDump(t, tt, kind+"-rest", dark)
			for i := 0; i < 80 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatal("cannot reach run field")
			}
			bounds := func() ui.Rect {
				if kind == extensions.ParamSelect {
					return mustFind(t, tt, label)
				}
				return control
			}
			target := bounds()
			absoluteY := target.Y + a.Body.Y
			for _, bottom := range []bool{false, true} {
				if bottom {
					tt.Key(ui.Shift, ui.KeyTab)
				} else {
					tt.Key(0, ui.KeyTab)
				}
				tt.Frame()
				a.Body.Y = absoluteY - viewport.Y + 8
				if bottom {
					a.Body.Y = absoluteY - viewport.Y - viewport.H + 8
				}
				tt.Frame()
				if bottom {
					tt.Key(0, ui.KeyTab)
				} else {
					tt.Key(ui.Shift, ui.KeyTab)
				}
				tt.Frame()
				target = bounds()
				side := "top"
				if bottom {
					side = "bottom"
				}
				runFieldEdgeDump(t, tt, kind+"-"+side, dark)
				if !tt.Focused(label) {
					t.Fatal("field entry lost focus")
				}
				inset := float32(0)
				if kind == extensions.ParamBoolean || kind == extensions.ParamSelect {
					inset = -2
				}
				t.Logf("%s %s target=%+v viewport=%+v", kind, side, target, viewport)
				for edge, y := range map[string]float32{"top": target.Y + inset, "bottom": target.Y + target.H - 1 - inset} {
					r, _, b := rgbAt(tt.Image(), int(target.X+target.W/2), int(math.Round(float64(y))))
					if b-r < 60 {
						t.Errorf("%s %s %s active edge missing: blue-red=%d", kind, side, edge, b-r)
					}
				}
				if target.Y+inset < viewport.Y+16 || target.Y+target.H-inset > viewport.Y+viewport.H-16 {
					t.Errorf("%s active edge intersects page fade: %+v", kind, target)
				}
			}
			runVisibilityWheel(t, a, tt)
			want := "continued"
			switch kind {
			case extensions.ParamBoolean:
				tt.Key(0, ui.KeySpace)
				want = "true"
			case extensions.ParamSelect:
				tt.Key(0, ui.KeyRight)
				want = "South"
			default:
				if kind == extensions.ParamNumber {
					want = "24"
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type(want)
			}
			tt.Frame()
			if values := a.runValues[in.ID]; values[field.ID] != want || values["p5"] != "12" {
				t.Fatalf("run edit ownership: %v", values)
			}
		})
	}
}

func TestRunFieldEdgeText(t *testing.T)   { checkRunFieldEdge(t, extensions.ParamText) }
func TestRunFieldEdgeNumber(t *testing.T) { checkRunFieldEdge(t, extensions.ParamNumber) }

func TestRunFieldEdgePath(t *testing.T)    { checkRunFieldEdge(t, extensions.ParamPath) }
func TestRunFieldEdgeBoolean(t *testing.T) { checkRunFieldEdge(t, extensions.ParamBoolean) }

func TestRunFieldEdgeSelect(t *testing.T) { checkRunFieldEdge(t, extensions.ParamSelect) }

func TestConfigMultiPageEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			in.Params, a.Actions.RunWithParams = nil, nil
			for i := 0; i < 12; i++ {
				in.Config = append(in.Config, ConfigField{Key: fmt.Sprint(i), Label: fmt.Sprintf("配置 %d", i), Type: "text", Default: "stored"})
			}
			field := &in.Config[4]
			field.Type, field.Default, field.Options = "multiselect", nil, []string{"JSON", "CSV", "Text", "HTML", "XML"}
			a.configOpen = map[string]bool{in.ID: true}
			a.Actions.SaveConfiguration = func(string, map[string]any, func(error)) { t.Error("focus saved configuration") }
			list := a.configListScroll(in.ID, field.Key)
			var viewport, inner ui.Rect
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				viewport = c.Resolve(a.Body.Handle).Bounds()
				inner = c.Resolve(list.Handle).Bounds()
			})
			runFieldEdgeDump(t, tt, "multi-page-rest", dark)
			for i := 0; i < 80 && !tt.Focused("JSON"); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused("JSON") {
				t.Fatal("cannot reach first option")
			}
			absoluteY := inner.Y + a.Body.Y
			for _, bottom := range []bool{false, true} {
				tt.Key(ui.Shift, ui.KeyTab)
				tt.Frame()
				a.Body.Y = absoluteY - viewport.Y + 8
				if bottom {
					a.Body.Y = absoluteY - viewport.Y - viewport.H + 8
				}
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				side := "top"
				if bottom {
					side = "bottom"
				}
				runFieldEdgeDump(t, tt, "multi-page-"+side, dark)
				if !tt.Focused("JSON") {
					t.Fatal("list entry lost current option")
				}
				if inner.Y-4 < viewport.Y+16 || inner.Y+inner.H+4 > viewport.Y+viewport.H-16 {
					t.Errorf("multiselect %s enters page fade: inner=%+v page=%+v", side, inner, viewport)
				}
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			values := a.draft(*in).values
			selected, ok := values[field.Key].([]any)
			if !ok || len(selected) != 1 || selected[0] != "JSON" || values["5"] != "stored" {
				t.Fatalf("list selection ownership: %v", values)
			}
			runVisibilityWheel(t, a, tt)
		})
	}
}
