package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"reflect"
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func formViewportDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KQ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kq-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkFormViewport(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			config := strings.HasPrefix(scenario, "config-")
			kind := strings.TrimPrefix(strings.TrimPrefix(scenario, "config-"), "run-")
			label := "参数 5"
			calls := 0
			a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
				calls++
				if id != in.ID || len(values) != 12 || values["p0"] != "12" {
					t.Errorf("resize changed run payload: %v", values)
				}
				return nil
			}
			if config {
				in.Params, a.Actions.RunWithParams = nil, nil
				for i := 0; i < 12; i++ {
					in.Config = append(in.Config, ConfigField{Key: fmt.Sprintf("c%d", i), Label: fmt.Sprintf("配置 %d", i+1), Type: "text", Default: "stored"})
				}
				field := &in.Config[4]
				field.Type, field.Options = kind, []string{"北方", "南方"}
				switch kind {
				case "number":
					field.Default = float64(12)
				case "boolean":
					field.Default = false
				case "select":
					field.Default = "北方"
				case "multiselect":
					field.Default = []string{"北方"}
				case "defaults", "save":
					field.Type = "text"
				}
				a.configOpen = map[string]bool{in.ID: true}
				label = field.Label
				if kind == "multiselect" {
					label = "北方"
				}
				if kind == "defaults" {
					label = a.copy().ConfigDefaults
				}
				if kind == "save" {
					label = a.copy().ConfigSave
				}
				a.Actions.SaveConfiguration = func(id string, values map[string]any, done func(error)) {
					calls++
					if id != in.ID || values["c0"] != "dirty" {
						t.Errorf("resize changed save payload: %v", values)
					}
					done(nil)
				}
				a.draft(*in).values["c0"] = "dirty"
			} else {
				field := &in.Params[4]
				field.Kind, field.Options = kind, []string{"北方", "南方"}
				if kind == "boolean" {
					field.Default = "false"
				}
				if kind == "select" {
					field.Default, label = "北方", "北方"
				}
				if kind == "cancel" {
					field.Kind, label = extensions.ParamText, a.copy().RunCancel
				}
				if kind == "submit" {
					field.Kind, label = extensions.ParamText, a.copy().RunLabel
				}
			}
			outside := false
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				c.Resolve(a.Body.Handle).Label("form-viewport")
				if outside {
					ui.Button(c, "外部出口").Absolute().Top(4).Right(4)
				}
			})
			formViewportDump(t, tt, scenario+"-rest", dark)
			optionExitFocus(t, tt, label)
			editable := kind == "text" || kind == "number" || kind == "path" || kind == "password"
			if editable {
				tt.Key(ui.Cmd, ui.KeyA)
				value := "continued"
				if kind == "number" {
					value = "1e"
				}
				tt.Type(value)
				tt.Frame()
			}
			for _, size := range [][2]int{{720, 360}, {620, 320}, {720, 520}} {
				tt.SetSize(size[0], size[1])
				r, viewport := mustFind(t, tt, label), mustFind(t, tt, "form-viewport")
				formViewportDump(t, tt, fmt.Sprintf("%s-%dx%d", scenario, size[0], size[1]), dark)
				t.Logf("%s size=%v target=%+v viewport=%+v scroll=%v", scenario, size, r, viewport, a.Body.Y)
				if !tt.Focused(label) || r.W <= 0 || r.H <= 0 || r.Y-3 < viewport.Y+16 || r.Y+r.H+3 > viewport.Y+viewport.H-16 {
					t.Errorf("%s resize hides active outlet: target=%+v viewport=%+v focused=%v", scenario, r, viewport, tt.Focused(label))
				}
			}
			if calls != 0 {
				t.Fatalf("resize dispatched %d operations", calls)
			}
			if editable {
				want := "continued"
				if kind == "number" {
					want = "1e"
				}
				got := a.runValues[in.ID]["p4"]
				if config {
					got = a.draft(*in).live["c4"]
				}
				if got != want {
					t.Fatalf("resize lost draft: got=%q want=%q", got, want)
				}
			}
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Fatal("stable viewport reclaimed manual scroll")
			}
			if editable {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
			} else {
				switch kind {
				case "boolean", "multiselect":
					tt.Key(0, ui.KeySpace)
				case "select":
					if config {
						tt.Key(0, ui.KeyDown)
						tt.Key(0, ui.KeyEnd)
						tt.Key(0, ui.KeyEnter)
					} else {
						tt.Key(0, ui.KeyRight)
					}
				default:
					tt.Key(0, ui.KeyEnter)
				}
			}
			tt.Frame()
			if config {
				var want any = "24"
				switch kind {
				case "number":
					want = float64(24)
				case "boolean":
					want = true
				case "select":
					want = "南方"
				case "multiselect":
					want = []string{}
				case "defaults", "save":
					want = "stored"
				}
				if !reflect.DeepEqual(a.draft(*in).values["c4"], want) {
					t.Errorf("resized edit ownership: got=%v want=%v", a.draft(*in).values["c4"], want)
				}
				if kind == "defaults" && a.draft(*in).values["c0"] != "stored" {
					t.Error("defaults cannot restore")
				}
				if kind == "save" && calls != 1 || kind != "save" && calls != 0 {
					t.Errorf("save calls=%d", calls)
				}
			} else {
				want := "24"
				switch kind {
				case "boolean":
					want = "true"
				case "select":
					want = "南方"
				case "cancel", "submit":
					want = "12"
				}
				if a.runValues[in.ID]["p4"] != want {
					t.Errorf("resized answer=%q want=%q", a.runValues[in.ID]["p4"], want)
				}
				if kind == "cancel" && a.runOpen[in.ID] {
					t.Error("cancel did not close form")
				}
				if kind == "submit" && calls != 1 || kind != "submit" && calls != 0 {
					t.Errorf("run calls=%d", calls)
				}
			}
			// Move outside this form before another resize.
			outside = true
			tt.Frame()
			if err := tt.Click("外部出口"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			manual = a.Body.Y
			tt.SetSize(700, 480)
			if a.Body.Y != manual {
				t.Errorf("unfocused resize stole scroll: got=%v want=%v", a.Body.Y, manual)
			}
		})
	}
}

func TestFormViewportRunEditors(t *testing.T) {
	for _, kind := range []string{"text", "number", "path"} {
		t.Run(kind, func(t *testing.T) { checkFormViewport(t, "run-"+kind) })
	}
}
func TestFormViewportRunToggle(t *testing.T) { checkFormViewport(t, "run-boolean") }

func TestFormViewportRunSelect(t *testing.T) { checkFormViewport(t, "run-select") }
func TestFormViewportRunCancel(t *testing.T) { checkFormViewport(t, "run-cancel") }
