package settingsui

import (
	"fmt"
	"image/png"
	"os"
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
