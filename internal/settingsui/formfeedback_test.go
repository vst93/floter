package settingsui

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func formFeedbackDump(t *testing.T, tt *ui.Tester, scenario string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KR_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kr-" + scenario + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkFeedbackVisible(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r, viewport := mustFind(t, tt, label), mustFind(t, tt, "feedback-viewport")
	if !tt.Focused(label) || r.W <= 0 || r.H <= 0 || r.Y-3 < viewport.Y+16 || r.Y+r.H+3 > viewport.Y+viewport.H-16 {
		t.Errorf("feedback reflow hides active outlet %q: target=%+v viewport=%+v focused=%v", label, r, viewport, tt.Focused(label))
	}
}

func checkRunFeedback(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			for i := 0; i < 4; i++ {
				in.Params[i].Default = ""
			}
			in.Params[4].Kind = kind
			if kind == "boolean" {
				in.Params[4].Default = "false"
			}
			label := "参数 5"
			if kind == "select" {
				in.Params[4].Default, in.Params[4].Options = "北方", []string{"北方", "南方"}
				label = "北方"
			}
			if kind == "cancel" {
				in.Params[4].Kind, label = "text", a.copy().RunCancel
			}
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
			optionExitFocus(t, tt, label)
			editable := kind == "text" || kind == "number" || kind == "path"
			want := "continued"
			if kind == "number" {
				want = "1e"
			}
			if editable {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type(want)
				tt.Frame()
			}
			formFeedbackDump(t, tt, "run-"+kind+"-rest", dark)
			for step, required := range []bool{true, false, true} {
				for i := 0; i < 4; i++ {
					in.Params[i].Required = required
				}
				tt.Frame()
				formFeedbackDump(t, tt, fmt.Sprintf("run-%s-change%d", kind, step), dark)
				checkFeedbackVisible(t, tt, label)
				if editable && a.runValues[in.ID]["p4"] != want {
					t.Fatal("feedback changed live answer")
				}
			}
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Fatal("stable feedback reclaimed manual scroll")
			}
			if editable {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				tt.Frame()
				if a.runValues[in.ID]["p4"] != "24" {
					t.Fatal("input lost original ownership")
				}
			} else if kind == "boolean" {
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.runValues[in.ID]["p4"] != "true" {
					t.Fatal("toggle lost original ownership")
				}
			} else if kind == "select" {
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if a.runValues[in.ID]["p4"] != "南方" {
					t.Fatal("selection lost original ownership")
				}
			} else {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.runOpen[in.ID] || a.runValues[in.ID]["p4"] != "12" {
					t.Fatal("cancel did not preserve original answers")
				}
			}
		})
	}
}

func TestFormFeedbackRunEditors(t *testing.T) {
	for _, kind := range []string{"text", "number", "path"} {
		t.Run(kind, func(t *testing.T) { checkRunFeedback(t, kind) })
	}
}
func TestFormFeedbackRunToggle(t *testing.T) { checkRunFeedback(t, "boolean") }
func TestFormFeedbackRunSelect(t *testing.T) { checkRunFeedback(t, "select") }
func TestFormFeedbackRunCancel(t *testing.T) { checkRunFeedback(t, "cancel") }

func checkConfigFeedback(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
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
				field.Default = []any{"北方"}
			case "defaults", "save", "copy", "export":
				field.Type = "text"
			}
			a.configOpen = map[string]bool{in.ID: true}
			draft := a.draft(*in)
			draft.values["c0"] = "dirty"
			calls := 0
			var pending func(error)
			a.Actions.SaveConfiguration = func(id string, values map[string]any, done func(error)) {
				calls++
				if id != in.ID || values["c0"] != "dirty" {
					t.Error("feedback changed save owner or payload")
				}
				pending = done
			}
			transfer := func(id string, values map[string]any, done func(error)) {
				calls++
				if id != in.ID || values["c0"] != "dirty" {
					t.Error("feedback changed transfer owner or payload")
				}
				done(nil)
			}
			a.Actions.CopyConfiguration, a.Actions.ExportConfiguration = transfer, transfer
			label := field.Label
			switch kind {
			case "multiselect":
				label = "北方"
			case "defaults":
				label = a.copy().ConfigDefaults
			case "save":
				label = a.copy().ConfigSave
			case "copy":
				label = a.copy().ConfigCopy
			case "export":
				label = a.copy().ConfigExport
			}
			outside := false
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
				c.Resolve(a.Body.Handle).Label("feedback-viewport")
				if outside {
					ui.Button(c, "外部出口").Absolute().Top(4).Right(4)
				}
			})
			optionExitFocus(t, tt, label)
			editable := kind == "text" || kind == "password" || kind == "number" || kind == "path"
			want := "continued"
			if kind == "number" {
				want = "1e"
			}
			if editable {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type(want)
				tt.Frame()
			}
			formFeedbackDump(t, tt, "config-"+kind+"-rest", dark)
			for step, present := range []bool{true, false, true} {
				if kind == "defaults" || kind == "save" {
					message := ""
					if present {
						message = strings.Repeat("写入失败，请检查目录权限。\n", 4)
					}
					if kind == "save" && present {
						tt.Key(0, ui.KeyEnter)
						tt.Frame()
						if pending == nil {
							t.Fatal("save not dispatched")
						}
						pending(errors.New(message))
					} else {
						a.configError[in.ID] = message
					}
				} else if kind == "copy" || kind == "export" {
					in.Error = ""
					if present {
						in.Error = strings.Repeat("程序自报诊断信息。\n", 24)
					}
				} else {
					// An existing editor's schema refresh revalidates its live value.
					limit := 40
					if present {
						limit = 2
					}
					for i := 0; i < 4; i++ {
						in.Config[i].MaxLength = &limit
						draft.setLive(in.Config[i], "stored")
					}
				}
				tt.Frame()
				formFeedbackDump(t, tt, fmt.Sprintf("config-%s-change%d", kind, step), dark)
				checkFeedbackVisible(t, tt, label)
				if editable && draft.live["c4"] != want {
					t.Fatal("feedback discarded editor draft")
				}
			}
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Fatal("stable feedback reclaimed scroll")
			}
			var expected any
			if editable {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				tt.Frame()
				expected = "24"
				if kind == "number" {
					expected = float64(24)
				}
			} else {
				switch kind {
				case "boolean":
					tt.Key(0, ui.KeySpace)
					expected = true
				case "multiselect":
					tt.Key(0, ui.KeySpace)
					expected = []any(nil)
				case "select":
					tt.Key(0, ui.KeyDown)
					tt.Key(0, ui.KeyEnd)
					tt.Key(0, ui.KeyEnter)
					expected = "南方"
				default:
					tt.Key(0, ui.KeyEnter)
					expected = "stored"
				}
				tt.Frame()
			}
			if !reflect.DeepEqual(draft.values["c4"], expected) {
				t.Fatalf("feedback ownership: got=%v want=%v", draft.values["c4"], expected)
			}
			if kind == "save" {
				if calls != 3 {
					t.Fatalf("save retries=%d want=3", calls)
				}
				pending(nil)
				tt.Frame()
			} else if kind == "copy" || kind == "export" {
				if calls != 1 {
					t.Fatalf("transfer calls=%d want=1", calls)
				}
			} else if calls != 0 {
				t.Fatalf("feedback dispatched %d operations", calls)
			}
			if kind == "defaults" && draft.values["c0"] != "stored" {
				t.Fatal("defaults did not restore")
			}
			outside = true
			tt.Frame()
			if err := tt.Click("外部出口"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			manual = a.Body.Y
			a.configError[in.ID] = "另一条错误"
			tt.Frame()
			if a.Body.Y != manual {
				t.Fatal("unfocused feedback stole scroll")
			}
		})
	}
}

func TestFormFeedbackConfigFields(t *testing.T) {
	for _, kind := range []string{"text", "password", "path", "number", "boolean", "select", "multiselect"} {
		t.Run(kind, func(t *testing.T) { checkConfigFeedback(t, kind) })
	}
}
func TestFormFeedbackConfigDefaults(t *testing.T) { checkConfigFeedback(t, "defaults") }
